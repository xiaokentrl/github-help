// Command gui 是界面壳：它把 frontend/ 构建出来的那一份界面读进窗口，
// 再把界面上的每一次点击原样转给后台服务，把服务推进来的每一条事件原样转回窗口。
// 它自己不碰 hosts、不检测、不排队，也不判断"服务大概还在跑"——界面看到的一切状态都由服务给。
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/ipc"
)

func main() {
	var socketPath, dist string
	flag.StringVar(&socketPath, "socket", "", "后台服务地址；不填就用本机用户的那条默认 Unix 端点")
	flag.StringVar(&dist, "dist", "", "界面构建产物（frontend/dist）在哪；不填就按可执行文件旁边、仓库里那两处找")
	flag.Parse()

	dir, err := resolveDist(dist)
	if err != nil {
		log.Fatalf("界面起不来: %v", err)
	}
	if socketPath == "" {
		p, err := ipc.Endpoint()
		if err != nil {
			log.Fatalf("界面起不来: %v", err)
		}
		socketPath = p
	}

	ctx, stop := context.WithCancel(context.Background())

	app := application.New(application.Options{
		Name:        "github-accel",
		Description: "GitHub 加速助手（界面壳：改动 hosts 的全部动作都在后台服务那一侧）",
		Assets: application.AssetOptions{
			// 从磁盘读那一份构建产物。为什么不在这里嵌第二份：嵌进二进制就等于把同一份界面存两处，
			// 改了 frontend/ 还得记着重编 Go，早晚会各走各的（上一版壳就是这么岔开的）。
			Handler: application.AssetFileServerFS(os.DirFS(dir)),
		},
		// 关窗口即收尾：桥那条 goroutine 靠这个 cancel 停，不留一条还在后台问话的壳。
		OnShutdown: stop,
	})

	br := newBridge(socketPath, func(name string, data any) { app.Event.Emit(name, data) })
	// 界面→壳这一句必须开 goroutine：一次调用最长等 10 秒，占着事件分发就会把后面所有点击一起卡住。
	// 回执自己带 id，所以几条同时在飞也不会对错人。
	app.Event.On(contract.WireInvoke, func(e *application.CustomEvent) {
		go br.handleInvoke(e.Data)
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "GitHub 加速助手",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
	})

	go br.Run(ctx)

	if err := app.Run(); err != nil {
		log.Fatalf("界面退出得不对劲: %v", err)
	}
}
