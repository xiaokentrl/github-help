/* 给单测用的一小套 DOM。干什么用：views 与件那一层的代码是"建节点、挂类名、听事件"，
   在 node 里跑不起来——本文件把这一小撮 API 补齐，让用例能真的 import 一个面板、build() 一次、点一下、看结果。
   为什么要有它：2026-10-22 这一轮，「检测升级」面板里有一句引用了根本不存在的变量，
   它在浏览器里炸的是整个 main.js（树也不画、按钮也不接），而当时 92 条用例全绿——
   因为没有任何一条用例真去 build 过一个面板。本文件就是为了让"build 一次"能在 node 里跑。
   会错在哪：① 这是"够用就行"的一小套，不是浏览器的替身：没有布局、没有 computed style、
                选择器只认「标签／.类／#id／[属性]／[属性=值]」这几种，用空格表示后代，逗号表示或；
                样式对不对、颜色够不够、点得中点不中，仍然只有真浏览器说得准（判据见 前端目录结构规划.md 第 8 节）。
             ② 事件只走"自己→往上冒，冒到头再交给 document"这一条（浏览器就是这么走的）：
                capture 不管、焦点陷阱不管；stopPropagation 认——中途有人喊停就不再往上、也不交给 document，
                托盘胶囊那颗钮就是靠这一条才不会"点开的同一下又被外面那条监听收掉"。
             ③ document 上那几条监听是全局的：同一个测试文件里建第二个 view 就多一条监听，
                它只管自己那块（关自己的菜单），彼此不动。真要干净就调 removeEventListener。
             ④ 它装在 globalThis 上，只应在 node 测试进程里用；界面代码不许 import 它。
   怎么回来：界面用到本文件没实现的 DOM 能力时，用例当场报"xxx is not a function"——
             要么那处该走已有的能力，要么在本文件补最小实现并把它写进上面那份能力清单。 */

class StubNode {
  constructor() {
    this.childNodes = [];
    this.parentNode = null;
  }

  get firstChild() { return this.childNodes[0] || null; }

  /** 浏览器里 children 只数元素、textContent 数全部；界面代码用前者挑行，这里跟着分清楚 */
  get children() { return this.childNodes.filter(k => k instanceof StubElement); }

  appendChild(node) {
    if (node === null || node === undefined) throw new Error('appendChild 收到空节点：多半是某个件没返回 .el');
    if (node.parentNode) node.parentNode.removeChild(node);
    node.parentNode = this;
    this.childNodes.push(node);
    return node;
  }

  removeChild(node) {
    const at = this.childNodes.indexOf(node);
    if (at >= 0) this.childNodes.splice(at, 1);
    node.parentNode = null;
    return node;
  }

  remove() { if (this.parentNode) this.parentNode.removeChild(this); }

  replaceChildren(...kids) {
    this.childNodes.length = 0;
    for (const kid of kids) this.appendChild(kid);
  }

  contains(node) {
    for (let cur = node; cur; cur = cur.parentNode) if (cur === this) return true;
    return false;
  }

  get textContent() { return this.childNodes.map(k => k.textContent).join(''); }

  set textContent(text) {
    this.childNodes = [];
    if (text !== '' && text != null) this.appendChild(new StubText(String(text)));
  }

  /** 深度优先把后代列出来（querySelectorAll 的地基） */
  * walk() {
    for (const kid of this.childNodes) {
      yield kid;
      yield* kid.walk();
    }
  }
}

class StubText extends StubNode {
  constructor(text) {
    super();
    this.text = String(text);
  }
  get textContent() { return this.text; }
  set textContent(v) { this.text = String(v); }
}

/** 选择器只支持这几种形状：'div'、'.a.b'、'#id'、'[attr]'、'[attr=value]'，空格＝后代，逗号＝或 */
function parseSelector(sel) {
  return sel.split(',').map(alt => alt.trim().split(/\s+/).map(part => {
    const compound = [];
    const re = /([a-zA-Z][\w-]*)|^\.([\w-]+)|^#([\w-]+)|^\[([\w-]+)(?:=([^\]]+))?\]/;
    let rest = part;
    while (rest.length) {
      const m = rest.match(re);
      if (!m) throw new Error(`tests/lib/dom-stub.mjs 的选择器只到这一步就认不出了：${part}`);
      if (m[1]) compound.push({ kind: 'tag', value: m[1].toLowerCase() });
      else if (m[2]) compound.push({ kind: 'class', value: m[2] });
      else if (m[3]) compound.push({ kind: 'id', value: m[3] });
      else compound.push({ kind: 'attr', value: m[4], eq: m[5] == null ? null : m[5].replace(/^["']|["']$/g, '') });
      rest = rest.slice(m[0].length);
    }
    return compound;
  }));
}

function matchCompound(node, compound) {
  if (!(node instanceof StubElement)) return false;
  for (const c of compound) {
    if (c.kind === 'tag' && node.tagName.toLowerCase() !== c.value) return false;
    if (c.kind === 'class' && !node.classList.contains(c.value)) return false;
    if (c.kind === 'id' && node.id !== c.value) return false;
    if (c.kind === 'attr') {
      const v = node.getAttribute(c.value);
      if (v == null) return false;
      if (c.eq != null && v !== c.eq) return false;
    }
  }
  return true;
}

class StubElement extends StubNode {
  constructor(tag) {
    super();
    this.tagName = String(tag).toUpperCase();
    this.attributes = new Map();
    this.dataset = {};
    this.listeners = new Map();
    this.style = {};
    this._classes = [];
    this.disabled = false;
    this.hidden = false;
    this.value = '';
    this._id = '';
  }

  get className() { return this._classes.join(' '); }
  set className(text) { this._classes = String(text).split(/\s+/).filter(Boolean); }

  get id() { return this._id; }
  set id(v) { this._id = String(v); this.attributes.set('id', this._id); }

  get classList() {
    const self = this;
    return {
      add(...names) { for (const n of names) if (!self._classes.includes(n)) self._classes.push(n); },
      remove(...names) { self._classes = self._classes.filter(c => !names.includes(c)); },
      contains(name) { return self._classes.includes(name); },
      toggle(name, force) {
        const on = force === undefined ? !self._classes.includes(name) : !!force;
        if (on) { if (!self._classes.includes(name)) self._classes.push(name); }
        else self._classes = self._classes.filter(c => c !== name);
        return on;
      }
    };
  }

  setAttribute(name, value) {
    const v = String(value);
    this.attributes.set(name, v);
    if (name === 'class') this.className = v;
    if (name === 'id') this._id = v;
    if (name === 'role') this.role = v;
    if (name.startsWith('data-')) {
      const camel = name.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase());
      this.dataset[camel] = v;
    }
  }

  getAttribute(name) {
    if (name === 'class') return this.className || null;
    if (name === 'id') return this._id || null;
    if (this.attributes.has(name)) return this.attributes.get(name);
    if (name.startsWith('data-')) {
      const camel = name.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase());
      return this.dataset[camel] === undefined ? null : String(this.dataset[camel]);
    }
    return null;
  }

  hasAttribute(name) { return this.getAttribute(name) != null; }
  removeAttribute(name) { this.attributes.delete(name); if (name.startsWith('data-')) { const camel = name.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase()); delete this.dataset[camel]; } }

  addEventListener(type, fn) {
    if (!this.listeners.has(type)) this.listeners.set(type, []);
    this.listeners.get(type).push(fn);
  }

  removeEventListener(type, fn) {
    const list = this.listeners.get(type);
    if (list) this.listeners.set(type, list.filter(f => f !== fn));
  }

  /** 冒泡：自己→父→…→文档头；中途有人 stopPropagation 就停，也不再交给 document（浏览器同理） */
  dispatch(type, extra = {}) {
    const event = { type, target: this, preventDefault() { event.defaultPrevented = true; }, stopPropagation() { event.propagationStopped = true; }, ...extra };
    for (let cur = this; cur; cur = cur.parentNode) {
      const list = cur.listeners && cur.listeners.get(type);
      if (list) for (const fn of list.slice()) fn.call(cur, event);
      if (event.propagationStopped) return event;
    }
    emitToDocument(event);
    return event;
  }

  click() { if (!this.disabled) this.dispatch('click'); }
  focus() { globalThis.document.activeElement = this; }
  blur() { }

  matches(selector) {
    return parseSelector(selector).some(alt => matchCompound(this, alt[alt.length - 1]));
  }

  closest(selector) {
    const alts = parseSelector(selector);
    for (let cur = this; cur; cur = cur.parentNode) {
      if (alts.some(alt => matchCompound(cur, alt[alt.length - 1]))) return cur;
    }
    return null;
  }

  querySelectorAll(selector) {
    const alts = parseSelector(selector);
    const out = [];
    for (const node of this.walk()) {
      if (alts.some(alt => matchesChain(node, alt))) out.push(node);
    }
    return out;
  }

  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }

  get options() { return this.querySelectorAll('option'); }
}

/** 后代链：从最后一个 compound 往前，逐级在祖先里找得到就算命中 */
function matchesChain(node, alt) {
  if (!matchCompound(node, alt[alt.length - 1])) return false;
  let at = alt.length - 2;
  let cur = node.parentNode;
  while (at >= 0) {
    let found = false;
    for (; cur; cur = cur.parentNode) {
      if (matchCompound(cur, alt[at])) { found = true; cur = cur.parentNode; break; }
    }
    if (!found) return false;
    at--;
  }
  return true;
}

/** type 与 title 这类属性，界面代码是当属性写的（input.type = 'checkbox'、node.title = 那句悬停说明）。
    浏览器里这么写会同步进属性、getAttribute 看得见、选择器 [type=checkbox] 也选得中；
    本文件不镜像它们，用例只能退成"抓所有 input／不看 title"，断言跟着变松（工具栏那两套 title 就验不了）。 */
for (const name of ['type', 'title']) {
  Object.defineProperty(StubElement.prototype, name, {
    get() { return this.attributes.get(name) === undefined ? '' : this.attributes.get(name); },
    set(v) { this.attributes.set(name, String(v)); },
    configurable: true,
    enumerable: true
  });
}

/** document 上挂的监听（installDom 之后才有人能用；见文件头第 ③ 条） */
const docListeners = new Map();

/** 事件冒到头之后交给 document——只有已经挂在文档树上的节点才走这一步（浏览器同理） */
function emitToDocument(event) {
  const doc = globalThis.document;
  if (!doc || !doc.documentElement.contains(event.target)) return;
  const list = docListeners.get(event.type);
  if (!list) return;
  for (const fn of list.slice()) fn.call(doc, event);
}

/** 装一次全局 DOM（每个测试文件一个 node 进程，装完就用，不用卸载） */
export function installDom() {
  globalThis.Node = StubNode;
  globalThis.Text = StubText;
  globalThis.Element = StubElement;
  globalThis.HTMLElement = StubElement;

  const documentElement = new StubElement('html');
  const body = new StubElement('body');
  documentElement.appendChild(body);

  const document = {
    documentElement,
    body,
    activeElement: body,
    createElement: tag => new StubElement(tag),
    createElementNS: (_ns, tag) => new StubElement(tag),
    createTextNode: text => new StubText(text),
    createDocumentFragment: () => new StubNode(),
    addEventListener(type, fn) {
      if (!docListeners.has(type)) docListeners.set(type, []);
      docListeners.get(type).push(fn);
    },
    removeEventListener(type, fn) {
      const list = docListeners.get(type);
      if (list) docListeners.set(type, list.filter(f => f !== fn));
    },
    contains: node => documentElement.contains(node)
  };
  globalThis.document = document;
  if (!globalThis.window) {
    globalThis.window = { innerHeight: 900, addEventListener() { }, removeEventListener() { }, document };
  }
  if (!globalThis.localStorage) {
    const bag = new Map();
    globalThis.localStorage = {
      getItem: k => (bag.has(k) ? bag.get(k) : null),
      setItem: (k, v) => bag.set(k, String(v)),
      removeItem: k => bag.delete(k),
      clear: () => bag.clear()
    };
  }
  return { document, body, documentElement };
}

/** 把一棵子树的可见文字抠出来，给断言消息用（出错时要能说清是哪一格，别只说"不对"） */
export function textOf(node) {
  return node.textContent.replace(/\s+/g, ' ').trim();
}
