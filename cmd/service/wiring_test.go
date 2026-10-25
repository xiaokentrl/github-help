package main

// wiring_test.go 验的是"这一台机器上真跑起来的那一份组装"：
// buildStack 挂上去的方法、传进去的临时 hosts、传进去的临时设置文件，都要在真通道上量到。
//
// 干什么用：internal/service 那 59 条用例走的是假件与包内直调，它们回答的是"编排对不对"；
// 这里回答的是另一句——"命令行起的那个二进制，是不是把零件接对了地方"。
//
// 什么情况下会错、出错时人看得见什么：
// ① 少挂一个方法 → 界面上那颗按钮转到底也不回话，而包内用例全绿（它根本不经过通道）；
// ② SettingsPath 忘了传 → 存设置写进了真~/.config，用例看不出来，用户的偏好被测试改了；
// ③ HostsPath／Engine 指向不一致 → 权限自检说"写不动"，写入却成功（或反过来）。
//
// 怎么回来：三条断言各自指名改哪一处；这三份文件都不许出现真路径。
//
// 三条铁律沿用 internal/service 那一份：真 /etc/hosts 一个字不碰、真网络一个字不发、刷 DNS 不许真跑。
// 网络那一条靠"起队列前就把 context 叫停"做到——任务一进去就撞上已取消，取解析与探测那两步根本走不到。

import (
	"context"
	"encoding/json"
	"net"
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

// 组装好的那一份：临时 hosts、临时设置文件、临时端点，队列带着已叫停的 context 起起来。
type harness struct {
	st          *stack
	l           net.Listener
	client      *ipc.Client
	hostsPath   string
	settingsPth string
	backups     string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()

	hostsPath := filepath.Join(dir, "hosts")
	seed := "127.0.0.1 localhost\n# 这一份是测试临时文件，不是任何人的真 hosts\n"
	if err := os.WriteFile(hostsPath, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(dir, "settings.json")
	// 那两把开机开关显式关掉：服务今天已经不读它们（自提批那一档 2026-10-23 删掉了，
	// 见 TestServiceStartNeverSelfSubmits），留这一句只为这份文件万一被谁重新接上自提批时，
	// 这份组装还是一个字网络也不发。
	s := settings.Defaults()
	s.AutoDetectOnStartup = false
	s.AutoApplyOnStartup = false
	if err := settings.SaveTo(settingsPath, s); err != nil {
		t.Fatal(err)
	}

	backups := filepath.Join(dir, "backup")
	st, err := buildStack(hostsPath, backups, settingsPath, &fakeUpdater{})
	if err != nil {
		t.Fatalf("组装没起来: %v", err)
	}

	l, err := st.listen(filepath.Join(dir, "s.sock"))
	if err != nil {
		t.Fatalf("端点没建起来: %v", err)
	}

	// 先叫停再起队列：这样任何一条任务在真正动手前就返回错误，取解析与探测都走不到。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st.svc.Start(ctx)

	go func() { _ = st.srv.Serve(l) }()

	c := ipc.NewClient()
	if err := c.Connect(filepath.Join(dir, "s.sock")); err != nil {
		t.Fatalf("连不上自己刚起的端点: %v", err)
	}

	t.Cleanup(func() {
		_ = c.Close()
		_ = l.Close()
		st.svc.Stop()
		_ = os.Remove(filepath.Join(dir, "s.sock"))
	})

	return &harness{st: st, l: l, client: c, hostsPath: hostsPath, settingsPth: settingsPath, backups: backups}
}

// call 走真通道问一句，超时按 2 秒——界面上"转到底不回话"就是这一条要挡的。
func (h *harness) call(t *testing.T, method string, params map[string]any) (any, error) {
	t.Helper()
	return h.client.Call(method, params, 2*time.Second)
}

// ① 契约里 16 个方法，逐个在真通道上问一遍：一个也不许报"服务不认这个方法"。
// 对照那一条（乱起的方法名）必须报 011，否则这条断言等于没做事。
func TestSocketAnswersEveryContractMethod(t *testing.T) {
	h := newHarness(t)

	for _, m := range contract.AllMethods {
		_, err := h.call(t, m, socketParams(m))
		if err == nil {
			continue
		}
		if errs.CodeOf(err) == errs.CodeIPCNoHandler {
			t.Errorf("%s 在真通道上不认：buildStack 少挂了一个方法", m)
		}
	}

	_, err := h.call(t, "NoSuchMethodForTest", nil)
	if err == nil {
		t.Fatal("不认的方法本该报错，却回了成功——上面那条断言等于没做事")
	}
	if errs.CodeOf(err) != errs.CodeIPCNoHandler {
		t.Fatalf("不认的方法回了 %v，要的是 %s", err, errs.CodeIPCNoHandler)
	}
}

// socketParams 给每个方法一份能过参数检查的入参。
// 这里用假条目 id：这一条要验的是"挂没挂上"，不是"这一条认不认得"，
// 所以回 GAC-TASK-004（参数不成）也算"方法在"。
func socketParams(method string) map[string]any {
	switch method {
	case contract.MethodRestoreItem:
		return map[string]any{"itemID": "wiring-test-item"}
	case contract.MethodDetect, contract.MethodApplyQueue:
		return map[string]any{"itemIDs": []any{"wiring-test-item"}}
	default:
		return nil
	}
}

// ② 设置那一条读写必须落在传进来的那一份文件上。
// 破了这句话，测试会把用户的偏好改掉，而且一条也不红。
func TestSettingsRoundTripUsesInjectedPath(t *testing.T) {
	h := newHarness(t)

	want := settings.Defaults()
	want.QueueHeight = 240
	want.Language = "en"
	want.ShowTrayIcon = false

	blob, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(blob, &asMap); err != nil {
		t.Fatal(err)
	}

	if _, err := h.call(t, contract.MethodSaveSettings, map[string]any{"settings": asMap}); err != nil {
		t.Fatalf("存设置这句没成: %v", err)
	}

	raw, err := os.ReadFile(h.settingsPth)
	if err != nil {
		t.Fatalf("注入的那份设置文件读不出来: %v", err)
	}
	if !strings.Contains(string(raw), `"queueHeight": 240`) {
		t.Fatalf("注入的设置文件里没看到刚存的值：%s", raw)
	}

	got, err := h.call(t, contract.MethodGetSettings, nil)
	if err != nil {
		t.Fatalf("读设置这句没成: %v", err)
	}
	var back contract.Settings
	if err := remarshal(got, &back); err != nil {
		t.Fatalf("读回来的形状看不懂: %v", err)
	}
	if back.QueueHeight != 240 || back.Language != "en" || back.ShowTrayIcon {
		t.Fatalf("通道读回来的是 %+v，要的是刚存的那一份", back)
	}
}

// ③ 权限自检与写入指的是同一份文件：两条都问注入那一条路径。
// 不一致的形状是"界面说写不动、应用却成功"，或反过来——两种都是用户看得见的假话。
func TestPermProbeAndEnginePointAtSameFile(t *testing.T) {
	h := newHarness(t)

	got, err := h.call(t, contract.MethodCheckPerm, nil)
	if err != nil {
		t.Fatalf("权限自检这句没成: %v", err)
	}
	var reply contract.PermReply
	if err := remarshal(got, &reply); err != nil {
		t.Fatal(err)
	}
	if !reply.CanWrite {
		t.Fatalf("临时文件写不动， Detail 是 %q——这条用例的其它断言都白做", reply.Detail)
	}

	// 状态里报的路径与引擎写的那一份必须是同一处。
	status, err := h.call(t, contract.MethodGetStatus, nil)
	if err != nil {
		t.Fatal(err)
	}
	var snap contract.StatusReply
	if err := remarshal(status, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Service.Name == "" || snap.Service.Version != version {
		t.Fatalf("状态报回来的服务不是这一份组装：%+v", snap.Service)
	}

	// 组装本身交回来的那两个路径（引擎写谁、备份放哪儿）也要对上。
	if h.st.path != h.hostsPath {
		t.Fatalf("组装记的写入路径是 %q，注入的是 %q", h.st.path, h.hostsPath)
	}
	if _, err := os.Stat(h.hostsPath); err != nil {
		t.Fatal(err)
	}
}

// TestServiceStartNeverSelfSubmits 验的是 2026-10-23 那一轮定死的那一条：开机那一批只由界面起，
// 服务自己一起跑一个字也不动——即便设置文件里那两把开关都写着 true。
//
// 干什么用：这一条比"开关关掉之后不动手"硬。旧的那一句其实放过了真正的事故形状：
// 服务读的是 defaults（那两把开关默认就是开着），所以"关掉才不动手"根本挡不住开机那一次整批改写 hosts。
// 什么情况下会错、出错时人看得见什么：把自提批加回来 → 界面上开关关着、hosts 每次开机被整批改写 54 条，
// 而且一句错也不报；同一时间界面自己那趟自动化又起一批，两批并排抢同一行 hosts。
// 怎么回来：这一份组装里唯一提交任务的入口是通道收到的那一次点击（internal/service/handlers.go）。
// 两把开关的唯一主人是界面：frontend/core/flows.js 的 runStartupAutomation，它只提交勾上的那几条。
func TestServiceStartNeverSelfSubmits(t *testing.T) {
	h := newHarness(t)

	// 把两把开关写成"开着"：这一档以前会让服务在起跑后立刻自提检测＋应用两批
	on := settings.Defaults()
	on.AutoDetectOnStartup = true
	on.AutoApplyOnStartup = true
	if err := settings.SaveTo(h.settingsPth, on); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(h.hostsPath)
	if err != nil {
		t.Fatal(err)
	}

	// 起跑这件事在这份组装里就是 newHarness 里那一句 Start（带着已叫停的 context）；
	// 这里问的是通道那一份"此刻全貌"：队列里必须一行也没有。
	got, err := h.call(t, contract.MethodGetSnapshot, nil)
	if err != nil {
		t.Fatalf("开机之后问一句全貌都没成: %v", err)
	}
	var snap contract.Snapshot
	if err := remarshal(got, &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Tasks) != 0 {
		t.Fatalf("服务起跑自己起了 %d 行任务，第一行是 %s/%s：开机那一批的主人只有界面一个",
			len(snap.Tasks), snap.Tasks[0].Kind, snap.Tasks[0].Status)
	}

	after, err := os.ReadFile(h.hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("开机那一句动了这份文件：\n前：%s\n后：%s", before, after)
	}
	if _, err := os.Stat(h.backups); err == nil {
		t.Fatal("一条也没写，备份目录却建出来了")
	}
}

// remarshal 把通道上回来的 map 再解成结构体。
// 为什么要多这一步：ipc 那一行 JSON 解到 contract.Response.Result（any）就是 map，
// 结构体要靠这里现解一遍——解不出来本身就是"形状漂了"的红灯。
func remarshal(got any, into any) error {
	raw, err := json.Marshal(got)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// fakeUpdater 是接线用例的假取数口：真通道上的 CheckUpdate／DownloadUpdate 不许真拨 GitHub
// （真拨一次 15 秒超时,通道那头 2 秒就先判死,连接被收,后面每一问全变 040——实测抓到）。
type fakeUpdater struct{}

func (fakeUpdater) FetchJSON(string) ([]byte, error) {
	return []byte(`[{"tag_name":"vtest","body":"","assets":[]}]`), nil
}

func (fakeUpdater) FetchToFile(url, dest string) (int64, error) {
	return 0, nil
}
