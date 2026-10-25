/* 设置中心那一整块弹窗（views 层第五块）。干什么用：把原型 :768-1035 那一大段外框加 :1535-1630 那套切换／保存，
   收成一个渲染器——标题行、左边那列导航、右边那一格内容、底下两颗钮，全在这里生成；
   每一格面板各自住在 panes/<面板id>/pane.js，由 panes/index.js 扫出来、main.js 注进来（ctx.panes），
   本文件一处也不写死有哪几格、有几格。
   会错在哪：① 两份状态各说各话。这一轮的规矩是"工作副本＋生效出口"：打开时从 state.settings 抄一份 draft，
                点一下立刻生效的那几项（宽度、队列高、缩放、主题）走 ctx.set——写 draft、写 state、刷界面、落盘四件一起做；
                点保存才生效的那九项（语言＋八个开关）只改 draft。所以「取消」＝扔掉 draft，下次 open 重新抄，
                不需要写一段"逐项回滚"（原型 :1560-1575 每次 open 都重新刷一遍控件，就是同一个道理）；
             ② 「保存」那颗钮在「检测升级」与「关于我们」两格要藏起来（原型 :1551 用的是 visibility，
                不是 display——用 display 会把底下那颗「取消」一起挪位置）；
             ③ 导航那几条的名字、顺序、分组，全部来自扫出来的 panes（collect.js 已按 order 排好并分过组），
                这里再手写一遍就成了第二份名单——加一格面板的人改不到它，界面少一格而没人报错；
             ④ 语言切换只在点保存时判一次"跟原来不一样"，然后提示下次启动生效（原型 :1612-1615）：
                真重画整站归下次启动（本轮不做热切换，改了就要满界面找哪些句子已经画出去）；
             ⑤ 保存那一句要说满：那四把只有外壳能动的开关（core/settings.js 的 SHELL_KEYS）动了其中几条，
                就要在「设置已保存」后面补半句"这几项还没那条通道，本次运行看不出效果"——
                只说"已保存"是在承诺一件今天做不到的事。名单在这里只读一次，别处不许再抄一份四个键；
             ⑥ 一句气泡只装得下最后一句：连着 ctx.toast 三次，人只看得见第三次（components/toast/Toast.js
                屏幕上就那一个 #toast）。所以保存那一下要说的几件事拼成一句，不排三段。
   怎么回来：本文件不改队列、不跑检测、不发请求——凡是要动外面的，全走 main.js 注进来的 ctx 那几个动词；
             它也不认外框的 id：外框只留 #settingsMask 这一个壳，里面那一整套结构由这里生成。
             面板只建一次（控件留在原地），每次打开靠各格的 sync() 把工作副本里的值补回去——重复建会把焦点与滚动位置弄丢。
             清单为什么由外面注进来而不是这里 import panes/index.js：那一句目录扫描是构建工具的能力，
             node 里的用例 import 不动它；注进来之后本文件跟 TreeView 收 categories 一样，能在单测里整块建一次。 */

import { normalize, SHELL_KEYS } from '../../core/settings.js';
import { state } from '../../core/store.js';
import { el, replace } from '../../utils/dom.js';

/** 打开时抄的那份工作副本里，只有这几项是"点保存才生效"的——其余立刻生效，本来就不该等保存 */
const DEFERRED_KEYS = ['language', 'trayMinimize', 'showTrayIcon', 'autoStart', 'startMinimized',
  'detectFocusFollow', 'applyFocusFollow', 'autoDetectOnStartup', 'autoApplyOnStartup'];

/**
 * @param {object} d
 * @param {HTMLElement} d.root  #settingsMask（这一整套面板由本文件往里生成）
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.ctx        外面给的那几个动词：{ panes, themes, langs, applyNow, expandQueue, persist, toast,
 *                               openThemePicker, checkUpdate, downloadUpdate }
 *                               （panes＝views/settings/panes/index.js 扫出来的那七格，跟 TreeView 收 categories 一样由外面注进来）
 */
export function createSettingsView({ root, t, ctx }) {
  const panes = ctx.panes;
  if (!Array.isArray(panes) || panes.length === 0) {
    throw new Error('createSettingsView 要 ctx.panes：由 main.js 从 views/settings/panes/index.js 扫出来注进来（本文件不认识构建工具的目录扫描）');
  }
  let draft = { ...state.settings };
  let currentId = panes[0].id;

  const paneCtx = {
    t,
    state,
    themes: ctx.themes,
    langs: ctx.langs,
    get: key => draft[key],
    /** 立刻生效：写工作副本、写真状态、刷界面、落盘，四件事只在这一个出口做（漏一件就是"看着改了其实没存"） */
    set: (key, value) => {
      draft[key] = value;
      state.settings[key] = value;
      ctx.applyNow();
      ctx.persist();
    },
    setDraft: (key, value) => { draft[key] = value; },
    expandQueue: () => ctx.expandQueue(),
    toast: text => ctx.toast(text),
    openThemePicker: () => ctx.openThemePicker(),
    checkUpdate: () => ctx.checkUpdate(),
    downloadUpdate: () => ctx.downloadUpdate()
  };

  const subEl = el('span', { class: 'sub' });
  const navEl = el('nav', { class: 'settings-nav' });
  const contentEl = el('div', { class: 'settings-content' });
  const saveBtn = el('button', { class: 'btn primary', type: 'button', text: t('settings.save'), dataset: { role: 'save' } });

  const navItems = [];
  const instances = [];
  const navKids = [];
  let lastGroup = null;
  for (const pane of panes) {
    if (pane.group !== lastGroup) {
      lastGroup = pane.group;
      // 第二组那一条上面多留一点空隙（原型 :785 那个 margin-top 收成了 .nav-group.gap）
      navKids.push(el('div', {
        class: 'nav-group' + (navKids.length ? ' gap' : ''),
        // 组名写成两句完整的键，不用模板拼：tests/i18n.test.mjs 那把尺子认字面，拼出来的它看不见
        text: t(pane.group === 'prefs' ? 'settings.group.prefs' : 'settings.group.info')
      }));
    }
    const item = el('div', {
      class: 'nav-item', dataset: { pane: pane.id },
      attrs: { role: 'tab', 'aria-selected': 'false', tabindex: '-1' }
    }, [
      el('span', { class: 'ico', attrs: { 'aria-hidden': 'true' }, text: pane.icon }),
      document.createTextNode(t(pane.nameKey)),
      pane.badge ? el('span', { class: 'badge-dot', hidden: true }) : null
    ]);
    navKids.push(item);
    navItems.push(item);

    const instance = pane.build(paneCtx);
    instance.el.dataset.pane = pane.id;
    instance.el.setAttribute('role', 'tabpanel');
    instances.push(instance);
  }
  replace(navEl, navKids);
  replace(contentEl, instances.map(i => i.el));

  const panel = el('div', { class: 'settings-panel', attrs: { role: 'dialog', 'aria-modal': 'true' } }, [
    el('div', { class: 'settings-head' }, [
      el('h2', { text: t('settings.title') }), subEl, el('div', { class: 'spacer' }),
      el('button', { class: 'close-btn', type: 'button', text: '×', title: t('settings.close'), dataset: { role: 'close' } })
    ]),
    el('div', { class: 'settings-body' }, [navEl, contentEl]),
    el('div', { class: 'settings-foot' }, [
      el('button', { class: 'btn', type: 'button', text: t('common.cancel'), dataset: { role: 'cancel' } }),
      saveBtn
    ])
  ]);
  root.appendChild(panel);

  function select(paneId) {
    const pane = panes.find(p => p.id === paneId) || panes[0];
    currentId = pane.id;
    navItems.forEach(item => {
      const on = item.dataset.pane === pane.id;
      item.classList.toggle('active', on);
      item.setAttribute('aria-selected', on ? 'true' : 'false');
      item.tabIndex = on ? 0 : -1;                       // roving tabindex：Tab 只落进当前这一条
    });
    for (const node of contentEl.children) node.classList.toggle('active', node.dataset.pane === pane.id);
    subEl.textContent = t(pane.subKey);
    saveBtn.style.visibility = pane.hasSave ? 'visible' : 'hidden';
    syncBadges();
  }

  /** 导航那条右边的红点跟着 state.update.hasUpdate 走（原型 :1712 那颗 #navUpdateDot） */
  function syncBadges() {
    for (const item of navItems) {
      const dot = item.querySelector('.badge-dot');
      if (dot) dot.hidden = !state.update.hasUpdate;
    }
  }

  navEl.addEventListener('click', e => {
    const item = e.target.closest('.nav-item');
    if (item) select(item.dataset.pane);
  });
  navEl.addEventListener('keydown', e => {
    const at = navItems.indexOf(e.target.closest('.nav-item'));
    if (at < 0) return;
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0;
    let next = -1;
    if (step) next = (at + step + navItems.length) % navItems.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = navItems.length - 1;
    if (next < 0) return;
    e.preventDefault();
    select(navItems[next].dataset.pane);
    navItems[next].focus();
  });

  function syncAll() {
    for (const instance of instances) instance.sync();
    syncBadges();
  }

  function open(paneId) {
    draft = { ...state.settings };
    syncAll();
    select(paneId || panes[0].id);
    root.classList.add('show');
  }

  function close() { root.classList.remove('show'); }

  function save() {
    const langChanged = draft.language !== state.settings.language;
    // 立刻生效那几项早就写进 state 了；这里把九项延后值一次整理一次落盘，
    // normalize() 顺手补上「自动应用依赖自动检测」那条（它的家就在 core/settings.js）
    const merged = normalize({ ...state.settings, ...pick(draft, DEFERRED_KEYS) }, ctx.themes.map(th => th.id));
    // 这四把开关只有程序外壳能动（名单的家在 core/settings.js 的 SHELL_KEYS）：动了其中几条，保存那一句就得带上后半句
    const shellChanged = SHELL_KEYS.filter(key => !!merged[key] !== !!state.settings[key]).length;
    Object.assign(state.settings, merged);
    ctx.persist();
    ctx.applyNow();
    // 一句话讲完：这一只气泡一次只装得下句子，连着 show 三次只有最后那一句看得见（components/toast/Toast.js）。
    // 所以「下次启动生效」与"这四项还没那条通道"都要拼进同一句，而不是让「设置已保存」把它们盖掉。
    const notes = [];
    if (langChanged) notes.push(t('settings.langSwitched'));
    if (shellChanged) notes.push(t('settings.shellPending', { n: shellChanged }));
    close();
    ctx.toast(notes.length ? `${t('settings.saved')}｜${notes.join('｜')}` : t('settings.saved'));
  }

  root.addEventListener('click', e => { if (e.target === root) close(); });
  panel.addEventListener('click', e => {
    const btn = e.target.closest('[data-role]');
    if (!btn) return;
    const role = btn.dataset.role;
    if (role === 'close' || role === 'cancel') close();
    else if (role === 'save') save();
  });

  select(currentId);

  return {
    open, close,
    isOpen: () => root.classList.contains('show'),
    /** 外面（拖动把手、检查更新那一下）改了 state 里相关的数，进来补一次画 */
    sync: () => { syncAll(); select(currentId); },
    currentPane: () => currentId
  };
}

function pick(obj, keys) {
  const out = {};
  for (const k of keys) out[k] = obj[k];
  return out;
}
