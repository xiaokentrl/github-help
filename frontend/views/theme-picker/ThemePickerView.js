/* 「选择主题」那六张卡的弹窗（views 层第六块）。干什么用：原型 :1039-1051 那段外框加 :3321-3351 那套建卡／点卡，
   收成一个渲染器——标题与一句说明、中间那张卡网格、底下一颗「完成」。
   会错在哪：① 卡片张数＝styles/themes 扫出来的那一份，不写死 6：加一套主题只多一个文件夹，这里跟着多一张卡；
             ② 点一张卡＝立刻换色并落盘（原型 :3335-3339 那三行：改 state.settings.theme、persist、重画网格）。
                这里绝不等「完成」才生效——那颗钮只管关窗，写成"点完成才换"就跟「主题暂时先这样吧」那句口径打架；
             ③ 换色走 main.js 注进来的 ctx.setTheme 这一个出口：html[data-theme] 那个属性由它管，
                设置中心「外观」那一格的"当前：某某"也是它的 sync 补的。这里自己写 dataset.theme 就会出现两处各画一半。
   怎么回来：本文件不认外框 id，只收 #themeMask 这一个壳；层级压在上面那件事交给 layout.css 的 --z-theme。 */

import { state } from '../../core/store.js';
import { ThemeCard } from './parts/ThemeCard.js';
import { el, replace } from '../../utils/dom.js';

/**
 * @param {object} d
 * @param {HTMLElement} d.root   #themeMask
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.ctx         { themes, setTheme(id) }
 */
export function createThemePickerView({ root, t, ctx }) {
  const gridEl = el('div', { class: 'theme-grid' });

  const panel = el('div', { class: 'theme-panel', attrs: { role: 'dialog', 'aria-modal': 'true' } }, [
    el('div', { class: 'theme-head' }, [
      el('div', {}, [
        el('h3', { text: t('theme.title') }),
        el('p', { text: t('theme.desc', { n: ctx.themes.length }) })
      ]),
      el('button', { class: 'close-btn', type: 'button', text: '×', title: t('settings.close'), dataset: { role: 'close' } })
    ]),
    el('div', { class: 'theme-body' }, [gridEl]),
    el('div', { class: 'theme-foot' }, [
      el('button', { class: 'btn', type: 'button', text: t('theme.done'), dataset: { role: 'done' } })
    ])
  ]);
  root.appendChild(panel);

  function render() {
    replace(gridEl, ctx.themes.map(theme => ThemeCard({
      theme,
      selected: theme.id === state.settings.theme,
      onPick: id => {
        if (id === state.settings.theme) return;
        ctx.setTheme(id);
        render();
      }
    }).el));
  }

  function open() {
    render();
    root.classList.add('show');
  }

  function close() { root.classList.remove('show'); }

  panel.addEventListener('click', e => {
    const btn = e.target.closest('[data-role]');
    if (btn) close();
  });
  root.addEventListener('click', e => { if (e.target === root) close(); });

  return {
    open, close,
    isOpen: () => root.classList.contains('show'),
    /** 「外观」那一格换了主题之后，下次 open 会重画；外面直接改 state 时也可以叫它补一次 */
    sync: render
  };
}
