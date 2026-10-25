package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/detect"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/menu"
	"github.com/hlep/github-accel/internal/resolve"
)

// detect_test.go 钉的是"两层各管一把尺子"这条链：选方式看稳定性与耗时，选地址看能不能服务与握手多快。
//
// 干什么用：界面上「IP 来源」那一行、备用顶上时那一档、以及"疑似被阻断别再重试"这三句实话，
// 全部由这里那几条用例背书。
// 什么情况下会错、出错时人看得见什么：主用失败就把任务判死 → 只有一种来源的条目永远修不好；
// 备用顶上却写"主用" → 那一行说假话；被阻断还换备用重试 → 用户看到的是转圈。
// 怎么回来：三条都改回 detectrun.go 那一个循环，不许在 detect 或 resolve 里加判定。

// runDetect 提交一条检测并跑完，回那一行任务。
func (st *stack) runDetect(itemID string) contract.Task {
	st.t.Helper()
	st.start()
	batch, err := st.svc.submit(contract.KindDetect, []string{itemID}, originUser, nil)
	if err != nil {
		st.t.Fatalf("入队失败：%v", err)
	}
	st.waitIdle()
	tks := st.svc.Snapshot().Tasks
	if len(tks) != 1 {
		st.t.Fatalf("批次 %s 里有 %d 行", batch, len(tks))
	}
	return tks[0]
}

func TestDetectFallsBackToBackupSource(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	menuID := menuOf(t, st.catalog, item.ID)

	// 甲排第一（DefaultPriority 小），但它一个地址也不给；乙顶上。
	first := st.mode("甲", "用例甲", menuID, 1)
	first.err = errs.New(errs.CodeDetectEmpty, nil, "甲这一轮交不回地址")
	st.mode("乙", "用例乙", menuID, 2, "5.6.7.8")
	st.insp.fallback = detect.Result{Best: "5.6.7.8", RTT: 30 * time.Millisecond}

	tk := st.runDetect(item.ID)
	if tk.Status != contract.TaskDone {
		t.Fatalf("备用顶上那一次判成了 %s（%s）", tk.Status, tk.LastErr)
	}
	got := st.statusOf(item.ID)
	if got.BestIp != "5.6.7.8" {
		t.Fatalf("最优地址是 %q，要的是备用给的那个", got.BestIp)
	}
	if got.Source != "用例乙" {
		t.Fatalf("「IP 来源」写的是 %q：备用顶上却记成主用，那一行就在说假话", got.Source)
	}
	if got.SourceRole != contract.SourceFallback {
		t.Fatalf("来源身份是 %q，要的是 %q", got.SourceRole, contract.SourceFallback)
	}
	if got.Latency != 30 {
		t.Fatalf("延迟是 %d 毫秒，探测器给的是 30", got.Latency)
	}
	if got.Status != contract.ItemDetected {
		t.Fatalf("条目那一档是 %s，检过没写应该显示已检测", got.Status)
	}
	// 甲这个方式每次都报错，而一条条目里几个域名各追各的：所以它被叫的次数就是域名数，不是一次。
	if first.count() != len(item.Domains) {
		t.Fatalf("失败的主用被跑了 %d 次，这条条目有 %d 个域名", first.count(), len(item.Domains))
	}
}

func TestDetectKeepsOneAddressPerDomain(t *testing.T) {
	st := newStack(t)
	// 找一条真带两个域名的条目：一条条目里几个域名各有自己的最优地址，这是写入形状的地基。
	item := multiDomainItem(t, st)
	menuID := menuOf(t, st.catalog, item.ID)
	st.mode("甲", "用例甲", menuID, 1, "1.1.1.1", "2.2.2.2")
	st.insp.byDomain[item.Domains[0]] = detect.Result{Best: "1.1.1.1", RTT: 10 * time.Millisecond}
	st.insp.byDomain[item.Domains[1]] = detect.Result{Best: "2.2.2.2", RTT: 99 * time.Millisecond}
	st.insp.fallback = detect.Result{Best: "9.9.9.9", RTT: time.Millisecond}

	if tk := st.runDetect(item.ID); tk.Status != contract.TaskDone {
		t.Fatalf("检测判成 %s（%s）", tk.Status, tk.LastErr)
	}

	st.svc.mu.Lock()
	best := copyIPs(st.svc.items[item.ID].bestIPs)
	st.svc.mu.Unlock()
	if best[item.Domains[0]] != "1.1.1.1" || best[item.Domains[1]] != "2.2.2.2" {
		t.Fatalf("几个域名共用了一个地址：%v", best)
	}
	// 界面上那一行只有一个数（契约里 Item 一行一个 bestIp），它给的是首要域名那一个。
	if got := st.statusOf(item.ID).BestIp; got != "1.1.1.1" {
		t.Fatalf("那一行显示的是 %q，要的是首要域名的地址", got)
	}
	// 延迟同理：取的是首要域名那支的握手延迟，不是"这一条里最快的那个"。
	if got := st.statusOf(item.ID).Latency; got != 10 {
		t.Fatalf("延迟列是 %d 毫秒，要的是首要域名那支的 10", got)
	}
}

// multiDomainItem 取一条真带两个以上域名的条目。
func multiDomainItem(t *testing.T, st *stack) menu.Item {
	t.Helper()
	bad := st.badValues()
	for _, m := range st.catalog.Menus() {
		for _, it := range m.Items {
			if len(it.Domains) >= 2 && !st.hasConflict(it) && len(bad[it.ID]) == 0 {
				return it
			}
		}
	}
	t.Fatal("清单里没有一条带两个域名的条目，用例形状得改")
	return menu.Item{}
}

// 阻断那一档要等备用也试过才下结论。依据是 2026-10-22 本机实测：系统解析回来的地址是脏的，
// 探测形状正好落进"超时＝阻断"那张表——主用一失败就停，等于把最需要修的条目判成"换 IP 无效"。
func TestDetectBlockedConcludedOnlyAfterBackup(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	menuID := menuOf(t, st.catalog, item.ID)
	st.mode("甲", "用例甲", menuID, 1, "1.1.1.1")
	backup := st.mode("乙", "用例乙", menuID, 2, "2.2.2.2")
	// 两种方式都判"疑似被针对性阻断"：这时候才许对界面说换 IP 无效。
	st.insp.fallback = detect.Result{
		Code:   errs.CodeDetectBlocked,
		Params: map[string]any{"domain": item.Domains[0]},
	}

	tk := st.runDetect(item.ID)
	if tk.Status != contract.TaskFailed {
		t.Fatalf("被阻断那一次判成 %s", tk.Status)
	}
	if got := st.statusOf(item.ID).ErrorCode; got != errs.CodeDetectBlocked {
		t.Fatalf("条目上留的码是 %q，要的是 GAC-DETECT-004", got)
	}
	if backup.count() != len(item.Domains) {
		t.Fatalf("备用跑了 %d 次，这条条目有 %d 个域名，要的是每个域名一次", backup.count(), len(item.Domains))
	}
	if st.insp.count() != 2*len(item.Domains) {
		t.Fatalf("探测器被叫了 %d 次，要的是主用＋备用各 %d 次", st.insp.count(), len(item.Domains))
	}
}

// 主用被判阻断、备用换一种来源就能选出通得过的地址：这一档必须救回来，且「IP 来源」写备用。
func TestDetectBlockedPrimaryIsRescuedByBackup(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	menuID := menuOf(t, st.catalog, item.ID)
	primary := st.mode("甲", "用例甲", menuID, 1, "1.1.1.1")
	st.mode("乙", "用例乙", menuID, 2, "2.2.2.2")
	// 主用那一批判阻断，备用那一批正常选出地址。
	// 假件按"第几次被叫"翻脸：探测器只看域名不看方式，不这么钉分不出"第一次失败、第二次救回来"。
	st.insp.byDomain[item.Domains[0]] = detect.Result{
		Code:   errs.CodeDetectBlocked,
		Params: map[string]any{"domain": item.Domains[0]},
	}
	st.insp.fallback = detect.Result{Best: "2.2.2.2", RTT: 2 * time.Millisecond}
	st.insp.before = func(n int) {
		if n == 1 {
			delete(st.insp.byDomain, item.Domains[0])
		}
	}

	tk := st.runDetect(item.ID)
	if tk.Status != contract.TaskDone {
		t.Fatalf("备用救回来了却判成 %s", tk.Status)
	}
	got := st.statusOf(item.ID)
	if got.BestIp != "2.2.2.2" || got.SourceRole != contract.SourceFallback {
		t.Fatalf("救回来之后来源写错：%+v", got)
	}
	// 主用那个域名被判阻断，其余域名照常由主用选出地址：所以主用被叫的次数就是域名数。
	if primary.count() != len(item.Domains) {
		t.Fatalf("主用跑了 %d 次，这条条目有 %d 个域名", primary.count(), len(item.Domains))
	}
}

// 一个域名拖垮整条条目＝2026-10-22 本机实测到的毛病（gist.github.com 六十支候选一支也不答，
// github.com 与 api.github.com 却通）。检得出地址的那几个必须留下，检不出的那个如实报：
// 任务照样失败（那一行要说"没全成"），但已经到手的地址不许跟着一起丢。
func TestDetectKeepsAddressesFromEveryDomainThatAnswered(t *testing.T) {
	st := newStack(t)
	item := multiDomainItem(t, st)
	menuID := menuOf(t, st.catalog, item.ID)
	dead := item.Domains[len(item.Domains)-1]

	st.mode("甲", "用例甲", menuID, 1, "1.1.1.1")
	st.insp.byDomain[dead] = detect.Result{
		Code:   errs.CodeDetectBlocked,
		Params: map[string]any{"domain": dead},
	}
	st.insp.fallback = detect.Result{Best: "2.2.2.2", RTT: 5 * time.Millisecond}

	tk := st.runDetect(item.ID)
	if tk.Status != contract.TaskFailed {
		t.Fatalf("还有一个域名没地址却判成 %s：那一行要说的是\"没全成\"", tk.Status)
	}
	got := st.statusOf(item.ID)
	if got.ErrorCode != errs.CodeDetectBlocked {
		t.Fatalf("条目上留的码是 %q，要的是没地址那个域名的 GAC-DETECT-004", got.ErrorCode)
	}
	if got.Status != contract.ItemDetected {
		t.Fatalf("条目那一档是 %s：地址还在，该回到\"已检测\"而不是\"出错\"", got.Status)
	}
	// 那一列地址说的是第一个有地址的域名，不是清单里排第一却检不出的那一个。
	if got.BestIp != "2.2.2.2" || got.Latency != 5 {
		t.Fatalf("那一行显示 %q／%d 毫秒", got.BestIp, got.Latency)
	}

	st.svc.mu.Lock()
	best := copyIPs(st.svc.items[item.ID].bestIPs)
	st.svc.mu.Unlock()
	for _, d := range item.Domains {
		if d == dead {
			if best[d] != "" {
				t.Fatalf("检不出的那个域名也被记了地址 %q：界面上就没法说清它没加速成", best[d])
			}
			continue
		}
		if best[d] != "2.2.2.2" {
			t.Fatalf("%s 的最优地址没留下（%v）：一个死域名把整条条目的结果一起丢了", d, best)
		}
	}
}

// 样本只由"要探测的方式"交回：固定屏蔽那类钉死地址的方式不参与选优，记进去反而污染两把尺子。
func TestSamplesRecordedOnlyForProbeModes(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	menuID := menuOf(t, st.catalog, item.ID)

	st.modes.add(resolve.Spec{ID: "甲", Title: "用例甲", Kinds: []string{menuID},
		NeedsProbe: true, DefaultPriority: 1, Rankable: true}, "1.1.1.1")
	st.insp.fallback = detect.Result{Best: "1.1.1.1", RTT: time.Millisecond}
	st.runDetect(item.ID)
	if len(st.svc.sampleView()["甲"]) == 0 {
		t.Fatal("要探测的方式一条样本也没记：S 与 V 从此没有原料")
	}

	st2 := newStack(t)
	item2 := st2.cleanItem(t)
	st2.modes.add(resolve.Spec{ID: "乙", Title: "用例乙", Kinds: []string{menuOf(t, st2.catalog, item2.ID)},
		NeedsProbe: false, DefaultPriority: 1, Rankable: false}, "127.0.0.1")
	st2.runDetect(item2.ID)
	if got := len(st2.svc.sampleView()["乙"]); got != 0 {
		t.Fatalf("不参与选优的方式记了 %d 条样本，S 的分母被它污染了", got)
	}
	if got := st2.statusOf(item2.ID).BestIp; got != "127.0.0.1" {
		t.Fatalf("免探测的方式交回的是 %q", got)
	}
	if st2.insp.count() != 0 {
		t.Fatal("免探测的方式还是被探了：NeedsProbe 这条开关没接上")
	}
}

// 方式选出来了但注册表里没有它的实现：如实报"候选为空"，不许静默换人。
func TestSelectedModeWithoutImplementationIsReported(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.modes.specs = append(st.modes.specs, resolve.Spec{
		ID: "幽灵", Title: "注册表里没有实现", Kinds: []string{menuOf(t, st.catalog, item.ID)},
		NeedsProbe: true, DefaultPriority: 1, Rankable: true,
	})

	tk := st.runDetect(item.ID)
	if tk.Status != contract.TaskFailed {
		t.Fatalf("那一行是 %s，选到没实现的方式必须失败", tk.Status)
	}
	if got := st.statusOf(item.ID).ErrorCode; got != errs.CodeDetectEmpty {
		t.Fatalf("留的码是 %q，要的是 GAC-DETECT-001", got)
	}
}

// 一块菜单一个方式也没有：当场回候选为空，界面那一行立刻显示原因，不许转圈。
func TestDetectWithoutAnyModeFailsImmediately(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	tk := st.runDetect(item.ID)
	if tk.Status != contract.TaskFailed {
		t.Fatalf("没有参选方式却判成 %s", tk.Status)
	}
	if got := st.statusOf(item.ID).ErrorCode; got != errs.CodeDetectEmpty {
		t.Fatalf("留的码是 %q，要的是 GAC-DETECT-001", got)
	}
	if st.insp.count() != 0 {
		t.Fatal("一个方式都没有还去探测了")
	}
}

// 预算那本账按"次"记，预留最坏那一档、跑完退回差额。
func TestRoundBudgetReservesWorstCaseAndRefunds(t *testing.T) {
	r := &roundState{}
	reserved := 2 * (1 + resolve.RTTRepeats)
	if err := r.wantProbes(2); err != nil {
		t.Fatalf("两次候选的预留就超预算：%v", err)
	}
	if r.plan.Probes != reserved {
		t.Fatalf("预留记成 %d，按最坏那一档该是 %d", r.plan.Probes, reserved)
	}
	// 只有一支答了话（另一支当场超时），差额必须退回，否则配额虚耗。
	r.usedProbes(reserved, 1)
	if r.plan.Probes != 1 {
		t.Fatalf("退完差额还剩 %d 次，真发出去的只有 1 次", r.plan.Probes)
	}

	r2 := &roundState{plan: resolve.RoundPlan{Fetches: resolve.FetchesPerRound}}
	if err := r2.wantFetch(); errs.CodeOf(err) != errs.CodeDetectBudget {
		t.Fatalf("取数已经到上限却回了 %v，要的是 GAC-DETECT-005", err)
	}
	r3 := &roundState{plan: resolve.RoundPlan{Probes: resolve.ProbesPerRound}}
	if err := r3.wantProbes(1); errs.CodeOf(err) != errs.CodeDetectBudget {
		t.Fatalf("探测到上限却回了 %v", err)
	}
}

// 一轮里真把预算跑满：撞上限的那些行要标成"本轮没轮到"（已跳过），而不是把条目涂成"出错"。
// 2026-10-22 本机实测到的正是涂红那一档：整张清单一次跑完要 160 次取数／591 次探测，
// 上限比这小时，38 条带着 GAC-DETECT-005 显示成"出错"，用户看到的是"这工具把 38 条域名测坏了"，
// 实际上一行都没测完。所以这边钉三件事：条目一个码都不留、任务行留原因、发出去的事不超上限。
func TestFullRoundTruncatesAtBudget(t *testing.T) {
	st := newStack(t)
	// 这一条要跑满一整轮，所以让这些方式适用全部菜单块（Kinds 不给全，其余那块菜单根本轮不到花预算）。
	var kinds []string
	slots := 0
	for _, m := range st.catalog.Menus() {
		kinds = append(kinds, m.ID)
		for _, it := range m.Items {
			slots += len(it.Domains)
		}
	}
	if slots == 0 {
		t.Fatal("清单里一个域名槽都没有，这条用例没有对手方")
	}
	// 一个域名槽在一档方式里花掉：一次取数，外加两个候选各 1＋RTTRepeats 次探测。
	// 方式数按这两个数现算，好让"需求确实超过两个上限"这件事跟着 budget.go 走——
	// 以后调那两个数，这条用例自己会多造几档，不必回来重抄一遍算术。
	perSlotProbes := 2 * (1 + resolve.RTTRepeats)
	ladder := 1
	for slots*ladder <= resolve.FetchesPerRound || slots*ladder*perSlotProbes <= resolve.ProbesPerRound {
		ladder++
		if ladder > 16 {
			t.Fatalf("要造到 %d 档方式才顶得上限，这本账已经不成形状", ladder)
		}
	}
	ladder++

	var resolvers []*fakeResolver
	for i := 0; i < ladder; i++ {
		resolvers = append(resolvers, st.modes.add(resolve.Spec{
			ID: fmt.Sprintf("甲%d", i), Title: fmt.Sprintf("用例甲%d", i), Kinds: kinds,
			NeedsProbe: true, DefaultPriority: 10 + i, Rankable: true,
		}, "1.1.1.1", "1.1.1.2"))
	}
	// 每一档全都探不上：每个域名槽因此要把名单走到底，预算才会真被花满。
	st.insp.fallback = detect.Result{Code: errs.CodeDetectNoServe}

	st.start()
	if _, err := st.svc.submit(contract.KindDetect, nil, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()

	snap := st.svc.Snapshot()
	var skipped, coded int
	for _, tk := range snap.Tasks {
		if tk.Status == contract.TaskSkipped && strings.Contains(tk.LastErr, "预算") {
			skipped++
			// 契约 v8：这一行的"为什么"要带码与参数，界面才能把这句话凑成用户那门语言。
			// 少了这一档，屏幕上出现的是"条目那列一句人话、队列那列一句服务侧原文"——同一件事两种说法。
			if tk.ErrorCode != errs.CodeDetectBudget {
				t.Errorf("「本轮没轮到」那一行留的码是 %q，要的是 %s", tk.ErrorCode, errs.CodeDetectBudget)
			}
			if len(tk.ErrorParams) == 0 {
				t.Errorf("「本轮没轮到」那一行只有码没有参数，界面凑不出\"花了多少次／上限多少次\"那句话：%+v", tk.ErrorParams)
			}
		}
	}
	for _, it := range snap.Items {
		if it.ErrorCode == errs.CodeDetectBudget {
			coded++
		}
	}
	if skipped == 0 {
		t.Fatal("一条也没标成\"本轮没轮到\"：上限到了还是把行涂红了？")
	}
	if coded != 0 {
		t.Errorf("%d 条条目带着 GAC-DETECT-005：那是\"这一轮不用再往下试\"，不是这一条坏了，不该进条目那一档", coded)
	}
	if !strings.Contains(snap.State.LastError, "预算") {
		t.Errorf("状态栏那句\"剩下的排队\"没发出去，实际是 %q", snap.State.LastError)
	}
	// 契约 v9：状态栏那一句同样要带码与参数。少了这两栏，界面上队列那一行说的是用户那门语言、
	// 状态栏这一格还是服务侧原文——同一屏两种说法，看的人以为工具坏了。
	if snap.State.LastErrorCode != errs.CodeDetectBudget {
		t.Errorf("状态栏带的码是 %q，要的是 %s", snap.State.LastErrorCode, errs.CodeDetectBudget)
	}
	if len(snap.State.LastErrorParams) == 0 {
		t.Errorf("状态栏只有码没有参数，界面凑不出\"上限多少次／已花多少次\"：%+v", snap.State.LastErrorParams)
	}

	// 发出去的事不许越过上限：取数按次数直接比，探测按每次预留的那一档折回去比。
	var fetches int
	for _, r := range resolvers {
		fetches += r.count()
	}
	if fetches > resolve.FetchesPerRound {
		t.Errorf("一轮真发了 %d 次取数，上限是 %d", fetches, resolve.FetchesPerRound)
	}
	if got := st.insp.count() * perSlotProbes; got > resolve.ProbesPerRound {
		t.Errorf("一轮真发了 %d 次探测，上限是 %d", got, resolve.ProbesPerRound)
	}
}

// 名单前三档都要走到：本机实测到的形状正是"第一档冷启动空手、第二档被判阻断、第三档才有真地址"。
// 只走两档的话，这台机器上一条 GitHub 条目也修不好（见 detectrun.go 上面第 ③ 条）。
func TestDetectWalksWholeLadder(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	menuID := menuOf(t, st.catalog, item.ID)

	first := st.mode("甲", "用例甲", menuID, 1) // 空手：一个地址也不给
	first.err = errs.New(errs.CodeDetectEmpty, nil, "甲这一轮交不回地址")
	second := st.mode("乙", "用例乙", menuID, 2, "2.2.2.2") // 交回一支，探不上，形状判成阻断
	third := st.mode("丙", "用例丙", menuID, 3, "3.3.3.3")  // 这一档才是通的
	st.insp.byDomain[item.Domains[0]] = detect.Result{
		Code:   errs.CodeDetectBlocked,
		Params: map[string]any{"domain": item.Domains[0]},
	}
	st.insp.fallback = detect.Result{Best: "3.3.3.3", RTT: 4 * time.Millisecond}
	st.insp.before = func(n int) {
		if n == 1 {
			delete(st.insp.byDomain, item.Domains[0])
		}
	}

	tk := st.runDetect(item.ID)
	if tk.Status != contract.TaskDone {
		t.Fatalf("第三档救回来了却判成 %s", tk.Status)
	}
	got := st.statusOf(item.ID)
	if got.BestIp != "3.3.3.3" || got.Source != "用例丙" {
		t.Fatalf("最优/来源写错：%+v", got)
	}
	if got.SourceRole != contract.SourceFallback {
		t.Fatalf("第三档顶上却写成 %q：那一行要说清它不是主用", got.SourceRole)
	}
	// 一条条目里每个域名各取一次地址、各探一次，所以"跑了几次"要按域名数折算：
	// 甲每次都空手（每个域名一次），乙取数正常但第一个域名被判阻断（其余两个域名照常选出地址），
	// 丙只补乙没救回的那一个域名。
	if first.count() != len(item.Domains) || second.count() != len(item.Domains) || third.count() != 1 {
		t.Fatalf("三档取的次数是 %d / %d / %d，要的是 %d / %d / 1",
			first.count(), second.count(), third.count(), len(item.Domains), len(item.Domains))
	}
	if st.insp.count() != len(item.Domains)+1 {
		t.Fatalf("探测器被叫了 %d 次，要的是乙每个域名一次（%d）再加上丙那一次",
			st.insp.count(), len(item.Domains))
	}
}
