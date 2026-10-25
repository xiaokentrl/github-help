package detect

import (
	"github.com/hlep/github-accel/internal/errs"
)

// blocked.go 管一件事：如实标出"这条域名疑似被针对性阻断，换 IP 也没用"。
//
// 干什么用：一轮里全批候选一条都没答上时，得说清这是"这批地址都不行"还是"这条域名被专门拦着"。
//
//	后者的处理完全不同：不用再反复重试，界面直接给"疑似被阻断"那一档（已定：技术方案第 8.2 节末）。
//
// 什么情况下会错、出错时人看得见什么：
// ① 把"全批失败"一律当阻断 → 源里配错一个地址、本机断网，都会被标成"换 IP 无效"而从此不再重试。
//
//	用户看到的是"工具说这条被封锁了"而其实只是清单写错／没网。所以判据看**失败形状**，不看失败条数。
//
// ② 一张表两处写 → 一处算阻断一处不算，同一个结果在队列里被两种说法处理。
//
//	所以形状表只在这个文件里，rankips.go 只问"算不算阻断"，不自己认形状。
//
// 怎么回来：要加一种阻断形状，就改 blockingShapes 这一张表；改完这条域名的码会在 004 与 003 之间自己换，
//
//	不用动别的文件。判据本身可以单测——它不联网，输入是一批读数。
//
// 今天这张表的依据：2026-10-22 本机实测到的两种拦法——dns.google 到点不答（timeout）、
// cloudflare-dns.com 的 TLS 被对端重置（reset）。8.8.8.8:443 是连不上（noRoute），
// 那是"这条路不通"而不是"这条域名被专门拦"，所以它没进这张表。
var blockingShapes = map[Shape]bool{
	ShapeTimeout: true,
	ShapeReset:   true,
}

const (
	codeEmpty   = errs.CodeDetectEmpty   // 一批候选都没有：这一轮这个方式没给回地址
	codeBlocked = errs.CodeDetectBlocked // 疑似被针对性阻断：换 IP 无效，别再重试
	codeNoServe = errs.CodeDetectNoServe // 交回的地址全都不能服务：换下一批候选还有意义
)

// isBlocked 判这一批读数是不是"整条域名被拦"那一档。
//
// 空批不算阻断：那是上游这个方式这一轮没给回地址，该由取数那侧记账，不该由探测这侧编出一条"被封锁"。
func isBlocked(outcomes []Outcome) bool {
	if len(outcomes) == 0 {
		return false
	}
	for _, o := range outcomes {
		if o.CanServe || !blockingShapes[o.Shape] {
			return false
		}
	}
	return true
}

// failCode 给"一个都没探上"挑该回界面哪一个码，并把每种形状各几支一起带出去。
// 参数是给语言文件填句子用的（GAC-DETECT-004 那句要能说出"探了几支、都是什么形状"）。
func failCode(domain string, outcomes []Outcome) (string, map[string]any) {
	shapes := map[string]int{}
	for _, o := range outcomes {
		shapes[string(o.Shape)]++
	}
	params := map[string]any{
		"domain":     domain,
		"candidates": len(outcomes),
		"shapes":     shapes,
	}
	switch {
	case len(outcomes) == 0:
		return codeEmpty, params
	case isBlocked(outcomes):
		return codeBlocked, params
	default:
		return codeNoServe, params
	}
}
