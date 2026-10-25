/* 后端菜单数据轴的生成器。干什么用：读前端 frontend/menu/ 那 11 个 module.js（全项目域名清单的唯一真源），
   生成 internal/menu/<菜单id>/menu.json 和同目录 common/common.json，外加两份 README.md——一共 11 × 4 = 44 个文件。
   为什么要有它：后端那 11 个数据目录是人不能手写的第二个家（总纲第 0.1 节第 6 问：手写清单一定会漏）。
     菜单数据只有一个真源在前端，后端这份是它的生成物，提交进仓库，所以 Go 构建不需要 Node；
     "重新生成应当一个字也不变"由 frontend/tests/backendMenu.test.mjs 每次现场跑一遍盯着（门禁 G4）。
   怎么会错：改了一条域名忘了跑它 → 界面列 54 条、后端认 53 条，少那条点了"应用"什么也不发生；
     或反过来手改了后端 JSON → 下一次生成就把它盖掉，改动看着像丢了。出错时看得见的是测试当场红，消息说清哪一块哪一条。
   怎么回来：跑 `node tools/gen-backend-menu.mjs`，它自己重算、重写、并按字典序输出，两次结果必须逐字节相同。
   用法：node tools/gen-backend-menu.mjs [--check]   加 --check 只比对不写盘，有出入就退出码 1（给 CI 用）。 */

import { readdir, readFile, writeFile, mkdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { collectMenuModules } from '../frontend/menu/collect.js';
import { isDomain, parentDomains } from './domain-shape.mjs';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const FRONTEND_MENU = path.join(ROOT, 'frontend', 'menu');
const BACKEND_MENU = path.join(ROOT, 'internal', 'menu');

const JSON_END = '\n';

function menuJson(m) {
  return JSON.stringify({
    id: m.id,
    name: m.name,
    order: m.order,
    items: m.items.map(it => ({ id: it.id, name: it.name, domains: it.domains })),
  }, null, 2) + JSON_END;
}

function commonJson(m) {
  const parents = parentDomains(m.items.flatMap(it => it.domains));
  return JSON.stringify({ parentDomains: parents }, null, 2) + JSON_END;
}

function menuReadme(m, parents) {
  const bad = m.items.flatMap(it => it.domains).filter(d => !isDomain(d));
  const lines = [
    `# ${m.name}（\`${m.id}\`）`,
    '',
    `这块菜单管 ${m.items.length} 条二级条目，排第 ${m.order}；条目的名字与域名见 \`menu.json\`。`,
    `数据来源：\`frontend/menu/${m.id}/module.js\`（唯一真源），本目录是它的生成物——手改这里会被下一次生成盖掉。`,
    `谁能用：\`internal/menu\` 读取包（服务启动时校验），编排层 \`internal/service\` 只读它，界面不直接读。`,
  ];
  if (bad.length > 0) {
    lines.push(`本块有 ${bad.length} 条不是合法域名形状（${bad.join('、')}），写 hosts 那一刻会被拒绝并如实标出来〔已定：D4 甲，数据不擅自改〕。`);
  }
  lines.push(`父域归属（本块 ${parents.length} 个）见 \`common/common.json\`，口径见 \`common/README.md\`。`);
  return lines.join('\n') + '\n';
}

function commonReadme(m, parents) {
  const lines = [
    `# ${m.name} 的公共部分（\`${m.id}\`）`,
    '',
    `这个目录放本块菜单 ${m.items.length} 条二级条目共用的东西〔已定：R8〕，现在只有父域归属：${parents.join('、')}。`,
    `父域由 \`items\` 里每条域名的最后两段现场算出（不合格形状跳过），与 \`common.json\` 做集合等值——门禁 G12 盯着，多一个少一个都红。`,
    `这里不许出现条目 id、条目数组或完整域名：那样它就变成域名清单的第二个家（G12 当场红）。`,
    `要不要探测（needProbe）与还原口径（restoreScope）此刻一块也没有值，所以不写；缺省只在 \`internal/menu/menu.go\` 一处给〔已定：D6 之外另有一条"默认值只许有一个家"，见后端规划第 10 节\`Common\` 那行〕。`,
  ];
  return lines.join('\n') + '\n';
}

async function readSourceMenus() {
  const names = (await readdir(FRONTEND_MENU, { withFileTypes: true }))
    .filter(e => e.isDirectory())
    .map(e => e.name)
    .sort();
  const modules = {};
  for (const name of names) {
    const file = path.join(FRONTEND_MENU, name, 'module.js');
    modules[`./${name}/module.js`] = await import(pathToFileURL(file).href);
  }
  return collectMenuModules(modules);
}

/** 把生成结果摊平成「相对路径 → 文件内容」，写盘与比对共用同一份，避免两条代码路径各算一遍 */
function buildFiles(menus) {
  const files = new Map();
  for (const m of menus) {
    const parents = parentDomains(m.items.flatMap(it => it.domains));
    files.set(`${m.id}/menu.json`, menuJson(m));
    files.set(`${m.id}/common/common.json`, commonJson(m));
    files.set(`${m.id}/README.md`, menuReadme(m, parents));
    files.set(`${m.id}/common/README.md`, commonReadme(m, parents));
  }
  return files;
}

const checkOnly = process.argv.includes('--check');

const menus = await readSourceMenus();
const files = buildFiles(menus);

let drift = [];
for (const [rel, content] of files) {
  const abs = path.join(BACKEND_MENU, rel);
  let current = null;
  try {
    current = await readFile(abs, 'utf8');
  } catch {
    current = null;
  }
  if (current === content) continue;
  drift.push(rel);
  if (!checkOnly) {
    await mkdir(path.dirname(abs), { recursive: true });
    await writeFile(abs, content);
  }
}

/* 后端目录里多出来的东西（比如有人给某条子菜单开了目录）也要报出来——不然"重新生成应无变化"只查了一半 */
const expected = new Set([...files.keys()]);
async function walk(dir, prefix = '') {
  const out = [];
  for (const e of await readdir(dir, { withFileTypes: true })) {
    const rel = prefix ? `${prefix}/${e.name}` : e.name;
    if (e.isDirectory()) out.push(...await walk(path.join(dir, e.name), rel));
    else out.push(rel);
  }
  return out;
}
const onDisk = (await walk(BACKEND_MENU)).filter(f => !f.endsWith('.go'));
for (const f of onDisk) {
  if (!expected.has(f)) drift.push(`${f}（后端目录里多出来的文件，生成器不认它）`);
}

const itemCount = menus.reduce((n, m) => n + m.items.length, 0);
console.log(`菜单 ${menus.length} 块 / 条目 ${itemCount} 条 / 生成文件 ${files.size} 个（每块 4 个）`);
if (drift.length > 0) {
  console.log(`${checkOnly ? '需要重新生成' : '已重写'}：${drift.join('、')}`);
  if (checkOnly) process.exit(1);
} else {
  console.log('生成物与真源一致，一个字没改');
}
