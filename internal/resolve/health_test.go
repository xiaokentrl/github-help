package resolve

import (
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/menu"
)

// 两把尺子各自怎么算，只住在这一个文件；窗口大小只有一个家（budget.go 的 SampleWindow）。
// 这一支用例查的就是"窗口口径有没有两处各数一遍"。

func TestStabilityIsRatioNotLastTry(t *testing.T) {
	cases := []struct {
		desc string
		in   []Sample
		want float64
	}{
		{"全成功", []Sample{one(true, time.Second)}, 1},
		{"全失败", []Sample{one(false, time.Second), one(false, time.Second)}, 0},
		{"三次里两次成功", []Sample{one(true, time.Second), one(false, time.Second), one(true, time.Second)}, 2.0 / 3.0},
		// 这条是"为什么用比例不用最近一次"的用头：最后一次失败，S 仍是一大半，主用不换人。
		{"最后一次刚失败", []Sample{one(true, time.Second), one(true, time.Second), one(true, time.Second), one(false, time.Second)}, 0.75},
	}
	for _, c := range cases {
		if got := Stability(c.in); got != c.want {
			t.Errorf("%s：got %v, want %v", c.desc, got, c.want)
		}
	}
}

// 没有样本必须回"还不知道"（-1），不许回 0——0 会被 Rank 当成"全失败"参与比较，
// 那样一个从没跑过的方式就被排到最后，冷启动那一次等于按失败排名。
func TestStabilityWithoutSamplesIsUnknown(t *testing.T) {
	if got := Stability(nil); got != -1 {
		t.Errorf("空样本该回 -1，实际 %v", got)
	}
	if got := Stability([]Sample{}); got != -1 {
		t.Errorf("空切片该回 -1，实际 %v", got)
	}
}

// 窗口只数最近 N 条（样本按新旧排，最后一条最近）。
// 这条用例的形状：老的那批全失败、新的全成功 → S 必须是 1；一旦窗口反过来，这里就是 0。
func TestHealthWindowsTakeTheNewest(t *testing.T) {
	list := make([]Sample, 0, SampleWindow+3)
	for i := 0; i < 3; i++ {
		list = append(list, one(false, time.Second)) // 最老的三条：全失败
	}
	for i := 0; i < SampleWindow; i++ {
		list = append(list, one(true, 10*time.Millisecond))
	}
	if got := Stability(list); got != 1 {
		t.Errorf("窗口该只吃最近 %d 条，实际 S=%v", SampleWindow, got)
	}
	if got := MedianTook(list); got != 10*time.Millisecond {
		t.Errorf("窗口反过来会把老样本的中位数（1 秒）当成这个方式的读数，实际 %v", got)
	}
}

func TestMedianTookIsDeterministic(t *testing.T) {
	cases := []struct {
		desc string
		in   []Sample
		want time.Duration
	}{
		{"一条", []Sample{one(true, 30*time.Millisecond)}, 30 * time.Millisecond},
		{"三条打乱顺序", []Sample{one(true, 90*time.Millisecond), one(true, 30*time.Millisecond), one(true, 60*time.Millisecond)}, 60 * time.Millisecond},
		// 偶数条取"中间偏左"那一个，不取两点平均：平均会算出一个两边都不是的数，跨轮比就没有可比的那一个。
		{"四条取中间偏左", []Sample{one(true, 10*time.Millisecond), one(true, 20*time.Millisecond), one(true, 30*time.Millisecond), one(true, 40*time.Millisecond)}, 20 * time.Millisecond},
		{"一次超时翻天不了", []Sample{one(true, 20*time.Millisecond), one(true, 25*time.Millisecond), one(true, 30*time.Millisecond), one(false, 5*time.Second)}, 25 * time.Millisecond},
	}
	for _, c := range cases {
		if got := MedianTook(c.in); got != c.want {
			t.Errorf("%s：got %v, want %v", c.desc, got, c.want)
		}
	}

	if got := MedianTook(nil); got != 0 {
		t.Errorf("没样本该回 0（没量过），实际 %v", got)
	}
}

// 窗口那个数是两把尺子共同的分母，必须一处定义、且是奇数（中位数不需要取平均）。
func TestSampleWindowIsOneHomeAndOdd(t *testing.T) {
	if SampleWindow%2 == 0 {
		t.Errorf("SampleWindow=%d 是偶数：中位数会落在两个样本之间，取平均就等于造出一个没量过的数", SampleWindow)
	}
	if SampleWindow < 3 {
		t.Errorf("SampleWindow=%d 太小：连续三轮判据（M2 那句「排名不再乱跳」）落在窗口里比不出名次", SampleWindow)
	}
}

// 方式声明的适用范围必须真有对手方——Kinds 里写一个不存在的菜单块，等于这个方式永不参选，
// 而界面看不出任何异常（第 10 节那句"Kinds 漏了"的反面）。
func TestEveryKindNamesARealMenu(t *testing.T) {
	catalog, err := menu.Load()
	if err != nil {
		t.Fatalf("读菜单清单失败：%v", err)
	}
	real := map[string]bool{}
	for _, m := range catalog.Menus() {
		real[m.ID] = true
	}

	var bad []string
	for _, s := range All() {
		for _, k := range s.Kinds {
			if !real[k] {
				bad = append(bad, s.ID+"→"+k)
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("有方式声明了不存在的菜单块：%v（这一档对那块永不参选）", bad)
	}

	// 反过来也要看得见：block 那一块只有 constant 参选，别的联网方式一律不许挤进去。
	// 挤进去的后果写在第 6.4 节第 2 条——某档"成功"给出公网 IP，屏蔽当场失效。
	for _, s := range All() {
		for _, k := range s.Kinds {
			if k == "block" && s.ID != "constant" {
				t.Errorf("%s 把 block 也声明成自己的适用范围：屏蔽会被它顶掉", s.ID)
			}
		}
	}
}
