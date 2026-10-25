/* 把扫描到的一堆设置面板模块整理成排好序的清单。放在这里是因为它不碰构建工具、不碰 DOM，能在 node --test 里直接跑。
   干什么用：`panes/index.js` 负责扫目录，本文件负责检查扫出来的东西合不合格、按 order 排好，并分出左侧导航那两组。
   会错在哪：三件事最容易静默出错——
             ① 文件夹改了名、`pane.js` 里的 id 没跟着改：外框那个 data-pane 值与切换表对不上，点导航切不过去；
             ② order 撞号还静默排序：导航里谁前谁后随构建工具的返回顺序漂；
             ③ 组名写错（不是 prefs 也不是 info）：那条面板在导航里根本不出现，但面板本体还在——界面少一格而没人报错。
   怎么回来：这三件当场抛错，不静默。 */

const GROUPS = ['prefs', 'info'];

function bad(path, msg) {
  return new Error(`设置面板 ${path}：${msg}`);
}

/** scanned 的形状就是 import.meta.glob(..., {eager:true}) 返回的 { 路径: 模块 } */
export function collectPanes(scanned) {
  const entries = Object.entries(scanned);
  if (entries.length === 0) throw new Error('panes/ 下一个面板也没扫到：检查 index.js 的扫描路径还对不对');

  const seenOrders = new Map();
  const panes = [];

  for (const [path, mod] of entries) {
    const dirId = (path.match(/^\.\/([^/]+)\/pane\.js$/) || [])[1];
    if (!dirId) throw bad(path, '不在 panes/<面板id>/pane.js 这个形状上，扫描结果没法认出它是哪一格');
    const pane = mod && mod.default !== undefined ? mod.default : mod;
    if (!pane || typeof pane !== 'object') throw bad(path, '没有默认导出一个面板对象');
    for (const key of ['id', 'order', 'group', 'icon', 'nameKey', 'subKey', 'build']) {
      if (pane[key] === undefined) throw bad(path, `缺字段 ${key}（第 6.2 节要求 pane.js 交代这七样）`);
    }
    if (typeof pane.id !== 'string' || pane.id === '') throw bad(path, '面板 id 得是非空字符串');
    if (pane.id !== dirId) throw bad(path, `面板 id「${pane.id}」和文件夹名「${dirId}」不一致：导航那一条与内容段就是这么对上的`);
    // id 唯一这件事不用再查一遍：扫描的键（路径）本来就唯一，而上一行刚要求 id 必须等于文件夹名。
    if (!Number.isInteger(pane.order)) throw bad(path, `面板 ${pane.id} 的 order 得是整数，现在是 ${JSON.stringify(pane.order)}`);
    const holder = seenOrders.get(pane.order);
    if (holder) throw bad(path, `面板 ${pane.id} 与 ${holder} 的 order 撞号（都是 ${pane.order}）：导航里谁前谁后没定，去改其中一格的 order`);
    seenOrders.set(pane.order, pane.id);
    if (!GROUPS.includes(pane.group)) throw bad(path, `面板 ${pane.id} 的 group 是「${pane.group}」，只认 ${GROUPS.join('／')}：这一格在导航里会根本不存在`);
    for (const key of ['icon', 'nameKey', 'subKey']) {
      if (typeof pane[key] !== 'string' || pane[key] === '') throw bad(path, `面板 ${pane.id} 的 ${key} 得是非空字符串`);
    }
    if (typeof pane.build !== 'function') throw bad(path, `面板 ${pane.id} 的 build 不是函数，这一格画不出任何东西`);
    if (pane.hasSave !== undefined && typeof pane.hasSave !== 'boolean') throw bad(path, `面板 ${pane.id} 的 hasSave 得是布尔值（原型 :1551：更新与关于两格不显示保存）`);
    if (pane.badge !== undefined && typeof pane.badge !== 'boolean') throw bad(path, `面板 ${pane.id} 的 badge 得是布尔值（导航那条右边那个红点）`);
    panes.push({ ...pane, hasSave: pane.hasSave !== false, badge: !!pane.badge, path });
  }

  panes.sort((a, b) => a.order - b.order);
  return panes;
}
