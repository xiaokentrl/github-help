/* 错误码 → 那一句人话。干什么用：服务与通道只回「码＋参数」（已定：D6 乙），
   界面上每一句"为什么没成"都在这里按码换成 zh-CN.js 的 `err.<码>` 那一句；
   ERROR_PARAMS 是这一族码的参数名单，它是 internal/errs/codes.go 与语言文件之间的那道闸。
   会错在哪：① 码在表里没有 → 界面显示成 ⟦err.GAC-XXX⟧，比显示一个裸码好一点，但翻译的人会漏整条；
             ② 句子里用了这个码根本没有的参数 → 那句 `{path}` 原样留在界面上（t() 少给参数时不许悄悄换成 undefined）；
             ③ Go 那边新增一个码而这里没跟着加 → 用户撞到它时只剩一句"未知错误"，那正是最难查的一档。
   怎么回来：门禁 frontend/tests/errors.test.mjs（后端目录规划第 15 节那条 G13）现读 internal/errs/codes.go，
             把三个方向都查一遍：码的集合等值（Go ↔ 这一份 ↔ zh-CN 的 err.*，两头都数）、
             句子里的占位符都在名单里、名单里的键名都在 Go（或 wire.js）真给过；
             只有 LOCAL_FILL 里登记的码准用"Go 那侧给不齐"的那几栏，用的时候当场保证有一个兜底值。
             新增一种语言＝在 locales/ 里加一个同键名文件，这一份表不用动。 */

/** 每个码能拿到的参数名。空数组＝这一族码回的时候不带参数，句子里也就不许用占位符。
    带「界面」注释的那几条是 wire.js 本地造的错（那条线根本没通到服务），Go 那侧同名码给的是裸消息。 */
export const ERROR_PARAMS = {
  'GAC-IPC-001': [],          // 通道上收到的行不是合法 JSON
  'GAC-IPC-002': [],          // 请求没有方法名
  'GAC-IPC-003': [],          // 回执写不出去
  'GAC-IPC-010': [],          // 对端身份核不过
  'GAC-IPC-011': [],          // 服务不认这个方法
  'GAC-IPC-012': [],          // 方法跑挂了（兜底码）
  'GAC-IPC-020': [],          // 建不出运行时目录
  'GAC-IPC-021': [],          // 监听失败
  'GAC-IPC-022': [],          // 端点权限没改成 0600
  'GAC-IPC-023': [],          // 旧端点清不掉
  'GAC-IPC-030': [],          // 本平台通道未实现
  'GAC-IPC-040': ['endpoint'], // 连不上后台服务：Go 侧给裸消息，界面这一侧带上下一条线连的是哪个文件
  'GAC-IPC-041': [],          // 等回执超时
  'GAC-IPC-042': [],          // 收到的回执不是合法形状
  'GAC-IPC-043': [],          // 两侧契约版本对不上

  'GAC-HOSTS-001': ['path'],
  'GAC-HOSTS-002': [],
  'GAC-HOSTS-003': ['path', 'dir', 'hosts'],
  'GAC-HOSTS-004': ['domain', 'want', 'got', 'itemId', 'path'],
  'GAC-HOSTS-005': ['path', 'backup'],
  'GAC-HOSTS-006': [],
  'GAC-HOSTS-007': ['domain', 'items', 'heldBy', 'itemId', 'count'],
  'GAC-HOSTS-008': ['itemId', 'values', 'domain', 'ip'],
  'GAC-HOSTS-009': ['domain', 'itemId', 'line', 'otherNames'],
  'GAC-HOSTS-010': [],
  'GAC-HOSTS-011': [],

  'GAC-SETTINGS-001': [],
  'GAC-SETTINGS-002': ['path'],
  'GAC-SETTINGS-003': ['path', 'cause'],
  'GAC-SETTINGS-004': ['path'],

  'GAC-DETECT-001': ['itemId', 'mode', 'domain'],
  'GAC-DETECT-002': ['mode', 'limit'],
  'GAC-DETECT-003': [],
  'GAC-DETECT-004': [],
  'GAC-DETECT-005': ['fetches', 'fetchLimit', 'probes', 'probeLimit'],
  'GAC-DETECT-006': [],

  'GAC-PERM-001': ['path', 'cause'],
  'GAC-PERM-002': [],

  'GAC-TASK-001': ['taskID', 'kind'],
  'GAC-TASK-002': ['taskID', 'waiting'],
  'GAC-TASK-003': ['path', 'mode'],
  'GAC-TASK-004': ['itemId'],

  'GAC-UPDATE-502': ['latest', 'platform'],
  'GAC-UPDATE-503': ['url'],

  'GAC-UNKNOWN-000': ['code', 'message']   // 兜底那一句：这两个数由本文件补，不是服务给的
};

/** 认不出来的码——包括 GAC-MODE-<方式id>-xxx 那一族（它由 errs.ModeCode() 现场拼，故意不进 codes.go 的清单）。
    为什么不能静默用原码去查句子：那一查得到的是 ⟦err.GAC-MODE-xxx⟧，用户看到的是一串键名，
    而"这条码没登记"这个事实本身才是该说的事。 */
const UNKNOWN = 'GAC-UNKNOWN-000';

/** 由界面这一侧补上的那几栏参数。干什么用：句子用了 {endpoint}，可这个数不是服务给的——
   服务那侧这一族只回一句裸消息（internal/ipc/client.go 的 ErrNoConnection、cmd/gui/bridge.go 的 errLinkDropped）。
   什么情况下会错、看得见什么：拿不到端点又不留兜底，屏幕上就是一句"连不上后台服务：。"，
   比露出 {endpoint} 更像界面坏了。
   怎么回来：这一份名单由 tests/errors.test.mjs 逐条查——只有登记在这里的码才准用"Go 现场给不齐"的参数。 */
const LOCAL_FILL = {
  'GAC-IPC-040': { endpoint: '那条线还没建起来' },
  'GAC-UNKNOWN-000': { code: UNKNOWN, message: '（服务没给原文）' }
};

export function errorKey(code) {
  return `err.${ERROR_PARAMS[code] ? code : UNKNOWN}`;
}

/** 把「码＋参数」换成界面上那一句。t 是 core/i18n/t.js 造的那支函数。
    params 里缺哪个占位符就留原样（t.js 的行为），所以句子里的占位符必须先在 ERROR_PARAMS 里登记——
    这一条由 tests/errors.test.mjs 逐码查。 */
export function errorText(t, code, params = {}, message = '') {
  const known = ERROR_PARAMS[code];
  if (!known) {
    return t(errorKey(code), { code: code || UNKNOWN, message: message || String(code || '') });
  }
  const given = params || {};
  const fallbacks = LOCAL_FILL[code] || {};
  const merged = {};
  for (const name of known) {
    const raw = given[name];
    // 空串与没给是一回事：屏幕上都不能留一个空括号
    const value = raw === undefined || raw === '' ? fallbacks[name] : raw;
    if (value !== undefined) merged[name] = value;
  }
  return t(`err.${code}`, merged);
}

/** 从一条 WireError／服务回执里取出码与参数，换成句子。调用点不必各自判形状——
    没有码就当未知处理，至少界面上是一句人话加一个键名，而不是空白。 */
export function replyErrorText(t, err) {
  return errorText(t, err?.code, err?.params ?? {}, err?.message ?? '');
}
