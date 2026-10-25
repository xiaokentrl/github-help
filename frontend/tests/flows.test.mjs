/* core 这一层的机器判据：门（gate）判得对不对、六条流程的形状对不对、断线那一支走不走得通。
   干什么用：用假 ui（记账用的回调）＋假 api（想让它回什么就回什么）＋自己手推的事件，在 node 里把每条流程跑一遍，
             一个 DOM 也不碰——这一段一旦搬进 views 就再也测不了，所以它的测试必须留在 node 这边。
   从这一版起形状跟着真服务改：一次点击＝一个请求，假 api 只回任务号（契约那四个异步方法回的就是 {taskID}），
             跑出来的每一行由用例自己用 flows.onEvent({ name, snapshot }) 推整份快照进来。
             没有 await 到跑完那种事——真服务不这么说话，测试也不能假装它这么说话。
   会错在哪：① 断言写"跑完没报错"没用，要看的是那几句用户能看见的话、条目最后那几栏、以及那两只把手松没松；
             ② 这一份和 tests/i18n.test.mjs 是一对：这里用真键表（zh-CN）拼话，
                所以键名写错不会静默显示成 ⟦…⟧，会直接对不上断言；
             ③ 用例不许在推帧之前先改条目字段来"帮它一把"——界面上那一栏只能由快照改出来。
                自己写进去的断言就测不到"界面偷偷改状态"那一类回归（flows.js 文件头 ①）。
   怎么回来：红的时候看断言消息——它比的是原型的原话（行号在 zh-CN.js 每条后面），
             改这句话要连原型行号一起改，别只改测试让它变绿。 */

import { test, beforeEach } from 'node:test';
import assert from 'node:assert/strict';

import { state, resetState, findItem } from '../core/store.js';
import { DEFAULTS } from '../core/settings.js';
import { METHODS } from '../core/contract/methods.js';
import { createFlows } from '../core/flows.js';
import * as gate from '../core/task/gate.js';
import { api } from '../core/api/client.js';
import { WireError } from '../core/api/wire.js';
import { clockFromIso } from '../utils/format.js';
import { createI18n } from '../core/i18n/t.js';
import zhCN from '../core/i18n/locales/zh-CN.js';

const t = createI18n({ 'zh-CN': zhCN });

function fakeItem(id, extra = {}) {
  return {
    id, name: `条目${id}`, checked: true, status: 'unset',
    bestIp: null, appliedIp: null, prevAppliedIp: undefined, managed: false,
    latency: 0, speed: '', score: 0, source: '', sourceRole: '', lastCheck: '', errorMsg: '',
    candidates: [], ...extra
  };
}

/** 每个用例一份干净状态：两条类目、四条条目（a 没检测过、b 已检测、c 托管着但应用前没记录、d 已应用） */
function freshState() {
  resetState({
    categories: [
      { id: 'c1', name: '类目一', items: [fakeItem('a'), fakeItem('b', { bestIp: '1.1.1.1', status: 'detected' })] },
      { id: 'c2', name: '类目二', items: [
        fakeItem('c', { managed: true, appliedIp: '3.3.3.3', prevAppliedIp: null }),
        fakeItem('d', { appliedIp: '2.2.2.2', bestIp: '2.2.2.2', status: 'applied', prevAppliedIp: '1.2.3.4', managed: true })
      ] }
    ],
    queue: [], selectedId: null, lastDetect: ''
  });
  state.collapsed.clear();
  Object.assign(state, {
    batchDetecting: false, queueRunning: false, paused: false, startupAutomationRunning: false,
    writeLockHeld: false, adminGranted: false,
    // 用例默认演的是"线是通的"那一段；断线那一支自己关掉它
    connected: true,
    lastError: { code: '', params: {}, text: '' }
  });
  state.pending.detect = false;
  state.pending.queue = false;
  state.settings = { ...DEFAULTS };
  state.update.current = '';
  state.update.buildDate = '';
}

/** 报错那一句加了标记：用例要分得开"提示"与"报错"，弹的话语义不同不能混着比 */
function makeUi({ confirmAnswer = true } = {}) {
  const calls = { toasts: [], confirms: [], scrolls: [], renders: 0 };
  return {
    calls,
    ui: {
      renderTree: () => { calls.renders++; },
      renderDetail: () => { calls.renders++; },
      renderStatus: () => { calls.renders++; },
      scrollToInt: id => calls.scrolls.push(id),
      confirm: opts => { calls.confirms.push(opts); return Promise.resolve(confirmAnswer); },
      toast: (text, kind) => calls.toasts.push(kind === 'error' ? `⟪错⟫${text}` : text)
    }
  };
}

/** 把 api 那些动词换成用例里想要的行为，用完还回去 */
function stubApi(stubs) {
  const real = {};
  for (const [key, value] of Object.entries(stubs)) {
    real[key] = api[key];
    api[key] = value;
  }
  return () => Object.assign(api, real);
}

/* ---- 服务那三份形状（contract.Item／Task／Snapshot 的 JSON 名，一个字段也不自己发明） ---- */

function srvItem(id, over = {}) {
  return {
    id, name: `条目${id}`, status: 'unset',
    appliedIp: '', prevAppliedIp: '', bestIp: '', managed: false,
    source: '', sourceRole: '', latency: 0, lastCheck: '',
    errorCode: '', errorParams: null, ...over
  };
}

function srvTask(id, kind, itemId, status, over = {}) {
  return {
    id, kind, itemId, name: itemId ? `类目 / 条目${itemId}` : '整份 hosts', status,
    phase: '', processed: 0, total: 0, durationMs: 0, ...over
  };
}

/** 一份完整快照：事件带的那一份与 GetSnapshot／ClearTasks 的回执用的是同一个形状 */
function table({ items = [], tasks = [], state: st = { queuePaused: false } } = {}) {
  return { state: st, items, tasks };
}

/** 一拍：把 dispatch 里那一次 await 与随后登记的等待者都放行（微任务在定时器之前先清空） */
const tick = () => new Promise(r => setTimeout(r, 0));

/** 起一条流程：先把那颗钮按下去（不等它），再按顺序把服务那几帧推给它，最后等它自己收工。 */
async function run(flows, kick, ...frames) {
  const done = kick();
  await tick();
  for (const snap of frames) {
    flows.onEvent({ name: 'queue:progress', snapshot: snap });
    await tick();
  }
  return done;
}

/** 只推一帧（用例常常要先让服务那张表变成用例要的样子） */
async function push(flows, snap) {
  flows.onEvent({ name: 'queue:progress', snapshot: snap });
  await tick();
}

function allUncheck() {
  for (const it of state.categories.flatMap(c => c.items)) it.checked = false;
}

beforeEach(freshState);

/* ================= 门：七处挡路话各是各的 ================= */

test('门：同一个"忙"在七处说的是七句原话，判的先后也照原型', () => {
  const mk = (over = {}) => ({ queue: [], batchDetecting: false, queueRunning: false, startupAutomationRunning: false, ...over });

  assert.equal(gate.busySingle(mk({ startupAutomationRunning: true })).key, 'gate.busy.startup');
  assert.equal(gate.busySingle(mk({ batchDetecting: true })).key, 'gate.busy.detecting');
  assert.equal(gate.busySingle(mk({ queueRunning: true })).key, 'gate.busy.queue');

  // 「检测全部」：批量与启动挡路但不出声（原型 :2321-2322 前面没有 toast）
  assert.deepEqual(gate.busyBatchDetect(mk({ batchDetecting: true })), { ok: false, key: null });
  assert.deepEqual(gate.busyBatchDetect(mk({ startupAutomationRunning: true })), { ok: false, key: null });
  assert.equal(gate.busyBatchDetect(mk({ queueRunning: true })).key, 'gate.busy.queueDetect');

  // 「应用队列」撞见队列时结尾没有「请稍候」（:2847），撞见批量时是另一句（:2848）；单项那句又是第三句
  assert.equal(gate.busyApplyQueue(mk({ queueRunning: true })).key, 'gate.busy.queueApply');
  assert.equal(gate.busyApplyQueue(mk({ batchDetecting: true })).key, 'gate.busy.detectWait');
  assert.equal(gate.busyApplyOne(mk({ queueRunning: true })).key, 'gate.busy.queue');

  // 还原那条判序是 启动→队列→批量，跟上面几条不一样
  assert.equal(gate.busyRestore(mk({ queueRunning: true })).key, 'gate.busy.queue');
  assert.equal(gate.busyRestoreTask(mk({ queueRunning: true })).key, 'gate.busy.restoreTask');

  // 工具栏「恢复」三条结尾是「无法恢复」，不能被上面那些「请稍候」盖掉
  assert.equal(gate.busyRestoreAll(mk({ startupAutomationRunning: true })).key, 'gate.busy.restoreAllStartup');
  assert.equal(gate.busyRestoreAll(mk({ queueRunning: true })).key, 'gate.busy.restoreAllQueue');
  assert.equal(gate.busyRestoreAll(mk({ batchDetecting: true })).key, 'gate.busy.restoreAllDetecting');
});

test('门：单项检测里"这一项自己正在跑"那条挡路但不出声（原型 :2278）', () => {
  const s = { queue: [], batchDetecting: false, queueRunning: false, startupAutomationRunning: false };
  assert.deepEqual(gate.detectSingle(s, fakeItem('a', { status: 'detecting' })), { ok: false, key: null });
  assert.equal(gate.detectSingle(s, fakeItem('a')).ok, true);
});

test('门：还原先问"有没有可还原的"再问忙——反过来会把原话盖掉', () => {
  const busy = { queue: [], batchDetecting: false, queueRunning: true, startupAutomationRunning: false };
  assert.equal(gate.restoreItem(busy, fakeItem('a')).key, 'gate.restore.nothing', '没托管行 → 说没记录');
  assert.equal(gate.restoreItem(busy, fakeItem('a', { prevAppliedIp: null })).key, 'gate.restore.nothing',
    'prevAppliedIp 不再是凭据：那是界面自己记的一笔账，服务那一栏才说实话（D3 甲）');
  assert.equal(gate.restoreItem(busy, fakeItem('a', { managed: true })).key, 'gate.busy.queue');
  assert.equal(gate.restoreCategory(busy, 0).key, 'gate.restore.noneInCat');
  assert.equal(gate.restoreCategory(busy, 2).key, 'gate.busy.queue');
});

test('门：「仅应用此项」在批量检测中照样放行（原型 :2872-2873 只挡启动与队列）', () => {
  const s = { queue: [], batchDetecting: true, queueRunning: false, startupAutomationRunning: false };
  assert.equal(gate.busyApplyOne(s).ok, true);
  assert.equal(gate.applyOne(s, fakeItem('a')).key, 'gate.apply.needDetect');
  assert.equal(gate.applyOne(s, fakeItem('a', { bestIp: '1.1.1.1' })).ok, true);
});

test('门：队列那四颗钮的禁用条件（原型 :2071-2075）', () => {
  const q = (status, kind = 'apply') => ({ status, kind });
  const s = { queue: [], batchDetecting: false, queueRunning: false };
  assert.deepEqual(gate.queueButtons(s), { pauseDisabled: true, cancelDisabled: true, clearDisabled: true, retryDisabled: true });

  s.queue = [q('waiting')];
  assert.equal(gate.queueButtons(s).cancelDisabled, false);
  assert.equal(gate.queueButtons(s).pauseDisabled, false);   // 有等待中的就能按暂停（哪怕队列还没跑）

  s.queue = [q('done'), q('skipped')];
  assert.equal(gate.queueButtons(s).clearDisabled, false);
  assert.equal(gate.queueButtons(s).cancelDisabled, true);

  s.queue = [q('failed'), q('rolledback')];
  assert.equal(gate.queueButtons(s).retryDisabled, false);
});

/* ================= 检测 ================= */

test('流程：单项检测只发一次请求，条目那几栏全由事件带的快照落进来', async () => {
  const seen = [];
  const restore = stubApi({ detect: ids => { seen.push(ids.slice()); return { taskID: 'batch-1' }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const a = findItem('a');
    await run(flows, () => flows.detectSingle(a),
      table({ items: [srvItem('a', { status: 'detecting' })], tasks: [srvTask('task-1', 'detect', 'a', 'running')] }),
      table({
        items: [srvItem('a', { status: 'detected', bestIp: '9.9.9.9', latency: 12, source: 'DoH 优选', lastCheck: '2026-10-21T10:00:00Z' })],
        tasks: [srvTask('task-1', 'detect', 'a', 'done', { durationMs: 1200 })]
      }));

    assert.deepEqual(seen, [['a']], '一次点击＝一个请求，参数就是那一条的 id');
    assert.equal(a.bestIp, '9.9.9.9');
    assert.equal(a.status, 'detected');
    assert.equal(state.lastDetect, clockFromIso('2026-10-21T10:00:00Z'), '状态栏那句「上次检测」取的是条目里最近那一次');
    assert.deepEqual(calls.toasts, [], '单项检测成功不弹话——那一行落在条目上');
    assert.equal(state.pending.detect, false);
    assert.equal(state.batchDetecting, false);
  } finally { restore(); }
});

test('流程：请求发出去到服务答话那一段里界面就是忙的——第二次点击真的发不出去（C2）', async () => {
  let answer;
  const seen = [];
  const restore = stubApi({ detect: ids => { seen.push(ids.slice()); return new Promise(r => { answer = r; }); } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const a = findItem('a');
    const first = flows.detectSingle(a);
    await tick();

    assert.equal(state.pending.detect, true);
    assert.equal(state.batchDetecting, true,
      '把手抬起来的那一刻就要重算忙标志：门问的是 batchDetecting 那一个数，不是把手本身');
    assert.equal(gate.busySingle(state).key, 'gate.busy.detecting');

    await flows.detectAll();
    assert.deepEqual(seen, [['a']], '那一段里再点一次没有变成第二个检测请求');
    assert.deepEqual(calls.toasts, [], '「检测全部」撞见批量忙时挡路但不出声（原型 :2321）');

    answer({ taskID: 'batch-1' });
    await push(flows, table({ items: [srvItem('a', { status: 'detecting' })], tasks: [srvTask('task-1', 'detect', 'a', 'running')] }));
    assert.equal(state.pending.detect, false, '答过话了就松手');
    assert.equal(state.batchDetecting, true, '松手之后那张表里还有活动行，界面照样报忙');

    await push(flows, table({ items: [srvItem('a', { status: 'detected', bestIp: '9.9.9.9' })], tasks: [srvTask('task-1', 'detect', 'a', 'done')] }));
    await first;
    assert.equal(state.batchDetecting, false);
  } finally { restore(); }
});

test('流程：开机那一问拿到回执＝这条线是通的——不等壳那一条 state 帧，把手照样拦得住第二次点击', async () => {
  let answer;
  const seen = [];
  const restore = stubApi({
    attach: async () => ({}),
    getSnapshot: async () => table({ items: [srvItem('a')], tasks: [] }),
    getStatus: async () => ({ service: {}, writeLockHeld: false }),
    checkPermissions: async () => ({ canWrite: true }),
    getVersion: async () => ({ current: '0.3.0' }),
    detect: ids => { seen.push(ids.slice()); return new Promise(r => { answer = r; }); }
  });
  const { ui } = makeUi();
  try {
    state.connected = false;   // 壳那句 connected:true 赶在页面订阅之前推完了（bridge.go 只在翻转时推），这一帧没收到
    const flows = createFlows({ t, ui });

    assert.equal(await flows.start(), true);
    assert.equal(state.connected, true, '第一手依据是"这一问有没有回话"；那个初值一直留着，下面那一档就永远算不出忙');

    const first = flows.detectAll();
    await tick();
    assert.equal(state.batchDetecting, true, '把手抬起来的那一刻忙标志要算得出来');
    await flows.detectAll();
    assert.equal(seen.length, 1, '第二次点击没变成第二个检测请求');

    answer({ taskID: 'batch-1' });
    await tick();   // 先让 dispatch 松手、把"回话在第几帧"记上，再推这一批的最后一帧（否则这一帧正好被算成"回话之前那一帧"）
    await push(flows, table({
      items: [srvItem('a', { status: 'detected', bestIp: '9.9.9.9' })],
      tasks: [srvTask('task-1', 'detect', 'a', 'done')]
    }));
    await first;
    assert.equal(state.batchDetecting, false);
  } finally { restore(); }
});

test('流程：批量检测把勾选的四条一次发出去，跑完给那一句汇总、折叠回到跑之前的样子', async () => {
  const seen = [];
  const restore = stubApi({ detect: ids => { seen.push(ids.slice()); return { taskID: 'batch-1' }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    state.settings.detectFocusFollow = true;
    state.collapsed.add('c1');

    const running = ['a', 'b', 'c', 'd'].map(id => srvTask(`task-${id}`, 'detect', id, id === 'a' ? 'running' : 'waiting'));
    const final = table({
      items: [
        srvItem('a', { status: 'detected', bestIp: '9.9.9.1', lastCheck: '2026-10-21T10:00:00Z' }),
        srvItem('b', { status: 'applied', bestIp: '1.1.1.1', appliedIp: '1.1.1.1' }),
        srvItem('c', { status: 'stale', bestIp: '3.3.3.3' }),
        srvItem('d', { status: 'error', errorCode: 'GAC-DETECT-001' })
      ],
      tasks: ['a', 'b', 'c', 'd'].map(id => srvTask(`task-${id}`, 'detect', id, 'done'))
    });
    const targets = await run(flows, () => flows.detectAll(), table({ tasks: running }), final);

    assert.deepEqual(seen, [['a', 'b', 'c', 'd']], '一次请求带整批 id，不是四条各发一次');
    assert.deepEqual(targets.map(i => i.id), ['a', 'b', 'c', 'd']);
    assert.deepEqual(calls.toasts, ['开始检测 4 项…', '检测完成：待应用 1 · 已是最优 1 · 待更新 1 · 失败 1'],
      '零的那一档不进句子（原型 :2422-2431）');
    assert.deepEqual(calls.scrolls, ['a'], '每一条任务只滚一次，滚到正在跑那一条');
    assert.deepEqual([...state.collapsed], ['c1'], '跑完回到跑之前：c1 还是收起的');
    assert.equal(state.batchDetecting, false);
  } finally { restore(); }
});

test('流程：整批在跑完之前就被取消，条目一条也没变时，那一句汇总说的是「无变化」', async () => {
  const restore = stubApi({ detect: () => ({ taskID: 'batch-1' }) });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const four = ['a', 'b', 'c', 'd'].map(id => srvTask(`task-${id}`, 'detect', id, 'waiting'));
    const cancelled = four.map(tk => ({ ...tk, status: 'cancelled' }));
    await run(flows, () => flows.detectAll(),
      table({ tasks: four }),
      // 四条全撤了：条目回到"没测过"那一档，四个统计桶（待应用／已是最优／待更新／失败）一个都没进
      table({ items: [srvItem('a'), srvItem('b'), srvItem('c'), srvItem('d')], tasks: cancelled }));
    assert.equal(calls.toasts[1], '检测完成：无变化');
  } finally { restore(); }
});

test('流程：一句都没勾选时批量检测给的是「请先勾选要检测的项」，一个请求也不发', async () => {
  const seen = [];
  const restore = stubApi({ detect: ids => { seen.push(ids); return { taskID: 'batch-1' }; } });
  const { ui, calls } = makeUi();
  try {
    allUncheck();
    const flows = createFlows({ t, ui });
    assert.deepEqual(await flows.detectAll(), []);
    assert.deepEqual(calls.toasts, ['请先勾选要检测的项']);
    assert.deepEqual(seen, []);
  } finally { restore(); }
});

test('流程：那一次请求当场回不来（服务回一句失败）就地松手，并说语言文件凑的那一句', async () => {
  const restore = stubApi({ detect: async () => { throw new WireError('GAC-HOSTS-001', { path: '/etc/hosts' }, '读不到 hosts'); } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await flows.detectSingle(findItem('a'));
    assert.equal(state.pending.detect, false);
    assert.equal(state.batchDetecting, false, '请求没成，界面不能一直报忙');
    assert.deepEqual(calls.toasts, ['⟪错⟫读不到 hosts 文件（/etc/hosts）：确认它还在不在、当前身份能不能读']);
    assert.equal(state.lastError.code, 'GAC-HOSTS-001');
  } finally { restore(); }
});

/* ================= 应用 ================= */

test('流程：应用队列先问那两行，检测交给服务自己补，这里只发一次 ApplyQueue', async () => {
  const seen = { detect: [], apply: [] };
  const restore = stubApi({
    detect: ids => { seen.detect.push(ids.slice()); return { taskID: 'batch-1' }; },
    applyQueue: ids => { seen.apply.push(ids.slice()); return { taskID: 'batch-2' }; }
  });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await flows.applyQueue();

    assert.equal(calls.confirms.length, 1, 'a 与 c 没检测过，要先问一句');
    assert.deepEqual(calls.confirms[0].lines, ['检测是应用的必经步骤。', '是否先自动检测这 2 项，再统一应用？'],
      '原型 :2857 那句用 <br> 断成两行，这里就是两行');
    assert.equal(calls.confirms[0].confirmText, '先检测再应用');
    assert.deepEqual(seen.detect, [], '不再单开一轮检测：applyrun.go ① 在任务里自己补，两边各跑一遍就是同一件事两个家');
    assert.deepEqual(seen.apply, [['a', 'b', 'c', 'd']]);
    assert.deepEqual(calls.toasts, ['已加入队列 4 项']);
    assert.equal(state.pending.queue, false, '服务答过话就松手，忙不忙改由那张表说');
  } finally { restore(); }
});

test('流程：那一句问了但点了"不去"，一个请求也不发', async () => {
  const seen = [];
  const restore = stubApi({ applyQueue: ids => { seen.push(ids.slice()); return { taskID: 'batch-2' }; } });
  const { ui, calls } = makeUi({ confirmAnswer: false });
  try {
    const flows = createFlows({ t, ui });
    await flows.applyQueue();
    assert.deepEqual(seen, []);
    assert.deepEqual(calls.toasts, []);
  } finally { restore(); }
});

test('流程：这一条自己的检测行还在表上时，「仅应用此项」说的是「该项已在队列中」', async () => {
  const seen = [];
  const restore = stubApi({ applyQueue: ids => { seen.push(ids.slice()); return { taskID: 'batch-2' }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    // 服务那张表里：a 有一条检测行在跑（batchDetecting 为真、queueRunning 为假——「仅应用此项」那道门放行）
    await push(flows, table({ items: [srvItem('a', { status: 'detected', bestIp: '9.9.9.9' })], tasks: [srvTask('task-1', 'detect', 'a', 'running')] }));
    assert.equal(state.batchDetecting, true);
    assert.equal(state.queueRunning, false);

    await flows.applySingle(findItem('a'));
    assert.deepEqual(seen, [], '一个应用请求都不该发');
    assert.deepEqual(calls.toasts, ['该项已在队列中'], '原型 :2880 那句，跟整批那句「所选项目已在队列中」不是一句');
  } finally { restore(); }
});

test('流程：入队那一句只说一次，条目那一栏等服务回（界面不自己把 bestIp 抄进 appliedIp）', async () => {
  const restore = stubApi({ applyQueue: () => ({ taskID: 'batch-2' }) });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const b = findItem('b');
    await flows.applySingle(b);
    assert.equal(b.appliedIp, null, '写没写进去只有服务和那一份 hosts 文件知道');
    assert.deepEqual(calls.toasts, ['已加入队列：条目b'], '原型 :2879 用的是条目名，不是「类目 / 条目」那串');

    await push(flows, table({ items: [srvItem('b', { status: 'applied', bestIp: '1.1.1.1', appliedIp: '1.1.1.1', managed: true })], tasks: [srvTask('task-5', 'apply', 'b', 'done')] }));
    assert.equal(b.appliedIp, '1.1.1.1');
    assert.equal(b.managed, true);
  } finally { restore(); }
});

/* ================= 还原 / 恢复 ================= */

test('流程：↺ 一条——确认框那三行照原型，RestoreItem 一次一条，那几栏等服务回', async () => {
  const seen = [];
  const restore = stubApi({ restoreItem: id => { seen.push(id); return { taskID: 'batch-3' }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const d = findItem('d');
    await run(flows, () => flows.restoreItemHosts(d),
      table({ items: [srvItem('d', { status: 'applied', managed: true, appliedIp: '2.2.2.2', prevAppliedIp: '1.2.3.4' })], tasks: [srvTask('task-9', 'restore', 'd', 'running')] }),
      table({ items: [srvItem('d', { status: 'detected', managed: false, appliedIp: '1.1.1.1', prevAppliedIp: '' })], tasks: [srvTask('task-9', 'restore', 'd', 'done')] }));

    assert.deepEqual(seen, ['d'], '契约 RestoreItem 一次只收一条');
    const box = calls.confirms[0];
    assert.equal(box.title, '还原 Hosts');
    assert.deepEqual(box.lines, ['将把 条目d 的 hosts 记录还原到本次应用前的状态。', '应用前 IP 1.2.3.4', '当前写入 IP 2.2.2.2']);
    assert.equal(box.warn, '还原后该项回到应用前的状态，还原按钮消失。');
    assert.equal(box.confirmText, '确认还原');
    assert.equal(d.managed, false, '还原凭据只认服务回的那一栏');
    assert.equal(d.prevAppliedIp, null);
    assert.equal(d.appliedIp, '1.1.1.1', '界面不自己清 appliedIp：那一栏是"文件里此刻写着什么"');
    assert.deepEqual(calls.toasts, [], '成功那一下不弹话——树那一行自己就变回"没托管"了');
  } finally { restore(); }
});

test('流程：应用前本来没有这条，确认框换成「相当于删除该项」那句', async () => {
  const restore = stubApi({ restoreItem: () => ({ taskID: 'batch-3' }) });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const c = findItem('c');       // managed 为真、prevAppliedIp 为 null 的那一条
    await run(flows, () => flows.restoreItemHosts(c),
      table({ items: [srvItem('c', { managed: false, prevAppliedIp: '', appliedIp: '' })], tasks: [srvTask('task-10', 'restore', 'c', 'done')] }));
    assert.equal(calls.confirms[0].warn, '应用前 hosts 无记录，还原后相当于删除该项。');
    assert.equal(calls.confirms[0].lines[1], '应用前 IP （无记录）');
    assert.equal(c.status, 'unset');
  } finally { restore(); }
});

test('流程：还原一类按条数逐条发，成功几条看 managed，三种结局三句话', async () => {
  const seen = [];
  const restore = stubApi({ restoreItem: id => { seen.push(id); return { taskID: 'batch-4' }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const c2 = state.categories[1];
    const running = [srvTask('task-11', 'restore', 'c', 'running'), srvTask('task-12', 'restore', 'd', 'waiting')];
    await run(flows, () => flows.restoreCategoryHosts(c2),
      table({ tasks: running }),
      table({
        // c 撤干净了（managed 假），d 那一行没撤动（managed 真）
        items: [srvItem('c', { status: 'unset', managed: false }), srvItem('d', { status: 'applied', managed: true, appliedIp: '2.2.2.2', bestIp: '2.2.2.2' })],
        tasks: [srvTask('task-11', 'restore', 'c', 'done'), srvTask('task-12', 'restore', 'd', 'failed')]
      }));

    assert.deepEqual(seen, ['c', 'd'], '两条＝两次 RestoreItem，契约没有"一类"那个方法');
    const box = calls.confirms[0];
    assert.equal(box.title, '还原「类目二」下的 Hosts');
    assert.deepEqual(box.lines, [
      '将把 2 项还原到应用前状态。',
      '还原后这些项的「还原Hosts」按钮会消失。',
      '可还原项：',
      '条目c：（无记录） ← 3.3.3.3',
      '条目d：1.2.3.4 ← 2.2.2.2'
    ], '原型 :2812-2819 就是这个顺序，第三行是「可还原项：」');
    assert.deepEqual(calls.toasts, ['「类目二」下 1 项成功，1 项失败']);
  } finally { restore(); }
});

test('流程：还原一类全失败说第二句——凭的是快照里那一栏，不是界面自己记的账', async () => {
  const seen = [];
  const restore = stubApi({ restoreItem: id => { seen.push(id); return { taskID: 'batch-4' }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const c2 = state.categories[1];
    await run(flows, () => flows.restoreCategoryHosts(c2),
      table({ tasks: [srvTask('t1', 'restore', 'c', 'done'), srvTask('t2', 'restore', 'd', 'done')] }),
      table({
        items: [srvItem('c', { managed: true, appliedIp: '3.3.3.3' }), srvItem('d', { managed: true, appliedIp: '2.2.2.2' })],
        tasks: [srvTask('t1', 'restore', 'c', 'failed'), srvTask('t2', 'restore', 'd', 'failed')]
      }));
    assert.equal(seen.length, 2);
    assert.deepEqual(calls.toasts, ['「类目二」下全部 2 项还原失败']);
  } finally { restore(); }
});

test('流程：恢复默认 hosts 的确认框那四段字照原型，等服务那张表排空才说完成', async () => {
  let n = 0;
  const restore = stubApi({ restoreHosts: () => { n++; return { taskID: 'batch-9' }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await run(flows, () => flows.restoreAll(),
      table({ tasks: [srvTask('t20', 'restore', 'd', 'running')] }),
      table({ items: [srvItem('a'), srvItem('b', { bestIp: '1.1.1.1' }), srvItem('c'), srvItem('d')], tasks: [srvTask('t20', 'restore', 'd', 'done')] }));

    assert.equal(n, 1, '无参数那一次调用只发一次，展开成"每一条一行"是服务的事');
    const box = calls.confirms[0];
    assert.equal(box.title, '恢复默认 hosts');
    assert.deepEqual(box.lines, ['将执行以下操作：', '• 移除所有由本工具写入的域名映射', '• 恢复修改前的 hosts 备份', '• 清空当前队列']);
    assert.equal(box.warn, '此操作不可撤销。');
    assert.equal(findItem('d').managed, false, '回到未替换是服务从那一份文件里数出来的');
    assert.equal(findItem('d').status, 'unset');
    assert.deepEqual(calls.toasts, ['已恢复默认 hosts，所有项回到「未替换」']);
    assert.equal(state.queueRunning, false);
  } finally { restore(); }
});

test('流程：队列行那颗「↺ 恢复到应用前」跑完给一句、没跑成给另一句，两种都不自己改那一行的状态', async () => {
  const restore = stubApi({ restoreItem: () => ({ taskID: 'batch-5' }) });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const appliedRow = srvTask('t7', 'apply', 'd', 'done', { name: '类目二 / 条目d' });
    await push(flows, table({
      items: [srvItem('d', { status: 'applied', managed: true, appliedIp: '2.2.2.2', prevAppliedIp: '1.2.3.4', bestIp: '2.2.2.2' })],
      tasks: [appliedRow]
    }));
    assert.equal(state.queue[0].name, '类目二 / 条目d', '那一行显示的名字跟着快照走，不由界面拿 itemId 去别处查');

    await run(flows, () => flows.restoreTaskRow('t7'),
      table({ items: [srvItem('d', { managed: true, appliedIp: '2.2.2.2', prevAppliedIp: '1.2.3.4' })], tasks: [appliedRow, srvTask('t8', 'restore', 'd', 'running')] }),
      table({ items: [srvItem('d', { status: 'detected', managed: false, appliedIp: '1.1.1.1' })], tasks: [appliedRow, srvTask('t8', 'restore', 'd', 'done')] }));
    assert.deepEqual(calls.toasts, ['类目二 / 条目d 已恢复']);
    assert.equal(state.queue[0].status, 'done', '原来那一行还是服务说的那个状态，界面不自己标"已恢复"');

    calls.toasts.length = 0;
    const row9 = srvTask('t9', 'apply', 'd', 'done', { name: '类目二 / 条目d' });
    await push(flows, table({ items: [srvItem('d', { status: 'applied', managed: true, appliedIp: '2.2.2.2', prevAppliedIp: '1.2.3.4' })], tasks: [row9] }));
    await run(flows, () => flows.restoreTaskRow('t9'),
      table({ items: [srvItem('d', { managed: true, appliedIp: '2.2.2.2' })], tasks: [row9, srvTask('t10', 'restore', 'd', 'failed', { errorCode: 'GAC-HOSTS-010' })] }));
    assert.deepEqual(calls.toasts, ['⟪错⟫恢复失败，请查看队列错误信息']);
  } finally { restore(); }
});

test('流程：已经没托管行的那一条再点那颗钮，只补一句并且不动那一行', async () => {
  const seen = [];
  const restore = stubApi({ restoreItem: id => { seen.push(id); return { taskID: 'x' }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await push(flows, table({ items: [srvItem('a', { managed: false })], tasks: [srvTask('t9', 'apply', 'a', 'done', { name: '类目一 / 条目a' })] }));
    await flows.restoreTaskRow('t9');
    assert.deepEqual(calls.toasts, ['该项已还原，无需重复操作']);
    assert.deepEqual(seen, [], '文件里已经没有我们那几行，就一次也不发');
    assert.equal(state.queue[0].status, 'done', '那一行说的是上一次的事实，这里不抹它');
    assert.equal(calls.confirms.length, 0, '连问都不必问');
  } finally { restore(); }
});

test('流程：没有可还原项时说的还是自己那两句，不被"队列正在执行中"盖掉', async () => {
  const restore = stubApi({});
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await push(flows, table({ tasks: [srvTask('t1', 'apply', 'b', 'running')] }));   // 队列在跑
    assert.equal(state.queueRunning, true);
    await flows.restoreItemHosts(findItem('a'));             // a 没托管行
    await flows.restoreCategoryHosts(state.categories[0]);   // 类目一里没有托管行
    assert.deepEqual(calls.toasts, ['没有可还原的状态', '此类目下没有可还原项'], '原型 :2774 与 :2805 那两句排在忙之前');
  } finally { restore(); }
});

/* ================= 队列那四颗钮 ================= */

test('流程：暂停与继续那一句照服务回的闸状态说，界面不自己翻', async () => {
  const restore = stubApi({ pauseQueue: () => ({ paused: true }), resumeQueue: () => ({ paused: false }) });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await flows.pauseToggle();
    assert.deepEqual(calls.toasts, ['将在当前任务完成后暂停']);
    assert.equal(state.paused, true);
    await flows.pauseToggle();
    assert.deepEqual(calls.toasts, ['将在当前任务完成后暂停', '队列已继续']);
    assert.equal(state.paused, false);
  } finally { restore(); }
});

test('流程：按了暂停但服务说闸没关上时，界面上那一格跟着服务说的那个数走', async () => {
  const restore = stubApi({ pauseQueue: () => ({ paused: false }) });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await flows.pauseToggle();
    assert.equal(state.paused, false, 'paused 只有服务能说：PauseQueue 与 ResumeQueue 是唯二能改它的地方（L2）');
    assert.deepEqual(calls.toasts, ['队列已继续']);
  } finally { restore(); }
});

test('流程：取消等待那句报的是服务实际撤掉的条数，不是界面数出来的行数', async () => {
  const seen = [];
  const restore = stubApi({ cancelTask: taskID => { seen.push(taskID); return { cancelled: true, count: 1 }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await push(flows, table({ tasks: [srvTask('t1', 'apply', 'a', 'waiting'), srvTask('t2', 'apply', 'b', 'waiting'), srvTask('t3', 'apply', 'c', 'running')] }));
    await flows.cancelWaiting();
    assert.deepEqual(calls.confirms[0].lines, ['将取消 2 个等待中的任务，', '正在执行的任务不受影响。'],
      '问的那一句是界面数出来的（点之前用户看见的就是那两条）');
    assert.deepEqual(seen, [undefined], '不指名的 CancelTask＝撤掉全部等待中的行');
    assert.deepEqual(calls.toasts, ['已取消 1 个等待任务'], '答的那一句用服务回的 count');
  } finally { restore(); }
});

test('流程：清空已完成＝点之前少了几行；重试没有就说一句', async () => {
  const restore = stubApi({ clearTasks: async () => table({ tasks: [srvTask('t5', 'apply', 'a', 'waiting')] }) });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const five = ['done', 'skipped', 'cancelled', 'restored', 'waiting'].map((status, i) => srvTask(`t${i}`, 'apply', 'a', status));
    await push(flows, table({ tasks: five }));
    assert.equal(state.queue.length, 5);

    await flows.clearDone();
    assert.equal(state.queue.length, 1);
    assert.deepEqual(calls.toasts, ['已清理 4 条已完成任务'], '原型 :3147 那四个状态都算已完成');

    await flows.retryFailed();
    assert.deepEqual(calls.toasts[1], '没有需要重试的任务');
  } finally { restore(); }
});

test('流程：重试失败的那几条＝把那几个条目重新提交一次，老的失败行留在表上', async () => {
  const seen = [];
  const restore = stubApi({ applyQueue: ids => { seen.push(ids.slice()); return { taskID: 'batch-7' }; } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await push(flows, table({
      tasks: [
        srvTask('t1', 'apply', 'a', 'failed'), srvTask('t2', 'apply', 'b', 'rolledback'),
        srvTask('t3', 'apply', 'a', 'failed'), srvTask('t4', 'restore', 'c', 'done')
      ]
    }));
    await flows.retryFailed();
    assert.deepEqual(seen, [['a', 'b']], '按条目去重；恢复那一行的失败不靠 ApplyQueue 重试');
    assert.deepEqual(calls.toasts, ['已重新入队 2 项']);
    assert.equal(state.queue.filter(tk => tk.status === 'failed').length, 2, '老的失败行一条也不抹——那一句说的是上一次的事实');

    calls.toasts.length = 0; seen.length = 0;
    await flows.retryOne('t2');
    assert.deepEqual(seen, [['b']]);
    assert.deepEqual(calls.toasts, ['已重新入队：类目 / 条目b']);
  } finally { restore(); }
});

test('判据：契约那 16 个方法里没有 RetryTask——重试只能是"重新提交一次"这一种写法', () => {
  assert.ok(!Object.values(METHODS).includes('RetryTask'), '将来真要加这一档，先改 internal/contract 那一版号');
});

/* ================= 同步状态 / 开机 / 断线 ================= */

test('流程：同步状态那句按"变了几条"说话，两次问同一份表就是第二句', async () => {
  const snap = table({ items: [srvItem('a', { status: 'detected', bestIp: '8.8.8.8' }), srvItem('b', { status: 'detected', bestIp: '1.1.1.1' })] });
  const restore = stubApi({ getSnapshot: async () => snap });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await flows.syncStatuses();
    assert.deepEqual(calls.toasts, ['同步完成，1 项状态有更新'], 'b 本来就是已检测，那条没变');
    await flows.syncStatuses();
    assert.deepEqual(calls.toasts, ['同步完成，1 项状态有更新', '同步完成，界面已与真实状态一致']);
  } finally { restore(); }
});

test('流程：开机那一下补上环境那三格与版本号；服务没起时一句人话、返回值说 false', async () => {
  const restore = stubApi({
    attach: async () => ({}),
    getSnapshot: async () => table({ items: [srvItem('a', { status: 'detected' })], tasks: [], state: { queuePaused: true, dnsService: 'systemd-resolved', dataDir: '/var/lib/gac', hostsPath: '/etc/hosts' } }),
    getStatus: async () => ({ service: { dnsService: 'systemd-resolved', dataDir: '/var/lib/gac', hostsPath: '/etc/hosts' }, writeLockHeld: true }),
    checkPermissions: async () => ({ canWrite: true }),
    getVersion: async () => ({ current: '0.3.0', buildDate: '' })
  });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    assert.equal(await flows.start(), true);
    assert.deepEqual(calls.toasts, []);
    assert.equal(state.paused, true, '暂停闸跟着快照走（PauseQueue／ResumeQueue 之外没人能改它）');
    assert.equal(state.writeLockHeld, true);
    assert.equal(state.adminGranted, true);
    assert.equal(state.appStatus.dnsService, 'systemd-resolved');
    assert.equal(state.appStatus.hostsPath, '/etc/hosts');
    assert.equal(state.update.current, '0.3.0');
    assert.equal(state.update.buildDate, '', '打包时没填就留空，由渲染那侧画破折号，不许显示 0');
    assert.equal(findItem('a').status, 'detected');
  } finally { restore(); }

  const down = stubApi({ attach: async () => { throw new WireError('GAC-IPC-040', { endpoint: '/run/user/1000/gac.sock' }, 'connect: no such file'); } });
  const second = makeUi();
  try {
    const flows = createFlows({ t, ui: second.ui });
    assert.equal(await flows.start(), false, '服务没起来这件事要能被调用方看见，不能演成"起来了"');
    assert.deepEqual(second.calls.toasts, ['⟪错⟫连不上后台服务（/run/user/1000/gac.sock）。它可能没在跑：启动它，界面会自己接上']);
    assert.equal(state.lastError.code, 'GAC-IPC-040');
  } finally { down(); }
});

test('流程：线断了——等着的流程当场拿到那句「连不上」并且只说一遍，把手松开，界面不再报忙', async () => {
  const restore = stubApi({ detect: () => ({ taskID: 'batch-1' }) });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    const done = flows.detectAll();
    await tick();
    await push(flows, table({ items: [srvItem('a', { status: 'detecting' })], tasks: [srvTask('task-1', 'detect', 'a', 'running')] }));
    assert.equal(state.batchDetecting, true);
    calls.toasts.length = 0;

    flows.onLink({ connected: false, endpoint: '/run/user/1000/gac.sock', err: 'connection reset by peer' });
    await done;

    assert.deepEqual(calls.toasts, ['⟪错⟫连不上后台服务（/run/user/1000/gac.sock）。它可能没在跑：启动它，界面会自己接上'],
      '同一句话不许说两遍：onLink 说过一次，等在这里的那条流程只补状态栏那一格');
    assert.equal(state.connected, false);
    assert.equal(state.pending.detect, false);
    assert.equal(state.pending.queue, false);
    assert.equal(state.batchDetecting, false, '线断了那张表不会再更新，继续报忙就是把用户锁在屏幕前');
    assert.equal(state.queueRunning, false);
    assert.equal(state.lastError.code, 'GAC-IPC-040');
  } finally { restore(); }
});

test('流程：那一次调用等成超时（12 秒没人答话）时说服务那句超时，把手照样松开', async () => {
  const restore = stubApi({ detect: async () => { throw new WireError('GAC-IPC-041', { method: 'Detect' }, 'timeout'); } });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await flows.detectSingle(findItem('a'));
    assert.equal(state.pending.detect, false);
    assert.equal(state.batchDetecting, false);
    assert.deepEqual(calls.toasts, ['⟪错⟫后台服务 12 秒没答话，这一句先当没成功，屏幕上还是上一次的读数']);
  } finally { restore(); }
});

test('流程：重连之后自己再取一次全貌，不靠用户去点同步', async () => {
  let n = 0;
  const restore = stubApi({
    attach: async () => ({}),
    getSnapshot: async () => { n++; return table({ items: [srvItem('a', { status: 'detected', bestIp: '4.4.4.4' })] }); },
    getStatus: async () => ({ service: {} }),
    checkPermissions: async () => ({ canWrite: false }),
    getVersion: async () => ({ current: '' })
  });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    await flows.start();
    assert.equal(n, 1);
    calls.toasts.length = 0;
    flows.onLink({ connected: true, endpoint: '/x', err: '', dropped: 0 });
    await tick();
    await tick();
    assert.equal(n, 2, '接上就重取一次：断的那一段里错过的变化全靠这一份补回来');
    assert.equal(findItem('a').bestIp, '4.4.4.4');
    assert.deepEqual(calls.toasts, [], '重连不是错误，不弹话');
  } finally { restore(); }
});

/* ================= 启动自动化 ================= */

test('流程：启动自动化把自己那颗忙标志抬着也照样把检测发出去（旧 bug：被自己那道门挡成空转）', async () => {
  const seen = { detect: [], apply: [] };
  const restore = stubApi({
    detect: ids => { seen.detect.push(ids.slice()); return { taskID: 'batch-1' }; },
    applyQueue: ids => { seen.apply.push(ids.slice()); return { taskID: 'batch-2' }; }
  });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    state.settings.autoDetectOnStartup = true;
    state.settings.autoApplyOnStartup = true;

    const running = ['a', 'b', 'c', 'd'].map(id => srvTask(`task-${id}`, 'detect', id, 'running'));
    const final = table({
      items: ['a', 'b', 'c', 'd'].map(id => srvItem(id, { status: 'detected', bestIp: `9.9.9.${id}` })),
      tasks: ['a', 'b', 'c', 'd'].map(id => srvTask(`task-${id}`, 'detect', id, 'done'))
    });
    const done = flows.runStartupAutomation();
    await tick();
    assert.equal(state.startupAutomationRunning, true);
    assert.deepEqual(seen.detect, [['a', 'b', 'c', 'd']], '启动自动化那一刻它自己就是"忙"，但检测请求照样发了出去');

    flows.onEvent({ name: 'detect:progress', snapshot: table({ tasks: running }) });
    await tick();
    flows.onEvent({ name: 'queue:done', snapshot: final });
    await done;

    assert.deepEqual(seen.apply, [['a', 'b', 'c', 'd']], '检测跑完之后接着入队');
    assert.deepEqual(calls.toasts, ['启动自动化：开始检测 4 项…', '启动自动化：入队 4 项，开始应用…']);
    assert.equal(state.startupAutomationRunning, false);
  } finally { restore(); }
});

test('流程：启动自动化中途把自动应用关掉就停在检测那一步', async () => {
  const seen = { detect: [], apply: [] };
  const restore = stubApi({
    detect: ids => { seen.detect.push(ids.slice()); return { taskID: 'batch-1' }; },
    applyQueue: ids => { seen.apply.push(ids.slice()); return { taskID: 'batch-2' }; }
  });
  const { ui, calls } = makeUi();
  try {
    const flows = createFlows({ t, ui });
    state.settings.autoDetectOnStartup = true;
    state.settings.autoApplyOnStartup = false;
    await run(flows, () => flows.runStartupAutomation(),
      table({ tasks: [srvTask('task-1', 'detect', 'a', 'running')] }),
      table({ items: ['a', 'b', 'c', 'd'].map(id => srvItem(id, { status: 'detected', bestIp: '7.7.7.7' })), tasks: [srvTask('task-1', 'detect', 'a', 'done')] }));
    assert.equal(seen.detect.length, 1);
    assert.deepEqual(seen.apply, [], '没开自动应用就不该入队');
    assert.equal(state.startupAutomationRunning, false);
    assert.ok(calls.toasts.includes('启动自动化：开始检测 4 项…'), calls.toasts.join('|'));
  } finally { restore(); }
});

test('流程：启动自动化没勾选项时给的是带前缀的那句，跟手点的那句分得开；开关关着时一句也不说', async () => {
  const restore = stubApi({});
  const { ui, calls } = makeUi();
  try {
    state.settings.autoDetectOnStartup = true;   // 默认已改关（2026-10-25），这里测的是开着时的分支
    allUncheck();
    const flows = createFlows({ t, ui });
    await flows.runStartupAutomation();
    assert.deepEqual(calls.toasts, ['启动自动化：没有勾选项，已跳过'], '不是「请先勾选要检测的项」那一句');
    assert.equal(state.startupAutomationRunning, false);

    calls.toasts.length = 0;
    state.settings.autoDetectOnStartup = false;
    state.categories[0].items[0].checked = true;
    await flows.runStartupAutomation();
    assert.deepEqual(calls.toasts, [], '开关关着＝这台机器上没人要它跑，不解释也不报错');
  } finally { restore(); }
});

test('流程：启动自动化撞见队列在跑时说的是自己那句，并且一个请求也不发', async () => {
  const seen = [];
  const restore = stubApi({ detect: ids => { seen.push(ids); return { taskID: 'x' }; } });
  const { ui, calls } = makeUi();
  try {
    state.settings.autoDetectOnStartup = true;   // 默认已改关（2026-10-25）
    const flows = createFlows({ t, ui });
    await push(flows, table({ tasks: [srvTask('t1', 'apply', 'b', 'running')] }));
    await flows.runStartupAutomation();
    assert.deepEqual(calls.toasts, ['启动自动化：勾选项都在执行中，已跳过']);
    assert.deepEqual(seen, []);
    assert.equal(state.startupAutomationRunning, false);
  } finally { restore(); }
});

test('流程：启动自动化里那一次请求当场失败＝说服务那一句，不再补一条「启动自动化异常」', async () => {
  const restore = stubApi({ detect: async () => { throw new WireError('GAC-IPC-041', { method: 'Detect' }, 'timeout'); } });
  const { ui, calls } = makeUi();
  try {
    state.settings.autoDetectOnStartup = true;   // 默认已改关（2026-10-25）
    const flows = createFlows({ t, ui });
    await flows.runStartupAutomation();
    assert.deepEqual(calls.toasts, ['启动自动化：开始检测 4 项…', '⟪错⟫后台服务 12 秒没答话，这一句先当没成功，屏幕上还是上一次的读数'],
      '那一次请求没回话由 dispatch 就地报（跟手点那一下同一条路），这里不叠第二句');
    assert.equal(state.startupAutomationRunning, false, '跑挂了也要把手放掉，否则后面全点不动');
    assert.equal(state.pending.detect, false);
  } finally { restore(); }
});

test('流程：启动自动化跑到一半线断了——断线那一句与带前缀那一句各说一遍，忙标志照样放掉', async () => {
  const restore = stubApi({ detect: () => ({ taskID: 'batch-1' }) });
  const { ui, calls } = makeUi();
  try {
    state.settings.autoDetectOnStartup = true;   // 默认已改关（2026-10-25）
    const flows = createFlows({ t, ui });
    const done = flows.runStartupAutomation();
    await tick();
    await push(flows, table({ tasks: [srvTask('task-1', 'detect', 'a', 'running')] }));
    assert.equal(state.batchDetecting, true);

    flows.onLink({ connected: false, endpoint: '/run/user/1000/gac.sock', err: 'eof' });
    await done;
    assert.deepEqual(calls.toasts, [
      '启动自动化：开始检测 4 项…',
      '⟪错⟫连不上后台服务（/run/user/1000/gac.sock）。它可能没在跑：启动它，界面会自己接上',
      '⟪错⟫启动自动化异常：连不上后台服务（/run/user/1000/gac.sock）。它可能没在跑：启动它，界面会自己接上'
    ], '第二句是通道报的，第三句是这一整轮自动化停在哪一步——留着它是为了让人知道检测没跑完，不是重复啰嗦');
    assert.equal(state.startupAutomationRunning, false);
    assert.equal(state.batchDetecting, false);
  } finally { restore(); }
});

/* ================= 注入形状 ================= */

test('流程：注入的 ui 少一个回调就在启动那一下抛错，不是跑到一半才静默停住', () => {
  const { ui } = makeUi();
  delete ui.scrollToInt;
  assert.throws(() => createFlows({ t, ui }), /缺 scrollToInt/);
});
