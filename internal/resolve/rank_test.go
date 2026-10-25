package resolve

import (
	"reflect"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// 这一支文件钉的是 M2 那句验收判据："连续三轮检测，主用/备用不换人"。
// 它钉得住，靠的就是 Rank 是纯函数——同样的样本进得来，必须同样的名次出得来。

// specs 从注册表里取真实自我介绍来构造用例。
// 为什么不许在测试里现编 Spec：那样 Rank 测的是我编的形状，不是六档方式实际声明的形状——
// 某一档把 Rankable 改成假，用例还是绿的。
func specs(t *testing.T, ids ...string) []Spec {
	t.Helper()
	byID := map[string]Spec{}
	for _, s := range All() {
		byID[s.ID] = s
	}
	var out []Spec
	for _, id := range ids {
		s, found := byID[id]
		if !found {
			t.Fatalf("用例里的方式 id %q 不在注册表里", id)
		}
		out = append(out, s)
	}
	return out
}

// one 是一条样本：探过了、能不能服务、取数花了多久。
func one(served bool, took time.Duration) Sample {
	return Sample{CanServe: served, Took: took}
}

// 双轮等值（G10 的正面）：同一批样本跑两轮，结果必须逐字相同。
func TestRankSameInputSameChoice(t *testing.T) {
	in := RankInput{
		Candidates: specs(t, "github520", "segment", "system", "doh"),
		Samples: map[string][]Sample{
			"github520": {one(true, 40*time.Millisecond), one(true, 60*time.Millisecond), one(true, 50*time.Millisecond)},
			"segment":   {one(true, 5*time.Millisecond), one(false, 9*time.Millisecond), one(true, 7*time.Millisecond)},
			"system":    {one(true, 2*time.Millisecond)},
			"doh":       nil,
		},
	}
	first, err := Rank(in)
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	second, err := Rank(in)
	if err != nil {
		t.Fatalf("第二轮 Rank 抛了：%v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("同一批样本选出两个结果：%+v / %+v", first, second)
	}
	if !first.Ranked {
		t.Error("有样本的一轮该把 ranked 报成真——界面得说得清这是历史选的还是默认猜的")
	}
	// 输入切片不许被改：service 拿同一份样本还要跑别的判断。
	if len(in.Candidates) != 4 || in.Candidates[0].ID != "github520" {
		t.Errorf("Rank 改了调用方的入参：%v", idsOf(in.Candidates))
	}
}

// 主用看稳定性 S，不看速度：这正是"第一稳定、第二快"那句原话里"第一"的形状。
func TestRankPrimaryUsesStabilityNotSpeed(t *testing.T) {
	in := RankInput{
		Candidates: specs(t, "cache", "system"),
		Samples: map[string][]Sample{
			"cache":  {one(true, 900*time.Millisecond), one(true, 950*time.Millisecond), one(true, 920*time.Millisecond)},
			"system": {one(true, 5*time.Millisecond), one(false, 6*time.Millisecond), one(true, 4*time.Millisecond)},
		},
	}
	got, err := Rank(in)
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	if got.Primary.ID != "cache" {
		t.Errorf("主用该是每次都成功的 cache，实际给了 %s", got.Primary.ID)
	}
	if got.Backup.ID != "system" {
		t.Errorf("备用该是剩下的那一档 system，实际 %s", got.Backup.ID)
	}
}

// 备用看速度 V；主用同分时也由 V 决胜，V 还平就比 id 字典序。
func TestRankTieBreaksByValueThenID(t *testing.T) {
	t.Run("S 相同比 V", func(t *testing.T) {
		in := RankInput{
			Candidates: specs(t, "github520", "segment"),
			Samples: map[string][]Sample{
				"github520": {one(true, 100*time.Millisecond), one(true, 100*time.Millisecond)},
				"segment":   {one(true, 20*time.Millisecond), one(true, 20*time.Millisecond)},
			},
		}
		got, err := Rank(in)
		if err != nil {
			t.Fatalf("Rank 抛了：%v", err)
		}
		if got.Primary.ID != "segment" {
			t.Errorf("同分时该让快的当主用，实际 %s", got.Primary.ID)
		}
	})

	t.Run("S 与 V 都相同比 id", func(t *testing.T) {
		in := RankInput{
			Candidates: specs(t, "system", "segment"),
			Samples: map[string][]Sample{
				"system":  {one(true, 100*time.Millisecond), one(true, 100*time.Millisecond)},
				"segment": {one(true, 100*time.Millisecond), one(true, 100*time.Millisecond)},
			},
		}
		got, err := Rank(in)
		if err != nil {
			t.Fatalf("Rank 抛了：%v", err)
		}
		// segment < system 字典序。这一条就是"决胜不许省"的用头——把它去掉，这里就换了人。
		if got.Primary.ID != "segment" || got.Backup.ID != "system" {
			t.Errorf("S 与 V 都相同时该按 id 字典序：%s / %s", got.Primary.ID, got.Backup.ID)
		}
	})
}

// 中位数抗抖动：一次超时不该把这个方式整档顶下去。
func TestRankMedianSurvivesOneSpike(t *testing.T) {
	in := RankInput{
		Candidates: specs(t, "github520", "segment"),
		Samples: map[string][]Sample{
			"github520": {one(true, 30*time.Millisecond), one(true, 4*time.Second), one(true, 35*time.Millisecond)},
			"segment":   {one(true, 60*time.Millisecond), one(true, 55*time.Millisecond), one(true, 65*time.Millisecond)},
		},
	}
	got, err := Rank(in)
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	if got.Primary.ID != "github520" {
		t.Errorf("一次抖动不该翻天，主用该还是中位更小的 github520，实际 %s", got.Primary.ID)
	}
}

// 冷启动没有任何样本：按 DefaultPriority 定，并把 ranked 报成假。
func TestRankWithoutSamplesFallsBackToDefault(t *testing.T) {
	in := RankInput{Candidates: specs(t, "github520", "segment", "system", "doh")}
	got, err := Rank(in)
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	if got.Ranked {
		t.Error("没有样本时 ranked 必须是假——界面那句「本轮按默认顺序（还没有历史样本）」靠它")
	}
	if got.Primary.ID != "github520" || got.Backup.ID != "segment" {
		t.Errorf("默认顺序该按 DefaultPriority 排，实际 %s / %s", got.Primary.ID, got.Backup.ID)
	}
}

// 只有一档有样本时，备用从还没样本的那批里按默认顺序给，不许空着。
func TestRankBackupComesFromUnsampledWhenOnlyOneScored(t *testing.T) {
	in := RankInput{
		Candidates: specs(t, "github520", "segment"),
		Samples:    map[string][]Sample{"github520": {one(true, 30*time.Millisecond)}},
	}
	got, err := Rank(in)
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	if got.Primary.ID != "github520" || !got.Ranked {
		t.Errorf("有样本那档该当主用并标 ranked，实际 %s ranked=%v", got.Primary.ID, got.Ranked)
	}
	if got.Backup.ID != "segment" {
		t.Errorf("备用该补上没样本那档，实际 %q", got.Backup.ID)
	}
}

// 只有一档参选：不许凭空造一个"备用"出来，界面那句"没有备用来源"靠这个零值。
func TestRankSingleCandidateHasNoBackup(t *testing.T) {
	got, err := Rank(RankInput{Candidates: specs(t, "github520")})
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	if got.Backup.ID != "" {
		t.Errorf("只有一档参选时备用必须是零值，实际 %s", got.Backup.ID)
	}
}

// 第 6.4 节第 2 条：固定屏蔽不参与 S/V，也不许被"速度最快"顶掉。
// 样本喂给它也不许产生名次变化——这条用例就是那句"block 类条目永远不产生 S/V 样本"。
func TestRankExcludesConstantFromScoring(t *testing.T) {
	only, err := Rank(RankInput{
		Candidates: specs(t, "constant"),
		Samples:    map[string][]Sample{"constant": {one(true, time.Microsecond)}},
	})
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	if only.Primary.ID != "constant" || only.Backup.ID != "" || only.Ranked {
		t.Errorf("block 那一档只能由 constant 自己当主用，且不设备用、不标 ranked，实际 %+v", only)
	}

	mixed, err := Rank(RankInput{
		Candidates: specs(t, "constant", "github520"),
		Samples: map[string][]Sample{
			"constant":  {one(true, time.Microsecond), one(true, time.Microsecond)},
			"github520": {one(true, time.Second), one(true, time.Second)},
		},
	})
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	if mixed.Primary.ID != "github520" {
		t.Errorf("一起参选时 constant 不许靠「最快」抢主用，实际 %s", mixed.Primary.ID)
	}
}

func TestRankEmptyCandidatesIsHonestError(t *testing.T) {
	got, err := Rank(RankInput{})
	if err == nil {
		t.Fatal("参选名单空必须报错，不能让界面转圈")
	}
	if code := errs.CodeOf(err); code != errs.CodeDetectEmpty {
		t.Errorf("该回 GAC-DETECT-001，实际 %s（%v）", code, err)
	}
	if got.Primary.ID != "" {
		t.Errorf("报错时不该带出一个名次：%+v", got)
	}
}

// 一个方式被叫停不等于它失败：service 见到 GAC-TASK-003 不许往里塞一条失败样本。
// 这条用例住在这儿，是因为"要不要给这个方式记一笔"最终影响的是名次；
// 它钉的是前提本身：多一条失败样本必须让 S 下降——不然上面那句护栏是空的。
func TestFailureSampleDoesDragStability(t *testing.T) {
	list := []Sample{one(true, 10*time.Millisecond), one(true, 12*time.Millisecond)}
	good := Stability(list)
	dragged := append(append([]Sample(nil), list...), one(false, 0))
	if !(Stability(dragged) < good) {
		t.Fatalf("前提没成立：S 从 %v 到 %v，「被叫停不采样」这句话就没人管", good, Stability(dragged))
	}
}

// Ladder 是"这一轮可以依次试到哪一档"的整串名单（2026-10-22 本机实测加的那一条的落点）。
// 它钉三件事：顺序仍然只在 Rank 这一处定；串里的每一档不许重复；前两档必须还是 Primary/Backup。
func TestRankLadderKeepsWholeOrderedChain(t *testing.T) {
	cold, err := Rank(RankInput{Candidates: specs(t, "doh", "segment", "github520", "cache", "system")})
	if err != nil {
		t.Fatal(err)
	}
	// 没有样本时按 DefaultPriority：cache(10) → github520(20) → segment(30) → system(40) → doh(50)。
	want := []string{"cache", "github520", "segment", "system", "doh"}
	if got := idsOf(cold.Ladder); !same(got, want) {
		t.Fatalf("冷启动那一次的名单是 %v，要的是 %v", got, want)
	}
	if cold.Primary.ID != want[0] || cold.Backup.ID != want[1] {
		t.Fatalf("名单前两档与主用/备用不是同一个数：%s / %s", cold.Primary.ID, cold.Backup.ID)
	}

	warm, err := Rank(RankInput{
		Candidates: specs(t, "doh", "segment", "github520", "cache", "system"),
		Samples: map[string][]Sample{
			"segment":   {one(true, 5*time.Millisecond)},
			"system":    {one(true, 2*time.Millisecond)},
			"github520": {one(true, 40*time.Millisecond)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, s := range warm.Ladder {
		if seen[s.ID] {
			t.Fatalf("名单里 %q 出现两次：%v", s.ID, idsOf(warm.Ladder))
		}
		seen[s.ID] = true
	}
	// 有样本的三档在前（S 全为 1，比 V 小的先），没样本的两档按默认顺序接在后面。
	if got, wantRun := idsOf(warm.Ladder), []string{"system", "segment", "github520", "cache", "doh"}; !same(got, wantRun) {
		t.Fatalf("有样本那批排完再接默认那批，要的是 %v，实际 %v", wantRun, got)
	}
	if warm.Primary.ID != "system" || warm.Backup.ID != "segment" {
		t.Fatalf("主用/备用与名单头两档脱节：%s / %s", warm.Primary.ID, warm.Backup.ID)
	}
}

// 只有一档有样本时，备用要从"还没样本的那批"里按默认顺序取——这条原先靠名单给的第一档，
// 现在必须由 Ladder 的第二档来说同一句话（两处一个数，才算"顺序只有一个家"）。
func TestRankBackupEqualsLadderSecond(t *testing.T) {
	in := RankInput{
		Candidates: specs(t, "doh", "cache", "segment"),
		Samples:    map[string][]Sample{"segment": {one(true, 3*time.Millisecond)}},
	}
	got, err := Rank(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Ladder) < 2 {
		t.Fatalf("名单只有 %d 档，这句没法比：%v", len(got.Ladder), idsOf(got.Ladder))
	}
	if got.Backup.ID != got.Ladder[1].ID {
		t.Fatalf("备用是 %q 而名单第二档是 %q：同一件事两处各说一套", got.Backup.ID, got.Ladder[1].ID)
	}
	// cache(10) 比 doh(50) 靠前，备用必须是它——按注册顺序取就正好取错。
	if got.Backup.ID != "cache" {
		t.Fatalf("备用是 %q，要的是默认顺序第一档 cache", got.Backup.ID)
	}
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
