// Command service 是常驻后台服务：它是唯一被允许改 hosts 的那个人。
// 这一版把整条链接上了：清单 → 引擎 → 队列 → 通道。检测、应用、恢复都在这里跑，
// 界面（cmd/gui 和 frontend/ 那棵树）只能通过通道问一句，别的路径一条也不给。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/hlep/github-accel/internal/settings"
)

// version 与 buildDate 由打包那一步（-ldflags -X）填进来。
//
// 什么情况下会错、出错时人看得见什么：buildDate 在代码里编一个死日期，
// 用户按它判断"我装的是哪一天的包"就完全是错的——所以它留空，界面显示"—"。
var (
	version   = "0.0.0-m1"
	buildDate string
)

func main() {
	var socketPath, hostsPath string
	flag.StringVar(&socketPath, "socket", "", "监听地址；不填就用本机用户的那条默认 Unix 端点")
	flag.StringVar(&hostsPath, "hosts", "", "要写的 hosts 文件；不填就用本机的默认位置")
	flag.Parse()

	// 备份目录拿不到就传空串——引擎会如实回执"这次没存备份"，不许它猜一个临时位置把用户的旧文件写进去。
	backupDir, err := settings.BackupDir()
	if err != nil {
		log.Printf("备份目录没拿到，这次不存备份: %v", err)
		backupDir = ""
	}

	st, err := buildStack(hostsPath, backupDir, defaultSettingsPath(), nil)
	if err != nil {
		log.Fatalf("起服务失败: %v", err)
	}

	l, err := st.listen(socketPath)
	if err != nil {
		log.Fatalf("起服务失败: %v", err)
	}
	defer func() {
		_ = l.Close()
		_ = os.Remove(st.socketPath)
	}()

	// 顺序是钉住的：方法先挂齐（buildStack 里做了），再起队列，最后才 Accept。
	// 这里**没有**"服务自己起一轮开机自动检测／自动应用"那一步，2026-10-23 这一轮把它删掉了：
	// 什么情况下会错、出错时人看得见什么——那两把开关说的是"自动对**已勾选**的项各跑一次"，
	// 而"勾了哪几条"只有界面知道（契约里条目根本没有 checked 这一栏，勾选是界面自己的状态）。
	// 服务自己起批只能按 nil＝"清单里全部 54 条"展开，症状是：界面上开关明明关着，
	// 每次开机 hosts 还是被整批改写；同时界面自己那趟自动化又跑一遍，等于两批并排。
	// 怎么回来：这一条的唯一主人是界面（frontend/core/flows.js 的 runStartupAutomation，
	// 它走的就是 Detect／ApplyQueue 那两个方法，跟人点按钮同一条 submit、同一批事件，L1 那句口径没动）。
	ctx, cancel := context.WithCancel(context.Background())
	st.svc.Start(ctx)

	go func() {
		if err := st.srv.Serve(l); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("服务退出: %v", err)
			cancel()
		}
	}()

	fmt.Fprintf(os.Stderr, "github-accel service %s 已在 %s 上等着（PID %d，写 %s）\n",
		version, st.socketPath, os.Getpid(), st.path)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	// 先关监听与在飞的网络调用，再等队列把手上那一条跑完——顺序反了就会留下半个写入。
	_ = l.Close()
	cancel()
	st.svc.Stop()
	_ = os.Remove(st.socketPath)
}
