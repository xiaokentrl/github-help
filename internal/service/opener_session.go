//go:build !windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/hlep/github-accel/internal/errs"
)

// opener_session.go 解决"root 服务怎么把一个文件打开在用户桌面上"〔2026-10-26 用户拍:
// 打开Hosts 要立刻把这个文件打开在桌面〕。
//
// 难点：服务以 root 常驻在 systemd 里，没有 DISPLAY／WAYLAND_DISPLAY／DBUS_SESSION_BUS_ADDRESS，
// 直接 xdg-open 只会在服务的黑屋子里失败。借力打力的办法：来连服务的界面进程就活在用户桌面里——
// 从 /proc 里找到它，借三样东西：它的 uid/gid（打开的编辑器以用户身份跑，不提权）、
// 它的 environ（里面有整套会话环境）、它还活着这个事实（会话一定在）。
//
// 安全边界：被打开的命令固定是 xdg-open＋服务钉死的路径，不经过 shell；
// 以界面进程的用户身份运行——root 自己的权限一分钟也不落到编辑器头上。

// desktopSession 是借来的会话三件套。
type desktopSession struct {
	uid, gid uint32
	env      []string
}

// sessionFinder／sessionSpawn 是两个可换的接缝：前者找会话，后者真拉进程。
// 用例换掉它们，一个字节也不碰真 /proc、真桌面。
var (
	sessionFinder = findDesktopSession
	sessionSpawn  = spawnAsDesktopUser
)

// findDesktopSession 在 /proc 里找界面进程（可执行文件名以 github-accel-gui 结尾），
// 借它的 uid/gid 与整套会话环境。找不到（界面还没起）＝ ok:false。
func findDesktopSession() (desktopSession, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return desktopSession{}, false
	}
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue // 不是数字目录＝不是进程
		}
		exe, err := os.Readlink("/proc/" + e.Name() + "/exe")
		if err != nil || !strings.HasSuffix(exe, "github-accel-gui") {
			continue
		}
		fi, err := os.Stat("/proc/" + e.Name())
		if err != nil {
			continue
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/environ")
		if err != nil {
			continue
		}
		env := strings.Split(string(raw), "\x00")
		clean := env[:0]
		for _, kv := range env {
			if kv != "" {
				clean = append(clean, kv)
			}
		}
		return desktopSession{uid: st.Uid, gid: st.Gid, env: clean}, true
	}
	return desktopSession{}, false
}

// spawnAsDesktopUser 以会话主人的身份拉起 xdg-open。Start 成功＝交给桌面了；
// 它随后打开失败（比如没有默认编辑器）服务这边看不见，那一段归桌面会话自己。
func spawnAsDesktopUser(cmd string, args []string, uid, gid uint32, env []string) error {
	c := exec.Command(cmd, args...)
	c.Env = env
	c.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: gid}}
	return c.Start()
}

// openInDesktop 是「打开Hosts」的"打开"那一步的统一出口：以界面进程的身份，
// 在它的桌面上用系统默认编辑器打开这个文件。
func openInDesktop(path string) error {
	sess, ok := sessionFinder()
	if !ok {
		return errs.New(errs.CodeIPCOpenHosts, map[string]any{"path": path},
			"没找到界面进程的桌面会话——先开着界面再点")
	}
	if err := sessionSpawn("xdg-open", []string{path}, sess.uid, sess.gid, sess.env); err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}
