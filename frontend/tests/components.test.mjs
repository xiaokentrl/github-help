/* 公共组件这一层的机器判据。干什么用：一条命令（`node --test frontend/tests/*.test.mjs`；传目录会 MODULE_NOT_FOUND）验五件事——
   `components/` 下的目录跟 组件库规划.md 8.1 那张唯一名单对不对得上（逐名对账，多一个少一个都红）、
   每个件目录是不是正好三个文件且命名合 8.2、件里有没有刻死的中文（8.3）、
   件有没有反着 import 视图层或菜单（8.5 第 6 条）、件里有没有用 innerHTML 整块塞（8.5 第 5 条的同一把尺）。
   会错在哪：这五条原本只写在文档里等人看。名单漂了（tooltip 真建起来却没人划掉"预留"）、
             有人图快在件里刻一句中文、把 views/ 反向引了进来——这三种病都是攒到第三个件才爆，届时没人敢删。
   怎么回来：红的时候断言消息会说是哪个件、哪一行、为什么。
             名单不是硬写的数，是现场从那篇文档的表里读出来的：改了那张表，这里的判据自动跟着走。
             裸色值与"var() 引了没定义的变量"那两条不在本文件重复一遍——已由 tests/styles.test.mjs 全站扫，
             两处各扫一遍迟早打架（8.1 末那条讲的正是这件事）。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { cjkLiterals } from './lib/literals.mjs';

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const ROOT = path.resolve(FRONTEND, '..');
const PLAN = path.join(ROOT, '组件库规划.md');
const COMPONENTS = path.join(FRONTEND, 'components');

import { namesOf, readRoster, listComponentDirs as listDirs } from './lib/roster.mjs';

// 名单那份解析搬去 tests/lib/roster.mjs 了——状态矩阵守档那条（tests/baseline.test.mjs）要用同一份，
// 在这儿再抄一遍就是两份判据，将来 8.1 那张表改了形，抄的那份会绿着失效。

async function filesOf(ext) {
  const out = [];
  for (const dir of await listDirs()) {
    for (const f of await readdir(path.join(COMPONENTS, dir))) {
      if (f.endsWith(ext)) out.push(path.join(COMPONENTS, dir, f));
    }
  }
  return out.sort();
}

/* 抓"源码里的字符串字面量"那把尺住在 tests/lib/literals.mjs：件层与视图层量的是同一件事
   （界面文案不许刻死在代码里），两处各抄一份迟早漂。 */

test('components/ 的目录集合与 8.1 那张唯一名单逐名对齐（预留件不许偷偷建）', async () => {
  const { dirs, reserved } = await readRoster();
  const found = await listDirs();
  assert.deepEqual(found, dirs,
    '名单只有一份，就是 组件库规划.md 8.1 那张表：多一个目录＝没登记就建，少一个目录＝登记了没建');
  for (const r of reserved) {
    assert.equal(found.includes(r), false,
      `components/${r}/ 建起来了，可它在 8.1 里还划着"预留"：要么把删除线划掉再建，要么把目录先删了`);
  }
});

test('每个件目录正好三件齐（入口 + 样式 + README），README 那三行一条不缺', async () => {
  for (const dir of await listDirs()) {
    const { js, css } = namesOf(dir);
    const files = (await readdir(path.join(COMPONENTS, dir))).sort();
    assert.deepEqual(files, ['README.md', css, js].sort(),
      `components/${dir}/ 里的文件不是 8.2 说的那三件（测试文件归 frontend/tests/components/，不进件目录）`);

    const readme = await readFile(path.join(COMPONENTS, dir, 'README.md'), 'utf8');
    for (const h of ['这个件管什么', '有哪些状态靠谁给', '谁在用']) {
      assert.ok(readme.includes(h), `components/${dir}/README.md 缺「${h}」这一行（第 8 节兜底 2 那条：不许有只放说明没有代码的壳，也不许有只有代码没说归谁的件）`);
    }
  }
});

test('门禁 8.5 第 2 条：件里不许刻死中文——文字一律由调用方传进来', async () => {
  const jsFiles = await filesOf('.js');
  assert.ok(jsFiles.length >= 17, `components/ 下只扫到 ${jsFiles.length} 个 .js，跟件数对不上：是不是有件没建入口`);
  for (const p of jsFiles) {
    const rel = path.relative(ROOT, p);
    // cjkLiterals 已经把"给开发者看的那几处"（throw new Error、collect 那三个件的 bad()、console）剔掉了
    for (const { body, line } of cjkLiterals(await readFile(p, 'utf8'))) {
      assert.fail(
        `${rel}:${line} 里刻了一句中文「${body.slice(0, 40)}」：件把界面文案写死，"新增一种语言＝只新增一个语言文件"当场作废（8.3 与总纲 C7 是同一件事的两头）`);
    }
  }
});

test('门禁 8.5 第 6 条：件不许反着 import 视图层或菜单（依赖箭头只许 视图→件）', async () => {
  for (const p of await filesOf('.js')) {
    const src = await readFile(p, 'utf8');
    const rel = path.relative(ROOT, p);
    assert.doesNotMatch(src, /from\s+['"][^'"]*views\//, `${rel} 里 import 了 views/：倒流回来就没人拆得动，第 4 节那条箭头是单向的`);
    assert.doesNotMatch(src, /from\s+['"][^'"]*menu\//, `${rel} 里 import 了 menu/：公共件不认识菜单，认识菜单的是行级件`);
    assert.doesNotMatch(src, /from\s+['"][^'"]*styles\//, `${rel} 里 import 了 styles/：样式按第 5 节那固定顺序由构建入口串，件各引各的会重复打进去`);
  }
});

test('件不用 innerHTML 整块塞（8.5 第 5 条同一把尺，先从件这一层守住）', async () => {
  for (const p of await filesOf('.js')) {
    const src = await readFile(p, 'utf8');
    assert.doesNotMatch(src, /\binnerHTML\b/,
      `${path.relative(ROOT, p)} 里出现 innerHTML：整块塞会把上一步的焦点冲掉，也会给"刻中文"留后门（8.4 那条）`);
  }
});

test('件的守卫都在：不认识的状态名/档位名/上下限，当场抛错而不是悄悄退回默认（件级用例的地基）', async () => {
  const load = async (file) => import(pathToFileURL(path.join(COMPONENTS, file)).href);
  // 这几个构造函数第一件事就是查参数，查在动 DOM 之前——所以在 node 里也能跑，不需要浏览器
  const { Tag } = await load('tag/Tag.js');
  assert.throws(() => Tag({ text: 'x', tone: 'detct' }), /不认识 tone/);
  const { Skeleton } = await load('skeleton/Skeleton.js');
  const { ProgressBar } = await load('progress-bar/ProgressBar.js');
  assert.throws(() => ProgressBar({ kind: 'globl' }), /不认识 kind/);
  assert.throws(() => ProgressBar({ kind: 'score', text: 'x' }), /容不下一行字/);
  const { DragHandle } = await load('drag-handle/DragHandle.js');
  assert.throws(() => DragHandle({ axis: 'z', min: 10, max: 20 }), /不认识轴/);
  assert.throws(() => DragHandle({ axis: 'x', min: 300, max: 200 }), /上下限给反了/);
  const { EmptyState } = await load('empty-state/EmptyState.js');
  const { ErrorBox } = await load('error-box/ErrorBox.js');
  const { SettingRow } = await load('setting-row/SettingRow.js');
  // 漏写 action 要抛（键必须在），显式给 null 是合法的：详情／队列／树那三处空态本来就没有钮
  assert.throws(() => EmptyState({ title: 'x' }), /必须写明有没有下一步/);
  assert.throws(() => ErrorBox({ text: 'x' }), /必须写明有没有下一步/);
  assert.throws(() => SettingRow({ title: 'x' }), /需要 control/);
  void Skeleton;
});
