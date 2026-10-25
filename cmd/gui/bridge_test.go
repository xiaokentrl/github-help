package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/ipc"
)

// 桥这一层的用例只借一张假服务：三条铁律要求单元测试不碰真 hosts、不发真网络，
// 所以这里一条 Unix 连接也不建——通道本身在 internal/ipc 那边已经用真连接验过了。

type frame struct {
	name string
	data any
}

type recorder struct {
	mu     sync.Mutex
	frames []frame
	logs   []string
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frames = append(r.frames, frame{name: name, data: data})
}

func (r *recorder) logf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}

func (r *recorder) all() []frame {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]frame(nil), r.frames...)
}

func (r *recorder) allLogs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.logs...)
}

// waitFrames 等到第 n 条递话出现。等不到就红：桥最坏的错法是"什么也不说地卡住"。
func waitFrames(t *testing.T, r *recorder, n int) []frame {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got := r.all()
		if len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等了 2 秒只递出 %d 条话，要 %d 条——这一条最可能是桥没接上窗口", len(r.all()), n)
	return nil
}

// waitLogs 等到第 n 行日志写下。它是"先递话、后写日志"那一步的尾巴，只等帧数会读到一个还没落盘的快照。
func waitLogs(t *testing.T, r *recorder, n int) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got := r.allLogs()
		if len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等了 2 秒只写了 %d 行日志，要 %d 行", len(r.allLogs()), n)
	return nil
}

type recordedCall struct {
	method  string
	params  map[string]any
	timeout time.Duration
}

// fakeCaller 是一张会按要求变坏的假服务：连不上／答不了／答慢，都由用例点名。
type fakeCaller struct {
	mu sync.Mutex

	connectErr  error
	dialCount   int
	connected   bool
	calls       []recordedCall
	reply       any
	replyErr    error
	events      chan contract.Event
	dropped     int64
	closeCalled bool
}

func newFakeCaller() *fakeCaller {
	return &fakeCaller{events: make(chan contract.Event, 8)}
}

func (f *fakeCaller) Connect(string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dialCount++
	if f.connectErr != nil {
		return f.connectErr
	}
	f.connected = true
	return nil
}

func (f *fakeCaller) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCalled = true
	return nil
}

func (f *fakeCaller) ActiveEndpoint() string { return "" }

func (f *fakeCaller) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}

func (f *fakeCaller) Call(method string, params map[string]any, timeout time.Duration) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, recordedCall{method: method, params: params, timeout: timeout})
	return f.reply, f.replyErr
}

func (f *fakeCaller) Events() <-chan contract.Event { return f.events }

func (f *fakeCaller) Dropped() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dropped
}

func (f *fakeCaller) setDropped(n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dropped = n
}

func (f *fakeCaller) drop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = false
}

func (f *fakeCaller) setReply(data any, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reply, f.replyErr = data, err
}

func (f *fakeCaller) recorded() []recordedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedCall(nil), f.calls...)
}

func (f *fakeCaller) dials() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dialCount
}

// testBridge 用最短的节拍跑：桥的节拍（10 秒／1 秒）由常量钉住，用例只关心先后顺序。
func testBridge(c caller, rec *recorder) *bridge {
	known := make(map[string]bool, len(contract.AllMethods))
	for _, m := range contract.AllMethods {
		known[m] = true
	}
	return &bridge{
		endpoints: []string{"/tmp/does-not-matter.sock"},
		client:    c,
		timeout:   50 * time.Millisecond,
		retry:     5 * time.Millisecond,
		emit:      rec.emit,
		logf:      rec.logf,
		known:     known,
	}
}

// lastReply 取出最近一条 gac:reply，并把它的形状认回 contract.Response。
// 干什么用：回执的形状的家在契约，这里绕一次 JSON 就是照那个家认——壳自己另立一份形状的话，
// 界面那一份解不出来，症状是某一行永远空着且不报错。
// 顺手比一次版本号：每一条回执都该带壳这一侧的契约版本，漏在某一个出口上正是最难查的那种漂。
func lastReply(t *testing.T, r *recorder) contract.Response {
	t.Helper()
	fs := r.all()
	for i := len(fs) - 1; i >= 0; i-- {
		if fs[i].name != contract.WireReply {
			continue
		}
		buf, err := json.Marshal(fs[i].data)
		if err != nil {
			t.Fatalf("回执拼不回去: %v", err)
		}
		var out contract.Response
		if err := json.Unmarshal(buf, &out); err != nil {
			t.Fatalf("回执的形状看不懂（桥递出去的那一份形状漂了）: %v", err)
		}
		if out.Protocol != contract.ProtocolVersion {
			t.Fatalf("回执没带壳这一侧的契约版本 v%d，实际 v%d：界面没法判断两边是不是同一轮构建",
				contract.ProtocolVersion, out.Protocol)
		}
		return out
	}
	t.Fatalf("没等到 %s 的回执，只递出 %d 条话", contract.WireReply, len(fs))
	return contract.Response{}
}

// lastFrame 取出最近一条递话，按名字认——认不出名字就算红，不让用例把顺序记错当成通过。
func lastFrame(t *testing.T, r *recorder, name string) any {
	t.Helper()
	fs := r.all()
	for i := len(fs) - 1; i >= 0; i-- {
		if fs[i].name == name {
			return fs[i].data
		}
	}
	t.Fatalf("没等到 %s，只递出 %d 条话", name, len(fs))
	return nil
}

func asJSON(t *testing.T, v any) string {
	t.Helper()
	buf, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("递出去的东西连 JSON 都拼不出来: %v", err)
	}
	return string(buf)
}

// fakeCallOnce 让一次点击发一次调用（异步四个方法在界面那侧本来就是"发一次、拿任务号"）。
func fakeCallOnce(b *bridge, id int64, method string) {
	b.handleInvoke(map[string]any{"id": float64(id), "method": method})
}

// 界面上的每一次点击都要变成一次真的 IPC 调用，参数、方法名、超时一个都不能在路上掉。
// 少了这一条，界面按了按钮而服务那头一个字也没听见——正是"点了没反应"那一种。
func TestInvokeReachesTheServiceAndItsAnswerComesBack(t *testing.T) {
	rec := &recorder{}
	fake := newFakeCaller()
	fake.setReply(contract.TaskIDReply{TaskID: "T-7"}, nil)
	b := testBridge(fake, rec)

	b.handleInvoke(map[string]any{"id": float64(3), "method": contract.MethodApplyQueue,
		"params": map[string]any{"itemIDs": []any{"api.github.com"}}})

	calls := fake.recorded()
	if len(calls) != 1 {
		t.Fatalf("界面一次点击该只发一次调用，实际 %d 次", len(calls))
	}
	if calls[0].method != contract.MethodApplyQueue {
		t.Fatalf("方法名在路上换了: %s", calls[0].method)
	}
	if ids := stringsParam(calls[0].params, "itemIDs"); len(ids) != 1 || ids[0] != "api.github.com" {
		t.Fatalf("参数掉了一半: %v", calls[0].params)
	}
	if calls[0].timeout != b.timeout {
		t.Fatalf("超时没把桥这一份传下去: %s", calls[0].timeout)
	}

	waitFrames(t, rec, 1)
	got := lastReply(t, rec)
	if !got.OK || got.ID != 3 {
		t.Fatalf("回执对不上那一次点击: %+v", got)
	}
	data, _ := got.Result.(map[string]any)
	if data["taskID"] != "T-7" {
		t.Fatalf("服务给的任务号没原样递回去: %+v", got.Result)
	}
}

// 契约里没有的名字不许发出去。
// 干什么用：让"界面这条线写错了"和"服务缺这一个注册"是两句不同的话。
// 怎么会错：照发不误的话服务回的是同一句 GAC-IPC-011，人只会以为工具还没做这个功能。
func TestUnknownMethodNameNeverLeavesTheShell(t *testing.T) {
	rec := &recorder{}
	fake := newFakeCaller()
	b := testBridge(fake, rec)

	b.handleInvoke(map[string]any{"id": float64(1), "method": "GetEverything"})

	if n := len(fake.recorded()); n != 0 {
		t.Fatalf("契约外的名字还是发出去了，发了 %d 次", n)
	}
	waitFrames(t, rec, 1)
	got := lastReply(t, rec)
	if got.Code != errs.CodeIPCNoHandler {
		t.Fatalf("该用同一句'没这个方法'的码：%s", got.Code)
	}
	if !strings.Contains(got.Err, "GetEverything") {
		t.Fatalf("报错里要带那个拼错的名字，人才知道改哪儿: %s", got.Err)
	}
}

// 16 个契约方法要一个一个能过这道闸。
// 为什么逐个试而不是只看数量：闸要是比契约窄（漏收一个名字），那个按钮点下去就变成"界面写错了"，
// 而这一条只有把每一个名字都真发一遍才看得出来。
func TestEveryContractMethodPassesTheGate(t *testing.T) {
	rec := &recorder{}
	fake := newFakeCaller()
	b := testBridge(fake, rec)

	for _, m := range contract.AllMethods {
		b.handleInvoke(map[string]any{"id": float64(1), "method": m})
	}
	calls := fake.recorded()
	if len(calls) != len(contract.AllMethods) {
		t.Fatalf("契约 %d 个方法只放行 %d 个，缺的那几个按钮会点不动", len(contract.AllMethods), len(calls))
	}
	seen := map[string]bool{}
	for _, c := range calls {
		seen[c.method] = true
	}
	for _, m := range contract.AllMethods {
		if !seen[m] {
			t.Fatalf("这个方法没过闸: %s", m)
		}
	}
}

// 连不上、版本对不上这两种错要带码回来，而不是让界面干等。
// 干什么用：这两句决定了人下一步做什么——重连服务，还是把两边换成同一份构建。
// 怎么会错：都写成"通道报错"就等于让人自己猜；一句不说地卡住更糟，人只会以为界面死了。
func TestCallFailuresComeBackAsCodesNotAsSilence(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		want    string
		saysAll string
	}{
		{"连不上", fmt.Errorf("%w: dial unix /x: no such file", ipc.ErrNoConnection),
			errs.CodeIPCDisconnect, "no such file"},
		{"版本对不上", fmt.Errorf("%w: 界面要 v4，服务给的是 v3", ipc.ErrVersionMismatch),
			errs.CodeIPCShape, "v3"},
		{"服务自己报的码", errors.New(errs.CodeHostsWrite + ": 写不进去"),
			errs.CodeHostsWrite, "写不进去"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			fake := newFakeCaller()
			fake.setReply(nil, tc.err)
			b := testBridge(fake, rec)

			b.handleInvoke(map[string]any{"id": float64(9), "method": contract.MethodGetStatus})

			waitFrames(t, rec, 1)
			got := lastReply(t, rec)
			if got.OK {
				t.Fatal("服务没答上来，回执却报了成功——界面接下来就会拿空字段画一副没事的样子")
			}
			if got.Code != tc.want {
				t.Fatalf("码该是 %s，实际 %s", tc.want, got.Code)
			}
			if !strings.Contains(got.Err, tc.saysAll) {
				t.Fatalf("原话该整句留着当依据: %s", got.Err)
			}
			if got.ID != 9 {
				t.Fatalf("回执要对得上那一次点击: %+v", got)
			}
		})
	}
}

// 桥看不懂的半句话要当场报错，不能当成"没有方法名"发出去。
// 为什么要单独立一条：id／method 这两个键名一旦在前端漂了，症状是每一次点击都只回同一句"看不懂"，
// 而这一条用例把那三个键名钉在这里。
func TestGarbledInvokeIsRejectedOutright(t *testing.T) {
	rec := &recorder{}
	fake := newFakeCaller()
	b := testBridge(fake, rec)

	b.handleInvoke(map[string]any{"id": "三", "method": contract.MethodGetStatus})

	if n := len(fake.recorded()); n != 0 {
		t.Fatalf("id 不是个数就不该发调用，发了 %d 次", n)
	}
	fs := waitFrames(t, rec, 1)
	if fs[0].name != contract.WireReply {
		t.Fatalf("该回一句回执: %v", fs[0].name)
	}
	raw, _ := json.Marshal(fs[0].data)
	if !strings.Contains(string(raw), errs.CodeIPCFrame) {
		t.Fatalf("该报'这句话看不懂': %s", raw)
	}

	// 键名漂了也是同一档：少了 method 这一键，就不许发一条"方法名是空"的调用出去。
	b.handleInvoke(map[string]any{"id": float64(1)})
	if n := len(fake.recorded()); n != 0 {
		t.Fatalf("没带方法名的句子不该发出去，发了 %d 次", n)
	}
}

// 服务推进来的每一条事件，要按契约里那五个名字之一递到窗口，快照整条带上。
// 干什么用：界面那五句订阅用的就是这些名字；桥要是加个前缀或换个名，订阅就全空。
// 怎么会错：只转 payload 不转 snapshot，界面就得拿上一帧往前推——丢一帧之后画出来的是不存在的那一半状态。
func TestServiceEventsForwardUnderTheirOwnNames(t *testing.T) {
	rec := &recorder{}
	fake := newFakeCaller()
	b := testBridge(fake, rec)

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()
	waitFrames(t, rec, 1) // 起步那一条 connected:true

	want := contract.Snapshot{State: contract.State{Phase: contract.ServiceRunning},
		Tasks: []contract.Task{{ID: "B-1", Kind: contract.KindApply, Phase: contract.PhaseWrite, Processed: 2, Total: 4}}}
	for _, name := range contract.AllEvents {
		fake.events <- contract.Event{Name: name, Snapshot: want, Payload: map[string]any{"itemID": "api.github.com"}}
	}

	fs := waitFrames(t, rec, 1+len(contract.AllEvents))[1:]
	stop()
	<-done

	for i, name := range contract.AllEvents {
		if fs[i].name != name {
			t.Fatalf("第 %d 条事件换了名字（界面那句订阅就收不到了）: %s ≠ %s", i, fs[i].name, name)
		}
		ev, ok := fs[i].data.(contract.Event)
		if !ok {
			t.Fatalf("第 %d 条递的不是整条事件: %T", i, fs[i].data)
		}
		if len(ev.Snapshot.Tasks) != 1 || ev.Snapshot.Tasks[0].Phase != contract.PhaseWrite {
			t.Fatalf("快照没整条带上: %+v", ev.Snapshot)
		}
		if ev.Payload["itemID"] != "api.github.com" {
			t.Fatalf("事件自己那半句话掉了: %v", ev.Payload)
		}
	}
}

// 连不上时要把这一句说出来，而且只在真的换了说法时说一次。
// 干什么用：界面壳刚开、服务还没起，是人最常碰到的一档；不说就只会看见一个空界面。
// 怎么会错：每秒重连就每秒推一条一模一样的话，窗口那条队列会被这种重复挤满。
func TestStateFlipsOnlyWhenItActuallyChanges(t *testing.T) {
	rec := &recorder{}
	fake := newFakeCaller()
	fake.connectErr = errors.New("端点文件还没建出来")
	b := testBridge(fake, rec)

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()

	// 让它至少试三轮：同一句坏消息反复出现，只许递第一条。
	time.Sleep(40 * time.Millisecond)
	if fake.dials() < 3 {
		t.Fatalf("连不上就该按节拍再试，只试了 %d 次", fake.dials())
	}
	if n := len(rec.all()); n != 1 {
		t.Fatalf("同一句坏消息只该递一次，实际递了 %d 次", n)
	}

	fake.mu.Lock()
	fake.connectErr = nil
	fake.mu.Unlock()
	fs := waitFrames(t, rec, 2)
	stop()
	<-done

	if fs[1].name != contract.WireState {
		t.Fatalf("第二条该是连接状态: %s", fs[1].name)
	}
	raw, _ := json.Marshal(fs[1].data)
	if !strings.Contains(string(raw), `"connected":true`) {
		t.Fatalf("接上了要说接上了（界面靠这一句去重新要一次快照）: %s", raw)
	}
	if !strings.Contains(string(raw), b.endpoints[0]) {
		t.Fatalf("状态里要带上连的是哪儿，权限不对时人才查得出: %s", raw)
	}
}

// 服务被人重启之后，桥要自己接回去，并把"重新接上了"这件事说出来。
// 怎么会错：不重连的话界面上的按钮每一个都回一句连不上，而服务其实早回来了——人只会以为工具坏了。
func TestReconnectsAfterTheServiceDropped(t *testing.T) {
	rec := &recorder{}
	fake := newFakeCaller()
	b := testBridge(fake, rec)

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()

	waitFrames(t, rec, 1) // 起步那一条 connected:true
	fake.drop()

	fs := waitFrames(t, rec, 3) // 断了说一句，接回来说一句
	stop()
	<-done

	var sawDown, sawUp bool
	for _, f := range fs[1:] {
		raw, _ := json.Marshal(f.data)
		switch {
		case strings.Contains(string(raw), `"connected":false`):
			sawDown = true
		case strings.Contains(string(raw), `"connected":true`):
			sawUp = true
		}
	}
	if !sawDown || !sawUp {
		t.Fatalf("断线与重连都要各说一句，实际 %v", fs)
	}
	if fake.dials() < 2 {
		t.Fatalf("重连该真去拨一次号，只拨了 %d 次", fake.dials())
	}
}

// 丢帧不许吞掉：涨一次写一行日志，带上涨到几。
// 为什么要写这一行：桥这头看不见的是"窗口画不过来"，而那正好是界面看着像跳格、查不出原因的那一档。
func TestDroppedFramesAreLogged(t *testing.T) {
	rec := &recorder{}
	fake := newFakeCaller()
	b := testBridge(fake, rec)

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()

	fake.setDropped(3)
	fake.events <- contract.Event{Name: contract.EventQueueProgress, Snapshot: contract.Snapshot{}}
	waitFrames(t, rec, 2)
	fake.setDropped(7)
	fake.events <- contract.Event{Name: contract.EventQueueProgress, Snapshot: contract.Snapshot{}}

	logs := waitLogs(t, rec, 2)
	stop()
	<-done

	if len(logs) != 2 {
		t.Fatalf("丢帧涨两次就该写两行，实际 %v", logs)
	}
	if !strings.Contains(logs[0], "3") || !strings.Contains(logs[1], "7") {
		t.Fatalf("日志要带上涨到几: %v", logs)
	}
}

// 窗口关掉之后连接要收掉。
// 怎么会错：留着就是"界面早关了、还有一条线在后台问话"，查进程时最难归因的那一类。
func TestClosingTheWindowClosesTheLink(t *testing.T) {
	rec := &recorder{}
	fake := newFakeCaller()
	b := testBridge(fake, rec)

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()

	waitFrames(t, rec, 1)
	stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("窗口关掉之后桥没停还在转")
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if !fake.closeCalled {
		t.Fatal("桥停了但连接没收掉")
	}
}

// 节拍那两个数不许漂到"等一次调用要等上半分钟"那一档；同时它必须真的传到客户端。
// 为什么钉在源码上而不是只测用例里那份：用例走的是 testBridge 那份短节拍，真壳用的常量要靠这一条现读现比。
func TestRealShellUsesTheDocumentedTimings(t *testing.T) {
	if callTimeout < 5*time.Second || callTimeout > 30*time.Second {
		t.Fatalf("一次调用的上限该在 5～30 秒之间（四个会跑久的方法在服务侧只回任务号），实际 %s", callTimeout)
	}
	if retryEvery < 200*time.Millisecond || retryEvery > 5*time.Second {
		t.Fatalf("重连节拍该在 0.2～5 秒之间，实际 %s", retryEvery)
	}
	// 真壳那条桥必须用上面这两个常量，不能测试用一套、真跑用另一套。
	// 按正则找而不按字面找：gofmt 会收掉结构体字面量里的对齐空格，字面比对跟着对齐漂。
	src := readGoFile(t, "bridge.go")
	timeoutRe := regexp.MustCompile(`timeout:\s+callTimeout`)
	retryRe := regexp.MustCompile(`retry:\s+retryEvery`)
	if !timeoutRe.MatchString(src) || !retryRe.MatchString(src) {
		t.Fatal("newBridge 没把那两个节拍接到真客户端上")
	}
}

// stringsParam 在这里只是把界面传来的 []any 折成字符串列表；
// 它跟服务侧 handlers.go 里那一份同形状，所以这里单独验一次形状，不给它另立第二个家。
func stringsParam(params map[string]any, key string) []string {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
