package service

import (
	"context"
	"os"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
)

// perms.go 实现 CheckPermissions 那一句问话：这一份文件此刻写不写得动、刷 DNS 那一步做不做得了。
//
// 干什么用：界面上那句"当前身份写不动 hosts"要在用户点应用之前就说出来，而不是等他点完再报 003。
//
// 什么情况下会错、出错时人看得见什么：
// ① 拿"能不能读"当"能不能写" → 界面说没问题，点下去才失败。所以这里真开一次写句柄；
// ② 开写句柄时忘了 O_APPEND → 会把用户的 hosts 直接截成空文件。
//
//	这是"你不点，我一个字不碰"那条铁律最坏的破法，所以下面那一行标志位一个都不许多给也不许少给；
//
// ③ 刷 DNS 在 Windows 上要动 Dnscache，没权限就刷不动（GAC-PERM-002）；Linux 那份是空实现，
//
//	永远刷得动（glibc 每次现读，本来就没有要清的缓存）。这不是猜的形状，是 dnsflush 那两份文件写的口径。
//
// 怎么回来：这两问都只回码与参数〔已定：D6 乙〕，整句人话在前端语言文件那一个家。

// checkPerms 是 CheckPermissions 的实现。它一次也不改文件内容。
func (s *Service) checkPerms() contract.PermReply {
	reply := contract.PermReply{}
	ctx := context.Background()
	if err := writeProbe(ctx, s.cfg.HostsPath); err != nil {
		reply.Detail = err.Error() // 带码前缀：GAC-PERM-001 那一句
	} else {
		reply.CanWrite = true
	}
	if err := s.cfg.Flusher.Flush(ctx); err != nil {
		// 刷不动这件事比写不动次要，但同样不许吞：它的症状正是"界面说已生效、浏览器还是旧地址"。
		reply.Detail = err.Error()
	} else {
		reply.DnsFlushed = true
	}
	return reply
}

// writeProbe 只问一句"这个文件开得动写句柄吗"。
//
// O_WRONLY|O_APPEND 这两个标志一个都不许换：只给 O_WRONLY（或给了 O_TRUNC）等于把用户的文件截空，
// 那是"一个字不碰"这条铁律最坏的破法。真写内容走的是 hosts.Engine 那五步
// （备份→写临时→rename→补属主→重读校验），这一个探针只负责"能不能碰"，不负责"碰成什么样"。
func writeProbe(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return errs.New(errs.CodeTaskPaused, map[string]any{"path": path}, "这一问在动手前就被叫停了").WithCause(ctx.Err())
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件还不存在（用例指向一个没建的临时路径）：写下去就会建出来，这不算"写不动"。
			// 真机上 /etc/hosts 不存在是怪事，但那条路归引擎管，不在这一问里编新码。
			return nil
		}
		return errs.New(errs.CodePermNoWrite, map[string]any{"path": path, "cause": err.Error()},
			"当前身份写不动 %s", path).WithCause(err)
	}
	defer f.Close()
	return nil
}
