/* 这一轮补上的那六块界面的机器判据。干什么用：顶栏、横幅、工具栏、状态栏、侧栏脚、托盘胶囊——
   六块各建一次、各点几下，验四件事：
   ① 六块在 node 里 build 得出来、一句键都不缺（缺键会留下 ⟦…⟧ 并走 console.error，两样都抓）；
   ② 该跟着状态走的那几样真跟着走：权限徽标的字与底色、「有新版本」那颗红点、三颗大钮的禁用、
      全选／折叠那两颗互斥钮的两套文字、状态栏那两个数与「已选 n / m 项」的拆段、同步那颗钮的转圈与防重入、
      圆点跟着喂进来的主题套数走；
   ③ 事件从件里递出来的形状对：每一颗钮各回一次、回的是哪一句、回完界面收不收；
   ④ 外框那一层不再有正文：index.html 的 body 只有容器（那把尺在 tests/structure.test.mjs，本文件只管件的行为）。
   会错在哪：① 本文件用的是 tests/lib/dom-stub.mjs 那一小套 DOM——它不是浏览器的替身：
                没有布局、没有 computed style，样式对不对、颜色够不够、点得中点不中仍然只有真浏览器说得准
                （那部分判据见 前端目录结构规划.md 第 8 节，走查记录在 F5 那一节）；
             ② 这里喂的 themes、categories 是假形状，只为数出圆点与勾选数；
                真主题清单由 styles/themes/ 扫出来，真菜单由 menu/ 扫出来，两边都不是本文件的职责；
             ③ state 是全站那一份，用例会改它（adminGranted、queueRunning、勾选…），
                所以每块自己开头把要动的字段记下、结尾还原——漏一笔，后面那个文件读到的是脏状态；
             ④ 托盘胶囊有一条 document 级监听：只有挂在文档树上的容器才收得到（dom-stub 按真实冒泡做）。
   怎么回来：红消息说清是哪一块、哪一下点击、期望什么形状。缺键去 locales/zh-CN.js 补一句；
             行为不对去看那一块 views/<名>/<名>View.js，别在测试里改期望。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { installDom } from './lib/dom-stub.mjs';

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

installDom();

const { createI18n } = await import(pathToFileURL(path.join(FRONTEND, 'core/i18n/t.js')).href);
const { state, allCheckedState, allCollapsedState, setAllChecked } =
  await import(pathToFileURL(path.join(FRONTEND, 'core/store.js')).href);
const { createTopbarView } = await import(pathToFileURL(path.join(FRONTEND, 'views/topbar/TopbarView.js')).href);
const { createBannerView } = await import(pathToFileURL(path.join(FRONTEND, 'views/banner/BannerView.js')).href);
const { createToolbarView } = await import(pathToFileURL(path.join(FRONTEND, 'views/toolbar/ToolbarView.js')).href);
const { createStatusbarView } = await import(pathToFileURL(path.join(FRONTEND, 'views/statusbar/StatusbarView.js')).href);
const { createSidebarFooterView } = await import(pathToFileURL(path.join(FRONTEND, 'views/sidebar-footer/SidebarFooterView.js')).href);
const { createTrayPillView } = await import(pathToFileURL(path.join(FRONTEND, 'views/tray-pill/TrayPillView.js')).href);
const { ProgressBar } = await import(pathToFileURL(path.join(FRONTEND, 'components/progress-bar/ProgressBar.js')).href);

const zhCN = (await import(pathToFileURL(path.join(FRONTEND, 'core/i18n/locales/zh-CN.js')).href)).default;
const t = createI18n({ 'zh-CN': zhCN });

/** 缺键的两种表现都抓住：console.error 那一句，和界面上留下的 ⟦键⟧。跑完把两样一起交回去 */
function captureConsoleErrors(fn) {
  const seen = [];
  const origin = console.error;
  console.error = msg => seen.push(String(msg));
  return Promise.resolve()
    .then(fn)
    .then(out => ({ seen, out }))
    .finally(() => { console.error = origin });
}

const hasMark = node => /\u27e6[^\u27e7\u27e6]*\u27e7/.test(node.textContent);

/** 外框的那几个壳：类名由外框带（id 只有 main.js 认），这里照 index.html 的形状摆一份 */
function shell(className, { attach = true } = {}) {
  const node = document.createElement('div');
  node.className = className;
  if (attach) document.body.appendChild(node);
  return node;
}

/** 两条类目、四条条目：勾满与折满两种情况都能在这份小数据上算出来 */
function seedCategories() {
  const cats = [
    { id: 'a', name: '类目甲', items: [{ id: 'a1', name: '条目一', checked: false }, { id: 'a2', name: '条目二', checked: false }] },
    { id: 'b', name: '类目乙', items: [{ id: 'b1', name: '条目三', checked: false }, { id: 'b2', name: '条目四', checked: false }] }
  ];
  state.categories = cats;
  state.collapsed = new Set();
  return cats;
}

function makeThemes(n) {
  return Array.from({ length: n }, (_, i) => ({
    id: `th${i + 1}`, order: i + 1, name: `主题${i + 1}`, desc: `说明${i + 1}`,
    colors: ['#a0', '#a1', '#a2', '#a3']
  }));
}

test('六块各 build 一次：不抛错、不缺键、壳里留下的是容器不是正文', async () => {
  const restore = snapshot();
  try {
    const built = await captureConsoleErrors(() => {
      var topbarRoot = shell('topbar');
      var bannerRoot = shell('banner');
      var toolbarRoot = shell('toolbar');
      var statusRoot = shell('statusbar');
      var footerRoot = shell('sidebar-footer');
      var pillRoot = shell('tray-pill');
      var menuRoot = shell('tray-menu');
      const noop = () => { };
      return {
        topbar: createTopbarView({ root: topbarRoot, t, handlers: { onCheckUpdate: noop, onPermClick: noop } }),
        banner: createBannerView({ root: bannerRoot, t, handlers: { onClose: noop } }),
        toolbar: createToolbarView({ root: toolbarRoot, progress: ProgressBar({ kind: 'global' }).el, t, handlers: {} }),
        statusbar: createStatusbarView({ root: statusRoot, t }),
        footer: createSidebarFooterView({ root: footerRoot, t, handlers: { onSync: noop, onSettings: noop } }),
        tray: createTrayPillView({ pill: pillRoot, menu: menuRoot, themes: makeThemes(6), t, ctx: { onSetTheme: noop, onAction: noop } }),
        roots: { topbarRoot, bannerRoot, toolbarRoot, statusRoot, footerRoot, pillRoot, menuRoot }
      };
    });
    assert.deepEqual(built.seen, [], `这六块里有一句键在 zh-CN.js 里没有：${built.seen.join(' ｜ ')}`);
    const roots = built.out.roots;
    for (const [name, node] of Object.entries(roots)) {
      assert.ok(node.childNodes.length > 0, `${name} 那块 build 完是空的：多半是某个件没返回节点`);
      assert.ok(!hasMark(node), `${name} 那块的界面上留着 ⟦键⟧，说明有一句没补`);
    }
    assert.equal(roots.menuRoot.children.length, 4, '菜单里四块：圆点那一区、一条分隔、两条动作');
    assert.equal(roots.menuRoot.querySelectorAll('[data-dot]').length, 6, '圆点数＝喂进来的主题套数（这里给 6 套）');
    assert.equal(roots.toolbarRoot.querySelectorAll('button').length, 7, '工具栏 7 颗钮：三小＋三大＋搜索清空那颗');
  } finally {
    restore();
  }
});

/** 用例会动 state 的哪几样，就在结尾原样还回去 */
function snapshot() {
  const cats = state.categories;
  const collapsed = new Set(state.collapsed);
  const flags = {
    adminGranted: state.adminGranted, queueRunning: state.queueRunning,
    batchDetecting: state.batchDetecting, startupAutomationRunning: state.startupAutomationRunning,
    lastDetect: state.lastDetect, bannerDismissed: state.bannerDismissed
  };
  const dns = state.appStatus.dnsService;
  const theme = state.settings.theme;
  const update = { ...state.update };
  return () => {
    state.categories = cats;
    state.collapsed = collapsed;
    Object.assign(state, flags);
    state.appStatus.dnsService = dns;
    state.settings.theme = theme;
    Object.assign(state.update, update);
  };
}

test('顶栏：权限徽标跟着 state 换字换底色，「有新版本」是同一颗钮上的一个类', async () => {
  const restore = snapshot();
  try {
    const root = shell('topbar');
    const calls = [];
    const view = createTopbarView({ root, t, handlers: { onCheckUpdate: () => calls.push('update'), onPermClick: () => calls.push('perm') } });
    const perm = root.querySelector('.perm');
    const text = perm.children.find(k => !k.classList.contains('dot'));   // 徽标两颗：一颗点 + 一句字

    state.adminGranted = false;
    view.render();
    assert.equal(text.textContent, zhCN['topbar.permDenied'], '没权限时徽标要说「需要管理员权限」（原型 :2220）');
    assert.ok(perm.classList.contains('warn'), '没权限要给 .warn：字与底色一次判完，别处不能再判一遍');

    state.adminGranted = true;
    view.render();
    assert.equal(text.textContent, zhCN['topbar.permGranted']);
    assert.ok(!perm.classList.contains('warn'), '已授权还留着 .warn＝字说管理员、底色说没权限（两份源的老毛病）');

    state.update.hasUpdate = true;
    view.render();
    assert.ok(view.updateBtn.classList.contains('has-update'), '有新版本＝同一颗钮加一个 .has-update（红点在 layout.css :54）');
    state.update.hasUpdate = false;
    view.render();
    assert.ok(!view.updateBtn.classList.contains('has-update'));

    view.updateBtn.click();
    perm.click();
    assert.deepEqual(calls, ['update', 'perm'], '两颗钮各回一次：徽标说权限、钮走检查更新，别把 handler 接串');
    assert.equal(perm.getAttribute('title'), zhCN['topbar.permTitle']);
  } finally {
    restore();
  }
});

test('横幅：只显示与收掉两个出口，那颗钮把话递给外面、自己不偷偷收', async () => {
  const restore = snapshot();
  try {
    const root = shell('banner');
    let closed = 0;
    const view = createBannerView({ root, t, handlers: { onClose: () => { closed++; view.hide(); } } });
    assert.ok(!view.isOpen(), '开机时横幅还没显示（.show 由 main.js 在启动那一下加）');
    view.show();
    assert.ok(view.isOpen());
    assert.ok(root.classList.contains('show'), '显示靠 .show 这一个类，不是内联 display（深色那几条覆盖关系会乱）');

    const sentence = root.querySelectorAll('span')[1];
    assert.ok(sentence.querySelector('b'), '中间那句要带一个 <b>：原型 :683 的强调不能丢');
    assert.equal(sentence.textContent, `${zhCN['banner.pre']}${zhCN['banner.steps']}${zhCN['banner.rest']}`,
      '三段拼起来要跟原型那一句逐字一样（少一段就是语言文件里少一句）');

    root.querySelector('button').click();
    assert.equal(closed, 1);
    assert.ok(!view.isOpen(), '点了「知道了」要收掉；这条只在本轮运行里有效（原型没落盘）');
  } finally {
    restore();
  }
});

test('工具栏：忙碌时三颗大钮禁用，互斥钮的两套文字跟着 store 换', async () => {
  const cats = seedCategories();
  const restore = snapshot();
  try {
    const root = shell('toolbar');
    const line = ProgressBar({ kind: 'global' }).el;
    const view = createToolbarView({ root, progress: line, t, handlers: {} });
    const { detectAll, applyQueue, restore: restoreBtn, selectToggle, foldToggle } = view.buttons;

    assert.equal(root.childNodes.at(-1), line, '全局进度线贴在工具栏末尾：位置在这一行下沿，且只有一根');

    state.queueRunning = true;
    view.render();
    for (const [name, btn] of [['检测全部', detectAll], ['应用队列', applyQueue], ['恢复', restoreBtn]]) {
      assert.ok(btn.el.disabled, `队列在跑的时候「${name}」必须禁用（判据只在 core/task/gate.js 的 isBusy 里）`);
    }

    state.queueRunning = false;
    view.render();
    assert.deepEqual([detectAll.el.disabled, applyQueue.el.disabled, restoreBtn.el.disabled], [false, false, false],
      '不忙了要能点：禁用只跟着 isBusy，别留一颗焊死的');

    assert.equal(selectToggle.el.textContent, zhCN['toolbar.selectAll'], '一条没勾时那颗钮说「全选」');
    setAllChecked(true);
    view.render();
    assert.equal(selectToggle.el.textContent, zhCN['toolbar.selectNone'], '勾满了要说「全不选」——同一颗钮的另一种文字，不是第二颗钮');
    assert.equal(selectToggle.el.getAttribute('title'), zhCN['toolbar.selectNoneTitle']);
    setAllChecked(false);
    view.render();
    assert.equal(selectToggle.el.textContent, zhCN['toolbar.selectAll']);

    assert.ok(!allCollapsedState(), '这份假数据一开始一条也没折');
    assert.equal(foldToggle.el.textContent, zhCN['toolbar.fold']);
    cats.forEach(c => state.collapsed.add(c.id));
    view.render();
    assert.equal(foldToggle.el.textContent, zhCN['toolbar.unfold'], '折满了要说「展开」');
    assert.ok(allCheckedState() === false, '上面还原过勾选：这里应为假');
  } finally {
    restore();
  }
});

test('工具栏：搜索框每敲一下递一次值，清空那颗钮把值清掉再递空串', async () => {
  const restore = snapshot();
  try {
    const root = shell('toolbar');
    const got = [];
    const view = createToolbarView({ root, t, handlers: { onSearch: v => got.push(v) } });
    const input = view.search.input;
    input.value = 'git';
    input.dispatch('input');
    assert.deepEqual(got, ['git'], '件把当前值递出来，150 毫秒防抖归 main.js（原型 :3097）');
    assert.ok(view.search.el.classList.contains('has-value'), '有值才露出清空那颗钮');

    view.search.el.querySelector('.search-clear').click();
    assert.deepEqual(got, ['git', ''], '清空要既清掉框里的字、又递一次空串，否则树还按旧词过滤');
    assert.equal(input.value, '');
    assert.ok(!view.search.el.classList.contains('has-value'));
  } finally {
    restore();
  }
});

test('工具栏：模式下拉三项，只有 Hosts 能选，值开机就是 hosts', async () => {
  const restore = snapshot();
  try {
    const root = shell('toolbar');
    const view = createToolbarView({ root, t, handlers: {} });
    const opts = view.mode.el.options;
    assert.deepEqual(opts.map(o => o.value), ['hosts', 'proxy', 'pac'], '三项的顺序照原型 :696-698，换成语言也不动');
    assert.deepEqual(opts.map(o => !!o.disabled), [false, true, true],
      '「系统代理／PAC」是占位项：解析方式换档属后端那一侧，界面上今天点不动是对的');
    assert.equal(view.mode.el.value, 'hosts');
    assert.equal(root.querySelectorAll('select').length, 1, '模式下拉只有那颗：外框原来的 #modeSelect 已搬进件里，不留第二个壳');
  } finally {
    restore();
  }
});

test('侧栏脚：按下转圈＋换字，忙碌期间再点不重入', async () => {
  const restore = snapshot();
  try {
    const root = shell('sidebar-footer');
    let syncs = 0;
    let settings = 0;
    const view = createSidebarFooterView({ root, t, handlers: { onSync: () => { syncs++; }, onSettings: () => { settings++; } } });
    assert.equal(root.querySelectorAll('.sf-btn').length, 2, '只有两颗钮：同步与设置（原型 :710-718）');

    view.buttons.sync.click();
    assert.equal(syncs, 1);
    view.setSyncBusy(true);
    assert.ok(root.querySelector('.ico').classList.contains('spinning'), '图标要转：转圈规则在 layout.css 的 .sf-btn .ico.spinning');
    assert.equal(root.querySelectorAll('.sf-btn')[0].querySelector('span').textContent, zhCN['sidebar.syncBusy']);

    view.buttons.sync.click();
    assert.equal(syncs, 1, '正在同步时再点不算第二次同步（原型 :1724 那句防重入）');
    assert.ok(!view.buttons.sync.disabled, '这两颗钮不吃 disabled：置灰会让人以为同步坏了');

    view.setSyncBusy(false);
    assert.ok(!root.querySelector('.ico').classList.contains('spinning'));
    assert.equal(root.querySelectorAll('.sf-btn')[0].querySelector('span').textContent, zhCN['sidebar.sync']);
    view.buttons.settings.click();
    assert.equal(settings, 1);
    assert.ok(root.querySelectorAll('.sf-btn')[1].querySelector('svg'), '图标是内联 SVG（⟳ 与 ⚙ 那两个字符跨平台宽度不一样，4.2 已定全换）');
  } finally {
    restore();
  }
});

test('托盘胶囊：圆点数＝喂进来的套数，点当前这套不落盘', async () => {
  const restore = snapshot();
  try {
    const pill = shell('tray-pill');
    const menu = shell('tray-menu');
    const picked = [];
    const actions = [];
    const themes = makeThemes(3);
    state.settings.theme = 'th2';
    // onSetTheme 这里照 main.js 那个唯一出口 applyThemeId 的写法：记下 id 之外还把 state 改掉，
    // 件重画时读的就是 state——不这么喂，测出来的是"点了没反应"，而那其实是件本来就不该自己改状态。
    const view = createTrayPillView({
      pill, menu, themes, t,
      ctx: { onSetTheme: id => { picked.push(id); state.settings.theme = id; }, onAction: k => actions.push(k) }
    });

    assert.equal(menu.querySelectorAll('[data-dot]').length, 3, '给三套就画三个点：六个是这份数据的六，不是写死的六');
    assert.deepEqual(menu.querySelectorAll('.theme-dot.active').map(d => d.dataset.dot), ['th2'], '当前那套要点上 active');

    pill.click();
    assert.ok(view.isOpen(), '点胶囊开菜单（.show 在 #trayMenu 那一块上）');
    pill.click();
    assert.ok(!view.isOpen(), '再点一下收掉');

    pill.click();
    const dots = menu.querySelector('.theme-dots');
    dots.querySelector('[data-dot=th2]').click();
    assert.deepEqual(picked, [], '点的就是当前这套：不落盘、不重画（原型 :3301）');

    const other = dots.querySelector('[data-dot=th3]');
    other.click();
    assert.deepEqual(picked, ['th3'], '点另一套要递 id：换主题只有 main.js 那一个出口（applyThemeId）');
    assert.ok(view.isOpen(), '点圆点那一下件里喊了 stopPropagation：菜单不能跟着这一下收掉');
    assert.equal(dots.querySelectorAll('.theme-dot.active').map(d => d.dataset.dot).join(), 'th3', '递完自己也要补画一次，否则用户看的是"点了没反应"');

    view.items[1].click();
    assert.deepEqual(actions, ['quit'], '菜单里那两条各回一个动作名');
    assert.ok(!view.isOpen(), '选完动作收菜单');
  } finally {
    restore();
  }
});

test('托盘胶囊：点外面收掉，点胶囊与菜单内不收（stopPropagation 认）', async () => {
  const restore = snapshot();
  try {
    const pill = shell('tray-pill');
    const menu = shell('tray-menu');
    const view = createTrayPillView({ pill, menu, themes: makeThemes(2), t, ctx: { onSetTheme: () => { }, onAction: () => { } } });
    const elsewhere = shell('app');

    pill.click();
    assert.ok(view.isOpen());
    elsewhere.click();
    assert.ok(!view.isOpen(), '点界面别处要收掉菜单（原型 :3316-3318 判的是 id，这里换成类名）');

    pill.click();
    assert.ok(view.isOpen(), '点胶囊那一下自己不能被 document 那条监听收掉：件里喊了 stopPropagation');
    menu.click();
    assert.ok(view.isOpen(), '点菜单自己（圆点那一区）也不收');

    view.close();
  } finally {
    restore();
  }
});

test('数量对照：这六块各建一次之后，界面上的钮与格子总数对得上原型那几段', async () => {
  const cats = seedCategories();
  const restore = snapshot();
  try {
    const topbarRoot = shell('topbar');
    const toolbarRoot = shell('toolbar');
    const statusRoot = shell('statusbar');
    const footerRoot = shell('sidebar-footer');
    createTopbarView({ root: topbarRoot, t, handlers: { onCheckUpdate: () => { }, onPermClick: () => { } } });
    createToolbarView({ root: toolbarRoot, progress: ProgressBar({ kind: 'global' }).el, t, handlers: {} });
    createStatusbarView({ root: statusRoot, t });
    createSidebarFooterView({ root: footerRoot, t, handlers: { onSync: () => { }, onSettings: () => { } } });

    assert.equal(topbarRoot.querySelectorAll('button').length, 1, '顶栏只有一颗钮（检查更新）：窗口那三颗由壳画，界面不画假的');
    assert.equal(toolbarRoot.querySelectorAll('button').length, 7,
      '工具栏 7 颗＝三颗小钮（全选、反选、折叠）＋三颗大钮＋搜索框里那颗清空；对不上就是搬件时多了或缺了一颗');
    assert.equal(statusRoot.querySelectorAll('.item').length, 6, '状态栏 6 格：网络、启动那一格、模式、DNS、上次检测、已选');
    assert.equal(footerRoot.querySelectorAll('button').length, 2);
  } finally {
    restore();
  }
});

test('状态栏：那两个数与「已选 n / m 项」的三块形状', async () => {
  const cats = seedCategories();
  const restore = snapshot();
  try {
    const root = shell('statusbar');
    const view = createStatusbarView({ root, t });
    const items = root.querySelectorAll('.item');
    assert.equal(items.length, 6);

    assert.equal(items[1].style.display, 'none', '「启动自动化中」开机藏着（这一格用 display，[hidden] 盖不住 .statusbar .item 的 flex）');
    state.startupAutomationRunning = true;
    view.render();
    assert.equal(items[1].style.display, '', '启动自动化在跑就要露出来');

    state.lastDetect = '10:20:30';
    state.appStatus.dnsService = 'systemd-resolved';
    cats[0].items[0].checked = true;
    view.render();
    assert.equal(items[4].querySelector('b').textContent, '10:20:30');
    assert.equal(items[3].querySelector('b').textContent, 'systemd-resolved');

    const selection = items[5];
    const kids = selection.childNodes;
    assert.equal(kids.length, 3, '已选那一格是 前段＋<b>数</b>＋后段 三块：合成一句就得走 innerHTML，那是明令不走的路');
    assert.equal(kids[0].textContent, zhCN['statusbar.selectionPre']);
    assert.equal(kids[1].textContent, '1');
    assert.equal(kids[2].textContent, zhCN['statusbar.selectionRest'].replace('{m}', '4'));
    assert.ok(/ $/.test(kids[0].textContent) && /^ /.test(kids[2].textContent),
      '两段各自带一个空格（前段结尾一个、后段开头一个），那是 <b> 前后那两个空格——去掉任何一个，这行就比原型挤');
    assert.equal(selection.textContent,
      `${zhCN['statusbar.selectionPre']}${kids[1].textContent}${zhCN['statusbar.selectionRest'].replace('{m}', '4')}`,
      '三块拼起来就是界面上那一句：中间那个数单独加粗，前后两个空格一个也不能少');
  } finally {
    restore();
  }
});

test('缺键的那一种：状态栏与顶栏都不许留下 ⟦…⟧（上面各条已跑过 build，这一条专门数符号）', async () => {
  const restore = snapshot();
  try {
    const { seen } = await captureConsoleErrors(() => {
      createStatusbarView({ root: shell('statusbar'), t });
      createTopbarView({ root: shell('topbar'), t, handlers: { onCheckUpdate: () => { }, onPermClick: () => { } } });
      createBannerView({ root: shell('banner'), t, handlers: { onClose: () => { } } });
      createToolbarView({ root: shell('toolbar'), t, handlers: {} });
      createSidebarFooterView({ root: shell('sidebar-footer'), t, handlers: { onSync: () => { }, onSettings: () => { } } });
      createTrayPillView({ pill: shell('tray-pill'), menu: shell('tray-menu'), themes: makeThemes(1), t, ctx: { onSetTheme: () => { }, onAction: () => { } } });
    });
    assert.deepEqual(seen, [], `有键没补：${seen.join(' ｜ ')}`);
  } finally {
    restore();
  }
});
