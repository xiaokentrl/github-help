/* 界面与外壳之间那一条线。干什么用：全项目只有这个文件允许碰 Wails 注入的那套事件——
   界面问一句 gac:invoke（带页面自己编的号），收一句 gac:reply（按号对回来）；
   连接通不通收 gac:state；业务那 5 条事件原样转发给 core/flows.js 之外的订阅者。
   名字、请求与回执那两个形状、契约版本号都不在这里写第二遍：家在 core/contract/methods.js，
   那一份又由 tests/contract.test.mjs 现读 internal/contract/contract.go 对账。

   会错在哪：
   ① 号不编在页面这一侧 → 壳里同时有几条调用在飞时回执会对错人：检测的回执被画成应用的。
      所以 seq 由这里递增、pending 按号存，桥只认号不认顺序。
   ② 超时给得比壳那一侧短 → 界面先把这条连接判废，同期还在飞的调用一起说"连不上"，
      看起来像工具自己坏了。壳那一次调用最长等 10 秒（cmd/gui/bridge.go 的 callTimeout），
      所以这里给 12 秒：只有壳真的没回音才判超时。
   ③ 版本号不比 → 界面与外壳不是同一轮构建时，缺的那一栏永远空着且不报错。
      第一次对不上就把这一判记下来，之后每条回执都提醒，别让人以为"只是这一条没回来"。
   ④ 找不到宿主（在浏览器里直接开 dist、或壳没起） → 绝不能装作"服务回了一句失败"：
      这两种话术在界面上长得一样，但一个是"这台机器上的服务出问题了"，一个是"这页界面根本不在壳里跑"。

   怎么回来：本文件不 import store、不 import i18n、不碰 DOM——它只把一句话送出去、把回执解回来。
   红了先看 linkState() 那四栏（连着／丢了几个帧／连的是哪条线／哪句错），
   那四栏正是 cmd/gui/bridge.go 的 setState 推出来的同一份形状。 */

import { METHODS, WIRE, EXPECTED_PROTOCOL } from '../contract/methods.js';
import { IPC_EVENTS } from '../contract/events.js';

/** 壳那一次调用最长等 10 秒（bridge.go 的 callTimeout），这里给 12 秒：比它短就会先误判 */
export const REPLY_TIMEOUT_MS = 12_000;

/** 一条线走不通时的四个码（都取自 internal/errs/codes.go，界面按码去语言文件换句子〔已定：D6 乙〕） */
const NO_HOST = 'GAC-IPC-040';      // 连不上后台服务：线根本不通，或这条调用赶在通之前
const TIMEOUT = 'GAC-IPC-041';      // 等回执超时：发出去了，12 秒没人答
const BAD_REPLY = 'GAC-IPC-042';    // 收到的回执不是合法形状
const SHAPE = 'GAC-IPC-043';        // 两侧契约版本对不上＝不是同一轮构建

/** 走不通时的错：带着码与参数，让界面那一句人话能在语言文件凑出来，不在这里拼中文。 */
export class WireError extends Error {
  constructor(code, params, message) {
    super(message || code);
    this.name = 'WireError';
    this.code = code;
    this.params = params || {};
  }
}

/** 默认取宿主：先认壳已经装好的 window.wails，再自己去加载那一份运行时。
    为什么两条都要：/wails/runtime.js 由壳的资产服务提供，页面加载它的时机不在我们手里；
    动态 import 用变量＋@vite-ignore 是因为 Vite 会把字面量 '/wails/runtime.js' 当本项目的源码路径去解析。 */
async function defaultLoadRuntime() {
  const installed = globalThis.wails?.Events;
  if (installed?.On && installed?.Emit) return installed;
  const url = '/wails/runtime.js';
  const mod = await import(/* @vite-ignore */ url);
  const events = globalThis.wails?.Events ?? mod.Events;
  if (!events?.On || !events?.Emit) {
    throw new WireError(NO_HOST, { endpoint: url }, `加载 ${url} 之后还是没有 Events.On／Events.Emit 这两个口`);
  }
  return events;
}

/** 壳→页面那一句的形状在两侧各写一次的可能只有一份真源，但"一次点击递几个参数"这件事 Wails 自己会变，
    所以这里把两种都认下来：单个对象，或只装着一个对象的数组。 */
function unwrap(data) {
  return Array.isArray(data) && data.length === 1 ? data[0] : data;
}

export function createWire({ loadRuntime = defaultLoadRuntime, timers } = {}) {
  const clock = timers || { set: (fn, ms) => setTimeout(fn, ms), clear: id => clearTimeout(id) };

  /** 这一条线此刻的形状，与 contract.LinkState 同一份四栏 */
  let link = { connected: false, dropped: 0, endpoint: '', err: '' };
  /** 页面这一侧的编号：0 留给"没编上号"，第一条真实调用从 1 开始 */
  let seq = 0;
  const pending = new Map();
  /** 业务事件的订阅者：name → 一组处理函数 */
  const handlers = new Map();
  let runtime = null;
  let attachPromise = null;
  /** 第一次对不上的版本号记下来。留着它是因为"这一条没回来"与"两侧不是同一轮构建"要分开说 */
  let versionMismatch = null;

  function fail(code, params, message) {
    return new WireError(code, params, message);
  }

  /** 接上宿主并装好三条线；只做一次，重复调用等同一个 Promise */
  function attach() {
    if (attachPromise) return attachPromise;
    attachPromise = (async () => {
      runtime = await loadRuntime();
      runtime.On(WIRE.reply, ev => onReply(unwrap(ev?.data ?? ev)));
      runtime.On(WIRE.state, ev => onState(unwrap(ev?.data ?? ev)));
      for (const name of Object.values(IPC_EVENTS)) {
        runtime.On(name, ev => dispatch(name, unwrap(ev?.data ?? ev)));
      }
      return runtime;
    })().catch(err => {
      // 加载失败要把这次尝试作废，否则"壳后来起来了"这一档永远接不上
      attachPromise = null;
      link = { ...link, connected: false, err: `${NO_HOST}: ${err?.message ?? err}` };
      throw fail(NO_HOST, { endpoint: link.endpoint }, err?.message);
    });
    return attachPromise;
  }

  function dispatch(name, event) {
    for (const fn of handlers.get(name) || []) {
      fn(event && typeof event === 'object' ? event : { name, snapshot: null });
    }
  }

  function onState(frame) {
    if (!frame || typeof frame !== 'object') return;
    link = {
      connected: !!frame.connected,
      dropped: Number(frame.dropped) || 0,
      endpoint: frame.endpoint || '',
      err: frame.err || ''
    };
    if (link.connected) {
      // 线重新通了：这一轮之前在飞的那几条永远等不到回执（壳那侧的连接已经换了一条），
      // 全部按"连不上"结掉，让点下去的人当场得到一句回话，而不是干等 12 秒。
      // 定时器要一起拆掉：只结回执不拆表，那条线上会留一串 12 秒后才空转一次的计时器。
      for (const [, entry] of pending) {
        clock.clear(entry.timer);
        entry.reject(fail(NO_HOST, { endpoint: link.endpoint }));
      }
      pending.clear();
    }
    for (const fn of handlers.get(WIRE.state) || []) fn(link);
  }

  function onReply(reply) {
    if (!reply || typeof reply !== 'object' || typeof reply.id !== 'number') {
      link = { ...link, err: `${BAD_REPLY}: 收到的回执不是合法形状` };
      return;
    }
    const entry = pending.get(reply.id);
    if (!entry) return;              // 迟到的回执：那一次调用已经超时结掉了，丢掉是对的
    pending.delete(reply.id);
    clock.clear(entry.timer);

    if (reply.protocol !== EXPECTED_PROTOCOL) {
      versionMismatch = { got: reply.protocol, want: EXPECTED_PROTOCOL };
      const text = `界面按 v${EXPECTED_PROTOCOL} 写，壳回的是 v${reply.protocol}`;
      // 同一句也写进 link 那四栏：状态栏是"这一判还会一直在"的唯一落点，
      // 只让点下去的人看见一次，下一个不点按钮的人就永远看不出两侧不是同一轮构建
      link = { ...link, err: `${SHAPE}: ${text}` };
      entry.reject(fail(SHAPE, versionMismatch, text));
      return;
    }
    if (reply.ok) {
      entry.resolve(reply.result);
      return;
    }
    // 服务那侧这一族只回一句裸消息（bridge.go 的 errLinkDropped），端点是这条线上的数，只有这里知道：补进去
    const params = reply.code === NO_HOST
      ? { ...reply.params, endpoint: reply.params?.endpoint || link.endpoint }
      : reply.params;
    entry.reject(fail(reply.code || BAD_REPLY, params, reply.err));
  }

  /** 问一句、等那一句的回执。没接上宿主时先接一次；接不上就当场回一句"连不上"，不许装作服务答了。 */
  async function call(method, params) {
    if (!Object.values(METHODS).includes(method)) {
      throw fail(SHAPE, { method }, `契约里没有这个方法名：${method}`);
    }
    if (versionMismatch) {
      throw fail(SHAPE, versionMismatch, `界面与外壳不是同一轮构建（v${EXPECTED_PROTOCOL} 对 v${versionMismatch.got}）`);
    }
    if (!runtime) await attach();
    seq += 1;
    const id = seq;
    return new Promise((resolve, reject) => {
      const timer = clock.set(() => {
        if (!pending.has(id)) return;
        pending.delete(id);
        reject(fail(TIMEOUT, { method }, `等了 ${REPLY_TIMEOUT_MS / 1000} 秒没等到「${method}」的回执`));
      }, REPLY_TIMEOUT_MS);
      pending.set(id, { resolve, reject, timer });
      try {
        runtime.Emit(WIRE.invoke, { id, method, params: params ?? {} });
      } catch (err) {
        pending.delete(id);
        clock.clear(timer);
        reject(fail(NO_HOST, { endpoint: link.endpoint }, err?.message));
      }
    });
  }

  /** 订阅一条线／一个事件。名字只准从 WIRE 或 IPC_EVENTS 里取，写了别的名字当场抛——
      拼错一个字母不会报错、只会永远收不到的那种漂，就在这一步拦掉。 */
  function on(name, fn) {
    const known = new Set([...Object.values(WIRE), ...Object.values(IPC_EVENTS)]);
    if (!known.has(name)) throw new Error(`wire：「${name}」不在那三条线与那 5 个事件里，订阅了也永远收不到`);
    if (!handlers.has(name)) handlers.set(name, new Set());
    handlers.get(name).add(fn);
    return () => handlers.get(name)?.delete(fn);
  }

  return {
    attach,
    call,
    on,
    linkState: () => ({ ...link }),
    /** 界面拿来判"这页现在到底在不在壳里跑"：宿主找不到与壳连不上是两句话 */
    hasHost: () => !!runtime,
    versionMismatch: () => versionMismatch
  };
}

/** 全站那一条线：main.js 用它，用例自己 new 一条假的。
    为什么只有一条：一次点击要能对上回执，页面里跑两条线就等于把编号存两处。 */
export const wire = createWire();
