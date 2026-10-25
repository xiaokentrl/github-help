package service

// snapshot_test.go 钉的是"界面拿到的那一份状态"。
//
// 干什么用：界面上那一张表完全由快照画出来，快照错了用户看到的就永远是错的。
// 什么情况下会错、出错时人看得见什么：
// ① 事件只带变化的那两个字段 → 界面靠猜补全，用户在外面手工改了 hosts 也看不见；
// ② 生效那一列从服务内存里读 → 服务记一份、文件一份，早晚各说一套〔已定：D3 甲〕；
// ③ items 的顺序跟着 map 走 → 每刷一次重排一次，鼠标一伸过去行就跑了；
// ④ 文件读不动却按"没有"处理 → 界面显示"一条都没生效"，而实话是"不知道"。
// 怎么回来：组装只在 snapshot.go 那一个地方，要加字段先加进 contract 再在这里填，两处一起红。

import (
	"os"
	"strings"
	"testing"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/dnsflush"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/hosts"
)

// totalItems 是清单里全部条目数——快照每一刻都该有这么多行，一行不许少。
func (st *stack) totalItems() int { return len(st.svc.order) }

func TestEveryEventCarriesFullSnapshot(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	menuID := menuOf(t, st.catalog, item.ID)
	st.mode("甲", "用例甲", menuID, 1, "1.2.3.4")
	st.run(contract.KindDetect, []string{item.ID}, nil)

	events := st.events.all()
	if len(events) == 0 {
		t.Fatal("一条事件也没推出去")
	}
	for i, e := range events {
		if len(e.snap.Items) != st.totalItems() {
			t.Fatalf("第 %d 条事件（%s）只带了 %d 行条目，要的是全部 %d 行",
				i+1, e.name, len(e.snap.Items), st.totalItems())
		}
		if e.snap.State.Name == "" || e.snap.State.Version != "test" {
			t.Fatalf("第 %d 条事件（%s）连服务是谁都没说：%+v", i+1, e.name, e.snap.State)
		}
		if len(e.snap.Tasks) == 0 {
			t.Fatalf("第 %d 条事件（%s）没带队列那一列，进度条画不出来", i+1, e.name)
		}
	}
}

func TestSnapshotHasEveryItemOnce(t *testing.T) {
	st := newStack(t)
	snap := st.svc.Snapshot()
	if len(snap.Items) != st.totalItems() {
		t.Fatalf("快照给了 %d 行，清单里有 %d 条", len(snap.Items), st.totalItems())
	}
	seen := map[string]bool{}
	for _, it := range snap.Items {
		if seen[it.ID] {
			t.Fatalf("条目 %q 在快照里出现两次，界面就会画两行", it.ID)
		}
		seen[it.ID] = true
	}
}

// 顺序的家只有 s.order 那一条：两次快照必须一模一样。
func TestSnapshotItemOrderNeverMoves(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, nil)

	first := idOrder(st.svc.Snapshot().Items)
	second := idOrder(st.svc.Snapshot().Items)
	if strings.Join(first, ",") != strings.Join(second, ",") {
		t.Fatal("同一份状态两次快照的行序不一样")
	}
	for i, e := range st.events.all() {
		if strings.Join(first, ",") != strings.Join(idOrder(e.snap.Items), ",") {
			t.Fatalf("第 %d 条事件的行序跟快照对不上", i+1)
		}
	}
}

// 这一列的唯一来源是文件〔已定：D3 甲〕：用户在外面手工改了 hosts，界面要跟着文件说，
// 而不是继续显示"我上次写进去的那个地址"。
func TestAppliedIpComesFromFileNotMemory(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	domain := item.Domains[0]
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, nil)
	if got := st.statusOf(item.ID); got.AppliedIp != "1.2.3.4" {
		t.Fatalf("刚写完那一行显示 %q", got.AppliedIp)
	}
	// 契约 v7 那一栏：文件里有我们写的行，界面那颗 ↺ 才出来。
	// 少了这一句，「应用完却还原不了」（用户 hosts 里本来没这些域名那一档）在 Go 这侧没人挡。
	if !st.statusOf(item.ID).Managed {
		t.Fatal("刚写完的那一条没报托管：界面据此会把 ↺ 藏起来，用户撤不掉自己刚应用的这一条")
	}

	// 绕过服务，直接把文件里那一行的地址改掉——用户在别处编辑的就是这个形状。
	raw := readFile(t, st.itemsPath)
	want := "9.9.9.9 " + domain
	i := strings.Index(raw, "1.2.3.4 "+domain)
	if i < 0 {
		t.Fatalf("没在文件里找到刚写的那一行：\n%s", raw)
	}
	next := raw[:i] + want + raw[i+len("1.2.3.4 "+domain):]
	writeFile(t, st.itemsPath, next)

	got := st.statusOf(item.ID)
	if got.AppliedIp != "9.9.9.9" {
		t.Fatalf("那一行还显示 %q，文件里已经是 9.9.9.9 了", got.AppliedIp)
	}
	// 文件里的地址跟这一轮检出的最优不一样：那一档是"有更快的没应用"，不是"已生效"。
	if got.Status != contract.ItemStale {
		t.Fatalf("那一档是 %s，要的是 stale", got.Status)
	}
}

// 用户自己写的一行同样生效，但绝不能算成"我们写的"——恢复时不许动它。
func TestUserOwnLineIsNotTreatedAsManaged(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	domain := item.Domains[0]
	writeFile(t, st.itemsPath, "8.8.8.8 "+domain+"\n")

	got := st.statusOf(item.ID)
	if got.AppliedIp != "" {
		t.Fatalf("用户自己写的那一行被当成了托管行：%q", got.AppliedIp)
	}
	if got.Managed {
		t.Fatal("用户自己写的那一行报了托管：界面会给这一条长出 ↺，点了就会把人家那行删掉")
	}
	doc := hosts.Parse(readFile(t, st.itemsPath))
	if len(doc.Managed()) != 0 {
		t.Fatalf("托管名单里出现了用户的行：%+v", doc.Managed())
	}
	// 但这台机器此刻确实按那一行解析：这台机器到底指到哪儿只由 mappings 回答。
	if doc.Mappings()[domain] != "8.8.8.8" {
		t.Fatalf("用户那行没被认成当前地址：%+v", doc.Mappings())
	}
}

// 文件读不动：那一列留空（是"不知道"，不是"没有"），并且把读不动本身写进状态栏。
func TestUnreadableHostsLeavesColumnEmptyAndSaysSo(t *testing.T) {
	st := newStack(t)
	st.svc.cfg.Engine = &fakeEngine{readErr: errs.New(errs.CodeHostsRead, map[string]any{"path": "/etc/hosts"}, "读不到 hosts")}
	snap := st.svc.Snapshot()

	if snap.State.LastError == "" {
		t.Fatal("读不到文件这件事没进状态栏，界面只会显示一条都没生效")
	}
	if !strings.Contains(snap.State.LastError, errs.CodeHostsRead) {
		t.Fatalf("状态栏那句话没带码，界面凑不出人话：%q", snap.State.LastError)
	}
	// 契约 v9：码与参数要跟着这一句一起换。少换一栏的样子是"新句子配旧码"——
	// 界面拿旧码凑话，屏幕上说的是"刷 DNS 没成"，实际是 hosts 读不动。
	if snap.State.LastErrorCode != errs.CodeHostsRead {
		t.Fatalf("状态栏带的是别的码：%q", snap.State.LastErrorCode)
	}
	if snap.State.LastErrorParams["path"] != "/etc/hosts" {
		t.Fatalf("状态栏那句的参数没跟上：%v", snap.State.LastErrorParams)
	}
	// 先记一件别的、再读不动：三栏必须一起被这一次的读不动盖掉。
	st.svc.noteSideError(errs.New(errs.CodePermFlush, nil, "刷不动 DNS"))
	mixed := st.svc.Snapshot().State
	if mixed.LastErrorCode != errs.CodeHostsRead || !strings.Contains(mixed.LastError, errs.CodeHostsRead) {
		t.Fatalf("读不动被上一句盖住了：%s／%q", mixed.LastErrorCode, mixed.LastError)
	}
	// 读不动只是"此刻这一次"的事实，不写回服务那一份：否则文件好了状态栏还挂着那句过期话。
	st.svc.cfg.Engine = &fakeEngine{}
	if after := st.svc.Snapshot().State; after.LastErrorCode != errs.CodePermFlush {
		t.Fatalf("读得动之后，状态栏该回到服务记着的那一件（刷 DNS），实际是 %q／%q", after.LastErrorCode, after.LastError)
	}
	if len(snap.Items) != st.totalItems() {
		t.Fatalf("读不出文件就少给行：只有 %d 行", len(snap.Items))
	}
	for _, it := range snap.Items {
		if it.AppliedIp != "" || it.PrevAppliedIp != "" {
			t.Fatalf("读不出内容却给了地址：%+v", it)
		}
	}
}

// 「写成了但该知道」（备份没存下那一档）也要带码进状态栏——契约 v9。
//
// 干什么用：Caveat 不是失败，任务行是 done、条目那一档是已生效，只有状态栏那一格要说这件事。
// 什么情况下会错、出错时人看得见什么：只写那句 "%s: %v" 原文（上一版的形状），
// 界面切到别的语言时这一格就成了全屏唯一一句中文；而队列那一行早换成当地话了。
// 怎么回来：写入口只有 service.go 的 noteStateCaveat，红了就看它有没有把 Code／Params 两栏一起写。
func TestCaveatGoesToStatuslineWithItsCode(t *testing.T) {
	st := newStack(t)
	st.svc.noteCaveat(hosts.Caveat{Code: errs.CodeHostsNoBackup, Params: map[string]any{"path": "/backups/hosts.bak"}})

	got := st.svc.Snapshot().State
	if got.LastErrorCode != errs.CodeHostsNoBackup {
		t.Fatalf("状态栏带的是 %q，要的是 %s", got.LastErrorCode, errs.CodeHostsNoBackup)
	}
	if got.LastErrorParams["path"] != "/backups/hosts.bak" {
		t.Fatalf("参数没跟上，界面凑不出\"备份没存下（在哪儿）\"：%v", got.LastErrorParams)
	}
	// 这件事不该把别的判定带坏：原文照样留着给日志看。
	if !strings.Contains(got.LastError, errs.CodeHostsNoBackup) {
		t.Fatalf("状态栏那句原文丢了码前缀：%q", got.LastError)
	}
}

// 真引擎那五步落地的文件，用例得能直接读到——这一条不测逻辑，
// 只钉"注入的那一个临时路径确实是本次写入动的那一个文件"这个前提：它错了，上面几条全是空转。
func TestInjectedHostsPathIsTheFileThatGetsWritten(t *testing.T) {
	st := newStack(t)
	if st.engine.Path() != st.itemsPath {
		t.Fatalf("引擎写的文件不是用例注入的那一个：%q ≠ %q", st.engine.Path(), st.itemsPath)
	}
	item := st.cleanItem(t)
	st.seedBest(item, "1.2.3.4")
	st.run(contract.KindApply, []string{item.ID}, nil)

	if _, err := os.Stat(st.itemsPath); err != nil {
		t.Fatalf("应用之后那份临时文件不在：%v", err)
	}
	// 备份必须落在注入的备份目录里，绝不允许写到用户机器的配置目录。
	entries, err := os.ReadDir(st.backupPath)
	if err != nil {
		t.Fatalf("备份目录读不出来：%v", err)
	}
	if len(entries) == 0 {
		t.Fatal("动过文件却没存下备份：出事时没有依据放回原文")
	}
}

// 三条铁律里的第一条（真 /etc/hosts 一个字不碰）在这里钉住装配：
// 用例注入的路径必须在临时目录里，而且不许等于默认路径。少这一条，
// 上面那些"读文件、改文件"的用例就有直接动用户机器的那一份可能。
func TestInjectedPathIsNeverTheRealHosts(t *testing.T) {
	st := newStack(t)
	if st.svc.cfg.HostsPath == hosts.DefaultPath() {
		t.Fatal("用例把真 hosts 当成了写入目标")
	}
	if !strings.HasPrefix(st.svc.cfg.HostsPath, os.TempDir()) && !strings.HasPrefix(st.svc.cfg.HostsPath, "/tmp") {
		t.Fatalf("注入的路径不在临时目录里：%q", st.svc.cfg.HostsPath)
	}
	if st.svc.cfg.SettingsPath == "" {
		t.Fatal("设置路径没注入，用例会把偏好写进用户的配置文件")
	}
}

func idOrder(items []contract.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

// 环境那三个数（dnsService／dataDir／hostsPath）必须跟着每一份快照走。
//
// 干什么用：「关于我们」里 DNS 服务名与数据目录那两格、状态栏那句"改的是哪一份文件"全靠它。
// 什么情况下会错、出错时人看得见什么：这一路断了，界面只能继续显示写死的假值
// （mock 那一份就是 systemd-resolved / ~/.config/github-accel）——那一行看着像真话，其实是猜的，
// 而 C5 定的正是"这一行由服务回，界面不猜"。
func TestSnapshotCarriesEnvironment(t *testing.T) {
	st := newStack(t)
	snap := st.svc.Snapshot()
	if snap.State.HostsPath != st.svc.cfg.HostsPath {
		t.Fatalf("快照里的 hostsPath 不是这一份服务真写的那一个：got=%q want=%q", snap.State.HostsPath, st.svc.cfg.HostsPath)
	}
	if snap.State.DataDir == "" {
		t.Fatal("dataDir 空着：界面上那一格只能显示猜出来的一份路径")
	}
	// DNS 服务名允许认不出来（空＝界面显示"—"），但一旦有值就必须是 dnsflush 那一份判据给的，
	// 不许 service 这层自己拼一个名字。
	if got, want := snap.State.DnsService, dnsflush.ServiceName(); got != want {
		t.Fatalf("DnsService 不是 dnsflush 给的那一个：got=%q want=%q", got, want)
	}
}
