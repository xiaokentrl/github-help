/* 界面逐屏读数的长期守档。干什么用：`node --test frontend/tests/*.test.mjs` 里多一条这路——
     拿 tests/baseline/screens.json 那包浏览器实测读数重算一遍判据（两块画面逐键比画法、
     六套主题逐档、十一项交互的算术、逐屏计数的自洽），并先比对"读的是哪一版源码＋哪一版原型"的指纹。
   会错在哪：① 读数是浏览器量的一次性快照，界面改一行样式它不会自己变——所以先比指纹，
               不一致就直接红并说"重测"，不拿旧读数装成新的；原型真身也算进指纹，它动了同样红；
             ② 数被人手改（把两边不等的那格改成相等）也骗不过：对比度拿同一条 fg/bg 现算，
               把手步长、全选两下、搜索变少与找回、面板数、逐屏计数全是当场重算的；
             ③ 与原型比用的是同一次运行、同一个视口、同一套主题，两边不等就是画法不等；
               豁免名单只有量具那一份（screens.probe.js 里的 STATE_ONLY 与 COUNT_STATE_ONLY），
               这边 import 现取，不抄第二份——写了却没压住任何一次差异，同样红。
             ④ 这一包的读数来自 Chrome/Electron 内核，不是这台机器上那层 webkit2GTK 壳：
               它守的是"排版与颜色跟原型一致"，不代表真宿主 GUI 已经逐屏视觉走查过（那一趟归 #52）。
   怎么回来：按 tests/views/screens.probe.js 文件头那三步重量一次（9222 + 9226 + 9223），再
             `node tests/lib/pin.mjs --kind screens /tmp/gac-screens-raw.json` 重钉。
             名单与判据本身不在这里改。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { FRONTEND } from './lib/roster.mjs';
import { sourceFingerprint, verify } from './lib/screens.mjs';

const BASELINE = path.join(FRONTEND, 'tests', 'baseline', 'screens.json');
const probe = await import('./views/screens.probe.js');

let baseline = null;
try {
  baseline = JSON.parse(await readFile(BASELINE, 'utf8'));
} catch {
  console.error('没有 tests/baseline/screens.json：界面逐屏还没量过第一份，按 tests/views/screens.probe.js 文件头那三步量一趟再 pin。');
}

test('第一份界面逐屏读数在，且确实是那一包', async () => {
  assert.ok(baseline, '缺 tests/baseline/screens.json');
  assert.equal(baseline.meta.kind, 'screens');
  assert.equal(baseline.meta.schema, 1, '基线格式版本号与守档这条对不上：改过结构就要一起改这里');
  assert.equal(baseline.meta.host, '浏览器直开（没有壳、没有真服务）',
    '这一包不是"浏览器直开"那趟量的：换了起点（连上真服务或进了壳），所有状态相关读数都要重量重钉');
  assert.ok(/^[0-9a-f]{64}$/.test(baseline.meta.sources.hash),
    '基线里没有源码指纹：那是 pin.mjs 那一步没跑，读数就成了无主快照');
  assert.equal(baseline.meta.sources.prototypeBytes,
    (await readFile(path.resolve(FRONTEND, '..', '前端原型设计稿唯一真实来源.html'))).length,
    '原型真身的大小跟钉档那一刻不一样了（指纹那条还会告诉你内容动没动）');
});

test('量这些读数时读的源码与原型，跟现在磁盘上是同一版', async () => {
  const now = await sourceFingerprint();
  assert.equal(now.files, baseline.meta.sources.files,
    `指纹覆盖的文件数从 ${baseline.meta.sources.files} 变成 ${now.files}：界面层多出/少了一个文件，旧读数不再成套`);
  assert.equal(now.hash, baseline.meta.sources.hash,
    `源码或原型动了（${baseline.meta.sources.hash.slice(0, 12)}… → ${now.hash.slice(0, 12)}…），旧读数不能接着当判据用：` +
    '按 tests/views/screens.probe.js 文件头那三步重量一次，再 node tests/lib/pin.mjs --kind screens 重钉');
});

test('两份画面量的锚点是同一批，一个都没漏量', () => {
  assert.ok(baseline, '缺基线');
  const boxIds = probe.BOX_ANCHORS.map(a => a[0]);
  const textIds = probe.TEXT_ANCHORS.map(a => a[0]);
  assert.deepEqual(baseline.meta.锚点.盒子, boxIds, '盒子锚点名单跟量具那份不是同一批');
  assert.deepEqual(baseline.meta.锚点.文字, textIds, '文字锚点名单跟量具那份不是同一批');
  for (const id of boxIds) {
    assert.ok(baseline.boxes[id], `我们这边没量到盒子「${id}」`);
    assert.ok(baseline.prototype.boxes[id], `原型那份没量到盒子「${id}」`);
  }
  for (const id of textIds) {
    assert.ok(baseline.texts[id], `我们这边没量到文字「${id}」`);
    assert.ok(baseline.prototype.texts[id], `原型那份没量到文字「${id}」`);
  }
  assert.equal(baseline.prototype.sweep.length, probe.THEMES.length, '原型那份主题扫描套数不够');
  for (const s of baseline.themeSweep) assert.equal(s.读数.length, textIds.length, `${s.theme}：这一趟只量了 ${s.读数.length} 格`);
});

test('钉下来的读数按同一套判据重算，一条不合格都没有', () => {
  assert.ok(baseline, '缺基线');
  const bad = verify(baseline, probe);
  assert.deepEqual(bad, [], bad.join('\n'));
});

test('豁免名单每条都写明了"这一格差的是状态不是画法"，且没有第二条抄本', () => {
  assert.ok(baseline, '缺基线');
  for (const [id, entry] of Object.entries(probe.STATE_ONLY)) {
    assert.ok(Array.isArray(entry.免) && entry.免.length > 0, `${id}：豁免没点名免哪几样，等于整格都不比`);
    assert.ok(typeof entry.reason === 'string' && entry.reason.length >= 12, `${id}：豁免没写理由，读的人不知道为什么能免`);
  }
  for (const [k, entry] of Object.entries(probe.COUNT_STATE_ONLY)) {
    assert.ok(typeof entry.reason === 'string' && entry.reason.length >= 12, `计数 ${k}：豁免没写理由`);
  }
  assert.deepEqual(baseline.meta.只比画法跳过的, { 锚点: probe.STATE_ONLY, 计数: probe.COUNT_STATE_ONLY },
    '钉档那一刻的豁免名单跟量具现在那份对不上：改了豁免要先重量重钉');
});
