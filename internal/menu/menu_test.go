package menu

// 这一份测的是"读出来的清单对不对"，用的是仓库里真那份数据（embed 进来的 11 个目录）。
//
// 干什么用：① 证明两支 embed 通配真能吃到东西（这是 D1 甲唯一的支点，验不过整条"一菜单一目录"退回乙）；
// ② 把 11 块／54 条／65 个域名／47 个父域这些数当场量一遍，数字漂了这里先红；
// ③ 域名形状的判定与生成器（tools/gen-backend-menu.mjs）共用同一张用例表，两边不许各判各的。
//
// 会错在哪：这些数以前只写在文档里靠人眼对；生成物与真源一漂，界面列 54 条、后端认 53 条，
// 少那条点了"应用"什么也不发生。出错时看得见的是这条用例红，消息说清是哪一块、少了什么。
//
// 怎么回来：改前端 frontend/menu/ 那份真源，跑 node tools/gen-backend-menu.mjs 重新生成；
// 不许手改 internal/menu 下的 JSON，也不许为了让这条用例变绿而把期望数改成实测值之外的数。

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func mustLoad(t *testing.T) *Catalog {
	t.Helper()
	c, err := Load()
	if err != nil {
		t.Fatalf("Load 失败：服务启动就该在这儿失败，错误是 %v", err)
	}
	return c
}

func TestLoadReadsEveryBlock(t *testing.T) {
	c := mustLoad(t)
	got := c.Menus()

	want := []struct {
		id    string
		order int
		items int
	}{
		{"github", 1, 8}, {"jetbrains", 2, 6}, {"ai", 3, 6}, {"steam", 4, 5},
		{"gameplat", 5, 6}, {"gamespec", 6, 6}, {"cdn", 7, 5}, {"ms", 8, 4},
		{"coursera", 9, 2}, {"block", 10, 3}, {"custom", 11, 3},
	}
	if len(got) != len(want) {
		t.Fatalf("扫出 %d 块菜单，期望 %d 块：embed 通配或菜单目录漂了", len(got), len(want))
	}
	total := 0
	for i, w := range want {
		m := got[i]
		if m.ID != w.id || m.Order != w.order {
			t.Errorf("第 %d 块应为 %s(order %d)，现在是 %s(order %d)", i+1, w.id, w.order, m.ID, m.Order)
		}
		if len(m.Items) != w.items {
			t.Errorf("%s 的条目数：期望 %d，实测 %d", w.id, w.items, len(m.Items))
		}
		if m.Name == "" {
			t.Errorf("%s 没有菜单名，界面上那块会是空白", w.id)
		}
		total += len(m.Items)
	}
	if total != 54 {
		t.Errorf("二级条目合计 %d，期望 54（与原型 CATEGORY_DEFS 逐字对过的那个数由前端门禁 G2 守，这里守两边条数一致）", total)
	}
}

// TestEmbedWildcards 就是 D1 甲那个〔待核〕：两支通配能不能各吃到 11 份。
// 通配 * 不跨路径分隔符，所以 menu.json 那一支吃不到 common 子目录里的东西，两支互不串——这里把结论钉成用例。
func TestEmbedWildcards(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    fs.FS
		base string
	}{
		{"menu.json 那一支", menuData, "menu.json"},
		{"common.json 那一支", commonData, "common.json"},
	} {
		var n int
		err := fs.WalkDir(tc.f, ".", func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && filepath.Base(p) == tc.base {
				n++
			}
			return err
		})
		if err != nil {
			t.Fatalf("%s 走不下去：%v", tc.name, err)
		}
		if n != 11 {
			t.Errorf("%s 吃到 %d 份，期望 11 份：一支也没吃到时服务启动当场失败，错误消息点名通配漂了（load.go 里那句）", tc.name, n)
		}
	}
}

// TestDomainShapeSharedTable 与生成器共用 tools/domain-cases.json 那一张表：判定两边各写一份，用例不许抄两份。
func TestDomainShapeSharedTable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "tools", "domain-cases.json"))
	if err != nil {
		t.Fatalf("读不到用例表：%v", err)
	}
	var cases struct {
		Valid   []string `json:"valid"`
		Invalid []string `json:"invalid"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("用例表解不开：%v", err)
	}
	for _, s := range cases.Valid {
		if !isDomain(s) {
			t.Errorf("表里判合格，代码判不合格：%q", s)
		}
	}
	for _, s := range cases.Invalid {
		if isDomain(s) {
			t.Errorf("表里判不合格，代码判合格：%q", s)
		}
	}
}

func TestParentDomainsCounts(t *testing.T) {
	c := mustLoad(t)
	want := map[string]int{
		"github": 4, "jetbrains": 1, "ai": 10, "steam": 4,
		"gameplat": 8, "gamespec": 7, "cdn": 6, "ms": 3,
		"coursera": 1, "block": 2, "custom": 1,
	}
	total := 0
	seen := map[string]int{}
	for id, n := range want {
		got := c.ParentDomains(id)
		if len(got) != n {
			t.Errorf("%s 的父域：%d，期望 %d（现在是 %v）", id, len(got), n, got)
		}
		total += len(got)
		for _, p := range got {
			seen[p]++
		}
	}
	if total != 47 {
		t.Errorf("父域合计 %d，期望 47", total)
	}
	if len(seen) != 44 {
		t.Errorf("父域去重后 %d，期望 44", len(seen))
	}
	var shared []string
	for p, n := range seen {
		if n > 1 {
			shared = append(shared, p)
		}
	}
	sort.Strings(shared)
	if want2 := []string{"ea.com", "googleapis.com", "ubisoft.com"}; !reflect.DeepEqual(shared, want2) {
		t.Errorf("跨块重叠的父域现在是 %v，期望 %v：这两块菜单谁管它，界面上那句话要说清", shared, want2)
	}
}

func TestContentIssuesAreKeptAndListed(t *testing.T) {
	c := mustLoad(t)

	if got := len(c.Domains()); got != 65 {
		t.Errorf("去重后的合法受管域名 %d 个，期望 65：这个数一变，「全部恢复默认」的范围就跟着变了", got)
	}

	wantUnsupported := []Unsupported{
		{"block", "bl-steven", "raw.githubusercontent.com/StevenBlack/hosts"},
		{"custom", "cu-manual", "自定义域名"},
		{"custom", "cu-china", "github.com/zhouzhehong/china-hosts-accelerator"},
	}
	if got := c.UnsupportedDomains(); !reflect.DeepEqual(got, wantUnsupported) {
		t.Errorf("不合格数据项清单漂了：\n 现在 %v\n 期望 %v\n〔已定：D4 甲，54 条数据一个字不改，由写入那一刻拒绝并如实标出来〕", got, wantUnsupported)
	}

	wantConflicts := []Conflict{{Domain: "ubisoft.com", Items: []string{"gp-ubi", "gs-r6"}}}
	if got := c.DomainConflicts(); !reflect.DeepEqual(got, wantConflicts) {
		t.Errorf("一个域名挂多条条目的清单漂了：现在 %v，期望 %v", got, wantConflicts)
	}

	if _, _, ok := c.ByItem("nosuch-item"); ok {
		t.Error("查不存在的条目却返回 ok=true：调用方会拿零值往下走")
	}
	if _, ok := c.Common("nosuch-menu"); ok {
		t.Error("查不存在的菜单却返回 ok=true")
	}
}

func TestCatalogIsReadOnly(t *testing.T) {
	c := mustLoad(t)

	m := c.Menus()[0]
	m.Items[0].Name = "被人改了"
	m.Items[0].Domains[0] = "被人改了.com"
	m.Common.ParentDomains[0] = "被人改了.com"
	c.Domains()[0] = "被人改了.com"
	if it, _, ok := c.ByItem(m.Items[0].ID); ok {
		it.Name = "被人改了"
	}

	if again := c.Menus()[0]; again.Items[0].Name == "被人改了" || again.Items[0].Domains[0] == "被人改了.com" {
		t.Error("改返回值能改到清单本体：那份「只读清单」是假的")
	}
	if p := c.ParentDomains("github"); p[0] == "被人改了.com" {
		t.Error("父域清单被调用方改动了")
	}
	if d := c.Domains(); d[0] == "被人改了.com" {
		t.Error("域名清单被调用方改动了")
	}
}
