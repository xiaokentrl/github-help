// Package detect 只管一件事：一种方式交回来的那批地址里，哪一个此刻真能服务这条域名、哪一个最快。
//
// 它不知道"这批地址是哪种方式给的"，也不知道"该用哪种方式"——那是 resolve 的活。
// 两层合流只有一处：internal/service/detectrun.go。第 8 节第 7 条用一条机器检查钉住这件事。
//
// 干什么用：
// 界面那行"当前最快 IP／延迟多少毫秒"、写进 hosts 的那一个地址，都是这里定的。
// 两项指标（已定：技术方案第 8.2 节甲）：
//   - 指标一＝带上正确域名发一次请求，看对面答不答（probe.go）；
//   - 指标二＝TCP 443 握手延迟，重复测取低分位（rtt.go）。只给指标一答上的那支量：
//     答不上的永远当不了 Best，量了纯吃预算（见 measureAll 那段说明）。
//
// 排序与"疑似被阻断"的判定是纯函数（rankips.go／blocked.go），能在 go test 里逐例钉死，不靠真网络。
//
// 什么情况下会错、出错时人看得见什么：
// ① 用下载速度排序 → 每次"最优"都在跳，M2 那句「排名不再乱跳」当场作废；
//
//	所以排序只看"能不能服务"和"握手多快"，决胜一律再比地址字符串，一次都不许交换顺序。
//
// ② 把"TLS 握手成功"当成"真能服务这条域名" → 写进 hosts 的地址一连就上但答非所问，
//
//	用户看到的是"页面打不开而工具说成功"。所以指标一必须带着域名发请求。
//
// ③ 拿证书名字对不对去判"这个 IP 行不行" → GitHub 的证书没有 IP SAN，按 IP 验名字永远失败，
//
//	一条都探不上。这里的做法是连接钉到候选 IP、而 SNI 与 Host 用域名（curl --resolve 那一路），
//	证书是拿去跟**域名**对的：验不过＝这个地址不是这条域名的正主（badCert），不算被阻断。
//
// ④ 被 SNI 阻断的域名换 IP 也不通 → 无限重试就是"界面一直显示检测中"。
//
//	所以全批候选一条都没答时给 GAC-DETECT-004，队列据此不再反复重试（blocked.go）。
//
// ⑤ 上限（超时、重复次数、并发、读回正文的字节数、端口）在本包一个都不许出现：
//
//	两处各定一套数＝同一批候选在不同轮里被不同尺子量过，排名看着像随机。
//	门禁 TestGateDetectTakesNoOwnLimits 扫的就是这个（数值只由 Limits 从外面喂进来）。
//
// 怎么回来：
// 排名怪——先读 Result.Outcomes 每条的 Shape 与 RTT，判据只在 rankips.go 那一处；
// 上限要调——回 resolve/budget.go 那一处改，由 service 注入过来，别在这里再写一份；
// 阻断误判——blocked.go 那张形状表就是判据本身，改它只改那一处。
package detect

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Limits 是探测这一轮要用的全部上限。它们唯一的家在 resolve/budget.go（G11），
// 由 service 读出来注入这里——本包因此不需要知道"上限该是多少"，也就抄不出第二份。
type Limits struct {
	// ProbeTimeout 是单支候选从发起到拿到回答的上限。
	ProbeTimeout time.Duration
	// RTTRepeats 是指标二对同一支地址重复测几取低分位。
	RTTRepeats int
	// Concurrency 是一批候选同时探几支。
	Concurrency int
	// BodyBytes 是一次探测最多读回多少正文。今天只用它数个数，不拿正文内容做判据
	// （拿正文判拦截页缺实测依据，等 M2 第一天量过再写，见技术方案第 15 节 E6）。
	BodyBytes int
	// Port 是探的对象端口。它是个数字，所以只能从外面进来（见本包顶上第 ⑤ 条）。
	Port int
}

// Shape 是一支候选的读数形状。它是内部数据不是文案：界面拿 Result.Code，不拿这个串。
type Shape string

const (
	ShapeServed   Shape = "served"   // 对面带着这个域名答了一句，状态码可用
	ShapeTimeout  Shape = "timeout"  // 到点还没答上
	ShapeReset    Shape = "reset"    // 连接被对端掐掉（阻断常见形状之一）
	ShapeRefused  Shape = "refused"  // 端口没人听
	ShapeBadCert  Shape = "badCert"  // 证书不认这条域名（名字对不上或根不认）：这个地址不是它的正主，不算阻断
	ShapeNoRoute  Shape = "noRoute"  // 连路由都没有（本机断网、地址不可达）
	ShapeProtocol Shape = "protocol" // 答回来的不是 HTTP 形状
	ShapeOther    Shape = "other"    // 认不出来的形状：如实留着，不硬塞进上面任何一档
)

// Outcome 是一支候选的读数。
type Outcome struct {
	IP       string
	CanServe bool
	Shape    Shape
	// RTT 是指标二取到的低分位；没测出来是 0（0 与"极快"分不开，所以配 RTTOK）。
	RTT   time.Duration
	RTTOK bool
	// Status 是指标一拿到的状态码；没答上为 0。
	Status int
	// BodyBytes 是读回的正文长度（只到 Limits.BodyBytes 为止）。
	BodyBytes int
	// Detail 给人看的原始失败原因（日志／调试用）。它不进契约，界面只看 Code。
	Detail string
}

// Result 是一条域名一轮探测的结论。
type Result struct {
	Domain string
	// Best 是最优地址；空串表示一个都没探上。
	Best string
	// RTT 是 Best 那支的握手延迟；Best 为空时为 0。
	RTT time.Duration
	// Outcomes 按名次排好（探上的在前，快的在前）。界面那行"探测明细"直接用它。
	Outcomes []Outcome
	// ProbeCount 是这一轮真发出去的连接次数，算给 budget.go 那本账用——
	// 所以这一处是唯一会数的地方，service 不许自己再拿候选数乘一遍（两处各算一份早晚对不上）。
	// 它是上限值：中途被叫停时真发的比它少，因为重复次数是按 RTTRepeats 记的。
	ProbeCount int
	// Code 是"全批都没探上"时给界面的错误码（GAC-DETECT-003／004）；探上了为空。
	Code string
	// Params 配 Code 用（域名、候选数、失败形状各几支）。整句人话在前端语言文件那一个家〔已定：D6 乙〕。
	Params map[string]any
}

// Blocked 为真表示这一条疑似被针对性阻断：队列据此不再反复重试（已定：技术方案第 8.2 节末）。
func (r Result) Blocked() bool { return r.Code == codeBlocked }

// Inspector 是编排层看这个包的全部形状：给它一批地址，它交回一份结论。
//
// 为什么要是接口（第 11.2 节那六个之一）：真探测要往外发连接，一轮跑下来成不成全看此刻的网络；
// service 的用例必须能把返回值钉死，否则"暂停停在方式之间"这类判据只能在真网络上碰运气。
// *Checker 就是它的真件，二者共用同一份判据代码。
type Inspector interface {
	Check(ctx context.Context, domain string, ips []string, lim Limits) Result
}

var _ Inspector = (*Checker)(nil)

// Checker 把"一条域名的一批候选"跑成一份 Result。
//
// 为什么是个结构体而不是一个函数：真探测要往外发连接，用例必须能换成假件——
// 判据不能靠"今天网络上运气如何"（第 11 节那句"能被换掉的那一处边界"）。
type Checker struct {
	probe   func(ctx context.Context, domain, ip string, lim Limits) Outcome
	measure func(ctx context.Context, ip string, lim Limits) (time.Duration, bool)
}

// NewChecker 造一个真探测器：真的带域名发 HTTPS 请求、真的量 TCP 握手。
func NewChecker() *Checker {
	return &Checker{probe: prober{}.ask, measure: measureRTT}
}

// Check 探测一批候选：指标一支不落，指标二只给答上的那支量（见 measureAll 里那段说明）。
//
// 它按 lim.Concurrency 摊开发，任何一支到 lim.ProbeTimeout 就单独判超时，不拖累整批；
// 一批里地址为空的（上游算错了）如实丢掉，不会当成一支"探不上的候选"去拉低结果。
// 空批不 panic 也不当成"全都不能服务"：那是这一轮这个方式没给回地址，如实回 GAC-DETECT-001，
// 跟"探了一堆全灭"（003）与"疑似被阻断"（004）分开——三种情况界面要说三句不同的话，队列对它们的处理也不同。
func (c *Checker) Check(ctx context.Context, domain string, ips []string, lim Limits) Result {
	outcomes, probes := c.measureAll(ctx, domain, cleanIPs(ips), lim)
	res := RankIPs(domain, outcomes)
	res.ProbeCount = probes
	return res
}

// measureAll 并发把两项指标量完，回来的顺序与进去的顺序一致（名次只由 RankIPs 定）。
// 第二个返回值是这一轮发出去的探测次数，进 Result.ProbeCount（那本账只有一个家，见它的注释）。
func (c *Checker) measureAll(ctx context.Context, domain string, ips []string, lim Limits) ([]Outcome, int) {
	out := make([]Outcome, len(ips))
	if len(ips) == 0 {
		return out, 0
	}

	// 并发给 0 不等于"不探"：那样信号量一支都发不出去，整轮挂在这儿。
	// 所以最小按 1 处理——一支一支探，慢但不死。上限本身由 service 从 budget.go 注入。
	workers := lim.Concurrency
	if workers < 1 || workers > len(ips) {
		workers = len(ips)
	}

	var probes atomic.Int64
	var wg sync.WaitGroup
	slots := make(chan struct{}, workers)
	for i, ip := range ips {
		slots <- struct{}{}
		wg.Add(1)
		go func(i int, ip string) {
			defer wg.Done()
			defer func() { <-slots }()

			// 两项指标各拿一份超时帽子：握手延迟不该被前一次探测用完的时间挤成"超时"。
			pctx, pcancel := context.WithTimeout(ctx, lim.ProbeTimeout)
			defer pcancel()
			probes.Add(1)
			o := c.probe(pctx, domain, ip, lim)

			// 指标一没答上的那支不再量握手延迟。为什么敢省：它永远当不了 Best
			// （rankips.go 第一位是 CanServe），量出来的延迟也进不了名次——白占预算。
			// 省多少：一支候选省掉 RTTRepeats 次握手。不省的话一条域名十支候选三轮就吃掉一本账，
			// GitHub 那条主力路径会被自家上限截断，症状是"检测跑完但排名少了一半地址"。
			// 省的是预算，不是名次。
			if o.CanServe {
				mctx, mcancel := context.WithTimeout(ctx, lim.ProbeTimeout)
				defer mcancel()
				o.RTT, o.RTTOK = c.measure(mctx, ip, lim)
				probes.Add(int64(lim.RTTRepeats))
			}

			out[i] = o
		}(i, ip)
	}
	wg.Wait()
	return out, int(probes.Load())
}

func cleanIPs(ips []string) []string {
	out := make([]string, 0, len(ips))
	seen := map[string]bool{}
	for _, ip := range ips {
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	return out
}

// addr 把候选地址与端口拼成"连到哪儿"。IPv6 必须带方括号，否则地址与端口分不开。
// 拼法只在这一处，而且不交给 net.JoinHostPort 去猜"带没带过括号"：
// 两处各拼一遍早晚有一处忘了带，症状是 IPv6 候选一条都探不上——那种失败看着像"这个地址不行"，不是像"我们拼错了"。
// 作用域后缀（fe80::1%eth0）在上游那几档就剥掉了，到这里应该是干净地址。
func addr(ip string, port int) string {
	host := ip
	if strings.Contains(ip, ":") && !strings.HasPrefix(ip, "[") {
		host = "[" + ip + "]"
	}
	return host + ":" + strconv.Itoa(port)
}
