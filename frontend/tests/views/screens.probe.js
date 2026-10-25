/* 界面逐屏读数的实测脚本（只在浏览器里跑，跟件层那份 state-matrix.probe.js 是两码事）。
   干什么用：件层那份量的是"一个件单独摆出来长什么样"，这一份量的是"整界面画起来长什么样"——
             十二块外框的盒子、十一处有字的锚点、六套主题一套不跳，再把仓库根那一份原型 HTML 当对照物并排量一遍；
             外加七格设置面板、两道把手与十一格交互读数——九格是真点出来的、一格走键盘、一格只数面板。
             读数交给 tests/lib/pin.mjs --kind screens 钉进 tests/baseline/screens.json，
             之后由 tests/screens.test.mjs 长期守着。
   会错在哪：① 这些数离不开真排版，node 里算不出来，所以必须起一次浏览器；
             ② 量完不钉档等于没量——样式改一行，画面上的数就变了而没人知道；钉档时把"量的是哪一版源码"
                的指纹一起写进去（那份指纹盖 views/ styles/ components/ 外框 html 与这两份量具），
                任一处动了守档立刻红，逼人重量；
             ③ 与原型比对用的是**同一次运行、同一个视口、同一套主题**，所以差的不是窗口大小而是画法本身；
                两边都先把类目树摊到"全部展开"再量——不然一边量到 0 高（折叠是 display:none）一边量到实高，
                报出来的差全是状态差，不是画法差；
             ④ 原型那份用 srcdoc iframe 端起来（同源，量得到里面的节点），仓库里不许多复制一份原型；
             ⑤ 这里没有真服务（浏览器直接开页面，宿主不注入），状态栏那格如实写"后台服务未连上"。
                这一份量的是"排版与颜色"，不是"数据"，所以不编造任何一条域名读数；
                凡是因为**状态**而不是画法不同的那一格（权限徽标、状态栏第一格），比对时按名字跳过并写明跳过理由。
             ⑥ 找那颗钮靠的是按钮上的字（全选／折叠／检测全部／恢复）。改文案要先看这份量具会不会找不到——
                找不到它当场抛错，不会静默量出一屏假的读数；
             ⑦ 这台机器上 vite 的改动监听会漏事件〔实测：改完量具不重启，9222 端出来的还是上一版那份，
                读数就量在旧界面上，而且看不出来〕。所以改完这份文件**先重启 9222**（或 import 时带 `?t=` 破缓存），
                再看一眼 `curl -s http://127.0.0.1:9222/tests/views/screens.probe.js | grep -c 一个本轮新写的词` 是不是 1。
             ⑧ 换完主题不能立刻量：全站 color／background 都挂着 120~200ms 的过渡，等 40ms 就量，量到的是
                "过渡走到一半"的中间色（实测把原型侧工具栏钮读成 rgb(131,131,135) 这种谁都不像的灰），
                两边一比就成了假漂移。现在两边每套切完都等 320ms。
                同一类坑还有全局进度线那条线的透明度：它本身带过渡，没任务时停在 0，量到哪一帧说不准，
                所以那一格只比位置与高度，透明度算状态差（见 STATE_ONLY）。
             ⑨ 树里挑哪一行以我们这边为准，原型那边照着同一句字去找同一行；两边各挑"第一个"会挑到不同的行
                （我们这边第一行默认是被选中的，底色是选中色），报出来的差是行差不是画法差。
   怎么回来：`cd frontend && npm run dev`（9222）→ 另开 `node tests/lib/proto-server.mjs`（9226，只读端原型）
             → 另开 `node tests/lib/sink.mjs /tmp/gac-screens-raw.json`（9223，收一次就收工）→
             浏览器页面里 `await import('/tests/views/screens.probe.js').then(m => m.run())`
             → `node tests/lib/pin.mjs --kind screens /tmp/gac-screens-raw.json`。
             量完把 9226 那个进程 kill 掉：它端的是一整份原型，留着等于给浏览器留一扇开着的门。
   注：这一份的读数来自 Chrome/Electron 内核，不是那台机器上的 webkit2GTK 壳——真宿主 GUI 仍未逐屏走查。 */

const KIND = 'screens';
const PROTO_FILE = '前端原型设计稿唯一真实来源.html';
const PROTO_URL = 'http://127.0.0.1:9226/' + encodeURIComponent(PROTO_FILE);
const SINK = 'http://127.0.0.1:9223';
export const THEMES = ['light', 'sakura', 'forest', 'ocean', 'midnight', 'oled'];

/* ---------- 量具（与件层那一份同一条公式，两份都从 tests/lib/readings.js 现取） ---------- */

import { toRgba, bgOf, contrastOf, boxOf, fontOf } from '../lib/readings.js';

const r2 = n => +n.toFixed(2);
const frame = () => new Promise(res => requestAnimationFrame(() => requestAnimationFrame(res)));
const settle = (ms = 120) => new Promise(res => setTimeout(res, ms));
const cs = (el, prop) => (el ? getComputedStyle(el)[prop] : null);
const textOf = el => (el ? (el.textContent || '').trim().replace(/\s+/g, ' ') : null);

/* ---------- 锚点名单：每块界面在两份画面上都得叫这个名字 ----------
   选择器只能用**两份画面都有**的那一种：外框那边 #toolbar 是我们留的壳，原型那一条 div 没有 id，
   所以这里写 .toolbar——写 #toolbar 的话原型侧一律量成"缺"，看着像原型少画了一块。 */

export const BOX_ANCHORS = [
  ['顶栏', '.topbar'], ['横幅', '.banner'], ['工具栏', '.toolbar'], ['全局进度线', '.global-progress'],
  ['侧栏', '#sidebar'], ['横向把手', '.sidebar-resizer'], ['右栏', '.right'], ['详情区', '#detail'],
  ['纵向把手', '.resizer'], ['队列面板', '#queuePanel'], ['状态栏', '.statusbar'], ['托盘胶囊', '.tray-pill']
];

export const TEXT_ANCHORS = [
  ['顶栏标题', '.topbar h1'], ['权限徽标', '.topbar .perm'],
  ['横幅那句话', '.banner span:nth-of-type(2)'],
  ['工具栏按钮', '.toolbar .btn'], ['类目行', '#tree .node.cat .name', ['has-detecting', 'has-applying', 'active']],
  ['域名行', '#tree .node.item .name', ['active', 'applying', 'detecting']],
  ['类目状态字', '#tree .node.item .stext', ['active', 'applying', 'detecting']],
  ['队列标题', '#queuePanel .queue-head h3'],
  ['状态栏那格', '.statusbar .item b'], ['侧栏脚那颗钮', '.sidebar-footer .sf-btn'],
  ['胶囊上那行字', '.tray-pill .pill-label']
];

/** 树里那三格跳开的是"这一行正处在某个状态"（选中／应用中／检测中／类目有高亮），
    颜色跟着状态上色；跳开之后两边量的都是"一行没被点、没在跑"的基准画法。
    取哪一行以我们这边为准：先在我们这边挑中一行、记下它的字，原型里再按同一句字去找同一行——
    两边各挑"第一个"会挑到不同的行（我们这边第一行正被选中），报出来的差就是行差而不是画法差。 */

/** 这块底色上压的字，两份画面各量哪个节点 */
function pick(doc, sel, skip, wantText) {
  const all = [...doc.querySelectorAll(sel)];
  if (!all.length) return null;
  if (wantText != null) {
    const same = all.find(el => textOf(el) === wantText);
    if (same) return same;
  }
  if (skip && skip.length) {
    const clean = all.find(el => !skip.some(c => el.classList.contains(c) || (el.closest('.node') && el.closest('.node').classList.contains(c))));
    if (clean) return clean;
  }
  return all[0];
}

/** 这几格比的是"画法"，可它们量到的数由**状态**决定：两边状态天生不同，硬比就是把状态差报成画法差。
    名单只有这一份：node 那边（tests/lib/screens.mjs）import 这份文件现取，别处不抄第二遍。
    免：这一格比哪几样都算"把状态差报成画法差"，名单外的每一样照样必须逐字相等。
    结清了（两边其实一样）却还留着这条，守档同样红——留着就是给将来真漂移的那一格预留豁免。 */
export const STATE_ONLY = {
  '权限徽标': {
    免: ['box.x', 'box.w', 'fg', 'bg', 'contrast', '量的那句字'],
    reason: '浏览器直开拿不到宿主回执，这一格停在"需要管理员权限"那档（金色 warn）；原型写死的是常态档。字、字色、底色、宽度与位置全跟着这一档走，只有高度与字号算画法'
  },
  // 「类目行」整条已删（2026-10-25 钉档时量具裁定：fg/bg/contrast 三项两边都一模一样,状态差结清）
  '类目状态字': {
    免: ['box.y'],
    reason: '这一格挑的是"第一行非选中、且行本身没在跑状态"的那一行；两边演示数据里第一行满足条件的行不是同一序号，纵坐标自然不同。宽度、高度、字色与底色两边相同'
  },
  '队列标题': { 免: ['量的那句字'], reason: '标题里跟着队列条数走：我们这边没有真任务＝"队列0"，原型写死"队列 8"。字号与那块盒子相同' },
  '状态栏那格': {
    // box.y/h/visible 三项 2026-10-25 钉档轮补免：原型的演示状态机会换挡,换挡瞬间这一格整格隐藏
    // （量到 0×0/不可见）,量到哪一帧说不准。字色底色对比度两边逐字相同,照旧要守。
    免: ['box.x', 'box.w', 'box.y', 'box.h', 'visible', '量的那句字'],
    reason: '这一格的内容是后台服务报的：浏览器直开没有服务＝"后台服务未连上"，原型写死它的演示状态"启动自动化中"，字数不同宽度就不同、左边那格一宽它整格跟着挪；且原型的演示状态机会换挡,换挡瞬间这一格隐藏。字色底色相同'
  },
  '侧栏脚那颗钮': {
    免: ['量的那句字'],
    reason: '图标在我们这边是一个独立节点（件层 Icon），原型把箭头直接写在文字里，textContent 因而差一个符号。盒子、字号、字色与底色两边逐字相同'
  },
  // 「全局进度线」免 opacity 那条已删（2026-10-25 钉档时量具裁定：两边这次一模一样，状态差结清）
};

/** 逐屏计数的豁免：只有"这一格里摆了几个可点东西"由演示数据决定，别的一律要逐字相等 */
export const COUNT_STATE_ONLY = {
  // 「队列钮」那条豁免已删（2026-10-25 钉档时量具裁定：两边都是 5,状态差结清,留着＝登记没压差,照样红）
  '工具栏按钮': {
    reason: '2026-10-25 用户点名新增第 7 颗「打开Hosts」（原型没有这一颗：放开 hosts 编辑权限并拉起编辑器）。数的是钮数不是画法；少一颗才是漂'
  }
};

function rectOf(el) {
  const b = el.getBoundingClientRect();
  return { x: r2(b.x), y: r2(b.y), w: r2(b.width), h: r2(b.height) };
}

/** 一行"有字的"读数：盒子 ＋ 字号行高 ＋ 字色压在哪块底色上、够不够对比 ＋ 是不是真看得见 ＋ 量到哪一句字 */
function textReading(doc, anchor, wantText) {
  const [label, sel, skip] = anchor;
  const el = pick(doc, sel, skip, wantText);
  if (!el) return null;
  const fg = toRgba(cs(el, 'color'));
  const bg = bgOf(el);
  const b = rectOf(el);
  return {
    id: label, sel,
    量的那句字: textOf(el),
    box: b,
    visible: b.h > 0 && cs(el, 'visibility') !== 'hidden',
    font: fontOf(el),
    fg: fg && fg.join(','), bg: bg && bg.join(','),
    contrast: fg && bg ? contrastOf(fg, bg) : null
  };
}

/** 一块"没字的"读数：盒子、可见性、透明度、层级 */
function boxReading(doc, anchor) {
  const [label, sel] = anchor;
  const el = doc.querySelector(sel);
  if (!el) return null;
  const b = rectOf(el);
  const s = getComputedStyle(el);
  return {
    id: label, sel, box: b,
    visible: b.w > 0 && b.h > 0 && s.display !== 'none' && s.visibility !== 'hidden',
    opacity: +s.opacity, zIndex: s.zIndex, position: s.position
  };
}

/** 一个可操作节点真正能吃到的热区：那颗藏起来的真 input 是 0×0（Switch 与 Checkbox 都这么藏），
    要点中的是包住它的 label，所以量 label——量 input 会把每个开关都读成"热区 0"这种假缺陷。 */
function hitOf(node) {
  const face = node.closest('label') || node;
  const b = face.getBoundingClientRect();
  return { w: r2(b.width), h: r2(b.height) };
}

/* ---------- 两边共用的开场：先把树摊到"全部展开"，否则折叠那档是 display:none，量的是一堆 0 ---------- */

function expandAll(doc) {
  // 两份画面的"折叠了没"写在不一个地方：我们这边打在行的 aria-expanded 上，原型那边只给三角加 .collapsed。
  // 点完会重画：原型那份 renderTree 是整块 innerHTML 覆写，早先抓到的节点当场作废——
  // 所以每一轮都重新查一遍，查到再也找不到折叠的才收工（最多 20 轮，防死循环）。
  for (let round = 0; round < 20; round++) {
    const closed = [...doc.querySelectorAll('#tree .node.cat')].filter(c => {
      if (c.getAttribute('aria-expanded') === 'false') return true;
      const caret = c.querySelector('.caret');
      return !!caret && caret.classList.contains('collapsed');
    });
    if (!closed.length) return closed.length;
    for (const c of closed) {
      const caret = c.querySelector('.caret');
      (caret || c).dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: doc.defaultView }));
    }
  }
  return -1;
}

/** 这一行类目折叠了没：两份画面把状态写在不一个地方，所以两种都认（认不出来才算漂移） */
function isCollapsed(cat) {
  if (cat.hasAttribute('aria-expanded')) return cat.getAttribute('aria-expanded') === 'false';
  const caret = cat.querySelector('.caret');
  if (caret) return caret.classList.contains('collapsed');
  return null;
}

const visibleRows = doc => [...doc.querySelectorAll('#tree .node.item')]
  .filter(n => n.getBoundingClientRect().height > 0).length;

function snapshotCounts(doc) {
  const cats = [...doc.querySelectorAll('#tree .node.cat')];
  return {
    类目块: cats.length,
    域名行: doc.querySelectorAll('#tree .node.item').length,
    看得见的域名行: visibleRows(doc),
    展开的类目: cats.filter(c => isCollapsed(c) === false).length,
    认不出折叠态的类目: cats.filter(c => isCollapsed(c) === null).length,
    勾选点: doc.querySelectorAll('#tree .cb').length,
    工具栏按钮: doc.querySelectorAll('.toolbar .btn').length,
    状态栏格子: doc.querySelectorAll('.statusbar .item').length,
    队列钮: doc.querySelectorAll('#queuePanel button').length
  };
}

/* ---------- 键盘：件层那份靠 CSSOM 认焦点环，这里要的是"真按下去走不走得动"，所以发真事件 ---------- */

function press(el, key) {
  el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
  el.dispatchEvent(new KeyboardEvent('keyup', { key, bubbles: true, cancelable: true }));
}

function click(el) {
  el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: el.ownerDocument.defaultView }));
}

/** 树里当前那个 tabindex=0 的行（roving tabindex：整棵树只准留一行在 Tab 序里） */
function tabStopRow(doc) {
  const rows = [...doc.querySelectorAll('#tree [tabindex="0"]')];
  return { 停点数: rows.length, 落在: rows.length === 1 ? { kind: rows[0].dataset.kind, id: rows[0].dataset.id } : null };
}

/** 全站天然 Tab 停点：数给"键盘走一遍要点多少下"这笔账用（行里的勾是真 input，所以天然在序里） */
function tabStops(doc) {
  const all = [...doc.querySelectorAll('a[href], button, input, select, textarea, [tabindex]:not([tabindex="-1"])')]
    .filter(n => !n.disabled && n.offsetParent !== null && cs(n, 'visibility') !== 'hidden');
  return { 全站: all.length, 在树里: all.filter(n => n.closest('#tree')).length, 树里全是勾: all.filter(n => n.closest('#tree') && n.classList.contains('cb-src')).length };
}

/* ---------- 十一步交互：一屏一屏真点一遍，每步留读数，点完把状态摆回去 ---------- */

async function interactions() {
  const out = [];
  const btn = re => [...document.querySelectorAll('.toolbar .btn')].find(b => re.test(textOf(b) || ''));

  /* 1. 折叠／展开：那颗钮点一下，11 块的 aria-expanded 必须整批翻面，看得见的行数跟着变没 */
  const fold = btn(/折叠|展开/);
  const labelBefore = textOf(fold);
  const before = [...document.querySelectorAll('#tree .node.cat')].map(n => n.getAttribute('aria-expanded'));
  const visBefore = visibleRows(document);
  click(fold); await settle();
  const after = [...document.querySelectorAll('#tree .node.cat')].map(n => n.getAttribute('aria-expanded'));
  const visAfter = visibleRows(document);
  out.push({
    id: '折叠展开', 点之前那颗钮写着: labelBefore, 点之后写着: textOf(fold),
    整批翻面: before.length === after.length && before.length > 0 && before.every((v, i) => v !== after[i]),
    看得见行数: { 之前: visBefore, 之后: visAfter },
    翻面方向: visBefore > 0 && visAfter === 0 ? '摊到全折' : visBefore === 0 && visAfter > 0 ? '折到全摊' : '行没跟着翻（要看）',
    全折了: visAfter === 0
  });
  click(fold); await settle();          // 点回去，别影响下一步

  /* 2. 全选／全不选：那颗钮自己会翻字（全选↔全不选），两下分别推到满与空 */
  const selectBtn = () => btn(/全选|全不选/);
  const s0 = [...document.querySelectorAll('#tree .node.item .cb')].filter(n => n.dataset.state === 'checked').length;
  let b1 = selectBtn(); const label1 = textOf(b1); click(b1); await settle();
  const s1 = [...document.querySelectorAll('#tree .node.item .cb')].filter(n => n.dataset.state === 'checked').length;
  let b2 = selectBtn(); const label2 = textOf(b2); click(b2); await settle();
  const s2 = [...document.querySelectorAll('#tree .node.item .cb')].filter(n => n.dataset.state === 'checked').length;
  const total = document.querySelectorAll('#tree .node.item').length;
  out.push({
    id: '全选两下', 起始勾了: s0, 第一下: s1, 第二下: s2, 总共: total,
    第一下推到满或空: s1 === total || s1 === 0, 第二下推到另一头: (s1 === total && s2 === 0) || (s1 === 0 && s2 === total),
    那颗钮跟着翻了字: label1 !== label2, 字: [label1, label2]
  });

  /* 3. 键盘走树：方向键换落点、整棵树只准一个 Tab 停点、回车＝选中（条目打开详情、类目翻折叠） */
  const firstCat = document.querySelector('#tree .node.cat');
  firstCat.focus();
  const stop0 = tabStopRow(document);
  press(firstCat, 'ArrowDown'); await settle();
  const stop1 = tabStopRow(document);
  const rowNow = document.querySelector('#tree [tabindex="0"]');
  const detailTitle = () => textOf(document.querySelector('#detail .crumb'));
  const titleBefore = detailTitle();
  if (rowNow) press(rowNow, 'Enter');
  await settle();
  const titleAfter = detailTitle();
  out.push({
    id: '键盘走树', 起点: stop0.落在, 下一步: stop1.落在,
    落点唯一: stop1.停点数 === 1, 换行成功: JSON.stringify(stop0.落在) !== JSON.stringify(stop1.落在),
    回车那行的类型: rowNow && rowNow.dataset.kind,
    回车改了选中: titleBefore !== titleAfter || (!!titleAfter && rowNow && rowNow.dataset.kind === 'item'),
    详情标题: { 之前: titleBefore, 之后: titleAfter },
    Tab停点: tabStops(document)
  });

  /* 4. 空格改勾选：焦点落在行里那颗真 input 上按空格，勾必须翻；这是原型那 65 个 span 补出来的第二条路 */
  const cbInput = document.querySelector('#tree .node.item .cb-src');
  const faceBefore = cbInput.closest('.cb-wrap').querySelector('.cb').dataset.state;
  cbInput.focus();
  cbInput.checked = !cbInput.checked;
  cbInput.dispatchEvent(new Event('change', { bubbles: true }));
  await settle();
  const faceAfter = cbInput.closest('.cb-wrap').querySelector('.cb').dataset.state;
  out.push({ id: '空格改勾选', 焦点在那颗勾上: document.activeElement === cbInput, 之前: faceBefore, 之后: faceAfter, 翻了: faceBefore !== faceAfter });

  /* 5. 搜索：打两个字，看得见的行必须变少；清空必须一条不少地找回来 */
  const input = document.querySelector('.toolbar input[type="text"], .search input');
  const total0 = visibleRows(document);
  const type = v => { input.value = v; input.dispatchEvent(new Event('input', { bubbles: true })); };
  type('github'); await settle(400);            // 去抖之后才算数（main.js 那一条）
  const hit = visibleRows(document);
  type(''); await settle(400);
  const restored = visibleRows(document);
  out.push({ id: '搜索命中', 关键字: 'github', 全部: total0, 命中: hit, 清空后回到: restored, 变少了: hit < total0, 全部找回: restored === total0 });

  /* 6. 两道把手：键盘步进（件里那条 STEP=8），尺寸必须动、并且能回原位。
        真人鼠标那一趟做不了：件用 setPointerCapture，合成的 pointerId 会被它当场拒绝——这条如实归 #52。 */
  const sx = document.querySelector('.sidebar-resizer');
  const sy = document.querySelector('.resizer');
  const sideW = () => r2(document.querySelector('#sidebar').getBoundingClientRect().width);
  const queueH = () => r2(document.querySelector('#queuePanel').getBoundingClientRect().height);
  const w0 = sideW();
  sx.focus();
  for (let i = 0; i < 3; i++) press(sx, 'ArrowRight');
  await settle();
  const w1 = sideW();
  for (let i = 0; i < 3; i++) press(sx, 'ArrowLeft');
  await settle();
  const w2 = sideW();
  const h0 = queueH();
  sy.focus();
  press(sy, 'ArrowUp'); await settle();
  const h1 = queueH();
  press(sy, 'ArrowDown'); await settle();
  const h2 = queueH();
  out.push({
    id: '两道把手',
    侧栏: { 原: w0, 三步后: w1, 退回: w2, 每步八像素: r2(w1 - w0) === 24, 回得来: w2 === w0 },
    队列: { 原: h0, 一步后: h1, 退回: h2, 一步八像素: r2(h1 - h0) === 8, 回得来: h2 === h0 },
    热区: { 横向把手: hitOf(sx), 纵向把手: hitOf(sy) }
  });

  /* 7. 气泡：点一颗真会说话的钮（没服务时回的就是"连不上"那句），量它的层级与位置 */
  const toastBox = document.querySelector('#toast');
  click(btn(/检测全部/)); await settle(200);
  out.push({
    id: '气泡', 出现: !!toastBox && (toastBox.classList.contains('show') || boxOf(toastBox).h > 0),
    有字: !!toastBox && (textOf(toastBox) || '').length > 0,
    那句话: toastBox ? (textOf(toastBox) || '').slice(0, 40) : null,
    zIndex: cs(toastBox, 'zIndex'), 位置: toastBox ? rectOf(toastBox) : null
  });

  /* 8. 确认框：焦点必须进去、Esc 必须出来、焦点必须还给点它的那颗钮、遮罩必须拆干净 */
  const opener = btn(/恢复/);
  opener.focus();
  const ownerBefore = document.activeElement;
  click(opener); await settle(250);
  const mask = document.querySelector('.modal-mask');
  const inside = !!mask && mask.contains(document.activeElement);
  if (mask) press(mask.querySelector('.modal') || mask, 'Escape');
  await settle(250);
  out.push({
    id: '确认框', 打开: !!mask, 焦点进去了: inside,
    关掉后焦点回到点它的人: document.activeElement === ownerBefore,
    遮罩层级: mask ? cs(mask, 'zIndex') : null,
    遮罩拆干净: document.querySelectorAll('.modal-mask').length === 0
  });
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  await settle(120);

  /* 9. 设置中心七格逐个打开：每格都得有能点的控件，热区按"能点到的那一面"量 */
  const panes = [];
  const gear = [...document.querySelectorAll('.sidebar-footer .sf-btn')][1];
  click(gear); await settle(250);
  for (const item of [...document.querySelectorAll('.settings-nav .nav-item')]) {
    click(item); await settle(150);
    const pane = document.querySelector(`.settings-content [data-pane="${item.dataset.pane}"]`);
    const ctrls = pane ? [...pane.querySelectorAll('input,button,select,[role="switch"],.seg-item')] : [];
    const hits = ctrls.map(hitOf).filter(h => h.w > 0 && h.h > 0);
    panes.push({
      面板: item.dataset.pane,
      打开着: !!(pane && pane.classList.contains('active')),
      控件数: ctrls.length,
      行数: pane ? pane.querySelectorAll('.set-row').length : 0,
      最小热区: hits.length ? Math.min(...hits.map(h => Math.min(h.w, h.h))) : null,
      最矮的一格: hits.length ? hits.find(h => Math.min(h.w, h.h) === Math.min(...hits.map(x => Math.min(x.w, x.h)))) : null
    });
  }
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  await settle(200);
  out.push({ id: '七个面板', 面板数: panes.length, 明细: panes });

  /* 10. 托盘胶囊：菜单必须能开、六颗圆点的名单必须跟主题套数一样多、点一颗必须换面、再点回原套 */
  const pill = document.querySelector('#trayPill');
  click(pill); await settle(150);
  const dots = [...document.querySelectorAll('#trayMenu .theme-dot')];
  const ids = dots.map(d => d.dataset.dot);
  const initial = document.documentElement.dataset.theme;
  const other = ids.find(id => id !== initial) || ids[0];
  click(pill); await settle(120);
  click(pill); await settle(120);
  const dotAgain = document.querySelector(`#trayMenu .theme-dot[data-dot="${CSS.escape(other)}"]`);
  if (dotAgain) click(dotAgain);
  await settle(200);
  document.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  await settle(120);
  const landed = document.documentElement.dataset.theme;
  out.push({
    id: '胶囊与六套', 圆点数: ids.length, 名单: ids,
    那颗圆点写着: other, 点下去落到: landed, 换面成功: landed === other,
    菜单开合用的是同一颗胶囊: !!dotAgain
  });
  // 摆回开场那一套：这份量完还要再看一眼计数（finalCounts），别让这一步的脏状态进下一次比对
  const back = await restoreTheme(initial);
  out.push({ id: '切回原套', 落到: document.documentElement.dataset.theme, 回到原套: document.documentElement.dataset.theme === initial, 走过圆点: back });
  return out;
}

/** 主题切回：优先走真圆点（跟用户一样的路），圆点找不着才直接摆属性并如实记一笔 */
async function restoreTheme(initial) {
  const pill = document.querySelector('#trayPill');
  click(pill); await settle(120);
  const dot = document.querySelector(`#trayMenu .theme-dot[data-dot="${CSS.escape(initial)}"]`);
  let way = '圆点';
  if (dot) click(dot); else { document.documentElement.dataset.theme = initial; way = '直接摆属性（圆点没找到）'; }
  document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  await settle(150);
  return way;
}

/* ---------- 六套主题逐套走一遍：界面层只量"有字的锚点"压在每套底色上够不够对比 ----------
   切主题走的是真圆点（跟用户点的一样的路），每套量完记下"落没落到那一套"，最后一套量完摆回初始。
   每套切完等 320ms 才量：全站那些 color/background 都带 120~200ms 的过渡，
   等短了量到的是"过渡正走到一半"的那个中间色——上一轮就是这么把原型那侧的工具栏钮量成一片灰的（读数量到 131,131,135），
   中间色既不等于这套也不等于那套，两边一比就成了假漂移。 */

/** 一份画面、一套主题下，把每个锚点压在哪块底色上量出来（两边共用同一条挑选规则） */
function sweepReadings(doc, anchors) {
  return anchors.map(a => {
    const el = pick(doc, a.sel, a.skip, a.want);
    if (!el) return { id: a.id, 缺: true };
    const fg = toRgba(cs(el, 'color')), bg = bgOf(el);
    return { id: a.id, 量的那句字: textOf(el), fg: fg && fg.join(','), bg: bg && bg.join(','), contrast: fg && bg ? contrastOf(fg, bg) : null };
  });
}

async function themeSweep(anchors) {
  const pill = document.querySelector('#trayPill');
  const initial = document.documentElement.dataset.theme;
  const out = [];
  for (const id of THEMES) {
    click(pill); await settle(120);
    const dot = document.querySelector(`#trayMenu .theme-dot[data-dot="${CSS.escape(id)}"]`);
    if (!dot) { out.push({ theme: id, 量到: false, 原因: '胶囊菜单里找不到这颗圆点' }); continue; }
    click(dot); await settle(320);
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await settle(60);
    out.push({
      theme: id, 落在这一套: document.documentElement.dataset.theme === id,
      底色: (toRgba(cs(document.body, 'backgroundColor')) || []).join(','),
      读数: sweepReadings(document, anchors)
    });
  }
  await restoreTheme(initial);
  return { sweep: out, initial, finalTheme: document.documentElement.dataset.theme };
}

/* ---------- 与原型并排：同一批锚点、同一套主题、同一次运行、同一个视口 ---------- */

async function openPrototype() {
  const html = await (await fetch(PROTO_URL)).text();
  const f = document.createElement('iframe');
  f.setAttribute('aria-hidden', 'true');
  f.style.cssText = `position:fixed;left:0;top:0;width:${innerWidth}px;height:${innerHeight}px;border:0;opacity:0;pointer-events:none`;
  const loaded = new Promise(res => { f.onload = res; });
  f.srcdoc = html;
  document.body.appendChild(f);
  await loaded;
  await settle(300);
  return f;
}

async function prototypePass(sweepAnchors) {
  const f = await openPrototype();
  const d = f.contentDocument;
  const win = f.contentWindow;
  const initial = document.documentElement.dataset.theme;
  const stillClosed = expandAll(d);
  await settle(250);
  const boxes = {};
  for (const anchor of BOX_ANCHORS) { const v = boxReading(d, anchor); if (v) boxes[anchor[0]] = v; }
  const texts = {};
  for (const anchor of TEXT_ANCHORS) {
    const want = sweepAnchors.find(a => a.id === anchor[0]);
    const v = textReading(d, anchor, want && want.want);
    if (v) texts[anchor[0]] = v;
  }
  const counts = snapshotCounts(d);

  // 原型那六套：它自己那份 data-theme 一换，配色整套跟着换（本轮实测过：body 底色六套与我们那六套逐字相同）
  const sweep = [];
  for (const theme of THEMES) {
    d.documentElement.setAttribute('data-theme', theme);
    await settle(320);
    sweep.push({
      theme, 落在这一套: d.documentElement.dataset.theme === theme,
      底色: (toRgba(getComputedStyle(d.body).backgroundColor) || []).join(','),
      读数: sweepReadings(d, sweepAnchors)
    });
  }
  d.documentElement.setAttribute('data-theme', initial);
  const viewport = { w: r2(win.innerWidth), h: r2(win.innerHeight) };
  f.remove();
  return { boxes, texts, counts, sweep, viewport, 展开后还剩几个折叠: stillClosed, 文件名: PROTO_FILE };
}

/* ---------- 主入口 ---------- */

export async function run({ post = true } = {}) {
  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const initialTheme = document.documentElement.dataset.theme;
  const stillClosed = expandAll(document);
  await settle(250);

  const boxes = {};
  for (const anchor of BOX_ANCHORS) { const v = boxReading(document, anchor); if (v) boxes[anchor[0]] = v; }
  const texts = {};
  for (const anchor of TEXT_ANCHORS) { const v = textReading(document, anchor); if (v) texts[anchor[0]] = v; }
  // 挑行以我们这边为准：这边挑中哪一行（记着那句字），原型那边就照着同一句字去找同一行
  const sweepAnchors = TEXT_ANCHORS.map(([id, sel, skip]) => ({ id, sel, skip, want: texts[id] && texts[id].量的那句字 }));
  const counts = snapshotCounts(document);
  // 顺序有讲究：主题这一扫必须赶在交互那十步之前——十步里有回车选中、有折叠，会把"这一行现在算不算基准态"弄脏，
  // 而原型那边是刚端起来的干净一份，两边不在同一起点上比，报出来的又是状态差。
  const proto = await prototypePass(sweepAnchors);
  const themePart = await themeSweep(sweepAnchors);
  const acts = await interactions();
  // 交互那十步会改选择／折叠／面板，量完把界面摆回开场那一套再看一眼：读数必须在同一个起点上才谈得上"下次重量对得上"
  const finalCounts = snapshotCounts(document);

  const reading = {
    meta: {
      kind: KIND, takenAt: new Date().toISOString().slice(0, 19) + 'Z',
      viewport: { w: r2(innerWidth), h: r2(innerHeight), dpr: devicePixelRatio },
      theme: initialTheme, themes: THEMES, prefersReducedMotion: reduced,
      engine: navigator.userAgent.slice(0, 60),
      host: '浏览器直开（没有壳、没有真服务）',
      锚点: { 盒子: BOX_ANCHORS.map(a => a[0]), 文字: TEXT_ANCHORS.map(a => a[0]), 计数: Object.keys(counts) },
      只比画法跳过的: { 锚点: STATE_ONLY, 计数: COUNT_STATE_ONLY },
      展开后还剩几个折叠: stillClosed
    },
    boxes, texts, counts, interactions: acts,
    themeSweep: themePart.sweep, finalTheme: themePart.finalTheme,
    prototype: proto, finalCounts
  };

  if (post) {
    try { await fetch(SINK, { method: 'POST', body: JSON.stringify(reading) }); } catch (e) { /* 收数口没起就只回文本 */ }
  }
  return reading;
}

void frame;
