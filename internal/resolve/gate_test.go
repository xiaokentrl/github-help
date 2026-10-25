package resolve

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 这一支文件放三条门禁（第 13 节 G8、G10 的纯函数那一半、G11 的两半）加一条接口形状断言。
//
// 为什么值得单开一个文件：这几条查的不是"某个方式对不对"，而是"这棵树的形状还立不立得住"——
// 加方式的人、改公共件的人、抄第二份上限的人各撞其中一条，报错要一眼指名到文件。
//
// 一条口径贯穿到底：比对一律"两侧各自解析出标识符，再做集合等值"，不许 strings.Contains——改名不报红。

// commonFiles 是四件公共件的文件名。G8 靠这张名单把它们排除，排除依据是文件名而不是"含不含"。
// 名单只许出现在测试里：它不是运行时的第二份清单，少一个公共件文件它会跟着红，那正是想要的。
var commonFiles = map[string]bool{
	"registry.go": true,
	"rank.go":     true,
	"health.go":   true,
	"budget.go":   true,
	"doc.go":      true,
}

// G8：包内方式文件集合 ⇄ 注册表 id 集合等值。
// 多一个文件没注册要红（它静静躺着，界面看不见）；注册表里多一个没文件的 id 也要红（第 6.1 节第 7 步取不到实现）。
func TestGateModeFilesEqualRegistryIDs(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读不了包目录：%v", err)
	}
	fromFiles := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if commonFiles[name] {
			continue
		}
		fromFiles[strings.TrimSuffix(name, ".go")] = true
	}

	fromRegistry := map[string]bool{}
	for _, s := range All() {
		fromRegistry[s.ID] = true
	}

	var onlyFile, onlyRegistry []string
	for id := range fromFiles {
		if !fromRegistry[id] {
			onlyFile = append(onlyFile, id)
		}
	}
	for id := range fromRegistry {
		if !fromFiles[id] {
			onlyRegistry = append(onlyRegistry, id)
		}
	}
	sort.Strings(onlyFile)
	sort.Strings(onlyRegistry)
	if len(onlyFile) > 0 {
		t.Errorf("有方式文件没注册（加了 .go 忘写 init()）：%s", strings.Join(onlyFile, "、"))
	}
	if len(onlyRegistry) > 0 {
		t.Errorf("注册表里有 id 却没有同名文件（id 拼错或文件被删）：%s", strings.Join(onlyRegistry, "、"))
	}
	if len(fromFiles) == 0 || len(fromRegistry) == 0 {
		t.Fatalf("注册表空了：包内 %d 个方式文件、注册 %d 条", len(fromFiles), len(fromRegistry))
	}

	// 每个 id 还得真取得到实现，而且实现自己报的名字与注册的一致——不然"改选备用"那一步会拿到 nil。
	for id := range fromRegistry {
		r, ok := Get(id)
		if !ok {
			t.Errorf("Get(%q) 取不到实现", id)
			continue
		}
		if got := r.Spec().ID; got != id {
			t.Errorf("方式 %s 自己报的名字是 %q，两边不一致", id, got)
		}
	}
}

// G10 的纯函数那一半：rank.go 里不许出现读时间、读随机数、碰磁盘的调用。
// 判据不是搜字符串，而是解析出所有 pkg.Func 形状的被调用标识符，再与一张禁用集合求交。
func TestGateRankIsPure(t *testing.T) {
	calls := calledQualifiedNames(t, "rank.go")

	forbidden := map[string]bool{
		"time.Now": true, "time.Since": true, "time.Sleep": true, "time.Tick": true, "time.After": true,
		"rand.Int": true, "rand.Intn": true, "rand.Shuffle": true, "rand.Float64": true,
		"rand.New": true, "math/rand.Int": true, "math/rand.Intn": true, "math/rand.Shuffle": true,
		"os.Getenv": true, "os.ReadFile": true, "os.Open": true, "os.Create": true, "syscall.Getpid": true,
	}
	var hits []string
	for name := range calls {
		if forbidden[name] {
			hits = append(hits, name)
		}
	}
	sort.Strings(hits)
	if len(hits) > 0 {
		t.Errorf("rank.go 里出现了外部输入调用 %s：同一批样本会选出不一样主用，"+
			"M2 那句「排名不再乱跳」和 G10 的双轮等值当场作废", strings.Join(hits, "、"))
	}

	// 反过来也钉一条：选优的输入必须由调用方喂进来，除了排序与错误码它不许伸手拿别的东西。
	var extra []string
	for _, p := range importedPackages(t, "rank.go") {
		if p != "sort" && p != "time" && !strings.HasSuffix(p, "/errs") {
			extra = append(extra, p)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Errorf("rank.go 多引了 %v", extra)
	}
}

// G11 前半：一轮取数的数字只许住在 budget.go。
//
// 射程要说清：它管的是声明处——const/var 的值表达式里的数字字面量。函数体里的一次性算术
// （切片下标、循环增量）不在射程内，那些不是"谁都能来抄一份的上限"。
// 方式自己的 Spec 里的 DefaultPriority 也不在射程内：那是自我介绍排老几，不是这一轮发多少次事。
func TestGateNumericLimitsOnlyInBudget(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读不了包目录：%v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "budget.go" {
			continue
		}
		for _, hit := range numericDecls(t, name) {
			t.Errorf("%s：上限数字不许写在这里，唯一的家是 budget.go（位置 %s，值 %s）",
				name, hit.pos, hit.value)
		}
	}
}

// G11 后半：方式 id 的字面量只许出现在它自己那个文件里。
//
// 为什么会错：只要有任何一份手写的方式清单存在（在 service 里数一遍、在菜单数据里列一遍），
//
//	第 5 节 R6 那句「加一种方式只加一个文件」就早晚被破，而破的时候界面一个字不变。
//
// 射程要说清：测试文件排除在外——用例为了造样本写个假 id 是常事，它不是运行时的第二份真源。
func TestGateModeIDLiteralsStayInOwnFile(t *testing.T) {
	ids := map[string]bool{}
	for _, s := range All() {
		ids[s.ID] = true
	}
	if len(ids) == 0 {
		t.Fatal("注册表是空的，这条门禁没有对手方")
	}

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("找不到仓库根：%v", err)
	}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "frontend" || d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		for _, lit := range stringLiterals(t, path) {
			if !ids[lit.value] {
				continue
			}
			if rel == filepath.Join("internal", "resolve", lit.value+".go") {
				continue // 方式自己那一个文件里写自己的 id，这是它唯一的家
			}
			t.Errorf("%s%s：%q 是方式 id，字面量不许出现在它自己那个文件之外（R6 的那道闸）",
				rel, lit.pos, lit.value)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("走查仓库失败：%v", err)
	}
}

// 接口形状：Resolver 必须只有 Spec 与 Resolve 两个方法。
// 多一个（比如 Score()）＝把「该不该用我」塞回方式自己手里，第 6.2 节那两把尺子立刻混成一把。
func TestGateResolverHasExactlyTwoMethods(t *testing.T) {
	typ := reflect.TypeOf((*Resolver)(nil)).Elem()
	var got []string
	for i := 0; i < typ.NumMethod(); i++ {
		got = append(got, typ.Method(i).Name)
	}
	want := []string{"Resolve", "Spec"}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolver 的方法集合 got %v, want %v——选优的权力不许多一处", got, want)
	}
}

type literal struct {
	value string
	pos   string
}

// calledQualifiedNames 解析出一个文件里所有 pkg.Func 形状的被调用标识符集合。
func calledQualifiedNames(t *testing.T, name string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, sel := range selectors(t, name) {
		if id, ok := sel.X.(*ast.Ident); ok {
			out[id.Name+"."+sel.Sel.Name] = true
		}
	}
	return out
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

// numericDecls 交回一个文件里"const/var 声明处的数字字面量"清单（Spec 复合字面量除外）。
func numericDecls(t *testing.T, name string) []literal {
	t.Helper()
	fset, file := parse(t, name)
	var out []literal
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, value := range vs.Values {
				if isSpecLiteral(value) {
					continue
				}
				ast.Inspect(value, func(n ast.Node) bool {
					lit, ok := n.(*ast.BasicLit)
					if !ok {
						return true
					}
					if lit.Kind == token.INT || lit.Kind == token.FLOAT {
						out = append(out, literal{value: lit.Value, pos: fset.Position(lit.Pos()).String()})
					}
					return true
				})
			}
		}
	}
	return out
}

// stringLiterals 交回一个文件里所有字符串字面量（含它的位置）。
func stringLiterals(t *testing.T, path string) []literal {
	t.Helper()
	fset, file := parse(t, path)
	var out []literal
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		out = append(out, literal{value: v, pos: ":" + strconv.Itoa(fset.Position(lit.Pos()).Line)})
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

func selectors(t *testing.T, name string) []*ast.SelectorExpr {
	t.Helper()
	_, file := parse(t, name)
	var out []*ast.SelectorExpr
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			out = append(out, sel)
		}
		return true
	})
	return out
}

// isSpecLiteral 判这个值表达式是不是 Spec 的复合字面量（var xxxSpec = Spec{...}）。
func isSpecLiteral(expr ast.Expr) bool {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return false
	}
	id, ok := lit.Type.(*ast.Ident)
	return ok && id.Name == "Spec"
}
