package detect

import (
	"context"
	"net"
	"sort"
	"time"
)

// rtt.go 是指标二：TCP 握手延迟，同一支地址重复测几次取最低那一个。
//
// 干什么用：给"能用"的那批地址排出快慢。它是排序的原料，不是判好坏的根据——
//
//	所以它失败（连不上）不写 Shape，那件事已经由指标一记过了，这里只交回"没测出来"。
//
// 什么情况下会错、出错时人看得见什么：
// ① 只测一次 → 偶发的一次排队就把一支地址排到最后，下一轮又回到第一，界面那行"最快 IP"天天换人。
//
//	所以按 lim.RTTRepeats 重复，取最低那一个：偶发的抖动只影响它那一次。
//
// ② 把"没测出来"当成 0 毫秒 → 一支连不上的地址会变成全批最快，直接当主用写进 hosts。
//
//	所以交回两个值：时长与"测到没有"，调用方拿 RTTOK 判，不许拿 0 猜。
//
// 怎么回来：重复次数与超时帽子都由 Limits 从外面进来（唯一的家在 resolve/budget.go）；
// 要改"取最低还是取中位"，只改 lowRTT 那一句。

func measureRTT(ctx context.Context, ip string, lim Limits) (time.Duration, bool) {
	dialer := &net.Dialer{}
	var samples []time.Duration
	for i := 0; i < lim.RTTRepeats; i++ {
		if ctx.Err() != nil {
			break // 帽子到了就停手：剩下的次数只会各等一个超时
		}
		started := time.Now()
		conn, err := dialer.DialContext(ctx, "tcp", addr(ip, lim.Port))
		if err != nil {
			continue
		}
		took := time.Since(started)
		_ = conn.Close()
		samples = append(samples, took)
	}
	return lowRTT(samples)
}

// lowRTT 从几次读数里取最低那一个；一个都没有就交回"没测出来"。
// 它是纯函数：排的是拷贝，传进来的切片改不动。
func lowRTT(samples []time.Duration) (time.Duration, bool) {
	if len(samples) == 0 {
		return 0, false
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[0], true
}
