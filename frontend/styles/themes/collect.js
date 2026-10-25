/* 把扫描到的一堆主题模块整理成排好序的清单。放在这里是因为它不碰构建工具、不碰 DOM，能在 node --test 里直接跑。
   干什么用：`index.js` 负责扫目录，本文件负责检查扫出来的东西合不合格、按 order 排好。
   会错在哪：三件事最容易静默出错——
             ① 文件夹改了名、`theme.js` 里的 id 没跟着改：`theme.css` 里那块 `html[data-theme="x"]` 就对不上，
                选这套主题时颜色一片都不换，界面上看着像"坏了一套"；
             ② order 撞号还静默排序：六个圆点的先后随机漂；
             ③ 预览四色少给一支：主题弹窗那张卡少一条色，而卡片是按数组画的。
   怎么回来：这三件当场抛错，不静默。 */

const HEX = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;

function bad(path, msg) {
  return new Error(`主题模块 ${path}：${msg}`);
}

/** scanned 的形状就是 import.meta.glob(..., {eager:true}) 返回的 { 路径: 模块 } */
export function collectThemes(scanned) {
  const entries = Object.entries(scanned);
  if (entries.length === 0) throw new Error('themes/ 下一个模块也没扫到：检查 index.js 的扫描路径还对不对');

  const seenOrders = new Map();
  const themes = [];

  for (const [path, mod] of entries) {
    const dirId = (path.match(/^\.\/([^/]+)\/theme\.js$/) || [])[1];
    if (!dirId) throw bad(path, '不在 themes/<主题id>/theme.js 这个形状上，扫描结果没法认出它是哪套主题');
    const theme = mod && mod.default !== undefined ? mod.default : mod;
    if (!theme || typeof theme !== 'object') throw bad(path, '没有默认导出一个主题对象');
    for (const key of ['id', 'name', 'desc', 'order', 'colors']) {
      if (theme[key] === undefined) throw bad(path, `缺字段 ${key}（第 3 节要求 theme.js 交代这五样）`);
    }
    if (typeof theme.id !== 'string' || theme.id === '') throw bad(path, '主题 id 得是非空字符串');
    if (theme.id !== dirId) throw bad(path, `主题 id「${theme.id}」和文件夹名「${dirId}」不一致：theme.css 里那块变量会盖不住`);
    if (typeof theme.name !== 'string' || theme.name === '') throw bad(path, `主题 ${theme.id} 缺中文名（圆点的 title 与卡片标题都用它）`);
    if (typeof theme.desc !== 'string' || theme.desc === '') throw bad(path, `主题 ${theme.id} 缺一句简介（卡片副标题用它）`);
    if (!Number.isInteger(theme.order)) throw bad(path, `主题 ${theme.id} 的 order 得是整数，现在是 ${JSON.stringify(theme.order)}`);
    const holder = seenOrders.get(theme.order);
    if (holder) throw bad(path, `主题 ${theme.id} 与 ${holder} 的 order 撞号（都是 ${theme.order}）：六个圆点谁前谁后没定，去改其中一套的 order`);
    seenOrders.set(theme.order, theme.id);
    if (!Array.isArray(theme.colors) || theme.colors.length !== 4)
      throw bad(path, `主题 ${theme.id} 的预览色得是四支（背景、面板、主色、强调），现在是 ${JSON.stringify(theme.colors)}`);
    for (const c of theme.colors) {
      if (typeof c !== 'string' || !HEX.test(c)) throw bad(path, `主题 ${theme.id} 的预览色「${c}」不是十六进制色值，卡片那一格会画成空白`);
    }
    themes.push({ ...theme, path });
  }

  themes.sort((a, b) => a.order - b.order);
  return themes;
}
