/* 文案这一层的机器判据。干什么用：四件事——键表跟代码对得上（少了立刻红、多了也红）、
   t() 缺键时大声、语言文件的名字与形状有人管、locales/index.js 的扫描排除名单跟目录同步。
   会错在哪：① 代码里写了个新键忘了进表：界面显示成一片 ⟦gate.xxx⟧，测试不查就得等用户截图来报；
             ② 表里留了一条没人用的：翻译的人照着表翻下一门语言，白花力气还看不出错；
             ③ 参数占位符写错名（{n} 写成 {count}）：句子里原地留 {n}，肉眼看得见，但没有机器检查——
                这里只钉住「少给参数时不许悄悄换成 undefined」这一半（t() 那两条用例）。
   怎么回来：红的时候看消息里那条键。改句子＝改用户看到的原话，要连 zh-CN.js 后面的原型行号一起改。
             err.* 那一族不在这里查：它们的键名是 `err.${码}` 现场拼的，源码里扫不到字符串，
             少了哪一码、句子用了没依据的占位符，由 tests/errors.test.mjs（G13）现读 internal/errs/codes.go 对账。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { createI18n } from '../core/i18n/t.js';
import { collectLocales } from '../core/i18n/locales/collect.js';
import zhCN from '../core/i18n/locales/zh-CN.js';

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
/** 谁允许说人话：core、views，加上构建入口 main.js（外框那四块还没搬进 views 之前，工具栏与状态栏的字由它递进 t()）；
    components 与 menu 里出现键名本身就违背「件不写文案」，所以不扫它们 */
const SCAN_DIRS = ['core', 'views'];
const SCAN_FILES = ['main.js'];

/** 组名单与 locales/zh-CN.js 顶上那段注释是同一份：新增一组（比如 #29 的 settings.*）＝两处一起加。
    名单里没有 phase：那一句随 mock 退役（正在干哪一步那八句在 core/meta.js 的表里）；
    也没有 err：那一族键名现场拼接，源码扫不到，归 tests/errors.test.mjs 管。 */
const KEY_RE = /'((?:gate|flow|tree|common|detail|queue|toolbar|statusbar|topbar|layout|sidebar|shortcut|settings|theme|tray|banner|startup|update|about)\.[A-Za-z0-9._]+)'/g;

async function sourceFiles(dir, base = path.join(FRONTEND, dir), prefix = dir) {
  const out = [];
  let entries;
  try {
    entries = await readdir(base, { withFileTypes: true });
  } catch (err) {
    if (err.code === 'ENOENT') return out;      // views/ 还没建，等建起来自动进扫描
    throw err;
  }
  for (const e of entries) {
    const rel = `${prefix}/${e.name}`;
    if (e.isDirectory()) { out.push(...(await sourceFiles(dir, path.join(base, e.name), rel))); continue; }
    if (!e.isFile() || !e.name.endsWith('.js')) continue;
    if (rel.includes('i18n/locales/')) continue;   // 键表自己不算调用方
    out.push(rel);
  }
  return out;
}

/** 现场把源码扫一遍取键名，不采信任何文档里抄的清单 */
async function usedKeys() {
  const map = new Map();
  const files = [
    ...(await Promise.all(SCAN_DIRS.map(dir => sourceFiles(dir)))).flat(),
    ...SCAN_FILES
  ];
  for (const rel of files) {
    const text = await readFile(path.join(FRONTEND, rel), 'utf8');
    for (const m of text.matchAll(KEY_RE)) {
      if (!map.has(m[1])) map.set(m[1], rel);
    }
  }
  return map;
}

test('键表：代码里用到的每条键都在 zh-CN 里（缺了就红）', async () => {
  const used = await usedKeys();
  assert.ok(used.size >= 90, `只扫到 ${used.size} 条键，多半是扫描名单或键名前缀漂了，去 tests/i18n.test.mjs 的 SCAN_DIRS/KEY_RE 看`);
  const missing = [...used].filter(([key]) => typeof zhCN[key] !== 'string');
  assert.deepEqual(missing, [], `这些键在用但表里没有：${missing.map(([k, f]) => `${k}（${f}）`).join('、')}`);
});

test('键表：表里没有写着却没人用的键（翻译要按这张表花力气）', async () => {
  const used = await usedKeys();
  // err.* 那一族由 tests/errors.test.mjs 逐码查（键名是现场拼的，这里的静态扫描永远扫不到）
  const orphan = Object.keys(zhCN).filter(key => !key.startsWith('err.') && !used.has(key));
  assert.deepEqual(orphan, [], `这些键在 zh-CN.js 里写着但 core/views 没用到：${orphan.join('、')}`);
});

test('t()：正常取句并把 {n} 换成参数', () => {
  const t = createI18n({ 'zh-CN': zhCN });
  assert.equal(t('gate.busy.queue'), '队列正在执行中，请稍候');
  assert.equal(t('flow.detect.start', { n: 3 }), '开始检测 3 项…');
  assert.equal(t('flow.restore.catMixed', { name: '类目二', ok: 1, fail: 2 }), '「类目二」下 1 项成功，2 项失败');
});

test('t()：参数少给一个就让 {n} 原样留在句子里，不悄悄换成 undefined', () => {
  const t = createI18n({ 'zh-CN': zhCN });
  assert.equal(t('flow.detect.start'), '开始检测 {n} 项…');
});

test('t()：缺键时大声——控制台点名＋界面上留一对 ⟦…⟧，回落那份也没有就用回落句', () => {
  const real = console.error;
  const seen = [];
  console.error = (...a) => seen.push(a.join(' '));
  try {
    const t = createI18n({ 'zh-CN': zhCN, en: { 'gate.busy.queue': 'Queue is running' } });
    assert.equal(t('flow.nope.notHere'), '⟦flow.nope.notHere⟧');
    assert.match(seen.at(-1), /在 zh-CN 和回落 zh-CN 里都没有/);

    t.setLang('en');
    assert.equal(t('gate.busy.queue'), 'Queue is running');
    assert.equal(t('gate.busy.detecting'), '批量检测进行中，请稍候', 'en 缺这条，先用回落那句');
    assert.match(seen.at(-1), /在 en 里缺，本次先用回落 zh-CN/);
  } finally {
    console.error = real;
  }
});

test('t()：切到没有的语言当场抛错，不留"切了没反应"那种现场', () => {
  const t = createI18n({ 'zh-CN': zhCN });
  assert.throws(() => t.setLang('fr'), /locales\/ 下没有 fr\.js/);
  assert.throws(() => createI18n({ en: {} }), /回落语言 zh-CN 不在清单里/);
});

/* ================= 语言目录那一侧的形状检查 ================= */

const dict = { 'gate.busy.queue': '甲' };

test('collectLocales：文件名就是语言 id，形状不对当场抛', () => {
  assert.deepEqual(Object.keys(collectLocales({ './zh-CN.js': { default: dict } })), ['zh-CN']);
  assert.throws(() => collectLocales({ './zh_CN.js': { default: dict } }), /得是 zh-CN 这种形状/);
  assert.throws(() => collectLocales({ './简体中文.js': { default: dict } }), /得是 zh-CN 这种形状/);
  assert.throws(() => collectLocales({ './zh-CN.js': { default: { k: 1 } } }), /键 k 的值不是字符串/);
  assert.throws(() => collectLocales({ './zh-CN.js': { default: [1] } }), /没有默认导出一份键表对象/);
  assert.throws(() => collectLocales({}), /下一个语言文件也没扫到/);
});

test('collectLocales：回落那份整个漏了就抛，不能让界面无处可退', () => {
  assert.throws(() => collectLocales({ './en.js': { default: dict } }), /回落语言这份不存在/);
});

/* locales/ 目录是平铺的，扫描会把邻居的工具文件一起扫进来，所以 index.js 里有一份 `!./xxx` 排除名单。
   干什么用：钉住「名单跟目录同步」，两个方向都查。
   会错在哪：① 工具文件没进名单——启动当场抛「键 xxx 的值不是字符串」，有声，好查；
             ② 语言文件被误写进名单——那个语言在界面上整个消失：切过去没反应、也不报错，没人知道。
   怎么回来：红的时候按消息把 index.js 那行 glob 改成目录的真实形状。 */
test('locales/：不是语言文件的 .js 必须在排除名单里，是语言文件的不许被排除', async () => {
  const dir = path.join(FRONTEND, 'core/i18n/locales');
  const src = await readFile(path.join(dir, 'index.js'), 'utf8');
  const excluded = new Set([...src.matchAll(/'!\.\/([^']+)'/g)].map(m => m[1]));
  assert.ok(excluded.has('index.js'), 'index.js 没在排除名单里：glob 会把自己也扫成一种语言');
  const files = (await readdir(dir)).filter(n => n.endsWith('.js') && n !== 'index.js');
  for (const name of files) {
    const dict = (await import(pathToFileURL(path.join(dir, name)).href)).default;
    const isLang = !!dict && typeof dict === 'object' && !Array.isArray(dict)
      && Object.values(dict).every(v => typeof v === 'string');
    if (isLang) {
      assert.ok(!excluded.has(name), `${name} 是一份键表却被写进排除名单：这个语言在界面上整个消失，切过去没反应也不报错`);
    } else {
      assert.ok(excluded.has(name), `${name} 不是语言文件却没在排除名单里：启动会把它当成一种语言，collect.js 当场抛`);
    }
  }
});

test('负向：真把一条键删掉，这两条门禁得红', () => {
  const cut = { ...zhCN };
  delete cut['flow.detect.start'];
  const real = console.error;
  console.error = () => {};
  try {
    assert.equal(createI18n({ 'zh-CN': cut })('flow.detect.start'), '⟦flow.detect.start⟧');
  } finally { console.error = real; }
});
