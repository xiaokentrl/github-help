package main

import (
	"fmt"
	"log"
	"net"

	"github.com/hlep/github-accel/internal/hosts"
	"github.com/hlep/github-accel/internal/ipc"
	"github.com/hlep/github-accel/internal/menu"
	"github.com/hlep/github-accel/internal/service"
	"github.com/hlep/github-accel/internal/settings"
)

// wiring.go 把"接哪几样、按什么顺序接"收在这一个文件里。
//
// 干什么用：main 只管命令行、信号和收尾，真正的组装（清单 → 引擎 → 队列 → 通道）全在 buildStack 这一处。
//
// 什么情况下会错、出错时人看得见什么：
// ① 通道先建、方法后挂 → 界面第一句就问"没这个方法"，看起来像服务坏了；所以 Serve 之前必须先把 16 个挂齐；
// ② 事件出口忘了接 → 界面上进度条永远不动，任务其实跑完了；所以 Broadcaster 直接给 srv，接不上就编不过；
// ③ 备份目录拿不到就猜一个临时位置 → 用户的旧 hosts 被写进别人家；所以拿不到就如实传空串，让引擎回执"这次没存备份"。
//
// 怎么回来：这三件事的断言在 wiring_test.go，红了就改这一个文件，不许把断言放宽。

// stack 是一次组装要用的全部零件。测试拿它走真通道、真临时路径，不碰任何人的真文件。
type stack struct {
	srv        *ipc.Server
	svc        *service.Service
	path       string // 服务实际写的那一份 hosts
	socketPath string
}

// buildStack 认清单、造引擎、接队列、挂方法。它不监听、不起队列，也不读设置之外的东西。
//
// hostsPath／settingsPath 为空就取本机默认；备份目录为空就交给引擎当"不存备份"。
func buildStack(hostsPath, backupDir, settingsPath string, upd service.Updater) (*stack, error) {
	catalog, err := menu.Load()
	if err != nil {
		return nil, fmt.Errorf("域名清单没认出来: %w", err)
	}
	if hostsPath == "" {
		hostsPath = hosts.DefaultPath()
	}

	srv := ipc.NewServer(ipc.CheckPeer)
	st := &stack{srv: srv, path: hostsPath}
	st.svc = service.New(service.Config{
		Catalog:      catalog,
		Engine:       hosts.NewFileEngine(hostsPath, backupDir),
		HostsPath:    hostsPath,
		SettingsPath: settingsPath,
		Version:      version,
		BuildDate:    buildDate,
		Broadcaster:  srv,

		Updater: upd})
	st.svc.Register(srv)
	return st, nil
}

// listen 在本机那条端点上开始服务。它要求调用方已经建好 stack——方法挂在 Serve 之前，
// 这条顺序由 serve() 这一个出口保证，别处不许自己先 Serve。
func (st *stack) listen(socketPath string) (net.Listener, error) {
	if socketPath == "" {
		p, err := ipc.Endpoint()
		if err != nil {
			return nil, err
		}
		socketPath = p
	}
	st.socketPath = socketPath
	return ipc.Listen(socketPath)
}

// defaultSettingsPath 给主程序用：取不到（第一次用、目录建不出来）就留空串，
// 让 service 自己在每一次读写时现取一次并把错误如实留在状态栏——不在这里编一个路径。
func defaultSettingsPath() string {
	p, err := settings.Path()
	if err != nil {
		log.Printf("设置文件位置没拿到，每次现取: %v", err)
		return ""
	}
	return p
}
