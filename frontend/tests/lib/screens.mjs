/* 界面逐屏读数的两份算法：一份算"读的是哪一版源码 + 哪一版原型"，一份把钉下来的读数当成判据重算一遍。
   干什么用：件层那份 matrix.mjs 管"一个件单独摆出来长什么样"，这一份管"整界面画起来长什么样"——
             十二块外框的盒子、十一处有字的锚点、六套主题、十一项交互、逐屏计数，
             再把仓库根那份原型 HTML 当对照物并排量的那一份读数一起守。
   会错在哪：① 指纹漏一个目录＝那个目录改样式而基线不红，读数早就漂了没人重测；
             ② 与原型比的是"画法"，两边状态天生不同（我们这边没连真服务、原型那份带演示数据），
                所以豁免名单只有一份：tests/views/screens.probe.js 里那两张表（STATE_ONLY 与 COUNT_STATE_ONLY），
                这边 import 现取，绝不抄第二份。名单是双向的：
                  该免的没免＝把状态差报成画法差，红；
                  免了却根本没派上用场（两边那一格其实一模一样）＝给将来的真漂移预留后门，同样红；
             ③ 浏览器给的布尔与对比度一律不信，拿同一条记录里的数在 node 现算一遍（把手每步八像素、
                全选两下推到满与空、搜索变少与找回、七格面板数、以及每条 fg 压在 bg 上的对比度）；
                算出来跟记的不一样＝这数被人编过，或者两边算式不再是同一套；
             ④ 计数那几格（勾选点＝域名行+类目块、展开的类目＝类目块……）是画面自己的算术，
                当场重算，不照抄读数。
   怎么回来：见 tests/views/screens.probe.js 文件头那三步（9222 页面 + 9226 端原型 + 9223 收数），
             量完 `node tests/lib/pin.mjs --kind screens /tmp/gac-screens-raw.json` 重钉。
   注：钉下来的读数来自 Chrome/Electron 内核，不是这台机器上那层 webkit2GTK 壳——真宿主 GUI 仍未逐屏视觉走查。 */

import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { FRONTEND } from './roster.mjs';
import { contrastOf } from './matrix.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
export const PROTO_NAME = '前端原型设计稿唯一真实来源.html';
const ROOT = path.resolve(HERE, '..', '..', '..');   // 仓库根：原型 HTML 住在这里，不在 frontend/ 里

/** 读数依赖的文件：画面本身（views/styles/components/core/utils/menu + 外框 html）
    ＋ 两份量具与它们共用的算式 ＋ 端原型那个只读服务 ＋ 原型真身。少一个就是"改了就白测"。 */
async function sourceFiles() {
  const out = [];
  const walk = async rel => {
    for (const e of await readdir(path.join(FRONTEND, rel), { withFileTypes: true })) {
      if (e.name === 'node_modules' || e.name === 'dist') continue;
      const p = path.join(rel, e.name);
      if (e.isDirectory()) await walk(p);
      else if (/\.(css|js|html|json)$/.test(e.name)) out.push(p);
    }
  };
  for (const dir of ['views', 'styles', 'components', 'core', 'utils', 'menu']) await walk(dir);
  for (const f of ['index.html', 'main.js',
    path.join('tests', 'views', 'screens.probe.js'),
    path.join('tests', 'components', 'state-matrix.probe.js'),
    path.join('tests', 'lib', 'readings.js'),
    path.join('tests', 'lib', 'matrix.mjs'),
    path.join('tests', 'lib', 'proto-server.mjs')]) out.push(f);
  return out.sort();
}

/** 源码指纹：把这些文件按名字排好、连内容一起摘要；原型真身单独摘要在后面。
    只当"变没变"用——原型改一笔（它是画法的权威），同样必须重量重钉。 */
export async function sourceFingerprint() {
  const files = await sourceFiles();
  assert.ok(files.length >= 120, `界面指纹只覆盖到 ${files.length} 个文件，比预想的少：是不是 frontend/ 的扫法改了`);
  const h = createHash('sha256');
  for (const rel of files) h.update(rel).update('\0').update(await readFile(path.join(FRONTEND, rel))).update('\0');
  const proto = await readFile(path.join(ROOT, PROTO_NAME));
  h.update('PROTO\0').update(proto).update('\0');
  return { hash: h.digest('hex'), files: files.length + 1, prototypeBytes: proto.length };
}

const ok = (cond, msg, bad) => { if (!cond) bad.push(msg); };

/** 一条读数摊平成"点号键"，豁免名单才好按名字点；这些键与量具里那两份读数对象的字段一一对应 */
function flatten(rec, prefix = '') {
  const out = {};
  for (const [k, v] of Object.entries(rec)) {
    if (k === 'id' || k === 'sel' || k === 'skip') continue;
    if (v && typeof v === 'object' && !Array.isArray(v)) Object.assign(out, flatten(v, `${prefix}${k}.`));
    else out[`${prefix}${k}`] = v;
  }
  return out;
}

/** 'box' 这种整块豁免摊成它底下的每一个点号键（名单里写 box 就等于写了 box.x/y/w/h 四条） */
function matchKeys(entry, keys) {
  return keys.filter(k => k === entry || k.startsWith(`${entry}.`));
}

/** 两边逐键比画法；豁免按名字点，且必须真的压住过一次"两边确实不一样"——压不住就是后门。
    这一格在哪些场合比（静态一次、六套主题各一次）键不完全一样，所以"名单点了个没这个键的场合"不当错报，
    统一由最后那条"整批读数里从没压住过任何一次差异"来拦：既拦写了却根本不存在的键，也拦结清了还留着的条目。 */
function comparePair(where, id, app, proto, 免, bad, used) {
  const a = flatten(app), p = flatten(proto);
  const keys = [...new Set([...Object.keys(a), ...Object.keys(p)])];
  const skip = new Map();
  for (const entry of 免 || []) for (const k of matchKeys(entry, keys)) skip.set(k, entry);
  for (const k of keys) {
    if (skip.has(k)) {
      if (JSON.stringify(a[k]) !== JSON.stringify(p[k])) used.add(`${id}·${skip.get(k)}`);
      continue;
    }
    if (!(k in a) || !(k in p)) { ok(false, `${where}·${k}：一边有另一边没有（${k in a ? '缺原型侧' : '缺我们侧'}）`, bad); continue; }
    ok(JSON.stringify(a[k]) === JSON.stringify(p[k]),
      `${where}·${k}：我们这边 ${JSON.stringify(a[k])}，原型那份 ${JSON.stringify(p[k])}——豁免名单没点它，就得逐字相等`, bad);
  }
}

/** 主题扫描里两边逐档比：只比字色／底色／对比度这三样，用的还是同一张豁免名单。
    两边套数不齐时不硬按序号往下走（套数不够那一条上面已经报了，这里越界只会把"缺档"报成一个看不懂的崩）。 */
function compareSweep(appSweep, protoSweep, 免, bad, used) {
  for (let i = 0; i < appSweep.length; i++) {
    const a = appSweep[i], p = protoSweep[i];
    if (!p) { ok(false, `主题扫描：原型那份只量到第 ${i} 档，我们这边有 ${appSweep.length} 档（${a.theme} 这一档原型没量）`, bad); continue; }
    ok(a.theme === p.theme, `主题扫描第 ${i + 1} 档两边不是同一套（${a.theme} / ${p.theme}）——两边必须同一顺序同一套`, bad);
    ok(a.底色 === p.底色, `${a.theme}：整页底色我们这边 ${a.底色}、原型那份 ${p.底色}`, bad);
    ok(a.落在这一套 === true && p.落在这一套 === true, `${a.theme}：有一侧没落到这一套（我们 ${a.落在这一套} / 原型 ${p.落在这一套}）`, bad);
    for (let j = 0; j < a.读数.length; j++) {
      const x = a.读数[j], y = p.读数[j];
      if (!y) { ok(false, `${a.theme}：原型那份这一档只量到 ${p.读数.length} 格，我们这边 ${a.读数.length} 格（缺第 ${j + 1} 格）`, bad); break; }
      ok(x.id === y.id, `${a.theme}：第 ${j + 1} 格两边不是同一个锚点（${x.id} / ${y.id}）`, bad);
      if (x.缺 === true || y.缺 === true) continue;   // 缺格由"锚点必须两边都量到"那条单独拦，这里不重复报
      comparePair(`${a.theme} / ${x.id}`, x.id, x, y, 免[x.id] && 免[x.id].免, bad, used);
    }
  }
}

/** 计数那几格是画面自己的算术：不照抄读数，当场重算 */
function checkCounts(where, c, bad) {
  ok(c.认不出折叠态的类目 === 0, `${where}：有 ${c.认不出折叠态的类目} 个类目既没有 aria-expanded 也没有 .caret.collapsed，折叠态认不出来`, bad);
  ok(c.展开的类目 === c.类目块, `${where}：摊到全开之后"展开的类目"=${c.展开的类目}，类目块一共 ${c.类目块}——没摊开就量，后面那些数全是折叠态的`, bad);
  ok(c.看得见的域名行 === c.域名行, `${where}：看得见的域名行 ${c.看得见的域名行} ≠ 域名行 ${c.域名行}（还有折叠的行没画出来）`, bad);
  ok(c.勾选点 === c.域名行 + c.类目块, `${where}：勾选点 ${c.勾选点} ≠ 域名行+类目块 ${c.域名行 + c.类目块}（每一行该各有一颗勾）`, bad);
}

/** 交互那十一步里，浏览器报回来的布尔一律拿同一条记录里的数重算一遍 */
function checkInteractions(list, counts, themes, bad) {
  const by = id => list.find(x => x.id === id);
  const want = ['折叠展开', '全选两下', '键盘走树', '空格改勾选', '搜索命中', '两道把手', '气泡', '确认框', '七个面板', '胶囊与六套', '切回原套'];
  for (const id of want) ok(by(id), `交互清单少了「${id}」这一步（这十一步是逐屏走一遍的账，缺一步就等于没走）`, bad);

  const fold = by('折叠展开');
  if (fold) {
    ok(fold.看得见行数.之前 === counts.看得见的域名行,
      `折叠展开：点之前看得见 ${fold.看得见行数.之前} 行，开场计数里是 ${counts.看得见的域名行}——两边不在同一起点，这一步的账不作数`, bad);
    ok(fold.整批翻面 === true && (fold.翻面方向 === '摊到全折' || fold.翻面方向 === '折到全摊'),
      `折叠展开：整批翻面=${fold.整批翻面} 方向=${fold.翻面方向}`, bad);
    ok(fold.点之前那颗钮写着 !== fold.点之后写着,
      `折叠展开：那颗钮点完还是写着「${fold.点之后写着}」——按钮的字该跟着状态翻`, bad);
  }

  const sel = by('全选两下');
  if (sel) {
    const 满空 = v => v === sel.总共 || v === 0;
    ok(满空(sel.第一下), `全选两下：第一下到 ${sel.第一下}，既不等于总数 ${sel.总共} 也不是 0`, bad);
    ok(sel.第一下推到满或空 === 满空(sel.第一下), `全选两下：记的布尔（${sel.第一下推到满或空}）跟重算的不是一个答案`, bad);
    const 另一头 = (sel.第一下 === sel.总共 && sel.第二下 === 0) || (sel.第一下 === 0 && sel.第二下 === sel.总共);
    ok(sel.第二下推到另一头 === 另一头 && 另一头, `全选两下：第二下 ${sel.第二下} 没推到第一下的另一头（第一下 ${sel.第一下}／总共 ${sel.总共}）`, bad);
    ok(sel.总共 === counts.域名行, `全选两下：这一步数到的总共 ${sel.总共} ≠ 计数里的域名行 ${counts.域名行}`, bad);
    ok(sel.那颗钮跟着翻了字 === true, `全选两下：那颗钮两次都写着「${sel.字 && sel.字[0]}」`, bad);
  }

  const kbd = by('键盘走树');
  if (kbd) {
    ok(kbd.落点唯一 === true, `键盘走树：按完方向键树里的 Tab 停点不止一个（roving tabindex 漏了）`, bad);
    ok(kbd.换行成功 === true, `键盘走树：方向键没换行（起点 ${JSON.stringify(kbd.起点)} 下一步 ${JSON.stringify(kbd.下一步)}）`, bad);
    ok(kbd.Tab停点 && kbd.Tab停点.在树里 > 0, '键盘走树：这一趟没量到 Tab 停点数', bad);
  }

  const space = by('空格改勾选');
  if (space) ok(space.翻了 === true && space.焦点在那颗勾上 === true, `空格改勾选：焦点在勾上=${space && space.焦点在那颗勾上}，${space && space.之前}→${space && space.之后} 没翻`, bad);

  const search = by('搜索命中');
  if (search) {
    ok(search.全部 === counts.看得见的域名行, `搜索命中：起点 ${search.全部} ≠ 计数里看得见的域名行 ${counts.看得见的域名行}`, bad);
    ok(search.变少了 === (search.命中 < search.全部) && search.变少了 === true, `搜索命中：「${search.关键字}」量到 ${search.命中} 行，全部 ${search.全部}`, bad);
    ok(search.全部找回 === (search.清空后回到 === search.全部) && search.全部找回 === true, `搜索命中：清空后回到 ${search.清空后回到}，起点是 ${search.全部}`, bad);
  }

  const drag = by('两道把手');
  if (drag) {
    const s = drag.侧栏, q = drag.队列;
    ok(s.回得来 === (s.退回 === s.原) && s.回得来 === true, `纵向账：侧栏宽度 ${s.原}→${s.三步后}→${s.退回} 回不了原位`, bad);
    ok(s.每步八像素 === (Math.round((s.三步后 - s.原) * 100) === 2400), `侧栏把手：三步量到 ${(s.三步后 - s.原).toFixed(2)}px，不是 3×8`, bad);
    ok(q.一步八像素 === (Math.round((q.一步后 - q.原) * 100) === 800), `队列把手：一步量到 ${(q.一步后 - q.原).toFixed(2)}px，不是 8`, bad);
    ok(q.回得来 === (q.退回 === q.原) && q.回得来 === true, `队列高度 ${q.原}→${q.一步后}→${q.退回} 回不了原位`, bad);
    for (const [k, v] of Object.entries(drag.热区 || {})) ok(v && v.w > 0 && v.h > 0, `把手热区 ${k}=${JSON.stringify(v)}：量到 0，说明这一趟没画出来`, bad);
  }

  const toast = by('气泡');
  if (toast) ok(toast.出现 === true && toast.有字 === true, `气泡：出现=${toast && toast.出现} 有字=${toast && toast.有字}（那句话：${toast && toast.那句话}）`, bad);

  const dlg = by('确认框');
  if (dlg) {
    for (const k of ['打开', '焦点进去了', '关掉后焦点回到点它的人', '遮罩拆干净']) {
      ok(dlg[k] === true, `确认框：${k}=${dlg && dlg[k]}（人性化清单第 2、3 条要的全是 true）`, bad);
    }
  }

  const panes = by('七个面板');
  if (panes) {
    ok(panes.面板数 === panes.明细.length, `七个面板：记了 ${panes.面板数} 格，明细里 ${panes.明细.length} 格`, bad);
    for (const p of panes.明细) ok(p.打开着 === true && p.控件数 > 0 && p.行数 > 0 && p.最小热区 > 0,
      `设置面板「${p.面板}」：打开着=${p.打开着} 控件 ${p.控件数} 行 ${p.行数} 最小热区 ${p.最小热区}`, bad);
  }

  const pill = by('胶囊与六套');
  if (pill) {
    ok(pill.圆点数 === themes.length, `胶囊与六套：菜单里 ${pill.圆点数} 颗圆点，主题名单是 ${themes.length} 套`, bad);
    ok(JSON.stringify([...pill.名单].sort()) === JSON.stringify([...themes].sort()),
      `胶囊与六套：圆点名单 ${pill.名单.join('/')} 跟主题名单 ${themes.join('/')} 不是同一批`, bad);
    ok(pill.换面成功 === true, `胶囊与六套：点「${pill.那颗圆点写着}」之后落到 ${by('切回原套') ? '' : ''}${pill.点下去落到}`, bad);
  }
  const back = by('切回原套');
  if (back) ok(back.回到原套 === true, `切回原套：最后停在 ${back.落到}，没摆回开场那一套（下一次比对的起点就脏了）`, bad);
}

/** 每条记了 fg/bg/contrast 的记录，node 拿同一条现算一遍（算式用 matrix.mjs 那一份，不抄第二条） */
function checkContrast(where, rec, bad) {
  if (rec.contrast === undefined) return;
  if (rec.contrast === null) { ok(!rec.fg || !rec.bg, `${where}：三样都齐了却记成 null`, bad); return; }
  const again = contrastOf(rec.fg, rec.bg);
  ok(again !== null, `${where}：记了对比度却没记字色或底色（${rec.fg} / ${rec.bg}）`, bad);
  ok(again === rec.contrast, `${where}：记录 contrast=${rec.contrast}，用同一条 ${rec.fg} 压在 ${rec.bg} 上重算是 ${again}——这数被人编过，或者两边公式不再是同一套`, bad);
}

/** 全部判据走一遍，返回失败清单（空数组＝这份基线站得住） */
export function verify(baseline, probe) {
  const bad = [];
  assert.equal(baseline.meta.kind, 'screens', '这份基线不是界面逐屏读数（meta.kind 对不上）');
  const app = baseline, proto = baseline.prototype;

  // 名单对账：量的那批锚点必须跟量具里那份名单一字不差（加一格就要重测，删一格不许偷偷删读数）
  const boxIds = probe.BOX_ANCHORS.map(a => a[0]);
  const textIds = probe.TEXT_ANCHORS.map(a => a[0]);
  assert.deepEqual(baseline.meta.锚点.盒子, boxIds, '量具的盒子锚点名单与钉下来的不是同一批');
  assert.deepEqual(baseline.meta.锚点.文字, textIds, '量具的文字锚点名单与钉下来的不是同一批');
  assert.deepEqual(baseline.meta.只比画法跳过的, { 锚点: probe.STATE_ONLY, 计数: probe.COUNT_STATE_ONLY },
    '钉档那一刻的豁免名单跟量具现在那份对不上：改了豁免要先重量重钉');
  for (const id of Object.keys(probe.STATE_ONLY)) {
    ok(boxIds.includes(id) || textIds.includes(id), `豁免名单里的「${id}」不在任何一张锚点名单里：这条是给谁留的？`, bad);
  }
  for (const id of boxIds) { ok(app.boxes[id], `我们这边没量到盒子锚点「${id}」`, bad); ok(proto.boxes[id], `原型那份没量到盒子锚点「${id}」`, bad); }
  for (const id of textIds) { ok(app.texts[id], `我们这边没量到文字锚点「${id}」`, bad); ok(proto.texts[id], `原型那份没量到文字锚点「${id}」`, bad); }

  // 两边必须同一窗口同一套起点，否则比出来的差是窗口差
  ok(proto.viewport.w === app.meta.viewport.w && proto.viewport.h === app.meta.viewport.h,
    `视口不一致：我们 ${app.meta.viewport.w}×${app.meta.viewport.h}，原型那一份 ${proto.viewport.w}×${proto.viewport.h}——两份不在同一大小的窗口里，比出来的差没意义`, bad);
  ok(app.meta.展开后还剩几个折叠 === 0, `我们这边展开到最后还剩 ${app.meta.展开后还剩几个折叠} 个折叠的类目（-1＝20 轮还没摊平）：树没摊到全开，行高那批数是折叠态的`, bad);
  ok(proto.展开后还剩几个折叠 === 0, `原型那边展开到最后还剩 ${proto.展开后还剩几个折叠} 个折叠的类目：两份不在同一起点`, bad);
  checkCounts('我们这边', app.counts, bad);
  checkCounts('原型那边', proto.counts, bad);

  // 画法比对：盒子、文字两批 + 六套主题逐档，全用同一张豁免名单
  const used = new Set();
  for (const id of boxIds) comparePair(`盒子「${id}」`, id, app.boxes[id], proto.boxes[id], probe.STATE_ONLY[id] && probe.STATE_ONLY[id].免, bad, used);
  for (const id of textIds) comparePair(`文字「${id}」`, id, app.texts[id], proto.texts[id], probe.STATE_ONLY[id] && probe.STATE_ONLY[id].免, bad, used);
  ok(app.themeSweep.length === probe.THEMES.length, `主题扫描 ${app.themeSweep.length} 档，主题名单 ${probe.THEMES.length} 套：有主题没量`, bad);
  ok(proto.sweep.length === probe.THEMES.length, `原型那份主题扫描 ${proto.sweep.length} 档，主题名单 ${probe.THEMES.length} 套`, bad);
  compareSweep(app.themeSweep, proto.sweep, probe.STATE_ONLY, bad, used);
  for (const [id, entry] of Object.entries(probe.STATE_ONLY)) {
    for (const 项 of entry.免) {
      ok(used.has(`${id}·${项}`),
        `豁免名单里「${id}」免了「${项}」，可这一项在这次读数里两边一模一样：状态差结清了就把这条删掉，别留给将来`, bad);
    }
  }

  // 计数比对：除名单点名的那几格（由演示数据决定），其余每一格两边必须逐字相等
  for (const k of Object.keys(app.counts)) {
    if (probe.COUNT_STATE_ONLY[k]) {
      ok(app.counts[k] !== proto.counts[k],
        `计数豁免「${k}」两边其实一样（都是 ${app.counts[k]}）：这笔状态差结清了，把名单里那条删掉`, bad);
      continue;
    }
    ok(app.counts[k] === proto.counts[k], `计数「${k}」：我们这边 ${app.counts[k]}，原型那份 ${proto.counts[k]}——豁免名单没点它，就得相等`, bad);
  }
  for (const k of Object.keys(probe.COUNT_STATE_ONLY)) ok(Object.keys(app.counts).includes(k), `计数豁免名单点了「${k}」，读数里没有这一格`, bad);

  checkInteractions(app.interactions, app.counts, probe.THEMES, bad);
  for (const id of textIds) {
    checkContrast(`我们这边 文字「${id}」`, app.texts[id], bad);
    checkContrast(`原型那边 文字「${id}」`, proto.texts[id], bad);
  }
  for (const s of [...app.themeSweep, ...proto.sweep]) {
    for (const r of s.读数) if (!r.缺) checkContrast(`${s.theme} / ${r.id}`, r, bad);
  }
  ok(app.finalTheme === app.meta.theme, `量完停在 ${app.finalTheme}，开场是 ${app.meta.theme}：界面没摆回起点，下一次重量对不上`, bad);
  return bad;
}
