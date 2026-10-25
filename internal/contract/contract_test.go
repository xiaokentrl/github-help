package contract

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 这两支正则是"源码里声明了哪些方法名／事件名"的取据方式：比对的是标识符与它的值，
// 不是拿字符串去 Contains——改个名字也能过的检查等于没检查。
var (
	methodRe = regexp.MustCompile(`(?m)^\t(Method[A-Za-z]+)\s+=\s+"([^"]+)"$`)
	eventRe  = regexp.MustCompile(`(?m)^\t(Event[A-Za-z]+)\s+=\s+"([^"]+)"$`)
)

// 方法清单钉死成 17 条（契约 v0.1 那 11 条 + 定档轮 D7 甲补的 CancelTask + 队列暂停那两条 + 联调轮补的
// GetSnapshot、ClearTasks + 2026-10-25 补的第 17 条 OpenURL，「关于我们」三颗入口钮接真）。
// 谁想再加一条，这条测试先红，目的就是逼他去改文档、走版本号，而不是在代码里悄悄扩契约。
func TestMethodListIsTheFrozenSeventeen(t *testing.T) {
	want := []string{
		"ApplyQueue", "CancelTask", "CheckPermissions", "CheckUpdate", "ClearTasks", "Detect", "DownloadUpdate",
		"GetSettings", "GetSnapshot", "GetStatus", "GetVersion", "OpenURL", "PauseQueue", "RestoreHosts",
		"RestoreItem", "ResumeQueue", "SaveSettings",
	}
	// 加了方法必须同时把版本号推上去，否则握手那一步拦不住新旧两侧。
	// 这里钉的是"这一份 16 条清单是哪一档定下的"：v7、v8、v9 三档都没加方法、只补了字段，
	// 所以钉的数字跟着版本号走，而"加一条方法不动版本号"照样会在这里红。
	if ProtocolVersion != 10 {
		t.Fatalf("清单里已经有 %d 个方法，ProtocolVersion 却还是 %d", len(want), ProtocolVersion)
	}
	if len(AllMethods) != len(want) {
		t.Fatalf("方法清单不是 %d 条，got=%d", len(want), len(AllMethods))
	}
	seen := map[string]bool{}
	for _, m := range AllMethods {
		if seen[m] {
			t.Fatalf("方法名重复：%s", m)
		}
		seen[m] = true
	}
	got := append([]string(nil), AllMethods...)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("方法名与契约对不上：\ngot  %v\nwant %v", got, want)
	}
}

// 事件清单＝契约 v0.1 那 4 条 + v0.2 增补的「单项检测完成」，一条不许多、一条不许少。
func TestEventListIsTheFrozenFive(t *testing.T) {
	want := []string{"detect:progress", "download:progress", "network:change", "queue:done", "queue:progress"}
	got := append([]string(nil), AllEvents...)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("事件名与契约对不上：got=%v want=%v", got, want)
	}
}

// 壳与页面之间那三条线钉死成三条。
//
// 干什么用：这三条是界面每一次点击、以及每一句"这条桥通不通"的走线名字。
// 什么情况下会错、出错时人看得见什么：改一个字母不会有人报错——页面发出去的那一句没人听，
// 症状是"每个按钮点下去都石沉大海"。它比"服务不认这个方法"更难查，因为那边至少还回一句 GAC-IPC-011，
// 而这一侧连回执都没有。页面那一份对照装在 frontend/tests/contract.test.mjs。
// 怎么回来：想加第四条线先回这一处改清单并推版本号；不许在壳里另起一个名字。
func TestWireNamesAreTheFrozenThree(t *testing.T) {
	want := []string{"gac:invoke", "gac:reply", "gac:state"}
	src, err := os.ReadFile("contract.go")
	if err != nil {
		t.Fatalf("读不到 contract.go，这条检查就没法做：%v", err)
	}
	declared := []string{}
	for _, m := range regexp.MustCompile(`(?m)^\t(Wire[A-Za-z]+)\s+=\s+"([^"]+)"`).FindAllStringSubmatch(string(src), -1) {
		declared = append(declared, m[2])
	}
	sort.Strings(declared)
	if !reflect.DeepEqual(declared, want) {
		t.Fatalf("壳与页面之间的线变了：源码里是 %v，两边说好的只有那三条 %v", declared, want)
	}
	got := []string{WireInvoke, WireReply, WireState}
	sort.Strings(got)
	if !reflect.DeepEqual(got, declared) {
		t.Fatalf("那三个常量与源码里那三条对不上：%v vs %v", got, declared)
	}
}

// LinkState 那四栏是页面上那一盏连接灯的全部原料。
//
// 干什么用：界面上"连不上"那一格要写得出连的是哪一条线、累计丢了几帧，原料只有这四栏。
// 什么情况下会错、出错时人看得见什么：多一栏＝界面读不到它，那句话永远缺一样；
// 少一栏＝壳回的那一句里这一栏没了，界面只能拿一句空话说"连不上"。两种都不报错。
// 怎么回来：改这一处必须同轮改 frontend/core/contract/types.d.ts，那边有一条等值比对在等。
func TestLinkStateFieldsAreTheFrozenFour(t *testing.T) {
	want := []string{"connected", "dropped", "endpoint", "err"}
	tags := []string{}
	ft := reflect.TypeOf(LinkState{})
	for i := 0; i < ft.NumField(); i++ {
		tag := strings.Split(ft.Field(i).Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			t.Fatalf("LinkState 的 %s 没有 json 标签：界面按标签名读，读不到就是空", ft.Field(i).Name)
		}
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	if !reflect.DeepEqual(tags, want) {
		t.Fatalf("LinkState 的栏位变了：%v，两边说好的是 %v", tags, want)
	}
}

// 常量与清单只能有一个家：AllMethods 必须是源码里那批 Method* 常量的全集，一条不漏一条不多。
// 为什么单独钉这一条，而且要去读源码而不是读测试里手抄的清单：
// 有人加了一个方法常量却没往 AllMethods 里写，服务启动那步"每个方法都注册了没有"就照着少的清单查，
// 于是界面真能调、检查却说没问题——漏的那条永远没人注册。手抄的清单查不出这种漏（这条测试自己也一样会漏）。
func TestListMatchesTheConstantsInSource(t *testing.T) {
	src, err := os.ReadFile("contract.go")
	if err != nil {
		t.Fatalf("读不到 contract.go，这条检查就没法做: %v", err)
	}
	declared := map[string]bool{}
	for _, m := range methodRe.FindAllStringSubmatch(string(src), -1) {
		declared[m[2]] = true
	}
	if len(declared) < 12 {
		t.Fatalf("源码里只解出 %d 个方法常量，八成是那边的写法变了，这个检查要跟着改", len(declared))
	}
	for _, m := range AllMethods {
		if !declared[m] {
			t.Fatalf("清单里的 %q 在源码里没有对应的 Method* 常量：那是把名字写了两遍", m)
		}
		delete(declared, m)
	}
	for extra := range declared {
		t.Fatalf("源码里有方法常量 %q 却没进 AllMethods：服务启动那步查不到它，界面调它会报 GAC-IPC-011", extra)
	}
}

// 事件那一对同理：源码里的 Event* 常量必须与 AllEvents 是同一批。
func TestEventListMatchesTheConstantsInSource(t *testing.T) {
	src, err := os.ReadFile("contract.go")
	if err != nil {
		t.Fatalf("读不到 contract.go: %v", err)
	}
	declared := map[string]bool{}
	for _, m := range eventRe.FindAllStringSubmatch(string(src), -1) {
		declared[m[2]] = true
	}
	if len(AllEvents) != 5 || len(declared) != 5 {
		t.Fatalf("事件一侧 %d 条、另一侧 %d 条，两边都该是 5 条", len(AllEvents), len(declared))
	}
	for _, e := range AllEvents {
		if !declared[e] {
			t.Fatalf("AllEvents 里的 %q 源码里没有", e)
		}
		delete(declared, e)
	}
	for extra := range declared {
		t.Fatalf("源码里有事件常量 %q 却没进 AllEvents", extra)
	}
}

// 五条事件里，每一条要么真有人往界面推，要么明写在待接名单里说清"谁接手它才有东西可推"。
//
// 干什么用：契约里列了名字，界面就敢订这个名字（frontend/main.js 五条全订）。
//
// 什么情况下会错、出错时人看得见什么：
//  1. 名字有、没人发——界面上那一格永远不动，且不报错。最坏的一种，因为看起来像"功能在，只是这次没数据"；
//  2. 反过来，事件真发出来了却还挂在待接名单上——那份登记就成了假账，下一个人照着它以为"这条还没接"，
//     于是接到的那一半代码没人去核对（本轮修的正是这一类账：设置那两条方法名字都注册了，却一个调用点都没有）。
//
// 怎么回来：红了只有两条路——要么给这条事件补一个真生产者（得先有那件事可报），要么把它登记进 EventsPending
// 并写明"哪一轮上线之后才有东西可推"。拿 0%→100% 两个数补一条假进度不算补生产者，那是本轮刚删掉的那种谎。
//
// 取据方式：认的是源码里 `contract.<常量名>` 这个写法，不是事件字符串本身——
// 用字符串去 Contains 会把注释、把界面文案里的 `queue:progress` 也算成生产者，那条闸就形同虚设。
func TestEveryEventHasProducerOrIsPending(t *testing.T) {
	src, err := os.ReadFile("contract.go")
	if err != nil {
		t.Fatalf("读不到 contract.go，这条检查就没法做：%v", err)
	}
	idents := map[string]string{} // 事件名 → 源码里那个常量标识符
	for _, m := range eventRe.FindAllStringSubmatch(string(src), -1) {
		idents[m[2]] = m[1]
	}

	// 整个模块里（除本包、除测试）扫一遍谁引用了这些常量。
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("找不到模块根目录：%v", err)
	}
	refs := map[string][]string{}
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != root && (name == ".git" || name == "node_modules" || name == "dist" || name == "build") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		if filepath.Dir(p) == filepath.Clean(root+"/internal/contract") {
			return nil // 这一包是声明处，不算生产者
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for name, id := range idents {
			if strings.Contains(string(body), "contract."+id) {
				refs[name] = append(refs[name], strings.TrimPrefix(p, root+"/"))
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("扫源码失败：%v", walkErr)
	}
	if len(idents) != len(AllEvents) {
		t.Fatalf("源码里解出 %d 个事件常量，清单却有 %d 条", len(idents), len(AllEvents))
	}

	for _, name := range AllEvents {
		producer := refs[name]
		why, pending := EventsPending[name]
		switch {
		case len(producer) > 0 && pending:
			t.Fatalf("事件 %q 已经有人发（%s），却还挂在 EventsPending 上：那份登记是假账",
				name, strings.Join(producer, ", "))
		case len(producer) == 0 && !pending:
			t.Fatalf("事件 %q 契约里发了名字、界面也订了，可全模块没有一个生产者，也没登记在 EventsPending：界面上这一格永远不动且不报错", name)
		case len(producer) == 0:
			if strings.TrimSpace(why) == "" {
				t.Fatalf("事件 %q 登记在待接名单上却没写为什么没接：那等于没登记", name)
			}
		}
	}

	// 待接名单里也不许留着契约之外的名字：那条事件哪天从契约里撤了，登记要跟着撤，不然是一份没人认领的旧账。
	known := map[string]bool{}
	for _, name := range AllEvents {
		known[name] = true
	}
	for name := range EventsPending {
		if !known[name] {
			t.Fatalf("EventsPending 里有 %q，可契约的事件清单里没有它", name)
		}
	}
}

func moduleRoot() (string, error) {
	p, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
			return p, nil
		}
		up := filepath.Dir(p)
		if up == p {
			return "", errors.New("上层目录里没有 go.mod")
		}
		p = up
	}
}

// 状态词表不许跟任务种类混：这一条只管"Go 侧自己别写出两个同名的值"，
// 跨语言那一份（core/meta.js 的 ItemMeta／TaskMeta、core/task/model.js 的 STATUS／KINDS／PHASES）
// 由 frontend/tests/contract.test.mjs 现读这两个文件比对。
// 为什么两边都要查：状态串是界面拿去查文案的 key，两边漂一个字母都不会报错，只会那一档永远显示不出来。
func TestStatusVocabularyHasNoStraySpelling(t *testing.T) {
	groups := map[string][]string{
		"条目状态": {ItemUnset, ItemDetecting, ItemDetected, ItemApplying, ItemApplied, ItemStale, ItemError, ItemRolledback},
		"任务状态": {TaskWaiting, TaskRunning, TaskDone, TaskFailed, TaskSkipped, TaskRolledback, TaskCancelled, TaskRestoring, TaskRestored},
		"任务种类": {KindDetect, KindApply, KindRestore},
		"来源身份": {SourcePrimary, SourceFallback},
		"写入阶段": {PhaseBackup, PhaseWrite, PhaseFlush, PhaseVerify},
	}
	counts := map[string]int{"条目状态": 8, "任务状态": 9, "任务种类": 3, "来源身份": 2, "写入阶段": 4}
	for name, list := range groups {
		if len(list) != counts[name] {
			t.Fatalf("%s 的词数不是 %d，got=%d", name, counts[name], len(list))
		}
		seen := map[string]bool{}
		for _, v := range list {
			if v == "" {
				t.Fatalf("%s 里有空串", name)
			}
			if seen[v] {
				t.Fatalf("%s 里 %q 重复", name, v)
			}
			seen[v] = true
		}
	}
}

// 字段名逐字对上这条硬性要求，最容易在 `appliedIp` 这类拼写上翻车，所以按序列化结果验。
func TestFieldNamesAreTheFrozenSpelling(t *testing.T) {
	for _, c := range []struct {
		name string
		zero any
	}{
		{"Item", Item{}},
		{"State", State{}},
		{"Snapshot", Snapshot{}},
		{"Task", Task{}},
		{"StatusReply", StatusReply{}},
		{"Settings", Settings{}},
		{"TaskIDReply", TaskIDReply{}},
		{"CancelReply", CancelReply{}},
		{"QueueReply", QueueReply{}},
		{"VersionReply", VersionReply{}},
		{"PermReply", PermReply{}},
	} {
		raw, err := json.Marshal(c.zero)
		if err != nil {
			t.Fatalf("%s 序列化失败: %v", c.name, err)
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("%s 解不开: %v", c.name, err)
		}
		for key := range obj {
			if key == "appliedIP" || key == "prevAppliedIP" {
				t.Fatalf("%s 里出现大写 IP 的错拼写：%s", c.name, key)
			}
		}
	}
	b, err := json.Marshal(Item{AppliedIp: "1.2.3.4", PrevAppliedIp: "5.6.7.8", BestIp: "9.9.9.9", ErrorCode: "GAC-HOSTS-003"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"","name":"","appliedIp":"1.2.3.4","prevAppliedIp":"5.6.7.8","status":"","managed":false,"bestIp":"9.9.9.9","errorCode":"GAC-HOSTS-003"}`
	if got := string(b); got != want {
		t.Fatalf("Item 的字段名或顺序漂了：\ngot  %s\nwant %s", got, want)
	}
	// managed 不许带 omitempty：界面那颗 ↺ 要分清"不是我们写的"和"字段没送来"，
	// 省掉这一个键就变成后一种猜，症状是这一条永远点不动还原且没有任何地方报错。
	b2, err := json.Marshal(Item{Managed: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b2), `"managed":true`) {
		t.Fatalf("托管标记没随快照送出去：\ngot  %s", b2)
	}
}
