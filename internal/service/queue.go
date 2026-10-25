package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
)

// errPaused 是"这一条被暂停叫停在方式之间"的那个记号。
//
// 为什么要一个专门的记号而不是直接把状态改成等待：跑检测的那段代码有好几处循环，
// 每一处都要"看见暂停就走人"。用一个能被 errors.Is 认出来的错误值，收尾那一步才知道
// 这一次不是失败、不许记失败样本、也不许把暂停标志清掉（L2 那句"只有恢复能离开暂停"）。
var errPaused = errors.New("队列暂停中，这一条退回等待")

// pausedNow 问一次闸此刻开着还是关着。
//
// 检查点只放在"一个方式跑完"的那个边界上，也就是 M3 那句"暂停要能停在方式之间"里的那个"之间"。
// 为什么不在半支握手上查：真停在探测中途＝这一次探测的样本丢了，
// 而丢了样本的方式在稳定性 S 上白掉一档，下一轮它被名次顶下去，界面看不出任何原因。
func (s *Service) pausedNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

// submit 把一批条目排进队列，立刻返回批次号——不等它们跑完〔已定：契约硬性要求"立刻返回任务号"〕。
//
// 干什么用：界面上一次点击（全部检测、勾选的那几条应用、恢复这一条）变成一个批次，
// 批次里每条一行，各自的进度与结果单独看得见。
// 什么情况下会错、出错时人看得见什么：给了清单里没有的条目 id → 当场回 GAC-TASK-004 并把那个 id 带回去；
// 静默收下的话界面那一行永远不动，用户只当服务卡住了。
// 怎么回来：id 一律先跟清单核一遍再入队；同一条目在一批里只留一行（两行就是两条写入抢同一行 hosts）。
func (s *Service) submit(kind string, itemIDs []string, origin string, picks map[string]string) (string, error) {
	switch kind {
	case contract.KindDetect, contract.KindApply, contract.KindRestore:
	default:
		return "", errs.New(errs.CodeTaskNotFound, map[string]any{"kind": kind}, "不认识的任务种类")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(itemIDs) == 0 {
		// 空＝全部：界面上「全部检测」「恢复默认」那两颗按钮就是这一档。
		itemIDs = append([]string(nil), s.order...)
	}
	seen := map[string]bool{}
	tasks := make([]*task, 0, len(itemIDs))
	for _, id := range itemIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		it, _, ok := s.cfg.Catalog.ByItem(id)
		if !ok {
			return "", errs.New(errs.CodeTaskBadItem, map[string]any{"itemId": id}, "清单里没有这条条目")
		}
		s.taskNo++
		tasks = append(tasks, &task{
			id:     fmt.Sprintf("task-%d", s.taskNo),
			kind:   kind,
			itemID: id,
			name:   it.Name,
			status: contract.TaskWaiting,
			origin: origin,
			pick:   picks[id],
		})
	}
	s.batchNo++
	batch := fmt.Sprintf("batch-%d", s.batchNo)
	round := &roundState{}
	for _, t := range tasks {
		t.batch = batch
		t.round = round
	}
	s.tasks = append(s.tasks, tasks...)
	s.notify()
	return batch, nil
}

// pause 把闸关上。正在跑的那一条照常跑完，只是不再取下一条；
// 唯一会打断在跑任务的是检测，它会在下一个方式边界上走人（detectrun.go 那个循环）。
//
// 这里绝不清任何收尾标志——收尾顺手清暂停就是 L2 那个"要点两下"。
func (s *Service) pause() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = true
	return s.paused
}

// resume 把闸打开。这是全项目唯一能把 paused 关掉的地方〔已定：L2〕。
func (s *Service) resume() bool {
	s.mu.Lock()
	s.paused = false
	s.mu.Unlock()
	s.notify()
	return s.pausedNow()
}

// cancel 只撤等待中的行，正在跑那一条不受影响（照原型 :3124 那句「正在执行的任务不受影响」）。
//
// 干什么用：taskID 是批次号就撤这一批里所有还没跑的，是单条号就只撤那一条；
// 空号＝撤全部等待中的（原型 :734 顶部那颗「取消等待」不针对某一行，给的就是这一档）。
// 什么情况下会错、出错时人看得见什么：号查不到 → GAC-TASK-001 并把号带回去；
// 号查得到但那一行早已跑完 → GAC-TASK-002，界面要说"这条已经跑完，撤不了"，不许假装撤掉了。
// 怎么回来：改状态一律走 moveTo，走不通（终态）就是撤不动；那张流转表在 state.go 一个地方。
func (s *Service) cancel(taskID string) (int, error) {
	s.mu.Lock()
	found := false
	n := 0
	for _, t := range s.tasks {
		if taskID != "" && t.id != taskID && t.batch != taskID {
			continue
		}
		if taskID != "" {
			found = true
		}
		if t.status == contract.TaskWaiting && t.moveTo(contract.TaskCancelled) {
			t.phase = ""
			n++
		}
	}
	s.mu.Unlock()

	if taskID != "" && !found {
		return 0, errs.New(errs.CodeTaskNotFound, map[string]any{"taskID": taskID}, "没有这个任务号")
	}
	if n == 0 {
		if taskID == "" {
			return 0, errs.New(errs.CodeTaskFinal, map[string]any{"waiting": 0}, "没有等待中的任务可撤")
		}
		return 0, errs.New(errs.CodeTaskFinal, map[string]any{"taskID": taskID}, "这一行已经跑完了，撤不动")
	}
	s.publish(contract.EventQueueProgress, map[string]any{"cancelled": n, "taskId": taskID})
	return n, nil
}

// clearFinished 把「已经跑完且不欠人一句话」的那些行从任务表里抹掉，并交出抹完之后那份快照（契约 ClearTasks）。
//
// 干什么用：界面上那颗「清空已完成」清的是服务这一份表，不是前端本机视图。
// 上一版只在前端把行 filter 掉，下一次事件带着同一批行回来，症状正是"点了没反应"。
// 什么情况下会错、出错时人看得见什么：
// ① 判据写成"状态不是 waiting" → 正在跑那一条也被抹掉：队列那一行当场消失，
//
//	服务却还在往一个界面上已经不存在的行推阶段事件，进度与结果两处对不上；
//
// ② 抹完不回快照 → 界面只能干等下一次事件才把行收掉，而"清空"这件事本身不产生任何变化可推。
// ③ 筛子用 isTerminal()（六档全抹）→ 失败与已回滚那两行一起没了，界面上「重试失败的那几条」
//
//	再无原料可重试：清空这一件事顺手砍掉了另一颗按钮要用的东西（已定：改留两档，见 state.go isClearable）。
//
// 怎么回来：该抹该留只由 isClearable() 判（state.go 那一处是唯一的家），
// waiting／running／restoring 三类原位留下、failed／rolledback 两档也留下；
// 批次那两个进度数由 tasksLocked 按剩下的行重新算，不另存。
// 抹掉与取快照在同一次取锁里做完：分两次取锁，中间插进一次收尾，回执里那张表就多出一行刚跑完的——
// 那行"该抹没抹"，用户看到的还是同一句"点了没反应"。
func (s *Service) clearFinished() contract.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.tasks[:0]
	for _, t := range s.tasks {
		if t.isClearable() {
			continue
		}
		kept = append(kept, t)
	}
	s.tasks = kept
	return s.snapshotLocked()
}

// pump 是那条串行通道：取一条、跑一条、收尾一条。
func (s *Service) pump(ctx context.Context) {
	for {
		t := s.take()
		if t == nil {
			return
		}
		s.finish(t, s.exec(ctx, t))
	}
}

// take 取出下一条该跑的任务；闸关着或没有可跑的就等唤醒。
func (s *Service) take() *task {
	for {
		s.mu.Lock()
		if s.stopping {
			// 服务要退了：等待中的行如实标"已跳过"，不许演成"跑完了"。
			for _, t := range s.tasks {
				if t.status == contract.TaskWaiting && t.moveTo(contract.TaskSkipped) {
					t.phase = ""
				}
			}
			s.mu.Unlock()
			s.publish(contract.EventQueueDone, nil)
			return nil
		}
		if !s.paused && s.running == nil {
			for _, t := range s.tasks {
				if t.status != contract.TaskWaiting {
					continue
				}
				t.moveTo(contract.TaskRunning)
				// 秒表每次取跑都重按一次：暂停退回等待、恢复后再跑的那一行，「耗时」只算最后这一段。
				t.startedAt = time.Now()
				s.running = t
				s.mu.Unlock()
				return t
			}
		}
		s.mu.Unlock()

		select {
		case <-s.wake:
		case <-s.stop:
		}
	}
}

// exec 按任务种类交给对应的执行体。种类只有契约里那三种。
func (s *Service) exec(ctx context.Context, t *task) error {
	switch t.kind {
	case contract.KindDetect:
		return s.detectItem(ctx, t)
	case contract.KindApply:
		return s.applyItem(ctx, t)
	case contract.KindRestore:
		return s.restoreItem(ctx, t)
	}
	return errs.New(errs.CodeTaskNotFound, map[string]any{"kind": t.kind}, "队列里出现了不认识的任务种类")
}

// finish 收尾：把结果落到任务行与条目状态上，然后推事件。
//
// 五条分支各管一件事，谁也不许顺手干别人的活：
//   - errPaused → 退回等待，不记失败、不清暂停；
//   - 服务在退 → 标"已跳过"，这才是"没跑"的实话；
//   - 上限用完（GAC-DETECT-005）→ 也标"已跳过"，但这一条不碰条目那一档：本轮没轮到不是这条坏了
//     （2026-10-22 实测的形状：一次「全部检测」有 38 条撞在上限上，界面上 38 行全红，
//     用户看到的是"这工具把 38 条域名测坏了"，实际上一行都没测完而已）；
//   - 成功 → 恢复类任务是 restored，其余是 done；检测与应用成功才清掉上一轮的失败原因；
//   - 带码的失败 → 行上留原文，条目留码与参数；写入那一步"重读对不上"另有一档（已回滚）。
func (s *Service) finish(t *task, err error) {
	s.mu.Lock()
	// 抬起秒表：下面五条分支都要走收尾，所以这一句放在分支之前，不在每一支里各写一遍
	// （漏一支的表现是那一行的「耗时」永远空着，而状态是跑完了——那一句自相矛盾的话没人查得动）。
	t.finishedAt = time.Now()
	st := s.itemStateOf(t.itemID)
	requeue := false
	switch {
	case err == nil:
		t.phase, t.lastErr, t.errCode, t.errParams = "", "", "", nil
		to := contract.TaskDone
		if t.kind == contract.KindRestore {
			to = contract.TaskRestored
		}
		t.moveTo(to)
		st.rolledback = false
		if t.kind != contract.KindRestore {
			st.errCode, st.errParams = "", nil
		}
	case errors.Is(err, errPaused):
		// 退回等待不是这一行跑坏了：原因要跟状态一起收掉，否则界面上会出现"待检测／等待中"
		// 旁边挂着上一轮那句"写入失败"的自相矛盾的话。
		t.phase, t.errCode, t.errParams = "", "", nil
		requeue = t.moveTo(contract.TaskWaiting)
	case errors.Is(err, context.Canceled), s.stopping:
		t.phase, t.errCode, t.errParams = "", "", nil
		t.moveTo(contract.TaskSkipped)
	case errs.CodeOf(err) == errs.CodeDetectBudget:
		// 这一档说的是"本轮不用再往下试了"，不是"这一条测坏了"：
		// 行上留原因（界面那一列要能回答"这条为什么没动"），状态栏那句"剩下的排队"照发，
		// 条目那一档一个字不碰——上一轮的档位继续用，这一轮已经拿到地址的域名由 commitDetect 记好了。
		t.phase, t.lastErr = "", err.Error()
		t.errCode, t.errParams = errs.CodeOf(err), errs.ParamsOf(err)
		t.moveTo(contract.TaskSkipped)
		s.noteStateErr(err)
	default:
		t.phase = ""
		t.lastErr = err.Error()
		code := errs.CodeOf(err)
		if code == "" || code == errs.CodeUnknown {
			code = errs.CodeIPCHandler
		}
		t.errCode, t.errParams = code, errs.ParamsOf(err)
		if code == errs.CodeHostsVerify {
			// 这一档的成因就是"写完重读对不上、已经用备份还原了"（hosts.Engine 里那一步），
			// 所以行上是"已回滚"、条目那一档也是 rolledback，而不是"没改成"。
			t.moveTo(contract.TaskRolledback)
			st.rolledback = true
		} else {
			t.moveTo(contract.TaskFailed)
		}
		st.errCode = code
		st.errParams = errs.ParamsOf(err)
		s.noteStateErr(err)
	}
	kind, itemID := t.kind, t.itemID
	s.running = nil
	done := s.idleLocked()
	s.mu.Unlock()

	payload := map[string]any{"taskId": t.id, "batchId": t.batch, "itemId": itemID, "kind": kind}
	if ev, ok := eventForKind(kind); ok {
		s.publish(ev, payload)
	}
	switch {
	case requeue:
		// 闸还关着，这次唤醒只让它再看一眼闸；不唤醒的话恢复的人要点两次（L2 另一半）。
		s.notify()
	case done:
		s.publish(contract.EventQueueDone, payload)
	default:
		s.notify()
	}
}

// idleLocked 判队列是不是空了（没有在跑的、也没有等待中的）。调用方必须已持有 s.mu。
func (s *Service) idleLocked() bool {
	for _, t := range s.tasks {
		switch t.status {
		case contract.TaskWaiting, contract.TaskRunning:
			return false
		}
	}
	return true
}

// eventForKind 给出"这一行跑完了该推哪条事件"。
//
// 为什么检测单独一条：契约 v0.2 加的 detect:progress 就是"单项检测完成"（技术方案第 6 节第 1 条），
// 界面靠它逐行刷新最优 IP；写入类任务走 queue:progress。queue:done 永远是"整批跑完"那一下。
func eventForKind(kind string) (string, bool) {
	switch kind {
	case contract.KindDetect:
		return contract.EventDetectProgress, true
	case contract.KindApply, contract.KindRestore:
		return contract.EventQueueProgress, true
	}
	return "", false
}

// setPhase 把这一行推进到某一个阶段，并推一条进度事件。
//
// 四个阶段的字面量只从 contract.Phase* 取：服务侧不写文案，界面拿 key 去语言文件换句子〔已定：D6 乙〕。
func (s *Service) setPhase(t *task, phase string) {
	s.mu.Lock()
	t.phase = phase
	s.mu.Unlock()
	s.publish(contract.EventQueueProgress, map[string]any{
		"taskId": t.id, "batchId": t.batch, "itemId": t.itemID, "kind": t.kind, "phase": phase,
	})
}
