package resolve

import (
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// 这一支清单是"一轮检测最多能发多少事"唯一的家。
//
// 干什么用：一轮里 54 条条目摊开是 69 个域名槽，每个槽要把参选名单走到底（github 与 ai 两块四档、
//
//	其余九块两档、屏蔽那块一档），不设上限就是 179 次取数再各乘一批探测，
//	把用户的网络跑死、界面全线转圈，看起来像工具坏了。
//
// 什么情况下会错、出错时人看得见什么：数字抄第二份（哪个方式文件自己写个"最多探 20 个"）→ 两处口径不一样，
//
//	调一处另一处照旧，用户看到的还是转圈；所以门禁 G11 只认这一处（除本文件外，包里不许出现数字常量）。
//
// 怎么回来：要调只调这一个文件，并且把新读数回写技术方案第 15 节那条（E6 记的就是这些数）。
//
// 下面每一个数都带 2026-10-22 在本机的实测依据，没有一个是我编的：
//   - 取数：raw.hellogithub.com/hosts 200／0.22 秒，api.github.com/meta 200／0.53 秒，gh-proxy 镜像 200／0.47 秒；
//     raw.githubusercontent.com 连不通，dns.google 在 6 秒整超时，cloudflare-dns.com 的 TLS 被对端重置。
//   - 探测：api.github.com/meta 的 web+api 共 68 个前缀（64 个 IPv4），按"每段最多取 3 个"去重得 60 个候选，
//     并发 8 跑完 5.01 秒，其中 57 支握上手，成功那批最慢 398 毫秒，剩 3 支一直拖到 5 秒超时；并发加到 16 还是 5.01 秒
//     ——拖时间的是超时不是数量，所以加并发不省时间，8 够用。
//   - 服务性判定（带上正确域名发一次 HTTPS）：16 支候选并发 8 跑完 3.01 秒，成功那批的握手 311～1062 毫秒。
const (
	// ModeTimeout 是一次取数的上限。依据：正常源实测都在 0.6 秒内交回，被拦住的源要 6 秒才确定失败——
	// 取 5 秒，既不给慢源添一次假失败，也不让它拖住整轮。
	ModeTimeout = 5 * time.Second
	// ProbeTimeout 是单次探测的上限。依据：TCP 443 成功那批最慢 398 毫秒，带域名的 HTTPS 成功那批最慢 1062 毫秒，
	// 2 秒把它们全保住，而到 5 秒才失败的那几支在 2 秒就已经能判死。
	ProbeTimeout = 2 * time.Second
	// Concurrency 是探测与取数共用的并发上限。依据：上一段那两组读数都是按 8 跑的；加到 16 墙钟一秒没省。
	Concurrency = 8
	// CandidatesPerPrefix 是官方段每个前缀最多取几个地址。依据：3 个跑出 60 个候选、命中率 57/60。
	// 取更密（每段 10 个）没量过，不猜；段数一多，整池由 MaxCandidatesPerMode 截住。
	CandidatesPerPrefix = 3
	// MaxCandidatesPerMode 是一个方式一轮最多交回几个候选地址。依据：16 支按并发 8 探完 3.01 秒，
	// 也就是一条域名从这一个方式到"选出最优 IP"约 3 秒；再乘上去就到分钟级了。
	MaxCandidatesPerMode = 16
	// SampleWindow 是"近 N 次"那个 N。依据见第 6.2 节末那句：这个数属"要先实测"那一类（E6 甲），
	// 今天能给的最诚实取法是 5——奇数，中位数不需要取两点平均，又等于"最近五轮"的历史。
	// 三轮判据（M2 验收那句"连续三轮排名不乱跳"）落在窗口里就是：同一批样本进得来、比得出。
	SampleWindow = 5
	// RTTRepeats 是指标二每条地址重复测几次取低分位。依据：单次抖动实测有（同一批里 74～404 毫秒都出现过），
	// 三次取最小能压掉一次偶发的排队；再多就只是多花 2×ProbeTimeout。
	RTTRepeats = 3
	// FetchesPerRound 是一轮取数次数的上限。依据分两档摆，两档都要装得下。
	//
	//	实测那一档：2026-10-22 在本机把这两个上限临时放到极大，一次「全部检测」（54 条／69 个域名槽）
	//	真花掉 160 次取数、277 秒跑完，54 条一条也没被截。
	//
	//	结构那一档更要紧：245 才是"每个域名槽把参选名单一路走到底"的上限——github 与 ai 两块 22 槽
	//	各 5 档（本地缓存／社区源／官方段／在线查询／DoH）110 次，其余九块 44 槽各 3 档 132 次，
	//	屏蔽那块 3 槽 1 档 3 次。为什么按这一档定而不是按实测：这台机器"部分域名被拦"才是常态，
	//	那种时候正是前面几档接连空手、一个槽要跑满整串名单的时候，截在这里等于在最需要它的时候不好用。
	//
	//	取 256 装得下 245。最坏墙钟：256 次每次都等满 5 秒＝21 分钟，那是"每一条每次都取数超时"那一档，
	//
	//	真正该在这一档短路的是断网判定（GAC-DETECT-006，今天还没接，欠账记在后端第 6.4 节第 1 条），
	//	不是靠把次数调小去遮——调小只会变成"本轮没轮到"，见 service/queue.go 收尾那五条分支。
	FetchesPerRound = 256
	// ProbesPerRound 是一轮里探测**次数**的上限——一次握手算一次，同一条地址重复测三次就占三次。
	//
	//	依据：还是那一次不限次的整张清单，退回差额之后净花 591 次（那一轮 30 条拿到地址）。
	//
	//	取 1024＝实测那次再留七成，一轮不截；最坏墙钟 1024／并发 8 批 × 2 秒＝4 分 16 秒，
	//
	//	跟实测那一轮的 277 秒同一量级，界面每一行都在往前走，不是"全线转圈像坏了"。
	//
	//	谁调这个数都得按"次"记账，别按"条"，否则等于配额白给三倍（同一条地址重复测三次算三次）。
	ProbesPerRound = 1024
	// MetaCacheTTL 是 api.github.com/meta 那份段清单能复用多久。
	// 依据：〔外部依据〕这个接口匿名 60 次/小时；缓存 30 分钟＝最多 2 次/小时，离撞墙还有很远。
	MetaCacheTTL = 30 * time.Minute
	// FetchBodyBytes 是一次取数最多读回多少正文。依据：社区清单实测 2498 字节，meta 那份 JSON 约 20 KB，
	// 64 KB 把两个正常源都装下，同时不让一个坏源把内存吃满。
	FetchBodyBytes = 64 * 1024
	// ProbePort 是探测打的端口。它跟上面那些数不是一类：不是"量出来定多少"的上限，而是协议事实——
	// HTTPS 就是 443，指标二量的也是"443 握上手要多久"，改成别的端口这两项指标量的就不是同一条路了。
	// 为什么放这儿而不是 detect：detect 一个数字都不许有（那包的第 ⑤ 条 + 门禁），
	// 这个数只能从外面喂；而唯一的家只能是这一本账，否则"探哪个端口"会变成两个地方各说一套。
	ProbePort = 443
)

// RoundPlan 是"这一轮到目前为止发了多少"。
type RoundPlan struct {
	Fetches int
	Probes  int
}

// Check 判这一轮还能不能再发事。
//
// 干什么用：把上面那两个上限变成一次调用问得出来的答案，service 每一批之前问一次。
// 什么情况下会错、出错时人看得见什么：没到上限却谎报"本轮按预算只算了前 N 条"＝骗人；
//
//	到上限了还继续发＝用户的网络被跑死、界面全转圈。这里只有后者会红，红得干脆。
//
// 怎么回来：返回 GAC-DETECT-005 并带上"已发多少／上限多少"，界面那句"剩下的排队"照走进度条，不许演假进度。
func Check(round RoundPlan) error {
	if round.Fetches > FetchesPerRound || round.Probes > ProbesPerRound {
		return errs.New(errs.CodeDetectBudget, map[string]any{
			"fetches": round.Fetches, "fetchLimit": FetchesPerRound,
			"probes": round.Probes, "probeLimit": ProbesPerRound,
		}, "本轮按预算只算了前 %d 次取数、%d 次探测，剩下的排队", round.Fetches, round.Probes)
	}
	return nil
}

// FetchRoom 给出这一轮还能发几次取数（已经超了就给 0，不给负数）。
func (r RoundPlan) FetchRoom() int {
	return room(FetchesPerRound, r.Fetches)
}

// ProbeRoom 给出这一轮还能发几次探测。
func (r RoundPlan) ProbeRoom() int {
	return room(ProbesPerRound, r.Probes)
}

func room(limit, used int) int {
	if used >= limit {
		return 0
	}
	return limit - used
}
