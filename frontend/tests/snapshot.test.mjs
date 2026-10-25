/* 快照进界面那一步的机器判据。干什么用：core/api/snapshot.js 是数据进 store 的唯一入口
   （开机那一次 GetSnapshot、每一条事件、ClearTasks 的回执三条路都走它），这里把它的合并规则逐条钉住。
   会错在哪：① 用例自己造一个"看起来对"的 store 再断言，比的是用例不是代码——所以这里喂的都是契约那三份形状
                （Item／Task／Snapshot 的 JSON 名），字段名与 internal/contract/contract.go 逐字一致，
                名单比对归 tests/contract.test.mjs，这里只判"拿到这样一份之后界面变成什么样"；
             ② 忙标志（batchDetecting／queueRunning）是全站"能不能点"的唯一来源，判错一处就是 C2 那笔账，
                所以这一份里给它单独两组用例，含断线那一档。
   怎么回来：红的时候先看断言消息里那一栏字段名——多半是服务那一侧改了形状（或这里写漏了一个 ?.），
             其次看 core/api/snapshot.js 里 applyItem／refreshBusy 那两处有没有长出第二条改状态的路。 */

import { test, beforeEach } from 'node:test';
import assert from 'node:assert/strict';

import { state, resetState, findItem } from '../core/store.js';
import { buildTree, mergeSnapshot, taskRow, refreshBusy } from '../core/api/snapshot.js';
import { createI18n } from '../core/i18n/t.js';
import zhCN from '../core/i18n/locales/zh-CN.js';
import { clockFromIso } from '../utils/format.js';

const t = createI18n({ 'zh-CN': zhCN });

function seedItem(id, extra = {}) {
  return {
    id, name: `条目${id}`, checked: true, status: 'unset', bestIp: null, appliedIp: null,
    prevAppliedIp: undefined, managed: false, latency: 0, speed: '', score: 0, source: '',
    sourceRole: '', lastCheck: '', errorMsg: '', domains: ['demo.test'], candidates: [], ...extra
  };
}

function seed() {
  resetState({
    categories: [
      { id: 'c1', name: '类目一', items: [seedItem('a'), seedItem('b', { checked: false })] },
      { id: 'c2', name: '类目二', items: [seedItem('c')] }
    ],
    queue: [], selectedId: 'a', lastDetect: ''
  });
  state.collapsed.clear();
  Object.assign(state, {
    batchDetecting: false, queueRunning: false, paused: false, connected: true,
    writeLockHeld: false, lastError: { code: '', params: {}, text: '' }
  });
  state.pending.detect = false;
  state.pending.queue = false;
  state.appStatus = { dnsService: '—', dataDir: '', hostsPath: '' };
}

beforeEach(seed);

/** 契约 Item 的一份最小形状：没写的栏就是"服务没给"，用例不许替它补 */
function item(over = {}) {
  return { id: 'a', name: '条目a', status: 'detected', ...over };
}

function snapOf({ items = [], tasks = [], state: st = {} } = {}) {
  return { state: st, items, tasks };
}

/* ================= 建树（开机第一次快照之前那一份空表） ================= */

test('建树：初态一律「未替换、未托管」，勾选只由清单里 init 那两档决定', () => {
  const menus = [
    { id: 'c1', name: '类目一', items: [
      { id: 'a', name: '条目a', domains: ['api.github.com'], init: 'unset' },
      { id: 'b', name: '条目b', domains: ['raw.githubusercontent.com'], init: 'applied' },
      { id: 'c', name: '条目c', domains: ['codeload.github.com'], init: 'stale' },
      { id: 'd', name: '条目d', domains: ['objects.githubusercontent.com'], init: 'pending' }
    ] }
  ];
  const tree = buildTree(menus);
  const items = tree[0].items;
  assert.deepEqual(items.map(i => i.status), ['unset', 'unset', 'unset', 'unset'],
    '服务还没答话之前屏幕上不许出现"已替换"那一档');
  assert.deepEqual(items.map(i => i.managed), [false, false, false, false], '托管凭据只由文件那一份说');
  assert.deepEqual(items.map(i => i.checked), [false, true, true, false],
    'init 只决定"这条默认勾上不"，别的什么都不决定');
  assert.deepEqual(items.map(i => i.bestIp), [null, null, null, null]);
  assert.deepEqual(items.map(i => i.appliedIp), [null, null, null, null]);
});

test('建树：域名表是复制出来的，清单那份随后怎么改都不影响界面', () => {
  const menus = [{ id: 'c1', name: '类目一', items: [{ id: 'a', name: '条目a', domains: ['x.test'], init: 'unset' }] }];
  const tree = buildTree(menus);
  menus[0].items[0].domains.push('y.test');
  assert.deepEqual(tree[0].items[0].domains, ['x.test']);
});

/* ================= 合并：条目那几栏 ================= */

test('合并：就地改写同一个对象——勾选、选中、候选表这些服务不认识的东西一个也不丢', () => {
  const a = findItem('a');
  mergeSnapshot(state, snapOf({ items: [item({ bestIp: '9.9.9.9' })] }), t);
  assert.equal(findItem('a'), a, '换新对象就等于把用户正在看的那一行丢了');
  assert.equal(a.checked, true);
  assert.equal(state.selectedId, 'a');
  assert.equal(a.bestIp, '9.9.9.9');
});

test('合并：那张表整个跟着服务这份快照走，界面不保留上一帧的行', () => {
  state.queue = [taskRow({ id: 'old', kind: 'detect', itemId: 'a', status: 'done' }, t)];
  mergeSnapshot(state, snapOf({ items: [], tasks: [] }), t);
  assert.deepEqual(state.queue, [], '契约的回执只回完整快照，界面"先收一半再猜另一半"是那条铁律禁止的');
  assert.equal(findItem('a').checked, true, '表换了不影响条目那几行——两条路各管各的');
});

test('合并：清单里没有的那一条宁可少画一行，也不凭空造一行', () => {
  mergeSnapshot(state, snapOf({ items: [item({ id: 'zz', name: '外来条目' })] }), t);
  assert.equal(state.categories.flatMap(c => c.items).length, 3);
  assert.equal(findItem('zz'), null);
});

test('合并：服务没给的那几栏留空，不许演成 0 或空串那种假读数', () => {
  const b = findItem('b');
  b.lastCheck = '10:00:00';
  b.speed = '3.4 MB/s';
  mergeSnapshot(state, snapOf({ items: [{ id: 'b', name: '条目b', status: 'detected' }] }), t);
  assert.equal(b.appliedIp, null, '没给＝没有，不是空串');
  assert.equal(b.prevAppliedIp, null);
  assert.equal(b.bestIp, null);
  assert.equal(b.latency, 0);
  assert.equal(b.lastCheck, '', '解不开的时钟留空，由渲染那侧画破折号，界面上不许出现 Invalid Date');
  assert.equal(b.speed, '3.4 MB/s', 'speed 不在契约里：服务没这一栏，界面自己那一份就不许被抹掉');
});

test('合并：prevAppliedIp 那一栏空串装着两件事，所以还原凭据只认 managed', () => {
  mergeSnapshot(state, snapOf({ items: [item({ prevAppliedIp: '', managed: true, appliedIp: '1.1.1.1' })] }), t);
  const a = findItem('a');
  assert.equal(a.prevAppliedIp, null, '空串归到 null');
  assert.equal(a.managed, true, '"文件里有我们写的行"这一件事只有这一栏说得出');
});

test('合并：界面那一句由码＋参数在语言文件凑，服务那句原文只留在 text 里（已定：D6 乙）', () => {
  mergeSnapshot(state, snapOf({
    items: [item({ status: 'error', errorCode: 'GAC-HOSTS-007', errorParams: { domain: 'github.com' } })],
    state: { lastError: 'hosts conflict: github.com', lastErrorCode: 'GAC-HOSTS-004', lastErrorParams: { path: '/etc/hosts' } }
  }), t);
  assert.equal(findItem('a').errorMsg, '「github.com」被两条条目一起管着，这一步拒绝写 hosts（先只留一条）');
  assert.equal(state.lastError.code, 'GAC-HOSTS-004');
  assert.deepEqual(state.lastError.params, { path: '/etc/hosts' });
  assert.equal(state.lastError.text, 'hosts conflict: github.com', '原文留着是给日志与控制台看的，不是给界面');
});

test('合并：认不出来的错误码要说"没登记的错误"那一句，不许留一个 ⟦…⟧ 在屏幕上', () => {
  mergeSnapshot(state, snapOf({ items: [item({ status: 'error', errorCode: 'GAC-HOSTS-999' })] }), t);
  assert.match(findItem('a').errorMsg, /撞上一条没登记的错误/);
  assert.ok(!findItem('a').errorMsg.includes('⟦'), findItem('a').errorMsg);
});

test('合并：状态栏「上次检测」取所有条目里最近那一次，一份都没有就留空', () => {
  mergeSnapshot(state, snapOf({ items: [
    item({ id: 'a', lastCheck: '2026-10-21T09:00:00Z' }),
    item({ id: 'b', lastCheck: '2026-10-21T10:30:00Z' }),
    item({ id: 'c', lastCheck: '' })
  ] }), t);
  assert.equal(state.lastDetect, clockFromIso('2026-10-21T10:30:00Z'),
    '取的是所有条目里最近那一次（服务没说"上一轮什么时候跑的"）');
  assert.ok(state.lastDetect !== clockFromIso('2026-10-21T09:00:00Z'), '不能拿第一条的时间盖上去');

  mergeSnapshot(state, snapOf({ items: [item({ id: 'a', lastCheck: '' }), item({ id: 'b', lastCheck: '不像时间' })] }), t);
  assert.equal(state.lastDetect, '', '一份都解不开就留空，由状态栏画破折号');
});

/* ================= 合并：队列那一行 ================= */

test('队列行：阶段名按种类取两套，认不出的阶段留空，耗时 0 不许写成 0.0s', () => {
  assert.equal(taskRow({ id: 't1', kind: 'apply', status: 'running', phase: 'write' }, t).phaseText, '写入域名映射…');
  assert.equal(taskRow({ id: 't2', kind: 'restore', status: 'restoring', phase: 'write' }, t).phaseText, '写回应用前映射…');
  assert.equal(taskRow({ id: 't3', kind: 'apply', status: 'running', phase: 'nonsense' }, t).phaseText, '');
  assert.equal(taskRow({ id: 't4', kind: 'apply', status: 'done', phase: '' }, t).phaseText, '');
  assert.equal(taskRow({ id: 't5', kind: 'apply', status: 'waiting' }, t).duration, '');
  assert.equal(taskRow({ id: 't6', kind: 'apply', status: 'done', durationMs: 1500 }, t).duration, '1.5s');
  assert.equal(taskRow({ id: 't7', kind: 'apply', status: 'done', durationMs: 0 }, t).duration, '');
});

test('队列行：那一列失败原因同样只认码＋参数，服务那句原文不进界面', () => {
  const row = taskRow({ id: 't8', kind: 'detect', itemId: 'a', status: 'failed', lastErr: 'doh timeout',
    errorCode: 'GAC-DETECT-001' }, t);
  assert.equal(row.errorMsg, '这一条没检出可用的地址，详情面板那一列还是空的');
  assert.equal(row.name, '', '服务没给 name 就留空：那一行显示的名字跟着快照走，不由界面拿 itemId 去别处查');
  assert.ok(!row.errorMsg.includes('doh timeout'));
});

/* ================= 忙标志（全站「能不能点」的唯一来源） ================= */

const TASK = (kind, status, over = {}) => ({ id: `t-${kind}-${status}`, kind, itemId: 'a', status, ...over });

test('忙标志：等待与正在跑都算忙，检测与队列分开算，跑完了都不忙', () => {
  state.queue = [TASK('detect', 'waiting')];
  refreshBusy(state);
  assert.equal(state.batchDetecting, true);
  assert.equal(state.queueRunning, false, '检测的行不能把队列也判成忙，否则一颗钮灰掉一片');

  state.queue = [TASK('restore', 'restoring')];
  refreshBusy(state);
  assert.equal(state.batchDetecting, false);
  assert.equal(state.queueRunning, true, '恢复那一行占的是同一条串行通道，算队列忙');

  state.queue = [TASK('detect', 'done'), TASK('apply', 'failed'), TASK('restore', 'cancelled')];
  refreshBusy(state);
  assert.equal(state.batchDetecting, false);
  assert.equal(state.queueRunning, false);

  state.queue = [];
  refreshBusy(state);
  assert.equal(state.batchDetecting, false);
});

test('忙标志：把手管的是"发出去了还没回话"那一段，这里只读不松', () => {
  state.pending.detect = true;
  state.queue = [];
  refreshBusy(state);
  assert.equal(state.batchDetecting, true, '抬起的那一刻就报忙——门问的是这两个数，不是把手');
  assert.equal(state.pending.detect, true, '松开只有 flows.js 的 dispatch（那一次调用回话了）与 onLink（线断了）两处');

  state.pending.detect = false;
  state.queue = [TASK('detect', 'running')];
  refreshBusy(state);
  assert.equal(state.batchDetecting, true, '表里有活动行时不看把手');
});

test('忙标志：线断了那张表一律算不忙，继续报忙就是把用户锁在屏幕前面', () => {
  state.connected = false;
  state.pending.queue = true;
  state.queue = [TASK('apply', 'running'), TASK('detect', 'waiting')];
  refreshBusy(state);
  assert.equal(state.batchDetecting, false);
  assert.equal(state.queueRunning, false);
  assert.equal(state.pending.queue, true, '这里不改把手：断线那一步由 flows.onLink 当场松，两处各写一套就漂了');

  state.connected = true;
  refreshBusy(state);
  assert.equal(state.queueRunning, true, '重连之后同一份表立刻恢复成忙，不需要再点一次');
});

test('合并：整份快照里那三格环境读数与暂停闸都跟着服务走，认不出来留 —', () => {
  mergeSnapshot(state, snapOf({ state: { queuePaused: true, dnsService: '', dataDir: '/var/lib/gac', hostsPath: '/etc/hosts' } }), t);
  assert.equal(state.paused, true);
  assert.equal(state.appStatus.dnsService, '—', '这台机器认不出谁在管解析就画破折号，绝不自己编一个（已定：C5）');
  assert.equal(state.appStatus.dataDir, '/var/lib/gac');
  assert.equal(state.appStatus.hostsPath, '/etc/hosts');

  mergeSnapshot(state, snapOf({ state: { queuePaused: false, dnsService: 'systemd-resolved' } }), t);
  assert.equal(state.paused, false);
  assert.equal(state.appStatus.dnsService, 'systemd-resolved');
});

test('合并：形状不完整的那一份（服务回了一半）不改画面上任何东西，也不抛', () => {
  const before = findItem('a').status;
  mergeSnapshot(state, null, t);
  mergeSnapshot(state, {}, t);
  assert.equal(findItem('a').status, before);
});
