/* 树上"一个域名"那一行（组件库规划.md 4.15 的 TreeRow）。干什么用：把原型 :1797-1811 那段模板变成一个能单行更新的对象——
   检测/应用跑的时候队列每几百毫秒补画一次，整棵树重画会把滚动位置和焦点一起弄丢（这是 8.4 那笔"整块重画"的老账）。
   会错在哪：① 这一行认得"条目""状态"这些业务，所以它进不了 components/（进了库，改一行队列要回归四个视图）；
             ② 行里一个字中文都不写：状态文字、还原钮的悬停说明、勾选的读屏名，全部由调用方从 i18n 取好传进来，
                漏传一个界面上就是一格空白，比写死中文更难查——所以 update() 里对必填项当场抛错；
             ③ 勾选必须是真 input：原型这里是 <span class="cb">（:1799），键盘根本选不中任何域名。
   怎么回来：接口只有一个 update(data)，data 的形状见下面 ViewModel；
             tests/views.test.mjs 里"parts 不许裸中文／不许 innerHTML"两条门禁对本文件生效。 */

import { Checkbox } from '../../../components/checkbox/Checkbox.js';
import { StatusDot } from '../../../components/status-dot/StatusDot.js';
import { el, replace, setTabStop } from '../../../utils/dom.js';
import { splitInto } from '../../../utils/highlight.js';

/**
 * @param {object} h
 * @param {(id:string)=>void} h.onActivate    点这一行（不是点勾、不是点↺）
 * @param {(id:string)=>void} h.onToggleCheck 勾一下
 * @param {(id:string)=>void} h.onRestore     点↺
 */
export function TreeRow(h) {
  const nameEl = el('span', { class: 'name' });
  const dot = StatusDot({ state: 'unset', label: '', size: 'tree' });
  const slot = el('span', { class: 'restore-slot' });
  const check = Checkbox({ state: 'unchecked', onChange: () => h.onToggleCheck(data.id) });

  const restoreBtn = el('button', {
    class: 'tree-restore-btn',
    type: 'button',
    text: '↺',
    dataset: { restoreItem: '' }
  });
  restoreBtn.addEventListener('click', e => {
    e.stopPropagation();
    h.onRestore(data.id);
  });

  const node = el('div', {
    class: 'node item',
    dataset: { kind: 'item', id: '' },
    attrs: { role: 'treeitem', 'aria-selected': 'false', tabindex: '-1' }
  }, [check.el, el('span', { class: 'caret-slot' }), nameEl, dot.el, slot]);

  node.addEventListener('click', e => {
    // 勾与↺ 自己会 stopPropagation，能冒泡到这里的只有"点了行"
    if (e.target.closest('.cb-wrap') || e.target.closest('.tree-restore-btn')) return;
    h.onActivate(data.id);
  });

  let data = {};

  function paint() {
    if (!data.id) throw new Error('TreeRow.update 少了 id：这一行不知道自己是哪条域名，还原按钮会点到别的条目上');
    if (data.statusText == null) throw new Error(`TreeRow.update(${data.id}) 少了 statusText：状态那一格会空着，读屏也念不出东西`);
    node.dataset.id = data.id;
    restoreBtn.dataset.restoreItem = data.id;

    node.className = 'node item'
      + (data.active ? ' active' : '')
      + (data.detecting ? ' detecting' : '')
      + (data.applying ? ' applying' : '');
    node.setAttribute('aria-selected', data.active ? 'true' : 'false');
    setTabStop(node, !!data.tabStop);

    check.update({ state: data.checked ? 'checked' : 'unchecked', ariaLabel: data.checkLabel });
    nameEl.title = data.title || '';
    splitInto(nameEl, data.name, data.re || null);
    dot.update({ state: data.dot, label: data.statusText });

    // 原型 :1806-1810：不该显示时这一格是空的，不是把按钮藏起来——藏起来的按钮还会被 Tab 走到
    replace(slot, data.showRestore ? restoreBtn : []);
    if (data.showRestore) {
      restoreBtn.title = data.restoreTitle || '';
      restoreBtn.setAttribute('aria-label', data.restoreTitle || '');
    }
  }

  return {
    el: node,
    update(next) { data = Object.assign(data, next); paint(); return this },
    focus() { node.focus(); return this }
  };
}
