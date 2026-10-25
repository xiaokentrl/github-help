package hosts

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultPath 给出这台机器的 hosts 文件在哪儿〔已定：C5——路径由这儿给，界面永远不写死〕。
//
// 干什么用：服务启动时调它一次，把结果交给 NewFileEngine；用例不调它，用例自己给临时路径。
// 什么情况下会错：Windows 上 SystemRoot 这个环境变量没设（几乎只出现在被剥了环境的进程里），
// 那时退到 C:\Windows——宁可给一个八成对的路径，也不要给一个空字符串让后面报"读不到 "。
// 怎么回来：这一份是只读的猜测，不写任何东西；路径确实不同时，服务侧可以把引擎换成显式路径那一个构造。
//
// 为什么是 runtime.GOOS 一个分支、不拆成 path.go + path_windows.go 两个文件：
// 两边各返回一个字符串，没有一处系统调用差别，为两个常量付两条构建分支不值（总纲第 0.1 节第 4 问）。
func DefaultPath() string {
	if runtime.GOOS == "windows" {
		sys := os.Getenv("SystemRoot")
		if sys == "" {
			sys = `C:\Windows`
		}
		return filepath.Join(sys, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}
