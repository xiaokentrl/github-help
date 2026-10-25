/* 状态栏那一行（views 层第八块）。干什么用：把外框里写死的那一排（原型 :744-754）收成一个渲染器——
   网络那颗点、「启动自动化中」这一格、模式、DNS 服务名、上次检测、右边那格"已选 n / m 项"。
   会错在哪：① DNS 服务名与权限态由服务回，界面不猜（已定：C5）。这里开机一律先画一个 —，
                等 flows.start() 那一次 GetStatus／CheckPermissions 把结果写进 state 再 render 一次；
             ② "已选 <b>n</b> / m 项"那个数要单独加粗，句子只能拆成前后两段（键名 statusbar.selectionPre／selectionRest，
                两段各带一个空格，见 zh-CN.js 那两行注释）；合成一句就得用 innerHTML，那是本项目明令不走的路；
             ③ 「启动自动化中」这一格用 display 内联样式收放，跟原型 :2224 一致——
                因为 .statusbar .item 自己写了 display:flex，[hidden] 那个属性盖不住它；
             ④ 「网络正常」这一格 2026-10-23 换了说法：它原先是死的（原型 :745 也是死的，④ 那句注释当时写的就是
                "网络探测属后端，没接之前写活的是假话"）。现在这一格说的是这台机器上真问得到的那一件事——
                界面与后台服务之间那条线通不通（state.connected，gac:state 那一句给的唯一真相）。
                为什么不留着"网络正常"等探测接上：服务没起来的时候它也照样写着"网络正常"，
                那是全界面唯一一处没有来源的读数，而它旁边每一格都有来源。
                真正的网络探测（断网判定 GAC-DETECT-006）接上那天，这一格改回照服务回的 networkOK 画，
                事件名 network:change 契约里已经占好了。
   怎么回来：容器由 main.js 递进来；这里刻一句中文，tests/views-settings.test.mjs 那把尺报红。 */

import { state, checkedItems, allItems } from '../../core/store.js';
import { el, replace } from '../../utils/dom.js';

/**
 * @param {object} d
 * @param {HTMLElement} d.root   外框那一条 #statusbar
 * @param {(key:string,params?:object)=>string} d.t
 */
export function createStatusbarView({ root, t }) {
  const startup = el('div', { class: 'item', style: 'display:none' },
    [el('span', { class: 'live-dot gold' }), el('b', { text: t('statusbar.startup') })]);
  const dnsB = el('b', { text: t('common.dash') });
  const lastB = el('b', { text: t('common.dash') });
  const selection = el('div', { class: 'item' });
  const linkDot = el('span', { class: 'live-dot' });
  const linkB = el('b', { text: '' });

  replace(root, [
    el('div', { class: 'item' }, [linkDot, linkB]),
    startup,
    el('div', { class: 'item' }, [document.createTextNode(t('statusbar.modeLabel')), el('b', { text: t('statusbar.modeHosts') })]),
    el('div', { class: 'item' }, [document.createTextNode(t('statusbar.dns')), dnsB]),
    el('div', { class: 'item' }, [document.createTextNode(t('statusbar.lastDetect')), lastB]),
    el('div', { class: 'spacer' }),
    selection
  ]);

  /** 顺序照原型 renderStatus（:2207-2226）里状态栏那几行：检测时间 → DNS → 已选 → 启动那一格。
      这一支还管第一格那颗点与那句话：那条线通不通只有 flows.onLink 知道，界面上这一格是它唯一的出口。 */
  function render() {
    // 第一格三层话,由里往外查:线不通(壳与服务那条线)＞断网(服务判的,netwatch)＞都正常。
    // 断网那一句 2026-10-26 接上(原先写死"网络正常"的那笔账,见文件头④)。
    if (state.connected && state.networkOffline) {
      linkDot.className = 'live-dot off';
      linkB.textContent = t('statusbar.networkOffline');
    } else {
      linkDot.className = 'live-dot' + (state.connected ? '' : ' off');
      linkB.textContent = state.connected ? t('statusbar.linkUp') : t('statusbar.linkDown');
    }
    lastB.textContent = state.lastDetect || t('common.dash');
    dnsB.textContent = state.appStatus.dnsService;
    replace(selection, [
      document.createTextNode(t('statusbar.selectionPre')),
      el('b', { text: String(checkedItems().length) }),
      document.createTextNode(t('statusbar.selectionRest', { m: allItems().length }))
    ]);
    startup.style.display = state.startupAutomationRunning ? '' : 'none';
  }

  render();
  return { el: root, render };
}
