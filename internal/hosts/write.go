package hosts

import (
	"os"
	"path/filepath"

	"github.com/hlep/github-accel/internal/errs"
)

// dirOf 取宿主文件所在目录——临时文件必须建在同一个目录里。
func dirOf(path string) string { return filepath.Dir(path) }

// replace 用"同目录临时文件 + rename"把整个文件换掉。
//
// 干什么用：把合并好的内容落成磁盘上那一份。
// 为什么非走临时文件不可：直接截断原文件再写，中间有一小段"文件是空的或半截的"，
// 那正好被内核解析器读到就是一次解析失败；rename 是原子的，读到的一直是完整的一份。
// 同目录是关键——跨卷 rename 不原子，中途断电就停在两个半个文件上。
// 什么情况下会错：目录没有写权限、或（Windows 上）别的进程正开着这个文件；
// 出错时人看得见什么：GAC-HOSTS-003，消息里带上到底哪个路径。
// 怎么回来：临时文件用 defer 删掉，不会在目录里留下垃圾；原文件在 rename 失败时压根没被动过。
//
// 权限在这里一次设好（而不是等 rename 之后再补）：新文件的默认权限跟着进程 umask 走，
// 建出来是 0644 还是 0666 全看谁启动的服务——先把要的那一个显式盖上，rename 之后才不会有意外。
func (e *FileEngine) replace(content string, mode os.FileMode) error {
	tmp, err := os.CreateTemp(dirOf(e.path), ".hosts-gac-*")
	if err != nil {
		return errs.New(errs.CodeHostsWrite, map[string]any{"path": e.path}, "建不出临时文件").WithCause(err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return errs.New(errs.CodeHostsWrite, map[string]any{"path": e.path}, "写临时文件失败").WithCause(err)
	}
	// Sync 在 rename 之前：内容没落住盘就盖过去，断电后新文件是空的，而备份已经算"成功存过了"。
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return errs.New(errs.CodeHostsWrite, map[string]any{"path": e.path}, "内容没落住盘").WithCause(err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return errs.New(errs.CodeHostsWrite, map[string]any{"path": e.path}, "临时文件的权限设不上").WithCause(err)
	}
	if err := tmp.Close(); err != nil {
		return errs.New(errs.CodeHostsWrite, map[string]any{"path": e.path}, "关临时文件失败").WithCause(err)
	}
	if err := os.Rename(tmpName, e.path); err != nil {
		return errs.New(errs.CodeHostsWrite, map[string]any{"path": e.path},
			"hosts 写不进去（权限不足或文件被占用）").WithCause(err)
	}
	return nil
}
