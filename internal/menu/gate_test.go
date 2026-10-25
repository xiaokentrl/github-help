package menu

// 门禁与漂移注入。干什么用：把"形状破了要当场红"的几条写成用例——
// G5（菜单目录里不许有 .go、目录里只许有那三样）＋ 读取层与校验层各自的坏形状。
//
// 会错在哪：坏形状多数不会编译失败，只会静默变空或静默多一块，界面看不出来；
// 所以这里每种坏形状都注入一次，断言它确实报错。注入用内存里的假目录（fstest.MapFS）和合成清单，
// 仓库里那 11 个真目录一个字也不动。
//
// 怎么回来：红了照消息改——数据改前端真源再重新生成，代码改这一层，不许改断言把它按下去。

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

func menuDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读不到本包目录：%v", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// TestGateNoGoInsideMenuDirs 是 G5 的第一半：一块菜单一个目录，目录里只有数据。
// 破了它，"加一块菜单＝加一个数据目录"就变成"加一个 Go 包"，逻辑会长出第十三个家。
func TestGateNoGoInsideMenuDirs(t *testing.T) {
	dirs := menuDirs(t)
	if len(dirs) != 11 {
		t.Fatalf("本包下有 %d 个数据目录，期望 11 个：%v", len(dirs), dirs)
	}
	for _, d := range dirs {
		if err := filepath.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				t.Errorf("%s：菜单目录里出现了 .go，代码只许在本包根上（后端规划第 8 节第 5 条）", p)
			}
			return nil
		}); err != nil {
			t.Fatalf("走查 %s 失败：%v", d, err)
		}
	}
}

// TestGateMenuDirShape 是 G5 的第二半：菜单目录根上只许 menu.json + README.md + common/，
// common/ 里只许 common.json + README.md。多出来的那一样（domains.txt、某个条目的目录）就是清单的第二个家。
func TestGateMenuDirShape(t *testing.T) {
	for _, d := range menuDirs(t) {
		root := ls(t, d)
		if want := []string{"README.md", "menu.json"}; !reflect.DeepEqual(root.files, want) {
			t.Errorf("menu/%s 根上的文件是 %v，期望 %v", d, root.files, want)
		}
		if want := []string{"common"}; !reflect.DeepEqual(root.dirs, want) {
			t.Errorf("menu/%s 下的子目录是 %v，期望 %v：子菜单（条目）不建目录〔已定：R9〕", d, root.dirs, want)
		}
		sub := ls(t, filepath.Join(d, "common"))
		if want := []string{"README.md", "common.json"}; !reflect.DeepEqual(sub.files, want) {
			t.Errorf("menu/%s/common 里的文件是 %v，期望 %v", d, sub.files, want)
		}
		if len(sub.dirs) != 0 {
			t.Errorf("menu/%s/common 下还有子目录 %v：公共层只放本块共用的那一样", d, sub.dirs)
		}
	}
}

type lsResult struct {
	dirs  []string
	files []string
}

func ls(t *testing.T, dir string) lsResult {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读不到 %s：%v", dir, err)
	}
	var out lsResult
	for _, e := range entries {
		if e.IsDir() {
			out.dirs = append(out.dirs, e.Name())
		} else {
			out.files = append(out.files, e.Name())
		}
	}
	sort.Strings(out.dirs)
	sort.Strings(out.files)
	return out
}

const (
	fixtureMenu   = `{"id":"github","name":"GitHub","order":1,"items":[{"id":"gh-main","name":"主站","domains":["github.com"]}]}`
	fixtureCommon = `{"parentDomains":["github.com"]}`
)

// TestCollectIDsRejectsBadShapes 注入四种坏形状：一支也没吃到、路径层数不对（条目开了目录）、
// 吃到别的文件名、公共层少了 common 那一层。这四种都不该让服务起来。
func TestCollectIDsRejectsBadShapes(t *testing.T) {
	cases := []struct {
		name string
		f    fs.FS
		base string
		sub  string
		want string
	}{
		{"一支也没吃到", fstest.MapFS{}, "menu.json", "", "也没读到"},
		{"条目开了目录", fstest.MapFS{
			"github/menu.json":         &fstest.MapFile{Data: []byte(fixtureMenu)},
			"github/gh-main/menu.json": &fstest.MapFile{Data: []byte(fixtureMenu)},
		}, "menu.json", "", "形状不是"},
		{"吃到别的文件名", fstest.MapFS{
			"github/menu.json": &fstest.MapFile{Data: []byte(fixtureMenu)},
			"github/extra.md":  &fstest.MapFile{Data: []byte("x")},
		}, "menu.json", "", "吃到了别的东西"},
		{"公共层少一层目录", fstest.MapFS{
			"github/common.json": &fstest.MapFile{Data: []byte(fixtureCommon)},
		}, "common.json", "common", "形状不是"},
	}
	for _, tc := range cases {
		if _, err := collectIDs(tc.f, tc.base, tc.sub); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s：期望报「%s」，实际是 %v", tc.name, tc.want, err)
		}
	}
}

// TestCollectIDsAcceptsTheRealShape 反向兜一手：上面那些断言要是写得太宽，好形状也会被一起报错，
// 那 G5 就成了"永远红"的门禁，同样没人改得动。
func TestCollectIDsAcceptsTheRealShape(t *testing.T) {
	f := fstest.MapFS{"github/menu.json": &fstest.MapFile{Data: []byte(fixtureMenu)}}
	ids, err := collectIDs(f, "menu.json", "")
	if err != nil || !reflect.DeepEqual(ids, []string{"github"}) {
		t.Errorf("好形状被拒了：%v / %v", ids, err)
	}
	g := fstest.MapFS{"github/common/common.json": &fstest.MapFile{Data: []byte(fixtureCommon)}}
	ids, err = collectIDs(g, "common.json", "common")
	if err != nil || !reflect.DeepEqual(ids, []string{"github"}) {
		t.Errorf("好形状被拒了：%v / %v", ids, err)
	}
	if err := sameSet([]string{"github"}, []string{"github"}); err != nil {
		t.Errorf("两边一致却被拒了：%v", err)
	}
}

func TestSameSetReportsWhichSide(t *testing.T) {
	if err := sameSet([]string{"ai", "github"}, []string{"github"}); err == nil || !strings.Contains(err.Error(), "ai") {
		t.Errorf("少一份 common 没报出是哪一块：%v", err)
	}
	if err := sameSet([]string{"github"}, []string{"ai", "github"}); err == nil || !strings.Contains(err.Error(), "ai") {
		t.Errorf("多一个 common 目录没报出来：%v", err)
	}
}

// TestStrictDecodeRejectsUnknownKeys 守的是「键名漂了不报错、那一栏永远空着」这一类最难查的错。
// 三个键都是真会漂的：parentDomain（少个 s）、items（公共层长成第二份条目清单）、init（前端初态混进后端〔已定：D3 甲，init 不进后端〕）。
func TestStrictDecodeRejectsUnknownKeys(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		out  any
	}{
		{"父域键名漂了", `{"parentDomain":["github.com"]}`, &MenuCommon{}},
		{"公共层里出现条目数组", `{"parentDomains":["github.com"],"items":[{"id":"gh-main"}]}`, &MenuCommon{}},
		{"菜单里混进初态", `{"id":"github","name":"GitHub","order":1,"init":"applied","items":[]}`, &Menu{}},
		{"一个文件两份 JSON", "[1]\n{}\n", &Menu{}},
	}
	for _, tc := range cases {
		f := fstest.MapFS{"x": &fstest.MapFile{Data: []byte(tc.raw)}}
		if err := decodeEmbedded(f, "x", tc.out); err == nil {
			t.Errorf("%s：严格解码放过去了，界面会显示成空白而没人报错", tc.name)
		}
	}
}

// TestStrictDecodeAcceptsTheRealKeys 反向兜一手：真源用的那几样键必须解得过，
// 否则上面那条门禁会把正常数据一起拒掉。
func TestStrictDecodeAcceptsTheRealKeys(t *testing.T) {
	f := fstest.MapFS{
		"menu.json":   &fstest.MapFile{Data: []byte(fixtureMenu)},
		"common.json": &fstest.MapFile{Data: []byte(fixtureCommon)},
	}
	var m Menu
	if err := decodeEmbedded(f, "menu.json", &m); err != nil {
		t.Errorf("真源形状被严格解码拒了：%v", err)
	}
	var mc MenuCommon
	if err := decodeEmbedded(f, "common.json", &mc); err != nil {
		t.Errorf("公共层形状被严格解码拒了：%v", err)
	}
}

func fixtureItem(id string, domains ...string) Item {
	return Item{ID: id, Name: id, Domains: domains}
}

// TestShapeDriftIsRejected 给校验层注入七种形状漂移：撞号、留洞、空条目、父域清单漂了（少算、顺序、多算）。
// 这七种都不该让服务起来——起来了就是"菜单少一条而没人报错"。
func TestShapeDriftIsRejected(t *testing.T) {
	one := fixtureItem("a-1", "a.com")
	two := fixtureItem("b-1", "b.com")

	cases := []struct {
		name  string
		menus []Menu
		want  string
	}{
		{"条目 id 撞号", []Menu{
			{ID: "a", Name: "A", Order: 1, Items: []Item{one}, Common: MenuCommon{[]string{"a.com"}}},
			{ID: "b", Name: "B", Order: 2, Items: []Item{one}, Common: MenuCommon{[]string{"a.com"}}},
		}, "条目 id 重复"},
		{"order 撞号", []Menu{
			{ID: "a", Name: "A", Order: 1, Items: []Item{one}, Common: MenuCommon{[]string{"a.com"}}},
			{ID: "b", Name: "B", Order: 1, Items: []Item{two}, Common: MenuCommon{[]string{"b.com"}}},
		}, "order 撞号"},
		{"order 留洞", []Menu{
			{ID: "a", Name: "A", Order: 1, Items: []Item{one}, Common: MenuCommon{[]string{"a.com"}}},
			{ID: "b", Name: "B", Order: 3, Items: []Item{two}, Common: MenuCommon{[]string{"b.com"}}},
		}, "断了第 2 位"},
		{"菜单名是空的", []Menu{
			{ID: "a", Order: 1, Items: []Item{one}, Common: MenuCommon{[]string{"a.com"}}},
		}, "菜单名是空的"},
		{"块里一个条目也没有", []Menu{
			{ID: "a", Name: "A", Order: 1},
		}, "一条条目也没有"},
		{"条目一个域名也没有", []Menu{
			{ID: "a", Name: "A", Order: 1, Items: []Item{{ID: "a-1", Name: "空条目"}}, Common: MenuCommon{nil}},
		}, "一个域名也没有"},
		{"父域少算一个", []Menu{
			{ID: "a", Name: "A", Order: 1, Items: []Item{fixtureItem("a-1", "a.com", "b.com")},
				Common: MenuCommon{[]string{"a.com"}}},
		}, "对不上"},
		{"父域顺序漂了", []Menu{
			{ID: "a", Name: "A", Order: 1, Items: []Item{fixtureItem("a-1", "b.com", "a.com")},
				Common: MenuCommon{[]string{"b.com", "a.com"}}},
		}, "对不上"},
		{"父域多抄一条完整域名", []Menu{
			{ID: "a", Name: "A", Order: 1, Items: []Item{fixtureItem("a-1", "x.a.com")},
				Common: MenuCommon{[]string{"a.com", "x.a.com"}}},
		}, "对不上"},
		{"同一条目里域名写两次", []Menu{
			{ID: "a", Name: "A", Order: 1, Items: []Item{fixtureItem("a-1", "a.com", "a.com")},
				Common: MenuCommon{[]string{"a.com"}}},
		}, "写了两次"},
	}
	for _, tc := range cases {
		c := &Catalog{menus: tc.menus, byItem: map[string]itemRef{}}
		err := c.index()
		if err == nil {
			err = c.validate()
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s：期望报「%s」，实际是 %v", tc.name, tc.want, err)
		}
	}
}

// TestShapeDriftRejectedNothingGoodIsRejected 反向兜一手：一份完全合规的合成清单必须过。
// 上面那十条断言要是写得太宽，合规数据也会被拒，那这一层就成了"永远红"。
func TestShapeDriftRejectedNothingGoodIsRejected(t *testing.T) {
	c := &Catalog{menus: []Menu{
		{ID: "a", Name: "A", Order: 1, Items: []Item{fixtureItem("a-1", "a.com", "x.a.com")},
			Common: MenuCommon{[]string{"a.com"}}},
		{ID: "b", Name: "B", Order: 2, Items: []Item{fixtureItem("b-1", "自定义域名")},
			Common: MenuCommon{nil}},
	}, byItem: map[string]itemRef{}}
	if err := c.index(); err != nil {
		t.Fatalf("合规清单被拒：%v", err)
	}
	if err := c.validate(); err != nil {
		t.Fatalf("合规清单被拒：%v", err)
	}
	if got := c.UnsupportedDomains(); len(got) != 1 || got[0].Value != "自定义域名" {
		t.Errorf("不合格项没登记上：%v", got)
	}
	if got := c.Domains(); !reflect.DeepEqual(got, []string{"a.com", "x.a.com"}) {
		t.Errorf("域名清单不是去重排序的那两份：%v", got)
	}
}
