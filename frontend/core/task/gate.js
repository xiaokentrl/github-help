/* 「这会儿能不能点」的门，集中在一处。干什么用：工具栏三个大按钮、详情那三个小按钮、队列那四个操作、
   树上那两个↺、还有恢复流程，全部来问这里；按钮自己一律不判。
   会错在哪：总纲已定的 C2 就是判漏了——同一个"执行中不许点"在七八个地方各写一遍，漏一处就能在队列跑着的时候点下恢复，
             两个写 hosts 的动作同时开工。条件散在各处还会漂成两套：这边算 batchDetecting，那边只算 queueRunning。
   怎么回来：每个判断返回 { ok, key, params }——能不能点是代码，为什么不能点是文案键，
             渲染器只把 key 交给 t() 显示成灰掉那行的小字；条件本身在 tests/flows.test.mjs 里喂假状态测（那七句原话逐字比）。

   这里为什么有六支"忙"的键而不是三支：原型这七处挡路话逐字不同，判的先后也不同
   （:2275 检测那条是 启动→批量→队列，:2777 还原那条是 启动→队列→批量，:2846 应用那条队列只说「队列正在执行中」，
    :3178 「恢复默认 hosts」那三条结尾是「无法恢复」，:2323 检测全部那两条根本不出声）。
   合三支是省事，但合完界面上的话就变了——那是"私自改变其他功能"，所以按原型那七处各给一支。 */

/* 「这一条有没有可还原的」只认服务回的那一栏（store.js 的 canRestoreHosts＝Item.managed，D3 甲）。
   这里不另立判据，也不看界面自己记的 prevAppliedIp——那是同一个事实的第二个家。 */
import { canRestoreHosts } from '../store.js';

const no = (key, params) => ({ ok: false, key, params });
const yes = { ok: true, key: null };
const mute = { ok: false, key: null };   // 挡路但不出声（原型 detectAll 那两条 return null 前没有 toast）

/** 有任何一件事正在跑（批量检测 / 队列 / 启动自动化）——界面拿来灰掉一片按钮的那一条 */
export function isBusy(state) {
  return !!(state.batchDetecting || state.queueRunning || state.startupAutomationRunning);
}

/** 单项检测 / 单项应用前那三条（原型 :2275-2277、:2872-2873） */
export function busySingle(state) {
  if (state.startupAutomationRunning) return no('gate.busy.startup');
  if (state.batchDetecting) return no('gate.busy.detecting');
  if (state.queueRunning) return no('gate.busy.queue');
  return yes;
}

/** 「检测全部」：批量与启动那两条挡路但不出声，只有队列忙给一句话（原型 :2321-2323） */
export function busyBatchDetect(state) {
  if (state.batchDetecting || state.startupAutomationRunning) return mute;
  if (state.queueRunning) return no('gate.busy.queueDetect');
  return yes;
}

/** 「应用队列」（原型 :2846-2848，队列那一条结尾没有「请稍候」） */
export function busyApplyQueue(state) {
  if (state.startupAutomationRunning) return no('gate.busy.startup');
  if (state.queueRunning) return no('gate.busy.queueApply');
  if (state.batchDetecting) return no('gate.busy.detectWait');
  return yes;
}

/** 树上/详情里那两条还原（原型 :2775-2779、:2806-2808：启动→队列→批量） */
export function busyRestore(state) {
  if (state.startupAutomationRunning) return no('gate.busy.startup');
  if (state.queueRunning) return no('gate.busy.queue');
  if (state.batchDetecting) return no('gate.busy.detectWait');
  return yes;
}

/** 队列行里那颗「↺ 恢复到应用前」（原型 :2891 只看队列） */
export function busyRestoreTask(state) {
  if (state.queueRunning) return no('gate.busy.restoreTask');
  return yes;
}

/** 工具栏那颗「恢复」（原型 :3178-3180，三条结尾都是「无法恢复」） */
export function busyRestoreAll(state) {
  if (state.startupAutomationRunning) return no('gate.busy.restoreAllStartup');
  if (state.queueRunning) return no('gate.busy.restoreAllQueue');
  if (state.batchDetecting) return no('gate.busy.restoreAllDetecting');
  return yes;
}

/** 单项检测：三条忙之外还有三条"这一项自己占着位"（原型 :2274-2282 那六次 early return）。
   其中 :2278「这一项自己正在 detecting／applying」那条原型只 return 不出话，所以给 mute。
   「检测全部」那一条的挡路顺序跟这里不一样（忙→勾选→筛完全没了），所以那段判断留在 flows.detectAll 里按原型写。 */
export function detectSingle(state, item) {
  const busy = busySingle(state);
  if (!busy.ok) return busy;
  if (item && (item.status === 'detecting' || item.status === 'applying')) return mute;
  const pendingDetect = state.queue.some(t =>
    t.kind === 'detect' && t.itemId === item.id && (t.status === 'waiting' || t.status === 'running'));
  if (pendingDetect) return no('gate.detect.alreadyQueued');
  const runningApply = state.queue.some(t => t.kind === 'apply' && t.itemId === item.id && t.status === 'running');
  if (runningApply) return no('gate.apply.runningItem');
  return yes;
}

/** 「仅应用此项」：原型这里只挡启动与队列那两条，批量检测中照样能入队（:2872-2873） */
export function busyApplyOne(state) {
  if (state.startupAutomationRunning) return no('gate.busy.startup');
  if (state.queueRunning) return no('gate.busy.queue');
  return yes;
}

/** 详情面板「仅应用此项」的挡路说明——就是那条灰字 hint（原型 :1940、:2874） */
export function applyOne(state, item) {
  const busy = busyApplyOne(state);
  if (!busy.ok) return busy;
  if (!item.bestIp) return no('gate.apply.needDetect');
  return yes;
}

/** 树上与详情里那两条还原。判序照原型 :2774 与 :2805——先问"这一项／这一类有没有可还原的"，
   再问忙；反过来写会让「没有可还原的状态」被"队列正在执行中"盖掉。
   「有没有可还原的」只有一个凭据（store 的 canRestoreHosts＝服务回的那一栏 managed），这里不另判。 */
export function restoreItem(state, item) {
  if (!canRestoreHosts(item)) return no('gate.restore.nothing');
  return busyRestore(state);
}

export function restoreCategory(state, restorableCount) {
  if (restorableCount === 0) return no('gate.restore.noneInCat');
  return busyRestore(state);
}

export function restoreTask(state) {
  return busyRestoreTask(state);
}

/** 队列那四个按钮的禁用条件，一行算式一处（原型 :2071-2075 那四行散在渲染函数里） */
export function queueButtons(state) {
  const q = state.queue;
  const waiting = q.filter(t => t.status === 'waiting').length;
  const done = q.filter(t => t.status === 'done').length;
  const skipped = q.filter(t => t.status === 'skipped').length;
  const failed = q.filter(t => ['failed', 'rolledback'].includes(t.status)).length;
  return {
    pauseDisabled: !!state.batchDetecting || !(state.queueRunning || waiting > 0),
    cancelDisabled: waiting === 0,
    clearDisabled: (done + skipped) === 0,
    retryDisabled: failed === 0
  };
}

/** 写入串行锁的前端配合位：服务那边写 hosts 时（writeLockHeld）界面不再放新的写动作进去 */
export function writeLockHeld(state) {
  return !!state.writeLockHeld;
}
