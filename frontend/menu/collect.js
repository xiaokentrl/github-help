/* 把扫描到的一堆菜单模块整理成排好序的清单。放在这里是因为它不碰构建工具、不碰 DOM，能在 node --test 里直接跑。
   干什么用：`menu/index.js` 负责扫目录，本文件负责检查扫出来的东西合不合格、按 order 排好。
   会错在哪：清单一旦改成人手写，就会出现"文件夹加了、清单没加"；id 或 order 撞号若静默排序，界面上少一块菜单没人看得出。
   怎么回来：这两件事当场抛错报错，不静默。 */

function bad(path, msg) {
  return new Error(`菜单模块 ${path}：${msg}`);
}

function checkItem(path, item, seenItemIds) {
  if (typeof item.id !== 'string' || item.id === '') throw bad(path, `条目缺 id：${JSON.stringify(item.name)}`);
  if (seenItemIds.has(item.id)) throw bad(path, `条目 id 重复：${item.id}（同一条域名项不能出现在两块菜单里）`);
  seenItemIds.add(item.id);
  if (typeof item.name !== 'string' || item.name === '') throw bad(path, `条目 ${item.id} 缺界面上的名字`);
  if (!Array.isArray(item.domains) || item.domains.length === 0) throw bad(path, `条目 ${item.id} 没有域名，画出来是条空行`);
  if (typeof item.init !== 'string' || item.init === '') throw bad(path, `条目 ${item.id} 缺初态（原型用它决定默认勾选与状态点）`);
}

/** scanned 的形状就是 import.meta.glob(..., {eager:true}) 返回的 { 路径: 模块 } */
export function collectMenuModules(scanned) {
  const entries = Object.entries(scanned);
  if (entries.length === 0) throw new Error('menu/ 下一个模块也没扫到：检查 index.js 的扫描路径还对不对');

  const seenIds = new Set();
  const seenOrders = new Map();
  const seenItemIds = new Set();
  const menus = [];

  for (const [path, mod] of entries) {
    const menu = mod && mod.default !== undefined ? mod.default : mod;
    if (!menu || typeof menu !== 'object') throw bad(path, '没有默认导出一个菜单对象');
    for (const key of ['id', 'name', 'order', 'items']) {
      if (menu[key] === undefined) throw bad(path, `缺字段 ${key}（第 3 节要求 module.js 交代这四样）`);
    }
    if (typeof menu.id !== 'string' || menu.id === '') throw bad(path, '菜单 id 得是非空字符串');
    if (seenIds.has(menu.id)) throw bad(path, `菜单 id 重复：${menu.id}`);
    seenIds.add(menu.id);
    if (!Number.isInteger(menu.order)) throw bad(path, `菜单 ${menu.id} 的 order 得是整数，现在是 ${JSON.stringify(menu.order)}`);
    const holder = seenOrders.get(menu.order);
    if (holder) throw bad(path, `菜单 ${menu.id} 与 ${holder} 的 order 撞号（都是 ${menu.order}）：界面上谁前谁后没定，去改其中一块的 order`);
    seenOrders.set(menu.order, menu.id);
    if (!Array.isArray(menu.items) || menu.items.length === 0) throw bad(path, `菜单 ${menu.id} 一条二级条目也没有`);
    for (const item of menu.items) checkItem(path, item, seenItemIds);
    menus.push({ ...menu, path });
  }

  menus.sort((a, b) => a.order - b.order);
  return menus;
}
