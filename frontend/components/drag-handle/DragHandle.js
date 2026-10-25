/* DragHandle 拖拽把手（组件库规划.md 4.15）：拖着改侧栏宽度、队列高度。
   干什么用：原型这里有**两套各写各的**（侧栏 :1359-1387 与队列 :1402 起），边界、dragging 类、收尾清理都抄了两遍。
   会错在哪：两套各算各的上下限——改一处忘另一处，就出现"一个能拖一个拖不动"；
             拖过最小宽度布局会挤坏又回不来；键盘完全做不到（原型这两套只监听鼠标，也没有 tabindex）。
   怎么回来：收成一个件。上下限由调用方传进来（min/max 都在件里钳，越过回不来这件事不可能发生）；
             拖动中把指针捕获到把手上（setPointerCapture），鼠标移出界面、窗口失焦都能正常收尾；
             把手带 role=separator + tabindex=0，方向键按 8px 步进，Home/End 直接到两端；
             拖动过程中只回调 onResize（跟手），松手那一下才 onCommit（写设置并持久化）——
             这两下分开，才不会拖一格存一次盘。 */

/** 轴 → 原型那两套的类名与光标，保持一一对上，逐屏比对才有原样 */
const AXES = {
  x: { cls: 'sidebar-resizer', cursor: 'col-resize', orientation: 'horizontal' },
  y: { cls: 'resizer', cursor: 'row-resize', orientation: 'vertical' }
};

const STEP = 8;   // 键盘步进（第 4 节 4.15 定的那个 8px）

/**
 * @param {object} o
 * @param {'x'|'y'} [o.axis]  x＝拖宽度（侧栏那道），y＝拖高度（队列那道），缺省 'x'
 * @param {number} o.min
 * @param {number} o.max
 * @param {()=>number} o.valueOf     当前尺寸（px）：件不持有目标元素，谁被拖谁自己报当前值
 * @param {(px:number)=>void} [o.onResize]  跟手：每帧一次，只改界面
 * @param {(px:number)=>void} o.onCommit     收尾：松手/按方向键之后各一次，写设置并持久化
 * @param {string} [o.ariaLabel]
 */
export function DragHandle(o) {
  const axis = AXES[o.axis || 'x'];
  if (!axis) throw new Error(`DragHandle 不认识轴「${o.axis}」，能用的是：x（拖宽度）/ y（拖高度）`);
  if (!(o.max > o.min)) throw new Error(`DragHandle 的上下限给反了或给了同一个数：min=${o.min} max=${o.max}——这样一拖就卡死`);

  const el = document.createElement('div');
  let drag = null;
  let rafPending = false;

  function clamp(px) { return Math.max(o.min, Math.min(o.max, px)); }

  function paintSize(px) {
    el.setAttribute('aria-valuenow', String(Math.round(px)));
    el.setAttribute('aria-valuemin', String(o.min));
    el.setAttribute('aria-valuemax', String(o.max));
    el.setAttribute('aria-valuetext', String(Math.round(px)));
  }

  function apply(px, commit) {
    const v = clamp(px);
    paintSize(v);
    if (o.onResize) o.onResize(v);
    if (commit && o.onCommit) o.onCommit(v);
    return v;
  }

  el.className = axis.cls;
  el.setAttribute('role', 'separator');
  el.setAttribute('aria-orientation', axis.orientation);
  el.tabIndex = 0;
  if (o.ariaLabel) el.setAttribute('aria-label', o.ariaLabel);

  el.addEventListener('pointerdown', e => {
    e.preventDefault();
    el.setPointerCapture(e.pointerId);
    const start = o.axis === 'y' ? e.clientY : e.clientX;
    // 队列那一套是"往上拖变高"，所以 y 轴取反；这条差异以前藏在两套各自的代码里
    const dir = o.axis === 'y' ? -1 : 1;
    drag = { start, base: o.valueOf(), dir, last: start };
    el.classList.add('dragging');
    document.body.style.cursor = axis.cursor;
    document.body.style.userSelect = 'none';
  });

  el.addEventListener('pointermove', e => {
    if (!drag) return;
    drag.last = o.axis === 'y' ? e.clientY : e.clientX;
    if (rafPending) return;
    rafPending = true;
    requestAnimationFrame(() => {
      rafPending = false;
      if (!drag) return;
      apply(drag.base + dirDelta(drag), false);
    });
  });

  function dirDelta(d) { return d.dir * (d.last - d.start); }

  function endDrag(e) {
    if (!drag) return;
    const px = clamp(drag.base + dirDelta(drag));
    drag = null;
    rafPending = false;
    el.classList.remove('dragging');
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    if (e && e.pointerId !== undefined && el.hasPointerCapture(e.pointerId)) el.releasePointerCapture(e.pointerId);
    paintSize(px);
    // 收尾只在松手这一下写盘：拖一格存一次会把设置文件写爆
    if (o.onCommit) o.onCommit(px);
  }

  el.addEventListener('pointerup', endDrag);
  el.addEventListener('pointercancel', endDrag);
  el.addEventListener('lostpointercapture', () => { if (drag) endDrag(null); });
  window.addEventListener('blur', () => { if (drag) endDrag(null); });   // 拖到窗口外松手：原型这套漏过，收尾会卡在 dragging

  el.addEventListener('keydown', e => {
    const cur = o.valueOf();
    let next = null;
    if (e.key === 'ArrowLeft' || e.key === 'ArrowDown') next = cur - STEP;
    else if (e.key === 'ArrowRight' || e.key === 'ArrowUp') next = cur + STEP;
    else if (e.key === 'Home') next = o.min;
    else if (e.key === 'End') next = o.max;
    else if (e.key === 'PageDown') next = cur - STEP * 4;
    else if (e.key === 'PageUp') next = cur + STEP * 4;
    else return;
    e.preventDefault();
    apply(next, true);          // 键盘没有"松手"这一下，每步就直接收尾
  });

  paintSize(o.valueOf());

  return {
    el,
    update(next = {}) { Object.assign(o, next); paintSize(o.valueOf()); return this },
    setBounds(min, max) { o.min = min; o.max = max; if (!(max > min)) throw new Error('DragHandle 上下限又给反了'); paintSize(o.valueOf()); return this },
    focus() { el.focus(); return this }
  };
}
