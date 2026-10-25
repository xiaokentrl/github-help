/* Switch 开关（组件库规划.md 4.4）：只管"改了立刻生效的全局偏好"这一类是非选择。
   干什么用：跟 Checkbox 分清楚——开关是偏好（一拨就存档），复选是"这一条要不要纳入本次操作"。
   会错在哪：这两种语义混着用，界面上一排"勾"没人知道哪个是设置、哪个是选中范围。
   怎么回来：结构照原型（label.switch > input + span.slider），真 input 在里面，
             所以整行点得中、键盘 Tab 得到、Space 能拨；热区补到 24 px 而不动 36×20 的视觉尺寸。 */

/**
 * @param {object} o
 * @param {boolean} [o.checked]
 * @param {boolean} [o.disabled]
 * @param {string} [o.ariaLabel]  这一行说的是什么偏好（由调用方给，件不持有文案）
 * @param {string} [o.title]      禁用时那句"在等什么"（人性化清单第 4 条）
 * @param {string} [o.onText]     给读屏用的状态字（"已开启"这类，中文由调用方给）
 * @param {string} [o.offText]    同上，关闭时那句
 * @param {(checked:boolean)=>void} o.onChange
 */
export function Switch(o) {
  const el = document.createElement('label');
  el.className = 'switch';
  const input = document.createElement('input');
  input.type = 'checkbox';
  input.setAttribute('role', 'switch');
  const slider = document.createElement('span');
  slider.className = 'slider';
  el.appendChild(input);
  el.appendChild(slider);

  function paint() {
    input.checked = !!o.checked;
    input.disabled = !!o.disabled;
    if (o.title) el.title = o.title; else el.removeAttribute('title');
    const words = o.checked ? o.onText : o.offText;
    if (words) input.setAttribute('aria-label', (o.ariaLabel ? o.ariaLabel + '：' : '') + words);
    else if (o.ariaLabel) input.setAttribute('aria-label', o.ariaLabel);
  }

  input.addEventListener('change', () => {
    o.checked = input.checked;
    paint();
    if (o.onChange) o.onChange(input.checked);
  });
  paint();

  return {
    el, input,
    update(next = {}) { Object.assign(o, next); paint(); return this },
    setChecked(on) { o.checked = !!on; paint(); return this },
    setDisabled(off) { o.disabled = !!off; paint(); return this },
    focus() { input.focus(); return this }
  };
}
