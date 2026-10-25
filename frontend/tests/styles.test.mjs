/* F3 这一步（拆样式）的机器判据。干什么用：一条命令验六件事——主题文件夹与原型 THEMES 一一对应、
   每套的变量值跟原型逐字一样、每套都给齐四支契约变量、组件层引的变量都在、硬色值扫描为空、撞号与错名当场报错。
   会错在哪：这六件事原本只写在文档里。第三件漏一支，新主题的某处就白压白；
   第四件漂一个字母，那一处颜色会悄悄退回继承值，界面看着"差一点"但没人知道差在哪。
   怎么回来：红的那条断言直接说出是哪一套、哪一支、跟原型第几行不一样。原型那份文件是对照物，两边一漂就红。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { collectThemes } from '../styles/themes/collect.js';

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const ROOT = path.resolve(FRONTEND, '..');
const PROTOTYPE = path.join(ROOT, '前端原型设计稿唯一真实来源.html');
const STYLES = path.join(FRONTEND, 'styles');
const THEMES = path.join(STYLES, 'themes');

/** 每套主题必须各给一遍的契约变量（组件库规划.md 第 6 节第 4 条：缺支就红，不搞"每套必须覆盖 :root 全部 21 支"那种第一天就红的假规矩）。 */
const CONTRACT = ['--on-primary', '--on-green', '--focus-ring', '--focus-glow'];
/** 这些位置在晨光白是硬写的一个值、在五套深色各是指针，所以每套深色必须逐条给到（原型那 33 条点名覆盖的等价物）。 */
const SEMANTIC = ['--chip-bg', '--mute-bg', '--head-bg', '--foot-bg', '--track-bg', '--row-current-hov',
  '--cand-line', '--q-line', '--line-soft-a', '--line-retry',
  '--line-soft-w', '--line-detect', '--line-soft-d', '--rail', '--cb-line', '--set-line', '--info-fg', '--error-fg',
  '--warn-fg', '--hl-bg', '--hl-fg', '--toast-fg', '--toast-line', '--toast-edge', '--sb-thumb', '--sb-thumb-hov',
  '--skel-a', '--skel-b', '--pop-lg', '--mask-settings', '--toast-bg'];
/** 晨光白不重复 tokens.css 的值（同一个数抄两处就会分叉），所以它只被要求给契约变量。 */
const LIGHT_ONLY = 'light';

const stripComments = css => css.replace(/\/\*[\s\S]*?\*\//g, '');

/** 把一份 CSS 拆成 { 变量名: 值 }；值里的空白归一，好跟原型逐字比。 */
function declsOf(css) {
  const out = new Map();
  for (const body of stripComments(css).match(/\{[^{}]*\}/g) || []) {
    for (const part of body.slice(1, -1).split(';')) {
      const p = part.trim();
      if (!p) continue;
      const i = p.indexOf(':');
      assert.notEqual(i, -1, `这条声明没有冒号，解析不了：${p}`);
      const name = p.slice(0, i).trim();
      if (!name.startsWith('--')) continue;                       // color-scheme 这类真属性不参与变量对账
      const value = p.slice(i + 1).replace(/\s+/g, ' ').trim();
      assert.ok(!out.has(name), `同一份文件里 ${name} 写了两遍：后一条会悄悄盖掉前一条，没人看得出来`);
      out.set(name, value);
    }
  }
  return out;
}

async function readCss(file) {
  return declsOf(await readFile(file, 'utf8'));
}

/** 现场读原型：THEMES 那六行 + 每套主题的两段变量块（合起来才算全量）。 */
async function readPrototypeThemes() {
  const lines = (await readFile(PROTOTYPE, 'utf8')).split('\n');
  const start = lines.findIndex(l => l.startsWith('const THEMES'));
  assert.notEqual(start, -1, '原型里找不到 THEMES，这份对照就失效了');
  const registry = [];
  for (let i = start; i < start + 12; i++) {
    const m = (lines[i] || '').match(/^\s*\{ id: '([a-z]+)', name: '([^']*)', desc: '([^']*)', colors: \[(.*)\] \}/);
    if (m) {
      registry.push({
        id: m[1], name: m[2], desc: m[3],
        colors: m[4].split(',').map(c => c.trim().replace(/^'|'$/g, '')),
        order: registry.length + 1
      });
    }
    if ((lines[i] || '').trim() === '];') break;
  }
  assert.equal(registry.length, 6, `原型 THEMES 应读出 6 套，现在 ${registry.length} 套：那一行的形状变了，解析要跟着改`);

  const blocks = {};
  lines.forEach((ln, i) => {
    const m = ln.match(/^  html\[data-theme="([a-z]+)"\]\{(.*)$/);
    if (!m) return;
    let body = m[2];
    let j = i;
    while (!body.includes('}')) { j++; body += '\n' + lines[j]; }
    body = body.slice(0, body.lastIndexOf('}'));
    const one = declsOf('x{' + body.replace(/^(color-scheme:[^;]*;)?/, '') + '}');
    for (const [k, v] of one) {
      assert.ok(!(blocks[m[1]] && blocks[m[1]].has(k)), `原型里 ${m[1]} 的 ${k} 出现在两段中：这里得先弄清哪一段才算数`);
    }
    blocks[m[1]] = new Map([...(blocks[m[1]] || []), ...one]);
  });
  return { registry, blocks };
}

async function themeFolders() {
  return (await readdir(THEMES, { withFileTypes: true })).filter(e => e.isDirectory()).map(e => e.name).sort();
}

test('主题文件夹与原型 THEMES 一一对应，一套不多一套不少（第 8 节兜底 3 第 4 条）', async () => {
  const { registry } = await readPrototypeThemes();
  const dirs = await themeFolders();
  assert.deepEqual(dirs, registry.map(t => t.id).sort(), '文件夹名＝主题 id：加一套主题＝加一个文件夹');
  assert.equal(dirs.length, 6, '扫出来的主题应为 6');
  for (const name of dirs) {
    const files = (await readdir(path.join(THEMES, name))).sort();
    assert.deepEqual(files, ['README.md', 'theme.css', 'theme.js'],
      `themes/${name}/ 里的文件形状应是 theme.css + theme.js + README.md 这三样，现在是 ${files.join(', ')}`);
  }
});

test('扫出来的六套与原型 THEMES 逐字一致（名称、简介、预览四色、先后）', async () => {
  const { registry } = await readPrototypeThemes();
  const scanned = {};
  for (const name of await themeFolders()) {
    scanned[`./${name}/theme.js`] = await import(pathToFileURL(path.join(THEMES, name, 'theme.js')).href);
  }
  const themes = collectThemes(scanned);
  assert.deepEqual(themes.map(t => t.order), [1, 2, 3, 4, 5, 6], 'order 应从 1 连到 6：六个圆点的先后就是它');
  themes.forEach((t, i) => {
    const src = registry[i];
    assert.equal(t.id, src.id, `第 ${i + 1} 套应为 ${src.id}，现在是 ${t.id}（跟原型数组的先后不一致）`);
    assert.equal(t.name, src.name, `${src.id} 的中文名跟原型不一样`);
    assert.equal(t.desc, src.desc, `${src.id} 的简介跟原型不一样`);
    assert.deepEqual(t.colors, src.colors, `${src.id} 的预览四色跟原型不一样：主题弹窗那张卡会画错颜色`);
  });
});

test('每套深色的变量值与原型那两段逐字一致（一段都不能漏）', async () => {
  const { blocks } = await readPrototypeThemes();
  for (const [tid, proto] of Object.entries(blocks)) {
    if (tid === LIGHT_ONLY) {
      assert.equal(proto.size, 0, `原型里晨光白不该有 html[data-theme="light"]{…} 那块（它就是 :root 的默认值），现在有 ${proto.size} 支`);
      continue;
    }
    const mine = await readCss(path.join(THEMES, tid, 'theme.css'));
    for (const [name, value] of proto) {
      if (CONTRACT.includes(name)) continue;              // 契约变量单独在下一条用例里验
      assert.equal(mine.get(name), value,
        `themes/${tid}/theme.css 的 ${name} 是 ${mine.get(name)}，原型 :468-526 那两段给的是 ${value}`);
    }
  }
});

test('契约变量每套都给齐四支；深色另给齐 31 支语义令牌（缺支就红，第 6 节第 4 条）', async () => {
  const root = await readCss(path.join(STYLES, 'tokens.css'));
  for (const name of CONTRACT) assert.ok(root.has(name), `tokens.css 的 :root 得有 ${name}，否则组件层引它就是未定义`);
  for (const tid of await themeFolders()) {
    const mine = await readCss(path.join(THEMES, tid, 'theme.css'));
    for (const name of CONTRACT) assert.ok(mine.has(name), `themes/${tid}/theme.css 缺契约变量 ${name}：这套主题一来那处就退回默认值`);
    if (tid === LIGHT_ONLY) continue;
    for (const name of SEMANTIC) assert.ok(mine.has(name), `themes/${tid}/theme.css 缺语义令牌 ${name}：这一处得靠点名覆盖才能变色，正是第 7 节要禁的事`);
  }
});

test('var(--x) 引用的变量必须有定义（未定义即红；引错了会悄悄退回继承值）', async () => {
  const root = await readCss(path.join(STYLES, 'tokens.css'));
  const cssFiles = [];
  const walk = async dir => {
    for (const e of await readdir(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) await walk(p);
      else if (e.name.endsWith('.css')) cssFiles.push(p);
    }
  };
  await walk(STYLES);
  for (const comp of ['components', 'views', 'menu']) {
    const d = path.join(FRONTEND, comp);
    try { await walk(d); } catch { /* 那一层还没建（F4/F5），建起来自动进这个检查 */ }
  }
  for (const file of cssFiles) {
    const src = stripComments(await readFile(file, 'utf8'));
    const rel = path.relative(ROOT, file);
    const own = declsOf(src);
    const inTheme = rel.includes('styles/themes/');
    for (const [, name] of src.matchAll(/var\((--[a-z0-9-]+)/g)) {
      if (root.has(name) || (inTheme && own.has(name))) continue;
      assert.fail(`${rel} 引了 var(${name})，但它既不在 tokens.css 也不在本套主题里——取出来是空，界面会悄悄用错颜色`);
    }
  }
});

test('硬色值扫描：tokens.css 与 themes/ 之外不许出现写死的颜色（第 7 节那条铁律）', async () => {
  const files = [];
  const walk = async dir => {
    for (const e of await readdir(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (path.basename(p) === 'themes') continue;      // 主题层就是放值的地方
        await walk(p);
      } else if (e.name.endsWith('.css') && e.name !== 'tokens.css') files.push(p);
    }
  };
  for (const d of [STYLES, path.join(FRONTEND, 'components'), path.join(FRONTEND, 'views'), path.join(FRONTEND, 'menu')]) {
    try { await walk(d); } catch { /* 未建的层跳过，建起来自动进这个检查 */ }
  }
  assert.ok(files.length >= 1, `至少该扫到 base.css，现在只找到 ${files.length} 份：这一层是不是没建`);
  for (const file of files) {
    const src = stripComments(await readFile(file, 'utf8'));
    const rel = path.relative(ROOT, file);
    const hit = src.match(/#[0-9a-fA-F]{3,8}\b|\brgba?\(/);
    assert.equal(hit, null, `${rel} 里写死了颜色${hit ? '「' + hit[0] + '」' : ''}：换主题时它不会跟着变，就得再加一条点名覆盖`);
  }
});

test('id 与文件夹名不一致、order 撞号、预览色少一支，全部当场报错', async () => {
  const ok = { id: 'a', name: 'A', desc: 'd', order: 1, colors: ['#fff', '#fff', '#3b82f6', '#16a34a'] };
  assert.throws(() => collectThemes({ './b/theme.js': { default: ok } }), /id「a」和文件夹名「b」不一致/);
  assert.throws(() => collectThemes({
    './a/theme.js': { default: ok }, './c/theme.js': { default: { ...ok, id: 'c', order: 1 } }
  }), /order 撞号/);
  assert.throws(() => collectThemes({ './a/theme.js': { default: { ...ok, colors: ['#fff', '#fff', '#3b82f6'] } } }), /四支/);
  assert.throws(() => collectThemes({ './a/theme.js': { default: { ...ok, colors: ['#fff', '#fff', 'blue', '#16a34a'] } } }), /不是十六进制/);
  assert.throws(() => collectThemes({}), /也没扫到/);
});

test('themes/index.js 的扫描模式跟目录形状一致（清单由扫描生成，人不写）', async () => {
  const src = await readFile(path.join(THEMES, 'index.js'), 'utf8');
  assert.match(src, /import\.meta\.glob\('\.\/\*\/theme\.js'/, '扫描模式必须正好是 ./*/theme.js');
  assert.doesNotMatch(src, /['"]\.\/light\/theme\.js['"]/, 'index.js 里不许出现某套主题的手写路径');
});

test('六套的先后不许写死在某处：只有 collect.js 按 order 排（防"第二个清单"长出来）', async () => {
  const src = await readFile(path.join(THEMES, 'collect.js'), 'utf8');
  assert.match(src, /sort\(\(a, b\) => a\.order - b\.order\)/, '排序只认 order');
  for (const tid of ['light', 'sakura', 'forest', 'ocean', 'midnight', 'oled'])
    assert.doesNotMatch(src, new RegExp(`['"]${['light', 'sakura', 'forest', 'ocean', 'midnight', 'oled'].join('|')}['"]`),
      `collect.js 里出现了主题名的字面量（${tid}）：清单本该由扫描生成，写死六套就退回手写名单`);
});
