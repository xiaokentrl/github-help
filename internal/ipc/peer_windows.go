//go:build windows

package ipc

import (
	"fmt"
	"net"

	"github.com/hlep/github-accel/internal/errs"
)

// Endpoint 是 Windows 侧那根命名管道的名字。
const pipeName = `\\.\pipe\github-accel.service`

func Endpoint() (string, error) { return pipeName, nil }

// Listen 在 Windows 上要建带 ACL 的命名管道、并取来连进程 ID 核路径，
// 这一整套属于 M1（写入落地）的范围：M0 只在 Linux 上打通两个程序。
// 这里如实报错，不摆一个"看着能用其实谁都能连"的假实现。
func Listen(string) (net.Listener, error) {
	return nil, fmt.Errorf("%s: Windows 命名管道尚未实现，本期请在 Linux 上跑 M0", errs.CodeIPCPlatform)
}

// DialEndpoints 界面侧的候选清单；Windows 只有一根命名管道，名单一条。
// 管道本身属 M1（Listen/CheckPeer 仍如实报错），这里先把形状摆齐，别让壳在 Windows 编不过。
func DialEndpoints() ([]string, error) { return []string{pipeName}, nil }

func CheckPeer(net.Conn) error {
	return fmt.Errorf("%s: Windows 命名管道尚未实现，本期请在 Linux 上跑 M0", errs.CodeIPCPlatform)
}
