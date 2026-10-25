/* 「该建哪些件目录」这份名单的唯一读法。干什么用：现场从 组件库规划.md 的 8.1 那张表 + 第 5 节分子件表
   把目录名单读出来，给 tests/components.test.mjs（对账目录）和 tests/baseline.test.mjs（对账状态矩阵的行数）共用。
   会错在哪：这份解析原来只写在 components.test.mjs 里；状态矩阵那份也要用同一份名单，
             复制一遍就成了两份判据——将来表形改了，改一处漏一处，漏的那处会"绿着失效"（读空了也不报）。
   怎么回来：读不出表、或读出来的数低于下面那两道下界，当场 assert 说清是哪一节被改名或挪走；
             两处用例都从这一个文件取名单，改表形只需改这里。 */

import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
export const ROOT = path.resolve(FRONTEND, '..');
export const PLAN = path.join(ROOT, '组件库规划.md');
export const COMPONENTS = path.join(FRONTEND, 'components');

/** 目录名 → 那个件的两个文件名主干：kebab 目录 ⇒ Pascal 入口 / kebab 样式（8.2 命名法） */
export function namesOf(dir) {
  const js = dir.split('-').map(s => s[0].toUpperCase() + s.slice(1)).join('');
  return { js: `${js}.js`, css: `${dir}.css` };
}

/** ① 8.1 那张原子件表（划了删除线的 tooltip 算预留、不建目录）
    ② 第 5 节分子件表里"放哪"那一栏写着 components/ 的那一个（就是算式末尾加进去的 setting-row）。
    两处都由表生成，件名一个都不在这里硬写。 */
export async function readRoster() {
  const lines = (await readFile(PLAN, 'utf8')).split('\n');
  const start = lines.findIndex(l => l.startsWith('### 8.1 '));
  assert.notEqual(start, -1, '组件库规划.md 里找不到 8.1 那一节，这份对照就失效了：去看它是不是被改名或挪走');
  const rest = lines.slice(start);
  const end = rest.findIndex((l, i) => i > 0 && l.startsWith('**算式写死在这'));
  assert.notEqual(end, -1, '8.1 那一节后面找不到"算式写死在这"那句，读不出表格的结尾');
  const dirs = [];
  const reserved = [];
  for (const ln of rest.slice(0, end)) {
    const cell = ln.match(/^\|\s*4\.\d+\s+[^|]+\|\s*([^|]+)\|/);
    if (!cell) continue;
    const m = cell[1].match(/`([a-z0-9-]+)\/`/);
    if (!m) continue;
    (cell[1].includes('~~') ? reserved : dirs).push(m[1]);
  }
  assert.ok(dirs.length >= 16 && reserved.length >= 1,
    `从 8.1 那张表只读出 ${dirs.length} 个件目录、${reserved.length} 个预留件——表形一改解析就要跟着改，别让它悄悄读空`);

  // 第 5 节：整份文档只有那一格写着「放哪＝components/…」，读不到就说明表形变了（宁可红，不要悄悄少算一个）
  const molStart = lines.findIndex(l => l.startsWith('## 5. '));
  assert.notEqual(molStart, -1, '找不到第 5 节"分子件"，读不出那个唯一进库的分子件目录');
  const molEnd = molStart + lines.slice(molStart + 1).findIndex(l => /^## /.test(l));
  const molecular = [];
  for (const ln of lines.slice(molStart, molEnd)) {
    for (const m of ln.matchAll(/`components\/([a-z0-9-]+)\/`/g)) molecular.push(m[1]);
  }
  assert.ok(molecular.length >= 1, '第 5 节那张表里读不出"放哪＝components/…"的行：算式末项和实际目录集会对不上');

  return { dirs: [...dirs, ...molecular].sort(), reserved: reserved.sort() };
}

/** 磁盘上真实存在的件目录 */
export async function listComponentDirs() {
  const entries = await readdir(COMPONENTS, { withFileTypes: true });
  return entries.filter(e => e.isDirectory()).map(e => e.name).sort();
}
