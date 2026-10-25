package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// readGoFile 现读本包里的源码。
// 为什么要读源码而不是只调函数：有几条口径只在写法上成立（真壳用的节拍、界面文件不许嵌第二份），
// 光跑用例读不到那一处，改了写法也不会红——这类"只有源码能证明"的账就用这几个用例盯。
func readGoFile(t *testing.T, name string) string {
	t.Helper()
	buf, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", name, err)
	}
	return string(buf)
}

func TestResolveDistUsesTheFlagWhenGiven(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, distIndex), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveDist(dir)
	if err != nil {
		t.Fatalf("给了 -dist 还报错: %v", err)
	}
	if got != dir {
		t.Fatalf("该用指的那一处: %s ≠ %s", got, dir)
	}
}

// -dist 指到一个空目录时必须红。
// 怎么会错：这里放行就变成"窗口开了、里面一片白"，跟界面脚本挂了长得一模一样，人只会以为工具没作用。
func TestResolveDistRefusesAFlagPointedAtNothing(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveDist(dir)
	if err == nil {
		t.Fatal("指到没有构建产物的目录却当可以开工——界面会开成一个空窗口")
	}
	if !strings.Contains(err.Error(), dir) || !strings.Contains(err.Error(), distIndex) {
		t.Fatalf("报错要写清在哪儿找、缺哪一样: %v", err)
	}
	if !strings.Contains(err.Error(), "npm run build") {
		t.Fatalf("报错要给出下一步怎么做: %v", err)
	}
}

// 三条都没命中时，要把找过的地方一次列全。
// 为什么要列全：开发态最常见的是"忘了跑构建"和"构建在另一个目录"，只说一句"找不到"两种都查不动。
func TestResolveDistListsEveryPlaceItTried(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := resolveDist(""); err == nil {
		t.Fatal("空目录里什么构建产物都没有，却报了成功")
	} else if n := strings.Count(err.Error(), "/"); n < 3 {
		t.Fatalf("报错里要列全找过的三处，实际: %v", err)
	} else if !strings.Contains(err.Error(), filepath.Join("web", "dist")) {
		t.Fatalf("发布态那一处（可执行文件旁的 web/dist）没列出来: %v", err)
	}
}

// 开发态那一处要真能命中：仓库根下面有 frontend/dist 就该用它。
// 为什么要连这条也验：go run 把可执行文件放临时目录，只有当前目录这一条能接住"在仓库根直接跑"，
// 这一条漂了，日常跑法就变成"每次都要手动加 -dist"。
func TestRepoLayoutStillResolvesWithoutTheFlag(t *testing.T) {
	repo := t.TempDir()
	dist := filepath.Join(repo, "frontend", "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, distIndex), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	got, err := resolveDist("")
	if err != nil {
		t.Fatalf("仓库根那种摆法认不出来: %v", err)
	}
	if got != dist {
		t.Fatalf("该认 frontend/dist 那一份: %s ≠ %s", got, dist)
	}
}

// 界面文件只许有一份：cmd/gui 里不许再嵌第二份，也不许留一个自己的 HTML。
// 干什么用：正式界面只有 frontend/ 那一棵树，壳从磁盘读构建产物。
// 怎么会错：一旦这里再嵌一份，改了 frontend/ 还得记着重编 Go，两处迟早各走各的（上一版壳就是这么岔开的）。
func TestNoSecondCopyOfTheInterfaceInShell(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读不动本目录: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".html") {
			t.Fatalf("壳里不许留自己的页面（那会变成第二份界面）: %s", e.Name())
		}
		if e.IsDir() {
			t.Fatalf("壳里不该再有子目录放界面文件: %s（构建产物在 frontend/dist，运行时从那儿读）", e.Name())
		}
	}
	src := readGoFile(t, "main.go") + readGoFile(t, "assets.go") + readGoFile(t, "bridge.go")
	if strings.Contains(src, "go:embed") {
		t.Fatal("壳里出现了 //go:embed——界面文件不许嵌进二进制")
	}
}

// 桥上那三个名字的家在 internal/contract（Wire*），壳里一个字面量也不许写。
// 干什么用：这一条与 contract_test.go 里 TestWireNamesAreTheFrozenThree 是一对——那边钉住那一家只有三条线，
// 这边钉住壳没有另立一份。
// 怎么会错：壳里多写一条字面量＝同一句话有两个家，改名时两边各改一半，
// 界面上每一次点击都变成"没人接的一句话"，而且一个字也不报。
// 怎么回来：把多出来的那一处换成 contract.Wire* 那三个常量；想加第四条线先回契约改清单。
func TestBridgeNamesAreTheOnlyThreeTheShellSpeaks(t *testing.T) {
	src := readGoFile(t, "main.go") + readGoFile(t, "assets.go") + readGoFile(t, "bridge.go")
	if n := len(regexp.MustCompile(`"gac:`).FindAllString(src, -1)); n != 0 {
		t.Fatalf("壳里写出了 %d 处 gac: 字面量：那三个名字只有一个家（internal/contract 的 Wire*）", n)
	}
	for _, name := range []string{"contract.WireInvoke", "contract.WireReply", "contract.WireState"} {
		if !strings.Contains(src, name) {
			t.Fatalf("壳里没用到 %s：这一条线在壳这一头就没接上", name)
		}
	}
}

// 从磁盘读到的那一份，要真能通过壳的资产处理器取回页面。
// 干什么用：resolveDist 只回答"目录在哪儿"，这一条验的是"指对了之后拿不拿得出 index.html"——
// 中间那一步（os.DirFS + 壳的处理器）一旦接错，症状同样是"窗口开出来是白的"。
// 声明清楚这一条能证到哪儿：它证的是处理器这一层，不证 webview 真把页面画出来了，那一档要真宿主走查。
func TestDistServesThePageThroughTheShellHandler(t *testing.T) {
	dir := t.TempDir()
	body := "<!doctype html><html><body><div id=\"gac-root\">界面在这一份构建里</div></body></html>"
	if err := os.WriteFile(filepath.Join(dir, distIndex), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(application.AssetFileServerFS(os.DirFS(dir)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("壳的处理器答不上来: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("取 index.html 回来的是 %d，不是 200", resp.StatusCode)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读不回内容: %v", err)
	}
	if !strings.Contains(string(got), "gac-root") {
		t.Fatalf("回来的不是那一份页面: %s", got)
	}
}
