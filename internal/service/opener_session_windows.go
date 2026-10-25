//go:build windows

package service

import (
	"os/exec"
)

// Windows 侧的"打开在桌面"：服务以管理员在交互会话里跑，桌面就是自己的,
// 直接 cmd /c start（第一个空参数是窗口标题占位）,不需要借谁的会话。
func openInDesktop(path string) error {
	return exec.Command("cmd", "/c", "start", "", path).Start()
}
