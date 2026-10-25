/* SettingRow 设置行（组件库规划.md 第 5 节，唯一进库的分子件）：一句标题 + 一句说明 + 右边那颗控件。
   干什么用：七个设置面板 18 处（使用点，数法见第 4 节那条）都是同一个形状，
             控件可以是开关／复选／下拉／输入框，但"点标题就该聚焦控件、禁用要连说明一起灰"这三件事必须一样。
   会错在哪：① 原型把标题、说明、控件三段散在 set-card 里各写各的，有一处漏了关联，
               点标题没反应，只能精确点中那颗 36×20 的开关；
             ② 禁用时只灰控件不灰说明，看着像"这句话还能点"，读屏也以为还能改；
             ③ 标题与说明的字号各处自定（12/13 混着写），下次改字号要满文件找。
   怎么回来：三段收成一个结构。控件里有真能聚焦的元素（开关/复选/输入框的 input、原生下拉本身）就给它挂 <label for>，
             点左半边直接聚焦；挂不上（比如传进来的是个纯按钮件）就退化成整行点击转交焦点，不硬造一个假关联。
             `setDisabled(true)` 连说明一起灰并把控件置灰；字号引 --fs-md / --fs-sm 两支。
             文字全由调用方给，件里一个中文也不写。 */

/**
 * @param {object} o
 * @param {string} o.title          这一行说的是什么（调用方给）
 * @param {string} [o.desc]         说明那一句（调用方给）
 * @param {object} o.control        Switch()/Checkbox()/Select()/TextField() 造出来的对象（要有 .el）
 * @param {string} [o.for]          控件的 id 已由外面管理时，指定 label 指向谁
 * @param {string} [o.icon]         标题前那个装饰字符（原型每张卡片都有一个，见 :795 那种 `<span class="ico">`）
 * @param {boolean} [o.disabled]    整行禁用（说明一起灰）
 * @param {string} [o.disabledReason] 禁用时那句"在等什么"：挂在行上，也递到控件自己那颗 title
 */
export function SettingRow(o) {
  if (!o.control || !o.control.el) {
    throw new Error('SettingRow 需要 control：传 Switch()/Checkbox()/Select()/TextField() 造出来的那个对象（带 .el）');
  }

  const el = document.createElement('div');
  el.className = 'set-row';
  const text = document.createElement('div');
  text.className = 'sr-text';
  const label = document.createElement('label');
  label.className = 'sr-title';
  const desc = document.createElement('p');
  desc.className = 'sr-desc';

  const box = o.control.el;
  // 控件里真能聚焦的那个：原生 input/select 自己，或包裹件里的第一个可聚焦元素
  const focusable = (box.matches && box.matches('input,select,textarea'))
    ? box
    : (box.querySelector ? box.querySelector('input,select,textarea,[tabindex]') : null);

  let wired = false;
  if (o.for) { label.htmlFor = o.for; wired = true; }
  else if (focusable) {
    if (!focusable.id) focusable.id = 'sr-' + Math.random().toString(36).slice(2, 8);
    label.htmlFor = focusable.id;
    wired = true;
  }
  if (!wired) {
    // 关联不上就别留一个不做事的 <label>：退化成普通一行字 + 整行点击转交焦点
    label.className = 'sr-title sr-plain';
    text.addEventListener('click', () => { if (focusable) focusable.focus(); });
  }

  text.appendChild(label);
  text.appendChild(desc);
  el.appendChild(text);
  el.appendChild(box);

  function paint() {
    el.classList.toggle('is-disabled', !!o.disabled);
    if (o.disabledReason) el.title = o.disabledReason; else el.removeAttribute('title');
    el.setAttribute('aria-disabled', o.disabled ? 'true' : 'false');
    // 标题前那个装饰符：原型每张卡片都有一个（:795 那种 span.ico），它不参与读屏，字色走 --primary
    label.replaceChildren();
    if (o.icon) {
      const ico = document.createElement('span');
      ico.className = 'ico';
      ico.setAttribute('aria-hidden', 'true');
      ico.textContent = String(o.icon);
      label.appendChild(ico);
      label.appendChild(document.createTextNode(' '));
    }
    label.appendChild(document.createTextNode(o.title == null ? '' : String(o.title)));
    desc.textContent = o.desc == null ? '' : String(o.desc);
    desc.style.display = o.desc ? '' : 'none';
    if (focusable) {
      focusable.disabled = !!o.disabled;
      // 读屏与鼠标悬停都落在控件那一下，所以这句也得挂它一份
      if (o.disabled && o.disabledReason) focusable.title = o.disabledReason;
      else if (!o.disabled) focusable.removeAttribute('title');
    }
  }

  paint();
  return {
    el, label, desc,
    update(next = {}) { Object.assign(o, next); paint(); return this },
    setTitle(t) { o.title = t; paint(); return this },
    setDesc(t) { o.desc = t; paint(); return this },
    /** 禁用连说明一起灰，并把控件本身置灰（读屏才不会以为这句还能改） */
    setDisabled(off) { o.disabled = !!off; paint(); return this },
    focus() { if (focusable) focusable.focus(); return this }
  };
}
