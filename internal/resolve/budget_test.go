package resolve

import (
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// 预算这一处是"别把用户网络跑死"唯一的家。用例查两件事：
// ① 上限之间的关系不许自相矛盾（探测比取数还长、窗口比并发还小，都是写出来看着对、跑起来出问题的形状）；
// ② 超了要当场报，报的是"本轮只算了前 N 条"这句实话，不是假进度。

func TestCheckPassesWithinBudget(t *testing.T) {
	if err := Check(RoundPlan{}); err != nil {
		t.Errorf("一次都没发不该报：%v", err)
	}
	if err := Check(RoundPlan{Fetches: FetchesPerRound, Probes: ProbesPerRound}); err != nil {
		t.Errorf("刚好用满不算超：%v", err)
	}
}

func TestCheckReportsOverBudget(t *testing.T) {
	cases := []struct {
		desc string
		in   RoundPlan
	}{
		{"取数超一次", RoundPlan{Fetches: FetchesPerRound + 1}},
		{"探测超一次", RoundPlan{Probes: ProbesPerRound + 1}},
		{"两个都超", RoundPlan{Fetches: FetchesPerRound + 1, Probes: ProbesPerRound + 1}},
	}
	for _, c := range cases {
		err := Check(c.in)
		if err == nil {
			t.Errorf("%s：该报，实际没报", c.desc)
			continue
		}
		if code := errs.CodeOf(err); code != errs.CodeDetectBudget {
			t.Errorf("%s：该回 GAC-DETECT-005，实际 %s", c.desc, code)
		}
		// 界面那句"剩下的排队"要用这两个数，参数不带就等于让界面显示一句空话。
		p := errs.ParamsOf(err)
		if p["fetchLimit"] != FetchesPerRound || p["probeLimit"] != ProbesPerRound {
			t.Errorf("%s：参数该带上两个上限，实际 %v", c.desc, p)
		}
	}
}

// 剩多少不能用负数表示：调用方拿负数去算"还能发几支"会算出一个更大的数。
func TestRoomNeverNegative(t *testing.T) {
	cases := []struct {
		in              RoundPlan
		wantFetch, want int
	}{
		{RoundPlan{}, FetchesPerRound, ProbesPerRound},
		{RoundPlan{Fetches: 4, Probes: 6}, FetchesPerRound - 4, ProbesPerRound - 6},
		{RoundPlan{Fetches: FetchesPerRound, Probes: ProbesPerRound}, 0, 0},
		{RoundPlan{Fetches: FetchesPerRound * 3, Probes: ProbesPerRound * 7}, 0, 0},
	}
	for _, c := range cases {
		if got := c.in.FetchRoom(); got != c.wantFetch {
			t.Errorf("%+v 的取数余量 got %d, want %d", c.in, got, c.wantFetch)
		}
		if got := c.in.ProbeRoom(); got != c.want {
			t.Errorf("%+v 的探测余量 got %d, want %d", c.in, got, c.want)
		}
	}
}

// 上限之间的关系：这几条不是挑出来的偏好，写反了这一轮必然出怪事，所以当场钉住。
func TestBudgetNumbersAreCoherent(t *testing.T) {
	if ProbeTimeout >= ModeTimeout {
		t.Errorf("单次探测 %v 不比一次取数 %v 短：探测会把整轮的帽子先顶穿", ProbeTimeout, ModeTimeout)
	}
	if Concurrency < 1 {
		t.Errorf("并发 %d 至少得是 1", Concurrency)
	}
	if MaxCandidatesPerMode < Concurrency {
		t.Errorf("每方式候选 %d 比并发 %d 还少：一批探测都填不满，白留一堆空闲线程", MaxCandidatesPerMode, Concurrency)
	}
	if CandidatesPerPrefix < 1 || CandidatesPerPrefix > MaxCandidatesPerMode {
		t.Errorf("每段取 %d 个候选，落在 1…%d 之外", CandidatesPerPrefix, MaxCandidatesPerMode)
	}
	if RTTRepeats < 2 {
		t.Errorf("指标二重复 %d 次：少于两次就谈不上取低分位，单次抖动直接进名次", RTTRepeats)
	}
	// 一个方式的候选池不许吃光全轮配额（同一把单位算：一条地址占 RTTRepeats 次）。
	// 原来这里写的判据是"一条地址重复几次的总时长要短于一次取数"——那是探测与取数两个阶段各自的帽子，
	// 互相约束不了，写成判据只会让人以为预算之间有这么回事。
	if perMode := MaxCandidatesPerMode * RTTRepeats; perMode > ProbesPerRound {
		t.Errorf("一个方式一轮就要 %d 次探测，全轮上限才 %d 次：来两档方式就吃光配额，剩下的全都排队",
			perMode, ProbesPerRound)
	}
	// 段清单的复用必须比"一轮"长，否则一轮里每档方式都去问一次 meta，60 次/小时的配额几下就撞墙。
	if MetaCacheTTL <= ModeTimeout {
		t.Errorf("meta 缓存只活 %v，比一次取数还短，等于没缓存", MetaCacheTTL)
	}
	if FetchBodyBytes < 1024 {
		t.Errorf("一次取数只让读 %d 字节，连正常源（实测 2498 字节）都装不下", FetchBodyBytes)
	}

	// 一轮探测的最坏墙钟要跟实测那一轮同一量级：2026-10-22 在本机把上限放到极大，一次「全部检测」
	// 花 591 次探测、277 秒跑完，界面每一行都在往前走。这条查的不是"多少秒算卡死"，
	// 而是"别把次数调到连一轮整清单都跑不完，或者要跑到十几分钟"——两头都要看得见。
	worst := time.Duration(ProbesPerRound/Concurrency+1) * ProbeTimeout
	if worst > 6*time.Minute {
		t.Errorf("按现在的预算，一轮探测最坏要 %v，比实测那一轮（277 秒）大出一个量级，用户会以为工具卡死了", worst)
	}

	// 上限必须装得下一整轮「全部检测」，否则一次点击只测完三分之一，界面上剩下的全得等下一次。
	// 这两个数是同一次实测的读数（54 条／69 个域名槽）：取数 160 次、探测 591 次。
	// 谁把它们调小，这条就红——红了要先重量一次整清单，再决定是不是真的要截。
	if FetchesPerRound < 160 {
		t.Errorf("取数上限 %d 装不下一轮实测的 160 次：一次「全部检测」会被截，剩下的只能等下一轮", FetchesPerRound)
	}
	if ProbesPerRound < 591 {
		t.Errorf("探测上限 %d 装不下一轮实测的 591 次：同上", ProbesPerRound)
	}
}
