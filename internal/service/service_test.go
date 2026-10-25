package service

// 这一份是 service 全部用例共用的假件。为什么全放在一个文件里：
// 这四个出口（Engine／Inspector／ModeSource／Flusher）就是本包对外认识的全部东西，
// 假件散到各测试文件就会长出第二份形状，改一个接口要修五处。
//
// 三条铁律（跟用例同等重要，改这里之前先读完）：
// ① 真 /etc/hosts 一个字不碰——引擎一律指 t.TempDir() 里的路径；
// ② 真网络一个字不发——方式与探测器全是假件，resolve 的注册表在用例里根本不出现；
// ③ 刷 DNS 不许真跑——用 fakeFlusher，它还能造"刷不动"那一档。

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/detect"
	"github.com/hlep/github-accel/internal/hosts"
	"github.com/hlep/github-accel/internal/menu"
	"github.com/hlep/github-accel/internal/resolve"
)

// 清单只读一次：embed 里那 11 块菜单是数据，用例不改它，每条用例重读纯属浪费。
var (
	catalogOnce sync.Once
	catalogDone *menu.Catalog
	catalogErr  error
)

func testCatalog(t *testing.T) *menu.Catalog {
	t.Helper()
	catalogOnce.Do(func() {
		catalogDone, catalogErr = menu.Load()
	})
	if catalogErr != nil {
		t.Fatalf("读清单失败，用例没法跑：%v", catalogErr)
	}
	return catalogDone
}

// recordedEvent 是假件收到的一条推送。
type recordedEvent struct {
	name    string
	snap    contract.Snapshot
	payload map[string]any
}

type fakeBroadcaster struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (f *fakeBroadcaster) Broadcast(event string, snap contract.Snapshot, payload map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, recordedEvent{name: event, snap: snap, payload: payload})
}

func (f *fakeBroadcaster) all() []recordedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedEvent(nil), f.events...)
}

// phasesOf 取出这一行任务被推进过哪几个阶段（按事件顺序，不去重）。
// 用例用它钉"四个阶段一个不少、顺序不许换"。
func (f *fakeBroadcaster) phasesOf(taskID string) []string {
	var out []string
	for _, e := range f.all() {
		if e.name != contract.EventQueueProgress {
			continue
		}
		if e.payload["taskId"] != taskID {
			continue
		}
		if p, ok := e.payload["phase"].(string); ok && p != "" {
			out = append(out, p)
		}
	}
	return out
}

// wait 轮询到出现满足条件的事件为止。为什么轮询而不是 channel：
// 一条用例里推事件的地方有七八处，用 channel 就得决定"哪一条该收第几个"，那种断言一重写就废。
func (f *fakeBroadcaster) wait(t *testing.T, within time.Duration, ok func(recordedEvent) bool) recordedEvent {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		for _, e := range f.all() {
			if ok(e) {
				return e
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("等事件超时（已收到 %d 条）", len(f.all()))
		}
		time.Sleep(time.Millisecond)
	}
}

type probeCall struct {
	domain string
	ips    []string
	limits detect.Limits
}

// fakeInspector 把 detect 那一层的读数钉死。
//
// 为什么必须钉：真探测要往外发连接，"暂停停在方式之间"这种时序判据在真网络上只能碰运气。
type fakeInspector struct {
	mu    sync.Mutex
	calls []probeCall
	// byDomain 钉死某一条域名的读数；没配的用 fallback。
	byDomain map[string]detect.Result
	fallback detect.Result
	// before 在每次读数交回去之前跑一次（用例用它造"跑到第几次时有人按暂停"）。
	before func(n int)
}

func newFakeInspector() *fakeInspector {
	return &fakeInspector{byDomain: map[string]detect.Result{}}
}

func (f *fakeInspector) Check(ctx context.Context, domain string, ips []string, lim detect.Limits) detect.Result {
	f.mu.Lock()
	f.calls = append(f.calls, probeCall{domain: domain, ips: append([]string(nil), ips...), limits: lim})
	n := len(f.calls)
	res, ok := f.byDomain[domain]
	if !ok {
		res = f.fallback
	}
	f.mu.Unlock()

	if f.before != nil {
		f.before(n)
	}
	// 补齐三个由"这一批候选"推出来的字段：假件也要照真件的形状交回，
	// 否则用例钉的是假件的形状，detect 那边一漂这里照样绿。
	if res.Domain == "" {
		res.Domain = domain
	}
	if res.ProbeCount == 0 {
		res.ProbeCount = len(ips) * (1 + lim.RTTRepeats)
	}
	out := detect.Result{
		Domain: res.Domain, Best: res.Best, RTT: res.RTT,
		Outcomes: res.Outcomes, ProbeCount: res.ProbeCount, Code: res.Code, Params: res.Params,
	}
	return out
}

func (f *fakeInspector) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeInspector) domains() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, c.domain)
	}
	return out
}

// fakeResolver 是一种解析方式的假件：交回的候选地址与耗时都由用例钉死。
type fakeResolver struct {
	spec resolve.Spec

	mu    sync.Mutex
	calls int
	// byDomain 钉死某条域名的候选；没配的用 ips。
	byDomain map[string][]string
	ips      []string
	err      error
	took     time.Duration
	// block 非空时 Resolve 先等它被关掉（用例用它造"这一条正跑着，别的先排着"那个时序）。
	block chan struct{}
}

func (f *fakeResolver) Spec() resolve.Spec { return f.spec }

func (f *fakeResolver) Resolve(ctx context.Context, q resolve.Query) (resolve.Set, error) {
	f.mu.Lock()
	f.calls++
	err := f.err
	ips, ok := f.byDomain[q.Domain]
	if !ok {
		ips = f.ips
	}
	took := f.took
	f.mu.Unlock()

	if err != nil {
		return resolve.Set{}, err
	}
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return resolve.Set{}, ctx.Err()
		}
	}
	// 真方式在交回之前会先吃 ctx 的叫停。假件照做，否则"取消之后方式那一层还在跑"这类漂移
	// 在本包里根本测不出来。
	if ctx.Err() != nil {
		return resolve.Set{}, ctx.Err()
	}
	return resolve.Set{ModeID: f.spec.ID, IPs: append([]string(nil), ips...), Took: took}, nil
}

func (f *fakeResolver) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeModes 是 ModeSource 的假件——用例靠它把"有哪几种方式参选"钉死。
type fakeModes struct {
	mu    sync.Mutex
	specs []resolve.Spec
	res   map[string]*fakeResolver
}

func newFakeModes() *fakeModes {
	return &fakeModes{res: map[string]*fakeResolver{}}
}

func (m *fakeModes) Specs() []resolve.Spec {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]resolve.Spec(nil), m.specs...)
}

func (m *fakeModes) Resolver(id string) (resolve.Resolver, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.res[id]
	if !ok {
		return nil, false
	}
	return r, true
}

// add 登记一种方式并交回它的假件，方便用例继续改候选地址。
// Kinds 由用例填对（填错了 applicable 就把它滤掉，那也是有用例钉着的形状）。
func (m *fakeModes) add(spec resolve.Spec, ips ...string) *fakeResolver {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := &fakeResolver{spec: spec, ips: append([]string(nil), ips...), byDomain: map[string][]string{}}
	m.specs = append(m.specs, spec)
	m.res[spec.ID] = r
	return r
}

type fakeFlusher struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (f *fakeFlusher) Flush(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.err
}

func (f *fakeFlusher) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeFlusher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeEngine 是 hosts.Engine 的假件，只在"真引擎造不出的形状"那几条用例里用：
// 写完又用备份还原回去（核对不上）、撤了但行还在、读不出来（快照那一列要留空）。
// 正常链路一律用真引擎 + 临时路径，免得用例钉的是假件而不是那五步。
type fakeEngine struct {
	mu sync.Mutex
	// text 是当前文件内容（内存里，不落盘）。
	text string
	// rollback 为真＝算出改动却不落地，读回去还是动手前那份——
	// 这正是真引擎"重读对不上就用备份还原"那一步交回来的形状（那一步自己有 hosts 的用例）。
	rollback bool
	// readErr／applyErr 非空＝那一步当场失败。
	readErr  error
	applyErr error
	restored [][]string
	applying []hosts.Change
	// restoreNoop 为真＝Restore 什么也不改（只用来造"撤了之后自己的行还在"那一档：
	// 真引擎撤得掉，这一档只能靠假件）。
	restoreNoop bool
	// block 非空时 Apply 先等它被关掉——写入锁那一条用例要的是"这边还在写，那边又来一次"。
	block chan struct{}
}

func (e *fakeEngine) Read() (*hosts.Doc, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.readErr != nil {
		return nil, e.readErr
	}
	return hosts.Parse(e.text), nil
}

func (e *fakeEngine) Apply(ctx context.Context, changes []hosts.Change) (hosts.Receipt, error) {
	e.mu.Lock()
	if e.applyErr != nil {
		err := e.applyErr
		e.mu.Unlock()
		return hosts.Receipt{}, err
	}
	block := e.block
	e.mu.Unlock()

	if block != nil {
		<-block
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	doc := hosts.Parse(e.text)
	rendered, err := hosts.Plan(doc, changes)
	if err != nil {
		return hosts.Receipt{}, err
	}
	before := doc.Render()
	if !e.rollback {
		e.text = rendered
	}
	e.applying = append(e.applying, changes...)
	return hosts.Receipt{Path: "内存", ChangedLines: diffLines(before, rendered)}, nil
}

func (e *fakeEngine) Restore(ctx context.Context, domains []string) (hosts.Receipt, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.restored = append(e.restored, append([]string(nil), domains...))
	if e.restoreNoop {
		return hosts.Receipt{Path: "内存"}, nil
	}
	changes := make([]hosts.Change, 0, len(domains))
	for _, d := range domains {
		changes = append(changes, hosts.Change{Domain: d, IP: ""})
	}
	doc := hosts.Parse(e.text)
	rendered, err := hosts.Plan(doc, changes)
	if err != nil {
		return hosts.Receipt{}, err
	}
	before := doc.Render()
	e.text = rendered
	return hosts.Receipt{Path: "内存", ChangedLines: diffLines(before, rendered)}, nil
}

func diffLines(a, b string) int {
	same := map[string]int{}
	for _, l := range split(a) {
		same[l]++
	}
	n := 0
	for _, l := range split(b) {
		if same[l] > 0 {
			same[l]--
			continue
		}
		n++
	}
	for _, c := range same {
		n += c
	}
	return n
}

func split(text string) []string {
	if text == "" {
		return nil
	}
	out := []string{}
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			out = append(out, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

// stack 是一条用例的全套装配：真清单 + 临时 hosts + 三个假件 + 一个事件收集器。
type stack struct {
	t       *testing.T
	svc     *Service
	events  *fakeBroadcaster
	insp    *fakeInspector
	modes   *fakeModes
	flush   *fakeFlusher
	engine  *hosts.FileEngine
	catalog *menu.Catalog
	// itemsPath 是那份临时 hosts 的路径（用例要直接读文件内容时用它）。
	itemsPath string
	// backupPath 是那份临时备份目录：写入前那一份一定存在这里，绝不许落到用户机器的配置目录。
	backupPath string
}

func newStack(t *testing.T) *stack {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	backup := filepath.Join(dir, "backup")
	st := &stack{
		t:          t,
		events:     &fakeBroadcaster{},
		insp:       newFakeInspector(),
		modes:      newFakeModes(),
		flush:      &fakeFlusher{},
		engine:     hosts.NewFileEngine(path, backup),
		catalog:    testCatalog(t),
		itemsPath:  path,
		backupPath: backup,
	}
	st.svc = New(Config{
		Catalog:      st.catalog,
		Engine:       st.engine,
		Inspector:    st.insp,
		Modes:        st.modes,
		Flusher:      st.flush,
		Broadcaster:  st.events,
		HostsPath:    path,
		SettingsPath: filepath.Join(dir, "settings.json"),
		Version:      "test",
	})
	return st
}

// start 放出那条串行通道，并保证用例结束时队列真的停下来（不然 goroutine 会带着临时目录跑）。
func (st *stack) start() {
	st.svc.Start(context.Background())
	st.t.Cleanup(st.svc.Stop)
}

// mode 登记一种参选方式，缺的字段按用例最常用的形状填好。
func (st *stack) mode(id, title, menuID string, prio int, ips ...string) *fakeResolver {
	spec := resolve.Spec{ID: id, Title: title, Kinds: []string{menuID}, NeedsProbe: true,
		DefaultPriority: prio, Rankable: true}
	return st.modes.add(spec, ips...)
}

// cleanItem 取一条"没有域名冲突、也没有非法值"的条目，正常链路都用它。
func (st *stack) cleanItem(t *testing.T) menu.Item {
	t.Helper()
	bad := st.badValues()
	for _, m := range st.catalog.Menus() {
		for _, it := range m.Items {
			if st.hasConflict(it) || len(bad[it.ID]) > 0 {
				continue
			}
			return it
		}
	}
	t.Fatal("清单里找不出一条干净的条目，用例形状得改")
	return menu.Item{}
}

// badValues 把清单里那几条非法数据项按条目 id 归一下，只给用例当索引用。
func (st *stack) badValues() map[string][]string {
	out := map[string][]string{}
	for _, u := range st.catalog.UnsupportedDomains() {
		out[u.ItemID] = append(out[u.ItemID], u.Value)
	}
	return out
}

func (st *stack) hasConflict(it menu.Item) bool {
	for _, d := range it.Domains {
		for _, c := range st.catalog.DomainConflicts() {
			if c.Domain == d {
				return true
			}
		}
	}
	return false
}

// conflictItem 取一条真带域名冲突的条目（清单里实测有，用它钉 GAC-HOSTS-007）。
func (st *stack) conflictItem(t *testing.T) menu.Item {
	t.Helper()
	cf := st.catalog.DomainConflicts()
	if len(cf) == 0 {
		t.Skip("清单里目前没有域名冲突这一档，用例改天再跑")
	}
	it, _, ok := st.catalog.ByItem(cf[0].Items[0])
	if !ok {
		t.Fatalf("冲突记录里的条目 id %q 在清单里找不到", cf[0].Items[0])
	}
	return it
}

// badValueItem 取一条真带非法数据项的条目（钉 GAC-HOSTS-008）。
func (st *stack) badValueItem(t *testing.T) menu.Item {
	t.Helper()
	list := st.catalog.UnsupportedDomains()
	if len(list) == 0 {
		t.Skip("清单里目前没有非法数据项这一档")
	}
	it, _, ok := st.catalog.ByItem(list[0].ItemID)
	if !ok {
		t.Fatalf("非法值记录里的条目 id %q 在清单里找不到", list[0].ItemID)
	}
	return it
}

// waitIdle 等队列空转（既没有在跑的也没有等待中的）。超时就直接红——它意味着"这一条永远不会动"，
// 正是界面上"点下去没反应"那个症状，不该靠延长超时蒙过去。
// 为什么轮询而不是加一个 done channel：队列里"收工"的形状有三种（跑完、跳过、暂停在那儿不动），
// 一个 channel 只能表达其中一种，测试就会开始猜。
func (st *stack) waitIdle() {
	st.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		st.svc.mu.Lock()
		idle := st.svc.idleLocked()
		st.svc.mu.Unlock()
		if idle {
			return
		}
		if time.Now().After(deadline) {
			st.t.Fatalf("队列没收工（已收到 %d 条事件）", len(st.events.all()))
		}
		time.Sleep(time.Millisecond)
	}
}

// statusOf 从最近一份快照里取某一条条目的那一行（用例据此看界面会画成什么样）。
func (st *stack) statusOf(itemID string) contract.Item {
	st.t.Helper()
	for _, it := range st.svc.Snapshot().Items {
		if it.ID == itemID {
			return it
		}
	}
	st.t.Fatalf("快照里没有条目 %q", itemID)
	return contract.Item{}
}

// taskByID 按任务 id 取队列那一行。
func (st *stack) taskByID(id string) contract.Task {
	st.t.Helper()
	for _, tk := range st.svc.Snapshot().Tasks {
		if tk.ID == id {
			return tk
		}
	}
	st.t.Fatalf("队列里没有任务 %q", id)
	return contract.Task{}
}

func (st *stack) taskByStatus(status string) []contract.Task {
	var out []contract.Task
	for _, tk := range st.svc.Snapshot().Tasks {
		if tk.Status == status {
			out = append(out, tk)
		}
	}
	return out
}

// restoreTask 取某一条条目最近那一行恢复任务（一批里有几行同 id 的用例靠它挑）。
func (st *stack) restoreTask(itemID string) contract.Task {
	st.t.Helper()
	var found *contract.Task
	for _, tk := range st.svc.Snapshot().Tasks {
		if tk.Kind == contract.KindRestore && tk.ItemID == itemID {
			cp := tk
			found = &cp
		}
	}
	if found == nil {
		st.t.Fatalf("队列里没有条目 %q 的恢复任务", itemID)
	}
	return *found
}

// runningID 问一句此刻哪一行在跑（空串＝没有）。
func (s *Service) runningID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running == nil {
		return ""
	}
	return s.running.id
}

// menuOf 取某条条目属于哪块菜单——用例给方式填 Kinds 时要的就是这一个数。
func menuOf(t *testing.T, cat *menu.Catalog, itemID string) string {
	t.Helper()
	_, m, ok := cat.ByItem(itemID)
	if !ok {
		t.Fatalf("清单里没有条目 %q", itemID)
	}
	return m.ID
}

// terminalCount 数一遍状态表里有几个终态。它跟 isTerminal 是两处各数一次，
// 所以留一条用例钉住"两处必须一起改"（改状态表的人多半不会想起另一处）。
func terminalCount() int {
	n := 0
	for _, to := range moves {
		if len(to) == 0 {
			n++
		}
	}
	return n
}
