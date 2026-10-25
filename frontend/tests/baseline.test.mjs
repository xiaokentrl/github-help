/* 状态矩阵读数的长期守档。干什么用：`node --test frontend/tests/*.test.mjs` 里多一条——
     拿 tests/baseline/state-matrix.json 那包浏览器实测读数重算一遍判据（热区、焦点环、禁用说明、
     双通道、对比度、减动效、气泡时长、弹窗焦点回还），并先比对源码指纹。
   会错在哪：① 读数是别的进程（浏览器）量的一次性快照，件改一行样式它不会自己变——所以先比指纹，
               不一致就直接红并说"重测"，不拿旧读数装成新的；
             ② 数被人手改（把 2.9 写成 4.6）也骗不过：对比度这条同一条记录里的 fg/bg 会当场重算核一遍；
             ③ 及格线以下的那几档不是"放行"，是要在 tests/baseline/contrast-debt.mjs 里有一条人写的登记，
               多一档、少一档、比登记值更差、账结清了还留着，四种都红。
   怎么回来：按 tests/components/state-matrix.probe.js 文件头那三步重量一次，再
             `node tests/lib/pin.mjs /tmp/gac-matrix-raw.json` 重钉。名单/判据本身不在这里改。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { installDom } from './lib/dom-stub.mjs';
import { FRONTEND, readRoster, listComponentDirs } from './lib/roster.mjs';
import { sourceFingerprint, verify } from './lib/matrix.mjs';
import { CONTRAST_DEBT } from './baseline/contrast-debt.mjs';

const BASELINE = path.join(FRONTEND, 'tests', 'baseline', 'state-matrix.json');

const baseline = JSON.parse(await readFile(BASELINE, 'utf8'));

test('第一份读数基线在，且确实是状态矩阵那一包', () => {
  assert.equal(baseline.meta.kind, 'state-matrix');
  assert.equal(baseline.meta.schema, 1, '基线格式版本号与守档这条对不上：改过结构就要一起改这里');
  assert.ok(Array.isArray(baseline.rows) && baseline.rows.length >= 60,
    `基线只有 ${baseline.rows && baseline.rows.length} 行，比钉档那趟的 61 行少——这份是被删过的，重量一次重钉`);
  assert.ok(baseline.meta.sources && /^[0-9a-f]{64}$/.test(baseline.meta.sources.hash),
    '基线里没有源码指纹（或格式不对）：那是 pin.mjs 那一步没跑，读数就成了无主快照');
});

test('量这些读数时读的源码，跟现在磁盘上这一版是同一版', async () => {
  const now = await sourceFingerprint();
  assert.equal(now.files, baseline.meta.sources.files,
    `指纹覆盖的文件数从 ${baseline.meta.sources.files} 变成 ${now.files}：件或样式多出/少了一个文件，旧读数不再成套`);
  assert.equal(now.hash, baseline.meta.sources.hash,
    `源码动了（${baseline.meta.sources.hash.slice(0, 12)}… → ${now.hash.slice(0, 12)}…），旧读数不能接着当判据用：` +
    '按 tests/components/state-matrix.probe.js 文件头那三步重量一次，再 node tests/lib/pin.mjs 重钉');
});

test('矩阵覆盖面＝8.1 那份唯一名单（一个件都没漏量）', async () => {
  const { dirs } = await readRoster();
  const found = await listComponentDirs();
  assert.deepEqual(found, dirs, 'components/ 与名单对不上，先修对账再谈读数');
  assert.deepEqual(baseline.meta.roster, dirs, '钉档那天的名单跟今天的名单不是一副：件目录动过，读数要重量');
  assert.deepEqual([...new Set(baseline.rows.map(r => r.comp))].sort(), dirs,
    '有件在矩阵里没有行：那个件的状态没人量过，等于没守');
  for (const row of baseline.rows) {
    assert.ok(dirs.includes(row.comp), `${row.id} 记的件目录 ${row.comp} 不在名单里`);
  }
});

test('六套主题那一遍确实扫过，主题名单与样式层的名单同源', () => {
  assert.ok(baseline.meta.themes.length >= 6, `基线只扫了 ${baseline.meta.themes.length} 套主题`);
  assert.ok(baseline.themeSweep.length >= baseline.meta.themes.length, '主题扫描是空的：那"六套都够不够"这句话没根据');
});

test('钉下来的读数按同一套判据重算，一条不合格都没有', () => {
  const bad = verify(baseline, CONTRAST_DEBT);
  assert.deepEqual(bad, [], bad.join('\n'));
});

test('低于及格线的每一档都在人工名单里，且名单里没有白占着的豁免', () => {
  const lows = baseline.themeSweep.filter(s => s.contrast < 4.5);
  const ids = new Set(lows.map(s => s.id));
  assert.deepEqual([...ids].sort(), CONTRAST_DEBT.map(d => d.id).sort(),
    `实测低过 4.5 的是 ${[...ids].sort().join(' / ')}，人工名单写的是 ${CONTRAST_DEBT.map(d => d.id).sort().join(' / ')}：两边必须逐名对上`);
  for (const d of CONTRAST_DEBT) {
    assert.ok(typeof d.parity === 'string' && d.parity.includes('组件库规划.md'), `${d.id} 那笔账要能指着文档说出哪一节`);
    assert.ok(d.floor > 0 && d.floor < 4.5, `${d.id} 的 floor=${d.floor} 不在"低过及格线"这一段里`);
  }
});

test('气泡时长那六个数不是抄来的：件里那条算式在 node 里现算一遍，逐点相同', async () => {
  installDom();
  const { Toast } = await import(pathToFileURL(path.join(FRONTEND, 'components', 'toast', 'Toast.js')).href);
  const t = Toast({});
  for (const d of baseline.durations) {
    assert.equal(t.toastMs('x'.repeat(d.chars)), d.ms,
      `${d.chars} 字这一档：基线记 ${d.ms}ms，件里那条算式现在给的是 ${t.toastMs('x'.repeat(d.chars))}ms——要么算式改了要重钉，要么这个数被人编过`);
  }
});
