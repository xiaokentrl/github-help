/* 六条主流程的编排：单独检测、批量检测、入队并跑队列、还原一条、还原一类、恢复全部、启动自动化。干什么用：
   把"点了那颗钮之后按什么顺序发生什么"集中在一处，界面只负责把进度画出来。
   从这一版起，这里一个字段也不自己改：一次点击＝一个请求（core/api/client.js 的动词），
   跑出来的每一行都由服务的事件带回来，进来的那一份快照一律走 core/api/snapshot.js 的 mergeSnapshot 这一条路。

   会错在哪：
   ① 在界面这边"顺手"写一次状态（点下去就把那一行标成执行中、把 IP 抄进条目）。
      为什么这是错的：界面上演出来的那一行，服务随后一条事件带完整快照回来就把它盖掉，
      症状是"先跳一下再退回原样"；更糟的是写入类的那一栏（appliedIp／托管与否）真值只在 hosts 文件里。
      所以现在抬起的只有 pending 那两只把手，别的都等服务说。
   ② 把手抬起之后忘了松。
      一次点击到服务答话之间有一段，没把手＝那一段里再点一次就是两次提交（两个写 hosts 的动作同时开工，C2 那笔账）；
      有把手却没有地方松＝界面从此一直忙。所以抬起与松开都在 dispatch 一处（＝那一次请求有没有回话），
      refreshBusy 只负责把把手与服务那张表合成两个忙标志，别处再写第三套判据就是漂移的开始。
   ③ 事件没来的时候当"没发生"。
      detect／apply／restore 三个方法回的是任务号，不是结果——真结果随后一条条从事件带快照推。
      所以"这一批跑完了"只能问那张表（whenKindIdle），不能 await 那一次调用；等不到表里有活动行就一律不算跑完。
   ④ 断线之后还让流程干等。
      线断了，那一批的行不会再有回执，等待者必须当场拿到一句"连不上后台服务"，
      否则界面上那颗钮永远转圈，而这正是用户最容易误以为"服务卡死了"的一种。

   怎么回来：ui 缺一个回调就在启动那一下抛错（不是跑到一半才炸）；每条流程的形状都在 tests/flows.test.mjs 里
             用假 api＋假事件源跑一遍，含断线那一支。红了先看这里有没有出现第二种"改状态"的路子。 */

import {
  state, allItems, findItem, findCategoryOfItem, checkedItems, canApply, canRestoreHosts,
  snapshotCollapsed, restoreCollapsed, collapseOnly
} from './store.js';
import { api } from './api/client.js';
import { WireError } from './api/wire.js';
import { mergeSnapshot, refreshBusy } from './api/snapshot.js';
import { replyErrorText } from './api/errors.js';
import * as gate from './task/gate.js';

const REQUIRED_UI = ['renderTree', 'renderDetail', 'renderStatus', 'scrollToInt', 'confirm', 'toast'];

/** 服务那张表里"还没跑完"的三档（等待／在跑／恢复中）。
    为什么在这里写一份而不用 model.js 那两个 isFinished：那两支是给队列行画字用的，
    判的是"这一行有没有出结果"（检测的 cancelled 算出结果、但这一行不该再滚进度），两句话不是一件事。 */
const ACTIVE = ['waiting', 'running', 'restoring'];

/** 三个种类各自那只把手：检测用自己的，应用与恢复共用队列那一只（服务那条串行通道本来就是一条）。 */
const LATCH = { detect: 'detect', apply: 'queue', restore: 'queue' };

export function createFlows({ t, ui }) {
  for (const k of REQUIRED_UI) {
    if (typeof ui[k] !== 'function') throw new Error(`flows：注入的 ui 缺 ${k}()，这条流程跑到那儿会静默停住`);
  }

  /** whenKindIdle 那一批等着的人：线一断就全部给一句"连不上"，不许干等 */
  const waiters = [];

  /** 一共并进来过几份快照（开机那一次、每一条事件、ClearTasks 的回执都算一份）。
      上面 whenKindIdle 的"回话之后"就是拿这个数比的，所以它只增不减。 */
  let frames = 0;
  /** 两只把手各自最近一次"服务答过话"是在第几份快照那一刻 */
  const repliedAt = { detect: 0, queue: 0 };

  /** 门给的话可能为空（原型那四处"挡路但不出声"），所以过一道 say() 而不是直接 ui.toast(t(...)) */
  function say(key, params) {
    if (key) ui.toast(t(key, params));
  }

  /* ---------- 画：一份快照进来之后，界面上该补的那几笔 ---------- */

  /** 焦点跟随（detectFocusFollow／applyFocusFollow 那两档）：跑的时候只展开当前这条所在的类目，跑完回到跑之前的样子。
      上一版是循环里逐条 step 的；现在没有循环了（跑在服务那边），所以改由"这一类里正在跑的那一行"驱动。 */
  function makeFocusFollow(flagName) {
    let begun = false;
    let original = null;
    let currentCat = null;
    return {
      step(item) {
        if (!state.settings[flagName]) return;
        if (!begun) {
          begun = true;
          original = snapshotCollapsed();
          state.collapsed.clear();
        }
        const cat = findCategoryOfItem(item.id);
        const id = cat ? cat.id : null;
        if (id === currentCat) return;
        collapseOnly(id);
        currentCat = id;
        ui.renderTree();
      },
      finish() {
        if (!begun) return;
        begun = false;
        currentCat = null;
        restoreCollapsed(original);
        original = null;
        ui.renderTree();
      }
    };
  }

  const follows = { detect: makeFocusFollow('detectFocusFollow'), apply: makeFocusFollow('applyFocusFollow') };
  /** 上一次滚到的是哪一条：同一行连着来几个事件不必每帧都滚一次（原型每个任务只滚一次） */
  let lastScrolled = '';

  function activeRow(kind) {
    return state.queue.find(tk => tk.kind === kind && ACTIVE.includes(tk.status)) || null;
  }

  function paint() {
    for (const kind of ['detect', 'apply']) {
      const row = activeRow(kind);
      const item = row ? findItem(row.itemId) : null;
      if (item) follows[kind].step(item);
      else follows[kind].finish();
    }
    ui.renderTree();
    ui.renderDetail();
    ui.renderStatus();
    const running = activeRow('detect') || activeRow('apply');
    if (running) {
      if (running.itemId !== lastScrolled) {
        lastScrolled = running.itemId;
        ui.scrollToInt(running.itemId);
      }
    } else {
      lastScrolled = '';
    }
    settleWaiters();
  }

  /** 三条来源（开机那一次 GetSnapshot、每一条事件、ClearTasks 的回执）都从这一个口子进 */
  function ingest(snapshot) {
    mergeSnapshot(state, snapshot, t);
    frames += 1;
    paint();
  }

  /* ---------- 忙把手与"这一批跑完了" ---------- */

  function hasActive(kind) {
    return state.queue.some(tk => tk.kind === kind && ACTIVE.includes(tk.status));
  }

  /** 等的是"回话之后并进来的那一份快照里，这一类没有活动行"。
      为什么要 after 那一道：回执之前拍出来的那一份快照可能还是上一轮排空时的旧表，
      拿它当"我这一批跑完了"就是提前收工——界面上「检测完成」那句会比真结果先到。
      为什么不用"看到活动行才算数"：服务的快照会合并，跑得快的那一批可能第一份就已是终态，
      那样等就成了永远等不到；而回执之后至少会并进来一份（每一条跑完都推一次，排空再推 queue:done）。
      还剩的一处靠运气：壳往页面写回执的那一个 goroutine 与转事件的那一个不排队，
      跑得非常快的那一批有可能所有帧都赶在回执之前到——这时候回话之后不会再有第二份快照，这一个等待者就收不了工。
      界面这一侧分不出"上一轮的旧表"与"这一批已经跑完的表"（契约的 Task 没有批次号，回执只回一个任务号），
      所以真修法在服务那侧：回执里带回提交那一刻的快照，或给 Task 加一栏批次号。登记在 #52，等一句要不要改契约。 */
  function whenKindIdle(kind) {
    return new Promise((resolve, reject) => waiters.push({ kind, after: repliedAt[LATCH[kind]], resolve, reject }));
  }

  /** 能收工的条件有三个：这一类真的回过话（把手已松）、回话之后并进来过快照、那张表里这一类没有活动行。 */
  function settleWaiters() {
    for (let i = waiters.length - 1; i >= 0; i--) {
      const w = waiters[i];
      if (hasActive(w.kind) || state.pending[LATCH[w.kind]] || frames <= w.after) continue;
      waiters.splice(i, 1);
      w.resolve();
    }
  }

  /** 线通不通的第一手依据是"这一问有没有回话"，不是壳推的那一条 gac:state。
      为什么要这一支：bridge.go 的 setState 只在通↔不通翻转的那一刻推，页面订阅得晚就没赶上那一句 connected:true，
      于是 store 里那个初值 false 会一直留着；而 refreshBusy 认的是"断线一律不忙"——
      留着 false 等于两只把手从此拦不住第二次点击（C2 那笔账，界面上看不出来，只是多写一次 hosts）。
      谁能说通：任何一次拿到回执的调用；谁能说断：只有 onLink 那一条线（这里不另写第二套判据）。 */
  function markLineUp() {
    if (state.connected) return;
    state.connected = true;
    refreshBusy(state);
  }

  /** 一次点击＝一个请求。两只把手管的就是"发出去了、服务还没答话"那一段：抬起与松开都只在 dispatch 这一处，
      snapshot.js 的 refreshBusy 只把它们与服务那张表合成界面上那两个忙标志。
      为什么抬起之后要立刻重算一次忙标志：门（gate.js）问的是 batchDetecting／queueRunning 那两个数，
      把手自己不是。抬了不重算，"发出去了、还没回话"那一段里门还是放行，第二次点击就真的发出去第二次（C2 那笔账）。
      为什么松手不再等"服务那张表里出现活动行"：任务号是服务立刻回的，事件却可能比回执早到、也可能晚到；
      把松开挂在事件上就会出现"请求明明成功了、把手永远抬着"那种把界面锁死的情形。 */
  async function dispatch(kind, call) {
    const latch = LATCH[kind];
    state.pending[latch] = true;
    refreshBusy(state);
    ui.renderStatus();
    let result = null;
    let thrown = null;
    try {
      result = await call();
    } catch (err) {
      thrown = err;
    }
    state.pending[latch] = false;
    if (!thrown) markLineUp();     // 回过话＝这条线是通的
    refreshBusy(state);
    if (thrown) {
      fail(thrown);           // fail 里补那一次 renderStatus
      return null;
    }
    // 记下"回话的那一刻见过几份快照"：这一批的结局只准看这之后并进来的那一份
    repliedAt[latch] = frames;
    ui.renderStatus();
    return result;
  }

  /* ---------- 出错就说一句人话 ---------- */

  /** 服务与通道只回码与参数，界面上这一句由语言文件凑（已定：D6 乙）；报错那条不自动收（Toast 的 kind:'error'）。
      err.announced 是断线那条线的挡板：那一句由 onLink 说过一次，等待中的流程再拿到同一个 err 只为了停下来，
      不许每处各说一遍——同屏两句一模一样的「连不上后台服务」看起来像工具自己坏了。 */
  function fail(err) {
    const code = err?.code || '';
    const params = err?.params || {};
    state.lastError = { code, params, text: err?.message || '' };
    if (!err?.announced) ui.toast(replyErrorText(t, err), 'error');
    ui.renderStatus();
  }

  /** 每个对外的动作都从 action() 出去：门→请求→等跑完，中间任何一步抛了都只换一句人话，不许留"点了没反应"。 */
  function action(fn) {
    return async (...args) => {
      try {
        return await fn(...args);
      } catch (err) {
        fail(err);
        return undefined;
      }
    };
  }

  /* ---------- 检测 ---------- */

  /** 「检测全部」跑完那一句汇总：待应用 3 · 已是最优 1——零的那档不进句子（原型 :2422-2431） */
  function detectSummary(targets) {
    const parts = [];
    const push = (n, key) => { if (n > 0) parts.push(t(key, { n })); };
    const live = targets.map(it => findItem(it.id)).filter(Boolean);
    push(live.filter(i => i.status === 'detected').length, 'flow.detect.partDetected');
    push(live.filter(i => i.status === 'applied').length, 'flow.detect.partApplied');
    push(live.filter(i => i.status === 'stale').length, 'flow.detect.partStale');
    push(live.filter(i => i.status === 'error').length, 'flow.detect.partError');
    return parts.length ? parts.join(' · ') : t('flow.detect.noChange');
  }

  /** 发一批检测并等到这一类全部跑完；发不出去（服务回一句失败）就是 false。
      不带门——门在调用它的那一颗钮上，这一句谁调都一样。上一版把门写在 detectAll 里面，
      于是启动自动化（它自己先把 startupAutomationRunning 抬起来）调它就被自己那道门挡成空转，一句也不测。 */
  async function sendDetect(itemIDs) {
    const r = await dispatch('detect', () => api.detect(itemIDs));
    if (!r) return false;
    await whenKindIdle('detect');
    return true;
  }

  const detectSingle = action(async (item) => {
    const g = gate.detectSingle(state, item);
    if (!g.ok) { say(g.key); return; }
    await sendDetect([item.id]);
  });

  const detectAll = action(async () => {
    const busy = gate.busyBatchDetect(state);
    // 批量与启动忙时原型直接 return 不出话（:2321-2322），只有队列那条给一句
    if (!busy.ok) { say(busy.key); return []; }
    const selected = checkedItems();
    if (!selected.length) { ui.toast(t('gate.detect.pick')); return []; }

    const queued = new Set(state.queue.filter(tk => tk.kind === 'detect' && ACTIVE.includes(tk.status)).map(tk => tk.itemId));
    const runningApply = new Set(state.queue.filter(tk => tk.kind === 'apply' && tk.status === 'running').map(tk => tk.itemId));
    const targets = selected.filter(it => !queued.has(it.id) && !runningApply.has(it.id));
    if (!targets.length) { ui.toast(t('gate.detect.nothing')); return []; }

    ui.toast(t('flow.detect.start', { n: targets.length }));
    if (!await sendDetect(targets.map(i => i.id))) return [];
    ui.toast(t('flow.detect.summary', { parts: detectSummary(targets) }));
    return targets;
  });

  /* ---------- 应用 ---------- */

  /** 还没检测过又被用户确认"先检测再应用"的那几条：服务在自己的任务里补检测（applyrun.go ①），
      所以这里不再单开一轮检测——上一版那条 detectSubset 就是同一个功能在两边各跑一遍。 */
  async function sendApply(itemIDs) {
    return !!(await dispatch('apply', () => api.applyQueue(itemIDs)));
  }

  const applyQueue = action(async () => {
    const busy = gate.busyApplyQueue(state);
    if (!busy.ok) { say(busy.key); return; }
    const selected = checkedItems();
    if (!selected.length) { ui.toast(t('gate.apply.pick')); return; }

    const undetected = selected.filter(i => !canApply(i) && i.status !== 'error');
    if (undetected.length) {
      const ok = await ui.confirm({
        title: t('flow.apply.askTitle', { n: undetected.length }),
        lines: [t('flow.apply.askBody'), t('flow.apply.askBody2', { n: undetected.length })],
        confirmText: t('flow.apply.askConfirm')
      });
      if (!ok) return;
    }

    const keep = new Set(undetected.map(i => i.id));
    const applicable = selected.filter(i => canApply(i) || keep.has(i.id));
    if (!applicable.length) { ui.toast(t('flow.apply.noneApplicable')); return; }
    const activeIds = new Set(state.queue.filter(tk => ACTIVE.includes(tk.status)).map(tk => tk.itemId));
    const targets = applicable.filter(i => !activeIds.has(i.id));
    if (!targets.length) { ui.toast(t('flow.apply.alreadyQueued')); return; }
    if (!await sendApply(targets.map(i => i.id))) return;
    ui.toast(t('flow.enqueued', { n: targets.length }));
  });

  const applySingle = action(async (item) => {
    const g = gate.applyOne(state, item);
    if (!g.ok) { say(g.key); return; }
    if (state.queue.some(tk => ACTIVE.includes(tk.status) && tk.itemId === item.id)) {
      ui.toast(t('flow.apply.alreadyQueuedOne'));
      return;
    }
    if (!await sendApply([item.id])) return;
    ui.toast(t('flow.enqueuedOne', { name: item.name }));
  });

  /* ---------- 还原 / 恢复 ---------- */

  /** 树上与详情里那颗 ↺：确认框的字照原型 :2780-2800 那三行，prevAppliedIp 为空就是"应用前本来没这条" */
  const restoreItemHosts = action(async (item) => {
    const g = gate.restoreItem(state, item);
    if (!g.ok) { say(g.key); return; }
    const ok = await ui.confirm({
      title: t('flow.restore.itemConfirmTitle'),
      lines: [
        t('flow.restore.itemConfirmBody', { name: item.name }),
        t('flow.restore.prevIp', { ip: item.prevAppliedIp || t('flow.noRecord') }),
        t('flow.restore.curIp', { ip: item.appliedIp || t('flow.noRecord') })
      ],
      warn: item.prevAppliedIp === null ? t('flow.restore.wasAbsent') : t('flow.restore.willDisappear'),
      confirmText: t('flow.restore.confirm'),
      danger: true
    });
    if (!ok) return;
    await dispatch('restore', () => api.restoreItem(item.id));
  });

  /** 一类里逐条还原：契约的 RestoreItem 一次只收一条，所以这里发 n 次；
      等的是这一类全部跑完，成功与否看回来那一份表——"文件里还有没有我们那几行"（managed）才是凭据。 */
  const restoreCategoryHosts = action(async (cat) => {
    const restorable = cat.items.filter(it => canRestoreHosts(it));
    const g = gate.restoreCategory(state, restorable.length);
    if (!g.ok) { say(g.key); return; }
    const ok = await ui.confirm({
      title: t('flow.restore.catTitle', { name: cat.name }),
      lines: [
        t('flow.restore.catCount', { n: restorable.length }),
        t('flow.restore.catDisappear'),
        t('flow.restore.catListLabel'),
        ...restorable.map(it => t('flow.restore.catLine', {
          name: it.name,
          prev: it.prevAppliedIp || t('flow.noRecord'),
          cur: it.appliedIp || t('flow.noRecord')
        }))
      ],
      confirmText: t('flow.restore.confirm'),
      danger: true
    });
    if (!ok) return;

    for (const it of restorable) {
      if (!await dispatch('restore', () => api.restoreItem(it.id))) break;
    }
    await whenKindIdle('restore');

    const live = restorable.map(it => findItem(it.id)).filter(Boolean);
    const successCount = live.filter(i => !i.managed).length;
    const failCount = restorable.length - successCount;
    if (failCount === 0) ui.toast(t('flow.restore.catAll', { name: cat.name, n: successCount }));
    else if (successCount === 0) ui.toast(t('flow.restore.catNone', { name: cat.name, n: failCount }));
    else ui.toast(t('flow.restore.catMixed', { name: cat.name, ok: successCount, fail: failCount }));
  });

  /** 队列行里那颗「↺ 恢复到应用前」：走的是服务那一条恢复任务（跑完这一行标"已恢复"），跟 ↺ 那颗不是同一回事 */
  const restoreTaskRow = action(async (taskId) => {
    const task = state.queue.find(x => x.id === taskId);
    if (!task || task.kind !== 'apply' || task.status !== 'done') return;
    const g = gate.restoreTask(state);
    if (!g.ok) { say(g.key); return; }

    const item = findItem(task.itemId);
    if (!item) return;
    if (!canRestoreHosts(item)) {
      // 文件里已经没有我们那几行了：这一行本来跑完过，服务那张表说的是实话，这里只补一句，不改那一行的状态
      ui.toast(t('flow.restore.alreadyDone'));
      return;
    }
    const ok = await ui.confirm({
      title: t('flow.restore.taskTitle'),
      lines: [
        t('flow.restore.taskBody', { name: task.name }),
        t('flow.restore.prevIp', { ip: item.prevAppliedIp || t('flow.noRecord') }),
        t('flow.restore.curIp', { ip: item.appliedIp || t('flow.noRecord') }),
        t('flow.restore.phases')
      ],
      confirmText: t('flow.restore.taskConfirm'),
      danger: true
    });
    if (!ok) return;

    if (!await dispatch('restore', () => api.restoreItem(item.id))) return;
    await whenKindIdle('restore');
    const rows = state.queue.filter(tk => tk.kind === 'restore' && tk.itemId === item.id);
    const last = rows.at(-1);
    if (last && (last.status === 'failed' || last.status === 'rolledback')) {
      ui.toast(t('flow.restore.taskFailedToast'), 'error');
      return;
    }
    ui.toast(t('flow.restore.taskDone', { name: task.name }));
  });

  /** 工具栏「恢复」：撤掉全部托管行（RestoreHosts 无参数，服务自己展开成"每一条一行"），不可撤销 */
  const restoreAll = action(async () => {
    const busy = gate.busyRestoreAll(state);
    if (!busy.ok) { say(busy.key); return; }
    const ok = await ui.confirm({
      title: t('flow.restoreAll.title'),
      lines: [t('flow.restoreAll.lead'), t('flow.restoreAll.drop'), t('flow.restoreAll.backup'), t('flow.restoreAll.clear')],
      warn: t('flow.restoreAll.warn'),
      confirmText: t('flow.restoreAll.confirm'),
      danger: true
    });
    if (!ok) return;
    if (!await dispatch('restore', () => api.restoreHosts())) return;
    await whenKindIdle('restore');
    ui.toast(t('flow.restoreAll.done'));
  });

  /* ---------- 队列操作 ---------- */

  /** 暂停／继续是同一颗钮的两种文字（原型 :2083）：闸此刻开着还是关着由服务回（QueueReply.paused），这里不自己翻 */
  async function pauseToggle() {
    try {
      const r = state.paused ? await api.resumeQueue() : await api.pauseQueue();
      state.paused = !!r?.paused;
      ui.toast(state.paused ? t('flow.queue.pauseWillHold') : t('flow.queue.resumed'));
    } catch (err) {
      fail(err);
      return;
    }
    ui.renderStatus();
  }

  const cancelWaiting = action(async () => {
    const waiting = state.queue.filter(tk => tk.status === 'waiting');
    if (!waiting.length) return;
    const ok = await ui.confirm({
      title: t('flow.queue.cancelTitle'),
      lines: [t('flow.queue.cancelBody', { n: waiting.length }), t('flow.queue.cancelBody2')],
      confirmText: t('flow.queue.cancelConfirm'),
      danger: true
    });
    if (!ok) return;
    // 不指名的 CancelTask＝撤掉全部等待中的行；撤掉几条由服务回的那个 count 说，不是界面自己数的行数
    const r = await api.cancelTask();
    if (!r) return;
    ui.toast(t('flow.queue.cancelled', { n: r.count }));
  });

  const clearDone = action(async () => {
    const before = state.queue.length;
    const snapshot = await api.clearTasks();
    if (!snapshot) return;
    ingest(snapshot);
    // 「清理了几条」＝用户点下去之前看见的那几行少了几行；服务留下的失败行本来就该还在，那是「重试失败的那几条」的原料
    ui.toast(t('flow.queue.cleared', { n: before - state.queue.length }));
  });

  /** 重试＝把这几条重新提交一次（契约里没有 RetryTask：16 个方法是冻的）。
      老的失败行留在表上，新的行从等待开始——那一行说的是上一次的事实，抹掉它才是撒谎。 */
  function failedApplyIDs() {
    const ids = [];
    for (const tk of state.queue) {
      if (tk.kind !== 'apply') continue;
      if (!['failed', 'rolledback'].includes(tk.status)) continue;
      if (tk.itemId && !ids.includes(tk.itemId)) ids.push(tk.itemId);
    }
    return ids;
  }

  const retryFailed = action(async () => {
    const ids = failedApplyIDs();
    if (!ids.length) { ui.toast(t('flow.queue.noRetry')); return; }
    if (!await sendApply(ids)) return;
    ui.toast(t('flow.queue.requeued', { n: ids.length }));
  });

  const retryOne = action(async (taskId) => {
    const tk = state.queue.find(x => x.id === taskId);
    if (!tk || tk.kind !== 'apply' || !tk.itemId) return;
    if (!await sendApply([tk.itemId])) return;
    ui.toast(t('flow.queue.requeuedOne', { name: tk.name }));
  });

  /* ---------- 同步状态 / 开机 ---------- */

  /** 「同步状态」＝再取一次此刻全貌：条目那几列本来就一直从文件读（D3 甲），这里要的就是那一份真值 */
  const syncStatuses = action(async (btnLabel) => {
    const before = new Map();
    for (const it of allItems()) before.set(it.id, it.status);
    const snapshot = await api.getSnapshot();
    if (snapshot) ingest(snapshot);
    let changed = 0;
    for (const it of allItems()) if (before.get(it.id) !== it.status) changed += 1;
    ui.renderTree(); ui.renderDetail(); ui.renderStatus();
    ui.toast(changed > 0 ? t('flow.sync.changed', { n: changed }) : t('flow.sync.clean'));
    if (btnLabel) btnLabel();
  });

  /** 开机那一下：接线 → 取全貌 → 补那三格环境读数（服务态／权限灯／版本号）。
      任何一问失败都只报一句人话，界面照常画已经拿到的那一部分——"服务没起来"与"这页不在壳里跑"都要看得见。 */
  async function start() {
    try {
      await api.attach();
      ingest(await api.getSnapshot());
      markLineUp();               // 开机这一问问到了回话：不等壳那一条 state 帧（它只在翻转时推，可能赶在订阅之前）
      await readEnvironment();
      paint();
      return true;
    } catch (err) {
      fail(err);
      return false;
    }
  }

  /** 快照里没有的那三格：写入锁此刻有没有人拿、这台机器能不能写 hosts、装的是哪一天的包。 */
  async function readEnvironment() {
    const st = await api.getStatus();
    state.writeLockHeld = !!st?.writeLockHeld;
    if (st?.service) {
      state.appStatus.dnsService = st.service.dnsService || t('common.dash');
      state.appStatus.dataDir = st.service.dataDir || '';
      state.appStatus.hostsPath = st.service.hostsPath || '';
    }
    const perm = await api.checkPermissions();
    state.adminGranted = !!perm?.canWrite;
    const v = await api.getVersion();
    state.update.current = v?.current || '';
    state.update.buildDate = v?.buildDate || '';
  }

  /** 线断了／重连了。断的时候把等待者一律结掉、把手松开——
      这时候服务那张表已经不可能会再更新了，界面继续"忙"就是把用户锁在屏幕前。
      忙标志本身由 snapshot.js 的 refreshBusy 按 state.connected 收：这里不另写第二套算法（C2 那笔账）。 */
  function onLink(link) {
    state.connected = !!link?.connected;
    if (state.connected) {
      resync();
      return;
    }
    state.pending.detect = false;
    state.pending.queue = false;
    refreshBusy(state);
    const err = new WireError('GAC-IPC-040', { endpoint: link?.endpoint || '' }, link?.err || '');
    // 顺序是定好的：先由这一处说一次「连不上」，再给等待者——它们拿到的是同一个 err，
    // 打上 announced 之后各自的 action() 只会补状态栏那一格，不会再各弹一句同样的话。
    fail(err);
    err.announced = true;
    for (const w of waiters.splice(0)) w.reject(err);
    ui.renderTree();
    ui.renderStatus();
  }

  /** 重连之后重新取一次全貌；取不到就再说一句（这一句不重复弹：start 里那一问失败会走 fail） */
  async function resync() {
    try {
      ingest(await api.getSnapshot());
      await readEnvironment();
      paint();
    } catch (err) {
      fail(err);
    }
  }

  /** 每一条事件都自带完整快照（契约那句"界面不许靠先收一半再猜另一半画界面"），所以五条走同一支处理函数。
      没带快照的那一条（通道给的形状不完整）只补画一次，不猜。 */
  function onEvent(event) {
    if (event && event.snapshot) {
      ingest(event.snapshot);
      return;
    }
    paint();
  }

  /* ---------- 启动自动化 ---------- */

  const runStartupAutomation = action(async () => {
    if (!state.settings.autoDetectOnStartup) return;
    state.startupAutomationRunning = true;
    ui.renderStatus();
    try {
      if (state.queue.some(tk => ACTIVE.includes(tk.status))) { ui.toast(t('flow.startup.allBusy')); return; }
      const initial = checkedItems();
      if (!initial.length) { ui.toast(t('flow.startup.noChecked')); return; }

      ui.toast(t('flow.startup.detect', { n: initial.length }));
      if (!await sendDetect(initial.map(i => i.id))) return;
      if (!state.settings.autoApplyOnStartup) return;

      const applicable = checkedItems().filter(i => canApply(i));
      if (!applicable.length) { ui.toast(t('flow.startup.nothingToApply')); return; }
      const activeIds = new Set(state.queue.filter(tk => ACTIVE.includes(tk.status)).map(tk => tk.itemId));
      const toApply = applicable.filter(i => !activeIds.has(i.id));
      if (!toApply.length) { ui.toast(t('flow.startup.alreadyQueued')); return; }

      ui.toast(t('flow.startup.apply', { n: toApply.length }));
      await sendApply(toApply.map(i => i.id));
    } catch (err) {
      // 这里把"连不上"也收进这一句：启动自动化没有那颗钮，没人点它，异常只能由这一句报出来
      ui.toast(t('flow.startup.crash', { msg: replyErrorText(t, err) }), 'error');
    } finally {
      state.startupAutomationRunning = false;
      ui.renderStatus();
    }
  });

  return {
    start, resync, onEvent, onLink, syncStatuses, runStartupAutomation,
    detectSingle, detectAll, applyQueue, applySingle,
    restoreItemHosts, restoreCategoryHosts, restoreTaskRow, restoreAll,
    pauseToggle, cancelWaiting, clearDone, retryFailed, retryOne
  };
}
