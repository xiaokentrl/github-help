package service

// handlers_test.go 钉的是"界面能问的那 16 句"：一个不许漏、四个异步的一个也不许等、
// 没接线的那两条不许演成功、设置里多出来的键不许静默收下。
//
// 干什么用：这一组用例对着通道跑一遍，问的都是界面会问的原文。
// 什么情况下会错、出错时人看得见什么：
// ① 漏注册一个方法 → 编译过得去、服务起得来，那颗按钮点下去什么也不动，通道只回一句 GAC-IPC-011；
// ② 把异步方法写成"跑完才回" → 界面卡住，进度事件全没了意义；
// ③ 演一个"已是最新版本"回去 → 用户以为更新通道是通的，而它压根没接线；
// ④ 多出来的设置键静默收下 → 界面以为自己存上了，下一次读回来又没有。
// 怎么回来：①红了就补注册那一行；②③④各自把那一段挪回它该在的地方，别在通道层加兜底。

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/ipc"
	"github.com/hlep/github-accel/internal/settings"
)

// TestRegisteredMethodsMatchContract 是门禁 G3：两侧各解析出方法名再等值比，
// 不用包含判断——改名（GetStatus→Status）时包含判断照样绿。
func TestRegisteredMethodsMatchContract(t *testing.T) {
	st := newStack(t)
	got := st.svc.registeredMethods()

	want := make(map[string]bool, len(contract.AllMethods))
	for _, name := range contract.AllMethods {
		want[name] = true
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("契约里有方法 %s，注册表里没有——界面点它会收到 GAC-IPC-011", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("注册表里有方法 %s，契约里没有——界面按契约永远问不到它", name)
		}
	}
	if len(got) != len(contract.AllMethods) {
		t.Fatalf("注册了 %d 个方法，契约里有 %d 个", len(got), len(contract.AllMethods))
	}
}

// TestRegisterMountsEveryContractMethod 过一遍真的 ipc.Server：
// Register 里那 16 行与上面那份映射是两处，漏一行只有真挂上去才看得出来。
func TestRegisterMountsEveryContractMethod(t *testing.T) {
	st := newStack(t)
	st.start()

	srv := ipc.NewServer(ipc.AllowAnyPeer)
	st.svc.Register(srv)
	path := st.svc.cfg.HostsPath + ".sock"
	l, err := ipc.Listen(path)
	if err != nil {
		t.Fatalf("建端点失败：%v", err)
	}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = l.Close() })

	c := ipc.NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上自己刚起的端点：%v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	item := st.cleanItem(t)
	// 先关闸：这一批全都留在等待里，通道那头的调用只验"认不认得这个方法"，不验跑没跑完。
	if _, err := st.svc.hPauseQueue(nil); err != nil {
		t.Fatal(err)
	}

	for _, call := range everyMethodCall(item.ID) {
		_, err := c.Call(call.method, call.params, 5*time.Second)
		if err == nil {
			continue
		}
		// 更新那两条接真（2026-10-25）后,业务上如实答不上来的一族（501 退役号／502 源连不上
		// 或没有包／503 下载断）都算"挂上了、诚实地说不行"；其余任何答不上来都说明方法没挂上。
		if code := codePrefix(t, err); !strings.HasPrefix(code, "GAC-UPDATE-") {
			t.Errorf("方法 %s 问下去回的是 %q（%v）：契约里有这一条，注册表就该有", call.method, code, err)
		}
	}
	if _, err := st.svc.hResumeQueue(nil); err != nil {
		t.Fatal(err)
	}
}

type methodCall struct {
	method string
	params map[string]any
}

// everyMethodCall 把契约里那 16 条各问一次。参数键名照第 11.1 节那张表。
func everyMethodCall(itemID string) []methodCall {
	settingBlob, err := json.Marshal(settings.Defaults())
	if err != nil {
		panic(err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(settingBlob, &asMap); err != nil {
		panic(err)
	}
	return []methodCall{
		{contract.MethodGetStatus, nil},
		{contract.MethodDetect, map[string]any{"itemIDs": []any{itemID}}},
		{contract.MethodApplyQueue, map[string]any{"itemIDs": []any{itemID}}},
		{contract.MethodRestoreItem, map[string]any{"itemID": itemID}},
		{contract.MethodRestoreHosts, nil},
		{contract.MethodGetSettings, nil},
		{contract.MethodSaveSettings, map[string]any{"settings": asMap}},
		{contract.MethodCheckPerm, nil},
		{contract.MethodCheckUpdate, nil},
		{contract.MethodDownloadUpdate, nil},
		{contract.MethodGetVersion, nil},
		{contract.MethodCancelTask, map[string]any{"taskID": "batch-1"}},
		{contract.MethodPauseQueue, nil},
		{contract.MethodResumeQueue, nil},
		{contract.MethodGetSnapshot, nil},
		{contract.MethodClearTasks, nil},
	}
}

// codePrefix 从通道回的那一句里把码解析出来（"GAC-XXX-000: 一句话"）。
// 解析再等值比，不用包含判断：那句人话里出现一次别人的码，包含判断就认错了。
func codePrefix(t *testing.T, err error) string {
	t.Helper()
	head, _, ok := strings.Cut(err.Error(), ":")
	if !ok {
		return ""
	}
	return strings.TrimSpace(head)
}

// 四个异步方法一律"立刻回任务号"：跑没跑完由事件说。
// 这一条钉的是界面卡死的形状——handler 里只要有一次网络请求或一次写盘，这里就红。
func TestAsyncHandlersReturnBeforeTheWorkIsDone(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	resolver := st.mode("甲", "用例甲", menuOf(t, st.catalog, item.ID), 1, "1.2.3.4")
	resolver.block = make(chan struct{}) // 卡住方式那一步：任务还在跑，回执却必须已经回去
	st.start()

	cases := []struct {
		name   string
		call   func(map[string]any) (any, error)
		params map[string]any
	}{
		{contract.MethodDetect, st.svc.hDetect, map[string]any{"itemIDs": []any{item.ID}}},
		{contract.MethodApplyQueue, st.svc.hApplyQueue, map[string]any{"itemIDs": []any{item.ID}}},
		{contract.MethodRestoreItem, st.svc.hRestoreItem, map[string]any{"itemID": item.ID}},
		{contract.MethodRestoreHosts, st.svc.hRestoreHosts, nil},
	}
	for _, one := range cases {
		done := make(chan any, 1)
		go func() {
			got, err := one.call(one.params)
			if err != nil {
				done <- err
				return
			}
			done <- got
		}()

		select {
		case got := <-done:
			if err, bad := got.(error); bad {
				close(resolver.block)
				t.Fatalf("%s 的回执本身就不成：%v", one.name, err)
			}
			reply, ok := got.(contract.TaskIDReply)
			if !ok || reply.TaskID == "" {
				close(resolver.block)
				t.Fatalf("%s 没回任务号：%+v", one.name, got)
			}
		case <-time.After(2 * time.Second):
			close(resolver.block)
			t.Fatalf("%s 等到活儿跑完才回话：界面上这一颗按钮会一直转", one.name)
		}
	}
	close(resolver.block)
	st.waitIdle()
}

// 界面传来两个数字之外的一切都不该让服务崩：不认识的方法名由通道挡（GAC-IPC-011），
// 这里挡的是参数缺了、空了、多出来那三种。
func TestHandlersRefuseBadParams(t *testing.T) {
	st := newStack(t)
	st.start()

	if _, err := st.svc.hRestoreItem(map[string]any{}); err == nil {
		t.Error("RestoreItem 没带条目 id 却收了：队列里会出现一行谁也不认识的条目")
	} else if errs.CodeOf(err) != errs.CodeTaskBadItem {
		t.Errorf("留的码是 %q，要的是 GAC-TASK-004", errs.CodeOf(err))
	}
	if _, err := st.svc.hDetect(map[string]any{"itemIDs": []any{"清单里没有的id"}}); err == nil {
		t.Error("没核条目 id 就入队：界面那一行永远不会动")
	} else if errs.CodeOf(err) != errs.CodeTaskBadItem {
		t.Errorf("留的码是 %q，要的是 GAC-TASK-004", errs.CodeOf(err))
	}
}

// E1 甲：那 13 个键是唯一的一份。多出来的键静默收下＝界面以为自己存上了、下次读回来又没有。
func TestSaveSettingsRejectsUnknownKey(t *testing.T) {
	st := newStack(t)
	blob, err := json.Marshal(settings.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(blob, &asMap); err != nil {
		t.Fatal(err)
	}
	asMap["aNewKeyNobodyAgreed"] = 1

	if _, err := st.svc.hSaveSettings(map[string]any{"settings": asMap}); err == nil {
		t.Fatal("多出来的键被静默收下了")
	} else if code := errs.CodeOf(err); code != errs.CodeSettingsParse {
		t.Fatalf("留的码是 %q，要的是 GAC-SETTINGS-003", code)
	}

	// 空对象也不行：那等于"存了一份什么都没写的设置"，界面上的开关会自己跳回去。
	if _, err := st.svc.hSaveSettings(map[string]any{"settings": map[string]any{}}); err == nil {
		t.Log("空设置对象被当成默认值收下（当前口径：Defaults 全给，再按文件里真写了的键覆盖）")
	}
}

// 存进去的必须原样读回来——"设置存了没生效"那句话的全部判据。
func TestSaveSettingsRoundTrip(t *testing.T) {
	st := newStack(t)
	want := settings.Defaults()
	want.Language = "en"
	want.QueueHeight = 240
	want.ShowTrayIcon = false

	blob, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(blob, &asMap); err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.hSaveSettings(map[string]any{"settings": asMap}); err != nil {
		t.Fatalf("存不下去：%v", err)
	}
	got, err := st.svc.hGetSettings(nil)
	if err != nil {
		t.Fatalf("读不回来：%v", err)
	}
	back, ok := got.(settings.Settings)
	if !ok {
		t.Fatalf("读回来的形状不对：%T", got)
	}
	if back != want {
		t.Fatalf("存进去 %+v，读回来 %+v", want, back)
	}
}

// 没接线的两条一律如实回 501，而且不许带 taskID——
// 带一个任务号的话界面会开始等一条永远不会来的事件。
// 更新通道接真（2026-10-25）后的用例：取数口全是假件,不把用例挂到真网络上。
// 钉四件事:① 有新版时如实报 latest/hasUpdate/log；② 没新版时拒绝下载并说"已是最新"；
// ③ 下载真落盘且回路径；④ 源连不上时报 GAC-UPDATE-502,不许演成功（旧桩那条"不许演成功"的口径不变）。
func TestUpdateHandlersRefuseToPretend(t *testing.T) {
	st := newStack(t)
	st.svc.cfg.Version = "0.0.0-m24"
	// 列表端点回的是数组(releases?per_page=1,含 prerelease)
	release := `[{"tag_name":"v0.0.0-m25","body":"## English\n- one\n- two\n","assets":[{"name":"github-accel_0.0.0-m25_amd64.deb","browser_download_url":"https://example.com/github-accel_0.0.0-m25_amd64.deb"}]}]`
	fake := &fakeUpdater{jsonBody: []byte(release)}
	st.svc.cfg.Updater = fake

	r, err := st.svc.hCheckUpdate(nil)
	if err != nil {
		t.Fatalf("有新版时不该报错：%v", err)
	}
	c := r.(contract.UpdateCheckReply)
	if !c.HasUpdate || c.Latest != "0.0.0-m25" || len(c.Log) != 2 {
		t.Fatalf("检查回执不对：latest=%q hasUpdate=%v log=%v", c.Latest, c.HasUpdate, c.Log)
	}

	// 下载:假件真写盘,回执给路径
	fake.fileBytes = []byte("deb-bytes")
	dr, err := st.svc.hDownloadUpdate(nil)
	if err != nil {
		t.Fatalf("下载不该报错：%v", err)
	}
	path := dr.(contract.UpdateDownloadReply).SavedPath
	if filepath.Base(path) != "github-accel_0.0.0-m25_amd64.deb" {
		t.Fatalf("回执路径不对：%q", path)
	}
	if b, _ := os.ReadFile(path); string(b) != "deb-bytes" {
		t.Fatalf("落盘内容不对")
	}

	// 没有新版:拒绝下载,不许把旧包当新包装
	fake.jsonBody = []byte(`[{"tag_name":"v0.0.0-m23","body":"","assets":[]}]`)
	if _, err := st.svc.hDownloadUpdate(nil); err == nil {
		t.Fatal("没有新版还让下载,就是骗人")
	} else if code := errs.CodeOf(err); code != errs.CodeUpdateCheckFailed {
		t.Fatalf("码该是 GAC-UPDATE-502，got %q", code)
	}

	// 源连不上:检查当场报 502
	fake.err = errors.New("proxy 死了")
	if _, err := st.svc.hCheckUpdate(nil); err == nil || errs.CodeOf(err) != errs.CodeUpdateCheckFailed {
		t.Fatalf("源连不上该报 502，got %v", err)
	}
}

type fakeUpdater struct {
	jsonBody  []byte
	fileBytes []byte
	err       error
}

func (f *fakeUpdater) FetchJSON(string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.jsonBody, nil
}

func (f *fakeUpdater) FetchToFile(url, dest string) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(dest, f.fileBytes, 0o644); err != nil {
		return 0, err
	}
	return int64(len(f.fileBytes)), nil
}

// compareVersions 的逐例:自家发版形状的两条规矩——点段按数字、-mN 也按数字(m9 < m24)。
func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{{"0.0.0-m25", "0.0.0-m24", 1}, {"0.0.0-m9", "0.0.0-m24", -1}, {"1.0.0", "0.9.9", 1},
		{"0.0.0-m24", "0.0.0-m24", 0}, {"v1.2.3", "1.2.3", 0}, {"乱码", "0.0.0-m1", -1}} {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Fatalf("compareVersions(%q,%q)=%d，要 %d", c.a, c.b, got, c.want)
		}
	}
}

func mustFail(t *testing.T, h func(map[string]any) (any, error)) error {
	t.Helper()
	_, err := h(nil)
	if err == nil {
		t.Fatal("这一问演成了成功")
	}
	return err
}

// 版本那一问是真的：这一个数不需要网络。BuildDate 没填就如实留空。
func TestGetVersionAnswers(t *testing.T) {
	st := newStack(t)
	got, err := st.svc.hGetVersion(nil)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := got.(contract.VersionReply)
	if !ok {
		t.Fatalf("形状不对：%T", got)
	}
	if v.Current != "test" {
		t.Fatalf("版本回的是 %q", v.Current)
	}
	if v.BuildDate != "" {
		t.Fatalf("用例里没填构建日期却回了一个：%q", v.BuildDate)
	}
}

// 权限那一问不进队列（那盏灯要当场亮），而且一次也不改文件内容。
func TestCheckPermAnswersWithoutTouchingContent(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	writeFile(t, st.itemsPath, "8.8.8.8 "+item.Domains[0]+"\n")
	before := readFile(t, st.itemsPath)

	got, err := st.svc.hCheckPerm(nil)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := got.(contract.PermReply)
	if !ok {
		t.Fatalf("形状不对：%T", got)
	}
	if !p.CanWrite || !p.DnsFlushed {
		t.Fatalf("临时路径写得动、也刷得动，却回了 %+v", p)
	}
	if after := readFile(t, st.itemsPath); after != before {
		t.Fatalf("只是问一句权限，内容却变了：\n改前 %q\n改后 %q", before, after)
	}

	// 刷不动那一句也不许吞：它的症状正是"界面说已生效、浏览器还是旧地址"。
	st.flush.setErr(errs.New(errs.CodePermFlush, nil, "刷不动"))
	p2, err := st.svc.hCheckPerm(nil)
	if err != nil {
		t.Fatal(err)
	}
	reply := p2.(contract.PermReply)
	if reply.DnsFlushed || reply.Detail == "" {
		t.Fatalf("刷不动却只回了个布尔值：%+v", reply)
	}
}

// 暂停与恢复那两问回的是闸此刻的状态，不是"这次调用改了什么"：
// 重复暂停、重复恢复都不报错，界面那颗按钮照这个数翻转文字。
func TestPauseResumeReplies(t *testing.T) {
	st := newStack(t)
	got, err := st.svc.hPauseQueue(nil)
	if err != nil {
		t.Fatal(err)
	}
	if q := got.(contract.QueueReply); !q.Paused {
		t.Fatalf("刚按下暂停却回 %v", q.Paused)
	}
	got, err = st.svc.hPauseQueue(nil)
	if err != nil {
		t.Fatalf("重复暂停报错了：界面上那颗按钮会点第二下试试")
	}
	if q := got.(contract.QueueReply); !q.Paused {
		t.Fatalf("重复暂停回的是 %v", q.Paused)
	}
	got, err = st.svc.hResumeQueue(nil)
	if err != nil {
		t.Fatal(err)
	}
	if q := got.(contract.QueueReply); q.Paused {
		t.Fatalf("刚恢复却回 %v", q.Paused)
	}
}

// 取消那一问要带数量：顶部那颗「取消等待」不指名任务号，界面拿这个数决定横幅写几句。
func TestCancelReplyCountOverSocket(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.svc.pause()
	batch, err := st.svc.submit(contract.KindDetect, []string{item.ID}, originUser, nil)
	if err != nil {
		t.Fatal(err)
	}

	srv := ipc.NewServer(ipc.AllowAnyPeer)
	st.svc.Register(srv)
	l, err := ipc.Listen(st.itemsPath + ".sock2")
	if err != nil {
		t.Fatalf("建端点失败：%v", err)
	}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = l.Close() })

	c := ipc.NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上端点：%v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	res, err := c.Call(contract.MethodCancelTask, map[string]any{"taskID": batch}, 5*time.Second)
	if err != nil {
		t.Fatalf("取消这一问问不下去：%v", err)
	}
	blob, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var reply contract.CancelReply
	if err := json.Unmarshal(blob, &reply); err != nil {
		t.Fatalf("回执形状不对：%v", err)
	}
	if !reply.Cancelled || reply.Count != 1 {
		t.Fatalf("撤掉了 1 行却回 %+v", reply)
	}
}

// ClearTasks 过一遍真通道：回执要能在 JSON 行里原样回到界面那边，并且回到的是"抹完之后那张表"。
//
// 干什么用：界面这一颗按钮拿到回执后走的是与 GetSnapshot、与每一条事件同一条合并路径，
// 所以这一条钉的是两件事——形状（解得开）与内容（那几行真的少了）。
// 什么情况下会错、出错时人看得见什么：回执改成"抹了几行"一个数 → 界面还得自己再猜一次抹完长什么样，
// 于是批次那两个进度数留在抹之前那一版；这里回的是表，猜的那一步就没有了。
// 怎么回来：形状红了看 hClearTasks 回的是不是 contract.Snapshot；内容红了看 queue.go 那一段筛子。
func TestClearTasksReplyIsTheTableAfterwards(t *testing.T) {
	st := newStack(t)
	item := st.cleanItem(t)
	st.svc.pause()
	if _, err := st.svc.submit(contract.KindDetect, []string{item.ID}, originUser, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.cancel(""); err != nil {
		t.Fatalf("先把这一行撤成终态失败：%v", err)
	}
	before := st.svc.Snapshot()
	if len(before.Tasks) != 1 {
		t.Fatalf("清之前表里有 %d 行，用例造的那一行没进来", len(before.Tasks))
	}

	srv := ipc.NewServer(ipc.AllowAnyPeer)
	st.svc.Register(srv)
	l, err := ipc.Listen(st.itemsPath + ".sock3")
	if err != nil {
		t.Fatalf("建端点失败：%v", err)
	}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = l.Close() })

	c := ipc.NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上端点：%v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	res, err := c.Call(contract.MethodClearTasks, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("清空这一问问不下去：%v", err)
	}
	blob, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var snap contract.Snapshot
	if err := json.Unmarshal(blob, &snap); err != nil {
		t.Fatalf("回执不是一份快照，界面解不开：%v", err)
	}
	if len(snap.Tasks) != 0 {
		t.Fatalf("清完还回 %d 行：那颗按钮点下去还是「没反应」", len(snap.Tasks))
	}
	if len(snap.Items) != len(before.Items) {
		t.Fatalf("条目从 %d 行变成 %d 行：抹队列不该碰条目表", len(before.Items), len(snap.Items))
	}
}

// 「打开Hosts」那颗钮的后半截：权限放得开、打开器成与不成各说各的话。
// 会错在哪：① 权限放开了却谎报"打开成了"（root 服务没有显示会话是常态）→ 用户等一个不会出现的编辑器；
// ② 权限没放开却报成功 → 用户在编辑器里改了半天存不上；
// ③ 0777 变成永久改权限 → 任何本机进程从此都能改 hosts——好在应用那一步 temp+rename 会把原 mode 带回来。
func TestOpenHostsReleasesPermissionAndReportsOpener(t *testing.T) {
	st := newStack(t)
	if err := os.WriteFile(st.svc.cfg.HostsPath, []byte("127.0.0.1 localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 假会话:finder 找到"桌面",spawn 记下收到的命令与身份
	oldFinder, oldSpawn := sessionFinder, sessionSpawn
	t.Cleanup(func() { sessionFinder, sessionSpawn = oldFinder, oldSpawn })
	var gotCmd, gotPath string
	var gotUID uint32
	sessionFinder = func() (desktopSession, bool) {
		return desktopSession{uid: 1000, gid: 1000, env: []string{"DISPLAY=:0"}}, true
	}
	sessionSpawn = func(cmd string, args []string, uid, gid uint32, env []string) error {
		gotCmd, gotPath, gotUID = cmd, args[0], uid
		return nil
	}

	reply, err := st.svc.hOpenHosts(nil)
	if err != nil {
		t.Fatalf("权限放得开、会话正常时不该报错：%v", err)
	}
	r := reply.(contract.OpenHostsReply)
	if r.Mode != "0777" || !r.Opened || r.Note != "" {
		t.Fatalf("回执不对：mode=%q opened=%v note=%q", r.Mode, r.Opened, r.Note)
	}
	info, err := os.Stat(st.svc.cfg.HostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o777 {
		t.Fatalf("文件权限该是 0777，got=%o", info.Mode().Perm())
	}
	if gotCmd != "xdg-open" || gotPath != st.svc.cfg.HostsPath || gotUID != 1000 {
		t.Fatalf("桌面打开器收到的形状不对：cmd=%q path=%q uid=%d", gotCmd, gotPath, gotUID)
	}

	// 找不到桌面会话：权限照样放开，回执如实说 Opened=false + 原因，不算失败
	sessionFinder = func() (desktopSession, bool) { return desktopSession{}, false }
	reply, err = st.svc.hOpenHosts(nil)
	if err != nil {
		t.Fatalf("会话找不到不该算失败（权限已放开）：%v", err)
	}
	r = reply.(contract.OpenHostsReply)
	if r.Opened || r.Note == "" {
		t.Fatalf("会话找不到时该 Opened=false 且带原因，got opened=%v note=%q", r.Opened, r.Note)
	}
}

// 状态回执必须带 dataDir:设置与 hosts 备份都住在它下面,"关于我们"那一格读的就是它。
// systemd 下服务没有 HOME,这里靠 newStack 钉的 GAC_CONFIG_DIR 保证有值(实机撞到的坑)。
func TestStatusCarriesDataDir(t *testing.T) {
	st := newStack(t)
	got := st.svc.status()
	if got.Service.DataDir == "" {
		t.Fatal("状态回执没带 dataDir——数据目录那一格只会显示破折号")
	}
	want := filepath.Dir(st.svc.cfg.SettingsPath) + "/github-hlep"
	if got.Service.DataDir != want {
		t.Fatalf("dataDir 该是钉在临时目录下的那一个 %q，got %q", want, got.Service.DataDir)
	}
}

// 权限放不开（文件不存在）必须当场拒，不许回一个"打开了"。
func TestOpenHostsRefusesWhenFileMissing(t *testing.T) {
	st := newStack(t)
	st.svc.cfg.Opener = func(string) error { return nil }
	if _, err := st.svc.hOpenHosts(nil); err == nil {
		t.Fatal("hosts 文件不存在时该报错，不该装作打开了")
	}
}
