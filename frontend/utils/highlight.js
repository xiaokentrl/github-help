/* 搜索命中的那几节字要变色。干什么用：原型 :1771-1772 那两行是「先 escapeHtml，再用正则把命中的片段包进
   <span class="hl">」，包的是字符串——搬进 DOM 拼装之后这一步不能再用 innerHTML，所以改成：
   按命中切成一段段文本节点，命中的那段外面套一个 span。
   会错在哪：① 直接 innerHTML 拼高亮＝把用户输入的字符当标记解析，一个 < 就坏一整行；
             ② 全局正则带 lastIndex，同一个 re 连续 matched 多条时第二行会从上一条结束的位置起算，看着像"只有一行命中"。
   怎么回来：每次比对前把 lastIndex 归零（splitInto 里做），且这里只产出 textContent，不解析任何 HTML。 */

import { escapeRegExp } from './escape.js';

/** 逐字照原型 :1771：空搜索词就没有高亮，非空才建正则 */
export function filterRegExp(filter) {
  const f = (filter || '').trim();
  return f ? new RegExp('(' + escapeRegExp(f) + ')', 'ig') : null;
}

/** 把 text 按 re 拆开塞进 node（node 里的旧内容先清掉）；re 为 null 时就是整段文字 */
export function splitInto(node, text, re) {
  while (node.firstChild) node.removeChild(node.firstChild);
  const value = text == null ? '' : String(text);
  if (!re) {
    node.appendChild(document.createTextNode(value));
    return node;
  }
  re.lastIndex = 0;
  let last = 0;
  let m = re.exec(value);
  while (m) {
    if (m.index > last) node.appendChild(document.createTextNode(value.slice(last, m.index)));
    const mark = document.createElement('span');
    mark.className = 'hl';
    mark.textContent = m[0];
    node.appendChild(mark);
    last = m.index + m[0].length;
    if (m[0].length === 0) re.lastIndex += 1;   // 空命中会原地打转，手动挪一格
    m = re.exec(value);
  }
  if (last < value.length) node.appendChild(document.createTextNode(value.slice(last)));
  return node;
}
