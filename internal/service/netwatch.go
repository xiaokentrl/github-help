package service

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/hlep/github-accel/internal/contract"
)

// netwatch.go 是断网判定（GAC-DETECT-006，欠了一路的那个"真网络探测"）〔2026-10-26 接上〕。
//
// 干什么用：一个循环每 interval 判一次"这台机器现在通不通网"，只在相位翻转时动作，
// 推 network:change{online:...}（带完整快照）——界面状态栏与气泡如实说"网络已断开／已恢复"。
// 探测的暂停**不在这一层拉闸**：queue.go 的 take() 在断网期间把检测类留在等待里
// （应用／恢复不需要网,照跑;恢复联网后自然放行）〔已定：2026-10-26 用户拍"断网暂停探测,再联网可'继续'"〕。
//
// 判据：TCP 拨一圈公共 DNS 的 443（223.5.5.5／119.29.29.29），2 秒一个，任一通＝在线。
// 为什么不用"网卡有没有地址"：网卡在而网关死是断网的常见形状，那一种只有真发包才知道。
// 为什么要一圈而不是一个：任一通就走，单点抖动不至于误报。
//
// 什么情况下会错、出错时人看得见什么：
// ① 判定循环本身挂了 → 没人再报断网。所以它跟着服务的 ctx 走，panic 由 recover 接住并如实发一条事件；
// ② 代理环境下直连 DNS 全被墙 → 误报"断网"。这是开发者工具的既定取舍（服务以 root 跑在用户会话里，
//    代理环境变量就在身边）；真误报了，队列的闸还在，点「继续」照跑，误报只是多一句话。
//
// 怎么回来：判据在 netCheckDefault（拨号名单就这一处）；翻转让谁暂停在 transition（那一个 if）；
// 用例把 svc.netChecker 换成假件拨开关，不碰真网。

// netProbeHosts 是判在线用的拨号名单。都是公共 DNS 的 443——不是"GitHub 通不通"，
// 通不通网是这台机器的事，GitHub 通不通是下一层的事（那种断法 detect 自己会按方式报错）。
var netProbeHosts = []string{"223.5.5.5:443", "119.29.29.29:443"}

const (
	netCheckTimeout = 2 * time.Second
	netWatchEvery   = 5 * time.Second
)

// netEvery 是判定节拍：New 给生产值（5 秒），用例调快——拉闸那一步等的是"下一个节拍"，
// 节拍写死的话用例就得干等 5 秒。
var netEvery = netWatchEvery

// netCheckDefault 拨一圈名单，任一通＝在线。
func netCheckDefault() bool {
	for _, host := range netProbeHosts {
		conn, err := net.DialTimeout("tcp", host, netCheckTimeout)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

// startNetworkWatch 起判定循环。ctx 走服务那一份：Stop 的时候它一起收工。
func (s *Service) startNetworkWatch(ctx context.Context) {
	if s.netChecker == nil {
		return
	}
	s.lane.Add(1)
	go func() {
		defer s.lane.Done()
		defer func() {
			if r := recover(); r != nil {
				s.publish(contract.EventNetworkChange, map[string]any{"online": false, "watcherPanic": fmt.Sprint(r)})
			}
		}()
		// 开局先判一次，把初值立起来（不推事件：开机那一份 GetSnapshot 里带着）
		online := s.netChecker()
		s.mu.Lock()
		s.netOffline = !online
		s.mu.Unlock()
		ticker := time.NewTicker(netEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			nowOnline := s.netChecker()
			s.mu.Lock()
			changed := nowOnline == s.netOffline
			if changed {
				s.netOffline = !nowOnline
			}
			s.mu.Unlock()
			if changed {
				// 泵在 wake 上睡着等活儿：联网翻转那一刻必须叫它醒来看一眼,
				// 不然等待中的检测会一直躺到下一次谁碰队列(容器实测抓到的坑)。
				s.notify()
				s.publish(contract.EventNetworkChange, map[string]any{"online": nowOnline})
			}
		}
	}()
}
