/* 树上"一个类目"那一行（组件库规划.md 4.15 的 CategoryHeader）。干什么用：勾选（含半选）、折叠箭头、
   计数、整类还原那颗↺。对应原型 :1814-1827 那段模板。
   会错在哪：① 半选态必须跟"这一类勾了几条"绑在一起算，行里自己判容易和条目行的勾漏一拍
                （界面上类目没勾、底下条目全勾着）；
             ② has-detecting 与 has-applying 互斥，检测优先（原型 :1784-1788 那段 if/else），
                两个类同时挂上会让那条流动的金线压在蓝线上，看着像出了两个当前项；
             ③ 折叠与否不能靠重建：整块重画会把滚动位置弄丢，所以 collapsed 只切 .children 的 hidden（由 TreeView 做）。
   怎么回来：只有一个 update(data)；勾选状态由调用方按 store.categoryCheckState 算好传进来，本件不自己数。 */

import { Checkbox } from '../../../components/checkbox/Checkbox.js';
import { el, replace, setTabStop } from '../../../utils/dom.js';
import { splitInto } from '../../../utils/highlight.js';

/**
 * @param {object} h
 * @param {(id:string)=>void} h.onToggleCheck     点勾（半选被点一下＝全勾，由调用方决定）
 * @param {(id:string)=>void} h.onToggleCollapse  点箭头或点行
 * @param {(id:string)=>void} h.onRestore         点这一类的↺
 */
export function CategoryHeader(h) {
  const nameEl = el('span', { class: 'name' });
  const caret = el('span', { class: 'caret', text: '▼', attrs: { 'aria-hidden': 'true' } });
  const count = el('span', { class: 'count' });
  const slot = el('span', { class: 'restore-slot' });
  const check = Checkbox({ state: 'unchecked', onChange: () => h.onToggleCheck(data.id) });

  const restoreBtn = el('button', {
    class: 'tree-restore-btn',
    type: 'button',
    text: '↺',
    dataset: { restoreCat: '' }
  });
  restoreBtn.addEventListener('click', e => {
    e.stopPropagation();
    h.onRestore(data.id);
  });

  const node = el('div', {
    class: 'node cat',
    dataset: { kind: 'cat', id: '' },
    attrs: { role: 'treeitem', 'aria-expanded': 'true', tabindex: '-1' }
  }, [check.el, caret, nameEl, el('span', { class: 'info-area info-count' }, count), slot]);

  node.addEventListener('click', e => {
    if (e.target.closest('.cb-wrap') || e.target.closest('.tree-restore-btn')) return;
    h.onToggleCollapse(data.id);
  });

  let data = {};

  function paint() {
    if (!data.id) throw new Error('CategoryHeader.update 少了 id：这一行不知道自己是哪个类目，勾一下会改到别类身上');
    if (!data.checkState) throw new Error(`CategoryHeader.update(${data.id}) 少了 checkState：半选态丢了，界面上"一类只勾了两条"和"一条没勾"长得一样`);
    node.dataset.id = data.id;
    restoreBtn.dataset.restoreCat = data.id;

    node.className = 'node cat' + (data.hasDetecting ? ' has-detecting' : (data.hasApplying ? ' has-applying' : ''));
    node.setAttribute('aria-expanded', data.collapsed ? 'false' : 'true');
    setTabStop(node, !!data.tabStop);

    caret.className = 'caret' + (data.collapsed ? ' collapsed' : '');
    check.update({ state: data.checkState, ariaLabel: data.checkLabel });
    nameEl.title = data.title || '';
    splitInto(nameEl, data.name, data.re || null);
    count.textContent = String(data.count == null ? '' : data.count);

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
