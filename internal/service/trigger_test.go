package service

// trigger_test.go 钉的是 2026-10-23 那一轮删掉的东西不许回来：服务自己不许起开机那一批。
//
// 干什么用：那两把开关（autoDetectOnStartup／autoApplyOnStartup）说的是"自动对**已勾选**的项各跑一次",
// 而"勾了哪几条"只有界面知道——契约里的条目没有 checked 这一栏。
// 什么情况下会错、出错时人看得见什么：把 Startup() 加回来 → 界面上开关关着、hosts 每次开机还是被整批改写,
// 而且一句错也不报（同一时间界面自己那趟自动化又起一批，两批并排抢同一行 hosts）。
// 所以这里比对的不是"代码里有没有那个函数"，而是"服务起跑之后队列里到底有没有行"——
// 两把开关都写 true、设置文件读得出来，队列还必须是空的。
// 怎么回来：开机的自动化只有界面那一个主人（frontend/core/flows.js 的 runStartupAutomation）；
// 它走 Detect／ApplyQueue 这两个方法，跟人点按钮同一条 submit、同一批事件（L1 那句口径没动）。
// 这一条在前端那侧由 tests/flows.test.mjs 钉"自动化只提交勾上的那几条"。

import (
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/settings"
)

// setStartupFlags 把那两把开机开关写进注入的临时设置文件。
func (st *stack) setStartupFlags(detectOn, applyOn bool) {
	st.t.Helper()
	next := settings.Defaults()
	next.AutoDetectOnStartup = detectOn
	next.AutoApplyOnStartup = applyOn
	p, err := st.svc.settingsPath()
	if err != nil {
		st.t.Fatalf("取设置路径失败：%v", err)
	}
	if err := settings.SaveTo(p, next); err != nil {
		st.t.Fatalf("写设置失败：%v", err)
	}
}

// countKind 数一数队列里有几行是这一种任务。
func (st *stack) countKind(tasks []contract.Task, kind string) int {
	n := 0
	for _, tk := range tasks {
		if tk.Kind == kind {
			n++
		}
	}
	return n
}

func TestServiceStartDoesNotSubmitAnythingItself(t *testing.T) {
	st := newStack(t)
	// 两把开关都开着：这是最容易被"顺手补一句自提批"骗过去的那一档
	st.setStartupFlags(true, true)
	st.start()

	// 起跑之后留一点点时间：真要自提批，那两批是在 Start 之后立刻提交的，等不出别的结果
	time.Sleep(50 * time.Millisecond)

	snap := st.svc.Snapshot()
	if len(snap.Tasks) != 0 {
		t.Fatalf("服务起跑自己起了 %d 行任务（检测 %d 行、应用 %d 行）：开机那一批的主人只有界面一个",
			len(snap.Tasks),
			st.countKind(snap.Tasks, contract.KindDetect),
			st.countKind(snap.Tasks, contract.KindApply))
	}
	if st.insp.count() != 0 {
		t.Fatalf("服务起跑没等人点就探测了 %d 次", st.insp.count())
	}
	if snap.State.LastError != "" {
		t.Fatalf("一切正常却留了一句：%s", snap.State.LastError)
	}
	// 一个字也没碰：那份临时 hosts 到这一步还不存在（readFile 把"没有这个文件"读成空串）
	if raw := readFile(t, st.itemsPath); raw != "" {
		t.Fatalf("服务起跑自己动了 hosts 文件：\n%s", raw)
	}
}
