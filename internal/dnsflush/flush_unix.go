//go:build !windows

package dnsflush

import (
	"context"
	"os"
	"strings"
)

// nop 是 Linux／macOS 那份实现：什么都不做，且这不是偷懒。
// glibc 的 nss_files 每次查询都现读 hosts，改完立刻生效，没有需要清的缓存；
// systemd-resolved 那一层缓存的是 DNS 应答，而 hosts 走的是文件那一条路，缓存里没有它。
type nop struct{}

func newPlatform() Flusher { return nop{} }

func (nop) Flush(context.Context) error { return nil }

// resolvedStub 是 systemd-resolved 的本机存根地址。它把这一串写进 resolv.conf 的 nameserver 行，
// 所以"这台机器归不归它管"只看这一条就够，不需要跑任何命令、发任何包。
const resolvedStub = "127.0.0.53"

// resolvConf 单独开一个变量：用例要喂一份假文件，而真读的那一条路只在这一处。
var resolvConf = "/etc/resolv.conf"

func platformServiceName() string { return serviceNameFrom(resolvConf) }

// serviceNameFrom 只读一个文件，一个网络包也不发（三铁律第 2 条：单测不许碰真网络）。
// 认不出来回空串——那是"不知道"，界面上显示"—"，不是"没有 DNS 服务"。
func serviceNameFrom(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		// 注释行（# ...）里也可能出现这个地址，所以先认出 nameserver 这一句再看地址。
		if !strings.HasPrefix(line, "nameserver") {
			continue
		}
		if strings.Contains(line, resolvedStub) {
			return "systemd-resolved"
		}
		// 第一行 nameserver 说了算：解析器就是按这一行的顺序问的，后面的备胎不改变"谁在管"。
		return ""
	}
	return ""
}
