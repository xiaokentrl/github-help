/* 造 DOM 的三下五除二。干什么用：views 与行级件都要"建一个标签、挂类名、塞子节点"，
   原型那一套是字符串模板加 innerHTML，搬进来就等于把转义的责任推给每一个写模板的人。
   会错在哪：① 各处自己写 helper，三个月后有两套 class 拼法，一处漏了 null 判断就画出 "undefined"；
             ② 用 innerHTML 拼用户数据（域名、条目名）——一个尖括号就能把整棵树的骨架顶坏。
   怎么回来：本文件是 views 层唯一的建节点入口，写完就这一份；
             尺子在 tests/components.test.mjs（件那一层）与 tests/structure.test.mjs（menu 那一层），
             views 这一层还没进尺子。
             这段注释本来写的是目录通配（两个星号紧挨一个斜杠）——那个写法会当场把块注释截断，
             剩下的半句变成代码，浏览器报「parts is not defined」。写中文注释别在星号后面紧跟斜杠。 */

/**
 * @param {string} tag
 * @param {object} [o]
 * @param {string} [o.class]        整串类名（跟原型的 class 逐字对齐时用这个）
 * @param {string} [o.text]         纯文本内容（走 textContent，不解析）
 * @param {string} [o.title]        悬停说明
 * @param {object} [o.dataset]      { restoreItem: 'x' } → data-restore-item="x"
 * @param {object} [o.attrs]        其余属性，按属性名原样设（aria-*、colspan、type…）
 * @param {string} [o.style]        内联样式：只给原型本来就用内联的那几处（进度条宽度等动态值）
 * @param {boolean} [o.hidden]
 * @returns {HTMLElement}
 */
export function el(tag, o = {}, kids = []) {
  const node = document.createElement(tag);
  if (o.class != null) node.className = o.class;
  if (o.text != null) node.textContent = String(o.text);
  if (o.title != null) node.title = o.title;
  if (o.style != null) node.setAttribute('style', o.style);
  if (o.hidden) node.hidden = true;
  if (o.dataset) for (const [k, v] of Object.entries(o.dataset)) node.dataset[k] = String(v);
  if (o.attrs) for (const [k, v] of Object.entries(o.attrs)) {
    if (v === null || v === undefined) node.removeAttribute(k);
    else node.setAttribute(k, String(v));
  }
  for (const kid of [].concat(kids)) {
    if (kid === null || kid === undefined || kid === false) continue;
    node.appendChild(kid instanceof Node ? kid : kid.el);
  }
  return node;
}

/** 清空一个容器（不碰它自己的类名） */
export function clear(node) {
  while (node.firstChild) node.removeChild(node.firstChild);
  return node;
}

/** 挂一串子节点，先清旧的；件里"整格换掉"用它，比各自 remove 少一处漏清 */
export function replace(node, kids) {
  clear(node);
  for (const kid of [].concat(kids)) {
    if (kid === null || kid === undefined || kid === false) continue;
    node.appendChild(kid instanceof Node ? kid : kid.el);
  }
  return node;
}

/** 给一行加进/挪出焦点序列：树上一次只有一个 tabindex="0"（其余 -1），方向键换焦点时跟着挪 */
export function setTabStop(node, on) {
  node.setAttribute('tabindex', on ? '0' : '-1');
  return node;
}
