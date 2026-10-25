/* 左侧类目树的渲染器（全站只有这一份，11 块菜单共用）。干什么用：把 store 里那 11 块 54 条画成一棵树，
   检测/应用跑的过程中每来一份快照重画一次，但只碰变了的那几行（行件自己比对新旧，见 parts/TreeRow.js）。
   对应原型 :1768-1890（renderTree / patchTreeItem / scrollTreeToItem）
   与 :3013-3062（树上的点击）。
   会错在哪：① 一块菜单一个渲染器＝11 份迟早长歪（目录规划第 3 节那条铁律），所以这里只认 menu/ 清单的形状，不认哪一块是 github；
             ② 整棵重画会丢滚动位置和键盘焦点——批量检测时每几百毫秒补画一次，重画一次焦点就退回 body。
                8.4 那句"单行更新与键盘可达必须同轮落地"说的正是这件事：只补一行不配键盘，等于把老毛病焊死；
             ③ 搜索命中要转义（原型 :1771-1772 先 escapeHtml 再把命中的片段包进 span）：这里改成文本节点拼装，一个标记也不解析。
   怎么回来：改数据一律走 main.js 注入的 handlers（勾选、折叠、选中、两颗↺），本文件不自己改 store；
             少传字段当场抛错（parts 那两个件里判），不会画出一格看不见的空白。 */

import {
  state, categoryCheckState, canRestoreHosts, hasRestorableInCategory
} from '../../core/store.js';
import { ItemMeta } from '../../core/meta.js';
import { CategoryHeader } from './parts/CategoryHeader.js';
import { TreeRow } from './parts/TreeRow.js';
import { el, clear, setTabStop } from '../../utils/dom.js';
import { filterRegExp } from '../../utils/highlight.js';

/** 逐字照原型 :1776-1780：类目名命中就整类留，否则只留名字或域名命中的条目；都没命中这类不出现 */
function planOf(cat, f, fLow) {
  const catMatch = cat.name.toLowerCase().includes(fLow);
  const items = (f && !catMatch)
    ? cat.items.filter(it => it.name.toLowerCase().includes(fLow)
      || it.domains.some(d => d.toLowerCase().includes(fLow)))
    : cat.items;
  return { cat, items, shown: !(f && !catMatch && items.length === 0) };
}

/**
 * @param {object} d
 * @param {HTMLElement} d.root                #tree
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.handlers  { onSelect, onToggleItem, onToggleCollapse, onToggleCategory,
 *                               onRestoreItem, onRestoreCategory }
 */
export function createTreeView({ root, t, handlers }) {
  /** catId → { header:CategoryHeader, children:HTMLElement, rows:Map<itemId,TreeRow> } */
  const nodes = new Map();
  let signature = '';
  let re = null;
  let tabStopId = null;

  root.setAttribute('role', 'tree');
  root.addEventListener('keydown', onKeydown);

  function itemData(item) {
    const meta = ItemMeta[item.status];
    if (!meta) throw new Error(`条目 ${item.id} 的状态「${item.status}」在 core/meta.js 的 ItemMeta 里没有——树这一行不知道该怎么画`);
    return {
      id: item.id,
      name: item.name,
      title: item.domains.join(', '),
      checked: !!item.checked,
      dot: meta.dot,
      statusText: meta.text,
      active: state.selectedId === item.id,
      detecting: item.status === 'detecting',
      applying: item.status === 'applying',
      showRestore: canRestoreHosts(item),
      restoreTitle: t('tree.restoreItemTitle'),
      checkLabel: t('tree.itemCheckAria', { name: item.name }),
      tabStop: tabStopId === item.id,
      re
    };
  }

  function catData(cat) {
    const hasDetecting = cat.items.some(i => i.status === 'detecting');
    return {
      id: cat.id,
      name: cat.name,
      checkState: categoryCheckState(cat),
      checkLabel: t('tree.catCheckAria', { name: cat.name }),
      collapsed: state.collapsed.has(cat.id),
      count: cat.items.length,
      // 原型 :1784-1788：两个高亮互斥，检测优先。件里按这个布尔值再决定挂哪个类，不自己判
      hasDetecting,
      hasApplying: cat.items.some(i => i.status === 'applying') && !hasDetecting,
      showRestore: hasRestorableInCategory(cat),
      restoreTitle: t('tree.restoreCatTitle', { name: cat.name }),
      tabStop: tabStopId === cat.id,
      re
    };
  }

  function plan() {
    const f = state.filter.trim();
    const fLow = f.toLowerCase();
    return state.categories.map(cat => planOf(cat, f, fLow)).filter(p => p.shown);
  }

  function signatureOf(list) {
    return list.map(p => `${p.cat.id}>${p.items.map(i => i.id).join(',')}`).join('|');
  }

  /** 结构（哪些类目、每类可见哪几条）变了才重建；重建前记住键盘停在哪一行，重建后放回同一行 */
  function rebuild(list) {
    const focused = root.contains(document.activeElement)
      ? document.activeElement.closest('.node')?.dataset.id : null;
    clear(root);
    nodes.clear();
    re = filterRegExp(state.filter);
    signature = signatureOf(list);

    for (const p of list) {
      const header = CategoryHeader({
        onToggleCheck: id => handlers.onToggleCategory(id),
        onToggleCollapse: id => handlers.onToggleCollapse(id),
        onRestore: id => handlers.onRestoreCategory(id)
      });
      const children = el('div', { class: 'children', attrs: { role: 'group' } });
      const rows = new Map();
      for (const item of p.items) {
        const row = TreeRow({
          onActivate: id => handlers.onSelect(id),
          onToggleCheck: id => handlers.onToggleItem(id),
          onRestore: id => handlers.onRestoreItem(id)
        });
        rows.set(item.id, row);
        children.appendChild(row.el);
      }
      nodes.set(p.cat.id, { header, children, rows });
      root.appendChild(header.el);
      root.appendChild(children);
    }

    tabStopId = focused && findNode(focused) ? focused : null;
    paintAll(list);
    // 只有原来焦点真在树里才抢回来，否则第一次画树不该把焦点从搜索框夺走
    if (focused && tabStopId) findNode(tabStopId).focus();
  }

  function paintAll(list) {
    for (const p of list) {
      const entry = nodes.get(p.cat.id);
      entry.header.update(catData(p.cat));
      entry.children.classList.toggle('hidden', state.collapsed.has(p.cat.id));
      for (const item of p.items) entry.rows.get(item.id).update(itemData(item));
    }
  }

  function findNode(id) {
    for (const entry of nodes.values()) {
      if (entry.header.el.dataset.id === id) return entry.header.el;
      const row = entry.rows.get(id);
      if (row) return row.el;
    }
    return null;
  }

  function visibleNodes() {
    const out = [];
    for (const entry of nodes.values()) {
      out.push(entry.header.el);
      if (entry.children.classList.contains('hidden')) continue;
      for (const row of entry.rows.values()) out.push(row.el);
    }
    return out;
  }

  /** 移动键盘落点：只让一行留在 Tab 序里（roving tabindex），其余行 tabindex="-1" */
  function moveTo(node) {
    if (!node) return;
    const old = tabStopId ? findNode(tabStopId) : null;
    if (old && old !== node) setTabStop(old, false);
    tabStopId = node.dataset.id;
    setTabStop(node, true);
    node.focus();
  }

  function onKeydown(e) {
    const node = e.target.closest('.node');
    if (!node) return;
    const list = visibleNodes();
    const at = list.indexOf(node);
    if (at === -1) return;
    const next = i => list[i];

    switch (e.key) {
      case 'ArrowDown': e.preventDefault(); return moveTo(next(at + 1));
      case 'ArrowUp': e.preventDefault(); return moveTo(next(at - 1));
      case 'Home': e.preventDefault(); return moveTo(next(0));
      case 'End': e.preventDefault(); return moveTo(next(list.length - 1));
      case 'Enter':
      case ' ':
        e.preventDefault();
        // 跟鼠标点同一行一个走法：条目＝选中，类目＝折叠/展开
        if (node.dataset.kind === 'item') handlers.onSelect(node.dataset.id);
        else handlers.onToggleCollapse(node.dataset.id);
        return;
      case 'ArrowRight': {
        if (node.dataset.kind !== 'cat') return;
        if (!state.collapsed.has(node.dataset.id)) return;
        e.preventDefault();
        handlers.onToggleCollapse(node.dataset.id);   // 展开之后 render() 会把条目画全
        return;
      }
      case 'ArrowLeft': {
        if (node.dataset.kind === 'cat') {
          if (!state.collapsed.has(node.dataset.id)) {
            e.preventDefault();
            handlers.onToggleCollapse(node.dataset.id);
          }
          return;
        }
        const cat = node.parentElement && node.parentElement.previousElementSibling;
        if (cat && cat.classList.contains('cat') && !state.collapsed.has(cat.dataset.id)) {
          e.preventDefault();
          handlers.onToggleCollapse(cat.dataset.id);
        }
        return;
      }
      default: return;
    }
  }

  /** 一次全量补画：结构没变就只 update 每一行，变了才重建 */
  function render() {
    const list = plan();
    const top = root.scrollTop;

    if (signatureOf(list) !== signature || nodes.size !== list.length) rebuild(list);
    else re = filterRegExp(state.filter);

    if (!list.length) {
      // 原型 :1831 这一句底下没有按钮（EmptyState 件强制要一颗"下一步"，硬塞就是改功能），所以这里照旧用最矮那条 inline 空态
      clear(root);
      root.appendChild(el('div', { class: 'tree-empty', text: t('tree.empty') }));
      root.scrollTop = top;
      return;
    }
    if (root.querySelector('.tree-empty')) rebuild(list);

    // Tab 进来得有个落点：选中那条优先，否则第一行
    if (!tabStopId || !findNode(tabStopId)) {
      const target = (state.selectedId && findNode(state.selectedId)) || (list.length ? nodes.values().next().value.header.el : null);
      if (target) {
        const old = findNode(tabStopId);
        if (old && old !== target) setTabStop(old, false);
        tabStopId = target.dataset.id;
        setTabStop(target, true);
      }
    }

    paintAll(list);
    root.scrollTop = top;
  }

  /** 当前项滚进可视区（原型 :1837-1846：完全看得见就不动，省掉一次没必要的滚动） */
  function scrollToInt(itemId) {
    if (!itemId) return;
    const node = findNode(itemId);
    if (!node || !node.isConnected) return;
    const nodeRect = node.getBoundingClientRect();
    const treeRect = root.getBoundingClientRect();
    if (nodeRect.top < treeRect.top || nodeRect.bottom > treeRect.bottom) {
      node.scrollIntoView({ block: 'nearest' });
    }
  }

  return { root, render, scrollToInt };
}
