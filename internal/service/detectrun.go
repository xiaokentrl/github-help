package service

import (
	"context"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/detect"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/menu"
	"github.com/hlep/github-accel/internal/resolve"
)

// detectrun.go 是全项目唯一一处把两层接起来的地方：resolve 定"这一轮该用哪个方式拿地址"，
// detect 定"这批地址里哪个真能服务该域名、哪个握手最快"。
//
// 为什么只准在这一处合流（技术方案第 8 节第 7、8 条）：两个包互不 import，
// 谁也不知道对方存在，所以"把方式耗时当 IP 快慢"这种混淆在这两个包里根本写不出来；
// 只有这里能同时看见 Set.Took 和 Outcome.RTT，也就只有这里可能搞混——判据写在下面 runMode 里，一条不许换顺序。

// roundState 是一个批次（＝界面一次点击＝"一轮"）的取数与探测预算账本。
//
// 干什么用：一轮里 51 条条目各要发几次取数、几次探测，全靠这一本账记着，超了就当场停。
// 什么情况下会错、出错时人看得见什么：不记账＝把用户的网络跑死、界面全线转圈，看起来像工具坏了；
// 记了却按"条"不按"次"＝配额白给三倍（同一条地址重复测三次是三次）。
// 怎么回来：上限的数字只在 resolve/budget.go 一个地方，这里只记账不定义；
// 超没超的判断也走 resolve.Check，所以"本轮按预算只算了前 N 条"那句话的参数只有一个家。
type roundState struct {
	plan resolve.RoundPlan
}

// wantFetch 预留一次取数。取不到额度就把 GAC-DETECT-005 原样抛回去，界面那句"剩下的排队"照走。
func (r *roundState) wantFetch() error {
	return r.spend(1, 0)
}

// wantProbes 预留"这一批候选最坏要发多少次探测"。
//
// 为什么按最坏那一档预留而不是按实际：实际次数要等探完才知道（答上的那几支还要再量延迟），
// 而预算的意义就是"别把用户的网络跑死"——事后记账等于没有预算。
// 探完之后按真实次数退回差额（usedProbes），所以预留不会虚耗额度。
func (r *roundState) wantProbes(candidates int) error {
	return r.spend(0, candidates*(1+resolve.RTTRepeats))
}

// usedProbes 把真发出去的次数退回差额。reserved 是 wantProbes 当时预留的那个数。
func (r *roundState) usedProbes(reserved, actual int) {
	back := reserved - actual
	if back <= 0 {
		return
	}
	r.plan.Probes -= back
	if r.plan.Probes < 0 {
		r.plan.Probes = 0
	}
}

// spend 先记下这一笔再问 resolve 还剩不剩余额：判断只有一个家（budget.go 的 Check），这里不自己比大小。
func (r *roundState) spend(fetches, probes int) error {
	next := r.plan
	next.Fetches += fetches
	next.Probes += probes
	if err := resolve.Check(next); err != nil {
		return err
	}
	r.plan = next
	return nil
}

// probeLimits 把预算那本账套成 detect 要的Limits。
//
// 为什么只能有这一个函数：detect 那一个包一个数字都不许有（它的包文档第 ⑤ 条 + 那条门禁），
// 所有上限只能从外面喂；而"从哪喂"这件事散到两处，同一轮就有两套上限了。
func probeLimits() detect.Limits {
	return detect.Limits{
		ProbeTimeout: resolve.ProbeTimeout,
		RTTRepeats:   resolve.RTTRepeats,
		Concurrency:  resolve.Concurrency,
		BodyBytes:    resolve.FetchBodyBytes,
		Port:         resolve.ProbePort,
	}
}

// applicable 给出适用这一块菜单的参选方式。
//
// 什么情况下会错、出错时人看得见什么：这一块菜单一个方式都没有 → 交回空清单，由 resolve.Rank
// 报 GAC-DETECT-001（"候选为空"），界面那一行立刻显示原因，不许转圈。
func (s *Service) applicable(menuID string) []resolve.Spec {
	var out []resolve.Spec
	for _, sp := range s.cfg.Modes.Specs() {
		for _, k := range sp.Kinds {
			if k == menuID {
				out = append(out, sp)
				break
			}
		}
	}
	return out
}

// detectItem 是检测那一条任务的全部动作：向 Rank 要那一轮的整串名单 → 从头一档试起 →
// 一条条目里每个域名各追各的地址，哪一档给出就记下，剩下的域名继续往后一档要。
//
// 干什么用：把技术方案第 6.1 节那七步落成一次可回归的调用，七步里只有第 6、7 步需要状态，
// 所以状态只在这儿改（bestIPs／source／latency／lastCheck）。
// 什么情况下会错、出错时人看得见什么：
// ① 主用失败就直接把任务标失败 → 界面上"只有一种来源"的条目永远修不好，备用那条路等于没接；
// ② 备用顶上却写主用 → 「IP 来源」那一行说假话，M2 那句"主用不许随便换人"就没法验；
// ③ "疑似被阻断"这一档在主用那一次就下结论 → 本机实测到的正是这一种：主用走系统解析，拿回来的地址是脏的，
//
//	探测形状正好落进"超时＝阻断"那张表，于是备用换一种来源也没机会试，条目被判成"换 IP 无效"从此不再重试——
//	而这恰恰是这台机器最需要修的那几条。所以阻断那一码先记下、继续往下试，全部参选方式都不行才把它交回界面。
//
// ④ 一个域名拖垮整条条目 → 2026-10-22 本机实测到的形状：github.com 与 api.github.com 探得上，
//
//	gist.github.com 六十支候选一支也不答。按整条成败来算，前两个已经到手的地址会跟着一起丢，
//	界面上那一行是"失败"，用户看到的是"这条根本没加速成"。所以按域名累积：拿到几个就记几个，
//	还没拿到的那几个各留最后一个错（阻断那一档优先），任务照样如实失败，地址却不白丢。
//
// 怎么回来：这四件事各有用例钉着（TestDetectFallsBackToBackupSource 一家三个、TestDetectBlocked* 那两条，
// 加上 TestDetectKeepsAddressesFromEveryDomainThatAnswered），红了改回这一个循环，不许把断言放宽。
func (s *Service) detectItem(ctx context.Context, t *task) error {
	item, m, ok := s.cfg.Catalog.ByItem(t.itemID)
	if !ok {
		return errs.New(errs.CodeTaskBadItem, map[string]any{"itemId": t.itemID}, "清单里没有这条条目")
	}
	choice, err := s.rankFor(m.ID)
	if err != nil {
		return err
	}
	// 试的顺序完全听 Rank 的整串名单（主用在前，往后就是决胜链的下一名）；这里只走，不排。
	tried := choice.Ladder
	if len(tried) == 0 {
		tried = []resolve.Spec{choice.Primary}
	}

	s.mu.Lock()
	known := copyIPs(s.itemStateOf(item.ID).bestIPs)
	s.mu.Unlock()

	run := newDetectRun()
	for i, spec := range tried {
		if i > 0 && s.pausedNow() {
			// 停在方式之间：上一个方式已经跑完（样本已记），下一个方式还没开始。
			return errPaused
		}
		role := contract.SourcePrimary
		if i > 0 {
			role = contract.SourceFallback
		}
		if err := s.runMode(ctx, t, spec, item, known, run, role); err != nil {
			// 预算用完或这一轮被叫停：已经拿到地址的域名不能白丢，先记下，再把原因交回界面。
			if len(run.best) > 0 {
				s.commitDetect(item, run)
			}
			return err
		}
		if len(run.best) == len(item.Domains) {
			s.commitDetect(item, run)
			return nil
		}
	}
	if len(run.best) > 0 {
		s.commitDetect(item, run)
	}
	// 名单走完了还有域名没地址：能记的都记了，这里只负责把"还没到手"那一句如实交回去。
	return run.residual(item)
}

// detectRun 是一条条目这一轮的累积结果。
//
// 干什么用：一条条目里几个域名可能由不同方式救回来（见 detectItem 上面第 ④ 条），
// 所以"哪个域名有了地址、由哪一档以什么身份给的、还没地址的那个最后一个错是什么"要各记各的。
// 什么情况下会错、出错时人看得见什么：这三个映射合成一个"整条成功才有内容"的结构，
// 就是第 ④ 条那个症状——已经能加速的域名跟着一个死域名一起丢。
// 怎么回来：只有 detectItem 那一个循环写它，界面读的是 commitDetect 落进 itemState 的那一份。
type detectRun struct {
	best map[string]string
	lat  map[string]int64
	// src 记每个域名的地址由哪一种方式、以什么身份（主用／备用）给回。
	src map[string]detectSrc
	// miss 记还没地址的域名各自的错；阻断那一码单独留一份，理由见 detectItem 第 ③ 条。
	miss map[string]detectMiss
}

type detectSrc struct {
	spec resolve.Spec
	role string
}

type detectMiss struct {
	last    error
	blocked error
}

func newDetectRun() *detectRun {
	return &detectRun{
		best: map[string]string{},
		lat:  map[string]int64{},
		src:  map[string]detectSrc{},
		miss: map[string]detectMiss{},
	}
}

// noteMiss 记下"这一档没给回这个域名的地址"。同一个域名换档再试，最后一个错跟着换，
// 但阻断那一码只留最早那一份：它说的是"这条域名被专门拦着"，比"这批地址都不行"更接近原因。
func (r *detectRun) noteMiss(domain string, err error) {
	m := r.miss[domain]
	m.last = err
	if errs.CodeOf(err) == errs.CodeDetectBlocked && m.blocked == nil {
		m.blocked = err
	}
	r.miss[domain] = m
}

// note 记下这个域名这一轮的最优地址，并把它从"还没地址"那本账里划掉。
func (r *detectRun) note(domain, ip string, ms int64, spec resolve.Spec, role string) {
	r.best[domain] = ip
	r.lat[domain] = ms
	r.src[domain] = detectSrc{spec: spec, role: role}
	delete(r.miss, domain)
}

// residual 挑出该交回界面那一个错：按清单顺序看第一个还没地址的域名，
// 它要是见过阻断形状就说那一档（换 IP 无效），否则说它最后一个错。
func (r *detectRun) residual(item menu.Item) error {
	for _, d := range item.Domains {
		if r.best[d] != "" {
			continue
		}
		m := r.miss[d]
		if m.blocked != nil {
			return m.blocked
		}
		if m.last != nil {
			return m.last
		}
	}
	return errs.New(errs.CodeDetectEmpty, map[string]any{"itemId": item.ID}, "这条条目没有可用的解析方式")
}

// rankFor 向 resolve.Rank 要这一轮的主用与备用。它只读样本，不改任何状态。
func (s *Service) rankFor(menuID string) (resolve.Choice, error) {
	s.mu.Lock()
	in := resolve.RankInput{Candidates: s.applicable(menuID), Samples: s.sampleView()}
	s.mu.Unlock()
	return resolve.Rank(in)
}

// runMode 用一种方式把这条条目里**还没拿到地址**的域名各跑一遍：取候选 → 探测 → 记分，结果直接累积进 run。
//
// 判据的顺序一次都不许换（这是"两把尺子各管一层"唯一的物理形状）：
//   - 方式好坏看 resolve 那两把尺子（S 选主用、V 选备用），本函数不参与；
//   - 地址好坏只看 detect 给的 CanServe，方式自己说行不算数；
//   - 延迟取 detect 的 RTT，不是 Set.Took——后者是取清单花的时间，拿它填界面那一列，
//     用户按它换 IP 越换越错（第 6.1 节那句"为什么会错"）。
//
// 返回的 error 只有两类：预算用完（GAC-DETECT-005）与这一轮被叫停。这两种都不是某个域名的错，
// 是"这一轮不用再往下试了"，所以由 detectItem 直接把任务收尾。
// 一个域名取数失败／探不上，只记进 run.miss 然后接着跑下一个域名——它不该拖垮同一条条目里别的域名。
func (s *Service) runMode(ctx context.Context, t *task, spec resolve.Spec, item menu.Item,
	known map[string]string, run *detectRun, role string) error {

	r, ok := s.cfg.Modes.Resolver(spec.ID)
	if !ok {
		// 选优点名的方式在注册表里没有实现：这一轮它一个候选地址也给不出，如实按"候选为空"记在每个还没地址的域名上。
		dead := errs.New(errs.CodeDetectEmpty, map[string]any{"mode": spec.ID},
			"方式 %s 在注册表里没有对应实现", spec.ID)
		for _, d := range item.Domains {
			if run.best[d] == "" {
				run.noteMiss(d, dead)
			}
		}
		return nil
	}

	for _, domain := range item.Domains {
		if run.best[domain] != "" {
			continue // 上一档已经给它选出地址了：同一域名不必再花一次预算
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := t.round.wantFetch(); err != nil {
			return err
		}
		started := time.Now()
		set, err := r.Resolve(ctx, resolve.Query{Domain: domain, Known: knownIPs(known, domain)})
		if err != nil {
			// 取数本身失败也是一轮样本，必须记：否则一个每次都慢到掐表的方式能一直赖在名单里
			//（resolve 那边 ctxError 把"叫停"与"太慢"分开就是为了让这一句判得准）。
			s.recordSample(spec.ID, resolve.Sample{ModeID: spec.ID, CanServe: false, Took: time.Since(started)})
			run.noteMiss(domain, err)
			continue
		}
		took := set.Took
		if len(set.IPs) == 0 {
			s.recordSample(spec.ID, resolve.Sample{ModeID: spec.ID, CanServe: false, Took: took})
			run.noteMiss(domain, errs.New(errs.CodeDetectEmpty, map[string]any{
				"mode": spec.ID, "domain": domain,
			}, "方式 %s 这条域名一个候选地址也没给回", spec.ID))
			continue
		}

		if !spec.NeedsProbe {
			// 不需要探测的方式（今天只有固定屏蔽）：地址是它钉死的，没有"能不能服务"这一说。
			// 所以它不记样本——它本来就不参与选优（Rankable 为假），记进去反而去污染 S 与 V。
			run.note(domain, set.IPs[0], 0, spec, role)
			continue
		}

		reserved := len(set.IPs) * (1 + resolve.RTTRepeats)
		if err := t.round.wantProbes(len(set.IPs)); err != nil {
			return err
		}
		res := s.cfg.Inspector.Check(ctx, domain, set.IPs, probeLimits())
		t.round.usedProbes(reserved, res.ProbeCount)
		s.recordSample(spec.ID, resolve.Sample{ModeID: spec.ID, CanServe: res.Best != "", Took: took})
		if res.Best == "" {
			run.noteMiss(domain, errs.New(res.Code, res.Params,
				"方式 %s 这条域名的候选地址全都不能服务", spec.ID))
			continue
		}
		run.note(domain, res.Best, res.RTT.Milliseconds(), spec, role)
	}
	return nil
}

// commitDetect 把这一轮累积出来的最优地址落到这条条目的内存态上，并推一条"单项检测完成"。
//
// 为什么延迟与来源只记一个域名的：契约里 Item 一行只有一个 latency／bestIp（原型 :1895-1896 那两列一个数），
// 而一条条目里几个域名各有自己的最优地址、也可能来自不同的方式。这一列说的是"首要域名"那一个，
// 而首要域名取的是清单顺序里第一个这一轮有了地址的（一个都没有才退回第一个域名）——
// 否则那个域名恰好没检出的条目，界面上会显示一个空地址却被标成"已检测"。
// 这条形状限制写在 state.go 的 itemState 上，不许在这里偷偷改成"取最快的那个"。
func (s *Service) commitDetect(item menu.Item, run *detectRun) {
	domain := headlineDomain(item, run.best)
	src := run.src[domain]
	now := time.Now().UTC().Format(time.RFC3339)

	s.mu.Lock()
	st := s.itemStateOf(item.ID)
	st.bestIPs = run.best
	st.source = src.spec.Title
	st.sourceRole = src.role
	st.latency = run.lat[domain]
	st.lastCheck = now
	st.errCode, st.errParams, st.rolledback = "", nil, false
	s.mu.Unlock()

	s.publish(contract.EventDetectProgress, map[string]any{
		"itemId": item.ID, "domain": domain, "mode": src.spec.ID, "role": src.role,
	})
}

// knownIPs 取出上一轮确认过能服务这个域名的地址，交给方式当"已知"提示。
//
// 为什么要由外面喂而不是方式自己去拿：第 8 节第 8 条管住 resolve 不许 import 存样本的地方，
// 样本归 service 管，这一层只吃参数。
func knownIPs(known map[string]string, domain string) []string {
	if ip := known[domain]; ip != "" {
		return []string{ip}
	}
	return nil
}

func copyIPs(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
