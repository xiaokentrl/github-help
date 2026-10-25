/* Select 下拉（组件库规划.md 4.5）：多选一，走原生 <select>。
   干什么用：模式选择、语言选择、代理类型那类"从几个里挑一个"。
   会错在哪：原生 select 在 Windows 与 Linux 上弹出样式不一样——这条已定"先按原生写，两平台一致性问题挂 M0 实测"，
             量出来确实对不上再换自绘（那时键盘与焦点得自己管，成本大一截）。
   怎么回来：件只暴露 value + onChange；选项文字与"未启用"那些占位项都由调用方给，件里不写中文。 */

/**
 * @param {object} o
 * @param {Array<{value:string,label:string,disabled?:boolean}>} o.options
 * @param {string} o.value
 * @param {'sel'|'mode'} [o.size]  'sel'＝设置面板那档（:394），'mode'＝工具栏那颗（:91）
 * @param {(value:string)=>void} [o.onChange]
 * @param {string} [o.ariaLabel]
 * @param {string} [o.title]  禁用时那句"在等什么"（调用方给；人性化清单第 4 条）
 * @param {boolean} [o.disabled]
 */
export function Select(o) {
  const el = document.createElement('select');
  el.className = o.size === 'mode' ? 'mode' : 'sel';
  if (o.ariaLabel) el.setAttribute('aria-label', o.ariaLabel);
  const nodes = [];
  function build() {
    el.replaceChildren();
    nodes.length = 0;
    for (const opt of o.options) {
      const item = document.createElement('option');
      item.value = opt.value;
      item.textContent = opt.label == null ? '' : String(opt.label);
      item.disabled = !!opt.disabled;
      el.appendChild(item);
      nodes.push(item);
    }
  }

  function paint() {
    const hit = nodes.find(n => n.value === o.value);
    el.value = hit ? hit.value : (nodes[0] ? nodes[0].value : '');
    // 这两条以前只有样式在等（select.css 里那两条 :disabled 早就写着），参数传了却没人落——
    // "看着灰了其实还能选"就是这么来的
    el.disabled = !!o.disabled;
    if (o.title) el.title = o.title; else el.removeAttribute('title');
  }

  el.addEventListener('change', () => {
    o.value = el.value;
    if (o.onChange) o.onChange(el.value);
  });
  build();
  paint();

  return {
    el,
    update(next = {}) {
      Object.assign(o, next);
      if (next.options) build();                    // 整批换选项时重建，免得留下半截旧清单
      paint();
      return this;
    },
    setDisabled(off) { el.disabled = !!off; return this },
    focus() { el.focus(); return this }
  };
}
