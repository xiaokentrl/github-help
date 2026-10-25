/* StatusDot 状态点（组件库规划.md 4.8）：一眼看这条现在是哪种状态，但绝不只靠颜色说话。
   干什么用：把原型里六处各写各的 `class="dot ..."` 收成一个件，顺手把"点 + 文字"两条通道钉死在一起。
   会错在哪：只给一个彩色小圆点，色盲用户和黑白打印下全废；换成六套主题时最容易漏的就是其中一处。
             状态名传错（比如把 applied 写成 apply）不报错的话，点会退回默认底色，看着像"没状态"。
   怎么回来：件里给 `label`（文字通道，由调用方从 core/meta.js 的 ItemMeta/TaskMeta 取，件自己不持有文案）；
             呼吸动画受全局减弱动效开关控制——base.css 那条规则管着，这里不重复写。 */

/** 原型 :157-163 认得的六个状态名，别的当场报错（别在这里加第七种却不配颜色） */
const STATES = ['applied', 'stale', 'unset', 'error', 'pending', 'detected'];

/**
 * @param {object} o
 * @param {'applied'|'stale'|'unset'|'error'|'pending'|'detected'} o.state
 * @param {string} [o.label]  与点同色的那句状态字（调用方给，例如 ItemMeta.stale.text）
 * @param {string} [o.ariaLabel]  只在不给 label 时用：这一档状态的读屏名字（件不持有文案）
 * @param {'tree'|'inline'} [o.size]  'tree'＝侧栏那种 7px 点 + 44px 右对齐文字；'inline'＝行内 6px 点
 * @param {string} [o.cls]    仅 'inline' 用：这一行是什么任务（waiting/running/done/detecting…），
 *                            挂在 qstatus 后面，字色由 styles/layout.css 那 12 条接管（原型 :311-322）。
 */
/** 状态名先查一遍再动 DOM：这样"新增状态忘了配色"这条守卫在 node 里也测得出来 */
function stateOf(o) {
  if (!STATES.includes(o.state)) {
    throw new Error(`StatusDot 不认识状态「${o.state}」，能用的是：${STATES.join(' / ')}（新增状态要先在样式里配色）`);
  }
  return o.state;
}

export function StatusDot(o) {
  stateOf(o);
  const el = document.createElement('span');
  const dot = document.createElement('span');
  const text = document.createElement('span');
  el.appendChild(dot);
  el.appendChild(text);

  function paint() {
    const state = stateOf(o);
    const inline = o.size === 'inline';
    // 行内那一档：字是一个不带类的 span，颜色从外层 .qstatus.<cls> 继承下来。
    // 原型这里本来就是裸文本节点（:2110、:2172），套上 .stext 就会被那条 color:var(--text-3) 压成灰字，
    // 「完成」的绿色、「检测失败」的红色全看不见了——这是逐屏比对最容易漏的一处。
    el.className = inline ? 'qstatus' + (o.cls ? ' ' + o.cls : '') : 'status';
    dot.className = 'dot ' + state;
    text.className = inline ? '' : 'stext ' + state;
    text.textContent = o.label == null ? '' : String(o.label);
    text.style.display = o.label ? '' : 'none';
    // 呼吸动画跟着 pending 这一档走（样式里那条 .dot.pending 就是原型 :162 的原写法），件里不再多一个开关分支。
    // 点自己不给读屏留字：有文字通道时点纯装饰，没文字时点必须带名
    if (o.label) {
      dot.setAttribute('aria-hidden', 'true');
      el.removeAttribute('aria-label');
      el.removeAttribute('role');
    } else {
      dot.removeAttribute('aria-hidden');
      // 没字可看时，名字要么在这儿，要么这一档就只剩颜色——那是第 7 节第 5 条最反对的样子
      if (o.ariaLabel) { el.setAttribute('role', 'img'); el.setAttribute('aria-label', o.ariaLabel); }
      else { el.removeAttribute('role'); el.removeAttribute('aria-label'); }
    }
  }

  paint();
  return {
    el, dot,
    update(next = {}) { Object.assign(o, next); paint(); return this },
    setState(state) { o.state = state; paint(); return this },
    setLabel(label) { o.label = label; paint(); return this }
  };
}
