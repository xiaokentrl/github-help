package resolve

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 注册表这一档不查某一个方式，查的是"人不写清单"这句话还成不成立。

func TestRegisterRejectsBadSpecs(t *testing.T) {
	before := len(All())
	cases := []struct {
		desc  string
		spec  Spec
		build func() Resolver
		want  string
	}{
		{"没填 id", Spec{Title: "X", Kinds: []string{"github"}}, newCache, "没填 id"},
		{"没填显示名", Spec{ID: "tempx", Kinds: []string{"github"}}, newCache, "没填界面显示名"},
		// Kinds 为空是最要紧的一条：放它进来＝这方式静默参选所有菜单块，一轮取数直接翻倍。
		{"没声明适用范围", Spec{ID: "tempx", Title: "X"}, newCache, "没声明适用范围"},
		{"没给构造函数", Spec{ID: "tempx", Title: "X", Kinds: []string{"github"}}, nil, "没给构造函数"},
	}
	for _, c := range cases {
		err := Register(c.spec, c.build)
		if err == nil {
			t.Errorf("%s：注册居然成功了", c.desc)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s：错误消息里没这句 %q，实际是 %v", c.desc, c.want, err)
		}
	}
	if got := len(All()); got != before {
		t.Errorf("被拒的注册不该进表：got %d, want %d", got, before)
	}
}

func TestRegisterRejectsDuplicateID(t *testing.T) {
	existing := All()[0]
	before := len(All())

	err := Register(existing, newCache)
	if err == nil {
		t.Fatal("同一个 id 注册两次应该抛")
	}
	// 为什么必须抛而不是"后者盖前者"：盖了以后界面少一种来源、日志一个字没有，半年后没人查得出来。
	if !strings.Contains(err.Error(), "撞号") {
		t.Errorf("错误消息要说出是撞号，实际是 %v", err)
	}
	// 消息要指名"是谁先登记的、是谁又来登记的"：靠 runtime.Caller 记下的文件名，
	// 这就是第 5.4 节那句"带两个文件里的包级常量名"的落点——只说"撞号"的话，还得人去猜哪一档。
	msg := err.Error()
	for _, want := range []string{existing.ID + ".go", "registry_test.go"} {
		if !strings.Contains(msg, want) {
			t.Errorf("错误消息该带上 %s，实际是 %v", want, err)
		}
	}
	if got := len(All()); got != before {
		t.Errorf("撞号那次不该改变注册表：got %d, want %d", got, before)
	}
}

func TestAllOrderIsStableAndNotRegistrationOrder(t *testing.T) {
	first, second := All(), All()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("All() 两轮不一样：第一轮 %v，第二轮 %v", idsOf(first), idsOf(second))
	}

	// 顺序的定义必须是（DefaultPriority, id）——它决定同分时谁顶在前面，不能是"谁先 init"。
	want := append([]Spec(nil), first...)
	sort.SliceStable(want, func(i, j int) bool {
		if want[i].DefaultPriority != want[j].DefaultPriority {
			return want[i].DefaultPriority < want[j].DefaultPriority
		}
		return want[i].ID < want[j].ID
	})
	if !reflect.DeepEqual(first, want) {
		t.Errorf("All() 的顺序不是（DefaultPriority, id）：%v", idsOf(first))
	}
}

func TestAllReturnsCopies(t *testing.T) {
	before := All()
	if len(before) == 0 {
		t.Fatal("注册表是空的")
	}
	list := All()
	list[0].Title = "改了也不该看见"
	list[0].Kinds = append(list[0].Kinds, "编进去的")

	after := All()
	if after[0].Title != before[0].Title {
		t.Errorf("调用方改得动显示名：%q", after[0].Title)
	}
	if len(after[0].Kinds) != len(before[0].Kinds) {
		t.Errorf("调用方改得动适用范围：%v（原本 %v）", after[0].Kinds, before[0].Kinds)
	}
}

func TestGetUnknownIDIsNotSilentlyEmpty(t *testing.T) {
	// 第 6.1 节第 7 步靠 Get 取备用；ok=false 必须让调用方看得见，静默给零值＝界面显示"已用备用"而它根本没跑。
	if r, ok := Get("没有这一档"); ok || r != nil {
		t.Errorf("取不存在的 id 该回 (nil, false)，实际 %v, %v", r, ok)
	}
}

func TestEveryModeIsUsableAndSelfConsistent(t *testing.T) {
	seen := map[int][]string{}
	for _, s := range All() {
		r, ok := Get(s.ID)
		if !ok {
			t.Errorf("%s 取不到实现", s.ID)
			continue
		}
		if got := r.Spec(); !reflect.DeepEqual(got, s) {
			t.Errorf("%s：注册表里的自我介绍与实现自己报的不一样：\n注册 %+v\n实现 %+v", s.ID, s, got)
		}
		if len(s.Kinds) == 0 {
			t.Errorf("%s 没声明适用范围", s.ID)
		}
		if s.Rankable {
			seen[s.DefaultPriority] = append(seen[s.DefaultPriority], s.ID)
		}
	}
}

func idsOf(list []Spec) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.ID
	}
	return out
}

// 冷启动那一次完全靠 DefaultPriority 定主用/备用（第 6.6 节），所以参选方式之间不许共用同一个数——
// 共用了就轮到切片顺序决胜，等于把"自动"做成"随机"。
func TestDefaultPrioritiesAreUniqueAmongRankable(t *testing.T) {
	groups := map[int][]string{}
	for _, s := range All() {
		if !s.Rankable {
			continue
		}
		groups[s.DefaultPriority] = append(groups[s.DefaultPriority], s.ID)
	}
	var shared []string
	for p, ids := range groups {
		if len(ids) > 1 {
			sort.Strings(ids)
			shared = append(shared, strconv.Itoa(p)+"："+strings.Join(ids, "、"))
		}
	}
	sort.Strings(shared)
	if len(shared) > 0 {
		t.Errorf("有参选方式共用 DefaultPriority（%s）：冷启动那次会靠切片顺序决胜", strings.Join(shared, "；"))
	}
}
