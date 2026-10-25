/* EmptyState 空态（组件库规划.md 4.14）：没数据／没结果／没选中时那一句人话，外加一个能点的下一步。
   干什么用：三处空态（树无匹配、详情未选、队列为空）原来各写各的 div，将来"下一步按钮"只会两处有、第三处漏。
   会错在哪：只写"队列为空"三个字，用户不知道下一步点哪；三处各写一遍，改文案要跑三个地方。
   怎么回来：这一件把"标题 + 一句提示 +（可给的）下一步"收成一处——`action` 给 { label, onClick } 就带一颗钮，
             写 null 就是明确说这一条没有下一步（详情 :1907 与队列 :2136 那两处原型本来只有字，硬塞钮＝改功能）；
             两条都不写＝漏传，当场抛错。档位名不认识也抛错，宁可红也不画出一个对不上号的壳。
             文案（含按钮的字）全部由调用方给，件里不写中文。 */

import { Button } from '../button/Button.js';
import { Icon } from '../icon/Icon.js';

/** 三档 = 原型那三套容器名与小字类名，逐字照 :89 / :199-201 / :280-281（改了逐屏比对就对不上） */
const VARIANTS = {
  panel: { box: 'detail-empty', big: true, hint: 'hint' },
  queue: { box: 'queue-empty', big: false, hint: 'small' },
  inline: { box: 'tree-empty', big: false, hint: null }
};

/**
 * @param {object} o
 * @param {string} o.title            那一句人话（调用方给）
 * @param {string} [o.hint]           补充说明，小一号灰字
 * @param {object|null} o.action      下一步：{ label, onClick, kind }；null＝这一条没有下一步（键必须存在）
 * @param {'panel'|'queue'|'inline'} [o.variant]  panel＝详情那块高的；queue＝队列表那两行矮的；inline＝树里那一行
 * @param {string} [o.icon]           Icon 的名字（例如 'list'）；不给就只留文字
 */
export function EmptyState(o) {
  if (!('action' in o)) {
    throw new Error('EmptyState 必须写明有没有下一步：给 action:{label, onClick}，或给 action:null 表示这一条只把话说完');
  }
  if (o.action !== null && !o.action.label) {
    throw new Error('EmptyState 的 action 少了 label：只给一颗没字的钮，用户不知道按下去做什么');
  }
  const variant = o.variant || 'panel';
  const shape = VARIANTS[variant];
  if (!shape) throw new Error(`EmptyState 不认识档位「${variant}」，能用的是：${Object.keys(VARIANTS).join(' / ')}`);

  const el = document.createElement('div');
  const big = document.createElement('div');
  big.className = 'big';
  if (o.icon) big.appendChild(Icon({ name: o.icon, size: 'lg', label: '' }).el);
  const title = document.createElement('div');
  const hint = document.createElement('div');
  if (shape.hint) hint.className = shape.hint;
  const action = o.action === null ? null : Button({
    label: o.action.label, kind: o.action.kind || 'primary', size: 'sm', onClick: o.action.onClick
  });

  if (shape.big) el.appendChild(big);
  el.appendChild(title);
  if (shape.hint) el.appendChild(hint);
  if (action) el.appendChild(action.el);

  function paint() {
    el.className = shape.box;
    title.textContent = o.title == null ? '' : String(o.title);
    hint.textContent = o.hint == null ? '' : String(o.hint);
    hint.style.display = o.hint ? '' : 'none';
    big.style.display = o.icon ? '' : 'none';
  }

  paint();
  return {
    el,
    update(next = {}) {
      Object.assign(o, next);
      paint();
      if (action && next.action) action.update(next.action);
      return this;
    },
    setTitle(t) { o.title = t; paint(); return this },
    setHint(t) { o.hint = t; paint(); return this }
  };
}
