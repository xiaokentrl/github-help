//go:build windows

package ipc

import (
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/hlep/github-accel/internal/errs"
)

// Endpoint 是 Windows 侧那根命名管道的名字。
const pipeName = `\\.\pipe\github-accel.service`

func Endpoint() (string, error) { return pipeName, nil }

// DialEndpoints 界面侧的候选清单；Windows 只有一根命名管道，名单一条。
func DialEndpoints() ([]string, error) { return []string{pipeName}, nil }

// pipeSDDL 是管道的门。取舍与 Linux 一侧（2026-10-25 权限模型）对齐：
//   - SY（SYSTEM）、BA（管理员）：服务自己以管理员跑；
//   - IU（交互式登录用户）：普通权限的界面要连得进来——开发者工具的既定口径。
//
// 与 Linux 的差别如实交底：Linux 那侧 CheckPeer 还会核一次"对端是本机 socket"，
// Windows 这侧的进程级核验（GetNamedPipeClientProcessId → 查对端映像路径）等真 Windows
// 测试机到位再上——写一段没法在本机跑的核验代码，比把门先开在 DACL 这一层更危险。
const pipeSDDL = "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;IU)"

// Listen 建命名管道服务端〔2026-10-26 接真——原先这里与 CheckPeer 都是"如实报错"的占位，
// Windows 包里的服务是哑的〕。DACL 上面那条就是门：SYSTEM/管理员/交互式用户可连，别的拒在系统层。
// 消息模式不启用：通道是"一行一条 JSON"的字节流语义，字节流正合适。
func Listen(string) (net.Listener, error) {
	l, err := winio.ListenPipe(pipeName, &winio.PipeConfig{
		SecurityDescriptor: pipeSDDL,
		MessageMode:        false,
		InputBufferSize:    1 << 16,
		OutputBufferSize:   1 << 16,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: 命名管道 %s 建不出来: %w", errs.CodeIPCListen, pipeName, err)
	}
	return l, nil
}

// CheckPeer 现阶段交由 DACL 把门（见 pipeSDDL 那条注释）；进程级核验等真 Windows 测试机。
func CheckPeer(net.Conn) error { return nil }
