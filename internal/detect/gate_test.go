package detect

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 这一支文件放本包自己的三条门禁。
//
// 为什么值得单开：它们查的不是"某支候选排对没排对"，而是"这个包还立不立得住第 8 节那两条规矩"——
// 一条是「排序与阻断判定是纯函数」（第 8.2 节），一条是「上限一个都不许住在这里」（G11 的那一侧）。
//
// 一条口径贯穿到底：比对一律"两侧各自解析出标识符，再做集合等值"，不许 strings.Contains——改名不报红。

// G-纯函数那一半：rankips.go 与 blocked.go 只许引排序与错误码，多一根管子就红。
//
// 为什么会错：一旦这里能读时间、能连网络，"同一批读数每次选出同一个地址"就不成立了，
// M2 那句「排名不再乱跳」和用例里的双轮等值当场作废——而且是在真机器上作废，本地永远绿。
func TestGateRankingFilesArePure(t *testing.T) {
	for _, name := range []string{"rankips.go", "blocked.go"} {
		var extra []string
		for _, p := range importedPackages(t, name) {
			if p != "sort" && !strings.HasSuffix(p, "/errs") {
				extra = append(extra, p)
			}
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			t.Errorf("%s 多引了 %v：排序与「换 IP 有没有用」这两条判据必须是纯函数，"+
				"输入只能是一批读数（第 8.2 节）", name, extra)
		}

		var hits []string
		for call := range calledQualifiedNames(t, name) {
			if forbidden[call] {
				hits = append(hits, call)
			}
		}
		sort.Strings(hits)
		if len(hits) > 0 {
			t.Errorf("%s 里出现了外部输入调用 %v：同一批读数会选出不一样的主用", name, hits)
		}
	}
}

var forbidden = map[string]bool{
	"time.Now": true, "time.Since": true, "time.Sleep": true, "time.After": true, "time.Tick": true,
	"rand.Int": true, "rand.Intn": true, "rand.Shuffle": true, "rand.Float64": true, "rand.New": true,
	"math/rand.Int": true, "math/rand.Intn": true, "math/rand.Shuffle": true,
	"os.Getenv": true, "os.ReadFile": true, "os.Open": true, "os.Create": true, "syscall.Getpid": true,
	"net.Dial": true, "net.Listen": true, "http.Get": true, "http.NewRequestWithContext": true,
}

// G-上限那一半（第 8 节第 ⑤ 条）：本包非测试文件里不许出现自己的数字。
//
// 超时几秒、重复几次、同时探几支、正文读多少字节、端口几号——这些唯一的家在 resolve/budget.go，
// 由 service 读出来经 Limits 注入。这里写下一个数，就等于同一批候选在不同轮里被两把尺子量过，
// 排名看着像随机，而界面上一个字都不提示。
//
// 射程要说清：它扫的是非测试文件里所有整型与浮点字面量。0 与 1 放行——那是循环起点、
// "并发给 0 就当 1 处理"这类结构性判断，不是"谁都能来抄一份的上限"。
func TestGateDetectTakesNoOwnLimits(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读不了包目录：%v", err)
	}
	allowed := map[string]bool{"0": true, "1": true}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, hit := range numericLiterals(t, name) {
			if allowed[hit.value] {
				continue
			}
			t.Errorf("%s：%s 处有数字字面量 %s。上限只能由 Limits 从外面进来（唯一的家是 resolve/budget.go），"+
				"否则同一批候选会被两把尺子量过", name, hit.pos, hit.value)
		}
	}
}

// 形状表的对手方：doc.go 声明的每一种形状，非测试代码里得真有人造出来。
//
// 为什么会错：加一档形状忘了在 probe.go 认（或认了却归不进去），这张表与 blocked.go 会照旧绿，
// 而界面上那批候选全被记成 other——"疑似被阻断"永远判不出来，队列就一遍一遍重试下去。
func TestGateEveryShapeIsProduced(t *testing.T) {
	declared := shapeConstNames(t, "doc.go")
	if len(declared) == 0 {
		t.Fatal("doc.go 里没有形状常量，这条门禁没有对手方")
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读不了包目录：%v", err)
	}
	used := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "doc.go" {
			continue
		}
		for ident := range referencedNames(t, name, declared) {
			used[ident] = true
		}
	}

	var dead []string
	for s := range declared {
		if !used[s] {
			dead = append(dead, s)
		}
	}
	sort.Strings(dead)
	if len(dead) > 0 {
		t.Errorf("形状 %v 在本包声明了却没人造出来：探测读数到不了这一档，"+
			"要么补 shapeOf 的归档，要么把这档从 doc.go 删掉", dead)
	}
}

type literal struct {
	value string
	pos   string
}

func importedPackages(t *testing.T, name string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", name, err)
	}
	var out []string
	for _, imp := range file.Imports {
		v, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatalf("import %s 读不懂：%v", imp.Path.Value, err)
		}
		out = append(out, v)
	}
	return out
}

// calledQualifiedNames 解析出一个文件里所有 pkg.Func 形状的被调用标识符集合。
func calledQualifiedNames(t *testing.T, name string) map[string]bool {
	t.Helper()
	_, file := parse(t, name)
	out := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				out[id.Name+"."+sel.Sel.Name] = true
			}
		}
		return true
	})
	return out
}

// numericLiterals 交回一个文件里所有整型与浮点字面量（含函数体里的那些）。
func numericLiterals(t *testing.T, name string) []literal {
	t.Helper()
	fset, file := parse(t, name)
	var out []literal
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok {
			return true
		}
		if lit.Kind == token.INT || lit.Kind == token.FLOAT {
			out = append(out, literal{value: lit.Value, pos: ":" + strconv.Itoa(fset.Position(lit.Pos()).Line)})
		}
		return true
	})
	return out
}

// shapeConstNames 解析出 doc.go 里那些"类型是 Shape 的常量"的标识符名。
// 比的是标识符不是字符串值：改名要两侧一起改，值写错了编译期就拦不住的那一类才需要这条门禁。
func shapeConstNames(t *testing.T, name string) map[string]bool {
	t.Helper()
	_, file := parse(t, name)
	out := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			typ, ok := vs.Type.(*ast.Ident)
			if !ok || typ.Name != "Shape" {
				continue
			}
			for _, n := range vs.Names {
				out[n.Name] = true
			}
		}
	}
	return out
}

// referencedNames 交回一个文件里出现过的、属于 want 那张名单的标识符。
func referencedNames(t *testing.T, name string, want map[string]bool) map[string]bool {
	t.Helper()
	_, file := parse(t, name)
	out := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if want[id.Name] {
			out[id.Name] = true
		}
		return true
	})
	return out
}

func parse(t *testing.T, name string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", name, err)
	}
	return fset, file
}
