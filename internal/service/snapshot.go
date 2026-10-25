package service

import (
	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/hosts"
	"github.com/hlep/github-accel/internal/menu"
)

// snapshot.go 是完整状态快照唯一的组装处。谁要往外推事件都得过它，一条也不许自己拼〔已定：契约"事件必须带完整快照"〕。
//
// 干什么用：把"服务内存里的运行态"和"hosts 文件此刻的真实内容"合成一份界面能直接画出来的东西。
// 什么情况下会错、出错时人看得见什么：
// ① 只带变化的那两个字段 → 界面靠猜补全，快照跟文件对不上，用户在外面手工改了 hosts 也看不见；
// ② 把"上次写入的地址"从服务内存里读 → 服务记一份、文件一份，早晚各说一套。
//
//	所以这一列永远现读文件算〔已定：D3 甲〕，读不动就把读不动这件事如实写进 lastError，
//	并把那一列留空——留空是"不知道"，不是"没有"，界面据此显示"—"而不是"未生效"。
//
// ③ items 的顺序跟着 map 走 → 界面每刷一次就重排一次。所以顺序只按 New 里认出的那一条（s.order）。
//
// 怎么回来：这一份代码是纯组装，不改任何状态；要加字段先加进 contract，再在这里填，两处一起红。

// Snapshot 交出此刻的完整快照（读文件、拼条目、拼任务）。
func (s *Service) Snapshot() contract.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

// publish 组装一份快照并推出去。
//
// 为什么锁在组装时拿、推之前放：广播要往每个连着的界面写 socket，攥着状态锁写会把整条队列停住。
// 放锁与推出去之间可能又有一次状态变化，那不要紧——事件带的永远是"某一刻的完整快照"，不是"半个变化"。
func (s *Service) publish(event string, payload map[string]any) {
	s.mu.Lock()
	snap := s.snapshotLocked()
	s.mu.Unlock()
	if s.cfg.Broadcaster == nil {
		return
	}
	s.cfg.Broadcaster.Broadcast(event, snap, payload)
}

// snapshotLocked 是真正的组装处。调用方必须已持有 s.mu。
func (s *Service) snapshotLocked() contract.Snapshot {
	doc, readErr := s.cfg.Engine.Read()
	state := s.stateLocked()
	if readErr != nil {
		// 读不出来这件事本身带码（GAC-HOSTS-001），界面据此说"读不到 hosts"，而不是"一条都没生效"。
		// 三样要一起换：只换原文（上一版的形状）就是"新句子配旧码"，界面会把读不到 hosts
		// 说成上一轮那句"刷 DNS 没成"。
		// 这里改的是这一份快照里的副本，不动 s.lastErr：读不动是"此刻这一次"的事实，
		// 每一帧都会重读一次，好了这句自然消失；把它记进服务那份就会留下一句过期话。
		state.LastError = readErr.Error()
		state.LastErrorCode = errs.CodeOf(readErr)
		state.LastErrorParams = errs.ParamsOf(readErr)
	}
	return contract.Snapshot{
		NetworkOffline: s.netOffline,
		State:          state,
		Items:          s.itemsLocked(doc),
		Tasks:          s.tasksLocked(),
	}
}

// itemsLocked 按固定顺序交出每一条的状态。doc 为 nil 表示这一份读不出来，那一列一律留空。
func (s *Service) itemsLocked(doc *hosts.Doc) []contract.Item {
	out := make([]contract.Item, 0, len(s.order))
	managed, prev := managedByItem(doc)
	for _, id := range s.order {
		it, _, ok := s.cfg.Catalog.ByItem(id)
		if !ok {
			continue // New 之后清单不会变；真到这一步是内存态与清单不一致，宁可少一行也不编一行
		}
		st := s.items[id]
		written := managed[id]
		before := prev[id]
		// 那一列地址说哪个域名，跟检测那一段用的是同一个判据（headlineDomain）。
		first := headlineDomain(it, st.bestIPs)
		out = append(out, contract.Item{
			ID:            id,
			Name:          it.Name,
			Status:        itemStatus(it, st, written),
			Managed:       len(written) > 0,
			AppliedIp:     written[first],
			PrevAppliedIp: before[first],
			BestIp:        st.bestIPs[first],
			Source:        st.source,
			SourceRole:    st.sourceRole,
			Latency:       st.latency,
			LastCheck:     st.lastCheck,
			ErrorCode:     st.errCode,
			ErrorParams:   st.errParams,
		})
	}
	return out
}

// itemStatus 算出这一条此刻该显示哪一档。八档全在 contract 那组常量里，这里只做判断。
//
// 判据的顺序就是这一列在界面上的故事：
// ① 文件里有我们写的行——先看它跟这一轮的最优地址是否逐域名一致，一致是"已生效"，不一致是"有更快的没应用"；
// ② 没有行、但有上一轮"写完发现不对、已经还原"的记号——那一档要盖过"已检测"，因为用户要点开那一行才知道发生过什么；
// ③ 没有行、检过——"已检测"；没检过却失败过——"出错"；两样都没有——"未设置"。
//
// 为什么这里不许出现"上次成功写入的地址"那份内存值：那一列的唯一来源是文件（D3 甲），
// 存了就等于承认服务有一份跟文件平起平坐的真相。
func itemStatus(item menu.Item, st *itemState, written map[string]string) string {
	if len(written) > 0 {
		if len(st.bestIPs) > 0 && matchesAll(item.Domains, written, st.bestIPs) {
			return contract.ItemApplied
		}
		return contract.ItemStale
	}
	switch {
	case st.rolledback:
		return contract.ItemRolledback
	case len(st.bestIPs) > 0:
		return contract.ItemDetected
	case st.errCode != "":
		return contract.ItemError
	}
	return contract.ItemUnset
}

// matchesAll 判"文件里我们写的那几行，是不是这一轮算出地址的那些域名都指到了对应的那个地址"。
//
// 干什么用：它是"已生效／有更快的没应用"那一档唯一的判据。
// 什么情况下会错、出错时人看得见什么：这一轮没检出地址的域名（见 detectItem 第 ④ 条）也算进来，
//
//	一条条目里死一个域名就永远显示"有更快的没应用"——那明明是没有更快的可应用，界面在说假话。
//
// 所以那种域名只在"文件里也还有我们写的行"时才算不匹配：那是上一轮的地址现在没人担保了，
// 如实留在"没应用"那一档，等下一轮检测给出地址。
//
// 怎么回来：判据只在这里，写入那一段（changesFor）用的是同一个"有没有地址"的分法。
func matchesAll(domains []string, written, want map[string]string) bool {
	for _, d := range domains {
		if want[d] == "" {
			if written[d] != "" {
				return false
			}
			continue
		}
		if written[d] != want[d] {
			return false
		}
	}
	return true
}

// managedByItem 把文件里那些行按条目分成两组：我们写的地址、以及写之前那一个地址。
//
// 为什么按条目 id 分而不是按域名分：一个域名挂在两条条目上正是清单里那条 ubisoft.com 的形状，
// 按域名分会让两条条目都以为这一行是自己的，界面上两行同时显示"已生效"。
func managedByItem(doc *hosts.Doc) (managed, prev map[string]map[string]string) {
	managed = map[string]map[string]string{}
	prev = map[string]map[string]string{}
	if doc == nil {
		return managed, prev
	}
	for _, l := range doc.Lines() {
		if !l.Managed || l.ItemID == "" {
			continue
		}
		if managed[l.ItemID] == nil {
			managed[l.ItemID] = map[string]string{}
			prev[l.ItemID] = map[string]string{}
		}
		for _, name := range l.Names {
			managed[l.ItemID][name] = l.IP
			prev[l.ItemID][name] = l.PrevIP
		}
	}
	return managed, prev
}

// tasksLocked 交出队列里每一行。processed／total 那两个数按批次算：
// 一次点击（一批）跑了几行、共几行，全局进度条只用这两个数，不许演假进度。
func (s *Service) tasksLocked() []contract.Task {
	total := map[string]int{}
	done := map[string]int{}
	for _, t := range s.tasks {
		total[t.batch]++
		if t.isTerminal() {
			done[t.batch]++
		}
	}
	out := make([]contract.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, contract.Task{
			ID:          t.id,
			Kind:        t.kind,
			Status:      t.status,
			ItemID:      t.itemID,
			Name:        t.name,
			Phase:       t.phase,
			Processed:   done[t.batch],
			Total:       total[t.batch],
			LastErr:     t.lastErr,
			ErrorCode:   t.errCode,
			ErrorParams: t.errParams,
			DurationMs:  t.durationMs(),
		})
	}
	return out
}

// status 给 GetStatus 那一问一答。
func (s *Service) status() contract.StatusReply {
	snap := s.Snapshot()
	return contract.StatusReply{Service: snap.State, WriteLockHeld: s.writeLockHeld()}
}

// noteCaveat 把回执上"写成了但该知道"的第一条记进状态栏。
//
// 为什么只记一条：这几条的成因互不相干（属主没补回来／备份没存下），一次写入里同时出两起的形状我没见到，
// 而状态栏那一行本来就只装得下一句。真要都显示，得先给契约加一个数组字段——那是显式改动，不在这里偷加。
func (s *Service) noteCaveat(c hosts.Caveat) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noteStateCaveat(c)
}
