/* Segmented 分段选择（组件库规划.md 4.3）：二选一/三选一，类名照原型 .seg / .seg button.active。
   干什么用：把"选一档"这件事收成一个件，只暴露 value 与 onChange。
   会错在哪：原型里选中态靠视图手工 `syncLayoutSegButtons()` 去 toggle class（:1509），
             于是"看着选了其实没存"这类不同步就是这么来的；件自己绝不改状态，只把选择报出去。
   怎么回来：键盘必须能切段（原型只有鼠标）。左右方向键换段、Home/End 到两端，焦点环走 base.css 那一条。 */

/**
 * @param {object} o
 * @param {Array<{value:string,label:string,title?:string}>} o.options 段清单，文字由调用方给
 * @param {string} o.value        当前选中值
 * @param {(value:string)=>void} o.onChange
 * @param {string} [o.ariaLabel]  这一组是干什么的（无障碍名，由调用方给）
 * @param {string} [o.title]      禁用时那句"在等什么"（人性化清单第 4 条）
 * @param {boolean} [o.disabled]
 */
export function Segmented(o) {
  const el = document.createElement('div');
  el.className = 'seg';
  el.setAttribute('role', 'radiogroup');
  if (o.ariaLabel) el.setAttribute('aria-label', o.ariaLabel);
  const buttons = o.options.map(opt => {
    const b = document.createElement('button');
    b.type = 'button';
    b.textContent = opt.label == null ? '' : String(opt.label);
    if (opt.title) b.title = opt.title;
    b.setAttribute('role', 'radio');
    b.dataset.value = opt.value;
    b.addEventListener('click', () => choose(opt.value));
    el.appendChild(b);
    return b;
  });

  function choose(value) {
    if (o.value === value) return;
    o.value = value;
    paint();
    if (o.onChange) o.onChange(value);
  }

  function paint() {
    // 禁用那句"在等什么"挂在组上：这一组每一段都在等同一件事，逐颗挂只是把同一句话抄四遍
    if (o.title) el.title = o.title; else el.removeAttribute('title');
    buttons.forEach(b => {
      const on = b.dataset.value === o.value;
      b.classList.toggle('active', on);
      b.setAttribute('aria-checked', on ? 'true' : 'false');
      b.tabIndex = on ? 0 : -1;                       // roving tabindex：Tab 只落进选中的那一段
      b.disabled = !!o.disabled;
    });
  }

  el.addEventListener('keydown', e => {
    const i = buttons.findIndex(b => b.dataset.value === o.value);
    const step = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
    let next = -1;
    if (step) next = (i + step + buttons.length) % buttons.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = buttons.length - 1;
    if (next < 0) return;
    e.preventDefault();
    choose(buttons[next].dataset.value);
    buttons[next].focus();
  });

  paint();
  return {
    el,
    setValue: choose,
    setDisabled(off) { o.disabled = !!off; paint(); return this },
    focus() { (buttons.find(b => b.classList.contains('active')) || buttons[0]).focus(); return this; }
  };
}
