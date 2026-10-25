/* 设置中心与主题弹窗这一层的机器判据。干什么用：一条命令（`node --test frontend/tests/*.test.mjs`）验四件事——
   ① 面板文件夹跟 前端目录结构规划.md 6.2 那张表逐名对齐、每个文件夹只有 pane.js ＋ README.md；
   ② 行级件跟 组件库规划.md 第 4 节那张表逐路径对齐（兜底 3 那六条数量断言的最后一条）；
   ③ 七格面板与外壳、主题弹窗，在 node 里真的 build() 一次、sync() 一次、点几下——不再只做源码扫描；
   ④ 视图层与 main.js 里不许刻死界面文案（与件层同一把尺，见 tests/lib/literals.mjs）。
   会错在哪：① 这一层以前只有"读源码的正则"在管，2026-10-22 那一轮真炸过一次：
                「检测升级」面板里有一句引用了根本不存在的变量，浏览器里 main.js 从那一行往后全不执行
                （树不画、按钮不接），而当时 99 条用例全绿。第 ③ 组用例就是为了让那一类错当场红。
             ② 本文件的 ③④ 两组靠 tests/lib/dom-stub.mjs 那一小套 DOM：它不是浏览器的替身，
                样式、颜色、点得中点不中仍然只有真浏览器说得准（那部分判据见前端目录结构规划.md 第 8 节与 F5 的落地判据）。
   怎么回来：红的时候看断言消息，它说清是哪一格面板、哪一行、缺哪个字段或哪句键没补。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { installDom, textOf } from './lib/dom-stub.mjs';
import { cjkLiterals } from './lib/literals.mjs';

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const ROOT = path.resolve(FRONTEND, '..');
const PANES = path.join(FRONTEND, 'views/settings/panes');
const VIEWS = path.join(FRONTEND, 'views');
const DIR_PLAN = path.join(ROOT, '前端目录结构规划.md');
const COMP_PLAN = path.join(ROOT, '组件库规划.md');

installDom();

const { collectPanes } = await import(pathToFileURL(path.join(PANES, 'collect.js')).href);
const { createI18n } = await import(pathToFileURL(path.join(FRONTEND, 'core/i18n/t.js')).href);
const { DEFAULTS } = await import(pathToFileURL(path.join(FRONTEND, 'core/settings.js')).href);
const { NATIVE_NAMES } = await import(pathToFileURL(path.join(FRONTEND, 'core/i18n/languages.js')).href);
const { state } = await import(pathToFileURL(path.join(FRONTEND, 'core/store.js')).href);
const { createSettingsView } = await import(pathToFileURL(path.join(FRONTEND, 'views/settings/SettingsView.js')).href);
const { createThemePickerView } = await import(pathToFileURL(path.join(FRONTEND, 'views/theme-picker/ThemePickerView.js')).href);

/** 真实那份语言表：面板里写错一个键，t() 会 console.error 并留下 ⟦键⟧，下面的用例就是把这两样都抓住 */
const zhCN = (await import(pathToFileURL(path.join(FRONTEND, 'core/i18n/locales/zh-CN.js')).href)).default;
const t = createI18n({ 'zh-CN': zhCN });

/** 六套主题的样子由 styles/themes 扫出来（这里给外壳与面板喂同一种形状，名字与真值无关） */
const THEMES = ['light', 'sakura', 'forest', 'ocean', 'midnight', 'oled'].map((id, i) => ({
  id, order: i + 1, name: `主题${i + 1}`, desc: `说明${i + 1}`,
  colors: ['#ffffff', '#f5f5f5', '#3b82f6', '#22c55e']
}));

async function paneFolders() {
  const dirs = [];
  for (const e of await readdir(PANES, { withFileTypes: true })) {
    if (e.isDirectory()) dirs.push(e.name);
  }
  return dirs.sort();
}

/** 从 6.2 那张表读"该有哪几格"：第二格是 data-pane 值，第三格是文件夹名，两格必须自己先对上 */
async function readPaneRoster() {
  const lines = (await readPlan(DIR_PLAN)).split('\n');
  const start = lines.findIndex(l => l.startsWith('### 6.2 '));
  assert.notEqual(start, -1, '前端目录结构规划.md 里找不到 6.2 那一节，这份对照就失效了：去看它是不是被改名或挪走');
  const rest = lines.slice(start + 1);
  const end = rest.findIndex((l, i) => i > 0 && /^### |^## /.test(l));
  const rows = rest.slice(0, end < 0 ? rest.length : end);
  const ids = [];
  for (const ln of rows) {
    const m = ln.match(/^\|[^|]+\|\s*`([a-z][a-z0-9-]*)`\s*\|\s*`[^`]*\/panes\/([a-z][a-z0-9-]*)\/`/);
    if (!m) continue;
    assert.equal(m[2], m[1], `6.2 那张表自己就不一致：data-pane 写 ${m[1]}，文件夹写 ${m[2]}`);
    ids.push(m[1]);
  }
  assert.ok(ids.length >= 7, `从 6.2 那张表只读出 ${ids.length} 格面板，表形一改解析就要跟着改，别让它悄悄读空`);
  return ids;
}

async function readPlan(file) {
  return readFile(file, 'utf8');
}

/** 从组件库规划第 4 节那张表读五个行级件的完整路径 */
async function readPartsRoster() {
  const out = [];
  for (const ln of (await readPlan(COMP_PLAN)).split('\n')) {
    const m = ln.match(/`((?:views|\u2026)\/[^`]*\/parts\/[A-Za-z]+\.js)`/);
    if (m) out.push(m[1].replace(/^\u2026\//, 'views/'));
  }
  assert.ok(out.length >= 5, `从 组件库规划.md 第 4 节那张表只读出 ${out.length} 个行级件路径：表形一改解析要跟着改`);
  return [...new Set(out)].sort();
}

async function findParts() {
  const out = [];
  const walk = async dir => {
    for (const e of await readdir(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) await walk(p);
      else if (e.name.endsWith('.js') && path.basename(path.dirname(p)) === 'parts') out.push(path.relative(FRONTEND, p));
    }
  };
  await walk(VIEWS);
  return out.sort();
}

/** 面板要的那个 ctx：值读工作副本，四个出口各记一笔，好在断言里看"这一下到底走了哪个出口" */
function makeCtx(over = {}) {
  const draft = { ...DEFAULTS, ...(over.draft || {}) };
  const calls = { set: [], setDraft: [], toast: [], expand: 0, picker: 0, check: 0, download: 0, apply: 0, persist: 0 };
  const ctx = {
    t,
    state,
    themes: over.themes || THEMES,
    langs: () => [{ value: 'zh-CN', label: '简体中文' }],
    get: key => draft[key],
    set: (key, value) => { draft[key] = value; calls.set.push(key); },
    setDraft: (key, value) => { draft[key] = value; calls.setDraft.push(key); },
    expandQueue: () => { calls.expand++; },
    toast: text => { calls.toast.push(text); },
    openThemePicker: () => { calls.picker++; },
    checkUpdate: () => { calls.check++; return Promise.resolve(over.checkResult === undefined
      ? { hasUpdate: true, latest: 'v1.1.0', log: ['一条', '两条'] } : over.checkResult); },
    downloadUpdate: () => { calls.download++; return Promise.resolve(); },
    applyNow: () => { calls.apply++; },
    persist: () => { calls.persist++; },
    draft,
    calls
  };
  return ctx;
}

/** 建一格面板，顺手把"build 期间 console.error 过"与"界面上留下 ⟦键⟧"两件事抓住 */
async function buildPane(def, ctx) {
  const errors = [];
  const real = console.error;
  console.error = (...args) => { errors.push(args.join(' ')); };
  try {
    const instance = def.build(ctx);
    instance.sync();
    instance.sync();
    return { instance, errors, ctx };
  } finally {
    console.error = real;
  }
}

const paneDefs = new Map();
for (const dir of await paneFolders()) {
  paneDefs.set(dir, (await import(pathToFileURL(path.join(PANES, dir, 'pane.js')).href)).default);
}
/** collectPanes 认的是构建工具那份形状（{ './<面板id>/pane.js': 模块 }），这里照它拼一份，
    免得用例传 Map 进去——它读 Object.entries(Map) 会得到空表，然后拿"一格也没扫到"那条错报红 */
const paneModules = Object.fromEntries([...paneDefs].map(([id, mod]) => [`./${id}/pane.js`, { default: mod }]));
const PANE_LIST = collectPanes(paneModules);

test('设置面板文件夹与 6.2 那张表逐名对齐，一个不多一个不少（兜底 3 第 3 条）', async () => {
  const roster = await readPaneRoster();
  const found = await paneFolders();
  assert.deepEqual(found, [...roster].sort(),
    '名单只有一份，就是 前端目录结构规划.md 6.2 那张表：多一个文件夹＝没登记就建，少一个＝登记了没建');
});

test('每个面板文件夹只有 pane.js ＋ README.md，README 那三行一条不缺', async () => {
  for (const dir of await paneFolders()) {
    const files = (await readdir(path.join(PANES, dir))).sort();
    assert.deepEqual(files, ['README.md', 'pane.js'],
      `panes/${dir}/ 里的文件不是"pane.js ＋ README.md"这两件：要加 pane.css 得先写清为什么 tokens.css 解决不了（第 7 节那条逃生口）`);
    const readme = await readFile(path.join(PANES, dir, 'README.md'), 'utf8');
    for (const h of ['这一格管什么', '值从哪来', '谁能用它']) {
      assert.ok(readme.includes(h), `panes/${dir}/README.md 缺「${h}」这一行（第 8 节兜底 2：不许有只放说明没有代码的壳）`);
    }
  }
});

test('行级件与 组件库规划.md 第 4 节那张表逐路径对齐（兜底 3 第 4 条：parts 里的件 = 5）', async () => {
  const roster = await readPartsRoster();
  const found = await findParts();
  assert.equal(found.length, 5, `views/**/parts/ 里现在有 ${found.length} 个行级件，文档那条算式是 5`);
  assert.deepEqual(found, roster, '行级件该在哪条路径上，只由那张表说：位置漂了＝件与视图的归属对不上');
});

test('七格面板在 node 里各 build 一次、sync 两次：不抛错、不缺键、形状对（那一轮 main.js 全炸的根因就在这）', async () => {
  const panes = collectPanes(paneModules);
  assert.equal(panes.length, 7, '扫出来的面板应为 7（第 6.2 节）');
  for (const pane of panes) {
    const ctx = makeCtx();
    const { instance, errors } = await buildPane(pane, ctx);
    const where = `面板 ${pane.id}`;
    assert.ok(instance && instance.el, `${where} 的 build() 没返回 { el, sync }`);
    assert.equal(typeof instance.sync, 'function', `${where} 的 build() 没给出 sync()：外壳每次打开都要靠它把值补回去`);
    assert.ok(instance.el.classList.contains('settings-pane'), `${where} 的根节点没有 .settings-pane 这个类：外壳靠它切显示`);
    assert.deepEqual(errors, [], `${where} 建的时候 i18n 报过错：\n  ${errors.join('\n  ')}`);
    const text = textOf(instance.el);
    assert.ok(!text.includes('⟦'), `${where} 界面上留下了 ⟦键名⟧：说明有一句键在 zh-CN.js 里没有`);
    assert.ok(text.length > 20, `${where} 画出来几乎是空的（只有 ${text.length} 个字），多半是某个卡片没排进去`);
  }
});

test('面板的读与写各走对出口：档位与缩放立刻生效（ctx.set），开关与语言等保存（ctx.setDraft）', async () => {
  const layout = await buildPane(paneDefs.get('layout'), makeCtx());
  const segs = layout.instance.el.querySelectorAll('.seg');
  assert.equal(segs.length, 2, '「布局」那格应有两档分段（宽度、队列高）');
  segs[0].querySelectorAll('button')[2].dispatch('click');
  assert.deepEqual(layout.ctx.calls.set, ['sidebarWidth'], '点宽度那一档必须走 ctx.set（立刻生效并落盘），现在走的不是它');
  assert.equal(textOf(layout.instance.el.querySelector('.update-status')), '480 px',
    '点完那档，右边那行读数要跟着变：只刷分段不刷读数，看着就像没生效');

  const startup = makeCtx();
  const built = await buildPane(paneDefs.get('startup'), startup);
  const inputs = built.instance.el.querySelectorAll('input[type=checkbox]');
  assert.equal(inputs.length, 4, '「开机自启动」那格应有四个开关');
  inputs[0].checked = false;
  inputs[0].dispatch('change');
  assert.deepEqual(startup.calls.setDraft, ['autoStart'], '开关只许改工作副本，点保存才落盘');
  assert.equal(startup.calls.set.length, 0, '开关不该走 ctx.set：那样「取消」就变成了骗人的');
});

test('「开机自启动」那条依赖在界面上是三件事一起：开关禁掉、那一行 aria-disabled、卡片加 .disabled', async () => {
  const ctx = makeCtx({ draft: { autoDetectOnStartup: false } });
  const { instance } = await buildPane(paneDefs.get('startup'), ctx);
  const cards = instance.el.querySelectorAll('.set-card');
  const last = cards[cards.length - 1];
  assert.ok(last.classList.contains('disabled'), '自动检测没开时，自动应用那张卡片应整张灰掉（原型 :1521-1528）');
  assert.equal(last.querySelector('.set-row').getAttribute('aria-disabled'), 'true',
    '那一行要带 aria-disabled：读屏才知道这句不能改，不然说明看着还能点');
  assert.equal(last.querySelector('input[type=checkbox]').disabled, true, '那颗开关本身也要真禁用');
});

test('「检测升级」与「关于我们」两格不显示「保存」，其余五格显示（原型 :1551 用的是 visibility）', async () => {
  const view = makeView();
  const save = view.root.querySelector('[data-role=save]');
  for (const pane of PANE_LIST) {
    view.view.open(pane.id);
    assert.equal(save.style.visibility, pane.hasSave ? 'visible' : 'hidden',
      `${pane.id} 这一格的「保存」该${pane.hasSave ? '露出来' : '藏起来'}；藏起来要用 visibility，用 display 会把「取消」那颗一起挪走`);
  }
});

test('外壳：导航条数＝面板数、顺序照 order，切格时标题下面那句跟着换', async () => {
  const view = makeView();
  const items = view.root.querySelectorAll('.nav-item');
  const panes = PANE_LIST;
  assert.equal(items.length, panes.length, '导航那几条与扫出来的面板必须一一对应');
  assert.deepEqual(items.map(i => i.dataset.pane), panes.map(p => p.id), '导航顺序就是 order 顺序，别在别处再排一遍');
  for (let i = 0; i < items.length; i++) {
    items[i].dispatch('click');
    const active = view.root.querySelectorAll('.settings-pane').filter(p => p.classList.contains('active'));
    assert.equal(active.length, 1, `${panes[i].id}：切过去应当只有一格是 active`);
    assert.equal(active[0].dataset.pane, panes[i].id);
    assert.ok(!textOf(view.root.querySelector('.settings-head .sub')).includes('⟦'), `${panes[i].id} 的副标题键没在 zh-CN.js 里`);
  }
});

test('外壳：键盘方向键与 Home/End 换格；Esc 之外那两颗钮（关闭／取消）都收掉弹窗', async () => {
  const view = makeView();
  view.view.open('layout');
  assert.ok(view.root.classList.contains('show'), 'open() 之后遮罩该有 .show');
  const items = view.root.querySelectorAll('.nav-item');
  items[0].dispatch('keydown', { key: 'ArrowDown' });
  assert.equal(view.view.currentPane(), 'zoom', '方向键该往下走一格');
  items[1].dispatch('keydown', { key: 'End' });
  assert.equal(view.view.currentPane(), 'about', 'End 该到最后一格');
  items[6].dispatch('keydown', { key: 'ArrowDown' });
  assert.equal(view.view.currentPane(), 'layout', '最后一格再往下该绕回第一格');
  view.root.querySelector('[data-role=cancel]').dispatch('click');
  assert.equal(view.root.classList.contains('show'), false, '「取消」要把弹窗收掉');
});

test('外壳：「保存」把九项延后值落进 state 并落盘，「取消」只是扔掉工作副本', async () => {
  const ctx = makeCtx();
  const root = document.createElement('div');
  const view = createSettingsView({ root, t, ctx: { ...ctx, panes: PANE_LIST } });
  Object.assign(state.settings, DEFAULTS);
  view.open('tray');
  // 面板拿的是外壳内部那份工作副本（ctx.setDraft 写的就是它），外面那份 ctx.draft 它一个字也不碰，
  // 所以这里只对真状态：没点保存之前 state 不许动，点了保存才跟着改。
  const box = '.settings-pane[data-pane=tray] input';
  const input = root.querySelector(box);
  const original = state.settings.trayMinimize;
  input.checked = !original;
  input.dispatch('change');
  assert.equal(state.settings.trayMinimize, original, '没点保存之前，真状态不该被动过');
  // 外壳没有公开 save()：保存这件事只有「保存」那颗钮能触发，用例就按人那一下点它
  root.querySelector('[data-role=save]').dispatch('click');
  assert.equal(state.settings.trayMinimize, !original, '点了保存，工作副本里那一项就该落进真状态');
  assert.ok(ctx.calls.persist >= 1, '保存要落盘一次，否则重启就回到旧值');
  assert.ok(ctx.calls.toast.some(x => typeof x === 'string' && x.length), '保存完该有一句回执（原型 :1616）');

  view.open('tray');
  const again = root.querySelector(box);
  assert.equal(again.checked, !original, '上一次保存的值，这次打开要还看得见');
  again.checked = original;
  again.dispatch('change');
  root.querySelector('[data-role=cancel]').dispatch('click');
  view.open('tray');
  assert.equal(root.querySelector(box).checked, !original,
    '「取消」之后重新打开，那一项要回到取消之前的值：工作副本每次 open 从真状态重抄一份');
});

test('主题弹窗：六张卡由外面喂进来的主题清单生成，点一张立刻回一个 setTheme', async () => {
  const picked = [];
  const root = document.createElement('div');
  const view = createThemePickerView({
    root, t,
    // 真的那个出口由 main.js 给，它改完 state 才重画；这里的假件要照做同一件事，
    // 否则弹窗画选中态用的 state.settings.theme 永远停在旧值，断言就变成验一件不存在的行为
    ctx: { themes: THEMES, setTheme: id => { picked.push(id); state.settings.theme = id; } }
  });
  Object.assign(state.settings, { theme: 'light' });
  view.open();
  const cards = root.querySelectorAll('.theme-card');  assert.equal(cards.length, THEMES.length, '卡片数＝扫出来的主题数，这里不写死六套');
  assert.equal(root.querySelector('.theme-card.selected').dataset.themeId, state.settings.theme,
    '打开时选中的那张应当就是当前主题');
  cards[3].dispatch('click');
  assert.deepEqual(picked, [THEMES[3].id], '点卡片要把那一个主题 id 报给外面（改值与落盘归外面那一处做）');
  assert.equal(root.querySelectorAll('.theme-card').length, THEMES.length, '重画之后卡片数不变：网格不许越点越多');
  assert.equal(root.querySelector('.theme-card.selected').dataset.themeId, THEMES[3].id, '重画后选中标记要挪到新那张');
  view.close();
  assert.equal(root.classList.contains('show'), false);
});

test('collectPanes 的守卫：撞号、组名写错、id 与文件夹不一致、build 不是函数，全部当场抛', () => {
  const base = { id: 'a', order: 1, group: 'prefs', icon: '▦', nameKey: 'x.y', subKey: 'x.z', build: () => ({}) };
  assert.throws(() => collectPanes({
    './a/pane.js': { default: base },
    './b/pane.js': { default: { ...base, order: 2, nameKey: 'x.b', subKey: 'x.bb' } }
  }), /面板 id「a」和文件夹名「b」不一致/);
  assert.throws(() => collectPanes({
    './a/pane.js': { default: base },
    './b/pane.js': { default: { ...base, id: 'b', nameKey: 'x.b', subKey: 'x.bb' } }
  }), /order 撞号/);
  assert.throws(() => collectPanes({ './a/pane.js': { default: { ...base, group: 'pref' } } }), /group 是「pref」/);
  assert.throws(() => collectPanes({ './a/pane.js': { default: { ...base, id: 'z' } } }), /和文件夹名「a」不一致/);
  assert.throws(() => collectPanes({ './a/pane.js': { default: { ...base, build: 1 } } }), /build 不是函数/);
  assert.throws(() => collectPanes({ './a/pane.js': { default: { ...base, hasSave: 'yes' } } }), /hasSave 得是布尔值/);
  assert.throws(() => collectPanes({ './a/pane.js': { default: { ...base, icon: undefined } } }), /缺字段 icon/);
  assert.throws(() => collectPanes({ './a/other.js': { default: base } }), /不在 panes/);
  assert.throws(() => collectPanes({}), /一个面板也没扫到/);
  const sorted = collectPanes({
    './b/pane.js': { default: { ...base, id: 'b', order: 2, nameKey: 'x.b', subKey: 'x.bb', group: 'info' } },
    './a/pane.js': { default: base }
  });
  assert.deepEqual(sorted.map(p => p.id), ['a', 'b'], '清单按 order 排，不按构建工具返回的顺序');
  assert.deepEqual(sorted.map(p => p.hasSave), [true, true], 'hasSave 不写就等于 true（只有两格信息面板要写 false）');
});

test('门禁：视图层与 main.js 里不许刻死界面文案（与件层同一把尺）', async () => {
  const files = [];
  const walk = async dir => {
    for (const e of await readdir(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) await walk(p);
      else if (e.name.endsWith('.js')) files.push(p);
    }
  };
  await walk(VIEWS);
  files.push(path.join(FRONTEND, 'main.js'));
  let scanned = 0;
  for (const p of files) {
    const src = await readFile(p, 'utf8');
    scanned += src.split('\n').length;
    for (const { body, line } of cjkLiterals(src)) {
      assert.fail(`${path.relative(ROOT, p)}:${line} 里刻了一句中文「${body.slice(0, 40)}」：`
        + '界面文案的家只有 locales/zh-CN.js 那一份，写死在视图里，"新增一种语言＝只新增一个语言文件"当场作废（总纲 C7）');
    }
  }
  // 防空转的下限：本轮实测 views/ 21 个 .js 加 main.js，合计 2528 行。
  // 门槛取"少一半也算红"，扫到 0 个文件却全绿的那条门禁比没有更坏。
  assert.ok(files.length >= 20 && scanned > 2000,
    `视图层只扫到 ${files.length} 个文件、${scanned} 行，多半是扫描目录漂了：这条门禁空转比不跑更坏`);
});

test('每门语言在 languages.js 里都有自己的写法（下拉里不许出现没有名字的语言）', async () => {
  const ids = (await readdir(path.join(FRONTEND, 'core/i18n/locales')))
    .filter(f => f.endsWith('.js') && f !== 'index.js' && f !== 'collect.js')
    .map(f => f.replace(/\.js$/, ''))
    .sort();
  assert.ok(ids.length >= 1, 'locales/ 下一个语言文件也没有：这条门禁空转');
  for (const id of ids) {
    assert.ok(NATIVE_NAMES[id], `语言 ${id} 在 core/i18n/languages.js 的 NATIVE_NAMES 里没有写法：`
      + '下拉里它会显示成裸 id，切过去也认不出是哪门（那份表的家就在 languages.js）');
  }
});

/** 外壳用例共用的一份：每次新建一个遮罩节点，免得用例之间互相留痕 */
function makeView() {
  const root = document.createElement('div');
  const ctx = makeCtx();
  const view = createSettingsView({ root, t, ctx: { ...ctx, panes: PANE_LIST } });
  return { root, ctx, view };
}

/* 「检测升级」这一格的两条判据。干什么用：本期服务对 CheckUpdate 如实回一句「本期不提供」（GAC-UPDATE-501），
   这一格不许拿界面话术盖住它，也不许在版本历史里摆编出来的旧版本（本轮删掉的就是那两行 v0.9.0／v0.5.0）。
   会错在哪：查过一次却失败，那一格退回「尚未检查」＝界面在说"没查过"，而服务明明答过话；
             历史里留一行编出来的版本号，用户会照着它判断"这台机器升过几版"。
   怎么回来：红了看是 statusText() 那一串判据漂了，还是这一格又开始自己造数。 */
test('「检测升级」：服务回过话那一格就不写「尚未检查」；版本历史只有真读数那一行', async () => {
  const saved = { ...state.update };
  try {
    Object.assign(state.update, {
      current: 'v1.2.3', buildDate: '2026-10-23', latest: '', hasUpdate: false,
      checking: false, downloading: false, downloaded: false, progress: 0, speed: '', log: [],
      checkError: '本期不提供更新检查'
    });
    const failed = await buildPane(paneDefs.get('update'), makeCtx());
    const status = failed.instance.el.querySelector('.update-status');
    assert.equal(textOf(status), '本期不提供更新检查',
      '服务答过话（哪怕是"本期不提供"），这一格要说那一句，不能替用户判断成"还没查过"');
    assert.ok(status.className.split(' ').includes('err'), '那一句要走 err 那一档：跟「已是最新」「发现新版本」区分开');

    const rows = failed.instance.el.querySelectorAll('.ver-h-item');
    assert.equal(rows.length, 2,
      '版本历史今天只有两行：当前版本那一行，加一句"更早的历史归更新通道"。多出来的每一行都是编的');
    assert.ok(textOf(rows[0]).includes('v1.2.3'), '当前版本那一行要跟着 state.update.current（GetVersion 回的那一个数）');
    assert.doesNotMatch(rows.map(textOf).join(' '), /v0\.\d|28 MB|1 分钟/,
      '这一格里出现了本轮删掉的那种编出来的数：旧版本号、更新包大小、耗时');

    Object.assign(state.update, { checkError: '' });
    const idle = await buildPane(paneDefs.get('update'), makeCtx());
    assert.equal(textOf(idle.instance.el.querySelector('.update-status')), t('settings.update.idle'),
      '真没查过（checkError 空、latest 空）才写「尚未检查」');
  } finally {
    Object.assign(state.update, saved);
  }
});

/* 保存那一句要说满。干什么用：那四把开关（core/settings.js 的 SHELL_KEYS）动的不是设置文件里的行为，
   而是窗口与开机启动项——那条通道归程序外壳，今天还不存在。
   会错在哪：只回一句「设置已保存」，用户以为开机自启动已经生效；
             连着弹三只气泡也不行——#toast 只有一个节点，后一句会把前一句盖掉（components/toast/Toast.js）。
   怎么回来：红了看 SettingsView 的 save()：note 那两句拼的是同一句还是分两次 show。 */
test('保存回执要说满：动了那四把只有外壳能动的开关，就要带上"本次运行看不出效果"那半句', async () => {
  const savedSettings = { ...state.settings };
  try {
    Object.assign(state.settings, DEFAULTS);
    const ctx = makeCtx();
    const root = document.createElement('div');
    const view = createSettingsView({ root, t, ctx: { ...ctx, panes: PANE_LIST } });

    view.open('tray');
    const input = root.querySelector('.settings-pane[data-pane=tray] input');
    input.checked = !state.settings.trayMinimize;
    input.dispatch('change');
    root.querySelector('[data-role=save]').dispatch('click');
    const note = t('settings.shellPending', { n: 1 });
    assert.ok(ctx.calls.toast.some(x => x.includes(t('settings.saved')) && x.includes(note)),
      `动了外壳那几把开关，保存回执里少了这半句：「${note}」——少这一句就是让用户以为已经生效`);
    assert.equal(ctx.calls.toast.length, 1, '一次保存只许弹一只气泡：#toast 只有一个节点，连着 show 只剩最后那句看得见');

    ctx.calls.toast.length = 0;
    view.open('tray');
    root.querySelector('[data-role=save]').dispatch('click');
    assert.deepEqual(ctx.calls.toast, [t('settings.saved')],
      '什么都没改就不该带上那半句：说了"这 0 项还没接"比不说更糊涂');
  } finally {
    Object.assign(state.settings, savedSettings);
  }
});
