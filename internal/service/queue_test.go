package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/detect"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/resolve"
)

// queue_test.go 钉的是那条串行通道本身：一次点击变成几行、谁能插队、暂停和取消各管到哪一步。
//
// 干什么用：界面上"点下去有没有反应""进度条是不是真的"这两件事，全看这几条成不成立。
// 什么情况下会错、出错时人看得见什么：这一组用例红了，界面上出现的是"某一排永远转圈"
// 或"取消点下去数字还在涨"那种症状——不是编译错误，所以只能靠它们当场红。
// 怎么回来：改的永远是 queue.go／state.go 那两处，不许把断言放宽。

func TestSubmitRejectsUnknownItemAndDedupes(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)

	if _, err := st.svc.submit(contract.KindDetect, []string{"没有这个条目"}, originUser, nil); err == nil {
		t.Fatal("清单里没有的条目 id 被静默收下了")
	} else if errs.CodeOf(err) != errs.CodeTaskBadItem {
		t.Fatalf("回的码是 %s，要的是 GAC-TASK-004", errs.CodeOf(err))
	} else if got := errs.ParamsOf(err)["itemId"]; got != "没有这个条目" {
		t.Fatalf("参数里没把那个 id 带回去，got=%v", got)
	}

	batch, err := st.svc.submit(contract.KindDetect, []string{item.ID, item.ID, item.ID}, originUser, nil)
	if err != nil {
		t.Fatal(err)
	}
	if batch == "" {
		t.Fatal("没回批次号，界面那一批没有可取消的把手")
	}
	tasks := st.svc.Snapshot().Tasks
	if len(tasks) != 1 {
		t.Fatalf("同一条目在一批里留了 %d 行，应该是 1 行（两行就是两次写入抢同一行 hosts）", len(tasks))
	}
	if tasks[0].Total != 1 || tasks[0].Processed != 0 {
		t.Fatalf("批次进度那两个数是 %d／%d，这一批只有 1 行", tasks[0].Processed, tasks[0].Total)
	}
}

func TestSubmitWithEmptyListExpandsToEveryItem(t *testing.T) {
	st := newStack(t)
	if _, err := st.svc.submit(contract.KindDetect, nil, originUser, nil); err != nil {
		t.Fatal(err)
	}
	snap := st.svc.Snapshot()
	if len(snap.Items) == 0 {
		t.Fatal("快照里一条条目也没有")
	}
	if len(snap.Tasks) != len(snap.Items) {
		t.Fatalf("「全部检测」展开成 %d 行，清单里却有 %d 条", len(snap.Tasks), len(snap.Items))
	}
	for _, tk := range snap.Tasks {
		if tk.Total != len(snap.Items) {
			t.Fatalf("那一行的批次总数是 %d，跟清单条数 %d 对不上", tk.Total, len(snap.Items))
		}
	}
}

// 一条通道：同一时刻只许有一行在跑。这条是 L3 的永久解，界面置灰只是挡手点。
func TestQueueRunsOneTaskAtATime(t *testing.T) {
	st := newStack(t)
	lane := &laneProbe{}
	st.svc.cfg.Inspector = lane

	first := st.catalog.Menus()[0]
	st.modes.add(resolve.Spec{ID: "lane", Title: "用例通道", Kinds: []string{first.ID},
		NeedsProbe: true, DefaultPriority: 1, Rankable: true}, "1.2.3.4")

	var ids []string
	for _, it := range first.Items {
		// 只挑单域名的条目：探测器按域名计数，多域名条目会让"跑了几行"这个数对不上。
		if len(it.Domains) != 1 {
			continue
		}
		ids = append(ids, it.ID)
		if len(ids) == 3 {
			break
		}
	}
	if _, err := st.svc.submit(contract.KindDetect, ids, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.start()
	st.waitIdle()

	if lane.max() > 1 {
		t.Fatalf("同一时刻有 %d 次探测在飞，通道变成并发", lane.max())
	}
	if lane.total() != len(ids) {
		t.Fatalf("跑了 %d 次，提交的是 %d 行", lane.total(), len(ids))
	}
}

// laneProbe 只干一件事：数"同一时刻有几支探测在飞"。
type laneProbe struct {
	mu       sync.Mutex
	inflight int
	peak     int
	calls    int
}

func (l *laneProbe) Check(ctx context.Context, domain string, ips []string, lim detect.Limits) detect.Result {
	l.mu.Lock()
	l.inflight++
	l.calls++
	if l.inflight > l.peak {
		l.peak = l.inflight
	}
	l.mu.Unlock()

	time.Sleep(2 * time.Millisecond)

	l.mu.Lock()
	l.inflight--
	l.mu.Unlock()
	return detect.Result{Domain: domain, Best: "1.2.3.4", RTT: time.Millisecond, ProbeCount: 1}
}

func (l *laneProbe) max() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.peak
}

func (l *laneProbe) total() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

// 暂停是闸，不是收尾动作：关上之后一行都不许起跑，但已经排着的行还留在队列里。
func TestPauseHoldsTheQueueUntilResume(t *testing.T) {
	st := newStack(t)
	st.start()
	if !st.svc.pause() {
		t.Fatal("pause 没把闸关上")
	}
	item := st.cleanItem(t)
	st.mode("甲", "用例甲", menuOf(t, st.catalog, item.ID), 1, "1.2.3.4")
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}

	if _, err := st.svc.submit(contract.KindDetect, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	if len(st.taskByStatus(contract.TaskRunning)) > 0 {
		t.Fatal("闸关着却有行在跑")
	}
	if st.insp.count() != 0 {
		t.Fatalf("闸关着却已经探了 %d 次", st.insp.count())
	}
	if !st.svc.pausedNow() {
		t.Fatal("paused 被谁提前放开了")
	}

	if st.svc.resume() {
		t.Fatal("resume 之后 paused 还是真的")
	}
	st.waitIdle()
	if st.insp.count() == 0 {
		t.Fatal("恢复之后一行也没跑：这就是界面上\"要点两下\"那个毛病（L2）")
	}
}

// 任务收尾那一步顺手清暂停＝L2 的"点两下"。这一条专门钉它。
func TestPausedSurvivesTaskFinish(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.mode("甲", "用例甲", menuOf(t, st.catalog, item.ID), 1, "1.2.3.4")
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}
	// 探测那一步顺手把闸关上：这样"任务收尾"与"有人按暂停"撞在同一瞬间，正是界面上那颗按钮的形状。
	st.insp.before = func(int) { st.svc.pause() }

	st.start()
	if _, err := st.svc.submit(contract.KindDetect, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()

	if !st.svc.pausedNow() {
		t.Fatal("收尾那一步把暂停标志清掉了（L2）")
	}
	st.svc.resume()
	if st.statusOf(item.ID).BestIp == "" {
		t.Fatal("闸开之后这一行没跑完")
	}
}

func TestCancelOnlyTouchesWaitingRows(t *testing.T) {
	st := newStack(t)
	first := st.catalog.Menus()[0]
	gate := make(chan struct{})
	r := st.mode("甲", "用例甲", first.ID, 1, "1.2.3.4")
	r.block = gate
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}

	var ids []string
	for _, it := range first.Items {
		ids = append(ids, it.ID)
		if len(ids) == 3 {
			break
		}
	}
	batch, err := st.svc.submit(contract.KindDetect, ids, originUser, nil)
	if err != nil {
		t.Fatal(err)
	}
	st.start()

	// 等第一行真跑起来（探测器叫到它），再撤这一批。
	deadline := time.Now().Add(5 * time.Second)
	for st.svc.runningID() == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	running := st.svc.runningID()
	if running == "" {
		close(gate)
		t.Fatal("第一行一直没起跑")
	}

	n, err := st.svc.cancel(batch)
	if err != nil {
		close(gate)
		t.Fatalf("撤这一批失败：%v", err)
	}
	if n != len(ids)-1 {
		t.Fatalf("撤掉 %d 行，应该是 %d 行（正在跑那一条不算）", n, len(ids)-1)
	}
	if got := st.taskByID(running); got.Status != contract.TaskRunning {
		t.Fatalf("正在跑那一条变成了 %s，取消不该动它", got.Status)
	}

	close(gate)
	st.waitIdle()

	// 再撤一次：这一批里全是终态了，要如实说"撤不动"，不许假装又撤掉几条。
	if _, err := st.svc.cancel(batch); errs.CodeOf(err) != errs.CodeTaskFinal {
		t.Fatalf("重复取消回的码是 %v，要的是 GAC-TASK-002", err)
	}
	if _, err := st.svc.cancel("task-9999"); errs.CodeOf(err) != errs.CodeTaskNotFound {
		t.Fatalf("不存在的任务号回的码是 %v，要的是 GAC-TASK-001", err)
	}
}

func TestCancelWithoutIDClearsEveryWaitingRow(t *testing.T) {
	st := newStack(t)
	st.start()
	st.svc.pause()

	item := st.cleanItem(t)
	st.mode("甲", "用例甲", menuOf(t, st.catalog, item.ID), 1, "1.2.3.4")
	if _, err := st.svc.submit(contract.KindDetect, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.submit(contract.KindApply, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}

	n, err := st.svc.cancel("")
	if err != nil {
		t.Fatalf("顶部那颗「取消等待」撤不动：%v", err)
	}
	if n != 2 {
		t.Fatalf("撤掉 %d 行，此刻排着的只有 2 行", n)
	}
	if len(st.taskByStatus(contract.TaskCancelled)) != 2 {
		t.Fatalf("界面上标了已取消的是 %d 行", len(st.taskByStatus(contract.TaskCancelled)))
	}
	// 撤干净之后再按一次：没有可撤的，要如实说一句，不许回"取消了 0 条"当成功。
	if _, err := st.svc.cancel(""); errs.CodeOf(err) != errs.CodeTaskFinal {
		t.Fatalf("没有可撤的行时回了 %v", err)
	}
}

// 契约 v7 那一列「耗时」的原料：只有跑完的那一行才有数，正在跑与撤掉的行都是 0。
//
// 干什么用：界面上队列那张表的「耗时」列此前只有 mock 在本地掐表，真服务的回执里一个时间也没有——
// 接上真服务就等于把这一列砍掉（用户说的是「功能不砍」）。
// 什么情况下会错、出错时人看得见什么：
// ① 正在跑那一行也报数 → 界面那一格每来一条事件就跳一下，看着像秒表坏了，
//
//	而它报的还是"到目前为止"，跟跑完那一条的总时长不是同一回事；
//
// ② 撤掉的行也报一个 0.0s → 界面写成"这一条跑了 0.0 秒"，而那一条根本没开始干活。
//
//	这两个毛病在界面上都不报错，只是那一列说的话不对。
//
// 怎么回来：秒表只在 take()（按下）与 finish()（抬起）两处动，读数只在 durationMs() 一处判。
func TestDurationOnlyFinishedRowsReport(t *testing.T) {
	st := newStack(t)
	first := st.catalog.Menus()[0]
	gate := make(chan struct{})
	r := st.mode("甲", "用例甲", first.ID, 1, "1.2.3.4")
	r.block = gate
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}

	var ids []string
	for _, it := range first.Items {
		ids = append(ids, it.ID)
		if len(ids) == 3 {
			break
		}
	}
	batch, err := st.svc.submit(contract.KindDetect, ids, originUser, nil)
	if err != nil {
		t.Fatal(err)
	}
	st.start()

	deadline := time.Now().Add(5 * time.Second)
	for st.svc.runningID() == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	running := st.svc.runningID()
	if running == "" {
		close(gate)
		t.Fatal("第一行一直没起跑")
	}
	time.Sleep(60 * time.Millisecond)

	// ①：还在跑的那一行，此刻该报 0（界面画破折号），不许报"已经跑了 60 毫秒"。
	if got := st.taskByID(running); got.DurationMs != 0 {
		close(gate)
		t.Fatalf("正在跑的那一行报了耗时 %d 毫秒，这一列只写跑完的那个数", got.DurationMs)
	}

	// 撤掉这一批里等待中的那两行，再放开闸让第一行跑完。
	if _, err := st.svc.cancel(batch); err != nil {
		close(gate)
		t.Fatal(err)
	}
	close(gate)
	st.waitIdle()

	// ②：撤掉的行没按过表，一律 0。
	for _, tk := range st.taskByStatus(contract.TaskCancelled) {
		if tk.DurationMs != 0 {
			t.Fatalf("被撤掉的 %s 报了耗时 %d 毫秒：这一条根本没开始跑", tk.ID, tk.DurationMs)
		}
	}
	done := st.taskByStatus(contract.TaskDone)
	if len(done) != 1 {
		t.Fatalf("跑完的该有 1 行，实际 %d 行", len(done))
	}
	// 跑完那一行：闸门挡了 60 毫秒以上，读数就该看得见（下限取 30 毫秒，留够调度与快照组装的余量）。
	if done[0].DurationMs < 30 {
		t.Fatalf("跑完那一行的耗时是 %d 毫秒，可这一行实实在在被挡了 60 毫秒：秒表没按下还是没抬起？",
			done[0].DurationMs)
	}
}

// 两个参数包是不是同一份话的原料：按发上界面的那一步（JSON）比，而不是按 Go 里的形状比。
// 干什么用：行上和条目上的参数走的是同一条链，比字节才等于比界面收到的东西；
//
//	map[string]any 里有 []string（比如 GAC-HOSTS-008 那"几个不像域名的值"），
//	maps.Equal 在这种值上会当场 panic，所以这里不图省事用它。
//
// 什么情况下会错、出错时人看得见什么：两边拼参数的键名漂了一个字（itemId／itemID），
//
//	比 Go 值也照样红，但这条消息能直接指出两份 JSON 各长什么样。
//
// 怎么回来：改的是各码那一个构造处（applyrun.go、detectrun.go 那一节），不许放宽这里。
func sameParams(t *testing.T, a, b map[string]any) bool {
	t.Helper()
	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("行上的参数发不出去：%v", err)
	}
	jb, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("条目上的参数发不出去：%v", err)
	}
	if string(ja) != string(jb) {
		t.Fatalf("两边凑那句话的原料不是同一份：\n行   %s\n条目 %s", ja, jb)
	}
	return true
}

// 这一行没跑成的原因，行上跟条目上必须给同一个码（契约 v8）。
//
// 干什么用：队列那张表的「错误信息」列跟详情/树里那句失败原因，说的是同一件事。
//
//	服务只回一句中文原文的话，界面这一列就是硬编码的中文——切语言、改措辞都动不了它。
//	这一条钉的是「行上也带码与参数，跟条目那一档一模一样」。
//
// 什么情况下会错、出错时人看得见什么：① snapshot 忘了抄这两栏 → 界面上条目说人话、队列那列是原文，
//
//	两边对不上；② finish 的某一支漏了赋值 → 那一支的行只有码没有参数，界面凑出「写入  失败」这种缺字的句子；
//	③ 成功那一支没清空 → 重跑成功的老行还挂着上一轮的错误。三种都不会崩，只是那一列说的话不对。
//
// 怎么回来：赋值在 queue.go finish() 那五支里，抄送在 snapshot.go tasksLocked()，改这两处，别放宽断言。
func TestFailedRowCarriesSameCodeAsItem(t *testing.T) {
	st := newStack(t)
	item := st.badValueItem(t) // 这一条在动手之前就被拒，且拒绝的理由带参数
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, nil)

	rows := st.taskByStatus(contract.TaskFailed)
	if len(rows) != 1 {
		t.Fatalf("失败的行有 %d 条，提交的是 1 行", len(rows))
	}
	want := st.statusOf(item.ID)
	if rows[0].ErrorCode != want.ErrorCode {
		t.Fatalf("行上留的码是 %q，条目上留的是 %q：同一件事两种说法", rows[0].ErrorCode, want.ErrorCode)
	}
	sameParams(t, rows[0].ErrorParams, want.ErrorParams)
	if len(rows[0].ErrorParams) == 0 {
		t.Fatal("行上只有码没有参数，界面凑不出「哪几个值不是域名」那一句人话")
	}
	if rows[0].LastErr == "" {
		t.Fatal("行上那句服务侧原文没了：日志与控制台那一侧要看的就是它")
	}

	// 跑完的那一行两个字段都得清空，不然界面上「已检测」旁边挂着上一轮那句错误。
	st2 := newStack(t)
	it2 := st2.cleanItem(t)
	st2.mode("甲", "用例甲", menuOf(t, st2.catalog, it2.ID), 1, "1.2.3.4")
	st2.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}
	st2.run(contract.KindDetect, []string{it2.ID}, nil)
	done := st2.taskByStatus(contract.TaskDone)
	if len(done) != 1 {
		t.Fatalf("跑完的行有 %d 条", len(done))
	}
	if done[0].ErrorCode != "" || len(done[0].ErrorParams) != 0 || done[0].LastErr != "" {
		t.Fatalf("跑完那一行还带着原因：%q／%v／%q", done[0].ErrorCode, done[0].ErrorParams, done[0].LastErr)
	}
}

func TestTaskStatesAreOneWay(t *testing.T) {
	terminal := []string{contract.TaskDone, contract.TaskFailed, contract.TaskSkipped,
		contract.TaskRolledback, contract.TaskCancelled, contract.TaskRestored}
	for _, from := range terminal {
		for _, to := range []string{contract.TaskWaiting, contract.TaskRunning, contract.TaskRestoring} {
			if canMove(from, to) {
				t.Fatalf("终态 %s 被允许变成 %s：队列里那一行会反复跳", from, to)
			}
		}
		if !canMove(from, from) {
			t.Fatalf("%s 连原地都不许，收尾那一步会当场改不动", from)
		}
	}
	if !canMove(contract.TaskRunning, contract.TaskWaiting) {
		t.Fatal("running → waiting 这条边没了：暂停回来的那一行无处可去")
	}
	if canMove(contract.TaskWaiting, contract.TaskRestoring) {
		t.Fatal("等待中的行能直接跳成恢复中：状态表被放宽了")
	}
	if !canMove(contract.TaskRestoring, contract.TaskRestored) {
		t.Fatal("恢复中到已恢复这条边断了")
	}

	// moveTo 走不通时一个字都不改。
	tk := &task{id: "t", status: contract.TaskDone}
	if tk.moveTo(contract.TaskRunning) {
		t.Fatal("已完成的那一行被改回正在跑")
	}
	if tk.status != contract.TaskDone {
		t.Fatalf("改不动却把状态写坏了：%s", tk.status)
	}
	if !tk.isTerminal() {
		t.Fatal("done 不算终态？")
	}
	if n := terminalCount(); n != 6 {
		t.Fatalf("终态数成 %d 个了，isTerminal 与状态表得一起改", n)
	}
}

// 服务退场时留在队列里的行要如实标"已跳过"——演成"跑完了"是假进度。
func TestStopMarksWaitingRowsAsSkipped(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.mode("甲", "用例甲", menuOf(t, st.catalog, item.ID), 1, "1.2.3.4")
	st.svc.pause()
	if _, err := st.svc.submit(contract.KindDetect, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.svc.Start(context.Background())
	st.svc.Stop()

	snap := st.svc.Snapshot()
	if len(snap.Tasks) != 1 {
		t.Fatalf("队列里 %d 行", len(snap.Tasks))
	}
	if snap.Tasks[0].Status != contract.TaskSkipped {
		t.Fatalf("退场时那一行是 %s，要的是 skipped", snap.Tasks[0].Status)
	}
	var sawDone bool
	for _, e := range st.events.all() {
		if e.name == contract.EventQueueDone {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatal("退场没推 queue:done，界面那一排永远停在\"进行中\"")
	}
}

// 整批跑完那一下必须只推一条 queue:done：界面靠它把进度条收回 100%。
func TestQueueDoneFiresOnceWhenBatchFinishes(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.mode("甲", "用例甲", menuOf(t, st.catalog, item.ID), 1, "1.2.3.4")
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}

	st.start()
	if _, err := st.svc.submit(contract.KindDetect, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitIdle()

	var done int
	for _, e := range st.events.all() {
		if e.name == contract.EventQueueDone {
			done++
		}
	}
	if done != 1 {
		t.Fatalf("queue:done 推了 %d 条，一批只该收一次", done)
	}
	if len(st.taskByStatus(contract.TaskDone)) != 1 {
		t.Fatal("这一行没标已完成")
	}
}

func TestSubmitRejectsUnknownKind(t *testing.T) {
	st := newStack(t)
	if _, err := st.svc.submit("探测一下", nil, originUser, nil); err == nil {
		t.Fatal("不认识的任务种类被收进队列了")
	} else if errs.CodeOf(err) != errs.CodeTaskNotFound {
		t.Fatalf("回的码是 %s", errs.CodeOf(err))
	}
}

func TestNewRefusesBrokenAssembly(t *testing.T) {
	for name, cfg := range map[string]Config{
		"没给清单": {Engine: &fakeEngine{}},
		"没给引擎": {Catalog: testCatalog(t)},
	} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("%s：New 本该当场抛，不许留下一个起得来但什么都干不了的服务", name)
				}
			}()
			New(cfg)
		}()
	}
}

// 「清空已完成」只抹跑得完的那几行：正在跑的那一条与还排着的必须原位留下。
//
// 干什么用：这颗按钮上一版只在前端把行 filter 掉，服务那份表一个字没动，
// 下一次事件把同一批行原样带回来——症状是"点了没反应"。这一条钉的是"真抹掉了"。
// 什么情况下会错、出错时人看得见什么：判据要是放宽成"状态不是 waiting"，
// 正在跑那一行会从屏幕上消失，而服务还在往它推阶段事件——进度没了、hosts 却照样被改了。
// 怎么回来：能动的那几行只由 isTerminal() 判；用例红了就回去看 clearFinished 那一段的筛子。
func TestClearFinishedDropsOnlyTerminalRows(t *testing.T) {
	st := newStack(t)
	first := st.catalog.Menus()[0]
	gate := make(chan struct{})
	r := st.mode("甲", "用例甲", first.ID, 1, "1.2.3.4")
	r.block = gate
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}

	var ids []string
	for _, it := range first.Items {
		ids = append(ids, it.ID)
		if len(ids) == 4 {
			break
		}
	}
	batch, err := st.svc.submit(contract.KindDetect, ids, originUser, nil)
	if err != nil {
		t.Fatal(err)
	}
	st.start()

	deadline := time.Now().Add(5 * time.Second)
	for st.svc.runningID() == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	running := st.svc.runningID()
	if running == "" {
		close(gate)
		t.Fatal("第一行一直没起跑")
	}
	if _, err := st.svc.cancel(batch); err != nil {
		close(gate)
		t.Fatalf("撤掉剩下那几行失败：%v", err)
	}

	snap := st.svc.clearFinished()
	if len(snap.Tasks) != 1 {
		close(gate)
		t.Fatalf("清完还剩 %d 行，要的是 1 行（正在跑那一条不该被抹）", len(snap.Tasks))
	}
	if snap.Tasks[0].ID != running || snap.Tasks[0].Status != contract.TaskRunning {
		close(gate)
		t.Fatalf("留下的那行是 %s/%s，要的是正在跑的 %s", snap.Tasks[0].ID, snap.Tasks[0].Status, running)
	}
	// 抹行不碰条目：54 条那一列一行不许少，检测出来的地址也不许跟着行一起没了
	itemsBefore := len(snap.Items)

	close(gate)
	st.waitIdle()

	snap2 := st.svc.clearFinished()
	if len(snap2.Tasks) != 0 {
		t.Fatalf("跑完之后再清还剩 %d 行，界面上那颗按钮还是「点了没反应」", len(snap2.Tasks))
	}
	if len(snap2.Items) != itemsBefore {
		t.Fatalf("条目从 %d 行变成 %d 行：抹队列那一步顺手抹到条目表上了", itemsBefore, len(snap2.Items))
	}
	if got := st.statusOf(ids[0]); got.BestIp == "" {
		t.Fatalf("清完之后条目 %s 的最优地址空了：跑完的行列本来就在条目状态里", ids[0])
	}
}

// 「清空已完成」把失败与已回滚那两行留下——界面上「重试失败的那几条」的原料就是它们。
//
// 干什么用：这一条钉的是"抹行这一步不许顺手砍掉另一颗按钮要用的东西"。
// 什么情况下会错、出错时人看得见什么：上一版的筛子是 isTerminal()（六档全抹），
// 用户点完"清空已完成"，屏幕上那几行红的跟着一起没了，
// 于是"重试失败的那几条"点下去说"没有可重试的行"，而同一个失败在条目那一列还挂着"出错"——
// 屏幕左半边说有事、右半边说没事，这正是本批账里最难查的那一种自相矛盾。
// 怎么回来：该不该抹只由 state.go 的 isClearable() 判；用例红了先看那两档有没有被放回筛子。
func TestClearFinishedKeepsFailedAndRolledbackRows(t *testing.T) {
	st := newStack(t)
	bad := st.badValueItem(t)
	st.seedBest(bad, "1.2.3.4")
	good := st.cleanItem(t)
	st.seedBest(good, "5.6.7.8")

	// 三次动作各成一个批次（界面上就是三次点击），行都留在同一张表上
	st.start()
	for _, job := range []struct {
		kind string
		ids  []string
	}{
		{contract.KindApply, []string{bad.ID}},
		{contract.KindApply, []string{good.ID}},
		{contract.KindRestore, []string{good.ID}},
	} {
		if _, err := st.svc.submit(job.kind, job.ids, originUser, nil); err != nil {
			t.Fatal(err)
		}
		st.waitIdle()
	}

	before := st.svc.Snapshot().Tasks
	if len(before) != 3 {
		t.Fatalf("清之前队列有 %d 行，要的是 3 行（失败／已生效／已恢复各一行）", len(before))
	}

	snap := st.svc.clearFinished()
	if len(snap.Tasks) != 1 {
		t.Fatalf("清完还剩 %d 行，要的是只留失败那一行：%+v", len(snap.Tasks), snap.Tasks)
	}
	left := snap.Tasks[0]
	if left.Status != contract.TaskFailed || left.ItemID != bad.ID {
		t.Fatalf("留下的那行是 %s／%s，要的是失败的 %s", left.Status, left.ItemID, bad.ID)
	}
	// 留下的这一行还得说得清自己为什么留着——契约 v8 那两栏不能因为抹行这一步掉掉。
	if left.ErrorCode != errs.CodeHostsBadValue || len(left.ErrorParams) == 0 {
		t.Fatalf("留下的那行少了码或参数，界面上「重试」那颗按钮凑不出这句话：%q／%v", left.ErrorCode, left.ErrorParams)
	}

	// 再点一次「清空已完成」：留下的那一行不该把自己抹掉，也不该长出第二行。
	if again := st.svc.clearFinished(); len(again.Tasks) != 1 {
		t.Fatalf("第二次清完剩 %d 行，要的还是 1 行", len(again.Tasks))
	}

	// 已回滚那一档同理留下：它说的是"写完重读对不上、已经还原了"，那是用户要处理的事，不是跑完了。
	st2 := newStack(t)
	item2 := st2.cleanItem(t)
	st2.svc.cfg.Engine = &fakeEngine{rollback: true}
	st2.seedBest(item2, "1.2.3.4")
	st2.run(contract.KindApply, []string{item2.ID}, nil)
	rows := st2.svc.clearFinished().Tasks
	if len(rows) != 1 || rows[0].Status != contract.TaskRolledback {
		t.Fatalf("清完之后已回滚那一行没了：%+v", rows)
	}
}

// 任务表不再只增不删：跑几轮、到点清一次，行数就该回到 0，峰值也只跟着清除的间隔走。
//
// 干什么用：上一版 s.tasks 只有 append（submit 那一步），一次开机点几十次就把这几百行一直带到进程退出，
// 而每一次快照都要把整张表重新拼一遍给界面——队列那一排越滚越长，界面也越刷越慢。
// 什么情况下会错、出错时人看得见什么：清的时候只抹"这一批"，别批的终态行留下 → 这里的行数还会往上涨。
// 怎么回来：筛子不看批次、只看 isTerminal()；这条红了就是那里漏了一档状态。
func TestClearTasksBoundsTheTableAcrossBatches(t *testing.T) {
	st := newStack(t)
	st.start()
	item := st.cleanItem(t)
	st.mode("甲", "用例甲", menuOf(t, st.catalog, item.ID), 1, "1.2.3.4")
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}

	var tallest int
	for round := 0; round < 7; round++ {
		if _, err := st.svc.submit(contract.KindDetect, []string{item.ID}, originUser, nil); err != nil {
			t.Fatal(err)
		}
		st.waitIdle()
		// 记下历次快照里的行数峰值：只在"每三轮抹一次"这个前提下它才该是 3
		if n := len(st.svc.Snapshot().Tasks); n > tallest {
			tallest = n
		}
		if round%3 == 2 {
			if n := len(st.svc.clearFinished().Tasks); n != 0 {
				t.Fatalf("第 %d 轮清完还剩 %d 行", round, n)
			}
		}
	}
	// 每三轮抹一次，所以峰值就该是 3：再高说明没抹掉，再小说明把还没到点那几行也抹了。
	if tallest != 3 {
		t.Fatalf("七轮里表最长有过 %d 行，要的是 3 行（每三轮抹一次的峰值）", tallest)
	}
}
