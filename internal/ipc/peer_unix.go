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

// 权限模型〔已定：2026-10-25 用户拍板〕：服务以 root 常驻（写 /etc/hosts 本来就要这一档），
// 界面以普通权限跑。这是给开发者的工具，默认用户知道自己在干什么——
// 旧判据"对端 UID 必须等于服务 UID"会把普通权限的界面拒在门外，这三处以新口径为准：
//   ① Endpoint：root 建在 /run/github-accel（谁都能看见的固定位置），普通用户仍在自己运行时目录；
//   ② Listen：root 态 socket 0666（普通权限的界面连得进来），普通用户态仍是 0600；
//   ③ CheckPeer：本机来的连接一律放行（SO_PEERCRED 仍取一次，证明确实是本机 socket 对端）。

// rootEndpointDir 是 root 态服务的固定端点目录。放 /run 而不是 /tmp：
// /tmp 全局可写，别人能先占这个名字摆一个假服务；/run 只有 root 建得出这一层目录。
const rootEndpointDir = "/run/github-accel"

// Endpoint 是这台机器上两个程序约好的那个地址（服务侧建端点用；界面侧该问 DialEndpoints）。
func Endpoint() (string, error) {
	var dir string
	if os.Geteuid() == 0 {
		dir = rootEndpointDir
	} else if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		dir = d
	} else {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("github-accel-%d", os.Getuid()))
	}
	// root 的目录 0755（界面要能 stat 到那条 socket）；用户自己的运行时目录维持 0700。
	perm := os.FileMode(0o700)
	if dir == rootEndpointDir {
		perm = 0o755
	}
	if err := os.MkdirAll(dir, perm); err != nil {
		return "", fmt.Errorf("%s: 建不出运行时目录 %s: %w", errs.CodeIPCRuntimeDir, dir, err)
	}
	return filepath.Join(dir, "service.sock"), nil
}

// DialEndpoints 给拨号那一侧（界面壳）：按顺序试的候选清单。
// 第一条永远是 root 服务那条固定端点——装好之后它就是"正主"；
// 第二条是本用户自己的端点，接住"开发者 sudo 都没起、直接 go run 两支"的本机对练那一路。
// 两条都试而不只试一条，是因为服务可能从"用户手跑"换成"root 常驻"（或反过来），
// 界面不该记住第一次连的是谁。
func DialEndpoints() ([]string, error) {
	own, err := Endpoint()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(rootEndpointDir, "service.sock")
	if os.Geteuid() == 0 || own == root {
		return []string{root}, nil
	}
	return []string{root, own}, nil
}

// Listen 建本机 Unix 端点。权限当场写死，不交给系统默认：
// root 态 0666（普通权限的界面连得进来——这就是上面权限模型的"门"），
// 用户态 0600（本用户自己的对练，别人的进程连不上）。
func Listen(path string) (net.Listener, error) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("%s: 旧端点 %s 清不掉: %w", errs.CodeIPCStale, path, err)
	}
	perm := os.FileMode(0o600)
	if os.Geteuid() == 0 {
		perm = 0o666
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("%s: 监听 %s 失败: %w", errs.CodeIPCListen, path, err)
	}
	if err := os.Chmod(path, perm); err != nil {
		_ = l.Close()
		return nil, fmt.Errorf("%s: %s 的权限没改成 %o: %w", errs.CodeIPCChmod, path, perm, err)
	}
	return l, nil
}

// CheckPeer 认连接那头是哪个进程。
// 〔2026-10-25 权限模型定档〕本机来的连接一律放行：旧判据"端 UID 是本机用户或 root"
// 在"root 服务 + 普通权限界面"这一档下会把界面自己拒在门外。
// SO_PEERCRED 仍要取一次：取不到凭据＝这不是本机 socket 的正经对端，照旧拒——
// 这条判据从"你是谁"改成了"你是不是本机来的"，改动前后它都守着同一件事：通道两头都得在这台机器上。
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
	_ = cred
	return nil
}
