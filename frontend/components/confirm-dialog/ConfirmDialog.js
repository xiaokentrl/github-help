/* ConfirmDialog 确认框（组件库规划.md 4.13）：危险动作前问那一句。
   干什么用：把原型那一个全局 `confirmDialog()`（:2245）收成一个件，顺手补它缺的可达性。
   会错在哪：① 现在关掉之后焦点丢回页面顶部，键盘用户得从头 Tab 一遍才回到那颗按钮（第 7 节第 3 条）；
             ② 框里的字写"确认操作"，没写后果——用户点完才知道影响 40 条域名、而且撤不回来；
             ③ Tab 能跳出框外，跳到遮罩后面的树上去。
   怎么回来：打开时记下触发件，关闭后把焦点还回去；Tab 与 Shift+Tab 关在框内（只在框里可聚焦件之间转）；
             文案固定三件由调用方给齐——要做什么（title）、影响几条（lines）、能不能撤（warn，红字那一行）；
             层级 60、遮罩、Esc 关掉都跟原型一致。件里不写一个字中文。 */

import { Button } from '../button/Button.js';

const FOCUSABLE = 'button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex]:not([tabindex="-1"])';

/**
 * @param {object} o
 * @param {string} o.title        要做什么（例如那句"还原全部 Hosts"，由调用方给）
 * @param {string[]} [o.lines]    影响哪几条，一行一条
 * @param {string} [o.warn]       能不能撤——单独一行、走红字样式
 * @param {string} [o.confirmText] 确认那颗的字
 * @param {string} [o.cancelText]  取消那颗的字
 * @param {boolean} [o.danger]     危险动作：确认钮用红底（原型 :2251 那两行 style 的写法收到这里来）
 */
export function confirmDialog(o) {
  return new Promise(resolve => {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';

    const box = document.createElement('div');
    box.className = 'modal';
    box.setAttribute('role', 'dialog');
    box.setAttribute('aria-modal', 'true');
    if (o.title) box.setAttribute('aria-label', o.title);

    const head = document.createElement('div');
    head.className = 'modal-head';
    const h3 = document.createElement('h3');
    h3.textContent = o.title == null ? '' : String(o.title);
    head.appendChild(h3);

    const body = document.createElement('div');
    body.className = 'modal-body';
    for (const line of o.lines || []) {
      const p = document.createElement('p');
      p.textContent = String(line);
      body.appendChild(p);
    }
    if (o.warn) {
      const w = document.createElement('p');
      w.className = 'warn-line';
      w.textContent = String(o.warn);
      body.appendChild(w);
    }

    const foot = document.createElement('div');
    foot.className = 'modal-foot';
    const cancel = Button({ label: o.cancelText, onClick: () => close(false) });
    const confirm = Button({
      label: o.confirmText,
      kind: 'primary',
      onClick: () => close(true)
    });
    // 危险动作：确认钮换成红底（原型 :2251 那句内联 style 写的就是这个，收进样式里那一档）
    if (o.danger) confirm.el.classList.add('is-danger');
    foot.appendChild(cancel.el);
    foot.appendChild(confirm.el);

    box.appendChild(head);
    box.appendChild(body);
    box.appendChild(foot);
    mask.appendChild(box);

    // 记下来的是"谁把这个框叫出来的"，关掉后焦点得回它那儿，否则键盘用户从头 Tab 一遍
    const opener = document.activeElement;

    function finish(result) {
      document.removeEventListener('keydown', onKey, true);
      mask.remove();
      if (opener && typeof opener.focus === 'function' && document.contains(opener)) opener.focus();
      resolve(result);
    }
    function close(result) { finish(result); }

    function onKey(e) {
      if (e.key === 'Escape') {
        e.preventDefault();
        finish(false);
        return;
      }
      if (e.key !== 'Tab') return;
      const items = Array.from(box.querySelectorAll(FOCUSABLE));
      if (!items.length) return;
      const first = items[0];
      const last = items[items.length - 1];
      // Tab 关在框内：从最后一格再往后就回到第一格，往前越界就落到最后一格
      if (e.shiftKey && (document.activeElement === first || !box.contains(document.activeElement))) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }

    mask.addEventListener('click', e => { if (e.target === mask) finish(false); });   // 点遮罩＝取消，跟原型一致
    document.addEventListener('keydown', onKey, true);
    document.body.appendChild(mask);
    confirm.focus();
  });
}
