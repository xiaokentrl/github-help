/* ProgressBar 进度条（组件库规划.md 4.9）：全局那条 3px 线、块里的 6px 条、候选表行内的评分条，三档形状一个件。
   干什么用：把"跑到哪儿了"收成一个件，并补上原型缺的两样——不知道还剩几项时也得有条在动、条子下面还得有一行字。
   会错在哪：① 只有线没有字，用户分不清是卡住了还是在跑（人性化清单第 5 条）；
             ② 全局那条线一旦写死像素定位，工具栏一变高它就压到按钮上——C3 当年真修过的 bug，
                原型 :76 那句注释就是当时的教训，别再犯；
             ③ 三档的容器各不相同：全局线绝对贴在工具栏下沿、评分条是行内塞在表格里，只有中间那档容得下一行字。
                想给另两档塞字就得包一层 div，一包，逐屏比对立刻跟原型对不上。
   怎么回来：位置永远靠 `top:100%` 贴父元素下沿（样式那条），件里不写任何像素；
             确定态给 `value/max`，`kind:'bar'` 配一句 `text`（「3 / 8 · 正在写入」这种，文字由调用方给）；
             不确定态传 `indeterminate:true`，条子来回扫，动画受全局减弱动效开关控制；
             传了 `kind` 不认识、或往容不下字的档位塞字，当场抛错，不静默退回默认形状。 */

/** 三档形状 = 原型那三套容器名，一个字没改（改了逐屏比对就对不上） */
const KINDS = {
  global: { box: 'global-progress', line: null, text: null, inline: false },  // 原型 :77-83
  bar: { box: 'dl-progress', line: 'dl-bar', text: 'dl-text', inline: false }, // 原型 :410-414
  score: { box: 'score-bar', line: null, text: null, inline: true }            // 原型 :251-252
};

/**
 * @param {object} o
 * @param {'global'|'bar'|'score'} [o.kind]  缺省 'bar'
 * @param {number} [o.value] 当前
 * @param {number} [o.max]   总量；评分条那种按 0～100 给
 * @param {boolean} [o.indeterminate] 不知道还剩多少时用这个
 * @param {string} [o.text]  仅 kind:'bar' 有效——条子下面那行小字（调用方给）
 * @param {string} [o.ariaLabel]
 */
export function ProgressBar(o = {}) {
  const kind = o.kind || 'bar';
  const shape = KINDS[kind];
  if (!shape) throw new Error(`ProgressBar 不认识 kind「${kind}」，能用的是：${Object.keys(KINDS).join(' / ')}`);
  if (o.text && !shape.text) {
    throw new Error(`ProgressBar：kind:'${kind}' 这一档容不下一行字（全局线是绝对定位、评分条是行内），文字只在 kind:'bar' 时给`);
  }

  const el = document.createElement(shape.inline ? 'span' : 'div');
  const fill = document.createElement('i');            // 三档都是「容器里那根 <i>」在变宽，跟原型一致
  let line = el;
  if (shape.line) {
    line = document.createElement('div');
    line.appendChild(fill);
    el.appendChild(line);
  } else {
    el.appendChild(fill);
  }
  const text = shape.text ? document.createElement('div') : null;
  if (text) el.appendChild(text);

  let shown = false;

  function paint() {
    const indet = !!o.indeterminate;
    // 全局线与块里那条都靠 .show 显隐（原型 :82 与 :411），行内评分条一直在，不需要这一档
    const shows = kind === 'global' || kind === 'bar';
    el.className = shape.box + (shown && shows ? ' show' : '') + (indet ? ' is-indet' : '');
    if (shape.line) line.className = shape.line;
    if (text) text.className = shape.text;

    const max = o.max === undefined ? 100 : o.max;
    let pct = 0;
    if (!indet) {
      const v = o.value === undefined ? 0 : o.value;
      pct = max > 0 ? Math.max(0, Math.min(100, (v / max) * 100)) : 0;
    }
    fill.style.width = pct + '%';

    el.setAttribute('role', 'progressbar');
    if (indet) el.removeAttribute('aria-valuenow');
    else {
      el.setAttribute('aria-valuenow', String(Math.round(pct)));
      el.setAttribute('aria-valuemin', '0');
      el.setAttribute('aria-valuemax', '100');
    }
    if (o.ariaLabel) el.setAttribute('aria-label', o.ariaLabel);
    if (text) text.textContent = o.text == null ? '' : String(o.text);
  }

  paint();
  return {
    el, fill,
    update(next = {}) { Object.assign(o, next); paint(); return this },
    set(value) { o.value = value; paint(); return this },
    setIndeterminate(on) { o.indeterminate = !!on; paint(); return this },
    /** 淡入淡出交给 CSS 那条 opacity 过渡，件里不写定时器——什么时候收由调用方定 */
    show(on) { shown = !!on; paint(); return this }
  };
}
