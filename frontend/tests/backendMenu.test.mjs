/* 后端菜单数据轴的门禁（G4 三方对账 + 生成物幂等）。干什么用：一条命令验五件事——
   ① 后端那 44 个文件形状对不对（11 块 × 4）；
   ② 后端 JSON ⇄ 前端 module.js ⇄ 原型 CATEGORY_DEFS 三方逐字一样；
   ③ 后端那份里不许出现 init／modes 这类键（初态只归界面，方式收窄那条已作废〔已定：D3 甲、R6〕）；
   ④ 每块的父域清单与 items 现算的结果等值，合计 47、去重 44；
   ⑤ 重新生成一次应当一个字也不变（幂等）。
   会错在哪：改了一条域名忘了跑生成器 → 界面列 54 条、后端认 53 条，少那条点了"应用"什么也不发生；
   手改了后端 JSON → 下一次生成把它盖掉，改动看着像丢了。出错时看得见的是这条用例红，消息说清哪一块哪一条。
   怎么回来：node tools/gen-backend-menu.mjs 重算重写；不许手改 internal/menu 下的 JSON，也不许改这里的期望数。
   原型那份解析器与前端门禁共用（./lib/prototype-menu.mjs），两边各写一份迟早会"各量各的"。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { isDomain, parentDomains } from '../../tools/domain-shape.mjs';
import { readPrototypeCategories } from './lib/prototype-menu.mjs';
import { loadFrontendMenus } from './lib/menu-source.mjs';

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const ROOT = path.resolve(FRONTEND, '..');
const BACKEND_MENU = path.join(ROOT, 'internal', 'menu');
const GENERATOR = path.join(ROOT, 'tools', 'gen-backend-menu.mjs');
const CASES = path.join(ROOT, 'tools', 'domain-cases.json');

async function readJson(p) {
  return JSON.parse(await readFile(p, 'utf8'));
}

/** 现场数后端菜单目录里的文件，相对路径列表（不含 .go，那是读取包自己的） */
async function listBackendFiles() {
  const out = [];
  const walk = async (dir, prefix) => {
    for (const e of await readdir(dir, { withFileTypes: true })) {
      const rel = prefix ? `${prefix}/${e.name}` : e.name;
      if (e.isDirectory()) await walk(path.join(dir, e.name), rel);
      else if (!e.name.endsWith('.go')) out.push(rel);
    }
  };
  await walk(BACKEND_MENU, '');
  return out.sort();
}

test('后端菜单目录正好 44 个数据文件：11 块 ×（menu.json + README + common/common.json + common/README）', async () => {
  const files = await listBackendFiles();
  assert.equal(files.length, 44, `后端数据文件 ${files.length} 个，应为 11 × 4 = 44`);
  const blocks = new Set(files.map(f => f.split('/')[0]));
  assert.equal(blocks.size, 11, `后端菜单块数 ${blocks.size}，应为 11`);
  for (const f of files) {
    assert.match(f, /^([^/]+)\/(menu\.json|README\.md|common\/(common\.json|README\.md))$/,
      `${f}：菜单目录里只许这四样，条目不建目录〔已定：R9〕`);
  }
});

test('三方逐字对账：后端 JSON ⇄ 前端 module.js ⇄ 原型 CATEGORY_DEFS', async () => {
  const proto = await readPrototypeCategories();
  const front = await loadFrontendMenus();
  assert.deepEqual(front.map(m => m.id).sort(), proto.map(c => c.id).sort(),
    '块先要对上：后端读的是生成物，生成物漂了会在这里露出来');

  // 原型里没有 order 字段：它排在第几就是第几块（CATEGORY_DEFS 那一段的先后＝界面上的菜单顺序）。
  for (const [i, src] of proto.entries()) {
    const built = await readJson(path.join(BACKEND_MENU, src.id, 'menu.json'));
    const fe = front.find(m => m.id === src.id);
    assert.ok(fe, `前端没有 ${src.id} 这块，原型 :${src.line} 却有`);
    for (const key of ['id', 'name']) {
      assert.equal(built[key], src[key], `后端 ${src.id} 的 ${key} 与原型 :${src.line} 不一样`);
      assert.equal(fe[key], src[key], `前端 ${src.id} 的 ${key} 与原型 :${src.line} 不一样`);
    }
    assert.equal(built.order, i + 1, `后端 ${src.id} 排第 ${built.order}，原型里它排第 ${i + 1}（菜单顺序漂了）`);
    assert.equal(fe.order, i + 1, `前端 ${src.id} 排第 ${fe.order}，原型里它排第 ${i + 1}`);
    assert.equal(built.items.length, src.items.length, `${src.id} 条目数：后端 ${built.items.length}，原型 ${src.items.length}`);
    built.items.forEach((it, i) => {
      assert.deepEqual({ id: it.id, name: it.name, domains: it.domains },
        { id: src.items[i].id, name: src.items[i].name, domains: src.items[i].domains },
        `后端 ${src.id} 第 ${i + 1} 条与原型 :${src.line} 那一段不一样（域名顺序也算不一样）`);
      assert.deepEqual(it.domains, fe.items[i].domains,
        `${src.id} 第 ${i + 1} 条：后端域名与前端 module.js 不一致，多半是改完没跑生成器`);
    });
  }
});

test('后端那份不抄初态也没有方式白名单：键的集合必须正好是规划里那几样〔已定：D3 甲、R6〕', async () => {
  for (const f of await listBackendFiles()) {
    if (!f.endsWith('.json')) continue;
    const doc = await readJson(path.join(BACKEND_MENU, f));
    if (f.endsWith('menu.json')) {
      // 顶层四样、条目三样：多一个 init（初态归界面）或多一个 modes（上一版必填白名单，已作废）都会在这儿红。
      assert.deepEqual(Object.keys(doc).sort(), ['id', 'items', 'name', 'order'], `${f} 的顶层键集合不对`);
      for (const it of doc.items) {
        assert.deepEqual(Object.keys(it).sort(), ['domains', 'id', 'name'], `${f} 里条目 ${it.id} 的键集合不对`);
      }
    } else {
      // 公共层只有一个键：出现 items、条目 id 或域名清单，它就变成第二个家〔已定：G12〕。
      assert.deepEqual(Object.keys(doc), ['parentDomains'], `${f} 的顶层键不止 parentDomains：公共层不许当第二份条目清单`);
    }
  }
});

test('每块父域与 items 现算结果等值；合计 47、去重 44、跨块重叠三条', async () => {
  const blocks = (await readdir(BACKEND_MENU, { withFileTypes: true })).filter(e => e.isDirectory()).map(e => e.name).sort();
  assert.equal(blocks.length, 11);
  let total = 0;
  const all = new Map();
  for (const id of blocks) {
    const doc = await readJson(path.join(BACKEND_MENU, id, 'menu.json'));
    const mc = await readJson(path.join(BACKEND_MENU, id, 'common', 'common.json'));
    const want = parentDomains(doc.items.flatMap(it => it.domains));
    assert.deepEqual(mc.parentDomains, want, `${id} 的父域清单与现算不一致：这份由生成器算，手改会被下一次生成盖掉`);
    total += want.length;
    for (const p of want) all.set(p, (all.get(p) ?? 0) + 1);
  }
  assert.equal(total, 47, `父域合计 ${total}，应为 47`);
  assert.equal(all.size, 44, `父域去重后 ${all.size}，应为 44`);
  assert.deepEqual([...all].filter(([, n]) => n > 1).map(([p]) => p).sort(),
    ['ea.com', 'googleapis.com', 'ubisoft.com'], '跨块重叠的父域变了：哪块菜单管它，界面那句话要跟着改');
});

test('域名判定与后端 Go 用同一张用例表（tools/domain-cases.json 只有一份）', async () => {
  const cases = await readJson(CASES);
  for (const d of cases.valid) assert.ok(isDomain(d), `表里判合格，JS 判不合格：${d}`);
  for (const d of cases.invalid) assert.ok(!isDomain(d), `表里判不合格，JS 判合格：${d}`);
  assert.deepEqual(parentDomains(['raw.githubusercontent.com/StevenBlack/hosts']), [], '不合格形状不该有父域');
});

test('生成物幂等：跑一次生成器（--check）应当一个字也不变', () => {
  const out = execFileSync(process.execPath, [GENERATOR, '--check'], { encoding: 'utf8' });
  assert.match(out, /生成物与真源一致，一个字没改/,
    `生成物与前端真源已经漂了，先跑 node tools/gen-backend-menu.mjs 再回来。生成器说：${out}`);
});
