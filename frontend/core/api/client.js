/* 界面发问的唯一出口。干什么用：全项目只有这一层把"人点了什么"翻成契约里那 16 个方法名之一，
   再由 core/api/wire.js 送出去；外面（core/flows.js、main.js、views/settings）只准叫这里的动词，不准碰方法名字符串。
   会错在哪：① 别处直接 wire.call('Detect') 或写 window.go.xxx＝同一个方法名长了第二个家，
                契约改了只改得动一处，症状是那颗按钮点下去回一句 GAC-IPC-011（看着像"服务没这个功能"）。
             ② 参数键名漂了不报错：服务把缺的键当"没给"，Detect 少了 itemIDs 就变成"全部检测"，
                用户点一条域名结果 54 条一起重测。所以这一层的每个键名都是照 internal/service/handlers.go 抄的，
                由 tests/contract.test.mjs 现读那一份源码比对。
             ③ 四个异步方法（Detect／ApplyQueue／RestoreItem／RestoreHosts）回的是任务号，不是结果——
                结果随后一条条从事件里带完整快照推回来。在这里"顺手 await 到跑完"＝契约那句"立刻返回任务号"作废，
                界面上进度事件全没了，队列那一排一直停在点下去那一刻。
   怎么回来：加一个方法＝在 core/contract/methods.js 加一条名字、这里加一个动词、internal/contract 那一份加一条常量，
             三处同一轮改完（少一处都有用例当场红）。数据进来之后怎么画，家在 core/api/snapshot.js。 */

import { wire } from './wire.js';
import { METHODS, WIRE } from '../contract/methods.js';
import { IPC_EVENTS } from '../contract/events.js';

/** 只有这一个出口能碰方法名：写错一个字母当场抛，而不是发出去没人答、等 12 秒回一句超时。
   为什么要拦在调用这一层：动词名（getStatus）与契约名（GetStatus）是两套，只对得上一次就少一处对账。 */
function call(verb, params) {
  const method = METHODS[verb];
  if (!method) throw new Error(`client：「${verb}」在 core/contract/methods.js 里没有这一条，界面发不出去`);
  return wire.call(method, params);
}

export const api = {
  /* ---- 纯读：问一次答一次，不进队列、不改任何文件 ---- */

  /** 契约 GetSnapshot：开机与断线重连之后那一份"此刻全貌"的唯一来源 */
  getSnapshot: () => call('getSnapshot', {}),
  /** 契约 GetStatus：服务状态那一份 ＋ 写入串行锁此刻有没有人拿着 */
  getStatus: () => call('getStatus', {}),
  /** 契约 CheckPermissions：那盏权限灯要当场亮，等不起一轮检测 */
  checkPermissions: () => call('checkPermissions', {}),
  /** 契约 GetVersion：版本号与构建日期（「关于我们」那两格） */
  getVersion: () => call('getVersion', {}),
  /** 契约 GetSettings（真家在服务那侧的文件）：开机问完前四件事之后由 main.js 的 pullSettings() 读回来覆盖本地缓存，
      本地还有没送出去的改动时这一问要让路（不然把人刚改的那几格盖回去） */
  getSettings: () => call('getSettings', {}),
  /** 契约 SaveSettings，参数键名是 handlers.go 的 params["settings"]；
      出口只有一个——main.js 的 persist() 延迟 600 毫秒送这一问，两把把手与设置中心那颗「保存」都走它 */
  saveSettings: settings => call('saveSettings', { settings }),

  /* ---- 四个异步方法：只回任务号，过程靠事件 ---- */

  /** 契约 Detect，收 {itemIDs}；不给＝全部检测（顶部那颗「检测全部」那一档） */
  detect: itemIDs => call('detect', itemIDs?.length ? { itemIDs } : {}),
  /** 契约 ApplyQueue，收 {itemIDs, picks}；picks 是"这一条用候选里那一个地址"（{条目id: IP}，#53 接线） */
  applyQueue: (itemIDs, picks) => call('applyQueue', {
    ...(itemIDs?.length ? { itemIDs } : {}),
    ...(picks && Object.keys(picks).length ? { picks } : {})
  }),
  /** 契约 RestoreItem，收 {itemID}（一条） */
  restoreItem: itemID => call('restoreItem', { itemID }),
  /** 契约 RestoreHosts：无参数，撤掉全部托管行（服务自己展开成"每一条一行"） */
  restoreHosts: () => call('restoreHosts', {}),

  /* ---- 队列的三只手 ---- */

  /** 契约 OpenURL，收 {url}；服务那侧的守卫只放 http(s)。成功回 {opened:true}，
      打不开（scheme 不对／打开器没跑成）回 GAC-IPC-013，调用点拿码去语言文件换句子 */
  openURL: url => call('openURL', { url }),

  /** 契约 CancelTask，收 {taskID}；不填＝撤掉所有等待中的行（顶部那颗「取消等待」） */
  cancelTask: taskID => call('cancelTask', taskID ? { taskID } : {}),
  /** 契约 PauseQueue：关闸，回的是闸此刻的状态 */
  pauseQueue: () => call('pauseQueue', {}),
  /** 契约 ResumeQueue：开闸；只有这里能关掉那一个闸（L2 那句"要点两下"的家） */
  resumeQueue: () => call('resumeQueue', {}),
  /** 契约 ClearTasks：抹掉跑完的那几行，回抹完之后那一份完整快照（与 GetSnapshot 同一条合并路径） */
  clearTasks: () => call('clearTasks', {}),

  /** 契约 CheckUpdate／DownloadUpdate：服务那边这一档如实回一句"本期不提供"（GAC-UPDATE-501），
      这里不替它编进度。 */
  checkUpdate: () => call('checkUpdate', {}),
  downloadUpdate: () => call('downloadUpdate', {}),

  /* ---- 通道本身：不是契约方法，但同样只准从这一层过 ---- */

  /** 接上壳那一条线（找不到宿主就抛 GAC-IPC-040）。
      为什么挂在 api 上而不是单独 export 一支：动词层是界面唯一准碰通道的那一层，
      开一个"main.js 自己 attach"的口子，下一个直接 call 也会跟着来。 */
  attach: () => wire.attach(),
  /** 这条线此刻通不通，只用来在人话里补一句"连的是哪个文件"；业务状态一律问 getStatus。 */
  linkState: () => wire.linkState()
};

/** 订阅那 5 个业务事件与那条连接的状态线；名字都从契约那两份清单取，这里不许再写一遍字符串。
   handlers 用事件名（不是动词名）当键：queueProgress／queueDone／downloadProgress／networkChange／detectProgress。 */
export function subscribe({ onEvent, onLink }) {
  const offs = [];
  if (onEvent) {
    for (const [key, name] of Object.entries(IPC_EVENTS)) {
      const handler = onEvent[key];
      if (handler) offs.push(wire.on(name, handler));
    }
  }
  if (onLink) offs.push(wire.on(WIRE.state, onLink));
  return () => offs.forEach(off => { try { off(); } catch { /* 解绑失败不影响界面 */ } });
}
