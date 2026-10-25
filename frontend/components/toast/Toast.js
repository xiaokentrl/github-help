/* Toast 气泡（组件库规划.md 4.12）：屏幕底部中间那一句"刚才发生了什么"。
   干什么用：原型这一句有 76 个调用点，全靠一个 showToast() 配固定 2200ms——长文案读不完就没了。
   会错在哪：① 2200ms 写死：一句四五十字的话根本读不完，用户只看到半句，还以为程序在乱跳；
             ② 报错那句也自动消失，回头想复制错误码时已经没了；
             ③ 鼠标移上去想看仔细，倒计时照走，气泡当场收掉（人性化清单第 4 条）。
   怎么回来：时长按字数给（短 2.2s＝原型那个值、长 4s、更长按字数加，见 toastMs）；
             报错类 kind:'error' 一律不自动收，必须人点掉，可另带一颗动作按钮（文字由调用方给）；
             悬停暂停、移开续数。文字全部由调用方传，件里一个中文也不写。 */

/**
 * @param {object} o
 * @param {(text:string)=>void} [o.onCopy] 报错类那颗动作按钮按下去做什么（例如把这句复制走）
 */
export function Toast(o = {}) {
  const el = document.createElement('div');
  el.id = 'toast';                       // 原型的样式与 76 个调用点都指到这个 id，名字不改才比得出零差异
  const msg = document.createElement('span');
  msg.className = 'toast-msg';
  const act = document.createElement('button');
  act.type = 'button';
  act.className = 'btn btn-sm toast-act';
  act.style.display = 'none';
  const close = document.createElement('button');
  close.type = 'button';
  close.className = 'toast-close';
  close.textContent = '×';               // 符号不是文案，不进"件里不许裸中文"那条门禁
  close.style.display = 'none';

  let timer = null;
  let remaining = 0;
  let startedAt = 0;
  let sticky = false;

  function clearTimer() {
    if (timer) { clearTimeout(timer); timer = null; }
  }

  function hide() {
    clearTimer();
    el.classList.remove('show');
    sticky = false;
    act.style.display = 'none';
    close.style.display = 'none';
  }

  /** 时长按字数给：24 字以内沿用原型那 2200ms，再长按读完的速度往上加，最贵 9s */
  function toastMs(text) {
    const n = (text || '').length;
    if (n <= 24) return 2200;
    if (n <= 60) return 4000;
    return Math.min(9000, 4000 + (n - 60) * 160);
  }

  function startTimer(ms) {
    clearTimer();
    remaining = ms;
    startedAt = Date.now();
    timer = setTimeout(hide, ms);
  }

  // 悬停暂停：正在读的那句不该从眼前跑掉；移开再把剩下的数走完
  el.addEventListener('mouseenter', () => {
    if (!sticky && timer) {
      remaining -= Date.now() - startedAt;
      clearTimer();
    }
  });
  el.addEventListener('mouseleave', () => {
    if (!sticky && !timer && remaining > 0) startTimer(remaining);
  });
  close.addEventListener('click', hide);
  act.addEventListener('click', () => {
    if (o.onCopy) o.onCopy(msg.textContent);
    hide();
  });

  el.appendChild(msg);
  el.appendChild(act);
  el.appendChild(close);

  /**
   * @param {object} m
   * @param {string} m.text              那一句（调用方给）
   * @param {'info'|'ok'|'warn'|'error'} [m.kind]
   * @param {number} [m.duration]        覆盖自动时长；报错类无视它（必须点掉）
   * @param {string} [m.actionText]      报错类那颗动作按钮上的字（例如"复制错误码"）
   * @param {string} [m.closeText]       点掉那颗的悬停说明
   */
  function show(m = {}) {
    const kind = m.kind || 'info';
    clearTimer();
    el.className = 'toast-' + kind;
    el.id = 'toast';
    msg.textContent = m.text == null ? '' : String(m.text);
    el.classList.add('show');
    // 报错那句要打断读屏，其余只是"顺便告诉你一声"
    el.setAttribute('role', kind === 'error' ? 'alert' : 'status');
    el.setAttribute('aria-live', kind === 'error' ? 'assertive' : 'polite');
    sticky = kind === 'error';
    if (sticky) {
      if (m.actionText) {
        act.textContent = m.actionText;
        act.style.display = '';
      }
      if (m.closeText) close.title = m.closeText;
      close.style.display = '';
    } else {
      act.style.display = 'none';
      close.style.display = 'none';
      startTimer(m.duration || toastMs(msg.textContent));
    }
    return this;
  }

  return { el, show, hide, toastMs, isShowing: () => el.classList.contains('show') };
}
