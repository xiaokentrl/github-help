/* 把浏览器量到的一包读数钉进 tests/baseline/。干什么用：读数只有配上"量的是哪一版源码"才算账——
   这一步在 node 里算源码指纹、跟判据名单对账，然后当场拿那一套判据跑一遍，跑得过才落盘。
   两档读数、两条路：
     默认（件层）  tests/components/state-matrix.probe.js → tests/baseline/state-matrix.json，判据在 matrix.mjs
     --kind screens（界面层） tests/views/screens.probe.js → tests/baseline/screens.json，判据在 screens.mjs
   会错在哪：① 验不过也照样写，等于把一份本来就红的基线钉成"标准答案"，之后每次 CI 都在守一个错的东西；
             ② 对比度那份账（tests/baseline/contrast-debt.mjs）本脚本从不写它——新出现一档弱对比就必须人来登记一次，
                否则"重钉一次"就把闸门松掉了；界面层这一份同样不给它留第二条及格线：
                颜色与对比度的权威就是原型那一份加上件层那份名单，这边只记数、不比线；
             ③ 手改过基线里的数再跑本脚本会照样被指纹拦下（源码没动、数却不同＝有人在编）。
   怎么回来：件层 `cd frontend && npm run dev`（9222 端口）→ 页面里
             `await import('/tests/components/state-matrix.probe.js').then(m => m.run())`，
             POST 给 `node tests/lib/sink.mjs /tmp/gac-matrix-raw.json`，再 `node tests/lib/pin.mjs /tmp/gac-matrix-raw.json`；
             界面层按 tests/views/screens.probe.js 文件头那三步（还要起 9226 端原型），
             再 `node tests/lib/pin.mjs --kind screens /tmp/gac-screens-raw.json`。 */

import assert from 'node:assert/strict';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { sourceFingerprint as matrixFingerprint, verify as verifyMatrix } from './matrix.mjs';
import { sourceFingerprint as screensFingerprint, verify as verifyScreens } from './screens.mjs';
import { readRoster, listComponentDirs } from './roster.mjs';
import { CONTRAST_DEBT } from '../baseline/contrast-debt.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const FRONTEND = path.resolve(HERE, '..', '..');

const argv = process.argv.slice(2);
let kind = 'state-matrix';
if (argv[0] === '--kind') {
  assert.equal(argv.length, 3, `--kind 后面要跟一个档位名和一个读数文件（现在给了 ${argv.join(' ')}）`);
  kind = argv[1];
  argv.splice(0, 2);
}
const src = argv[0];
if (!src) {
  console.error('用法：node tests/lib/pin.mjs [--kind state-matrix|screens] <浏览器 POST 落下来的那份读数文件>');
  process.exit(2);
}

const raw = JSON.parse(await readFile(src, 'utf8'));
assert.equal(raw.meta.kind, kind, `这份文件不是 ${kind} 读数（meta.kind=${raw.meta.kind}）——别拿别的探针来钉档`);

const ROUTES = {
  'state-matrix': {
    out: path.join(FRONTEND, 'tests', 'baseline', 'state-matrix.json'),
    async prepare() {
      // 名单对账：17 个件目录每一个都得有行，多出来的目录名同样拦（件删了但矩阵里还留着行＝读数会指到不存在的件）
      const { dirs } = await readRoster();
      const found = await listComponentDirs();
      assert.deepEqual(found, dirs, 'components/ 下的目录跟 8.1 那份唯一名单对不上，先修对账再钉档');
      const covered = [...new Set(raw.rows.map(r => r.comp))].sort();
      assert.deepEqual(covered, dirs,
        `状态矩阵只覆盖了 ${covered.length} 个件，名单里有 ${dirs.length} 个：没量到的那些件就等于没守——缺的是 ${dirs.filter(d => !covered.includes(d)).join(' / ')}`);
      const fp = await matrixFingerprint();
      return { fp, extra: { roster: dirs }, verify: b => verifyMatrix(b, CONTRAST_DEBT) };
    },
    said(b) {
      return `${b.rows.length} 行 / ${b.themeSweep.length} 档主题对比 / 源码 ${b.meta.sources.files} 个文件 指纹 ${b.meta.sources.hash.slice(0, 12)}…`;
    }
  },
  screens: {
    out: path.join(FRONTEND, 'tests', 'baseline', 'screens.json'),
    async prepare() {
      // 锚点名单与豁免名单都只有一份：量具那份文件在 node 里 import 进来现取（它顶层不碰 DOM，取得动）
      const probe = await import('../views/screens.probe.js');
      const fp = await screensFingerprint();
      return { fp, extra: {}, verify: b => verifyScreens(b, probe) };
    },
    said(b) {
      return `${Object.keys(b.boxes).length} 块盒子 / ${Object.keys(b.texts).length} 处文字 / ` +
        `${b.themeSweep.length} 套主题×${b.themeSweep[0].读数.length} 格对比 / 原型那一份同窗口并排量过 / ` +
        `源码与原型 ${b.meta.sources.files} 个文件 指纹 ${b.meta.sources.hash.slice(0, 12)}…`;
    }
  }
};

const route = ROUTES[kind];
assert.ok(route, `不认得这一档读数：--kind ${kind}（现在只有 state-matrix 与 screens）`);
const { fp, extra, verify } = await route.prepare();

const baseline = {
  ...raw,
  meta: {
    ...raw.meta,
    schema: 1,
    pinnedAt: new Date().toISOString().slice(0, 19) + 'Z',
    sources: { hash: fp.hash, files: fp.files, prototypeBytes: fp.prototypeBytes },
    ...extra
  }
};

const bad = verify(baseline);
if (bad.length) {
  console.error(`这份读数不合格（${bad.length} 条），没有钉档：`);
  for (const m of bad) console.error('  ✗ ' + m);
  console.error('要么修界面/样式，要么按消息去量具那份豁免名单里写明"这一格差的是状态不是画法"，然后再钉。');
  process.exit(1);
}

await mkdir(path.dirname(route.out), { recursive: true });
await writeFile(route.out, JSON.stringify(baseline, null, 1) + '\n');
console.log(`钉档完成（${kind}）：${route.said(baseline)} → ${path.relative(FRONTEND, route.out)}`);
