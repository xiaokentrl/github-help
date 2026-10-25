/* 设置卡片那一层的两种排法（一张卡片、卡片里那一行控件），放在这里是因为七个面板都要用同一个形状。
   干什么用：原型 :794-812 那一段是"卡片 → 带装饰符的标题 → 说明 → 一行控件"，这里只把这套排法建一次，
             面板自己只管往里放控件和句子。
   会错在哪：SettingRow 那个库件管的是控件＝开关／复选／下拉／输入框那九处（它会给标题挂 <label for>，
             让点标题就能聚焦控件）。分段选择、"读数＋按钮"、"键＋值"这三类是复合行，那个关联反而会把人带坏：
             label 指到分段里那颗钮上，点一下标题就等于改了一次宽度。所以这三类不走 SettingRow，走这里的排法——
             卡片标题在上方、控件在下方那一行，跟原型逐字一样。
   怎么回来：类名一个不新增，全用 layout.css 第 11 节已经搬好的 .set-card / .set-card-title / .set-card-desc / .set-row；
             装饰符那格的字色也走原型那条 .set-card-title .ico。 */

import { el } from '../../../utils/dom.js';

/** 标题前那个装饰符：纯装饰，读屏跳过（原型 :795 那种 <span class="ico">） */
function iconNode(icon) {
  if (!icon) return null;
  return el('span', { class: 'ico', attrs: { 'aria-hidden': 'true' }, text: String(icon) });
}

/**
 * @param {object} o
 * @param {string} [o.icon]     标题前那个装饰符
 * @param {string} [o.title]    卡片标题；不给就没有标题这一行
 * @param {string} [o.desc]     卡片说明
 * @param {Array} [o.rows]      下面那些行（DOM 节点或带 .el 的件都行）
 * @returns {HTMLElement}
 */
export function setCard(o = {}) {
  const kids = [];
  if (o.title != null) kids.push(el('div', { class: 'set-card-title' }, [iconNode(o.icon), document.createTextNode(o.title)]));
  if (o.desc != null) kids.push(el('p', { class: 'set-card-desc', text: o.desc }));
  for (const row of o.rows || []) kids.push(row.el || row);
  return el('div', { class: 'set-card' }, kids);
}

/** 卡片里那一行：左边那格和右边那格靠 .set-row 的两端对齐，只放一个子节点时它会贴在左边（原型 :829 那种开关行） */
export function setRow(kids) {
  return el('div', { class: 'set-row' }, kids);
}
