package menu

// 读取层：把 11 个数据目录吃进内存、严格解码、排好序、交给校验层。
//
// 干什么用：服务启动那一刻把这块菜单的家变成一份只读清单（Catalog），
// 上层（internal/service）拿到的东西要么完整要么干脆起不来，不存在"少一块菜单也能跑"。
//
// 会错在哪：那两条 embed 通配是这一层唯一的支点——通配一漂，编译照样过、服务照样起，
// 只是清单变成 0 块，界面上菜单空着，用户以为程序坏了；所以一个也没读到时错误消息必须点名"通配漂了"。
// 数据里的键名漂一个字母（parentDomains 写成 parentDomain）如果用宽松解码会被静默忽略，
// 于是"这块菜单管哪些站"那句话永远为空，也没人报错——这里一律用严格解码，未知键当场抛。
//
// 怎么回来：错误消息带目录名（menu/github/…），改哪一眼就找得着；
// 目录形状本身另有门禁 G5 守着（菜单目录里不许出现 .go），不在这一层重复检查。

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// menuData 吃每个菜单目录根下的那一份 menu.json。
//
//go:embed */menu.json
var menuData embed.FS

// commonData 吃每个菜单目录下面 common 子目录里的那一份 common.json。
// 分成两个变量而不是并成一条，是为了出错时能说出是哪一支通配漂了——并成一条只会给一个"0 个文件"。
//
//go:embed */common/common.json
var commonData embed.FS

// Load 读全部菜单，校验后返回只读清单。任何一块读不到、解不出、对不上，都当场返回错误，
// 不静默跳过：菜单少一条，界面上那条就点不动，那种「看起来是对的」比起不来更难查。
func Load() (*Catalog, error) {
	ids, err := collectIDs(menuData, "menu.json", "")
	if err != nil {
		return nil, err
	}
	commonIDs, err := collectIDs(commonData, "common.json", "common")
	if err != nil {
		return nil, err
	}
	if err := sameSet(ids, commonIDs); err != nil {
		return nil, err
	}

	c := &Catalog{byItem: map[string]itemRef{}}
	for _, id := range ids {
		m, err := readMenu(id)
		if err != nil {
			return nil, err
		}
		var mc MenuCommon
		if err := decodeEmbedded(commonData, id+"/common/common.json", &mc); err != nil {
			return nil, err
		}
		m.Common = mc
		c.menus = append(c.menus, m)
	}

	sort.SliceStable(c.menus, func(i, j int) bool { return c.menus[i].Order < c.menus[j].Order })

	if err := c.index(); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// collectIDs 从 embed 里还原「有哪些菜单目录」，并当场兜住三种通配吃歪的形状：
// 一支也没吃到、吃到了不该吃的文件名、路径层数不对（那正是"有人给条目开了目录"的形状）。
// sub 是菜单目录名与文件之间那段固定目录名：menu.json 那支传空串，common 那支传 common。
func collectIDs(f fs.FS, base, sub string) ([]string, error) {
	shape := "菜单目录名/" + base
	levels := 2 // 路径段数：菜单目录名 + 文件名
	if sub != "" {
		shape = "菜单目录名/" + sub + "/" + base
		levels = 3
	}
	var ids []string
	seen := map[string]bool{}
	err := fs.WalkDir(f, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if path.Base(p) != base {
			return fmt.Errorf("%s 那一支吃到了别的东西 %q：它只该吃「%s」", base, p, shape)
		}
		parts := strings.Split(p, "/")
		if len(parts) != levels {
			return fmt.Errorf("%q 的形状不是「%s」：一块菜单一个目录，条目不建目录", p, shape)
		}
		if sub != "" && parts[1] != sub {
			return fmt.Errorf("%q 中间那层目录叫 %q 不叫 %q：公共层的家只有这一个名字", p, parts[1], sub)
		}
		if seen[parts[0]] {
			return fmt.Errorf("菜单目录 %q 有两份 %s：清单会变成掷硬币", parts[0], base)
		}
		seen[parts[0]] = true
		ids = append(ids, parts[0])
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("读 embed 里的 %s 失败：%w", base, err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("embed 里一个 %s 也没读到：多半是它上面那条通配漂了或菜单目录被删，不是「真的没有菜单」", base)
	}
	sort.Strings(ids)
	return ids, nil
}

// sameSet 要求两支通配认出同一批菜单目录：少一份 common.json 会让那块菜单的父域归属静默变成空，
// 界面上看不出任何异常，所以当场抛。
func sameSet(a, b []string) error {
	in := map[string]bool{}
	for _, x := range a {
		in[x] = true
	}
	for _, y := range b {
		if !in[y] {
			return fmt.Errorf("%q 有 common 目录却没有 menu.json：菜单目录形状不对", y)
		}
		delete(in, y)
	}
	missing := make([]string, 0, len(in))
	for x := range in {
		missing = append(missing, x)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("菜单 %v 有 menu.json 却没有 common/common.json：这几块的父域归属没有家", missing)
	}
	return nil
}

func readMenu(id string) (Menu, error) {
	var m Menu
	if err := decodeEmbedded(menuData, id+"/menu.json", &m); err != nil {
		return Menu{}, err
	}
	if m.ID != id {
		return Menu{}, fmt.Errorf("menu/%s/menu.json 里写的 id 是 %q：目录名就是菜单 id，两边必须一样", id, m.ID)
	}
	return m, nil
}

// decodeEmbedded 用严格解码：数据里出现没登记过的键就报错。
// 宽松解码会把「键名漂了」变成"那一栏永远空着"，那是本项目最难查的一类错（契约字段名同一条规矩）。
// 参数取 fs.FS 而不是 embed.FS，是为了让"键名漂了"这种漂移能在测试里注入（fstest.MapFS），不必改仓库里的真数据。
func decodeEmbedded(f fs.FS, name string, out any) error {
	raw, err := fs.ReadFile(f, name)
	if err != nil {
		return fmt.Errorf("读 menu/%s 失败：%w", name, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("menu/%s 解不出来：%w", name, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("menu/%s 里有第二份 JSON：一个文件只该有一个对象", name)
	}
	return nil
}
