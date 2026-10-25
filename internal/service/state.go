package service

import (
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/menu"
	"github.com/hlep/github-accel/internal/resolve"
)

// task 是队列里的一行。一行只干一件事：一条域名的检测、应用或恢复。
//
// 为什么要 batch：界面上一次「全部检测」是一个动作，队列里却是 51 行。
// 那个动作要有个能取消、能报总进度（processed／total）的把手，靠的就是这个批次号；
// 而每一行自己的状态又必须单独看得见，所以两者分开存。
type task struct {
	id     string
	batch  string
	kind   string
	itemID string
	name   string
	status string
	phase  string
	// origin 是"谁来发起的"（trigger.go），只登记、不参与任何判定（L1 的解）。
	origin string
	// picks 是界面上指定的地址（契约 ApplyQueue 那条参数）；为空就用自己的最优地址。
	pick string
	// lastErr 是给日志与控制台看的那一句原文。给用户看的那句话由码＋参数在语言文件凑，不在这里拼。
	// errCode／errParams 就是"凑那句话要的两个数"（契约 Task.errorCode／errorParams）：
	// 只留原文的话，队列那一行换了语言还是中文，而同一个失败在条目那一行已经换成本地话了。
	lastErr   string
	errCode   string
	errParams map[string]any
	// startedAt／finishedAt 是这一行的秒表：取跑那一刻按下、收尾那一刻抬起，
	// 队列那张表的「耗时」列读的就是这两个数之差（契约 Task.durationMs）。
	// 为什么要零值判据而不是"跑完就写死一个串"：等待中与已取消的行根本没跑，
	// 那一格要如实是空的，界面拿 0 显示破折号——编一个数出来就是假进度。
	startedAt  time.Time
	finishedAt time.Time
	// round 是同批次共用的那一本预算账（一轮取数到底发了多少事）。指针共享，所以批次里每条都记在同一本上。
	round *roundState
}

// itemState 是一条条目在内存里的那点运行态。
//
// 注意这里刻意**不存**"当前写进 hosts 的地址"：那一列永远现读文件算〔已定：D3 甲〕。
// 存了就等于承认服务有一份和文件平起平坐的真相，用户在外面手工改了 hosts，界面就会跟文件各说一套。
type itemState struct {
	// bestIPs 是最近一轮检测给每个域名选出的最优地址。域名是键，所以一条条目里几个域名各有自己的地址——
	// hosts 一行一域名是硬约束，共用一个地址只会把其中几个域名写坏。
	// 这一轮没检出地址的那个域名根本不在键里：那不是丢了，是"这个域名还没地址"，
	// 写入那一段（changesFor）据此只写有的那几个，并把没写的那个留在状态栏里。
	bestIPs map[string]string
	// source／sourceRole／latency 说的是这条条目**那一列地址**所属域名的那一个数：
	// 契约里 Item 一行只有一个 bestIp／appliedIp（原型 :1895-1896 那两列就是一个数），
	// 而一条条目里几个域名各有自己的地址、也可能来自不同方式。哪一个域名填这一列由 headlineDomain 定，
	// 其余域名各自写了各自的地址。这条形状限制如实记在这儿。
	source     string
	sourceRole string
	latency    int64
	lastCheck  string
	// errCode／errParams 是最近一次失败。成功了就清空，不留上一轮的旧原因。
	errCode   string
	errParams map[string]any
	// rolledback 是"上次写完发现不对、已经还原回去"这件事。它盖过 detected／unset，
	// 因为用户要点开那一行才知道发生了什么；下一次写入或检测成功就清掉。
	rolledback bool
}

// 任务状态的单向流转表（L2 的那张图）。
//
// 干什么用：把"能变成什么"写死在一处，谁也不许在别处顺手改状态。
// 什么情况下会错、出错时人看得见什么：终态被"收尾"那一步改回去（done 又变 running）→
//
//	队列里那一行反复跳，而且界面上没有任何一处说得清为什么。
//
// 怎么回来：唯一一处允许回头的边是 running → waiting，且只有暂停这一条路能走它（queue.go 那三行）；
// 其余任何回头都由这个表当场拒绝。用例 TestTaskStatesAreOneWay 钉住这张表。
var moves = map[string][]string{
	contract.TaskWaiting: {
		contract.TaskRunning,
		contract.TaskCancelled,
		// 服务退场时还排在等待里的行标"已跳过"——那是"没跑"的实话，演成"跑完了"就是假进度。
		// 这一条边只属于退场那一步（take() 里那个 stopping 分支），别处不许拿它当"重新排队"的口子。
		contract.TaskSkipped,
	},
	contract.TaskRunning: {
		contract.TaskWaiting, // 只有暂停走这条
		contract.TaskDone, contract.TaskFailed, contract.TaskRolledback,
		contract.TaskRestored, contract.TaskSkipped,
	},
	contract.TaskDone:       nil,
	contract.TaskFailed:     nil,
	contract.TaskSkipped:    nil,
	contract.TaskRolledback: nil,
	contract.TaskCancelled:  nil,
	contract.TaskRestoring:  {contract.TaskRestored, contract.TaskFailed, contract.TaskRolledback, contract.TaskCancelled},
	contract.TaskRestored:   nil,
}

// canMove 判这一步走不走得通。
func canMove(from, to string) bool {
	if from == to {
		return true
	}
	for _, t := range moves[from] {
		if t == to {
			return true
		}
	}
	return false
}

// moveTo 改状态，走不通就返回 false 并且一个字不改。
//
// 为什么不让它 panic：状态改不动的原因是运行时的（比如刚要跑一条已被取消的任务），
// 那种情况要的是"什么都不做"，不是把服务弄死。调用方必须看返回值决定要不要记一句。
func (t *task) moveTo(to string) bool {
	if !canMove(t.status, to) {
		return false
	}
	t.status = to
	return true
}

// isTerminal 判这一行是不是已经跑完了。终态一共六条，全在这一个函数里数，别处不许各自再判一遍。
func (t *task) isTerminal() bool {
	switch t.status {
	case contract.TaskDone, contract.TaskFailed, contract.TaskSkipped,
		contract.TaskRolledback, contract.TaskCancelled, contract.TaskRestored:
		return true
	}
	return false
}

// isClearable 判这一行能不能被「清空已完成」抹掉——比 isTerminal 少两档：失败与已回滚留下。
//
// 干什么用：界面上「重试失败的那几条」那颗按钮，原料就是队列里那几行失败行；
// 用户点了"清空已完成"之后，屏幕上那些红色的行还在、点一下还能重来。
// 什么情况下会错、出错时人看得见什么：上一版的筛子是 isTerminal()（六档全抹），
// 于是"清空已完成"顺手把重试的原料抹掉了——那颗按钮点了没东西可重试，
// 而那几行失败本来在条目那一档还挂着原因，屏幕上半句说"出错"、下半句说"没有可重试"，是一句自相矛盾的话。
// 为什么"已跳过／已取消／已恢复"能抹：它们说的是"没跑／人不要它跑了／已经还原回去了"，
// 没有任何欠用户的一句话；failed／rolledback 说的是"跑了，但坏了"，那一行本身就是答案。
// 怎么回来：只有这一处判"该不该留"，clearFinished 用它；用例 TestClearFinishedKeepsFailedRows 钉住两档。
func (t *task) isClearable() bool {
	switch t.status {
	case contract.TaskDone, contract.TaskSkipped,
		contract.TaskCancelled, contract.TaskRestored:
		return true
	}
	return false
}

// durationMs 是队列那一行「耗时」列唯一的原料：按下与抬起两个时刻齐了才给数，
// 没跑完（等待中、正在跑）与没按过表（已取消、已跳过）都是 0，界面据此画破折号。
// 为什么要判到这么死：这一列以前只有 mock 在本地掐表，真服务的回执里一个时间也没有；
// 把它留成"界面自己算"就等于那一列永远空着，而空着和"这一行没花时间的跑完了"是两种说法。
func (t *task) durationMs() int64 {
	if !t.isTerminal() || t.startedAt.IsZero() || t.finishedAt.IsZero() {
		return 0
	}
	ms := t.finishedAt.Sub(t.startedAt).Milliseconds()
	if ms < 0 {
		// 时钟往回跳过（NTP 校时那种）：宁可报 0，也不报一个负数让界面显示"-1.2s"。
		return 0
	}
	return ms
}

// recordSample 把一次取数记进这个方式的样本窗口。
//
// 为什么只在这一处记：两把尺子的原料只有一个家，样本散到两处，S 和 V 的分母就不一样了（第 6.2 节那句）。
// 窗口大小用 resolve.SampleWindow，数字不在这里出现第二次。
func (s *Service) recordSample(modeID string, sample resolve.Sample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := append(s.samples[modeID], sample)
	if len(list) > resolve.SampleWindow {
		list = list[len(list)-resolve.SampleWindow:]
	}
	s.samples[modeID] = list
}

// sampleView 交出样本的一份拷贝给选优用。
//
// 为什么要拷贝：Rank 只读，但调用它在锁外跑的那段时间里，队列可能正在往同一串后面追加。
func (s *Service) sampleView() map[string][]resolve.Sample {
	out := make(map[string][]resolve.Sample, len(s.samples))
	for id, list := range s.samples {
		out[id] = append([]resolve.Sample(nil), list...)
	}
	return out
}

// itemStateOf 取出这一条的内存态。调用方要么持有 s.mu，要么在单线程的队列执行体里。
func (s *Service) itemStateOf(itemID string) *itemState {
	st, ok := s.items[itemID]
	if !ok {
		st = &itemState{bestIPs: map[string]string{}}
		s.items[itemID] = st
	}
	return st
}

// headlineDomain 定出界面那一行"唯一的地址列"该说哪一个域名：按清单顺序，第一个这一轮有地址的；
// 一个都没有就退回第一个域名。
//
// 干什么用：契约里一条条目只有一列 bestIp／一列 appliedIp，可是一条条目可以有好几个域名，
// 而它们未必个个都检得出地址（2026-10-22 本机实测：github.com 通、gist.github.com 不通）。
// 什么情况下会错、出错时人看得见什么：写死第一个域名 → 那个域名没检出地址时，
// 这一行显示一个空地址却被标成"已检测／已生效"，用户看不出加速到底做成了一半。
// 怎么回来：检测那一段（commitDetect）与快照那一段（itemsLocked）都用这同一个函数，两处不会各挑一个。
func headlineDomain(item menu.Item, have map[string]string) string {
	for _, d := range item.Domains {
		if have[d] != "" {
			return d
		}
	}
	if len(item.Domains) > 0 {
		return item.Domains[0]
	}
	return ""
}
