/* 界面与外壳那一条线的机器判据。干什么用：core/api/wire.js 是全项目唯一碰 Wails 注入事件的地方，
   这里用一条假线（自己记账的 On／Emit）＋一个自己拨的时钟，把它的四条规矩逐条钉住：
   ① 号编在页面这一侧，回执按号对回来；② 超时判得比壳那一侧晚（壳 10 秒，界面 12 秒）；
   ③ 契约版本号对不上就当场粘住；④ 找不到宿主要说"线根本不通"，不许装作服务回了一句失败。
   会错在哪：① 用真宿主（globalThis.wails）→ 用例在没有壳的机器上必红，红的理由是"环境"不是"逻辑"；
                所以 loadRuntime 与 timers 都从外面喂进去，时间由用例自己拨，一根真实 setTimeout 都不留。
             ② 只测"发出去"不测"对回来" → 同时在飞几条调用正是回执认错人的那一档，必须两条一起测。
             ③ 事件那一路只测一条线 → 壳把参数包成数组还是对象是 Wails 自己会变的事，两种都要认下来。
   怎么回来：红了先看假线记的那两本账（emitted 发过什么、listeners 装了几条线），
             再看 wire.js 的 onReply／onState／attach 三处；名字本身对不对归 tests/contract.test.mjs 管，
             这一份不管名字、只管走线。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';

import { createWire, WireError, REPLY_TIMEOUT_MS } from '../core/api/wire.js';
import { METHODS, WIRE, EXPECTED_PROTOCOL } from '../core/contract/methods.js';
import { IPC_EVENTS } from '../core/contract/events.js';

const SOCK = '/run/user/1000/service.sock';

/** 一条假线：把页面发出去的和壳推进来的都记账，用例随时能看能拨 */
function makeRuntime({ emitThrows = false } = {}) {
  const listeners = new Map();
  const emitted = [];
  return {
    emitted,
    listeners,
    On(name, fn) {
      if (!listeners.has(name)) listeners.set(name, []);
      listeners.get(name).push(fn);
    },
    Emit(name, data) {
      if (emitThrows) throw new Error('runtime.js 那侧断了');
      emitted.push({ name, data });
    },
    /** 壳→页面：三种投递形状都从这里走（对象／包一层 data／装在一个元素的数组里） */
    fire(name, data, shape = 'direct') {
      const payload = shape === 'direct' ? data : shape === 'wrapped' ? { data } : [data];
      for (const fn of listeners.get(name) || []) fn(payload);
    },
    count(name) { return (listeners.get(name) || []).length; }
  };
}

/** 自己拨的时钟：用例不看真秒表，也能验"12 秒"这一个数与"定时器拆干净没有" */
function makeClock() {
  const timers = new Map();
  let next = 1;
  return {
    api: {
      set(fn, ms) { const id = next++; timers.set(id, { fn, ms }); return id; },
      clear(id) { timers.delete(id); }
    },
    live: () => [...timers.values()].map(v => v.ms),
    ids: () => [...timers.keys()],
    fire(id) { const e = timers.get(id); if (e) { timers.delete(id); e.fn(); } },
    fireAll() { for (const [id, e] of [...timers]) { timers.delete(id); e.fn(); } }
  };
}

/** 起一条接好线的假桥。返回的 w 已经 attach 过，可以直接 call */
async function ready(over = {}) {
  const runtime = makeRuntime(over);
  const clock = makeClock();
  const w = createWire({ timers: clock.api, loadRuntime: async () => runtime });
  await w.attach();
  return { w, runtime, clock };
}

/** 最近一次发出去的那一句（号由桥自己编，用例不许写死） */
function lastInvoke(runtime) {
  const e = runtime.emitted[runtime.emitted.length - 1];
  assert.equal(e.name, WIRE.invoke, '发出去只准走 gac:invoke 那一条线');
  return e.data;
}

function replyFor(runtime, over) {
  return { id: lastInvoke(runtime).id, ok: true, protocol: EXPECTED_PROTOCOL, ...over };
}

/* ================= ① 编号：回执按号对回来 ================= */

test('两条在飞时各编一个号，先回哪条都不认错人', async () => {
  const { w, runtime } = await ready();
  const pDetect = w.call(METHODS.detect, { itemIDs: ['a'] });
  const pStatus = w.call(METHODS.getStatus, {});
  assert.deepEqual(runtime.emitted.map(e => e.data.id), [1, 2], '号从 1 开始递增，0 留给"没编上号"');

  // 壳先回第二条：状态那份必须回到 getStatus 那一次，不能被检测那条吃掉
  runtime.fire(WIRE.reply, { id: 2, ok: true, protocol: EXPECTED_PROTOCOL, result: { service: { name: 'gac' }, writeLockHeld: true } });
  assert.deepEqual(await pStatus, { service: { name: 'gac' }, writeLockHeld: true });

  runtime.fire(WIRE.reply, { id: 1, ok: true, protocol: EXPECTED_PROTOCOL, result: { taskID: 'task-7' } });
  assert.deepEqual(await pDetect, { taskID: 'task-7' });
});

test('发出去的那一句是完整形状：号＋契约方法名＋参数（没给参数也补一个空对象）', async () => {
  const { w, runtime } = await ready();
  w.call(METHODS.restoreHosts);
  assert.deepEqual(lastInvoke(runtime), { id: 1, method: 'RestoreHosts', params: {} });
  w.call(METHODS.cancelTask, { taskID: 'task-9' });
  assert.deepEqual(lastInvoke(runtime), { id: 2, method: 'CancelTask', params: { taskID: 'task-9' } });
});

test('契约里没有的方法名当场抛 043，并且根本不往线上发', async () => {
  const runtime = makeRuntime();
  const clock = makeClock();
  const w = createWire({ timers: clock.api, loadRuntime: async () => runtime });
  await assert.rejects(w.call('DetectAll', {}), e => e.code === 'GAC-IPC-043' && e.params.method === 'DetectAll');
  assert.equal(runtime.emitted.length, 0);
  assert.equal(runtime.listeners.size, 0, '方法名都不对，连宿主都不该去接——省得把"拼错方法"演成"连不上"');
});

/* ================= ② 超时：判得比壳晚 ================= */

test('界面这一侧的超时是 12 秒：壳那一次调用最长等 10 秒（bridge.go 的 callTimeout）', () => {
  assert.equal(REPLY_TIMEOUT_MS, 12_000,
    '比壳短就会先误判这条连接废掉，同期在飞的调用一起说"连不上"');
});

test('12 秒没人答才判超时，报的是 041 并带上方法名；定时器当场拆掉', async () => {
  const { w, runtime, clock } = await ready();
  const p = w.call(METHODS.applyQueue, { itemIDs: ['a'] });
  assert.deepEqual(clock.live(), [12_000]);
  clock.fire(clock.ids()[0]);
  await assert.rejects(p, e => e.code === 'GAC-IPC-041' && e.params.method === 'ApplyQueue');
  assert.deepEqual(clock.live(), [], '判过时的计时器不许留着空转');

  // 迟到的回执：丢掉是对的，不该把已经判废的这条线再改一次
  const errBefore = w.linkState().err;
  runtime.fire(WIRE.reply, replyFor(runtime, { result: { taskID: 'task-1' } }));
  assert.equal(w.linkState().err, errBefore);
});

/* ================= ③ 版本号：对不上就粘住 ================= */

test('壳回的协议号不同＝不是同一轮构建：这一条报 043，之后每一条都当场报，不再发出去', async () => {
  const { w, runtime, clock } = await ready();
  const p = w.call(METHODS.detect, {});
  runtime.fire(WIRE.reply, { id: lastInvoke(runtime).id, ok: true, protocol: EXPECTED_PROTOCOL + 1, result: {} });
  await assert.rejects(p, e => e.code === 'GAC-IPC-043'
    && e.params.got === EXPECTED_PROTOCOL + 1 && e.params.want === EXPECTED_PROTOCOL);
  assert.deepEqual(w.versionMismatch(), { got: EXPECTED_PROTOCOL + 1, want: EXPECTED_PROTOCOL }, '记下来，别再假装只是这一条没回来');
  assert.equal(w.linkState().err.includes('GAC-IPC-043'), true, 'link 那四栏里也要看得见，状态栏才说得清');

  const n = runtime.emitted.length;
  await assert.rejects(w.call(METHODS.getStatus, {}), e => e.code === 'GAC-IPC-043');
  assert.equal(runtime.emitted.length, n, '已经知道两侧不是同一轮构建了，就不该继续往线上发');
  assert.deepEqual(clock.live(), []);
});

/* ================= ④ 走不通的四种说法各归各家 ================= */

test('宿主找不到（浏览器里直接开 dist）报 040，不许演成"服务回了一句失败"', async () => {
  let broken = true;
  const runtime = makeRuntime();
  const clock = makeClock();
  const w = createWire({ timers: clock.api, loadRuntime: async () => { if (broken) throw new Error('window.wails 不存在'); return runtime; } });

  await assert.rejects(w.call(METHODS.getStatus, {}), e => e.code === 'GAC-IPC-040');
  assert.equal(w.hasHost(), false);
  assert.equal(w.linkState().err.startsWith('GAC-IPC-040'), true);

  // 加载失败要作废那一次尝试，否则"壳后来起来了"这一档永远接不上
  broken = false;
  await w.attach();
  assert.equal(w.hasHost(), true);
  assert.equal(runtime.listeners.size, 2 + Object.keys(IPC_EVENTS).length,
    '接上一次就装两条线（回执＋状态）加 5 个事件；gac:invoke 只发不收，不在这里');
});

test('壳那句"线断了"回的是裸消息，端点这条线上的数只有界面知道：补进参数里', async () => {
  const { w, runtime } = await ready();
  runtime.fire(WIRE.state, { connected: false, dropped: 0, endpoint: SOCK, err: 'link dropped' });
  assert.equal(w.linkState().endpoint, SOCK);

  const p = w.call(METHODS.detect, {});
  runtime.fire(WIRE.reply, replyFor(runtime, { ok: false, code: 'GAC-IPC-040', err: 'link dropped' }));
  await assert.rejects(p, e => e.code === 'GAC-IPC-040' && e.params.endpoint === SOCK,
    '语言文件那句要读 endpoint，壳没给就由桥把 link 上那份补上');
});

test('服务回一句别的失败码：原样带着码与参数抛出去，不换成 040', async () => {
  const { w, runtime } = await ready();
  const p = w.call(METHODS.restoreItem, { itemID: 'a' });
  runtime.fire(WIRE.reply, replyFor(runtime, {
    ok: false, code: 'GAC-HOSTS-007', err: 'hosts conflict', params: { domain: 'github.com' }
  }));
  await assert.rejects(p, e => e.code === 'GAC-HOSTS-007' && e.params.domain === 'github.com'
    && e.params.endpoint === undefined);
});

test('发出去那一步就炸（runtime.Emit 抛）：报 040 并带上此刻这条线连的是哪个文件', async () => {
  const { w, runtime, clock } = await ready({ emitThrows: true });
  runtime.fire(WIRE.state, { connected: true, dropped: 0, endpoint: SOCK, err: '' });
  await assert.rejects(w.call(METHODS.pauseQueue, {}), e => e.code === 'GAC-IPC-040' && e.params.endpoint === SOCK);
  assert.deepEqual(clock.live(), [], '发没出去的调用不许留计时器');
});

test('收到的回执不是合法形状：记在 link.err 上（042），不动已经在飞的那几条', async () => {
  const { w, runtime } = await ready();
  const p = w.call(METHODS.getStatus, {});
  runtime.fire(WIRE.reply, { ok: true, protocol: EXPECTED_PROTOCOL });     // 没有 id
  runtime.fire(WIRE.reply, 'not an object');                                // 根本不是对象
  assert.equal(w.linkState().err.includes('GAC-IPC-042'), true);
  runtime.fire(WIRE.reply, replyFor(runtime, { result: { service: {}, writeLockHeld: false } }));
  assert.deepEqual(await p, { service: {}, writeLockHeld: false }, '非法回执不该把合法的那一条顶掉');
});

/* ================= 连接状态那一条线 ================= */

test('gac:state 的每一帧都原样变成 link 那四栏，并通知订阅者', async () => {
  const { w, runtime } = await ready();
  const seen = [];
  w.on(WIRE.state, link => seen.push(link));
  runtime.fire(WIRE.state, { connected: false, dropped: 7, endpoint: SOCK, err: 'connection refused' });
  assert.deepEqual(w.linkState(), { connected: false, dropped: 7, endpoint: SOCK, err: 'connection refused' });
  assert.deepEqual(seen, [w.linkState()]);
  assert.equal(w.linkState() !== seen[0], true, '交出去的是副本：订阅者改不动桥里那一份');

  runtime.fire(WIRE.state, { connected: true, dropped: 0, endpoint: SOCK, err: '' });
  assert.equal(seen.length, 2);
  runtime.fire(WIRE.state, '这一帧不是对象');
  assert.equal(seen.length, 2, '形状不对就整帧丢掉，不许把四栏写成 NaN');
});

test('线重新通的那一刻，之前在飞的全部按"连不上"结掉，不让人干等 12 秒', async () => {
  const { w, runtime, clock } = await ready();
  const p1 = w.call(METHODS.detect, {});
  const p2 = w.call(METHODS.restoreHosts, {});
  assert.equal(clock.live().length, 2);
  runtime.fire(WIRE.state, { connected: true, dropped: 0, endpoint: SOCK, err: '' });
  await assert.rejects(p1, e => e.code === 'GAC-IPC-040' && e.params.endpoint === SOCK);
  await assert.rejects(p2, e => e.code === 'GAC-IPC-040' && e.params.endpoint === SOCK);
  assert.deepEqual(clock.live(), [], '结掉的同时要拆计时器，否则线每翻一次面就攒下一串空转的 12 秒');
});

/* ================= 业务事件那 5 条 ================= */

test('业务事件原样转发；订阅了陌生名字当场抛——拼错一个字母不会报错，只会永远收不到', async () => {
  const { w, runtime } = await ready();
  const got = [];
  const off = w.on(IPC_EVENTS.queueProgress, ev => got.push(ev));
  runtime.fire(IPC_EVENTS.queueProgress, { name: 'queue:progress', snapshot: { items: [], tasks: [] } });
  assert.deepEqual(got, [{ name: 'queue:progress', snapshot: { items: [], tasks: [] } }]);
  off();
  runtime.fire(IPC_EVENTS.queueProgress, { name: 'queue:progress', snapshot: null });
  assert.equal(got.length, 1, '解绑之后不能再收到');

  assert.throws(() => w.on('queue:progresss', () => {}), /不在那三条线与那 5 个事件里/);
  assert.throws(() => w.on('GetStatus', () => {}), /不在那三条线与那 5 个事件里/);
});

test('壳把一句事件包成数组或包一层 data，两种都认；没带对象的那一条补成空快照', async () => {
  const { w, runtime } = await ready();
  const got = [];
  w.on(IPC_EVENTS.detectProgress, ev => got.push(ev));
  const frame = { name: 'detect:progress', snapshot: { items: [], tasks: [] }, payload: { taskID: 'task-3' } };
  runtime.fire(IPC_EVENTS.detectProgress, frame, 'direct');
  runtime.fire(IPC_EVENTS.detectProgress, frame, 'wrapped');
  runtime.fire(IPC_EVENTS.detectProgress, frame, 'array');
  runtime.fire(IPC_EVENTS.detectProgress, null);
  assert.deepEqual(got.slice(0, 3), [frame, frame, frame], '三种投递形状都同一条事件');
  assert.deepEqual(got[3], { name: 'detect:progress', snapshot: null }, '空帧也要有个形状，别让处理函数拿到 undefined');
});

test('attach 只做一次：重复调用等同一个 Promise，线不装两遍', async () => {
  const runtime = makeRuntime();
  const clock = makeClock();
  const w = createWire({ timers: clock.api, loadRuntime: async () => runtime });
  const [a, b] = await Promise.all([w.attach(), w.attach()]);
  await w.attach();
  assert.equal(a, runtime);
  assert.equal(b, runtime);
  assert.equal(runtime.count(WIRE.reply), 1, '装两遍就是一条回执处理两次，编号会错认');
});

test('抛出来的错都是 WireError，带着码与参数——界面那句人话由语言文件凑，不在桥里拼中文', async () => {
  const { w, runtime } = await ready();
  const p = w.call(METHODS.detect, {});
  runtime.fire(WIRE.reply, replyFor(runtime, { ok: false, code: 'GAC-HOSTS-001', err: 'hosts file busy' }));
  const err = await p.catch(e => e);
  assert.ok(err instanceof WireError);
  assert.equal(err.name, 'WireError');
  assert.equal(err.code, 'GAC-HOSTS-001');
  assert.deepEqual(err.params, {});
});
