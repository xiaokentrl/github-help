/* 组件状态矩阵的实测脚本（只在浏览器里跑）。
   干什么用：把 components/ 那 17 个件、每个件的每一种状态都真造一遍，量出「人眼实际会看到什么」的那几个数——
             热区多大、焦点环画不画、禁用有没有说明、彩点旁边有没有一句字、文字压在底色上够不够对比、
             减弱动效那一档开起来动画停不停。量完把读数交给 tests/lib/pin.mjs 钉进
             tests/baseline/state-matrix.json，之后由 tests/baseline.test.mjs 长期守着。
   会错在哪：① 这些数没法在 node 里算（没有真排版、没有 CSSOM），所以必须起一次浏览器；
             ② 量完不钉档就等于没量——件改了一行样式，读数早就变了，没人知道；
                所以钉档时把「量这些数时读的是哪一份源码」的哈希一起写进去，源码一动，守档那条立刻红，逼人重测。
   怎么回来：`cd frontend && npm run dev`（9222 端口），在打开的页面上执行
             `await import('/tests/components/state-matrix.probe.js').then(m => m.run())`，
             把返回的 JSON 原文存成一个文件，再跑 `node tests/lib/pin.mjs <那个文件>`。 */

import '/styles/index.js';
/* 颜色归一、底色往上找、对比度、盒子像素这几样，与界面层那份 tests/views/screens.probe.js 用的是同一条公式：
   算式只留 tests/lib/readings.js 那一份。这里不再各自写一遍，否则两份基线迟早算出两个答案。 */
import { toRgba, bgOf, contrastOf, lenOf, boxOf } from '../lib/readings.js';

const DIRS = ['button', 'checkbox', 'confirm-dialog', 'drag-handle', 'empty-state', 'error-box',
  'icon', 'progress-bar', 'segmented', 'select', 'setting-row', 'skeleton', 'status-dot',
  'switch', 'tag', 'text-field', 'toast'];

const moduleOf = dir => import(`/components/${dir}/${camel(dir)}.js`);
function camel(dir) {
  return dir.split('-').map(s => s[0].toUpperCase() + s.slice(1)).join('');
}

/* ---------- 这一份自己的量具：焦点环、键盘可达、ARIA ----------
   通用的那几样（颜色 / 底色 / 对比度 / 盒子 / 声明值换算）在 tests/lib/readings.js。 */

/** 焦点环：从 CSSOM 里捞出真正会作用在这个元素上的那条 :focus-visible 声明。
    为什么不用 el.focus() 再看：Chrome 里代码发的 focus() 对按钮不算「键盘焦点」，
    :focus-visible 根本不匹配，量出来会是「没有焦点环」这个假结论。
    去掉伪类后能用 matches() 命中，说明这条规则确实归这个元素——画不画由声明本身说话。
    裸的那一条（全站 `:focus-visible{…}`）剥完是空串，空串的意思是"谁都归我管"，
    当成匹配不上就会把「全站都有焦点环」量成「一个都没有」——这条最容易量反。 */
function focusRing(el) {
  const out = { outline: 'none', boxShadow: 'none', matched: [] };
  for (const sheet of document.styleSheets) {
    let rules;
    try { rules = sheet.cssRules; } catch { continue; }
    for (const r of rules) {
      if (!r.selectorText || !r.selectorText.includes(':focus-visible')) continue;
      const plain = r.selectorText.split(',').map(s => {
        let t = s.trim();
        for (let i = 0; i < 4 && /:focus-visible\b/.test(t); i++) t = t.replace(/:focus-visible\b/g, '');
        t = t.replace(/::?[-a-zA-Z]*(hover|active|focus)\b/g, '').replace(/[\s,]+/g, ' ').trim();
        return t;
      }).filter(Boolean);
      const hits = plain.length === 0 || plain.some(s => { try { return el.matches(s); } catch { return false; } });
      if (!hits) continue;
      const o = r.style.outline;
      const bs = r.style.boxShadow;
      if (o && o !== 'none') { out.outline = o; out.matched.push(sheet.href || 'inline'); }
      if (bs && bs !== 'none') { out.boxShadow = bs; out.matched.push(sheet.href || 'inline'); }
    }
  }
  return out;
}

/** 能不能用键盘走到：原生能聚焦的、tabindex≥0 的才算；disabled 的一律不算 */
function tabbable(el) {
  if ('disabled' in el && el.disabled) return false;
  return el.tabIndex >= 0;
}

function ariaOf(el) {
  const keys = ['role', 'aria-checked', 'aria-busy', 'aria-disabled', 'aria-label', 'aria-hidden',
    'aria-orientation', 'aria-valuenow', 'title'];
  const out = {};
  for (const k of keys) {
    const v = el.getAttribute(k);
    if (v != null && v !== '') out[k] = v;
  }
  if (el.tabIndex >= 0) out.tabindex = String(el.tabIndex);
  if ('disabled' in el && el.disabled) out.disabled = 'true';
  return out;
}

/* ---------- 状态矩阵：一行 = 一个件的一种状态 ----------
   comp  件目录名（守档那条按它跟名单对账）
   guard 这行答的是组件库规划.md 第 7 节人性化清单的第几条
   need  这行必须满足的数（守档当场重算，不信任下面记的读数）：
         hit=[宽,高] 热区下限 / ring=该元素要能吃到全站焦点环 / reason=禁用要带说明 /
         dual=彩点旁边要有字 / text=这行有文字，要算对比 / anim=这行带动画，减弱动效下必须停 /
         slot=这一行换个真容器（把手要 flex 拉伸，块高槽位量出来是 0 高）/
         对比度不在这里写下限：低于 4.5 的每一档归 tests/baseline/contrast-debt.mjs 那份人工登记的账，
         两处各写一条迟早变成"哪条说了算"——只留一份。 */
const MATRIX = [
  // —— 4.1 按钮：四种型 + 小尺寸 + 禁用 + 加载中
  { id: 'button/default', comp: 'button', need: { ring: 1, text: 1 }, make: ({ Button }) => Button({ label: 'ABCD' }) },
  { id: 'button/primary', comp: 'button', need: { ring: 1, text: 1 }, make: ({ Button }) => Button({ label: 'ABCD', kind: 'primary' }) },
  { id: 'button/danger', comp: 'button', need: { ring: 1, text: 1 }, make: ({ Button }) => Button({ label: 'ABCD', kind: 'danger' }) },
  { id: 'button/ghost', comp: 'button', need: { ring: 1, text: 1 }, make: ({ Button }) => Button({ label: 'ABCD', kind: 'ghost' }) },
  { id: 'button/sm', comp: 'button', need: { ring: 1, text: 1, hit: [1, 24] }, make: ({ Button }) => Button({ label: 'ABCD', size: 'sm' }) },
  { id: 'button/disabled', comp: 'button', guard: [4], need: { reason: 1, text: 1 }, make: ({ Button }) => Button({ label: 'ABCD', disabled: true, title: 'waiting for queue' }) },
  { id: 'button/busy', comp: 'button', guard: [4], need: { reason: 1, text: 1 }, make: ({ Button }) => { const b = Button({ label: 'ABCD', title: 'running now' }); b.setBusy(true); return b; } },
  { id: 'button/icon', comp: 'button', need: { ring: 1, text: 1 }, make: ({ Button, Icon }) => Button({ label: 'ABCD', icon: Icon({ name: 'refresh' }).el }) },

  // —— 4.4 复选：三态 + 禁用；方框本身 14 px，热区靠那层看不见的罩子扩到 24
  { id: 'checkbox/unchecked', comp: 'checkbox', guard: [6], need: { ring: 1, hit: [24, 24] }, make: ({ Checkbox }) => Checkbox({ ariaLabel: 'x' }) },
  { id: 'checkbox/checked', comp: 'checkbox', need: { hit: [24, 24] }, make: ({ Checkbox }) => Checkbox({ state: 'checked', ariaLabel: 'x' }) },
  { id: 'checkbox/indeterminate', comp: 'checkbox', need: { hit: [24, 24] }, make: ({ Checkbox }) => Checkbox({ state: 'indeterminate', ariaLabel: 'x' }) },
  { id: 'checkbox/disabled', comp: 'checkbox', guard: [4], need: { reason: 1, hit: [24, 24] }, make: ({ Checkbox }) => Checkbox({ disabled: true, ariaLabel: 'x', title: 'waiting for queue' }) },

  // —— 4.4 开关：开 / 关 / 禁用；给读屏的状态字由调用方传
  { id: 'switch/on', comp: 'switch', need: { ring: 1, hit: [24, 24] }, make: ({ Switch }) => Switch({ checked: true, onChange() { }, ariaLabel: 'x', onText: 'on', offText: 'off' }) },
  { id: 'switch/off', comp: 'switch', need: { ring: 1, hit: [24, 24] }, make: ({ Switch }) => Switch({ checked: false, onChange() { }, ariaLabel: 'x', onText: 'on', offText: 'off' }) },
  { id: 'switch/disabled', comp: 'switch', guard: [4], need: { reason: 1, hit: [24, 24] }, make: ({ Switch }) => Switch({ checked: true, disabled: true, onChange() { }, ariaLabel: 'x', title: 'waiting for startup' }) },

  // —— 4.8 状态点：六档 + 「只有颜色没字」那一档必须自己带无障碍名（第 7 节第 5 条）
  ...['applied', 'stale', 'unset', 'error', 'pending', 'detected'].map(state => ({
    id: `status-dot/${state}`, comp: 'status-dot', guard: [5],
    need: { dual: 1, text: 1 },
    make: ({ StatusDot }) => StatusDot({ state, label: 'STATE' })
  })),
  { id: 'status-dot/noLabel', comp: 'status-dot', guard: [5], need: { dual: 1 }, make: ({ StatusDot }) => StatusDot({ state: 'applied', ariaLabel: 'APPLIED' }) },
  { id: 'status-dot/inline', comp: 'status-dot', guard: [5], need: { dual: 1, text: 1 }, make: ({ StatusDot }) => StatusDot({ state: 'detected', size: 'inline', cls: 'done', label: 'STATE' }) },

  // —— 4.7 标签：三档色 × 两档字号；legacy 那档 10 px 是「已定不动」的旧账，只量不判
  { id: 'tag/green-legacy', comp: 'tag', need: { text: 1 }, make: ({ Tag }) => Tag({ text: 'BEST' }) },
  { id: 'tag/detect-md', comp: 'tag', need: { text: 1 }, make: ({ Tag }) => Tag({ text: 'DETECT', tone: 'detect', size: 'md' }) },
  { id: 'tag/apply-md', comp: 'tag', need: { text: 1 }, make: ({ Tag }) => Tag({ text: 'APPLY', tone: 'apply', size: 'md' }) },
  { id: 'tag/restore-md', comp: 'tag', need: { text: 1 }, make: ({ Tag }) => Tag({ text: 'RESTORE', tone: 'restore', size: 'md' }) },

  // —— 4.5 下拉：两档尺寸 + 禁用（选项文字由调用方给）
  { id: 'select/sel', comp: 'select', need: { ring: 1, hit: [1, 24], text: 1 }, make: ({ Select }) => Select({ options: [{ value: 'a', label: 'AAA' }, { value: 'b', label: 'BBB' }], value: 'a', ariaLabel: 'x' }) },
  { id: 'select/mode', comp: 'select', need: { ring: 1, hit: [1, 24], text: 1 }, make: ({ Select }) => Select({ size: 'mode', options: [{ value: 'a', label: 'AAA' }], value: 'a', ariaLabel: 'x' }) },
  { id: 'select/disabled', comp: 'select', guard: [4], need: { reason: 1, text: 1 }, make: ({ Select }) => Select({ options: [{ value: 'a', label: 'AAA' }], value: 'a', disabled: true, ariaLabel: 'x', title: 'not editable here' }) },

  // —— 4.3 分段控件：两段 + 禁用；键盘走左右方向键
  { id: 'segmented/two', comp: 'segmented', need: { ring: 1, hit: [1, 24], text: 1 }, make: ({ Segmented }) => Segmented({ options: [{ value: 'a', label: 'AAA' }, { value: 'b', label: 'BBB' }], value: 'a', onChange() { }, ariaLabel: 'x' }) },
  { id: 'segmented/disabled', comp: 'segmented', guard: [4], need: { reason: 1, text: 1 }, make: ({ Segmented }) => Segmented({ options: [{ value: 'a', label: 'AAA' }], value: 'a', disabled: true, onChange() { }, ariaLabel: 'x', title: 'layout locked' }) },

  // —— 4.9 进度：三档 + 未知进度；全局那根线只许贴父元素下沿
  { id: 'progress/bar', comp: 'progress-bar', need: {}, make: ({ ProgressBar }) => { const p = ProgressBar({ kind: 'bar', value: 3, max: 8 }); p.show(true); return p; } },
  { id: 'progress/bar-text', comp: 'progress-bar', need: { text: 1 }, make: ({ ProgressBar }) => { const p = ProgressBar({ kind: 'bar', value: 3, max: 8, text: '3 / 8' }); p.show(true); return p; } },
  { id: 'progress/global', comp: 'progress-bar', need: { anim: 1 }, make: ({ ProgressBar }) => { const p = ProgressBar({ kind: 'global', indeterminate: true }); p.show(true); return p; } },
  { id: 'progress/score', comp: 'progress-bar', need: {}, make: ({ ProgressBar }) => ProgressBar({ kind: 'score', value: 72, max: 100 }) },

  // —— 4.10 骨架：三档宽高，全部由尺寸令牌凑
  { id: 'skeleton/value', comp: 'skeleton', need: { anim: 1 }, make: ({ Skeleton }) => Skeleton({ from: 'value', ariaLabel: 'loading' }) },
  { id: 'skeleton/line', comp: 'skeleton', need: { anim: 1 }, make: ({ Skeleton }) => Skeleton({ from: 'line', lines: 3, ariaLabel: 'loading' }) },
  { id: 'skeleton/block', comp: 'skeleton', need: {}, make: ({ Skeleton }) => Skeleton({ from: 'block', ariaLabel: 'loading' }) },

  // —— 4.6 输入框：空 / 有字（清空钮才出现）/ 禁用；拼音组合中不触发过滤
  { id: 'text-field/empty', comp: 'text-field', need: { ring: 1, hit: [1, 24] }, make: ({ TextField }) => TextField({ placeholder: 'search', clearTitle: 'clear', onInput() { } }) },
  { id: 'text-field/filled', comp: 'text-field', need: { ring: 1, hit: [1, 24], text: 1 }, make: ({ TextField }) => TextField({ value: 'github.com', placeholder: 'search', clearTitle: 'clear', onInput() { } }) },
  { id: 'text-field/disabled', comp: 'text-field', guard: [4], need: { reason: 1 }, make: ({ TextField }) => TextField({ placeholder: 'search', onInput() { }, disabled: true, title: 'waiting for the list' }) },

  // —— 4.14 空态：带下一步 / 没有下一步（action:null 合法，缺这个键要抛错）
  { id: 'empty/panel-action', comp: 'empty-state', guard: [8], need: { text: 1 }, make: ({ EmptyState }) => EmptyState({ title: 'TITLE', hint: 'HINT', action: { label: 'NEXT', onClick() { } }, variant: 'panel' }) },
  { id: 'empty/queue-null', comp: 'empty-state', guard: [8], need: { text: 1 }, make: ({ EmptyState }) => EmptyState({ title: 'TITLE', action: null, variant: 'queue' }) },
  { id: 'empty/inline', comp: 'empty-state', need: { text: 1 }, make: ({ EmptyState }) => EmptyState({ title: 'TITLE', action: null, variant: 'inline' }) },

  // —— 4.14 错误条：必须带重试（retry:null 表示这条只把话说完）
  { id: 'error/retry', comp: 'error-box', guard: [9], need: { ring: 1, text: 1 }, make: ({ ErrorBox }) => ErrorBox({ text: 'ERR', detail: 'GAC-IPC-011', retry: { label: 'RETRY', onClick() { } } }) },
  { id: 'error/warn-noRetry', comp: 'error-box', guard: [9], need: { text: 1 }, make: ({ ErrorBox }) => ErrorBox({ text: 'WARN', kind: 'warn', retry: null }) },

  // —— 第 5 节 设置行：整行灰 = 行灰 + 说明灰 + 控件 disabled
  { id: 'setting-row/switch', comp: 'setting-row', need: { text: 1 }, make: ({ SettingRow, Switch }) => SettingRow({ title: 'TITLE', desc: 'DESC', control: Switch({ checked: true, onChange() { }, ariaLabel: 'x' }) }) },
  { id: 'setting-row/disabled', comp: 'setting-row', guard: [4], need: { reason: 1, text: 1 }, make: ({ SettingRow, Switch }) => SettingRow({ title: 'TITLE', desc: 'DESC', disabled: true, disabledReason: 'waiting for the switch above', control: Switch({ checked: true, onChange() { }, ariaLabel: 'x' }) }) },

  // —— 4.15 把手：静止 / 拖动中。热区下限这里不是 24 而是 12：
  //    把手自己的样式文件头写明了原因（往 24 扩会盖住行末"还原"那颗与树行右边缘，点哪都变成拖宽度）
  { id: 'drag-handle/idle', comp: 'drag-handle', guard: [6], need: { ring: 1, hit: [12, 1] }, slot: 'display:flex;align-items:stretch;height:44px;width:100%;', make: ({ DragHandle }) => DragHandle({ axis: 'x', min: 180, max: 520, valueOf: () => 260, onCommit() { }, ariaLabel: 'x' }) },
  { id: 'drag-handle/dragging', comp: 'drag-handle', need: { hit: [12, 1] }, slot: 'display:flex;align-items:stretch;height:44px;width:100%;', make: ({ DragHandle }) => { const d = DragHandle({ axis: 'x', min: 180, max: 520, valueOf: () => 260, onCommit() { }, ariaLabel: 'x' }); d.el.classList.add('dragging'); return d; } },

  // —— 4.13 确认框：普通 / 危险；关掉后焦点要回调用它的地方
  { id: 'confirm/normal', comp: 'confirm-dialog', guard: [2, 3], need: { text: 1 }, modal: true, make: ({ confirmDialog }) => ({ open: confirmDialog, close: 'cancel' }) },
  { id: 'confirm/danger', comp: 'confirm-dialog', guard: [14], need: { text: 1 }, modal: true, make: ({ confirmDialog }) => ({ open: confirmDialog, danger: true, close: 'esc' }) },

  // —— 4.2 图标：三档 + 自己承担意思时带名
  { id: 'icon/sm', comp: 'icon', guard: [6], need: {}, make: ({ Icon }) => Icon({ name: 'refresh', size: 'sm' }) },
  { id: 'icon/md', comp: 'icon', need: {}, make: ({ Icon }) => Icon({ name: 'refresh', size: 'md' }) },
  { id: 'icon/lg', comp: 'icon', need: {}, make: ({ Icon }) => Icon({ name: 'list', size: 'lg' }) },
  { id: 'icon/labelled', comp: 'icon', need: {}, make: ({ Icon }) => Icon({ name: 'close', label: 'CLOSE' }) },

  // —— 4.12 气泡：四种色 + 时长按字数给（第 7 节第 10 条）；报错类不许自动消失
  { id: 'toast/info', comp: 'toast', guard: [10], need: { text: 1 }, show: { text: 'SHORT' }, make: ({ Toast }) => Toast({}) },
  { id: 'toast/ok', comp: 'toast', need: { text: 1 }, show: { text: 'SHORT', kind: 'ok' }, make: ({ Toast }) => Toast({}) },
  { id: 'toast/warn', comp: 'toast', need: { text: 1 }, show: { text: 'SHORT', kind: 'warn' }, make: ({ Toast }) => Toast({}) },
  { id: 'toast/error', comp: 'toast', guard: [10], need: { text: 1 }, show: { text: 'SHORT', kind: 'error', actionText: 'COPY', closeText: 'CLOSE', onCopy() { } }, make: ({ Toast }) => Toast({}) }
];

/* 件的导出名汇到一处：状态矩阵里有一行要同时用两颗件（按钮带图标、设置行带开关），
   所以 make() 拿到的是「查得到所有导出名的一张表」，不是某一个件的模块。查不到就是 undefined，
   立刻在 make() 里炸出来，比"悄悄用错名字"好查。 */
const MODS = {};
const POOL = new Proxy({}, {
  get(_, key) {
    for (const dir of DIRS) if (MODS[dir] && MODS[dir][key]) return MODS[dir][key];
    throw new Error(`状态矩阵要用的「${String(key)}」在 components/ 那 17 个件的导出名里找不到：件改过名，矩阵要跟着改`);
  }
});

async function ensureModules() {
  for (const dir of DIRS) {
    if (MODS[dir]) continue;
    const mod = await moduleOf(dir);
    MODS[dir] = mod;
    const missing = MATRIX.filter(r => r.comp === dir && !mod[camel(dir)] && !(dir === 'confirm-dialog' && mod.confirmDialog))
      .map(r => r.id);
    if (missing.length) throw new Error(`${dir} 里找不到 ${camel(dir)} 这个导出，量不了：${missing.join(', ')}`);
  }
}

function makeHost() {
  const old = document.getElementById('state-matrix-host');
  if (old) old.remove();
  const host = document.createElement('div');
  host.id = 'state-matrix-host';
  host.setAttribute('aria-hidden', 'true');
  // 挪到屏幕外但照常排版：visibility:hidden 会让有些浏览器的动画与排版省电跳过，那样量出来是假的
  host.style.cssText = 'position:fixed;left:-20000px;top:0;width:420px;z-index:1;';
  document.body.appendChild(host);
  return host;
}

/** 焦点与禁用真正长在谁身上：优先件自己交出来的 input，其次能 tab 到的第一个后代，最后才是根。
    认错人会读出"没有焦点环/不能 Tab"这种假结论（分段控件、错误块这两行就是这么翻车的）。 */
function focusTargetOf(api) {
  if (api.input) return api.input;
  if (tabbable(api.el)) return api.el;
  const kids = api.el.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
  for (const k of kids) if (tabbable(k)) return k;
  return api.el;
}

/** 一行量完返回读数；need 里写了什么就必须量出什么 */
function readRow(def, api, focusTarget) {
  const el = api.el;
  const cs = getComputedStyle(el);
  const need = def.need || {};
  const textEl = need.text ? findTextCarrier(el) : null;
  const textCs = textEl ? getComputedStyle(textEl) : cs;
  const fg = toRgba(textCs.color);
  const bg = bgOf(textEl || el);
  const out = {
    id: def.id, comp: def.comp,
    ...(def.guard ? { guard: def.guard } : {}),
    ...(Object.keys(need).length ? { need } : {}),
    box: boxOf(el),
    fg: fg ? textCs.color : null,
    bg: `rgb(${bg.slice(0, 3).join(' ')})`,
    ...(need.text && fg ? { contrast: contrastOf(fg, bg), fontSize: textCs.fontSize, fontWeight: textCs.fontWeight } : {}),
    cursor: cs.cursor,
    opacity: cs.opacity,
    aria: ariaOf(el),
    focusTarget: focusTarget === el ? 'root' : (focusTarget === api.input ? 'input' : 'child'),
    ...(needsRing(def) ? {
      ring: (() => {
        const r = focusRing(focusTarget);
        const cs2 = getComputedStyle(focusTarget);
        // 声明里是 var(--focus-ring)，量的人要看见换成真色之后的样子，所以把令牌值一起记下来
        return { outline: r.outline, boxShadow: r.boxShadow === 'none' ? 'none' : 'set', token: cs2.getPropertyValue('--focus-ring').trim() || null };
      })(),
      tabbable: tabbable(focusTarget)
    } : {})
  };
  if (need.hit) {
    out.hit = hitArea(el);
  }
  if (need.anim) {
    const animated = [...[el, ...el.querySelectorAll('*')]].filter(n => {
      const s = getComputedStyle(n);
      return s.animationName && s.animationName !== 'none';
    });
    out.animation = { count: animated.length, names: animated.slice(0, 3).map(n => getComputedStyle(n).animationName), duration: animated[0] ? getComputedStyle(animated[0]).animationDuration : null };
  }
  if (need.dual) {
    const dot = el.querySelector('.dot') || el;
    out.dot = { size: boxOf(dot).w || boxOf(el).w, hidden: dot.getAttribute('aria-hidden'), name: el.getAttribute('aria-label') || dot.getAttribute('aria-label') };
    out.textVisible = (() => {
      const t = el.querySelector('.stext') || el.lastElementChild;
      if (!t) return false;
      const s = getComputedStyle(t);
      return s.display !== 'none' && t.textContent.trim().length > 0;
    })();
  }
  if (need.reason) {
    // 一组分段按钮全都禁用时，focusTarget 会退回外层壳（那颗粒子已经 tab 不到了），
    // 只看根节点就把"整组灰了"读成"没禁用"。所以这一条要看真控件：件里的每一个可操作节点都得禁用才算数。
    // 光标那一档不量：手落在哪并不总在根上（复选的透明罩子、开关那颗滑块、设置行右边那颗控件），
    // 而宿主在屏幕外、elementFromPoint 打不到——这一档由样式层那几条 :disabled / .is-disabled 守着
    const CONTROLS = 'button, input, select, textarea, [tabindex]:not([tabindex="-1"])';
    const controls = [el, ...el.querySelectorAll(CONTROLS)]
      .filter(n => /^(BUTTON|INPUT|SELECT|TEXTAREA)$/.test(n.tagName) || n.tabIndex >= 0);
    const t = [el, focusTarget, ...controls].filter(Boolean).map(n => n.getAttribute('title')).find(v => v) || null;
    // aria-label 是"这是什么"，title 才是"为什么点不动"——拿前者凑后者会把没说明的当成有说明
    out.disabledReason = t;
    const off = n => n.disabled === true || n.getAttribute('aria-disabled') === 'true';
    out.disabledControls = controls.length;
    out.isDisabled = controls.length > 0 && controls.every(off);
  }
  return out;
}

/** 文字挂在谁身上：件根节点常常只是个壳，取第一个真有字的后代 */
function findTextCarrier(el) {
  const kids = [el, ...el.querySelectorAll('*')];
  return kids.find(n => n.textContent.trim().length > 0 && getComputedStyle(n).display !== 'none') || el;
}

/** 热区：元素自己的方子，加上那层专门用来扩热区的透明罩子（Checkbox 的罩子就是干这个的）。
    罩子多半画在伪元素上（`.cb-wrap::after{width:var(--hit-min)}`），而 getBoundingClientRect 摸不到伪元素，
    所以伪元素那一层改从声明值算：绝对定位 + 居中 + 自己的宽高，算成以元素中心为心的一个方子。
    漏了这一步，就会把"热区已经扩到 24"量成"热区还是 14"——那是量具的错，会逼人改掉本来对的东西。 */
function hitArea(el) {
  const base = el.getBoundingClientRect();
  let x0 = base.left, y0 = base.top, x1 = base.right, y1 = base.bottom;
  const union = (l, t, r, b) => {
    if (r > l && b > t) { x0 = Math.min(x0, l); y0 = Math.min(y0, t); x1 = Math.max(x1, r); y1 = Math.max(y1, b); }
  };
  for (const n of [el, ...el.querySelectorAll('*')]) {
    const s = getComputedStyle(n);
    const nr = n.getBoundingClientRect();
    // 这一档没出现时（空框下的清空按钮 display:none）rect 全是 0，并进去会把热区算到视口原点，
    // 量出 20010×1766 这种数——那是量具自己造出来的，不是件的
    if (s.display === 'none' || s.visibility === 'hidden') continue;
    if (nr.width <= 0 && nr.height <= 0) continue;
    if (s.position === 'absolute' || s.position === 'fixed') union(nr.left, nr.top, nr.right, nr.bottom);
    for (const pe of ['::before', '::after']) {
      const ps = getComputedStyle(n, pe);
      if (!ps.position || (ps.position !== 'absolute' && ps.position !== 'fixed')) continue;
      const w = lenOf(ps.width, nr.width);
      const h = lenOf(ps.height, nr.height);
      if (!(w > 0 && h > 0)) continue;
      // 罩子只有两种写法在用：以宿主正中为中心（Checkbox），或贴顶满高（把手）。
      // 偏移取 left/top 的声明值，再按 transform 里的 translate(-50%) 挪回去——这两步漏一步就量小一档
      let x = nr.left + lenOf(ps.left, nr.width);
      let y = nr.top + lenOf(ps.top, nr.height);
      const tf = ps.transform || '';
      if (/translate\(-50%, -50%\)/.test(tf) || /translate\(self\)/.test(tf)) { x -= w / 2; y -= h / 2; }
      else if (/translateX\(-50%\)/.test(tf)) { x -= w / 2; }
      else if (/translateY\(-50%\)/.test(tf)) { y -= h / 2; }
      union(x, y, x + w, y + h);
    }
  }
  return { w: Math.round(x1 - x0), h: Math.round(y1 - y0) };
}

/** 弹窗这一类不归宿主容器管：它自己 append 到 body，量完必须当场关掉，否则下一行读到上一个框 */
async function measureModal(def) {
  const opener = document.createElement('button');
  document.body.appendChild(opener);
  opener.focus();
  const before = document.querySelectorAll('.modal-mask').length;
  const spec = def.make(POOL);
  const p = MODS['confirm-dialog'].confirmDialog({
    title: 'TITLE', lines: ['L1', 'L2'], warn: spec.danger ? 'CANNOT UNDO' : undefined,
    confirmText: 'YES', cancelText: 'NO', danger: !!spec.danger
  });
  await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
  const mask = document.querySelectorAll('.modal-mask')[before] || document.querySelector('.modal-mask.show');
  const box2 = mask && mask.querySelector('.modal');
  const focusables = box2 ? Array.from(box2.querySelectorAll('button, [href], input, select, [tabindex]:not([tabindex="-1"])')) : [];
  const active = document.activeElement;
  const first = focusables[0];
  const last = focusables[focusables.length - 1];
  const readings = {
    id: def.id, comp: def.comp, modal: true, ...(def.guard ? { guard: def.guard } : {}),
    need: def.need,
    box: box2 ? boxOf(box2) : null,
    aria: box2 ? ariaOf(box2) : {},
    focusables: focusables.length,
    autoFocusInside: !!(box2 && box2.contains(active)),
    tabCloses: !!(first && last),
    fg: first ? getComputedStyle(first).color : null,
    bg: box2 ? `rgb(${bgOf(box2).slice(0, 3).join(' ')})` : null,
    contrast: box2 && first && toRgba(getComputedStyle(first).color)
      ? contrastOf(toRgba(getComputedStyle(first).color), bgOf(box2)) : null
  };
  if (spec.close === 'esc') document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  else if (mask) mask.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  await p;
  const backHome = document.activeElement === opener;
  readings.focusReturnsToOpener = backHome;
  readings.maskRemoved = document.querySelectorAll('.modal-mask').length === before;
  opener.remove();
  return readings;
}

export async function run() {
  await ensureModules();
  // 上一趟量到一半抛错会把 reduce-motion 留在 body 上，那一档把所有动画按住——
  // 于是这一趟"动画在不在"集体读成 0，减动效那条就成了"看着像通过"。开头先清干净。
  document.body.classList.remove('reduce-motion');
  const host = makeHost();
  const slots = [];
  for (const def of MATRIX) {
    if (def.modal) continue;
    const slot = document.createElement('div');
    slot.className = 'slot';
    // 槽位给成真容器该有的样子：有块宽、有最小高度。不给的话，"宽度交给容器"的那几档会量成 0×0，
    // 那是量具的错，不是件的错——读数就白记了
    slot.style.cssText = def.slot || 'display:block;height:44px;width:100%;';
    host.appendChild(slot);
    const api = def.make(POOL);
    slot.appendChild(api.el);
    if (def.show && api.show) api.show(def.show);
    slots.push({ def, api, slot });
  }
  await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));

  const themes = Array.from(document.querySelectorAll('meta[name="gac-themes"]')).length
    ? document.querySelector('meta[name="gac-themes"]').content.split(',')
    : await scanThemes();
  const defaultTheme = document.documentElement.dataset.theme || themes[0] || 'daylight';

  const rows = [];
  for (const { def, api } of slots) {
    // 禁用与焦点都长在"真能禁、真能聚焦"的那个节点上：件根常常是层壳（label 包 input）
    rows.push(readRow(def, api, focusTargetOf(api)));
  }

  // 弹窗两行（自带收尾）
  for (const def of MATRIX.filter(d => d.modal)) {
    rows.push(await measureModal(def));
  }

  // 减弱动效这一档：全站钩子有两条（prefers-reduced-motion 与 body.reduce-motion），量手动那条
  const animRows = rows.filter(r => r.animation && r.animation.count > 0).map(r => r.id);
  document.body.classList.add('reduce-motion');
  await new Promise(r => requestAnimationFrame(r));
  const stopped = [];
  for (const { def, api } of slots) {
    if (!def.need || !def.need.anim) continue;
    const still = [...[api.el, ...api.el.querySelectorAll('*')]].filter(n => {
      const s = getComputedStyle(n);
      return s.animationName && s.animationName !== 'none';
    });
    const row = rows.find(r => r.id === def.id);
    if (row) row.reduced = still.length;
    stopped.push({ id: def.id, stillRunning: still.length });
  }
  document.body.classList.remove('reduce-motion');

  // 六套主题各扫一遍文字对比：同一行、同一种字色，换主题后够不够
  const sweep = [];
  for (const theme of themes) {
    const prev = document.documentElement.dataset.theme;
    document.documentElement.dataset.theme = theme;
    // 件普遍带 .12s~.3s 的过渡：不等色走完就读，读到的是过渡中途的混合色，
    // 对比度会集体偏低（这一轮第一版就把 oled 的按钮量成 2.71，实际是 17 以上）
    const kill = document.createElement('style');
    kill.textContent = '#state-matrix-host *,#state-matrix-host *::before,#state-matrix-host *::after{transition:none !important}';
    document.head.appendChild(kill);
    await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(() => { kill.remove(); r(); })));
    for (const { def, api } of slots) {
      if (!def.need || !def.need.text) continue;
      const el = api.el;
      const carrier = findTextCarrier(el);
      const fg = toRgba(getComputedStyle(carrier).color);
      if (!fg) continue;
      // 字色与底色一起记：守档那条在 node 里按同一套 WCAG 公式重算一遍对比度，
      // 只记 contrast 一个数的话，手改基线里那个数就没人查得出来
      const carrierBg = bgOf(carrier);
      sweep.push({ theme, id: def.id, contrast: contrastOf(fg, carrierBg),
        fg: getComputedStyle(carrier).color, bg: `rgb(${carrierBg.slice(0, 3).join(' ')})` });
    }
    document.documentElement.dataset.theme = prev;
  }
  document.documentElement.dataset.theme = defaultTheme;
  host.remove();

  const durations = durationReadings();

  return {
    meta: {
      kind: 'state-matrix',
      builtAt: new Date().toISOString().slice(0, 19) + 'Z',
      theme: defaultTheme,
      themes,
      view: `${window.innerWidth}x${window.innerHeight}`,
      devicePixelRatio: window.devicePixelRatio,
      prefersReducedMotion: matchMedia('(prefers-reduced-motion: reduce)').matches,
      userAgent: navigator.userAgent,
      components: DIRS
    },
    rows,
    reduceMotion: { asked: animRows, stopped },
    themeSweep: sweep,
    durations
  };
}

/** 主题 id 的家在 styles/themes/，那边只有 Vite 认得目录扫描，所以让装载器把名单挂到 window 上；
    挂不到就退回「按 html 上那一个 + 已知三档」，量出来的档数会少，但不会假装量过了。 */
async function scanThemes() {
  try {
    const m = await import('/styles/themes/index.js');
    const list = m.themes || m.default || [];
    const ids = Array.isArray(list) ? list.map(t => t.id || t).filter(Boolean) : [];
    if (ids.length) return ids;
  } catch { /* 下面兜底 */ }
  return [document.documentElement.dataset.theme || 'daylight'];
}

function needsRing(def) {
  return !!(def.need && (def.need.ring || def.need.tabbable));
}

/** 气泡时长：件里那个 toastMs(字串) 是纯算式，直接调，不靠 DOM。
    字数点取 4／24／25／60／61／120 六个——24 与 60 是算式里那两道边界，跨过去了读数就该变 */
function durationReadings() {
  const { Toast } = MODS.toast;
  const t = Toast({});
  return [4, 24, 25, 60, 61, 120].map(n => ({ chars: n, ms: t.toastMs('x'.repeat(n)) }));
}
