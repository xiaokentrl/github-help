//go:build windows

package hosts

import (
	"os"

	"github.com/hlep/github-accel/internal/errs"
)

// statFor Windows 一支。
//
// Windows 上没有 uid/gid 这一套，文件的"归谁、谁能读写"是 SecurityDescriptor（DACL），
// 标准库的 Mode() 只能给出勉强意义上的只读位，所以这里一律报"没有原属主可补"（-1）。
func statFor(path string) (os.FileMode, int, int) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0o644, -1, -1
	}
	return fi.Mode().Perm(), -1, -1
}

// own Windows 一支：只把只读位盖回去，DACL 补不回来并且如实说出来。
//
// 为什么返回 owned=false：rename 建出来的新文件的访问权限来自所在目录的默认继承，
// 不是原来那份 hosts 的 ACL。要保住 ACL 得走 ReplaceFileW，那是 M5 那一站要在真 Windows 上核的活〔待核〕，
// 本机没有 Windows，编得过不等于跑得对，所以这一支现在只做能确定的那半，剩下半交给 Note 里那句话。
// 出错时人看得见什么：写入照样成功，回执的 Note 会写"DACL 没保住"，界面上点得开看细节。
func own(path string, mode os.FileMode, uid, gid int) (bool, error) {
	if err := os.Chmod(path, mode); err != nil {
		return false, errs.New(errs.CodeHostsWrite, map[string]any{"path": path},
			"新文件的权限补不回原来那一份").WithCause(err)
	}
	return false, nil
}
