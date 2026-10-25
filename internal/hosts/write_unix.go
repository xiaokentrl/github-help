//go:build !windows

package hosts

import (
	"os"
	"syscall"

	"github.com/hlep/github-accel/internal/errs"
)

// statFor 记下"改之前这个文件是什么权限、归谁"。
//
// uid/gid 取不到时给 -1，含义是"没有原属主可补"——文件本来不存在（用例指向一个还没建的临时路径）时就这样。
// 文件不存在时权限取 0644：hosts 必须人人可读，否则写完这台机器谁都解析不了域名，
// 那比"这一次没改成"严重得多，所以这一条不跟 umask 走。
func statFor(path string) (os.FileMode, int, int) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0o644, -1, -1
	}
	mode := fi.Mode().Perm()
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return mode, int(st.Uid), int(st.Gid)
	}
	return mode, -1, -1
}

// own rename 之后把权限与属主补回原来那一份。
//
// 为什么补：临时文件是我们这个进程建的，落盘就带上了"建它的人"和 umask 里的权限；
// 不补回去的话，用完这一次工具，/etc/hosts 的主人就从 root 变成了登录用户。
// 返回值 owned=false 表示属主没补上而权限补上了：当前身份不是 root 时 Chown 一定被拒（EPERM），
// 这是常态而不是意外——所以它只进回执的 Note，不触发回滚。
// 把这种常见情形判成失败，等于让非 root 用户永远用不了这个功能，而文件内容其实是对的。
// 只有权限（chmod）真的补不上才算写坏了：那时内容没错但外人读不到它，后果和没写一样。
func own(path string, mode os.FileMode, uid, gid int) (bool, error) {
	if err := os.Chmod(path, mode); err != nil {
		return false, errs.New(errs.CodeHostsWrite, map[string]any{"path": path},
			"新文件的权限补不回原来那一份").WithCause(err)
	}
	if uid < 0 || gid < 0 {
		return true, nil // 没有原属主可补：文件本来就是新建的
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return false, nil // 内容对了、权限对了，只有主人没换回去——如实带进 Note
	}
	return true, nil
}
