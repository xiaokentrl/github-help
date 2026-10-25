/* Button 按钮（组件库规划.md 4.1）：普通 / 主操作 / 危险 / 弱化四型，常规与小两尺寸。
   干什么用：界面上"点一下做一件事"的那颗。类名照原型（.btn / .primary / .danger / .ghost / .btn-sm），
             这样那份冻结基线的标记能原样复用，逐屏比对才比得出零差异。
   会错在哪：文案一律由调用方传进来，本文件一个字中文不写（8.3）——件里刻死中文，
             "新增一种语言＝只新增一个语言文件"当场作废。
   怎么回来：禁用要说清在等什么，靠 title 传一句话（例如"队列执行中，先等它跑完"）；
             Tooltip 那件一建起来就把 title 换成它（4.11 说的就是这件事）。 */

/**
 * @param {object} o
 * @param {string} o.label      按钮上的文字（调用方给，件不持有文案）
 * @param {string} [o.kind]     '' | 'primary' | 'danger' | 'ghost'
 * @param {string} [o.size]     '' | 'sm'（小尺寸＝原型的 .btn-sm）
 * @param {object} [o.icon]     Icon.js 造出来的节点，放在文字前面
 * @param {string} [o.title]    悬停说明：禁用时写清"在等什么"
 * @param {boolean} [o.disabled]
 * @param {(e:MouseEvent)=>void} [o.onClick]
 */
export function Button(o) {
  const el = document.createElement('button');
  el.type = 'button';
  const spin = document.createElement('i');
  spin.className = 'btn-spin';
  spin.setAttribute('aria-hidden', 'true');
  const text = document.createElement('span');
  text.className = 'btn-label';

  let busy = false;

  function paint() {
    el.className = 'btn'
      + (o.kind ? ' ' + o.kind : '')
      + (o.size ? ' btn-sm' : '')
      + (busy ? ' is-busy' : '');
    text.textContent = o.label == null ? '' : String(o.label);
    if (o.title) el.title = o.title; else el.removeAttribute('title');
    // busy 与 disabled 都进不了 Tab 序；只有 busy 保留 cursor:wait（原型 :35 那颗检查更新就是这个样子）
    el.disabled = busy || !!o.disabled;
    el.setAttribute('aria-busy', busy ? 'true' : 'false');
  }

  el.appendChild(spin);
  if (o.icon) el.appendChild(o.icon.el || o.icon);
  el.appendChild(text);
  el.addEventListener('click', e => {
    if (el.disabled) { e.preventDefault(); return; }
    if (o.onClick) o.onClick(e);
  });
  paint();

  return {
    el,
    /** 只改必要的几项，其余保持上次传进来的值 */
    update(next = {}) {
      Object.assign(o, next);
      paint();
      return this;
    },
    /** 加载中：置灰 + 转圈 + cursor:wait，别再让它吃一次点击 */
    setBusy(on) { busy = !!on; paint(); return this; },
    setDisabled(off) { o.disabled = !!off; paint(); return this; },
    focus() { el.focus(); return this; }
  };
}
