//go:build windows

package dnsflush

import (
	"context"
	"os/exec"

	"github.com/hlep/github-accel/internal/errs"
)

type win struct{}

func newPlatform() Flusher { return win{} }

// platformServiceName 给的就是上面那条命令刷的那个服务。
// 这一句不是猜的：Flush 唯一的动作就是 ipconfig /flushdns，而它刷的是 Dnscache 那份预加载。
func platformServiceName() string { return "Dnscache" }

// Flush 调系统那条刷新命令。Windows 的 Dnscache 会把解析结果预加载，
// 不刷的话"界面说成功、浏览器还是旧的"——所以这一步失败必须回给上层，不许吞掉。
// 本机没有 Windows 可跑（这一句是实情，判据要在 M5 出包那天在真机验），
// 因此这里只保证：命令找不着或返回非零都会落成 GAC-PERM-002，而不是静默成功。
func (win) Flush(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "ipconfig", "/flushdns")
	if err := cmd.Run(); err != nil {
		return errs.New(errs.CodePermFlush, nil, "刷新本机 DNS 缓存没成，浏览器可能还在用旧地址").WithCause(err)
	}
	return nil
}
