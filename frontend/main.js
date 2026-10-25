/* 全站接线：把外框、数据、流程、渲染器接到一起，并把启动那一下的顺序钉死。干什么用：
     读缓存设置 → 应用外观与布局 → 造数据 → 建三个渲染器 → 接上每颗钮和每个快捷键 → 首次渲染
     → 问服务（含设置的真家）→ 补画 → 跑启动自动化（目录规划第 2 节那条启动顺序，对应原型 :3352-3362；
     自动化那一趟从"开机 500 毫秒后瞎跑"改成"服务答过话、设置拉回来之后才跑"，理由见第 9 节那一段）。
   会错在哪：① 这里是目前唯一允许直接摸外框 id 的地方（views 只收容器节点，件只收数据）。所以取不到某个 id
                就当场抛错并说出是哪个：留着 null 往下跑，表现是"树是空的但没报错"，最难查。
             ② 钮的禁用条件、勾选的算法、折叠的算法都不在这儿写——全部问 core/store.js 与 core/task/gate.js。
                在这儿再写一遍就是 C2 那笔账（同一个"执行中不许点"散在七八处，漏一处两个写 hosts 的动作同时开工）。
             ③ 键盘那段（Ctrl+A／D／Enter）在确认框开着的时候必须整段跳过（原型 :2224、:3225 那两条 return）：
                否则按 Ctrl+A 会一边把框里的字全选、一边把 54 条域名全勾上。
             ④ 工具栏那条全局进度线、两道拖动把手由这里建：进度线交给 views/toolbar 贴到那一行末尾，
                把手插在 #sidebar 之后、#queuePanel 之前（外框只留容器，见外框注释⑤）。
                顶栏／横幅／工具栏／状态栏／侧栏脚／托盘胶囊这六块，2026-10-22 这一轮已经全部搬进 views，
                外框里再没有一句写死的中文——"新增一种语言＝只加一个语言文件"那句从此才有机器判据。
             ⑤ 设置有两个家，这一层是唯一把它们对上的人：缓存（localStorage）先画对，真家在服务那一份文件里。
                界面上关了开机开关而服务不知道，下次开机就还是照写 hosts——所以 persist() 一定要把改动送出去，
                pullSettings() 一定要把真家拉回来，两句话都在这一层，面板不许自己发请求。
   怎么回来：这一轮把数据源从 core/api/mock.js 换成了真服务：树里那 54 条由 menu/ 的清单建出行、
             读数一律来自 GetSnapshot（core/api/snapshot.js 那一条合并路径），队列的进度来自那 5 条事件，
             状态栏那三格与权限灯来自 GetStatus／CheckPermissions，版本号来自 GetVersion——开机那一下由 flows.start() 一次问完。
             假服务（mock.js）已随这一轮删除：留着就是"界面有时候说的是假话"。
             设置这一轮接上了真回执：GetSettings 拉、SaveSettings 送（#51 那一笔账到此清掉）。
             还没接的两处照实登记：下载进度那一条事件（download:progress）今天还没有转发——
             更新那两格只如实回一句「本期不提供」（GAC-UPDATE-501），不替它编进度；
             托盘那两条动作只回一句提示（还没接系统托盘那条通道，见 locales/zh-CN.js 的 tray.* 那段注释），
             连带那四个只有壳能做的开关（trayMinimize／showTrayIcon／autoStart／startMinimized）
             也只做到"记下来"，保存那一下会如实说明，名单的家在 core/settings.js 的 SHELL_KEYS。 */

import './styles/index.js';

import { menus } from './menu/index.js';
import { themes } from './styles/themes/index.js';
import { locales } from './core/i18n/locales/index.js';
import { createI18n } from './core/i18n/t.js';
import {
  loadSettings, saveSettings, normalize, SIDEBAR_MIN_W, SIDEBAR_MAX_W, QUEUE_MIN_H, queueMaxHeight
} from './core/settings.js';
import {
  state, resetState, findItem, findCategory, categoryCheckState,
  allCheckedState, allCollapsedState, setAllChecked, invertAll, toggleCategoryCollapsed
} from './core/store.js';
import { api, subscribe } from './core/api/client.js';
import { buildTree } from './core/api/snapshot.js';
import { replyErrorText } from './core/api/errors.js';
import { createFlows } from './core/flows.js';
import { createTreeView } from './views/menu/TreeView.js';
import { createDetailView } from './views/detail/DetailView.js';
import { createQueueView } from './views/queue/QueueView.js';
import { createTopbarView } from './views/topbar/TopbarView.js';
import { createBannerView } from './views/banner/BannerView.js';
import { createToolbarView } from './views/toolbar/ToolbarView.js';
import { createStatusbarView } from './views/statusbar/StatusbarView.js';
import { createSidebarFooterView } from './views/sidebar-footer/SidebarFooterView.js';
import { createTrayPillView } from './views/tray-pill/TrayPillView.js';
import { createSettingsView } from './views/settings/SettingsView.js';
import { panes } from './views/settings/panes/index.js';
import { createThemePickerView } from './views/theme-picker/ThemePickerView.js';
import { languageOptions } from './core/i18n/languages.js';
import { ProgressBar } from './components/progress-bar/ProgressBar.js';
import { DragHandle } from './components/drag-handle/DragHandle.js';
import { Toast } from './components/toast/Toast.js';
import { confirmDialog } from './components/confirm-dialog/ConfirmDialog.js';

/** 表里没有这份语言时用它兜底——设置里存的是用户上次选的那门语言，不能让它把界面弄崩（t.js 抛的就是这种错） */
const FALLBACK_LANG = 'zh-CN';
const SEARCH_DEBOUNCE_MS = 150;   // 原型 :3097：搜索每敲一个字就重画整棵树会卡

function byId(id) {
  const node = document.getElementById(id);
  if (!node) throw new Error(`外框 index.html 里没有 id="${id}" 这个容器——本文件只认外框留的那几个壳，少一个就画不出那一块`);
  return node;
}

/* ============ 1. 读设置（原型 :3352） ============ */

resetState({ settings: loadSettings(themes.map(th => th.id)) });

const t = createI18n(locales, {
  lang: locales[state.settings.language] ? state.settings.language : FALLBACK_LANG,
  fallback: FALLBACK_LANG
});

/* 设置有两个家，得说清哪个是真的：服务那一份文件（internal/settings）是真家——高权限服务开机读的、
   开机自动检测／自动应用那两把开关起作用的都是它；这一份 localStorage 只是"服务还没答话时先把界面画对"的缓存。
   干什么用：改一格 → 立刻缓存（界面不能等）→ 攒 600 毫秒送一次契约 SaveSettings。
   会错在哪：① 只缓存不送：界面上把两把开机开关关了，服务那侧还是默认值，下次开机照样自提 54 条的应用批——
                那是用户以为不许、hosts 却照写。这一条本轮补上（契约的 SaveSettings 从没人调用过，就是这个洞）；
             ② 每改一档发一次：拖动侧栏宽那种连续改动会把设置文件写爆，服务那侧的写还要串行排队。
                所以合并成一发；拖动把手本来只在松手那次动 state，这一层是防下一个"实时改设置"的面板；
             ③ 送失败不吭声：界面已经改了、服务没记下，下次开机说的是服务那一份——
                不说＝"设置丢了但没人认账"。所以失败要说一句人话；
                不回滚界面上那几个数：那是人刚点下去的选择，倒回旧值反而像"点错了"，
                要说清的是"这一份还没送到服务那里"，下一次改动会连带把它补送出去。
   怎么回来：动词只有这一个出口（设置中心那颗「保存」与两道把手都叫它）。
             pushPending 那句是给 pullSettings() 看的：本地有还没送出去的改动时不再拉，
             拉回来就把人刚改的那几格盖掉了。 */
const SETTINGS_PUSH_DEBOUNCE_MS = 600;
let pushTimer = null;

function persist() {
  saveSettings(state.settings);
  clearTimeout(pushTimer);
  pushTimer = setTimeout(() => {
    pushTimer = null;
    api.saveSettings({ ...state.settings }).catch(err => {
      toast.show({ text: replyErrorText(t, err), kind: 'error' });
    });
  }, SETTINGS_PUSH_DEBOUNCE_MS);
}

/** 「送出后还没回话」＝true：开机那一次拉设置要先让路，别把人刚改的那几格盖掉 */
const pushPending = () => pushTimer !== null;

/* ============ 2. 外观与布局（原型 applyLayoutFromSettings :1490-1498） ============ */

/** 把 state.settings 里那四个"看一眼就知道生效了"的数刷到界面上：主题、缩放、侧栏宽、队列高。
    开机走它一次，设置中心每改一档走它一次——只有这一个出口，别处再写就出现"面板改了、界面还旧"。 */
function applyNow() {
  // 主题就是 html[data-theme] 这一个属性；色板全在 styles/themes/<id>/theme.css 里（core/settings.js 已按扫出来的名单校过）
  document.documentElement.dataset.theme = state.settings.theme;
  document.body.style.zoom = String(state.settings.zoom);
  sidebarEl.style.width = `${state.settings.sidebarWidth}px`;
  // 队列折叠着的时候不动它的高度（原型 :1492 那句判的就是这个：折叠态下改高度看不出来，也不该撑开）
  if (!state.queueCollapsed) queuePanel.style.height = `${state.settings.queueHeight}px`;
}

const sidebarEl = byId('sidebar');
const queuePanel = byId('queuePanel');
applyNow();

// 原型 :3253-3254：这台机器是 Linux 就给 body 挂 reduce-motion（webkit2gtk 那侧的合成器跟不上动画）
const ua = navigator.userAgent || '';
if (/Linux/i.test(ua) && !/Android/i.test(ua)) document.body.classList.add('reduce-motion');

/* ============ 3. 数据：菜单清单 → 树里那 54 行（原型 buildData :3256） ============ */

/* 这里只建行，不给读数：上一版由 mock 的 init 把假地址与"已替换"直接画进树里，服务还没答话屏幕上就演完了。
   现在初态一律"未替换"，init 那个字段只决定这一条默认勾不勾上；真读数等 flows.start() 那一次 GetSnapshot。 */
resetState({ categories: buildTree(menus) });
// 原型 :3257：打开时全部类目是折叠的（54 条一次性摊开没人看得完）
state.categories.forEach(cat => state.collapsed.add(cat.id));

/* ============ 4. 三个件：进度线、两道把手、气泡 ============ */

const globalProgress = ProgressBar({ kind: 'global' });

const sidebarHandle = DragHandle({
  axis: 'x', min: SIDEBAR_MIN_W, max: SIDEBAR_MAX_W,
  valueOf: () => state.settings.sidebarWidth,
  onResize: px => { sidebarEl.style.width = `${px}px`; },
  // 松手那一下才写盘（件里已经把拖动过程中的每帧分给 onResize 了）：拖一格存一次会把设置写爆
  onCommit: px => { state.settings.sidebarWidth = px; persist(); }
});
sidebarHandle.el.title = t('layout.sidebarTitle');
sidebarEl.after(sidebarHandle.el);

const queueHandle = DragHandle({
  axis: 'y', min: QUEUE_MIN_H, max: queueMaxHeight(window.innerHeight),
  valueOf: () => state.settings.queueHeight,
  onResize: px => { queuePanel.style.height = `${px}px`; },
  onCommit: px => { state.settings.queueHeight = px; persist(); }
});
queueHandle.el.title = t('layout.queueTitle');
queuePanel.before(queueHandle.el);

// 窗口变矮时把队列夹回可视范围（原型 :1446-1452：只夹界面，不改设置里那个数）
window.addEventListener('resize', () => {
  const max = queueMaxHeight(window.innerHeight);
  // 窗口矮到连最小队列高度都放不下时不收上下限：件里 min 必须严格小于 max（DragHandle 会抛），
  // 而且这种窗口高度下折叠队列是唯一的出路，界面本来也没地方夹
  if (max <= QUEUE_MIN_H) return;
  queueHandle.setBounds(QUEUE_MIN_H, max);
  if (state.queueCollapsed) return;
  const cur = queuePanel.offsetHeight;
  if (cur > max) queuePanel.style.height = `${max}px`;
  else if (cur < QUEUE_MIN_H) queuePanel.style.height = `${QUEUE_MIN_H}px`;
});

const toast = Toast();
document.body.appendChild(toast.el);

/* ============ 5. 渲染器（views 层那三块） ============ */

/** 流程要在渲染器之后才建，所以这些 handler 里引用的 flows 是"点下去那一刻才存在"的那个 */
let flows = null;

const tree = createTreeView({
  root: byId('tree'),
  t,
  handlers: {
    onSelect: id => { state.selectedId = id; renderAll(); },
    onToggleItem: id => {
      const item = findItem(id);
      if (item) item.checked = !item.checked;
      renderAll();
    },
    // 类目名与三角：折叠/展开只补画树（原型 :3047-3061 这里没走 renderAll，队列与状态栏不受折叠影响）。
    // 但那一趟 renderTree() 自己会顺手 syncToggleButtons()（原型 :1833），那颗"折叠↔展开"的字因此跟着树走，
    // 不管你是点三角逐个展开的还是点那颗钮一把展开的——本轮实测（#30）我们发现只补画树会把那钮的字留在旧档上，
    // 所以下面这一行不是可选的装饰：少了它，逐个展开之后字还写着"展开"，点下去却是在折叠。
    onToggleCollapse: id => { toggleCategoryCollapsed(id); tree.render(); toolbar.render(); },
    onToggleCategory: id => {
      const cat = findCategory(id);
      if (!cat) return;
      // 半选时再点＝全选（原型 :3036 那句 categoryCheckState(cat) !== 'checked'，判一次、整类同一个值）
      const next = categoryCheckState(cat) !== 'checked';
      cat.items.forEach(i => { i.checked = next; });
      renderAll();
    },
    onRestoreItem: id => { const item = findItem(id); if (item) flows.restoreItemHosts(item); },
    onRestoreCategory: id => { const cat = findCategory(id); if (cat) flows.restoreCategoryHosts(cat); }
  }
});

const detail = createDetailView({
  root: byId('detail'),
  t,
  handlers: {
    onDetectOne: item => flows.detectSingle(item),
    onApplyOne: item => flows.applySingle(item),
    onRestoreHosts: item => flows.restoreItemHosts(item),
    onViewLog: item => toast.show({ text: t('detail.logToast', { name: item.name }) })   // 原型 :1990
  }
});

const queue = createQueueView({
  root: queuePanel,
  progress: globalProgress,
  resizer: queueHandle.el,
  t,
  handlers: {
    onPause: () => flows.pauseToggle(),
    onCancel: () => flows.cancelWaiting(),
    onClearDone: () => flows.clearDone(),
    onRetryFailed: () => flows.retryFailed(),
    onToggleQueue: () => { state.queueCollapsed = !state.queueCollapsed; queue.render(); },
    onRestoreTask: id => flows.restoreTaskRow(id),
    onRetryTask: id => flows.retryOne(id)
  }
});

/* ============ 6. 另外六块界面（原型 :655-754 里剩下那几段） ============ */

const topbar = createTopbarView({
  root: byId('topbar'),
  t,
  handlers: {
    onPermClick: () => toast.show({ text: state.adminGranted ? t('topbar.permToastGranted') : t('topbar.permToastDenied') }),
    // 顶栏那颗「检查更新」：先把人带到那一格，再顺手查一次（原型 :1713-1716 就是这个顺序，两句写在同一个 handler 里）
    onCheckUpdate: () => {
      settings.open('update');
      if (!state.update.checking) checkUpdate();
    }
  }
});

/** 搜索的 150 毫秒防抖（原型 :3097）：定时器归这一层管，工具栏那件只管把字递出来 */
let searchTimer = null;

const banner = createBannerView({
  root: byId('banner'),
  t,
  // 只在本次运行里收掉，跟原型 :3356-3357 一致（没有落盘）：下次开机还会提一次
  handlers: { onClose: () => { state.bannerDismissed = true; banner.hide(); } }
});

const toolbar = createToolbarView({
  root: byId('toolbar'),
  progress: globalProgress.el,      // 那条线贴工具栏下沿，位置在这一行末尾（原型 :704）
  t,
  handlers: {
    onSelectToggle: () => {
      const isAll = allCheckedState();
      setAllChecked(!isAll);
      renderAll();
      toast.show({ text: isAll ? t('toolbar.checkedNone') : t('toolbar.checkedAll') });
    },
    onInvert: () => {
      invertAll();
      renderAll();
      toast.show({ text: t('toolbar.inverted') });
    },
    onFoldToggle: () => {
      const isAllCollapsed = allCollapsedState();
      state.collapsed.clear();
      if (!isAllCollapsed) state.categories.forEach(c => state.collapsed.add(c.id));
      toast.show({ text: isAllCollapsed ? t('toolbar.expanded') : t('toolbar.folded') });
      tree.render();
      toolbar.render();
    },
    // 150 毫秒防抖留在这一层（原型 :3097）：每敲一个字就重画整棵树会卡
    onSearch: value => {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(() => {
        state.filter = value;
        tree.render();
      }, SEARCH_DEBOUNCE_MS);
    },
    onDetectAll: () => flows.detectAll(),
    onApplyQueue: () => flows.applyQueue(),
    onRestore: () => flows.restoreAll()
  }
});

const statusbar = createStatusbarView({ root: byId('statusbar'), t });

const sidebarFooter = createSidebarFooterView({
  root: byId('sidebarFooter'),
  t,
  handlers: {
    // 「同步状态」：转圈＋换字那一段照原型 :1726-1727，防重入在件里那颗钮上（:1724）
    onSync: () => {
      sidebarFooter.setSyncBusy(true);
      flows.syncStatuses(() => sidebarFooter.setSyncBusy(false));
    },
    onSettings: () => settings.open('layout')
  }
});

const trayPill = createTrayPillView({
  pill: byId('trayPill'),
  menu: byId('trayMenu'),
  themes,
  t,
  ctx: {
    // 圆点点下去就是换主题，与「选择主题」那六张卡同一条口径（已定 C6：点了立刻生效，不等完成）
    onSetTheme: applyThemeId,
    onAction: kind => toast.show({ text: kind === 'quit' ? t('tray.toastQuit') : t('tray.toastMinimize') })
  }
});

/** 这就是注入给 flows 的那支 ui.renderStatus：状态栏 → 顶栏那颗徽标 → 队列 → 工具栏 → 托盘胶囊，顺序照原型 :2226-2228。
    托盘胶囊排在最后：它那颗点与状态栏那盏灯读的是同一个 state.connected，两处必须同一轮重画，
    否则会出现"状态栏说没连上、胶囊还在闪"——那是两句互相打脸的话，比两句都错更难查。 */
function renderStatus() {
  statusbar.render();
  topbar.render();
  queue.render();
  toolbar.render();
  trayPill.render();
}

function renderAll() {
  tree.render();
  detail.render();
  renderStatus();
}

/* ============ 7. 六条流程：把"画"的活儿交回给上面这几个渲染器 ============ */

/* 这里给的是"画"的六个出口，一个也不给"改数据"的出口：流程自己不动 store 里服务管的那些字段，
   快照进来只走 core/api/snapshot.js。patchItem／patchTask 那两支单行补画上一版是给流程用的，
   现在每一次事件带的都是整份快照，渲染器自己按 id 串判要不要重建（TreeView.render :219、QueueView.paintRows），
   留着这两支就是留着第二条"什么时候画哪一行"的路子。 */
flows = createFlows({
  t,
  ui: {
    renderTree: () => tree.render(),
    renderDetail: () => detail.render(),
    renderStatus,
    scrollToInt: id => tree.scrollToInt(id),
    // flows 只给"要做什么＋确认那颗的字"，取消那颗与没人给确认字时的兜底在这里补（原型 :2245 的默认值就是「确认」）
    confirm: o => confirmDialog({
      ...o,
      cancelText: t('common.cancel'),
      confirmText: o.confirmText || t('common.confirm')
    }),
    toast: (text, kind) => toast.show({ text, kind })
  }
});

/* 五条业务事件与那条连接线都接到 flows 上：每条事件自带整份快照，界面不猜另一半。
   订阅写在这里而不是 flows 里面：这一层负责"哪一块界面接哪一路数据"，flows 只管数据进来之后怎么画。 */
subscribe({
  onEvent: {
    detectProgress: flows.onEvent,
    queueProgress: flows.onEvent,
    queueDone: flows.onEvent,
    networkChange: flows.onEvent,
    downloadProgress: flows.onEvent
  },
  onLink: flows.onLink
});

/* ============ 8. 工具栏与侧栏那几颗钮 ============ */

/* 这一节原来那十一段 addEventListener 在本轮搬进了各自的处理函数里：
   工具栏那七颗在 createToolbarView 的 handlers、横幅「知道了」在 createBannerView、
   权限徽标与「检查更新」在 createTopbarView、侧栏那两颗在 createSidebarFooterView——
   都在这里再接一次就成了两份监听，点一次跑两遍（原型 :3067-3101、:1722-1744 那些句，行为一字未动）。 */

/* ============ 8.5 设置中心、主题弹窗与「检查更新」（原型 :1535-1630、:1632-1719、:3321-3351） ============ */

/* 版本那两个数开机从服务取（flows.start() 里的 GetVersion）：原型把 v1.0.0 与 2026-10-18 直接写死在卡片里
   （:971-972、:1023-1024），这里跟着写死的话，「关于我们」那一格会说出一台机器上根本没装的那个版本。
   取到之前那一格显示 — ——那是"还不知道"的实话，不是坏了一格。 */

/** 更新那一下要同时画三处：顶栏那颗钮的圆点、导航那条右边的红点、面板里那一格（原型 :1712-1715 一次画完） */
function renderUpdate() {
  topbar.render();
  if (settings.isOpen()) settings.sync();
}

/** 契约方法 CheckUpdate：本期服务那侧如实回一句「本期不提供」（GAC-UPDATE-501），
   所以这里没有 latest／hasUpdate 可写，只把那一句摆在这一格上。
   为什么不自己编一个"已是最新"：那是拿界面盖住服务没说过的话，用户按它判断"查过了"。
   失败那句除了弹一次气泡，还留在 state.update.checkError 里：那一格不能退回"尚未检查"，
   服务明明答过话，界面说没查过就是界面在说假话。 */
async function checkUpdate() {
  const u = state.update;
  if (u.checking) return null;
  u.checking = true;
  u.checkError = '';
  renderUpdate();
  try {
    const r = await api.checkUpdate();
    u.latest = r?.latest || '';
    u.hasUpdate = !!r?.hasUpdate;
    u.log = Array.isArray(r?.log) ? r.log : [];
    return r;
  } catch (err) {
    const text = replyErrorText(t, err);
    u.hasUpdate = false;
    u.log = [];
    u.checkError = text;
    toast.show({ text, kind: 'error' });
    return null;
  } finally {
    u.checking = false;
    renderUpdate();
  }
}

/** 契约方法 DownloadUpdate：同上一句——服务回的是"本期不提供"，这里不替它编一条进度条。
    下载进度那一条事件（download:progress）真起来之后才走 state.update.progress／speed，登记在 #51。 */
async function downloadUpdate() {
  const u = state.update;
  if (u.downloaded) { toast.show({ text: t('settings.update.toastRelaunch') }); return; }   // 原型 :1697：下载完了再点就是这一句
  if (u.downloading) return;
  u.downloading = true;
  u.checkError = '';
  renderUpdate();
  try {
    await api.downloadUpdate();
    u.downloaded = true;
    toast.show({ text: t('settings.update.toastDownloaded') });
  } catch (err) {
    u.checkError = replyErrorText(t, err);
    toast.show({ text: u.checkError, kind: 'error' });
  } finally {
    u.downloading = false;
    renderUpdate();
  }
}

/* 换主题只有这一个出口：主题弹窗那六张卡、托盘那六个圆点、「外观」那一格的下拉，三处都叫它。
   干什么用：改值 → 立刻上色（applyNow）→ 落盘 → 谁开着就补画谁。
   会错在哪：三处各写一遍，表现是"圆点换了色、弹窗里的选中圈还在旧的那套"（原型 :3302-3306 那段漏了上色那一句，
             它的托盘圆点点了根本不变色，这一条不跟）。 */
function applyThemeId(id) {
  state.settings.theme = id;
  applyNow();
  persist();
  if (settings.isOpen()) settings.sync();
  if (themePicker.isOpen()) themePicker.sync();
}

const themePicker = createThemePickerView({
  root: byId('themeMask'),
  t,
  ctx: {
    themes,
    /** 点一张卡立刻换（原型 :3335-3339 那三行：改值、落盘、重画网格），不等「完成」——那颗钮只管关窗 */
    setTheme: applyThemeId
  }
});

const settings = createSettingsView({
  root: byId('settingsMask'),
  t,
  ctx: {
    panes,
    themes,
    langs: () => languageOptions(t.langs()),
    applyNow,
    persist,
    // 折叠着的时候改队列高度看不出来（原型 :1592 那一下先展开再改），所以这一格由面板开口要展开
    expandQueue: () => {
      if (!state.queueCollapsed) return;
      state.queueCollapsed = false;
      queue.render();
    },
    toast: text => toast.show({ text }),
    openThemePicker: () => themePicker.open(),
    checkUpdate,
    downloadUpdate
  }
});

// 「设置中心」与「检查更新」那两颗钮的监听，一轮里只在一处：前者在 createSidebarFooterView 的 handlers，
// 后者在 createTopbarView 的 handlers——两句要带的上下文（open 哪一格、要不要顺手查一次）都跟着那颗钮走。

/* 快捷键（原型 :3223-3238）。确认框或设置中心开着时整段跳过——否则 Ctrl+A 会连树一起全选；
   焦点在输入格／下拉里时也跳过，那是用户正在打字，不是在下命令。
   Escape 不在快捷键那一段里：确认框自己管（件里的捕获监听），设置中心与主题弹窗由下面这两句按"先里后外"的顺序收
   （原型 :2263-2268：先关主题弹窗，再关设置中心；确认框开着时它自己已经吃掉这一键，这里不该再动后面的层）。 */
function dialogOpen() {
  return !!document.querySelector('.modal-mask.show') || !!document.querySelector('.settings-mask.show');
}

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    if (document.querySelector('.modal-mask.show')) return;      // 确认框开着：这一键归它自己（件里已经接了）
    if (themePicker.isOpen()) { themePicker.close(); return; }
    if (settings.isOpen()) { settings.close(); return; }
  }
  if (dialogOpen()) return;
  const tag = (e.target.tagName || '').toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return;
  if (!(e.ctrlKey || e.metaKey) || typeof e.key !== 'string') return;
  const key = e.key.toLowerCase();
  if (key === 'a') {
    e.preventDefault();
    setAllChecked(true);
    renderAll();
    toast.show({ text: t('shortcut.checkedAll') });
  } else if (key === 'd') {
    e.preventDefault();
    flows.detectAll();
  } else if (e.key === 'Enter') {
    e.preventDefault();
    flows.applyQueue();
  }
});

/* ============ 9. 首次渲染、开机取数与启动自动化（原型 :3355-3362） ============ */

renderAll();
banner.show();

/** 契约方法 GetSettings：把设置的真家（服务那一份文件）读回来，覆盖本地缓存并立刻刷界面。
    干什么用：本地那份缓存可能是上一次运行留下的、也可能是这台机器第一次运行时的那套默认值——
              只有服务答过话，界面上那两把开机开关说的才是服务下一次真的会照做的那两把。
    会错在哪：① 拉过来就 saveSettings＋persist：又把刚读到的那一份送回去写一遍，
                 服务那侧的写是串行排队的，开机白排一格；所以这里只落缓存，不走 persist。
             ② 本地正有一改动还没送出去时拉：把人刚改的那几格盖掉，表现是"改了又弹回去"。
                所以先看 pushPending()。
             ③ 语言以服务为准、但不热切换（已定 C7 那条口径：改了下一启动生效）。
                这里要是发现两份语言不一样就提示一句，别让人以为已经换了。
    怎么回来：normalize() 只有一个家（core/settings.js），拉回来的东西按同一把尺子校过再进 state；
              这一问失败不弹气泡——start() 那一句已经说过"连不上"，同一件事在一屏之内不许有两种说法（已定：D6 乙）。 */
async function pullSettings() {
  if (pushPending()) return;
  let raw;
  try {
    raw = await api.getSettings();
  } catch {
    return;
  }
  if (!raw) return;
  const merged = normalize(raw, themes.map(th => th.id));
  const langChanged = merged.language !== t.getLang();
  Object.assign(state.settings, merged);
  saveSettings(state.settings);
  applyNow();
  if (langChanged) toast.show({ text: t('settings.langSwitched') });
}

/* 开机那一下问的现在有五件事（整份快照 → 服务态与写入锁 → 权限灯 → 版本号 → 设置那一份真家），
   前四件在 flows.start() 里，第五件在上面这一支。顺序钉死：服务答过话 → 拉设置 → 补画 → 才跑启动自动化。
   为什么自动化要排在拉设置之后：那两把开关（自动检测／自动应用）判的就是 state.settings 里那两个值,
   拿缓存去判＝用一个可能过期的数决定要不要动 hosts。start() 回 false（连不上服务）时干脆不跑：
   这时候连"清单里有哪几条、哪些已经替换"都不知道，跑下去只会拿半份数据往外发写动作。
   起不来也照实画：那一问失败会由 flows 报一句人话（找不到宿主与连不上服务是两句话），树还是那 54 行，
   只是读数一栏全是 — ——那是"还不知道"，不是坏了一格。 */
flows.start().then(async ok => {
  if (ok) await pullSettings();
  renderAll();
  if (settings.isOpen()) settings.sync();
  if (ok) flows.runStartupAutomation();
});
