/* TextField 搜索框（组件库规划.md 4.6）：按类目名／域名／平台过滤那一个框。
   干什么用：把"输入 → 过滤"这件事收成一个件，顺带补原型缺的两样——有内容时能一键清空、输入法没打完不触发过滤。
   会错在哪：拼音还在组合中就发过滤请求，结果会跟着半截拼音乱跳（人性化清单第 11 条，原型没有这条）。
   怎么回来：组合期间挂 composing 标记，compositionend 之后补一次；placeholder／清空按钮的说明文字都由调用方给。 */

/**
 * @param {object} o
 * @param {string} [o.value]
 * @param {string} [o.placeholder]  占位文字（调用方给）
 * @param {string} [o.clearTitle]   清空那颗按钮的悬停说明（调用方给）
 * @param {boolean} [o.disabled]   整框不能用（例如后台还没连上）
 * @param {string} [o.title]  禁用时在等什么的说明（人性化清单第 4 条：灰了要能说清为什么灰）
 * @param {(value:string)=>void} [o.onInput]
 */
export function TextField(o) {
  const el = document.createElement('div');
  el.className = 'search';
  const input = document.createElement('input');
  input.type = 'text';
  input.value = o.value || '';
  if (o.placeholder) input.placeholder = o.placeholder;
  // 禁用要带说明（人性化清单第 4 条）：这句"在等什么"由调用方给，件不持有文案
  input.disabled = !!o.disabled;
  if (o.title) input.title = o.title;
  const clear = document.createElement('button');
  clear.type = 'button';
  clear.className = 'search-clear';
  if (o.clearTitle) clear.title = o.clearTitle;
  clear.disabled = !!o.disabled;
  el.appendChild(input);
  el.appendChild(clear);

  let composing = false;

  function emit() {
    o.value = input.value;
    el.classList.toggle('has-value', input.value.length > 0);
    if (o.onInput) o.onInput(input.value);
  }

  input.addEventListener('compositionstart', () => { composing = true });
  input.addEventListener('compositionend', () => { composing = false; emit() });
  input.addEventListener('input', () => {
    el.classList.toggle('has-value', input.value.length > 0);
    if (composing) return;                          // 拼音没打完，先不发过滤
    emit();
  });
  input.addEventListener('keydown', e => {
    if (e.key === 'Escape' && input.value) { e.stopPropagation(); clear.click(); }
  });
  clear.addEventListener('click', () => {
    input.value = '';
    input.focus();
    emit();
  });
  el.classList.toggle('has-value', input.value.length > 0);

  return {
    el, input,
    update(next = {}) {
      Object.assign(o, next);
      if (next.placeholder !== undefined) input.placeholder = next.placeholder;
      if (next.clearTitle !== undefined && next.clearTitle) clear.title = next.clearTitle;
      if (next.disabled !== undefined) { input.disabled = !!next.disabled; clear.disabled = input.disabled; }
      if (next.title !== undefined) { if (next.title) input.title = next.title; else input.removeAttribute('title'); }
      if (next.value !== undefined && next.value !== input.value) {
        input.value = next.value;
        el.classList.toggle('has-value', input.value.length > 0);
      }
      return this;
    },
    focus() { input.focus(); return this }
  };
}
