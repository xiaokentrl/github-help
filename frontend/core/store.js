/* 全站唯一可变状态。干什么用：类目树、勾选、折叠、搜索词、队列、忙标志、设置、版本状态，只有这一份；
   界面（views/）读它、改它都走这里的函数，改完 notify() 一次，订阅者（渲染器）跟着补画。
   会错在哪：① 别处再 new 一个全局对象，就会出现"树勾了、状态栏说没勾"；
             ② 这里一个 document. 也不许出现——它一碰 DOM 就没法在 node 里跑单测（目录规划第 4 节那条 grep 门禁）。
   怎么回来：数据从哪儿来只有一个入口——core/api/snapshot.js 的 mergeSnapshot（真服务的快照），
             这里自己不做任何判断，只存；pending 那两只把手是界面唯一抬起的东西，松开规则在 refreshBusy。
             查询辅助（findItem / categoryCheckState / canApply…）全是纯函数，tests/store.test.mjs 直接喂对象测。 */

import { DEFAULTS } from './settings.js';

export const state = {
  categories: [],
  selectedId: null,
  filter: '',
  collapsed: new Set(),
  queue: [],
  lastDetect: '',
  queueCollapsed: false,
  bannerDismissed: false,
  adminGranted: false,

  batchDetecting: false,
  queueRunning: false,
  paused: false,
  startupAutomationRunning: false,
  writeLockHeld: false,
  /* 两只"发出去了、服务还没答话"的把手。干什么用：一次点击到那一次请求回话之间有一段，
     没有它，那一段里再点一次就是两次提交（两个写 hosts 的动作同时开工，正是 C2 那笔账）。
     抬起与松开都在 core/flows.js 的 dispatch 一处；core/api/snapshot.js 的 refreshBusy 只把它们与
     服务那张表合成 batchDetecting／queueRunning 这两个界面上用的数——这里存的不是"忙不忙"，是"问出去了没回话"。 */
  pending: { detect: false, queue: false },
  // 这条线与壳之间此刻通不通（gac:state 那一句给的唯一真相）。它只拿来判忙：线断了，服务那张表
  // 永远不会再更新，界面就不能继续说"正在执行中"——那是把用户锁在屏幕前面。
  connected: false,
  // 状态栏那一句"为什么"（契约 v9 的 lastErrorCode／lastErrorParams）。text 是服务侧原文，只给日志看，
  // 界面上说的是 err.<码> 那句——同一件事在一屏之内不许有两种说法（已定：D6 乙）。
  lastError: { code: '', params: {}, text: '' },

  detectOriginalCollapsed: null,
  detectCurrentCatId: null,
  applyOriginalCollapsed: null,
  applyCurrentCatId: null,

  // 这三栏全部由服务回，界面不猜（已定：C5）；空一律留空串，渲染那侧按 — 画
  appStatus: { dnsService: '—', dataDir: '', hostsPath: '' },
  // 断网判定（network:change 事件与每份快照都带）：状态栏第一格与气泡读它
  networkOffline: false,
  // current 与 buildDate 不在这里写死：开机那一下由 core/flows.js 的 start()→readEnvironment() 走契约 GetVersion 填进来。
  // progress 与 speed 只属于"正在下载"那一段，下载完了留着最后一次那两个数（原型 :1699-1706 就是这个表现）。
  // checkError 是最近一次「检查更新」服务回的那句失败（人话由 err.<码> 凑）。为什么单开一栏而不是塞进 log：
  // log 是"这一版更新了哪些内容"的清单，只在真有新版本那一段才画；查失败了把句子塞进去，那一格整张卡片会被藏起来
  // （卡片按 hasUpdate 显示），用户看到的是"尚未检查"——服务明明答过话。
  update: {
    current: '', buildDate: '', latest: '', hasUpdate: false,
    checking: false, downloading: false, downloaded: false,
    progress: 0, speed: '', log: [], checkError: ''
  },
  settings: { ...DEFAULTS }
};

const listeners = new Set();

export function subscribe(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

/** 状态改完之后由改它的那个人调一次；渲染器只在这里被叫醒，不去别处找钩子 */
export function notify() {
  for (const fn of Array.from(listeners)) fn(state);
}

export function resetState(next) {
  if (Array.isArray(next.categories)) state.categories = next.categories;
  if (Array.isArray(next.queue)) state.queue = next.queue;
  if (typeof next.selectedId === 'string' || next.selectedId === null) state.selectedId = next.selectedId;
  if (typeof next.lastDetect === 'string') state.lastDetect = next.lastDetect;
  if (next.appStatus) Object.assign(state.appStatus, next.appStatus);
  if (next.settings) Object.assign(state.settings, next.settings);
}

/* ============ 查询辅助（与原型 :1290-1349 同一套算法，一个 document 也不碰） ============ */

export function allItems() {
  return state.categories.flatMap(c => c.items);
}

export function findItem(id) {
  for (const c of state.categories) {
    const it = c.items.find(i => i.id === id);
    if (it) return it;
  }
  return null;
}

export function findCategory(id) {
  return state.categories.find(c => c.id === id) || null;
}

export function findCategoryOfItem(id) {
  return state.categories.find(c => c.items.some(i => i.id === id)) || null;
}

export function categoryCheckState(cat) {
  const total = cat.items.length;
  const checked = cat.items.filter(i => i.checked).length;
  if (checked === 0) return 'unchecked';
  if (checked === total) return 'checked';
  return 'indeterminate';
}

export function checkedItems() {
  return allItems().filter(i => i.checked);
}

export function canApply(item) {
  return !!item.bestIp;
}

/** ↺ 能不能点的唯一凭据：hosts 里有没有我们为这一条写下的行（契约 Item.managed，来源只有文件〔已定：D3 甲〕）。
    上一版认的是 item.prevAppliedIp 是不是 undefined，那是界面自己入队时记的一笔账——
    服务侧那个字段是字符串、omitempty，空串装着两件事（"不是我们写的"与"写之前本来没有"），分不开就点不动。 */
export function canRestoreHosts(item) {
  return !!item?.managed;
}

export function hasRestorableInCategory(cat) {
  return cat.items.some(it => canRestoreHosts(it));
}

export function allCheckedState() {
  const items = allItems();
  if (!items.length) return false;
  return items.every(i => i.checked);
}

export function allCollapsedState() {
  if (!state.categories.length) return false;
  return state.categories.every(c => state.collapsed.has(c.id));
}

export function setAllChecked(val) {
  allItems().forEach(i => { i.checked = val; });
}

export function invertAll() {
  allItems().forEach(i => { i.checked = !i.checked; });
}

export function toggleCategoryCollapsed(catId) {
  if (state.collapsed.has(catId)) state.collapsed.delete(catId);
  else state.collapsed.add(catId);
}

/** 检测/应用跑完之后回到跑之前的折叠状态（原型的 focusFollow 那一套） */
export function snapshotCollapsed() {
  return new Set(state.collapsed);
}

export function restoreCollapsed(snap) {
  state.collapsed.clear();
  if (snap) snap.forEach(id => state.collapsed.add(id));
}

export function collapseOnly(catId) {
  state.categories.forEach(c => {
    if (c.id === catId) state.collapsed.delete(c.id);
    else state.collapsed.add(c.id);
  });
}

/** 服务回来的整棵树：按 id 接住界面上正在看的这条，别让一次同步把选中态弄丢 */
export function replaceCategories(categories) {
  const prevSelected = state.selectedId;
  state.categories = categories;
  if (prevSelected && findItem(prevSelected)) state.selectedId = prevSelected;
  else if (!prevSelected) state.selectedId = null;
}
