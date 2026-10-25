/* F1+F2 这两步的机器判据。干什么用：一条命令（`node --test frontend/tests/`）验十件事——菜单文件夹齐不齐、11 块 54 条对不对得上、跟原型逐字一样吗、"不许空文件夹"和"菜单里不许画界面"那两条门禁破了没、撞号会不会报错、注释有没有被自己写的那个通配符截断、外框 id 与 JS 点名的 id 是不是双向对齐、外框 body 里还刻没刻着中文、视图里还往文档上找节点吗。
   会错在哪：这些数以前只写在文档里靠人眼对，改菜单的人不回头改文档，文档就变成假话；这里改成现场量——原型那份文件是它的对照物，两边一漂就红。
   怎么回来：红的时候看断言消息，它说清是哪一块、哪一条、跟原型的第几行不一样。
             文档第 8 节第 3 条那六条数量断言现在分在三个文件里跑：本文件管菜单 11 块与 54 条，
             styles.test.mjs 管主题 6 与 components 17，views-settings.test.mjs 管设置面板 7 与行级件 5。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { collectMenuModules } from '../menu/collect.js';
import { ItemMeta } from '../core/meta.js';
import { readPrototypeCategories } from './lib/prototype-menu.mjs';
import { readMenuSourceDirs, loadFrontendMenus } from './lib/menu-source.mjs';
import { CJK } from './lib/literals.mjs';

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const ROOT = path.resolve(FRONTEND, '..');
const MENU_DIR = path.join(FRONTEND, 'menu');

// 目录怎么读、模块怎么 import 在 ./lib/menu-source.mjs（后端那条门禁 G4 用同一把尺子）；
// 这里只加"不许空文件夹、菜单文件夹里只许这三种文件"那两条门禁断言。
async function readMenuFolders() {
  const folders = await readMenuSourceDirs();
  for (const { name, files } of folders) {
    assert.ok(files.includes('module.js'), `menu/${name}/ 缺 module.js（第 8 节兜底 2：不许有只放说明没有代码的壳）`);
    assert.ok(files.includes('README.md'), `menu/${name}/ 缺 README.md（这块菜单管什么·条目从哪来·谁能用它，三行）`);
    for (const f of files) {
      assert.match(f, /^(module\.js|README\.md|module\.css)$/,
        `menu/${name}/${f} 是第 3 节目录树里没有的文件形状：菜单文件夹只允许这三种`);
    }
    if (files.includes('module.css')) {
      assert.fail(`menu/${name}/ 建了 module.css：第 7 节说这是逃生口，建之前要先写清为什么 tokens.css 解决不了`);
    }
  }
  return { names: folders.map(f => f.name), modules: await loadFrontendMenus(folders) };
}

test('菜单文件夹与原型 CATEGORY_DEFS 的块一一对应，一块不多一块不少', async () => {
  const cats = await readPrototypeCategories();
  const { names } = await readMenuFolders();
  assert.deepEqual(names, cats.map(c => c.id).sort(),
    '目录名＝菜单 id：加一块菜单＝加一个文件夹，文件夹名就是它');
  assert.equal(names.length, 11, '扫出来的菜单文件夹应为 11（第 8 节兜底 3 第 1 条）');
});

test('二级条目合计 54，且与原型逐字一致（id、名字、域名、初态、排第几）', async () => {
  const cats = await readPrototypeCategories();
  const { modules } = await readMenuFolders();
  const menus = collectMenuModules(modules);
  assert.equal(menus.reduce((n, m) => n + m.items.length, 0), 54, '二级条目合计应为 54（第 8 节兜底 3 第 2 条）');
  assert.deepEqual(menus.map(m => m.order), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11],
    'order 应从 1 连到 11 不留洞：界面上的菜单顺序就是它');
  for (let i = 0; i < cats.length; i++) {
    const src = cats[i];
    const built = menus[i];
    assert.equal(built.id, src.id, `第 ${i + 1} 块应为 ${src.id}，现在是 ${built.id}（菜单顺序与原型的先后不一致）`);
    for (const key of ['id', 'name', 'order', 'items']) assert.ok(built[key] !== undefined, `${src.id} 的 module.js 缺字段 ${key}`);
    assert.equal(built.name, src.name, `${src.id} 的菜单名跟原型不一样：原型 :${src.line}`);
    assert.deepEqual(built.items, src.items, `${src.id} 的条目跟原型 :${src.line} 那一段不一样（域名顺序也算不一样）`);
  }
});

test('每块条数与第 6.1 那张表一致：8/6/6/5/6/6/5/4/2/3/3', async () => {
  const { modules } = await readMenuFolders();
  assert.deepEqual(collectMenuModules(modules).map(m => m.items.length), [8, 6, 6, 5, 6, 6, 5, 4, 2, 3, 3]);
});

test('默认勾选 8 项、初态 6 applied + 2 stale + 1 error + 45 unset（F2 完成判据那句）', async () => {
  const cats = await readPrototypeCategories();
  const all = cats.flatMap(c => c.items);
  const tally = {};
  for (const it of all) tally[it.init] = (tally[it.init] || 0) + 1;
  assert.deepEqual(tally, { applied: 6, stale: 2, error: 1, unset: 45 },
    '初态分布漂了：原型 createItem :1245 那句"applied 或 stale 才算勾上"决定了默认勾选数');
  assert.equal(all.filter(i => i.init === 'applied' || i.init === 'stale').length, 8, '默认勾选应为 8 项');
  for (const it of all) assert.ok(ItemMeta[it.init], `初态 ${it.init} 在 core/meta.js 的 ItemMeta 里没有，界面会显示成空白`);
});

test('撞号当场报错，不静默排序（第 8 节兜底 1）', async () => {
  const base = { id: 'a', name: 'A', order: 1, items: [{ id: 'x', name: 'X', domains: ['x.com'], init: 'unset' }] };
  const dupId = { './a/module.js': { default: base }, './b/module.js': { default: { ...base } } };
  assert.throws(() => collectMenuModules(dupId), /菜单 id 重复：a/);
  const dupOrder = {
    './a/module.js': { default: base },
    './b/module.js': { default: { ...base, id: 'b', name: 'B', items: [{ id: 'y', name: 'Y', domains: ['y.com'], init: 'unset' }] } }
  };
  assert.throws(() => collectMenuModules(dupOrder), /order 撞号/);
  assert.throws(() => collectMenuModules({}), /一个模块也没扫到/);
});

test('门禁：menu/** 里没有 document.、没有 innerHTML、没有 import views/（第 4 节那两条）', async () => {
  const files = [];
  const walk = async dir => {
    for (const e of await readdir(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) await walk(p);
      else if (/\.(js|css)$/.test(e.name)) files.push(p);
    }
  };
  await walk(MENU_DIR);
  assert.ok(files.length >= 13, `menu/ 下的 js/css 至少该有 11 个 module.js + index.js + collect.js，现在只找到 ${files.length} 个`);
  for (const p of files) {
    const src = await readFile(p, 'utf8');
    const rel = path.relative(ROOT, p);
    assert.doesNotMatch(src, /\bdocument\./, `${rel} 里出现了 document.：菜单文件夹只交代"这块有什么"，画界面归 views/menu/ 那一份`);
    assert.doesNotMatch(src, /\binnerHTML\b/, `${rel} 里出现了 innerHTML：同上`);
    assert.doesNotMatch(src, /from\s+['"][^'"]*views\//, `${rel} 里 import 了 views/：依赖不许从菜单倒流向界面（第 4 节那条箭头）`);
  }
});

test('menu/index.js 的扫描模式跟目录形状一致（清单由扫描生成，人不写）', async () => {
  const src = await readFile(path.join(MENU_DIR, 'index.js'), 'utf8');
  assert.match(src, /import\.meta\.glob\('\.\/\*\/module\.js'/,
    '扫描模式必须正好是 ./*/module.js —— 一个菜单文件夹一个 module.js，改了模式上面那些断言就要跟着改');
  assert.doesNotMatch(src, /['"]\.\/github\/module\.js['"]/, 'index.js 里不许出现某个菜单的手写路径');
});

/* 外框 id 清单。干什么用：界面只认 index.html 留的那几个壳（byId 拿不到就当场抛），
   这条用例不等运行就把两个方向一起算出来——外壳少一个、外框多留一个没人认的壳，当场都报得出名字和位置。
   会错在哪：① 只查一个方向是上一轮的实情：那时有六个 id（modeSelect、pillDot、pillLabel、trayDots、
                trayMenu、trayPill）没有 JS 认它们，因为顶栏／横幅／工具栏／状态栏／侧栏脚／托盘胶囊这六块
                还挂在待办上。这一轮把六块建齐、外框里写死的中文全部搬空，两边正好一样多，
                所以这一条改成双向——单靠数量下限挡不住"多留一个死壳"；
             ② 两个方向各自怎么坏：外框改了名而 JS 没跟着改 → 运行时 byId 抛「null」，
                那一行之后的 main.js 全不执行，界面看着像空壳；JS 认了一个外框没有的 id → 同样炸；
                外框留了个没人认的壳 → 一处死 DOM，半年后没人敢删，因为它看着像还有用；
             ③ 两道数量下限不是摆设：两把正则同时漂成空集（改外框形状 + 改 byId 写法）才会让相等成立，
                下限就是挡这一种的；实测两边都是 13 个，门槛照第 8 节那条"少一半也算红"取 6。
   怎么回来：加一块壳 → 同时在 main.js 给它一个 owning view（byId 一次）；删一块壳 → 两边一起删。
             红消息会说清是"外框少了这个壳"还是"这个壳没人认"。 */
test('门禁：外框 id 与 JS 点名的 id 双向对齐（多一个少一个都红）', async () => {
  const html = await readFile(path.join(FRONTEND, 'index.html'), 'utf8');
  const declared = new Set([...html.matchAll(/\bid="([A-Za-z][\w-]*)"/g)].map(m => m[1]));

  const files = [path.join(FRONTEND, 'main.js')];
  const walk = async dir => {
    for (const e of await readdir(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) await walk(p);
      else if (e.name.endsWith('.js')) files.push(p);
    }
  };
  await walk(path.join(FRONTEND, 'views'));

  const wanted = new Map();
  for (const p of files) {
    const src = await readFile(p, 'utf8');
    src.split('\n').forEach((line, idx) => {
      for (const m of line.matchAll(/\b(?:byId|getElementById)\('([A-Za-z][\w-]*)'\)/g)) {
        // byId 自己的定义那一行不算引用（它的参数是变量，正则也只抓带引号的字面量）
        if (!wanted.has(m[1])) wanted.set(m[1], `${path.relative(ROOT, p)}:${idx + 1}`);
      }
    });
  }
  assert.ok(declared.size >= 6 && wanted.size >= 6,
    `外框只认出 ${declared.size} 个 id、JS 只认出 ${wanted.size} 个，多半是这两把正则之一漂了：两条都空时"双向对齐"会假绿，空转比不跑更坏`);

  const noShell = [...wanted].filter(([id]) => !declared.has(id)).map(([id, at]) => `${at} 认的 #${id}`);
  assert.deepEqual(noShell, [],
    `这些位置点名的 id 在 index.html 里不存在：byId 会在运行时抛错，而那一行之后的 main.js 全不执行（界面看着像空壳）`);

  const noOwner = [...declared].filter(id => !wanted.has(id)).map(id => `#${id}`);
  assert.deepEqual(noOwner, [],
    `这些壳在 index.html 里留着却没有一处 JS 认它：多出来的位置就是一块没人画的界面，或一个已经搬进 views 却忘了删的外框残留`);
});

/* 外框里不许刻文案。干什么用：index.html 的 body 只剩容器，界面正文一个字都不在这儿——
   文案的家只有 core/i18n/locales/zh-CN.js 那一份（总纲 C7）。
   会错在哪：① 外框里写一句中文，切语言时它跟着页面冻在那儿，语言文件里同一句改十遍也不动；
                这跟件层、视图层那两条"不许刻死文案"是同一件事，只是这一份源在 HTML 里，
                那两条尺（剥注释后抓 JS 字符串字面量）看不见它，所以要单独一条；
             ② 注释里写中文不算违例——注释不进界面。所以先剥掉 HTML 注释再查，
                剥的时候用非贪婪，两处注释之间的正文得留着；
             ③ <head> 里的 <title> 与 lang 也不查：一个窗口标题由壳画、一个给浏览器看初始语言，
                两个都不是界面正文（外框页头第 ⑥ 条写的就是这个），所以只截 body 那一段。
   怎么回来：把这句中文搬进 locales/zh-CN.js，view 里用 t(key) 取。
             红消息给出行号与那一段文字，照上面对号入座。 */
test('门禁：index.html 的 body 里不许有中文（注释除外）', async () => {
  const html = await readFile(path.join(FRONTEND, 'index.html'), 'utf8');
  const open = html.indexOf('<body');
  const close = html.lastIndexOf('</body>');
  assert.ok(open >= 0 && close > open, 'index.html 里找不到 body 那一段：这条门禁会空转');
  const body = html
    .slice(open, close + 1)
    .replace(/<!--[\s\S]*?-->/g, m => m.replace(/[^\n]/g, ' '));   // 注释挖成等长空白，行号不动
  const lineOffset = html.slice(0, open).split('\n').length - 1;    // body 前面有几行，报出来的行号要接着数下去
  const hits = [];
  body.split('\n').forEach((line, idx) => {
    if (CJK.test(line)) hits.push(`${idx + lineOffset + 1}：${line.trim().slice(0, 60)}`);
  });
  assert.deepEqual(hits, [],
    `body 的第 ${hits.map(h => h.split('：')[0]).join('、')} 行还有中文：界面正文写在外框里就有两份源（外框一份、语言文件一份），`
    + `切语言时改一处漏一处。逐行内容：${hits.join(' ｜ ')}`);
});

/* 视图不许自己找外框。干什么用：views 那几块只接 main.js 递进来的容器（root/pill/menu），
   外框 id 的名单只写在 main.js 一处。
   会错在哪：① view 里写一句 getElementById，它就从"接容器的组件"变成"依赖某个具体页面形状的东西"：
                同一块界面换一处摆（侧栏里、弹窗里、以后多一个窗口）就选不到节点，
                而且外框 id 从此散在两处，改名时只会改到一处；
             ② 单测也就没法只造一个 div 就把这块 build 出来——本轮那些"build 一次"的用例全靠递容器才跑得起来；
             ③ 尺子只咬"往文档上找"这三下（getElementById、document.querySelector、document.querySelectorAll），
                不咬 item.querySelector('.badge-dot') 这种在自己造出来的子树里挑行——那是件自己的事，
                本来就该它自己找（三处实测都属这一类：SettingsView、QueueView、TreeView）。
   怎么回来：view 的入参加一个 root（或 pill/menu 这种多容器），main.js 里 byId 一次递进去。 */
test('门禁：views 里不许出现 getElementById 与 document 级查找（外框 id 的家只有 main.js）', async () => {
  const dir = path.join(FRONTEND, 'views');
  const files = [];
  const walk = async d => {
    for (const e of await readdir(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) await walk(p);
      else if (e.name.endsWith('.js')) files.push(p);
    }
  };
  await walk(dir);
  assert.ok(files.length >= 26, `views 只扫到 ${files.length} 个 .js，多半是目录漂了：这条门禁空转比不跑更坏`);
  const hits = [];
  for (const p of files) {
    const src = await readFile(p, 'utf8');
    src.split('\n').forEach((line, idx) => {
      if (/\bgetElementById\s*\(|\bdocument\s*\.\s*querySelector(All)?\s*\(/.test(line)) {
        hits.push(`${path.relative(ROOT, p)}:${idx + 1}`);
      }
    });
  }
  assert.deepEqual(hits, [], `这些地方在视图里往文档上找节点：${hits.join('、')}——外框容器要由 main.js 递进来`);
});

/* 注释里写出「星号紧挨斜杠」这一串，会当场把块注释截断——本轮真炸过两次，其中一次是这个文件自己。
   干什么用：扫遍前端源码（tests 与产物除外），引号外面出现这两种形状就红：
             ① 两个星号加一个斜杠（目录通配的写法）；
             ② 一个星号加一个斜杠，且同一行它后面还有汉字——说明注释在这一行中间就断了，后半句掉进代码里。
   会错在哪：块注释认得的结束符就是「星号斜杠」，出现第一个就收。于是 dom.js 那句
             「views 通配 与 parts 通配 出现 innerHTML 即红」的后半段变成代码：
             JS 里浏览器抛「parts is not defined」，整页一个字不画；
             CSS 里 esbuild 只留一条不显眼的警告，把中文当选择器接着解析。
             两种都不拦编译，注释里肉眼看不出漏了什么。
   怎么回来：注释里改说人话（「views 目录下的 parts 子目录」），别在注释里贴通配原文；
             构建工具的 glob 参数写在字符串字面量里，那是真参数，不算。 */
test('门禁：注释里不许出现「星号斜杠」这类形状——它会提前结束块注释', async () => {
  const SKIP = new Set(['node_modules', 'dist', 'tests']);
  const files = [];
  const walk = async dir => {
    for (const e of await readdir(dir, { withFileTypes: true })) {
      if (e.isDirectory()) { if (!SKIP.has(e.name)) await walk(path.join(dir, e.name)); continue; }
      if (/\.(js|css)$/.test(e.name)) files.push(path.join(dir, e.name));
    }
  };
  await walk(FRONTEND);
  assert.ok(files.length >= 80, `前端源码只扫到 ${files.length} 个 .js/.css，多半是扫描目录或跳过名单漂了`);
  const starSlash = '*'.concat('/');          // 本文件讲的就是这两个字符，把它们拼出来才不会被自己的尺子截断
  const hits = [];
  for (const p of files) {
    const lines = (await readFile(p, 'utf8')).split('\n');
    lines.forEach((line, idx) => {
      const outsideQuotes = at => (line.slice(0, at).match(/['"`]/g) || []).length % 2 === 0;
      for (let at = line.indexOf('**'.concat('/')); at >= 0; at = line.indexOf('**'.concat('/'), at + 1)) {
        if (outsideQuotes(at)) hits.push(`${path.relative(ROOT, p)}:${idx + 1}（星号星号斜杠）`);
      }
      for (let at = line.indexOf(starSlash); at >= 0; at = line.indexOf(starSlash, at + 1)) {
        if (outsideQuotes(at) && /[一-龥]/.test(line.slice(at + starSlash.length))) {
          hits.push(`${path.relative(ROOT, p)}:${idx + 1}（注释断了，后面还跟着汉字）`);
        }
      }
    });
  }
  assert.deepEqual(hits, [], `这些地方会让块注释在行中间断掉、后半句变成代码：${hits.join('、')}`);
});

