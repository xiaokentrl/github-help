/* 顶部中间那颗托盘胶囊与它的下拉（views 层第十块，原型 :655-664 ＋脚本 :3289-3320）。
   干什么用：胶囊本身（一颗点、一行字）＋点开的菜单（六个主题圆点、一条分隔、两条"最小化到托盘／退出"）。
   会错在哪：① 六个圆点由 styles/themes/ 扫出来的那份 themes 喂，一个也不写死（第 6.4 那条）。
                多一套主题就多一个圆点；这里如果数成六个，加主题那天就少一个点而没人报错；
             ② 点圆点换主题这件事，原型 :3302-3306 只写了改值＋落盘＋重画圆点，漏了 applyThemeId() 那一句，
                所以它的圆点点了不变色。这里跟已定的那一条口径走：「选择主题」那六张卡点了立刻换（C6），
                同六个圆点也照这一条；不跟着原型那个漏抄，否则界面上就是"点了没反应"；
             ③ 菜单靠 .tray-menu 的 .show 这一个类收放（layout.css :428），点外面收掉时判的是类名
                （.tray-pill／.tray-menu），不认 id——id 只有 main.js 认；
             ④ 那两条菜单项今天只回一句提示：最小化与退出要走系统托盘与壳（技术方案 M0／F7 那一侧），
                界面上今天没有那条通道。句子照原型 :3308 那两句，别当成功能坏了；
             ⑤ 胶囊上那颗点不写死 running：它说的是"这条线与壳之间通不通"（state.connected，
                gac:state 那一句给的唯一真相）。原型 :657 把它当装饰写死成"正在运行"，
                断线的时候它还在那儿闪——那一颗点就成了全界面唯一一句谎话。
   怎么回来：两个容器由 main.js 递进来；圆点与主题对不上时，报错的是本文件上面那条数法——
             新增一套主题只加 styles/themes/<id>/ 那一个文件夹，这里一个字不改。
             那颗点的颜色由 render() 按 state.connected 换类名，main.js 的 renderStatus() 每次状态变化都会走到这里。 */

import { state } from '../../core/store.js';
import { Icon } from '../../components/icon/Icon.js';
import { el, replace } from '../../utils/dom.js';

/** 菜单里那两条：图标名与动作名照原型 :662-663 那两行的 data-action */
const MENU_ITEMS = [
  { action: 'minimize', icon: 'minimize', key: 'tray.minimize' },
  { action: 'quit', icon: 'quit', key: 'tray.quit' }
];

/**
 * @param {object} d
 * @param {HTMLElement} d.pill     外框那一条 #trayPill
 * @param {HTMLElement} d.menu     外框那一条 #trayMenu
 * @param {Array<object>} d.themes styles/themes/ 扫出来的那一套（每套要有 id、name、colors 四支）
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.ctx           { onSetTheme(id), onAction('minimize'|'quit') }
 */
export function createTrayPillView({ pill, menu, themes, t, ctx }) {
  pill.title = t('tray.pillTitle');
  const dotEl = el('span', { class: 'pill-dot' });
  replace(pill, [
    dotEl,
    el('span', { class: 'pill-label', text: t('common.appName') })
  ]);

  const dots = el('div', { class: 'theme-dots' });
  const items = MENU_ITEMS.map(mi => el('div', { class: 'tray-menu-item', dataset: { action: mi.action } },
    [Icon({ name: mi.icon, size: 'sm' }).el, document.createTextNode(t(mi.key))]));

  replace(menu, [dots, el('div', { class: 'tray-menu-sep' }), ...items]);

  /** 六个（不是写死的六，是 themes 有几套就几个）圆点：底色与强调色取自各自那套的预览色（原型 :3295） */
  function renderDots() {
    replace(dots, themes.map(th => el('button', {
      class: 'theme-dot' + (th.id === state.settings.theme ? ' active' : ''),
      title: th.name,
      attrs: { type: 'button', 'aria-label': th.name, style: `--dot-bg:${th.colors[0]};--dot-accent:${th.colors[2]}` },
      dataset: { dot: th.id }
    })));
  }

  function close() { menu.classList.remove('show'); }
  function isOpen() { return menu.classList.contains('show'); }

  pill.addEventListener('click', e => {
    e.stopPropagation();
    if (isOpen()) { close(); return; }
    renderDots();                    // 每次打开重画一次：主题可能是在设置中心里换的
    menu.classList.add('show');
  });

  dots.addEventListener('click', e => {
    const hit = e.target && e.target.closest ? e.target.closest('[data-dot]') : null;
    if (!hit) return;
    e.stopPropagation();
    if (hit.dataset.dot === state.settings.theme) return;   // 点的就是当前这套：不落盘、不重画（原型 :3301）
    ctx.onSetTheme(hit.dataset.dot);
    renderDots();
  });

  for (const node of items) {
    node.addEventListener('click', () => {
      close();
      ctx.onAction(node.dataset.action);
    });
  }

  /** 那颗点：通着＝跑动的脉冲，断了＝灰着不动。除这一处别处不许再判一次 connected（C2 那笔账）。 */
  function renderDot() {
    dotEl.classList.toggle('running', state.connected);
    dotEl.classList.toggle('down', !state.connected);
  }

  // 点哪儿都收掉，除了胶囊与自己这两块（原型 :3316-3318 判的就是这两个 id，这里换成类名）
  document.addEventListener('click', e => {
    const target = e.target;
    if (!target || !target.closest) return;
    if (!target.closest('.tray-pill') && !target.closest('.tray-menu')) close();
  });

  renderDot();
  renderDots();
  return { el: pill, render: () => { renderDot(); renderDots(); }, close, isOpen, dots, menu, items };
}
