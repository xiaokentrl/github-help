/* 左侧栏底部那两颗钮（views 层第九块）。干什么用：把外框里写死的那两格（原型 :710-718）收成一个渲染器——
   「同步状态」与「设置中心」，图标一律 Icon 件的内联 SVG（⟳ 与 ⚙ 这两个字符在两个平台上宽度不一样，
   是"跨平台观感不一致"那一笔，已定全换 SVG，4.2）。
   会错在哪：① 「同步状态」按下之后要转圈＋换字，并且再点一次不算第二次同步（原型 :1724 用 dataset.busy 挡的）。
                这一下由本件的 setSyncBusy() 一处管：main.js 只负责在流程跑完时回调 setSyncBusy(false)；
                别处再判一次"是不是正在同步"就成了两份源，表现是同步完了那颗钮还转着；
             ② 图标是 SVG 之后，转圈必须绕自己那一格转（transform-box:fill-box），绕左上角转会甩出去——
                那条规则在 layout.css 的 .sf-btn .ico.spinning，键帧在 base.css 的 spin；
             ③ 这两颗钮不吃 disabled：原型只是"点了不重入"，置灰会让人以为同步坏了。
   怎么回来：容器由 main.js 递进来；这里刻一句中文，tests/views-settings.test.mjs 那把尺报红。 */

import { Icon } from '../../components/icon/Icon.js';
import { el, replace } from '../../utils/dom.js';

/**
 * @param {object} d
 * @param {HTMLElement} d.root       外框那一条 .sidebar-footer
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.handlers        { onSync, onSettings }
 */
export function createSidebarFooterView({ root, t, handlers }) {
  const syncIco = Icon({ name: 'refresh', size: 'sm' }).el;
  const syncLabel = el('span', { text: t('sidebar.sync') });
  const syncBtn = el('button', { class: 'sf-btn', title: t('sidebar.syncTitle'), attrs: { type: 'button' } },
    [syncIco, syncLabel]);
  const settingsBtn = el('button', { class: 'sf-btn', title: t('sidebar.settingsTitle'), attrs: { type: 'button' } },
    [Icon({ name: 'gear', size: 'sm' }).el, el('span', { text: t('sidebar.settings') })]);

  let syncing = false;
  syncBtn.addEventListener('click', () => {
    if (syncing) return;                 // 原型 :1724 那句防重入，判在同一颗钮上
    handlers.onSync();
  });
  settingsBtn.addEventListener('click', () => handlers.onSettings());

  replace(root, [syncBtn, settingsBtn]);

  return {
    el: root,
    /** 转圈＋换字那一下（原型 :1726-1727）；由 main.js 在 syncStatuses 的收尾回调里关掉 */
    setSyncBusy(on) {
      syncing = !!on;
      syncIco.classList.toggle('spinning', syncing);
      syncLabel.textContent = syncing ? t('sidebar.syncBusy') : t('sidebar.sync');
      return this;
    },
    isSyncBusy() { return syncing; },
    buttons: { sync: syncBtn, settings: settingsBtn }
  };
}
