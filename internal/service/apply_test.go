package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/detect"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/hosts"
	"github.com/hlep/github-accel/internal/menu"
)

// apply_test.go 钉的是写入那一段：四个阶段一个不少、动手之前先拦不合格的数据、
// 以及"写成了但该知道的那几句"（刷 DNS 没成、备份没存下）不许把任务判死。
//
// 干什么用：这一组用例是"点应用到底有没有生效"这句话的全部依据。
// 什么情况下会错、出错时人看得见什么：红了之后界面上出现的是"显示已生效但浏览器还是旧的"
// 或"两条条目互相把对方的行盖掉"——两种都是用户最没法自己发现的那类。
// 怎么回来：改 applyrun.go 那四个阶段与那两道前置检查，hosts 那五步另有它自己那一份用例。

// seedBest 直接给这一条填上"最近一轮检出的最优地址"，让写入那一段能单独跑。
// 检测自己那条链由 detect_test.go 钉，不在这里重跑一遍网络。
func (st *stack) seedBest(item menu.Item, ip string) {
	st.t.Helper()
	st.svc.mu.Lock()
	st.svc.itemStateOf(item.ID).bestIPs = map[string]string{}
	for _, d := range item.Domains {
		st.svc.itemStateOf(item.ID).bestIPs[d] = ip
	}
	st.svc.mu.Unlock()
}

// seedBestSome 只给这一条的一部分域名填地址，剩下那一个当作"这一轮没检出"（本机实测的形状）。
func (st *stack) seedBestSome(item menu.Item, ip string, without string) {
	st.t.Helper()
	st.svc.mu.Lock()
	defer st.svc.mu.Unlock()
	b := map[string]string{}
	for _, d := range item.Domains {
		if d != without {
			b[d] = ip
		}
	}
	st.svc.itemStateOf(item.ID).bestIPs = b
}

func (st *stack) run(kind string, itemIDs []string, picks map[string]string) {
	st.t.Helper()
	st.start()
	if _, err := st.svc.submit(kind, itemIDs, originUser, picks); err != nil {
		st.t.Fatalf("入队失败：%v", err)
	}
	st.waitIdle()
}

func TestApplyWritesEveryDomainAndPassesFourPhases(t *testing.T) {
	st := newStack(t)
	item := multiDomainItem(t, st)
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, nil)

	tk := st.svc.Snapshot().Tasks[0]
	phases := st.events.phasesOf(tk.ID)
	want := []string{contract.PhaseBackup, contract.PhaseWrite, contract.PhaseFlush, contract.PhaseVerify}
	if len(phases) != len(want) {
		t.Fatalf("推进了 %v，要的是 %v", phases, want)
	}
	for i, p := range want {
		if phases[i] != p {
			t.Fatalf("第 %d 个阶段是 %q，要的是 %q", i+1, phases[i], p)
		}
	}
	if tk.Status != contract.TaskDone {
		t.Fatalf("这一行是 %s（%s）", tk.Status, tk.LastErr)
	}

	// 文件里每个域名各有一行，且都带着这一条条目的标记。
	raw := readFile(t, st.itemsPath)
	for _, d := range item.Domains {
		if !strings.Contains(raw, "1.2.3.4 "+d) {
			t.Fatalf("文件里没有这一行：\n%s", raw)
		}
	}
	if len(mineFor(hosts.Parse(raw), item.ID)) != len(item.Domains) {
		t.Fatalf("写进去的行没带上这一条条目的标记，恢复时认不出是谁写的：\n%s", raw)
	}
	if st.flush.count() == 0 {
		t.Fatal("写完没刷 DNS")
	}

	got := st.statusOf(item.ID)
	if got.Status != contract.ItemApplied || got.AppliedIp != "1.2.3.4" {
		t.Fatalf("界面上那一行是 %s／%q", got.Status, got.AppliedIp)
	}
}

// 一个域名有地址、另一个没有：有地址的照写，没地址的那个一个字不碰，但那一句要留在状态栏里。
// 依据是 2026-10-22 本机实测——gist.github.com 检不出地址，github.com 与 api.github.com 检得出。
// 整条一起不写才是这台机器上最要命的毛病：用户点一次应用，结果一个字没变。
func TestApplyWritesOnlyDomainsWithAddress(t *testing.T) {
	st := newStack(t)
	item := multiDomainItem(t, st)
	dead := item.Domains[len(item.Domains)-1]
	st.seedBestSome(item, "1.2.3.4", dead)

	st.run(contract.KindApply, []string{item.ID}, nil)

	tk := st.svc.Snapshot().Tasks[0]
	if tk.Status != contract.TaskDone {
		t.Fatalf("这一行是 %s（%s）：能写的都写了，不该判死", tk.Status, tk.LastErr)
	}
	raw := readFile(t, st.itemsPath)
	for _, d := range item.Domains {
		if d == dead {
			if strings.Contains(raw, d) {
				t.Fatalf("没地址的那个域名也被写进文件了：\n%s", raw)
			}
			continue
		}
		if !strings.Contains(raw, "1.2.3.4 "+d) {
			t.Fatalf("有地址的域名没写上：\n%s", raw)
		}
	}
	if n := len(mineFor(hosts.Parse(raw), item.ID)); n != len(item.Domains)-1 {
		t.Fatalf("文件里这一条有 %d 行，要的是 %d 行", n, len(item.Domains)-1)
	}
	if got := st.statusOf(item.ID); got.Status != contract.ItemApplied {
		t.Fatalf("那一档是 %s：写进去的那几行就是这一条现在的样子", got.Status)
	}
	if st.svc.Snapshot().State.LastError == "" {
		t.Fatal("状态栏里没留那一句：用户看不出这一条只加速了一半")
	}
}

// 用户原来那一行要留成 prev，恢复才有依据；这条是"还原不丢用户内容"的地基。
func TestApplyKeepsPreviousAddressInMarker(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	domain := item.Domains[0]
	writeFile(t, st.itemsPath, "10.0.0.1 "+domain+"\n")
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, nil)

	got := st.statusOf(item.ID)
	if got.PrevAppliedIp != "10.0.0.1" {
		t.Fatalf("应用前那个地址没进快照：%q", got.PrevAppliedIp)
	}
	if got.AppliedIp != "1.2.3.4" {
		t.Fatalf("当前地址是 %q", got.AppliedIp)
	}
}

// 点应用但手上还没地址：同一次任务里先补一轮检测，不能让用户多点一次。
func TestApplyDetectsFirstWhenNoAddress(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	menuID := menuOf(t, st.catalog, item.ID)
	st.mode("甲", "用例甲", menuID, 1, "1.2.3.4")
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}

	st.run(contract.KindApply, []string{item.ID}, nil)

	if st.insp.count() == 0 {
		t.Fatal("没检测就直接谈应用：这一条根本没跑过检测")
	}
	if got := st.statusOf(item.ID); got.Status != contract.ItemApplied {
		t.Fatalf("补完检测还是没写上：%s／%q", got.Status, got.AppliedIp)
	}
}

// 没地址又检测不出来：如实失败，回 GAC-DETECT-001，不许写一个空串进 hosts。
func TestApplyWithoutAnyAddressRefusesToWrite(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.run(contract.KindApply, []string{item.ID}, nil)

	if raw := readFile(t, st.itemsPath); strings.Contains(raw, item.Domains[0]) {
		t.Fatalf("一条地址都没有却动了文件：\n%s", raw)
	}
	got := st.statusOf(item.ID)
	if got.ErrorCode != errs.CodeDetectEmpty {
		t.Fatalf("留的码是 %q", got.ErrorCode)
	}
}

// 点应用时补的那一轮检测只救回一半：救回来的那几个域名照写，别把这一条判成"没改成"。
// 这一条钉的是 applyItem 里"检测报错但手上已经有地址"那一格——红了就是"点了应用却一个字没变"。
func TestApplyDetectingFirstKeepsPartialResults(t *testing.T) {
	st := newStack(t)
	item := multiDomainItem(t, st)
	dead := item.Domains[len(item.Domains)-1]
	menuID := menuOf(t, st.catalog, item.ID)
	st.mode("甲", "用例甲", menuID, 1, "1.2.3.4")
	st.insp.byDomain[dead] = detect.Result{
		Code:   errs.CodeDetectBlocked,
		Params: map[string]any{"domain": dead},
	}
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}

	st.run(contract.KindApply, []string{item.ID}, nil)

	raw := readFile(t, st.itemsPath)
	if strings.Contains(raw, dead) {
		t.Fatalf("没检出地址的那个域名也被写进文件了：\n%s", raw)
	}
	written := len(mineFor(hosts.Parse(raw), item.ID))
	if written != len(item.Domains)-1 {
		t.Fatalf("补一轮检测只写了 %d 行，这条条目有 %d 个域名、其中一个检不出",
			written, len(item.Domains))
	}
	if got := st.statusOf(item.ID); got.Status != contract.ItemApplied {
		t.Fatalf("那一档是 %s／%q", got.Status, got.AppliedIp)
	}
}

// 界面上选了某个候选地址：这一条就按那一个写，不再用自己那一份最优。
func TestApplyHonoursPickedAddress(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, map[string]string{item.ID: "8.8.8.8"})

	if raw := readFile(t, st.itemsPath); !strings.Contains(raw, "8.8.8.8 "+item.Domains[0]) {
		t.Fatalf("没按界面选的那个地址写：\n%s", raw)
	}
	if got := st.statusOf(item.ID); got.AppliedIp != "8.8.8.8" {
		t.Fatalf("那一行显示 %q", got.AppliedIp)
	}
}

// 一个域名挂在两条条目上：动手之前就拒，并把抢它的两条一起带回去（D5 甲）。
func TestApplyRefusesConflictingDomain(t *testing.T) {
	st := newStack(t)
	item := st.conflictItem(t)
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, nil)

	got := st.statusOf(item.ID)
	if got.ErrorCode != errs.CodeHostsConflict {
		t.Fatalf("留的码是 %q，要的是 GAC-HOSTS-007", got.ErrorCode)
	}
	owners, _ := got.ErrorParams["items"].([]string)
	if len(owners) < 2 {
		t.Fatalf("参数里没把抢它的两条带回去：%v", got.ErrorParams)
	}
	if raw := readFile(t, st.itemsPath); raw != "" {
		t.Fatalf("拒了却还是写了文件：\n%s", raw)
	}
}

// 数据本身不是域名形状：同样在动手之前拒，不静默跳过（D4 甲）。
func TestApplyRefusesBadValueEntry(t *testing.T) {
	st := newStack(t)
	item := st.badValueItem(t)
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, nil)

	got := st.statusOf(item.ID)
	if got.ErrorCode != errs.CodeHostsBadValue {
		t.Fatalf("留的码是 %q，要的是 GAC-HOSTS-008", got.ErrorCode)
	}
	if vals, _ := got.ErrorParams["values"].([]string); len(vals) == 0 {
		t.Fatalf("参数里没写是哪几个值：%v", got.ErrorParams)
	}
}

// 写完重读对不上＝引擎已经用备份还原过：那一档是"已回滚"，不是"没改成"。
func TestApplyVerifyMismatchMarksRolledback(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	eng := &fakeEngine{rollback: true}
	st.svc.cfg.Engine = eng
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, nil)

	tk := st.svc.Snapshot().Tasks[0]
	if tk.Status != contract.TaskRolledback {
		t.Fatalf("那一行是 %s，对不上要的是 rolledback", tk.Status)
	}
	got := st.statusOf(item.ID)
	if got.ErrorCode != errs.CodeHostsVerify {
		t.Fatalf("留的码是 %q，要的是 GAC-HOSTS-004", got.ErrorCode)
	}
	// 契约 v8：「已回滚」那一行也要带码——队列那一列得说得出"写完重读对不上、已经还原了"这句话，
	// 而不是把服务侧原文照抄上去（界面切语言时这一列会跟着别的列走，只有它不动）。
	if tk.ErrorCode != got.ErrorCode {
		t.Fatalf("行上留的码是 %q，条目上留的是 %q：同一件事两种说法", tk.ErrorCode, got.ErrorCode)
	}
	sameParams(t, tk.ErrorParams, got.ErrorParams)
	if got.Status != contract.ItemRolledback {
		t.Fatalf("条目那一档是 %q，要的是 rolledback", got.Status)
	}
	// 文件里一个字没有：那一列显示的是文件此刻的真实内容（D3 甲），不是"我本来想写的那个"。
	if got.AppliedIp != "" {
		t.Fatalf("回滚之后那一行还显示地址 %q", got.AppliedIp)
	}
	doc, err := eng.Read()
	if err != nil {
		t.Fatalf("假件读不出自己那份内容：%v", err)
	}
	if len(mineFor(doc, item.ID)) != 0 {
		t.Fatalf("还原之后文件里不该还有这一条的行：\n%s", doc.Render())
	}
}

// 刷 DNS 没成 ≠ 没写成：任务照样成功，但那一句要留在状态栏里。
func TestFlushFailureKeepsTaskSucceeded(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.seedBest(item, "1.2.3.4")
	st.flush.setErr(errs.New(errs.CodePermFlush, nil, "刷不动 Dnscache"))

	st.run(contract.KindApply, []string{item.ID}, nil)

	tk := st.svc.Snapshot().Tasks[0]
	if tk.Status != contract.TaskDone {
		t.Fatalf("刷 DNS 没成却把任务判死了：%s", tk.Status)
	}
	if got := st.statusOf(item.ID); got.Status != contract.ItemApplied {
		t.Fatalf("那一档是 %s，写成就该显示已生效", got.Status)
	}
	if st.svc.Snapshot().State.LastError == "" {
		t.Fatal("状态栏里没留那一句：用户只会以为浏览器不该是旧的")
	}
}

// 恢复只撤自己那几行，别的条目的行一个字不动——那是"不许私自改变其他功能"。
func TestRestoreRemovesOnlyThisItemLines(t *testing.T) {
	st := newStack(t)
	a := st.cleanItem(t)
	b := secondCleanItem(t, st, a.ID)
	st.seedBest(a, "1.2.3.4")
	st.seedBest(b, "5.6.7.8")

	st.start()
	if _, err := st.svc.submit(contract.KindApply, []string{a.ID, b.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()

	if _, err := st.svc.submit(contract.KindRestore, []string{a.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()

	raw := readFile(t, st.itemsPath)
	if strings.Contains(raw, "1.2.3.4 "+a.Domains[0]) {
		t.Fatalf("恢复之后这一条的行还在文件里：\n%s", raw)
	}
	if !strings.Contains(raw, "5.6.7.8 "+b.Domains[0]) {
		t.Fatalf("把别的条目的行一起撤掉了：\n%s", raw)
	}
	if got := st.statusOf(b.ID); got.Status != contract.ItemApplied {
		t.Fatalf("另一条变成 %s 了", got.Status)
	}
	if got := st.statusOf(a.ID); got.Status != contract.ItemDetected {
		t.Fatalf("撤掉之后这一条是 %s，地址还在，该回到\"已检测、未应用\"", got.Status)
	}
}

// 自己一行都没写过：恢复不该动文件，也不该为它备份、为它刷 DNS。
// 「恢复默认」是整份清单各排一行，这一条钉的就是那几十次无谓重写——那是拿用户的 hosts 凑行数。
func TestRestoreWithoutOwnLinesWritesNothing(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	other := secondCleanItem(t, st, item.ID)
	st.seedBest(other, "5.6.7.8")

	st.start()
	if _, err := st.svc.submit(contract.KindApply, []string{other.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()
	before := readFile(t, st.itemsPath)
	backupsBefore := backupCount(t, st.backupPath)

	if _, err := st.svc.submit(contract.KindRestore, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()

	if got := readFile(t, st.itemsPath); got != before {
		t.Fatalf("这一条一行都没写过，恢复却动了文件：\n%s", got)
	}
	if n := backupCount(t, st.backupPath); n != backupsBefore {
		t.Fatalf("备份从 %d 份变成 %d 份：没写文件那一次不该再存一份", backupsBefore, n)
	}
	if st.flush.count() != 1 {
		t.Fatalf("刷 DNS 跑了 %d 次，只有应用那一次该刷", st.flush.count())
	}
	if tk := st.restoreTask(item.ID); tk.Status != contract.TaskRestored {
		t.Fatalf("那一行是 %s：本来就没有要撤的东西，这就是成功", tk.Status)
	}
}

func backupCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("读备份目录 %s 失败：%v", dir, err)
	}
	return len(entries)
}

// 「恢复默认」＝撤掉全部托管：与单项恢复共用同一个执行体，参数决定范围。
func TestRestoreAllClearsEveryManagedLine(t *testing.T) {
	st := newStack(t)
	a := st.cleanItem(t)
	b := secondCleanItem(t, st, a.ID)
	st.seedBest(a, "1.2.3.4")
	st.seedBest(b, "5.6.7.8")

	st.start()
	if _, err := st.svc.submit(contract.KindApply, []string{a.ID, b.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()
	if _, err := st.svc.submit(contract.KindRestore, nil, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()

	raw := readFile(t, st.itemsPath)
	if strings.Contains(raw, "1.2.3.4 ") || strings.Contains(raw, "5.6.7.8 ") {
		t.Fatalf("「恢复默认」之后托管行还在：\n%s", raw)
	}
	// 只核自己那两条：清单里那几条抢同一个域名／值不是域名形状的条目，
	// 恢复同样会在动手之前被拒（那是 007、008 的用例），不该被这一条算成失败。
	for _, id := range []string{a.ID, b.ID} {
		tk := st.restoreTask(id)
		if tk.Status != contract.TaskRestored {
			t.Fatalf("恢复 %s 那一行是 %s（%s）", id, tk.Status, tk.LastErr)
		}
	}
}

// 恢复之后自己的行还在＝没撤干净，要如实报，不许标"已恢复"。
func TestRestoreVerifyLeftIsReported(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	// 引擎从一开始就是这一份"撤不动"的假件：中途换装体会绕开 New 里那些装配检查，
	// 而且队列跑起来之后再改 cfg 是数据竞争。
	eng := &fakeEngine{restoreNoop: true}
	st.svc.cfg.Engine = eng
	st.seedBest(item, "1.2.3.4")

	st.start()
	if _, err := st.svc.submit(contract.KindApply, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()
	if _, err := st.svc.submit(contract.KindRestore, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()

	tks := st.svc.Snapshot().Tasks
	tk := tks[len(tks)-1]
	if tk.Kind != contract.KindRestore {
		t.Fatalf("最后一行是 %s 类任务，用例没跑到恢复那一步", tk.Kind)
	}
	// 与"写完重读对不上"同一档：码就是 GAC-HOSTS-004，收尾那一步只认码。
	if tk.Status != contract.TaskRolledback {
		t.Fatalf("没撤干净却判成 %s", tk.Status)
	}
	if got := st.statusOf(item.ID).ErrorCode; got != errs.CodeHostsVerify {
		t.Fatalf("留的码是 %q，要的是 GAC-HOSTS-004", got)
	}
	doc, err := eng.Read()
	if err != nil {
		t.Fatalf("假件读不出自己那份内容：%v", err)
	}
	if len(mineFor(doc, item.ID)) == 0 {
		t.Fatalf("假件里那份内容不该还有这一条条目的托管行：\n%s", doc.Render())
	}
}

// hostsMarker 把标记的前缀按字面写在这里：doc.go 里那一个是包私有的，用例够不着。
// 故意重复一次字面量——它改了而这里没改，下面那几条用例会当场红。
// mineFor 取出文档里属于这一条条目的托管行（"标记写对了没有"就看这几行认不认得出来）。
func mineFor(doc *hosts.Doc, itemID string) []hosts.Line {
	var out []hosts.Line
	for _, l := range doc.Managed() {
		if l.ItemID == itemID {
			out = append(out, l)
		}
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("读 %s 失败：%v", path, err)
	}
	return string(raw)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写 %s 失败：%v", path, err)
	}
}

// secondCleanItem 取第二条能走正常链路的条目（两条一起才看得出"撤自己别撤别人"）。
func secondCleanItem(t *testing.T, st *stack, exceptID string) menu.Item {
	t.Helper()
	bad := st.badValues()
	for _, m := range st.catalog.Menus() {
		for _, it := range m.Items {
			if it.ID == exceptID || st.hasConflict(it) || len(bad[it.ID]) > 0 {
				continue
			}
			return it
		}
	}
	t.Fatal("清单里找不出第二条可用条目")
	return menu.Item{}
}
