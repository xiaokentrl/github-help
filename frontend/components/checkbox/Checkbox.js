/* Checkbox 复选（组件库规划.md 4.4）：只管"这一条要不要纳入本次操作"，另有半选（indeterminate）。
   干什么用：类目树和队列表里那些勾。原型那两处是 <span class="cb">，既不能 Tab 聚焦也没有 role，
             键盘根本选不中任何域名（:1799、:1816）；这个件把"看得见的方框"跟"真 input"叠在一起，
             外观仍是原型那个 14×14 的 span，键盘与读屏立刻就有主了。
   会错在哪：整行文字要点得中、只有 14 px 的方框能点这两件事必须同时成立——
             所以热区用一层看不见的 24×24 罩子扩出去，而不是把方框本身放大（放大就把树的列宽顶歪了）。
   怎么回来：半选的原型本来就有（一横在 :137，值由 :1302 算），这里保留 data-state 那三个取值，
             同时把 indeterminate 这个真 property 也设上，读屏才知道自己是"部分选中"而不是"没选"。 */

/**
 * @param {object} o
 * @param {'unchecked'|'checked'|'indeterminate'} [o.state]
 * @param {boolean} [o.disabled]
 * @param {string} [o.ariaLabel]  这一勾勾的是什么（由调用方给，件不持有文案）
 * @param {string} [o.title]      禁用时那句"在等什么"（人性化清单第 4 条）
 * @param {(state:string)=>void} [o.onChange] 只在 checked/unchecked 之间来回；半选由上层自己决定下一步
 */
export function Checkbox(o) {
  o.state = o.state || 'unchecked';
  const wrap = document.createElement('label');
  wrap.className = 'cb-wrap';
  const input = document.createElement('input');
  input.type = 'checkbox';
  input.className = 'cb-src';
  const face = document.createElement('span');
  face.className = 'cb';
  wrap.appendChild(input);
  wrap.appendChild(face);

  function paint() {
    face.dataset.state = o.state;
    input.checked = o.state === 'checked';
    input.indeterminate = o.state === 'indeterminate';
    input.disabled = !!o.disabled;
    wrap.classList.toggle('is-disabled', !!o.disabled);
    if (o.ariaLabel) input.setAttribute('aria-label', o.ariaLabel);
    if (o.title) wrap.title = o.title; else wrap.removeAttribute('title');
    input.setAttribute('aria-checked', o.state === 'indeterminate' ? 'mixed' : String(input.checked));
  }

  input.addEventListener('change', () => {
    // 半选被点一下 = 全选，这是树里类目行的常规走法；往哪走由上层定，件只把结果报出去
    o.state = input.checked ? 'checked' : 'unchecked';
    paint();
    if (o.onChange) o.onChange(o.state);
  });
  paint();

  return {
    el: wrap, input, face,
    update(next = {}) { Object.assign(o, next); paint(); return this },
    setState(state) { o.state = state; paint(); return this },
    setDisabled(off) { o.disabled = !!off; paint(); return this },
    focus() { input.focus(); return this }
  };
}
