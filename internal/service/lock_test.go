package service

// lock_test.go 钉的是"同一刻只许一个写入碰这份文件"。
//
// 干什么用：界面上那三颗会写的按钮能在同一瞬间被点下去，队列之外的任何一道关都挡不住
// 有人绕开队列直接调写入——这把锁在服务侧，界面上的置灰只挡手点。
// 什么情况下会错、出错时人看得见什么：
// ① 锁没拿住就写 → 后写的把前写的盖掉，用户看到的是"明明应用了两条，文件里只剩一条"；
// ② 拿住了忘了还 → 整条队列停摆，界面从此不动；
// ③ 撞上了却排队等待 → 那一条任务在界面上像卡住了，而实话是"有别的写入在跑"。
// 怎么回来：还锁一定在 defer 里（出错也还）；撞锁那一句回 GAC-HOSTS-006，界面据此置灰。

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/hosts"
)

// waitLocked 等到"此刻有写入在跑"为止（进没进到引擎里就看这把锁）。
func (st *stack) waitLocked(want bool) {
	st.t.Helper()
	for i := 0; i < 2000; i++ {
		if st.svc.writeLockHeld() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	st.t.Fatalf("等了 2 秒，写锁没变成 %v", want)
}

func TestWriteLockRefusesSecondWriter(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	// 引擎卡在 Apply 这一次调用里＝锁正被攥着（applyrun.go 里 withWriteLock 包住的就是这一次调用）。
	eng := &fakeEngine{block: make(chan struct{})}
	st.svc.cfg.Engine = eng
	st.seedBest(item, "1.2.3.4")

	st.start()
	if _, err := st.svc.submit(contract.KindApply, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitLocked(true)

	// 第二个人绕开队列直接来：当场拒，而且不许跑那一段。
	ran := false
	err := st.svc.withWriteLock(func() error { ran = true; return nil })
	if err == nil {
		close(eng.block)
		st.waitIdle()
		t.Fatal("已经有写入在跑，第二个写入却拿到了锁——后写的会把前写的盖掉")
	}
	if ran {
		close(eng.block)
		st.waitIdle()
		t.Fatal("被拒了却还是跑了那一段：等于这道关根本没拦")
	}
	if code := errs.CodeOf(err); code != errs.CodeHostsBusy {
		close(eng.block)
		st.waitIdle()
		t.Fatalf("留的码是 %q，要的是 GAC-HOSTS-006", code)
	}
	if got := st.svc.status(); !got.WriteLockHeld {
		t.Error("GetStatus 的 writeLockHeld 没说真话：界面上那几颗按钮不会置灰")
	}

	close(eng.block)
	st.waitLocked(false)
	if st.svc.status().WriteLockHeld {
		t.Error("写入跑完了锁没还：整条队列从此停摆")
	}
	st.waitIdle()
	if tk := st.svc.Snapshot().Tasks[0]; tk.Status != contract.TaskDone {
		t.Fatalf("被拦的那一次把正经任务带坏了：%s（%s）", tk.Status, tk.LastErr)
	}
}

// 撞锁不排队：等下去的话界面以为任务卡住了，而实话是"有别的写入在跑，请等它结束"。
func TestSecondWriterIsRefusedImmediately(t *testing.T) {
	st := newStack(t)
	eng := &fakeEngine{block: make(chan struct{})}
	item := st.cleanItem(t)
	st.svc.cfg.Engine = eng
	st.seedBest(item, "1.2.3.4")

	st.start()
	if _, err := st.svc.submit(contract.KindApply, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	st.waitLocked(true)

	done := make(chan error, 1)
	go func() { done <- st.svc.withWriteLock(func() error { return nil }) }()
	select {
	case err := <-done:
		if err == nil {
			close(eng.block)
			t.Fatal("第二个写入拿到了锁")
		}
	case <-time.After(time.Second):
		close(eng.block)
		t.Fatal("撞锁那一句在排队等待：界面上这一行看起来就是卡住了")
	}
	close(eng.block)
	st.waitIdle()
}

// 写入当场失败也必须还锁——否则一次失败就把服务弄死。
func TestWriteLockReleasedAfterFailure(t *testing.T) {
	st := newStack(t)
	eng := &fakeEngine{applyErr: errs.New(errs.CodeHostsWrite, map[string]any{"path": "内存"}, "写不下去")}
	item := st.cleanItem(t)
	st.svc.cfg.Engine = eng
	st.seedBest(item, "1.2.3.4")

	st.run(contract.KindApply, []string{item.ID}, nil)

	st.waitLocked(false)
	if err := st.svc.withWriteLock(func() error { return nil }); err != nil {
		t.Fatalf("上一次写入失败后锁没还：%v", err)
	}
}

// 正常链路上这把锁一次也不该撞：队列是一条串行通道（L3）。
// 撞上了说明有人在队列之外起写入——那才是这把锁真正要防的事。
func TestBatchOfWritesNeverCollides(t *testing.T) {
	st := newStack(t)
	ids := []string{}
	for _, it := range st.catalog.Menus()[0].Items {
		if st.hasConflict(it) || len(st.badValues()[it.ID]) > 0 {
			continue
		}
		ids = append(ids, it.ID)
		st.seedBest(it, "1.2.3.4")
		if len(ids) == 4 {
			break
		}
	}
	if len(ids) < 2 {
		t.Skip("这块菜单里可用条目不足两条")
	}
	// 引擎每开一次写都记一次"此刻有几个写入在锁里"。
	eng := &countingEngine{FileEngine: st.engine, peak: &peak{}}
	st.svc.cfg.Engine = eng

	st.run(contract.KindApply, ids, nil)

	if got := eng.peak.get(); got > 1 {
		t.Fatalf("一批里同时有 %d 个写入在文件上：队列那条串行通道破了", got)
	}
	if eng.peak.calls == 0 {
		t.Fatal("一次写入也没进来：这一条用例没测到东西")
	}
}

// peak 是"同时在引擎里有几个"的表针。
type peak struct {
	mu    sync.Mutex
	n     int
	max   int
	calls int
}

func (p *peak) enter() {
	p.mu.Lock()
	p.n++
	p.calls++
	if p.n > p.max {
		p.max = p.n
	}
	p.mu.Unlock()
}

func (p *peak) leave() {
	p.mu.Lock()
	p.n--
	p.mu.Unlock()
}

func (p *peak) get() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.max
}

// countingEngine 只往真引擎上加一只表针，其余全部转发：
// 链路上那五步还是真引擎在跑，用例读的是"有没有两个写入同时在文件上"。
type countingEngine struct {
	*hosts.FileEngine
	peak *peak
}

func (e *countingEngine) Apply(ctx context.Context, changes []hosts.Change) (hosts.Receipt, error) {
	e.peak.enter()
	defer e.peak.leave()
	return e.FileEngine.Apply(ctx, changes)
}

func (e *countingEngine) Restore(ctx context.Context, domains []string) (hosts.Receipt, error) {
	e.peak.enter()
	defer e.peak.leave()
	return e.FileEngine.Restore(ctx, domains)
}
