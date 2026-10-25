// Package dnsflush 只管一件事：改完 hosts 之后，这台机器要不要"刷一下缓存"。
//
// 干什么用：给上层一个 Flusher，让它写完 hosts 之后叫一次，两个平台各一份实现。
// 什么情况下会错、出错时人看得见什么：Windows 上不刷 Dnscache，界面显示"已生效"而浏览器还是旧地址，
// 用户只会以为工具没作用；Linux 上去刷一次，就是白多一个外部命令调用、白多一处权限面。
// 怎么回来：接口只有一个方法，平台差异只出现在这两个按 build tag 分开的文件里，调用点一条分叉都没有。
package dnsflush

import "context"

// Flusher 是唯一的对外形状。故意只有一个方法——多一个方法就要多一个平台分叉。
type Flusher interface {
	// Flush 让系统重新去读 hosts。返回错误只有一种含义：这次没刷成，界面要如实标"地址改了但本机可能还在用旧的"。
	Flush(ctx context.Context) error
}

// New 给出本平台那份实现。运行时的 if 一条都不写，分叉全交给构建标签。
func New() Flusher {
	return newPlatform()
}

// ServiceName 说出这台机器此刻谁在管本机解析。
//
// 干什么用：界面「关于我们」那一格要写得出 DNS 服务名。C5 定的口径是"这一行由服务回，界面不猜"，
// 而此前契约里根本没有它的家——界面只能继续显示一份写死的假值。
// 什么情况下会错、出错时人看得见什么：认不出来就回空串，界面显示"—"。
// 回一个编好的名字才是错：用户按那一行判断"我这台机器归谁管"，而那句话此刻是假的。
// 怎么回来：每个平台文件各自补自己的判据（flush_unix.go 读 resolv.conf，flush_windows.go 用它自己刷的那个服务名），
// 调用点仍然一条分叉也没有——Flusher 那"只有一个方法"的规矩不受影响，因为这是包级函数，不是接口上的第二个方法。
func ServiceName() string {
	return platformServiceName()
}
