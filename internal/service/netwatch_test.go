package service

import (
	"context"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/detect"
)

// 断网判定的用例：判定器是假件拨开关，全程不碰真网。
// 钉四件事（语义见 netwatch.go 头注释——暂停在 queue.take 的过滤里,不在 watcher 里拉闸）：
//
//	① 相位翻转才推事件，每条都带完整快照（契约那条铁规矩），快照里 offline 与事件载荷一致；
//	② 断网时提交的**检测**任务留在等待里（take 那道过滤），不被拿去跑；
//	③ 恢复联网后检测自然放行，不需要谁去点「继续」；
//	④ 断网不妨碍应用——它不需要网。
func TestNetworkWatchHoldsDetectWhileOffline(t *testing.T) {
	st := newStack(t)
	st.svc.netChecker = func() bool { return st.online }
	oldEvery := netEvery
	netEvery = 50 * time.Millisecond
	t.Cleanup(func() { netEvery = oldEvery })

	// 在线起步:先让一条检测真跑完（开局初值与第一条 offline 事件都由后面的翻转给出）
	// 方式的 Kinds 要对上这条条目所在的菜单（用例甲的写法）：拨 "1.2.3.4" 让探测真过
	st.mode("甲", "用例甲", menuOf(t, st.catalog, st.cleanItem(t).ID), 1, "1.2.3.4")
	st.insp.fallback = detect.Result{Best: "1.2.3.4", RTT: time.Millisecond}
	st.svc.Start(context.Background())
	it := st.cleanItem(t)
	if _, err := st.svc.submit(contract.KindDetect, []string{it.ID}, "test", nil); err != nil {
		t.Fatalf("入队失败：%v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		tks := st.svc.Snapshot().Tasks
		return len(tks) == 1 && tks[0].Status == contract.TaskDone
	})

	// 断网:事件推 {online:false},快照里 offline=true
	st.online = false
	waitFor(t, 3*time.Second, func() bool { return st.svc.Snapshot().NetworkOffline })

	// 断网状态下再排一条检测:必须留在等待里,不被拿去跑
	if _, err := st.svc.submit(contract.KindDetect, []string{it.ID}, "test", nil); err != nil {
		t.Fatalf("入队失败：%v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := st.svc.Snapshot().Tasks[1].Status; got != contract.TaskWaiting {
		t.Fatalf("断网时检测该留在等待里，got %q", got)
	}

	// 恢复联网:检测自然放行,到终态
	st.online = true
	waitFor(t, 3*time.Second, func() bool {
		return st.svc.Snapshot().Tasks[1].Status != contract.TaskWaiting
	})

	// 两条事件都推过,快照与载荷一致
	var sawOffline, sawOnline bool
	for _, ev := range st.events.all() {
		if ev.name != contract.EventNetworkChange {
			continue
		}
		if v, ok := ev.payload["online"].(bool); ok {
			if !v && ev.snap.NetworkOffline {
				sawOffline = true
			}
			if v && !ev.snap.NetworkOffline {
				sawOnline = true
			}
		}
	}
	if !sawOffline || !sawOnline {
		t.Fatalf("network:change 事件没推齐:offline=%v online=%v", sawOffline, sawOnline)
	}
}

// 断网不妨碍应用：它不需要网,断网状态下照样四阶段跑完。
func TestNetworkWatchDoesNotHoldApply(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.seedBest(item, "1.2.3.4")
	st.svc.netChecker = func() bool { return st.online }
	oldEvery := netEvery
	netEvery = 50 * time.Millisecond
	t.Cleanup(func() { netEvery = oldEvery })
	st.online = false
	st.svc.Start(context.Background())
	waitFor(t, 2*time.Second, func() bool { return st.svc.Snapshot().NetworkOffline })

	if _, err := st.svc.submit(contract.KindApply, []string{item.ID}, "test", nil); err != nil {
		t.Fatalf("入队失败：%v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		return st.svc.Snapshot().Tasks[0].Status == contract.TaskDone
	})
}

func waitFor(t *testing.T, d time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("等到超时也没等到那个条件")
}
