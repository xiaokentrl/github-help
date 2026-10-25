package resolve

import (
	"sort"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// RankInput 是选优的输入：参选名单（已被 service 按 Kinds 过滤过）+ 每个方式手上的样本。
type RankInput struct {
	Candidates []Spec
	// Samples 按方式 id 分好组，每组按新旧排（最后一条最近）。
	// 里面出现了名单外的 id 会被直接忽略——那是 service 喂错了，不是本层该管的第二份清单。
	Samples map[string][]Sample
}

// Choice 是选优的结果：主用与备用，外加"按同一串决胜排好的完整名单"。
type Choice struct {
	Primary Spec
	// Backup 为零值（ID 为空）表示没有备用可顶——只有一个方式参选时就是这样，界面要如实说"没有备用来源"。
	Backup Spec
	// Ladder 是这条域名一轮里可以依次试过去的顺序：主用在前，往后就是决胜链的下一名。
	// 为什么要把整串交出去而不是只给前两名（2026-10-22 本机实测加的）：冷启动那一次第一名是本地缓存，
	// 缓存里一个地址也没有，它当场空手；第二名社区源给的那一支又探不上——两名用完，
	// 而这台机器上真正通得过的官方段那一名连一次机会都没拿到，条目就被判成"换 IP 无效"。
	// 顺序仍然只在 Rank 这一处定，别处（含 service）不许再排第二遍。
	// 只有一两名参选时它就等于那两名，多出来的那几档平时走不到（前面一旦成功就收），不额外花钱。
	Ladder []Spec
	// Ranked 为真表示"主用是按历史样本选的"，为假表示"还没有样本，按 DefaultPriority 给的"。
	// 这一句要进事件快照：界面必须分得清"历史上这么定的"和"默认猜的"（第 6.6 节）。
	Ranked bool
}

// Rank 从"各方式的样本统计"定出主用与备用。它是纯函数：不读时间、不读随机数、不碰磁盘，
// 所以"同样的样本永远选出同样的结果"能在用例里逐例钉死（G10）。
//
// 干什么用：把"自动选第一稳定、第二速度"这两句话落成一段能跑的代码——它是这件事唯一的家。
// 什么情况下会错、出错时人看得见什么：
// ① 拿 Took 当稳定性（两把尺子合成一把）→ 每轮主用换人，界面"IP 来源"天天变，正是原型 :1281 那个老毛病；
// ② 同分不决胜 → 排名取决于切片初始顺序，等于把"自动"做成"随机"；
// ③ 参选名单空 → 交回错误码 GAC-DETECT-001，界面显示"候选为空"，不许转圈。
// 怎么回来：判据是固定的三条——主用比 S（高者先），S 平比 V（小者先），还平比 id 字典序；
//
//	备用比 V（小者先），V 平比 id 字典序；决胜链只在这一个文件里。
func Rank(in RankInput) (Choice, error) {
	if len(in.Candidates) == 0 {
		return Choice{}, errs.New(errs.CodeDetectEmpty, nil, "没有可参选的解析方式，这条域名没法检测")
	}

	var rankable, fixed []Spec
	for _, s := range in.Candidates {
		if s.Rankable {
			rankable = append(rankable, s)
		} else {
			fixed = append(fixed, s)
		}
	}

	// 一种都不参选（只有固定屏蔽那种钉死地址的方式）：按默认顺序给，且不设备用——
	// 备用换人等于让屏蔽地址被顶掉，正是 6.4 第 2 条要挡的事。
	if len(rankable) == 0 {
		ordered := byDefault(fixed)
		return Choice{Primary: ordered[0], Ladder: ordered}, nil
	}

	scored, unsampled := splitSampled(rankable, in.Samples)
	if len(scored) == 0 {
		ordered := byDefault(rankable)
		c := Choice{Primary: ordered[0], Ladder: ordered}
		if len(ordered) > 1 {
			c.Backup = ordered[1]
		}
		return c, nil
	}

	top := byScore(scored)
	// 还没样本的那批排在有样本的那批后面，内部再按默认顺序：
	// 注释里那句"备用从还没样本的那批按默认顺序给"要到这一行才真是这么做（原先取的是名单给的第一档）。
	tail := byDefault(specsOf(unsampled))
	c := Choice{Primary: top[0].Spec, Ranked: true, Ladder: append(specsOf(top), tail...)}
	switch {
	case len(top) > 1:
		c.Backup = top[1].Spec
	case len(tail) > 0:
		c.Backup = tail[0]
	}
	return c, nil
}

// specsOf 把一组 scored 里的 Spec 抽出来，好让 Ladder 只有一条链。
func specsOf(in []scored) []Spec {
	out := make([]Spec, 0, len(in))
	for _, s := range in {
		out = append(out, s.Spec)
	}
	return out
}

// scored 是一个方式算好的两把尺子。S 为 -1 表示还没样本，不进这一组。
type scored struct {
	Spec Spec
	S    float64
	V    time.Duration
}

// byScore 排"有样本的那批"：主用看 S（高者先）；同分先比 V（小者先），还平比 id 字典序。
// 这条决胜链是"排名不再乱跳"唯一能被机器验的形状，所以顺序一次都不许交换。
func byScore(in []scored) []scored {
	out := append([]scored(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].S != out[j].S {
			return out[i].S > out[j].S
		}
		if out[i].V != out[j].V {
			return out[i].V < out[j].V
		}
		return out[i].Spec.ID < out[j].Spec.ID
	})
	return out
}

// byDefault 排"没有样本可比的场合"：先按 DefaultPriority（小者先），再按 id 字典序。
// 冷启动那一次全靠它（第 6.6 节），所以它也必须确定性——不许写成"按注册顺序"。
func byDefault(in []Spec) []Spec {
	out := append([]Spec(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DefaultPriority != out[j].DefaultPriority {
			return out[i].DefaultPriority < out[j].DefaultPriority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// splitSampled 把参选名单分成"有样本"和"没样本"两组，并把两把尺子算出来。
// 窗口口径只在 health 那一处（SampleWindow），这里只吃算好的数。
func splitSampled(candidates []Spec, samples map[string][]Sample) (has, missing []scored) {
	for _, s := range candidates {
		list := samples[s.ID]
		st := Stability(list)
		if st < 0 {
			missing = append(missing, scored{Spec: s})
			continue
		}
		has = append(has, scored{Spec: s, S: st, V: MedianTook(list)})
	}
	return has, missing
}
