/* 底部队列那一块（views 层第四块）。干什么用：把原型 :2031-2198 那三段（renderQueueHeader / patchQueueRow / renderQueue）
   收成一个渲染器——头部那一行汇总、正在跑的那一条、四颗操作钮、底下那张表，全在这里。
   会错在哪：① 表格整块重画会把滚动位置弄丢（跑队列时每几百毫秒画一次，表格一直在跳）。
                原型为此留了 queueTableSignature（:2032）与 patchQueueRow（:2071）——照搬成 paintRows 里那两档：
                id 串没变就只 update 每一行，变了才重建。真服务只有一条画路（快照进来→render()），
                所以这里不再单独-export 一支 patchTask：那一档 update 已经在 paintRows 里，多一支就是同两行代码各写一遍；
             ② 表头那四颗钮的禁用条件只在 core/task/gate.js 的 queueButtons() 里算一遍（:2071-2075 原来散在渲染函数里），
                这里再抄一遍就又是 C2 那笔账；
             ③ 「暂停队列／继续队列」是同一颗钮的两种文字（:2083），不是两颗钮；「正在跑的那一条」空了要把内容清空，
                靠样式那条 .q-current:empty 才会整格消失（留着空格子就像界面坏了一块）；
             ④ 全局进度条那根线（:2087-2098）的"跑完再停 1400 毫秒收掉"是队列的语义，可它按已定的外框规矩长在 .toolbar 里
                （绝对定位贴工具栏下沿，位置属于工具栏）。所以这里收的是 main.js 建好的 ProgressBar 件实例，
                队列只把数喂给它，不自己再造第二根线。
   怎么回来：本文件不自己改 store，也不自己跑队列——八个动作全走 main.js 注入的 handlers；
             空态用 EmptyState 的 queue 档（原型 :2136 那两行本来没有钮，所以传 action:null）。 */

import { state, findItem, canRestoreHosts } from '../../core/store.js';
import { queueButtons } from '../../core/task/gate.js';
import {
  taskStatusText, taskStatusDot, taskStatusCls,
  isDetectFinished, isApplyFinished
} from '../../core/task/model.js';
import { Button } from '../../components/button/Button.js';
import { EmptyState } from '../../components/empty-state/EmptyState.js';
import { QueueRow } from './parts/QueueRow.js';
import { el, replace } from '../../utils/dom.js';

/** 原型 :2087 那个"跑完再亮 1400 毫秒"的时长，照抄（改了就是改了演示节奏，不属于本轮） */
const PROGRESS_HOLD_MS = 1400;

/**
 * @param {object} d
 * @param {HTMLElement} d.root        #queuePanel（头部与列表都由这里生成）
 * @param {object} [d.progress]       ProgressBar({kind:'global'}) 那一个件实例：线在工具栏里，数由队列给
 * @param {HTMLElement} [d.resizer]   队列那道横条（DragHandle 建的那一个）：折叠那一刻要收掉（原型 :1457、:1467）
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.handlers  { onSelect?, onPause, onCancel, onClearDone, onRetryFailed,
 *                               onToggleQueue, onRestoreTask, onRetryTask }
 */
export function createQueueView({ root, progress, resizer, t, handlers }) {
  const countEl = el('span', { class: 'qcount', text: '0' });
  const summaryEl = el('span', { class: 'qsummary' });
  const currentEl = el('span', { class: 'q-current' });
  const head = el('div', { class: 'queue-head' });
  const listEl = el('div', { class: 'queue-list' });

  const pauseBtn = Button({ label: t('queue.pause'), onClick: () => handlers.onPause() });
  const cancelBtn = Button({ label: t('queue.cancel'), onClick: () => handlers.onCancel() });
  const clearBtn = Button({ label: t('queue.clearDone'), onClick: () => handlers.onClearDone() });
  const retryBtn = Button({ label: t('queue.retryFailed'), onClick: () => handlers.onRetryFailed() });
  const toggleBtn = Button({
    label: '▾', title: t('queue.collapse'), kind: 'toggle-btn', onClick: () => handlers.onToggleQueue()
  });

  /** cat: 表头那两颗粒「↑ ↓」建的，点了只滚动，不改数据（原型 :2190-2195） */
  function scrollBtn(dir, label, title) {
    const btn = el('button', { class: 'q-scroll', type: 'button', text: label, title, dataset: { scroll: dir } });
    btn.addEventListener('click', () => {
      if (dir === 'top') listEl.scrollTo({ top: 0, behavior: 'smooth' });
      else listEl.scrollTo({ top: listEl.scrollHeight, behavior: 'smooth' });
    });
    return btn;
  }

  replace(head, [
    el('h3', {}, [el('span', { text: t('queue.title') }), countEl]),
    summaryEl, currentEl,
    el('div', { class: 'spacer' }),
    pauseBtn.el, cancelBtn.el, clearBtn.el, retryBtn.el, toggleBtn.el
  ]);
  replace(root, [head, listEl]);

  /** 表头只有建表那一次会走到（结构没变就不碰表头，省得表头跟着抖） */
  function buildTable() {
    return el('table', { class: 'qtable' }, [
      el('thead', {}, el('tr', {}, [
        el('th', { text: '#' }),
        el('th', { text: t('queue.colTask') }),
        el('th', { text: t('queue.colStatus') }),
        el('th', { text: t('queue.colDuration') }),
        el('th', {}, el('div', { class: 'q-col-op' }, [
          el('span', { class: 'lbl', text: t('queue.colAction') }),
          scrollBtn('top', '↑', t('queue.scrollTop')),
          scrollBtn('bottom', '↓', t('queue.scrollBottom'))
        ]))
      ]))
    ]);
  }

  /** 空态那一句建一次就够：每次画都 new 一个的话，队列一直空着时这一格会被反复重造 */
  const empty = EmptyState({
    variant: 'queue', title: t('queue.emptyTitle'), hint: t('queue.emptyHint'), action: null
  });

  /* ---- 一行的数据：三张映射表都从 core/task/model.js 取，这里不自己判状态 ---- */
  const KINDS = {
    detect: { tone: 'detect', key: 'queue.kindDetect' },
    apply: { tone: 'apply', key: 'queue.kindApply' },
    restore: { tone: 'restore', key: 'queue.kindRestore' }
  };

  function rowData(task, index) {
    const kind = KINDS[task.kind];
    if (!kind) throw new Error(`任务 ${task.id} 的种类「${task.kind}」认不出来——core/task/model.js 的 KINDS 里没这一档`);
    const running = task.status === 'running' || task.status === 'restoring';
    let action = null;
    if (task.kind === 'apply' && task.status === 'done') {
      const item = findItem(task.itemId);
      if (item && canRestoreHosts(item)) {
        action = { cls: 'restore', label: t('queue.actRestore'), title: t('queue.actRestoreTitle') };
      }
    } else if (task.kind === 'apply' && ['failed', 'rolledback'].includes(task.status)) {
      action = { cls: 'retry', label: t('queue.actRetry'), title: t('queue.actRetryTitle') };
    }
    return {
      id: task.id,
      index,
      kindText: t(kind.key),
      kindTone: kind.tone,
      name: task.name,
      phaseText: running && task.phaseText ? task.phaseText : '',
      errorMsg: task.errorMsg || '',
      dot: taskStatusDot(task),
      cls: taskStatusCls(task),
      statusText: taskStatusText(task),
      duration: task.duration || '',
      dash: t('common.dash'),
      action
    };
  }

  /* ---- 头部那一行（原型 renderQueueHeader :2031-2099） ---- */
  const rows = new Map();     // taskId → QueueRow
  let signature = null;       // 队列 id 串；null＝表还没建（或现在是空态）
  let progressTimer = null;

  function paintSummary() {
    countEl.textContent = String(state.queue.length);
    const detectTasks = state.queue.filter(x => x.kind === 'detect');
    const applyTasks = state.queue.filter(x => x.kind === 'apply');
    const parts = [];
    if (detectTasks.length) {
      parts.push(t('queue.summaryDetect', {
        done: detectTasks.filter(isDetectFinished).length, total: detectTasks.length
      }));
    }
    if (applyTasks.length) {
      parts.push(t('queue.summaryApply', {
        done: applyTasks.filter(isApplyFinished).length, total: applyTasks.length
      }));
    }
    summaryEl.textContent = parts.join(' · ');
  }

  /** 「正在跑的那一条」（原型 :2060-2070）：转圈 + 「检测中：域名」 + 那句阶段小字 */
  function paintCurrent() {
    const running = state.queue.find(x => x.status === 'running' || x.status === 'restoring');
    if (!running) {
      currentEl.className = 'q-current';
      currentEl.removeAttribute('title');
      replace(currentEl, []);
      return;
    }
    const isDetecting = running.kind === 'detect';
    const prefix = running.status === 'restoring'
      ? t('queue.cur.restore') : (isDetecting ? t('queue.cur.detect') : t('queue.cur.apply'));
    const line = t('queue.cur.line', { prefix, name: running.name });
    currentEl.className = 'q-current' + (isDetecting ? ' detecting' : '');
    currentEl.title = running.phaseText ? `${line} · ${running.phaseText}` : line;
    replace(currentEl, [
      el('span', { class: 'spin', attrs: { 'aria-hidden': 'true' } }),
      el('span', { text: `${prefix}：` }),
      el('span', { class: 'nm', text: running.name }),
      running.phaseText ? el('span', { class: 'ph', text: `· ${running.phaseText}` }) : null
    ]);
  }

  function paintButtons() {
    const b = queueButtons(state);
    pauseBtn.setDisabled(b.pauseDisabled);
    pauseBtn.update({ label: state.paused ? t('queue.resume') : t('queue.pause') });
    cancelBtn.setDisabled(b.cancelDisabled);
    clearBtn.setDisabled(b.clearDisabled);
    retryBtn.setDisabled(b.retryDisabled);
  }

  /** 全局进度那根线（原型 :2087-2098 三条分支一个不少）：跑完先亮满再收，收之前看一眼是不是又开始了。
   *  线与 <i> 的几何、.show 的显隐都归 ProgressBar 件，这里只把队列算出来的数交给它。 */
  function paintProgress() {
    if (!progress) return;
    const total = state.queue.length;
    const finished = state.queue.filter(x => {
      if (x.kind === 'detect') return isDetectFinished(x);
      return ['done', 'failed', 'skipped', 'rolledback'].includes(x.status);
    }).length;

    if (total > 0 && (state.queueRunning || state.batchDetecting || finished < total)) {
      progress.update({ value: finished, max: total });
      progress.show(true);
    } else if (finished > 0 && finished === total) {
      progress.update({ value: 1, max: 1 });
      progress.show(true);
      if (progressTimer) clearTimeout(progressTimer);
      progressTimer = setTimeout(() => {
        if (!state.queueRunning && !state.batchDetecting) progress.show(false);
      }, PROGRESS_HOLD_MS);
    } else {
      progress.update({ value: 0, max: 1 });
      progress.show(false);
    }
  }

  /* ---- 那张表 ---- */
  function paintRows() {
    const sig = state.queue.map(x => x.id).join('|');
    if (!state.queue.length) {
      signature = null;
      rows.clear();
      replace(listEl, empty.el);
      return;
    }
    if (sig !== signature || !listEl.querySelector('table.qtable')) {
      const prevTop = listEl.scrollTop;
      rows.clear();
      const table = buildTable();
      const tbody = el('tbody');
      state.queue.forEach((task, i) => {
        const row = QueueRow({
          onRestore: id => handlers.onRestoreTask(id),
          onRetry: id => handlers.onRetryTask(id)
        });
        row.update(rowData(task, i));
        rows.set(task.id, row);
        tbody.appendChild(row.el);
      });
      table.appendChild(tbody);
      replace(listEl, table);
      listEl.scrollTop = Math.min(prevTop, listEl.scrollHeight);
      signature = sig;
      return;
    }
    state.queue.forEach((task, i) => {
      const row = rows.get(task.id);
      if (row) row.update(rowData(task, i));
    });
  }

  /** 折叠与展开（原型 toggleQueueCollapsed :1456-1471）：高度、那道横条、那颗钮的字与说明一起改 */
  function paintCollapsed() {
    root.classList.toggle('collapsed', !!state.queueCollapsed);
    if (resizer) resizer.style.display = state.queueCollapsed ? 'none' : '';
    toggleBtn.update({
      label: state.queueCollapsed ? '▴' : '▾',
      title: state.queueCollapsed ? t('queue.expand') : t('queue.collapse')
    });
    if (!state.queueCollapsed) root.style.height = `${state.settings.queueHeight}px`;
  }

  function render() {
    paintSummary();
    paintCurrent();
    paintButtons();
    paintProgress();
    paintRows();
    paintCollapsed();
  }

  render();
  return { root, render, listEl };
}
