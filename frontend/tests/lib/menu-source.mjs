/* 前端菜单真源（frontend/menu/ 那 11 个 module.js）唯一的读法。干什么用：
   两个测试文件都要"现场把 11 块菜单读出来"——structure.test.mjs 守前端 ⇄ 原型，backendMenu.test.mjs 守后端 ⇄ 前端 ⇄ 原型。
   会错在哪： walker 写两份，其中一份漏了某个子目录或换了排序，两边就"各量各的都说对"，那是比红更糟的假绿。
   怎么回来：这里只负责"读出来"，不负责判断；判断留在各自的测试里，形状断言改了不会影响另一侧。 */

import { readdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { collectMenuModules } from '../../menu/collect.js';

const LIB_DIR = path.dirname(fileURLToPath(import.meta.url));
export const FRONTEND_MENU = path.resolve(LIB_DIR, '..', '..', 'menu');

/** 返回 [{name, dir, files}]，按目录名字典序；不 import，纯看目录形状 */
export async function readMenuSourceDirs() {
  const names = (await readdir(FRONTEND_MENU, { withFileTypes: true }))
    .filter(e => e.isDirectory()).map(e => e.name).sort();
  const out = [];
  for (const name of names) {
    const dir = path.join(FRONTEND_MENU, name);
    out.push({ name, dir, files: (await readdir(dir)).sort() });
  }
  return out;
}

/** 逐个 import 各块 module.js 后交给 collectMenuModules：撞号当场抛，清单顺序就是 order */
export async function loadFrontendMenus(folders) {
  const list = folders ?? await readMenuSourceDirs();
  const modules = {};
  for (const f of list) {
    modules[`./${f.name}/module.js`] = await import(pathToFileURL(path.join(f.dir, 'module.js')).href);
  }
  return collectMenuModules(modules);
}
