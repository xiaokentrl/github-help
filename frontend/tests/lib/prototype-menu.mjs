/* 原型那份 CATEGORY_DEFS 的现场解析器。干什么用：把「原型是菜单数据的对照物」这句话变成一个函数，
   两个测试文件（structure.test.mjs 守前端 ⇄ 原型、backendMenu.test.mjs 守后端 ⇄ 前端 ⇄ 原型）共用同一把尺子。
   会错在哪：解析器写两份，其中一份漂了就会"两边都说对但其实各量各的"——本项目最坏的那类假绿。
   怎么回来：原型里那一段的形状改了（换行、加字段、改名），这里当场红并指出第几行，改解析器而不是改断言。 */

import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const LIB_DIR = path.dirname(fileURLToPath(import.meta.url));
export const PROTOTYPE = path.resolve(LIB_DIR, '..', '..', '..', '前端原型设计稿唯一真实来源.html');

/** 现场读原型里的 CATEGORY_DEFS 那一段，不采信任何文档里抄的数 */
export async function readPrototypeCategories(prototypePath = PROTOTYPE) {
  const lines = (await readFile(prototypePath, 'utf8')).split('\n');
  const start = lines.findIndex(l => l.startsWith('const CATEGORY_DEFS'));
  assert.notEqual(start, -1, '原型里找不到 CATEGORY_DEFS，这份对照就失效了：去看它是不是被挪走或改名');
  const end = lines.findIndex((l, i) => i > start && l === '];');
  const cats = [];
  let cur = null;
  for (let i = start; i <= end; i++) {
    const ln = lines[i];
    const cat = ln.match(/^  \{ id:'([^']+)', name:'([^']+)', items:\[$/);
    if (cat) { cur = { id: cat[1], name: cat[2], line: i + 1, items: [] }; cats.push(cur); continue; }
    if (ln.trim() === ']},' || ln.trim() === ']}') { cur = null; continue; }
    const item = ln.match(/^    \{ id:'([^']+)', name:'([^']+)', domains:\[(.*?)\], init:'([^']+)' \},?$/);
    if (item && cur) {
      cur.items.push({
        id: item[1], name: item[2],
        domains: [...item[3].matchAll(/'([^']*)'/g)].map(m => m[1]),
        init: item[4]
      });
      continue;
    }
    const t = ln.trim();
    if (t === '' || t.startsWith('const CATEGORY_DEFS') || t === '];' || t.startsWith('/*')) continue;
    assert.fail(`原型 :${i + 1} 这行 CATEGORY_DEFS 里的形状不认识，解析要跟着改：${ln}`);
  }
  return cats;
}
