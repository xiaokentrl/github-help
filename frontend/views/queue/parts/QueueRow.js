/* 队列里"一个任务"那一行（组件库规划.md 4.15 的 QueueRow）。干什么用：把原型 :2146-2174 那段模板变成一个能单行更新的对象，
   对应原型的 patchQueueRow(:2071-2103)——跑队列时每几百毫秒只改变了的那一行，整张表不重画。
   会错在哪：① 这一行认得"检测/应用/恢复"这三种任务，所以进不了 components/（进了库，改一行队列要回归四个视图）；
             ② 状态那一格的字色靠 .qstatus.<cls>（layout.css 那 12 条），点靠 .dot.<state>：两套类名分别来自
                core/task/model.js 的 taskStatusCls 与 taskStatusDot，传错一个不会报错，只会那一格悄悄变成灰字；
             ③ 「恢复」「重试」两颗钮按原型是"该有才生成"（:2098-2102），不是画出来再藏——藏着的钮还在 Tab 序里，
                键盘走到一颗按下去没反应的钮。
   怎么回来：只有一个 update(data)，data 形状见下面注释；每句中文（含悬停说明）由调用方从 i18n 取好传进来。 */

import { StatusDot } from '../../../components/status-dot/StatusDot.js';
import { Tag } from '../../../components/tag/Tag.js';
import { el, replace } from '../../../utils/dom.js';

/**
 * @param {object} h
 * @param {(id:string)=>void} h.onRestore  点「恢复」
 * @param {(id:string)=>void} h.onRetry    点「重试」
 */
export function QueueRow(h) {
  const idx = el('td', { class: 'q-idx' });
  const kindTag = el('span');
  const nameEl = document.createTextNode('');
  const taskLabel = el('span', {}, [kindTag, nameEl]);
  const phaseEl = el('span', { class: 'phase' });
  const errEl = el('span', { class: 'err' });
  const taskWrap = el('div', { class: 'q-task' }, taskLabel);
  const task = el('td', {}, taskWrap);
  const dot = StatusDot({ state: 'unset', size: 'inline' });
  const dur = el('td', { class: 'q-dur' });
  const op = el('td', { class: 'q-op' });
  const node = el('tr', {}, [idx, task, el('td', {}, dot.el), dur, op]);

  function actionButton(cls, label, title, onClick) {
    const btn = el('button', { class: 'q-act ' + cls, type: 'button', text: label, title });
    btn.addEventListener('click', () => onClick(data.id));
    return btn;
  }

  let data = {};

  function paint() {
    if (!data.id) throw new Error('QueueRow.update 少了 id：这一行不知道自己是哪个任务，点"恢复"会恢复到别的条目上');
    if (data.statusText == null) throw new Error(`QueueRow.update(${data.id}) 少了 statusText：状态那一格会空着，读屏也念不出这一条跑到哪了`);
    node.dataset.taskId = data.id;

    idx.textContent = String(data.index + 1);
    nameEl.nodeValue = data.name == null ? '' : String(data.name);
    // 种类小标的类名与文字成对来（原型 :2066-2069）：tone 认不出时 Tag 自己抛错
    kindTag.replaceChildren(Tag({ text: data.kindText, tone: data.kindTone, size: 'legacy' }).el);

    // 阶段小字只在跑着的时候有（原型 :2059 那句判据），错误正文就是那句 errorMsg；两个节点建一次，该没有时摘掉
    phaseEl.textContent = data.phaseText || '';
    errEl.textContent = data.errorMsg || '';
    replace(taskWrap, [taskLabel, data.phaseText ? phaseEl : null, data.errorMsg ? errEl : null]);

    dot.update({ state: data.dot, cls: data.cls, label: data.statusText });
    dur.textContent = data.duration || data.dash;

    const action = data.action;
    replace(op, action
      ? actionButton(action.cls, action.label, action.title, action.cls === 'retry' ? h.onRetry : h.onRestore)
      : []);
  }

  return {
    el: node,
    update(next) { data = Object.assign(data, next); paint(); return this }
  };
}
