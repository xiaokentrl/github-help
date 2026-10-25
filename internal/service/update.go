package service

import (
	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
)

// update.go 接住自动更新那三条问话。本期（M0～M4）不做这件事，但三条都必须如实回话：
//
// 干什么用：界面上"检查更新／下载更新"那两颗按钮要有一个能问的对象，问到的答案是"这一站还没做"。
// 什么情况下会错、出错时人看得见什么：最坏的做法是演一个"已是最新版本"回去——
//
//	用户会以为更新通道是通的，而它压根没接线；所以这两条一律回 GAC-UPDATE-501，界面显示"本期不提供"〔已定：不许演成功〕。
//
// 怎么回来：M5 接线那天改的只是这一个文件（外加把 GAC-UPDATE-501 那一条从"没接线"改成真判据），
// 方法名、参数键名、返回形状都不动。

// hCheckUpdate 属 M5：现在没人能替用户回答"有没有新版本"。
func (s *Service) hCheckUpdate(map[string]any) (any, error) {
	return nil, notWired("CheckUpdate")
}

// hDownloadUpdate 同上。它本该是第五个异步方法（回任务号），但通道没接线时回一个任务号更坏——
// 界面会开始等一条永远不会来的事件。所以这里直接回码，不造一个假的 taskID。
func (s *Service) hDownloadUpdate(map[string]any) (any, error) {
	return nil, notWired("DownloadUpdate")
}

// hGetVersion 是真的：这一个数不需要网络，也不需要 M5 那套东西。
//
// BuildDate 由打包那一步（-ldflags）填进来；没填就如实留空，界面显示"—"。
// 编一个日期进二进制，用户按它判断"我装的是哪一天的包"就完全是错的。
func (s *Service) hGetVersion(map[string]any) (any, error) {
	return contract.VersionReply{Current: s.cfg.Version, BuildDate: s.cfg.BuildDate}, nil
}

func notWired(method string) error {
	return errs.New(errs.CodeUpdateNotWired, map[string]any{"method": method, "milestone": "M5"},
		"%s 这一站还没做，本期不提供", method)
}
