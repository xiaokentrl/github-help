/* 快照进界面的那一步：服务回的整份状态 → store 里那一份，只有这一个入口。干什么用：
   界面看到的每一行（54 条域名、队列那几张表、状态栏那几格）都是这里从 contract.Snapshot 抄出来的；
   上一轮数据来自 core/api/mock.js（前端自己造的假结果），从这一份起，画面只认服务回的那一份。
   会错在哪：
   ① 别处再抄一遍字段 → 服务加一栏、事件那一处改了、开机那一次没改，症状是"点完才画得出来，刷新一下又是空的"。
      所以 GetSnapshot、每一条事件、ClearTasks 的回执三条路都走 mergeSnapshot，一个也不许自己拼。
   ② 条目对象整个换新 → 树上那一行的勾选、选中态跟着丢（服务不认识"勾选"这件事，那是界面自己的）。
      所以这里是就地改写：先按 id 找到界面上那一个对象，只盖服务管的字段。
   ③ 拿服务那句中文原文当界面要显示的话 → 切语言时那一列不动（已定：D6 乙，服务只回码与参数）。
      所以 errorMsg／状态栏那一句都由 errorText(码＋参数) 在语言文件凑；原文只在日志侧看。
   ④ 把没有的值当 0 画 → "延迟 0 ms""耗时 0.0s"都是假读数。空一律留空串，渲染那侧按 — 画。
   怎么回来：形状的真源是 internal/contract/contract.go，界面这一侧的镜像是 core/contract/types.d.ts，
   两边的比对在 tests/contract.test.mjs；这一份自己的合并规则由 tests/snapshot.test.mjs 逐条钉。 */

import { PHASES, RESTORE_PHASES } from '../meta.js';
import { errorText } from './errors.js';
import { clockFromIso, durationFromMs } from '../../utils/format.js';

/** 队列那一行「正在跑的那一步」的小字：应用与恢复走的是两套阶段名（core/meta.js 那两份），种类决定用哪一套 */
function phaseText(kind, phase) {
  if (!phase) return '';
  const list = kind === 'restore' ? RESTORE_PHASES : PHASES;
  const hit = list.find(p => p.key === phase);
  return hit ? hit.text : '';
}

/** 界面上 54 条的初态。干什么用：开机先按菜单清单把行建出来，第一次 GetSnapshot 再填真读数。
    会错在哪：初态里放假地址（上一版 mock 的 init 那么干）→ 服务还没答话，屏幕上已经写着"已替换"。
    所以这里一律未设置；init 那个字段只拿来决定"这条默认勾上不勾上"，别的什么都不决定。 */
export function buildTree(menus) {
  return menus.map(m => ({
    id: m.id,
    name: m.name,
    items: m.items.map(def => ({
      id: def.id,
      name: def.name,
      domains: def.domains.slice(),
      checked: def.init === 'applied' || def.init === 'stale',
      status: 'unset',
      appliedIp: null,
      prevAppliedIp: null,
      bestIp: null,
      managed: false,
      latency: 0,
      speed: '',
      score: 0,
      source: '',
      sourceRole: '',
      lastCheck: '',
      candidates: [],
      errorMsg: ''
    }))
  }));
}

/** contract.Task → 队列那一行。服务给的是这一行的全部真相：状态、阶段、耗时、失败的码与参数。 */
export function taskRow(tk, t) {
  return {
    id: tk.id,
    kind: tk.kind,
    itemId: tk.itemId || '',
    name: tk.name || '',
    status: tk.status,
    phase: tk.phase || null,
    phaseText: phaseText(tk.kind, tk.phase),
    processed: tk.processed || 0,
    total: tk.total || 0,
    duration: durationFromMs(tk.durationMs),
    errorMsg: tk.errorCode ? errorText(t, tk.errorCode, tk.errorParams) : ''
  };
}

/** 一条条目的字段就地改写。只盖服务管的这些，界面自己管的（勾选、候选表那几列还没进契约）一个不碰。 */
function applyItem(cur, it, t) {
  cur.name = it.name || cur.name;
  cur.status = it.status || 'unset';
  // 托管凭据只有一个来源：文件里有没有我们写的那几行（界面上 ↺ 点不点得动就认这一栏，D3 甲）
  cur.managed = !!it.managed;
  cur.appliedIp = it.appliedIp || null;
  cur.prevAppliedIp = it.prevAppliedIp || null;
  cur.bestIp = it.bestIp || null;
  cur.source = it.source || '';
  cur.sourceRole = it.sourceRole || '';
  cur.latency = it.latency || 0;
  cur.lastCheck = clockFromIso(it.lastCheck);
  cur.errorMsg = it.errorCode ? errorText(t, it.errorCode, it.errorParams) : '';
}

/** 此刻有没有还在跑的行。等待与正在跑都算，restoring 也算（那是恢复那一行的"正在跑"）。
    为什么在这一处判：界面上"这会儿能不能点"问的是忙标志，而忙标志的唯一来源应该是服务那张表——
    上一版是前端自己一条条 await 出来的，接了真服务之后就再没有那回事了。 */
function isActive(tk) {
  return tk.status === 'waiting' || tk.status === 'running' || tk.status === 'restoring';
}

/** 把一份快照并进 store。三条来源（开机那一次 GetSnapshot、每一条事件、ClearTasks 的回执）都走这一个函数。 */
export function mergeSnapshot(state, snapshot, t) {
  if (!snapshot || typeof snapshot !== 'object') return state;
  const byId = new Map();
  for (const cat of state.categories) for (const item of cat.items) byId.set(item.id, item);

  let newestAt = 0;
  let newestIso = '';
  for (const it of snapshot.items || []) {
    const cur = byId.get(it.id);
    if (!cur) continue;   // 清单里没有这一条：宁可少画一行，也不凭空造一行
    applyItem(cur, it, t);
    const at = Date.parse(it.lastCheck || '');
    if (Number.isFinite(at) && at > newestAt) {
      newestAt = at;
      newestIso = it.lastCheck;
    }
  }
  // 状态栏「上次检测」取的是所有条目里最近那一次——服务没说"上一轮什么时候跑的"，只有每条自己的时间
  state.lastDetect = clockFromIso(newestIso);

  // 断网判定（netwatch）此刻的读数：状态栏第一格照它画。快照没带（旧服务）就是"没这回事"，不乱猜。
  state.networkOffline = !!snapshot.networkOffline;

  state.queue = (snapshot.tasks || []).map(tk => taskRow(tk, t));

  const st = snapshot.state || {};
  state.paused = !!st.queuePaused;
  state.appStatus.dnsService = st.dnsService || t('common.dash');
  state.appStatus.dataDir = st.dataDir || '';
  state.appStatus.hostsPath = st.hostsPath || '';
  // 状态栏那一句"为什么"（契约 v9 的码＋参数）。原文留在 text 里，界面上说的是 errorText 凑出来的那一句。
  state.lastError = {
    code: st.lastErrorCode || '',
    params: st.lastErrorParams || {},
    text: st.lastError || ''
  };

  refreshBusy(state);
  return state;
}

/** 忙标志只有这一个合成算法：界面上的「正在批量检测／队列正在执行」＝ 服务那张表里还有没有这一类的行在跑 ＋
    那一次请求有没有回话（pending 把手）。
    把手归 core/flows.js 的 dispatch 一家：抬起与松开都在那里（＝那一次调用回没回来），这里一只也不碰。
    这里为什么不再写"看到活动行就松""整张表空了就松"：把手抬起来的那一刻服务那张表本来就是空的（任务还没建出来），
    后一条会把它当场按下去，C2 那一段又露出来；而事件有可能比回执早到也有可能永远排在回执之后，
    前一条会留下"请求成功了、把手永远抬着"的卡死。两条都不如"回话了就松"这句话准。
    线断了（connected 为假）一律算不忙：那张表不会再更新了，继续报"正在执行中"就是把用户锁在屏幕前面。 */
export function refreshBusy(state) {
  const down = state.connected === false;
  const detecting = !down && state.queue.some(tk => tk.kind === 'detect' && isActive(tk));
  const queueing = !down && state.queue.some(tk => (tk.kind === 'apply' || tk.kind === 'restore') && isActive(tk));
  state.batchDetecting = !down && (state.pending.detect || detecting);
  state.queueRunning = !down && (state.pending.queue || queueing);
  return state;
}
