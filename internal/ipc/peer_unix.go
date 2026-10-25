//go:build !windows

package ipc

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/hlep/github-accel/internal/errs"
)

// Endpoint 是这台机器上两个程序约好的那个地址。
// 放 XDG_RUNTIME_DIR 而不是 /tmp：/tmp 全局可写，别人能先占这个名字摆一个假服务。
func Endpoint() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("github-accel-%d", os.Getuid()))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("%s: 建不出运行时目录 %s: %w", errs.CodeIPCRuntimeDir, dir, err)
	}
	return filepath.Join(dir, "service.sock"), nil
}

// Listen 建本机 Unix 端点，权限当场写死 0600。
// 不写就等于交给系统默认，而默认可能是 0666——那等于谁都能改 hosts。
func Listen(path string) (net.Listener, error) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("%s: 旧端点 %s 清不掉: %w", errs.CodeIPCStale, path, err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("%s: 监听 %s 失败: %w", errs.CodeIPCListen, path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = l.Close()
		return nil, fmt.Errorf("%s: %s 的权限没改成 0600: %w", errs.CodeIPCChmod, path, err)
	}
	return l, nil
}

// CheckPeer 认连接那头是哪个进程：本机任何进程都连得上本机端点，
// 所以"连上了"不等于"是自己人"，必须核对端 UID。
// 判据：端 UID 是本机用户，或者是 root（装服务、排障时 root 会来连）。
func CheckPeer(c net.Conn) error {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("%s: 对端不是 Unix 连接，认不出是谁", errs.CodeIPCPeer)
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return fmt.Errorf("%s: 取不到对端套接字: %w", errs.CodeIPCPeer, err)
	}
	var cred *syscall.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return fmt.Errorf("%s: 控制调用失败: %w", errs.CodeIPCPeer, err)
	}
	if credErr != nil {
		return fmt.Errorf("%s: 取不到对端凭据: %w", errs.CodeIPCPeer, credErr)
	}
	if int(cred.Uid) != os.Getuid() && cred.Uid != 0 {
		return fmt.Errorf("%s: 对端 UID %d 不是本机用户，拒了", errs.CodeIPCPeer, cred.Uid)
	}
	return nil
}
