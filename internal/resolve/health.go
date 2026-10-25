package resolve

import (
	"sort"
	"time"
)

// Sample 是一次检测留给选优的一条底账：这一轮这个方式交回的地址，到底能不能服务这条域名、它自己花了多久。
//
// 谁喂它：service。第 8 节第 8 条管住"resolve 不许 import store"，所以样本是当参数递进来的，本包不伸手去拿。
type Sample struct {
	ModeID string
	// CanServe 的判据是 detect 给的结论（指标一），不是方式自己说行。
	// 为什么不能由方式自己说：社区源会给出一个 TLS 都握不上的地址，它自己永远"成功"。
	CanServe bool
	// Took 是取数花的时间（Set.Took），不是握手延迟。
	Took time.Duration
}

// Stability 算第 6.2 节那把尺子 S：窗口里"这个方式给的地址被判能服务"的比例。
//
// 干什么用：选主用（第一）只看它。
// 什么情况下会错、出错时人看得见什么：用"最近一次成不成功"当稳定性 → 一轮网络抖动就把主用换人，
//
//	界面"IP 来源"那一行天天变，用户以为工具坏了；所以这里用比例。
//
// 怎么回来：窗口大小只有一个家（budget.go 的 SampleWindow），要改口径只改那一处；
//
//	没有样本时返回 -1（"还不知道"），调用方不许把 -1 当成 0 参与比较。
func Stability(samples []Sample) float64 {
	in := window(samples)
	if len(in) == 0 {
		return -1
	}
	served := 0
	for _, s := range in {
		if s.CanServe {
			served++
		}
	}
	return float64(served) / float64(len(in))
}

// MedianTook 算第 6.2 节那把尺子 V：窗口里 Took 的中位数。
//
// 干什么用：选备用（第二）看它；主用同分也由它决胜。
// 什么情况下会错、出错时人看得见什么：用平均值 → 某一次取数撞上超时就把这个方式拖到很后面，
//
//	一次抖动顶掉整档名次；中位数不会因为一次炸掉而翻天。
//
// 怎么回来：中位取"排序后中间偏左"那一个（偶数条时不取两点平均），
//
//	这样同一批样本永远只对应一个值，G10 的"两轮逐字相同"才钉得住。
func MedianTook(samples []Sample) time.Duration {
	in := window(samples)
	if len(in) == 0 {
		return 0
	}
	values := make([]time.Duration, len(in))
	for i, s := range in {
		values[i] = s.Took
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[(len(values)-1)/2]
}

// window 截出最近 N 条。样本按新旧排，最后一条最近——这个约定写在这儿，
// 因为一旦反过来，S 和 V 算的就是最老的那批，界面上一个字都看不出来。
func window(samples []Sample) []Sample {
	if n := len(samples); n > SampleWindow {
		return samples[n-SampleWindow:]
	}
	return samples
}
