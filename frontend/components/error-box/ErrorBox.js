/* ErrorBox 错误块（组件库规划.md 4.14）：这一条为什么没成，加一颗"重试"。
   干什么用：详情面板那条红/黄横条（原型 :227-229 样式、:1936 用法）收成一个件，补上它缺的重试与说明。
   会错在哪：只把内部函数名或错误码贴上去（那就是 C8 修掉的病），用户看完不知道该干什么；
             更常见的是"红条一闪，过两秒连那句错误也找不到了"，想复制错误码时已经没了。
   怎么回来：给 `retry:{label, onClick}` 就带一颗重试，走调用方接的那条幂等路（原型 resetTaskToWaiting :3169）；
             写 `retry: null` 就是明确说"这一条没有下一步"——详情那两条横条（原型 :1936）本来只有字没有钮，
             硬塞一颗"重试"属于私自改变功能。两条都不写＝漏传，当场抛错。
             错误正文由调用方传（将来走 t()），件里不写中文。
             左边那个「!」是符号不是文案（照原型 :1936 的写法，逐屏比对才有那个字形）。 */

import { Button } from '../button/Button.js';

/**
 * @param {object} o
 * @param {string} o.text          那句错误正文（调用方给）
 * @param {'error'|'warn'} [o.kind]  warn＝有备选方案的那种黄条（原型 :1935 那句 `isWarn` 判的就是这个）
 * @param {object|null} o.retry    { label, onClick }＝带一颗重试；null＝这一条只把话说完（键必须存在）
 * @param {string} [o.detail]      可选：错误码/接口名这类"给人复制"的等宽小字
 */
export function ErrorBox(o) {
  if (!('retry' in o)) {
    throw new Error('ErrorBox 必须写明有没有下一步：给 retry:{label, onClick}，或给 retry:null 表示这一条只把话说完');
  }
  if (o.retry !== null && !o.retry.label) {
    throw new Error('ErrorBox 的 retry 少了 label：只给一颗没字的钮，用户不知道按下去做什么');
  }

  const el = document.createElement('div');
  const ico = document.createElement('span');
  ico.className = 'ico';
  ico.textContent = '!';
  ico.setAttribute('aria-hidden', 'true');
  const body = document.createElement('span');
  body.className = 'eb-body';
  const detail = document.createElement('span');
  detail.className = 'eb-detail';
  el.appendChild(ico);
  el.appendChild(body);
  el.appendChild(detail);

  // 重试那颗只在"给了"的时候才存在：件里没有它，DOM 里就不会有一颗点了没反应的钮
  const retry = o.retry === null ? null
    : (() => {
      const b = Button({ label: o.retry.label, size: 'sm', kind: 'ghost', onClick: o.retry.onClick });
      b.el.classList.add('eb-retry');
      el.appendChild(b.el);
      return b;
    })();

  function paint() {
    el.className = 'error-box' + (o.kind === 'warn' ? ' warn' : '');
    body.textContent = o.text == null ? '' : String(o.text);
    detail.textContent = o.detail == null ? '' : String(o.detail);
    detail.style.display = o.detail ? '' : 'none';
  }

  paint();
  return {
    el,
    update(next = {}) {
      Object.assign(o, next);
      paint();
      if (retry && next.retry) retry.update({ label: next.retry.label, onClick: next.retry.onClick });
      return this;
    },
    setText(t) { o.text = t; paint(); return this },
    setDisabled(off) { if (retry) retry.setDisabled(!!off); return this }
  };
}
