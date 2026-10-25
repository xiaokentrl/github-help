package internal

// deps_test.go 是门禁 G6：后端目录结构规划第 8 节那八条 import 方向，一条一个断言。
//
// 干什么用：那八条写的是"哪一层准知道哪一层"。它们是纸面规矩时，谁都可以顺手多引一行——
// 编译过得去、跑起来也对，只是职责乱了，而这正是肉眼走查看不出来的那类漂移。
// 什么情况下会错、出错时人看得见什么：
// ① resolve 引了 detect（想直接拿"这个 IP 延迟多少"给方式打分）→ 两把尺子合成一把，
//
//	M2 那条"连续三轮排名不乱跳"当场作废，而且代码看起来还是对的；
//
// ② 引擎层引了 service／ipc → 每个引擎都得为通道写一套测试替身，用例开始测装配；
// ③ 界面壳引了 hosts／detect → 界面上那颗按钮就能绕开服务直接改文件，"唯一出口"这句破了；
// ④ menu 里出现 .go → "一个菜单一个目录"退化成"一个菜单一个包"，加菜单要改代码（破 R6 的邻居）。
//
// 怎么回来：红的那一条写明"该挪回哪儿"，不许反过来放宽断言——放宽等于把这条规矩作废。
//
// 全部判断都走 `go list -deps`（编译器的真实答案），没有一处用 strings.Contains 比名字；
// 文件级的检查用 go/parser 解析 import 再等值比，改名照样红。

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/hlep/github-accel"

// depsOf 列出一个包的全部依赖（含传递依赖，不含它自己）——这是编译器的答案，
// 不是我数 import 行的答案。
func depsOf(t *testing.T, pkg string) []string {
	t.Helper()
	self := path.Base(pkg)
	out := []string{}
	for _, line := range listLines(t, "-deps", pkg) {
		if !strings.HasPrefix(line, module+"/internal/") {
			continue
		}
		name := strings.TrimPrefix(line, module+"/internal/")
		if name == self {
			continue // `go list -deps` 把被查的那个包自己也列进来
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return dedupe(out)
}

// directImportsOf 列出这个包自己写下的 import（不含传递）。
// 第 2 条要的是这个：壳"只准认识通道与契约"说的是它能点谁的名，
// 而通道自己顺手引了 errs，不该算到壳头上。
func directImportsOf(t *testing.T, pkg string) []string {
	t.Helper()
	out := []string{}
	for _, line := range listLines(t, "-f", `{{join .Imports "\n"}}`, pkg) {
		if strings.HasPrefix(line, module+"/internal/") {
			out = append(out, strings.TrimPrefix(line, module+"/internal/"))
		}
	}
	sort.Strings(out)
	return dedupe(out)
}

// listLines 跑一条 go list 并把输出切成非空行。
func listLines(t *testing.T, args ...string) []string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	cmd.Dir = moduleRoot(t)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %v 没跑成：%v", args, err)
	}
	out := []string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// internalDeps 给出这个包依赖了本模块哪几个 internal 包（去重、排序，便于等值比）。
func internalDeps(t *testing.T, pkg string) []string { return depsOf(t, pkg) }

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("找不到模块根（go.mod 在哪儿）")
		}
		dir = parent
	}
}

func dedupe(in []string) []string {
	var out []string
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// 第 1 条：contract 只准 import 标准库。它是名字真源，一旦能引 errs，字段名与错误码就缠在一起。
func TestContractImportsNothingInternal(t *testing.T) {
	got := internalDeps(t, "./internal/contract")
	if len(got) != 0 {
		t.Fatalf("contract 引了本模块的包 %v：它是唯一名字真源，不许有任何依赖", got)
	}
}

// 第 2 条：界面壳只准认识通道与契约。它若能引 hosts，那颗按钮就能绕开服务直接改文件。
func TestGuiShellTouchesOnlyChannelAndContract(t *testing.T) {
	// 先按传递依赖挡一遍：这些名字连"绕一圈再来"都不许出现。
	banned := []string{"hosts", "detect", "resolve", "store", "settings", "service", "menu"}
	got := internalDeps(t, "./cmd/gui")
	for _, b := range banned {
		if contains(got, b) {
			t.Errorf("cmd/gui 依赖里出现了 internal/%s：%v", b, got)
		}
	}
	// 再按它自己点的名数一遍：名单之外就是有人在壳里办事。
	// （通道自己顺手引了 errs，那一笔算不到壳头上，所以这里看的是直接 import。）
	//
	// 2026-10-23 联调轮把 errs 加进这一份名单，理由写在这儿：
	// 壳拦下来的两种情况（界面递来半句看不懂的话、界面上喊了一个契约里没有的方法名）根本没能到达服务，
	// 回执上的那一句人话只能由壳按码发（已定：D6 乙——服务与壳都只回码，整句人话在前端语言文件里）。
	// errs 里只有码表，没有一个字节的行为，所以"壳不许办事"这条线没有因此松动：
	// 上面那份 banned 名单才是这条规矩的执行者，它一个字没放宽。
	want := []string{"contract", "errs", "ipc"}
	if direct := directImportsOf(t, "./cmd/gui"); !sameSet(direct, want) {
		t.Errorf("cmd/gui 直接 import 了 %v，要的是 %v（只准通道 + 契约 + 码表）", direct, want)
	}
}

// 第 3 条：引擎层不知道"谁调我"，只认参数。
func TestEnginesDoNotKnowTheChannel(t *testing.T) {
	for _, pkg := range []string{"hosts", "detect", "resolve", "dnsflush"} {
		got := internalDeps(t, "./internal/"+pkg)
		for _, banned := range []string{"ipc", "service"} {
			if contains(got, banned) {
				t.Errorf("internal/%s 依赖了 internal/%s：引擎只认参数，不许认识通道或服务", pkg, banned)
			}
		}
	}
}

// 第 4 条：menu 只是数据 + 校验，除了 errs 谁也不引。
// 上一版给它开过一条"仅类型引 resolve"的口子，M1 第一步实测那条口子没有对手方，已收回。
// 注意 errs 是"准许"不是"必须"：今天它一条校验错误也没往外报码，所以依赖清单是空的，
// 这一条要挡的是第三个名字，不是"少了 errs"。
func TestMenuDependsOnErrsOnly(t *testing.T) {
	got := internalDeps(t, "./internal/menu")
	for _, name := range got {
		if name != "errs" {
			t.Fatalf("menu 的 internal 依赖里出现了 %v（完整清单 %v）：它只准认识 errs 这一样", name, got)
		}
	}
}

// 第 5 条：internal/menu 下面那些菜单目录里一个 .go 也不许有。
// 有＝"一菜单一目录"退化成"一菜单一包"，加一块菜单要改代码，破的是已定的那条自动加载口径。
func TestMenuDataDirsContainNoGoFiles(t *testing.T) {
	root := filepath.Join(moduleRoot(t), "internal", "menu")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", root, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var found []string
		err := filepath.WalkDir(filepath.Join(root, e.Name()), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(p, ".go") {
				found = append(found, p)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("扫 %s 失败：%v", e.Name(), err)
		}
		if len(found) > 0 {
			t.Errorf("菜单目录 %s 里出现了 .go 文件：%v——那一层只能是数据", e.Name(), found)
		}
	}
}

// 第 6 条：服务侧不许 import Wails（整条依赖链，含传递依赖）。
// 破了这句话，服务就得带着 GTK 那一大堆运行时常驻，而它本来只需要一个 socket。
func TestServiceSideNeverLinksWails(t *testing.T) {
	for _, pkg := range []string{"./cmd/service", "./internal/service"} {
		cmd := exec.Command("go", "list", "-deps", pkg)
		cmd.Dir = moduleRoot(t)
		raw, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list -deps %s 没跑成：%v", pkg, err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "github.com/wailsapp/") {
				t.Errorf("%s 的依赖链里出现了 Wails：%s", pkg, strings.TrimSpace(line))
			}
		}
	}
}

// 第 7 条：resolve 与 detect 互不认识——两把尺子各管一层。
// 合流只准在 internal/service/detectrun.go 那一个文件里。
func TestResolveAndDetectAreIndependent(t *testing.T) {
	if contains(internalDeps(t, "./internal/resolve"), "detect") {
		t.Fatal("resolve 引了 detect：方式层直接拿 IP 延迟打分，两把尺子合成一把")
	}
	if contains(internalDeps(t, "./internal/detect"), "resolve") {
		t.Fatal("detect 引了 resolve：IP 层反过来给方式排名，同样是合成一把")
	}

	both := filesImportingBoth(t, "internal/service", "resolve", "detect")
	// 允许同时点这两个包名的文件只有三个：合流那一个（detectrun.go）与两个装配文件
	// （doc.go 声明注入的出口形状、service.go 现造它们的默认实现）。
	// 规划第 8 节第 7 条原文写的是"detectrun.go 之外不许同时出现两个包名"——
	// 那一版落地时 service 包还没建，实测按原文一红就红在装配文件上（它们必须同时写出
	// detect.Inspector 与 []resolve.Spec 才接得上），所以这里按实际职责放行这三份，
	// 并把这一处偏离登记在技术方案与后端规划那一轮。
	want := []string{"detectrun.go", "doc.go", "service.go"}
	if !sameSet(both, want) {
		t.Fatalf("同时 import resolve 与 detect 的文件是 %v，要的是 %v——合流只准在 detectrun.go，装配那两份另有说明", both, want)
	}
}

// 第 8 条：resolve 不许伸手去拿存储。样本由 service 读出来喂给 Rank，
// 否则"换一种存储"要改六个方式文件，样本口径也会散在两处。
func TestResolveDoesNotReachForStorage(t *testing.T) {
	got := internalDeps(t, "./internal/resolve")
	if contains(got, "store") {
		t.Fatalf("resolve 引了 store：%v——数据库不参与判定这句破了", got)
	}
	// 它连 service 也不许认识（第 3 条同一判据的邻居）：Rank 的入参必须是纯数据。
	if contains(got, "service") {
		t.Fatalf("resolve 引了 service：%v", got)
	}
}

// filesImportingBoth 解析每个 .go 文件的 import 块，交出同时点了这两个包名的文件名。
// 用 go/parser 而不是 grep：grep 会把注释里的一句"resolve 与 detect"算进来，
// 而把真 import 改成别名又躲得过去——那正是"改名不报红"的形状。
func filesImportingBoth(t *testing.T, dir, a, b string) []string {
	t.Helper()
	root := filepath.Join(moduleRoot(t), filepath.FromSlash(dir))
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", dir, err)
	}
	fset := token.NewFileSet()
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(root, e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", e.Name(), err)
		}
		var hasA, hasB bool
		for _, imp := range f.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s 的 import 路径读不出：%v", e.Name(), err)
			}
			switch importPath {
			case module + "/internal/" + a:
				hasA = true
			case module + "/internal/" + b:
				hasB = true
			}
		}
		if hasA && hasB {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
