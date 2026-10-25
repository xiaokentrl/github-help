/* 状态矩阵读数的两份算法：一份算"读的是哪一版源码"，一份把钉下来的读数当成判据重算一遍。
   干什么用：浏览器量的那 61 行只说明"量那一刻是这样"，件改一行样式，读数早就变了而没人知道。
             所以钉档时把源码指纹一起写进去，node 这边先比指纹、再拿记录里的数重算每条 need——
             源码一动就红，逼人重测；手改基线里的数想蒙过去也一样红（数是自己算的，不是照抄的）。
   会错在哪：① 指纹只覆盖"读数依赖的那些文件"（件、样式、量具本身），漏一个就等于那条改了就白测；
             ② 对比度那条不拿 4.5 一刀切：组件库规划.md :118 记着三笔"已按甲不动"的弱对比旧账，
                一刀切会逼人改掉主题（那是用户亲口拍过"先这样"的事）。低于 4.5 的每一档必须在
                tests/baseline/contrast-debt.mjs 里有对得上的一行，那份名单是人手登记的，多一档、少一档、更差一档都红。
   怎么回来：`cd frontend && npm run dev`（9222），页面里
             `await import('/tests/components/state-matrix.probe.js').then(m => m.run())` → POST 给 tests/lib/sink.mjs →
             `node tests/lib/pin.mjs /tmp/gac-matrix-raw.json` 重钉一次；名单变了要人自己改 contrast-debt.mjs。 */

import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { COMPONENTS, FRONTEND, PLAN, listComponentDirs, readRoster } from './roster.mjs';

export const PROBE = path.join(FRONTEND, 'tests', 'components', 'state-matrix.probe.js');

/** 对比度的及格线（组件库规划.md 第 6 节那条：正文与压在主色上的字要过 4.5） */
export const CONTRAST_BAR = 4.5;

/** 读数依赖的文件：件的两份 + 样式层 + 量具本身 + 量具共用的那份算式（tests/lib/readings.js）。漏一个＝那个改了没人重测。 */
async function sourceFiles() {
  const out = [];
  for (const dir of await listComponentDirs()) {
    for (const f of await readdir(path.join(COMPONENTS, dir))) {
      if (f.endsWith('.js') || f.endsWith('.css')) out.push(path.join('components', dir, f));
    }
  }
  const walk = async rel => {
    for (const e of await readdir(path.join(FRONTEND, rel), { withFileTypes: true })) {
      const p = path.join(rel, e.name);
      if (e.isDirectory()) { if (e.name !== 'node_modules') await walk(p); }
      else if (/\.(css|js)$/.test(e.name)) out.push(p);
    }
  };
  await walk('styles');
  out.push(path.join('tests', 'components', 'state-matrix.probe.js'));
  // 量具共用的那几算式（颜色/底色/对比度/盒子/声明值）：改它等于改所有读数，必须一起摘要
  out.push(path.join('tests', 'lib', 'readings.js'));
  return out.sort();
}

/** 源码指纹：把这些文件按名字排好、连内容一起摘要。
    只当"变没变"用，不当"变到哪一版"用——所以文件顺序、路径写法都得钉死，否则白红。 */
export async function sourceFingerprint() {
  const files = await sourceFiles();
  assert.ok(files.length >= 40, `指纹只覆盖到 ${files.length} 个文件，比预想的少：是不是 components/ 或 styles/ 的扫法改了`);
  const h = createHash('sha256');
  for (const rel of files) h.update(rel).update('\0').update(await readFile(path.join(FRONTEND, rel))).update('\0');
  return { hash: h.digest('hex'), files: files.length };
}

/** 下面这几个函数与量具里那两份必须逐字同一套公式（WCAG 相对亮度）。
    为什么 node 这边要再算一遍：基线里的 contrast 是浏览器给的数，手改那个数就能把弱对比那一档"改成合格"。
    现在拿同一条记录里的 fg/bg 现算，算出来跟记的不一样＝这数被人编过，当场红。
    两份基线（件层 state-matrix、界面层 screens）都用这一个 parseRgb/contrastOf，及格线与算式不许多抄一份。 */
function parseRgb(text) {
  if (!text) return null;
  const m = String(text).match(/rgba?\(([^)]+)\)/);
  const body = m ? m[1] : (/^\d+(\.\d+)?[,\s]+\d+(\.\d+)?[,\s]+\d+/.test(String(text)) ? String(text) : null);
  if (!body) return null;
  const p = body.split(/[,\/\s]+/).filter(Boolean).map(Number);
  return p.length >= 3 && p.slice(0, 3).every(Number.isFinite) ? p.slice(0, 3) : null;
}

function luminance(rgb) {
  const s = rgb.map(v => {
    const x = v / 255;
    return x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * s[0] + 0.7152 * s[1] + 0.0722 * s[2];
}

export function contrastOf(fg, bg) {
  const f = parseRgb(fg), b = parseRgb(bg);
  if (!f || !b) return null;
  const a = luminance(f), c = luminance(b);
  return +((Math.max(a, c) + 0.05) / (Math.min(a, c) + 0.05)).toFixed(2);
}

const ok = (cond, msg, bad) => { if (!cond) bad.push(msg); };

/** 记下来的那个对比度必须是拿同一条 fg/bg 算出来的；两边任缺一格都当成"这行没量" */
function checkRecorded(what, item, bad) {
  if (item.contrast === undefined) return;
  const again = contrastOf(item.fg, item.bg);
  ok(again !== null, `${what}：记了对比度却没记字色或底色（${item.fg} / ${item.bg}），这条没核`, bad);
  ok(again === item.contrast,
    `${what}：记录 contrast=${item.contrast}，用同一条 ${item.fg} 压在 ${item.bg} 上重算是 ${again}——这数被人改过，或者两边的公式不再是同一套`, bad);
}

/** 一行 need 该重算出什么，全在这里；need 里写了一条就必须量出一条，缺读数同样算失败
    ——"没量"跟"量出来不合格"是同一件事，不能让基线靠"这行没记数"混过去。 */
function checkRow(row, bad) {
  const need = row.need || {};
  if (need.hit) {
    ok(row.hit, `${row.id}：need.hit 要量热区，读数里却没有 hit`, bad);
    ok(row.hit.w >= need.hit[0] && row.hit.h >= need.hit[1],
      `${row.id}：热区 ${row.hit.w}×${row.hit.h} 不到 ${need.hit.join('×')}`, bad);
  }
  if (need.ring) {
    ok(row.ring, `${row.id}：need.ring 要量焦点环，读数里却没有 ring`, bad);
    ok(!(row.ring.outline === 'none' && row.ring.boxShadow === 'none'), `${row.id}：焦点既无 outline 也无 boxShadow`, bad);
    ok(row.tabbable === true, `${row.id}：键盘走不到（tabbable=${row.tabbable}）`, bad);
  }
  if (need.reason) {
    ok(row.disabledControls > 0, `${row.id}：说要量禁用，可这个件里一个可操作节点都没有`, bad);
    ok(row.isDisabled === true, `${row.id}：这行说要量禁用，但件里有 ${row.disabledControls} 个可操作节点并没全禁用（"整组灰了却还能点"就是这种）`, bad);
    ok(typeof row.disabledReason === 'string' && row.disabledReason.length > 0,
      `${row.id}：禁用了却没写"在等什么"（人性化清单第 4 条）`, bad);
  }
  if (need.dual) {
    const hasName = !!(row.dot && row.dot.name);
    ok(row.textVisible || hasName, `${row.id}：只有颜色没有第二通道（字与无障碍名两样都缺）`, bad);
  }
  if (need.anim) {
    ok(row.animation && row.animation.count > 0, `${row.id}：这行该有动画，量到 count=${row.animation && row.animation.count}`, bad);
    ok(row.reduced === 0, `${row.id}：减动效那一档开起来还剩 ${row.reduced} 条动画在跑（读数缺了＝没量这一档）`, bad);
  }
  if (need.text) {
    // 对比度的及格线不在这里再写一遍：六套主题那 222 档由 checkSweep 统一对 contrast-debt.mjs 那份账，
    // 默认主题那一条本来就在扫描里（theme=meta.theme），两处各判一次迟早打架
    ok(row.contrast !== undefined, `${row.id}：need.text 要算对比，读数里却没有 contrast`, bad);
    checkRecorded(row.id, row, bad);
    if (row.modal) ok(row.contrast >= CONTRAST_BAR,
      `${row.id}：弹窗那两行不进主题扫描，就当场按及格线判——读到 ${row.contrast}，不过线`, bad);
  }
  if (row.modal) {
    for (const k of ['autoFocusInside', 'tabCloses', 'focusReturnsToOpener', 'maskRemoved']) {
      ok(row[k] === true, `${row.id}：弹窗那条 ${k} 读数是 ${row[k]}（第 7 节第 2、3 条要求的全是 true）`, bad);
    }
  }
}

/** 六套主题那 222 档逐档对账：低于及格线的每一档都要在名单里，且不许比登记值更差，也不许有结清了还留着的条目 */
function checkSweep(baseline, debt, bad) {
  const byId = new Map(baseline.rows.map(r => [r.id, r]));
  const seen = new Set();
  // 每一档"有字的行"都要在每一套主题下量到过，缺档就等于"这条没判"——按行数×主题数当场点清
  // 弹窗那两行不在这里扫（一趟只开合一次，六套主题各开一遍留给 #30）：它们在默认主题下直接吃及格线
  const textRows = baseline.rows.filter(r => r.need && r.need.text && !r.modal);
  const want = textRows.length * baseline.meta.themes.length;
  ok(baseline.themeSweep.length >= want,
    `主题扫描只有 ${baseline.themeSweep.length} 档，${textRows.length} 行有字的 × ${baseline.meta.themes.length} 套主题该有 ${want} 档：有行没量到`, bad);
  for (const r of textRows) {
    const got = baseline.themeSweep.filter(x => x.id === r.id).map(x => x.theme);
    for (const theme of baseline.meta.themes) {
      ok(got.includes(theme), `${r.id} 在 ${theme} 这套主题下没量到对比度（缺档＝没判）`, bad);
    }
  }
  for (const s of baseline.themeSweep) {
    if (s.contrast >= CONTRAST_BAR) continue;
    const entry = debt.find(d => d.id === s.id && (d.themes === '*' || d.themes.includes(s.theme)));
    ok(!!entry, `${s.theme} / ${s.id}：${s.contrast} 低过及格线，contrast-debt.mjs 里没有这一档——要么登记它，要么这就是新出现的弱对比`, bad);
    checkRecorded(`${s.theme} / ${s.id}`, s, bad);
    if (!entry) continue;
    seen.add(entry);
    ok(s.contrast >= entry.floor,
      `${s.theme} / ${s.id}：${s.contrast} 比登记的 ${entry.floor} 更差了（要放宽得人在名单里改并写明理由）`, bad);
    ok(typeof entry.parity === 'string' && /组件库规划\.md/.test(entry.parity),
      `${s.id} 那笔账的 parity 没指向 组件库规划.md：说清是哪一节`, bad);
    ok(byId.has(s.id), `${s.id}：出现在主题扫描里却没有对应行`, bad);
  }
  for (const d of debt) {
    ok(seen.has(d), `contrast-debt.mjs 里登记的 ${d.id}（floor ${d.floor}）这次没有低于及格线的读数了：这笔账结清了，把人写的那条删掉`, bad);
  }
}

/** 气泡时长：这是件里那条纯算式，量的是"边界跨过去读数就该变"，不是 DOM */
function checkDurations(baseline, bad) {
  const d = baseline.durations;
  const at = n => d.find(x => x.chars === n);
  for (const n of [4, 24, 25, 60, 61, 120]) ok(at(n), `时长读数少了 ${n} 字这一档`, bad);
  ok(at(25).ms > at(24).ms, `25 字比 24 字时长没变长（${at(24).ms}→${at(25).ms}）：那条按字数加的算式没跨过 24 字这道界`, bad);
  ok(at(61).ms > at(60).ms, `61 字比 60 字时长没变长：算式里 60 字那道界没生效`, bad);
  ok(at(4).ms <= 2500, `短到 4 字也要 ${at(4).ms}ms，比"短句 2.2s"那档还长`, bad);
  ok(at(120).ms >= 8000, `96 字以上只给 ${at(120).ms}ms，读不完那句话（第 7 节第 10 条）`, bad);
  for (let i = 1; i < d.length; i++) ok(d[i].ms >= d[i - 1].ms, `字数越多时长反而越短（${d[i - 1].chars}→${d[i].chars}）`, bad);
}

/** 全部判据走一遍，返回失败清单（空数组＝这份基线站得住） */
export function verify(baseline, debt) {
  const bad = [];
  assert.equal(baseline.meta.kind, 'state-matrix', '这份基线不是状态矩阵读数（meta.kind 对不上）');
  ok(baseline.meta.prefersReducedMotion === false,
    '量这组读数时系统本身就开着"减弱动效"：动画那三条读成什么样都不作数，关掉系统设置重量', bad);
  for (const r of baseline.rows) checkRow(r, bad);
  checkSweep(baseline, debt, bad);
  checkDurations(baseline, bad);
  return bad;
}

export { readRoster, listComponentDirs, PLAN };
