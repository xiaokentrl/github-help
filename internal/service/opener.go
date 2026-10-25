package service

import (
	"net/url"
	"os/exec"
	"runtime"

	"github.com/hlep/github-accel/internal/errs"
)

// Opener 是"打开外部页面"那一步的出口。为什么要做成依赖而不是 handlers 里现写 exec：
// 用例碰不到第二条路径——外网不可控是一回事，真把浏览器拉起来是另一回事；
// 测试传内存假件记下收到的 URL，生产走 cmd/service 那份默认实现（与 Inspector／Flusher 同一口径）。
type Opener func(u string) error

// defaultOpener 用系统的"打开这一个 URL"入口：Linux 走 xdg-open，Windows 走 cmd /c start。
// 为什么要 argv 直传、不许拼 shell 字符串：URL 里有 &、?、# 这些 shell 语义字符，
// 拼字符串等于把界面递来的任意内容当命令执行——argv 数组把内容钉死在"一个参数"上。
func defaultOpener(u string) error {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", "start", "", u).Start()
	}
	return exec.Command("xdg-open", u).Start()
}

// validateOpenURL 是 OpenURL 这一条唯一的门卫：scheme 只认 http／https。
// 为什么要拦在服务这一侧而不是界面：通道那头的界面（乃至借通道说话的任何本机进程）不该能
// 命令服务去执行 file://／自定 scheme——能写 hosts 的进程不该同时是个"帮人打开任意东西"的跳板。
func validateOpenURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return errs.New(errs.CodeIPCOpenURL, map[string]any{"url": raw},
			"只打得开 http(s) 链接，这一句不收：%s", raw)
	}
	return nil
}
