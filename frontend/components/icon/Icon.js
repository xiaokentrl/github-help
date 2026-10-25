/* Icon 图标（组件库规划.md 4.2）：干什么用——界面里那些小符号，一律内联 SVG。
   会错在哪：原型用的是字符（⟳ ↺ ↓ × ✓ ▾ …）。同一支字符在 Windows 和 Linux 的字体里宽度不一样，
             同一行会左右漂、深色下粗细也不一致——这正是跨平台观感不一致的真实来源之一，已定全换内联 SVG。
   怎么回来：尺寸只有 14/16/18 三档（--icon-sm/md/lg）；要加一个图标只在本文件那张表里加一条 path，
             视图里不许自己写 <svg>，否则同一个箭头会长出两副样子。 */

const PATHS = {
  'caret': ['M6 9l6 6 6-6'],
  'check': ['M4 12.5l5 5L20 6.5'],
  'close': ['M6 6l12 12', 'M18 6L6 18'],
  'minus': ['M5 12h14'],
  'square': ['M6.5 6.5h11v11h-11z'],
  'refresh': ['M19 12a7 7 0 1 1-2.05-4.95', 'M19 5.2v3.1h-3.1'],
  'restore': ['M5 12a7 7 0 1 0 2.05-4.95', 'M5 5.2v3.1h3.1'],
  'arrow-down': ['M12 5v13', 'M6 12.5l6 6 6-6'],
  'info': ['M12 11.5v5.5', 'M12 7.6v.9'],
  'list': ['M4 7.5h16', 'M4 12h16', 'M4 16.5h16'],   // 详情未选那格（原型 :1907 那个字符 ☰）
  // 下面三支是搬外框那几段时补的：⚙ 与 ⟳ 那两个字符、托盘菜单那两条（原型 :662-663 的两支内联 SVG）
  'gear': ['M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
    'M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z'],
  'minimize': ['M6 19h12', 'M4 5h16v10H4z'],
  'quit': ['M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4', 'M16 17l5-5-5-5', 'M21 12H9']
};

/**
 * @param {object} o
 * @param {keyof typeof PATHS} o.name  图标名；不在表里当场报错，免得画出一个空白
 * @param {'sm'|'md'|'lg'} [o.size]   14 / 16 / 18 三档，别的不收
 * @param {string} [o.label]          只在"这颗图标自己承担意思"时给；纯装饰就给 aria-hidden
 */
export function Icon(o) {
  const paths = PATHS[o.name];
  if (!paths) throw new Error(`Icon：没有「${o.name}」这个图标。要加就在 components/icon/Icon.js 的 PATHS 里加一条，别在视图里自己画`);
  const el = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  el.setAttribute('viewBox', '0 0 24 24');
  el.setAttribute('class', 'ico ico-' + (o.size || 'md'));
  if (o.label) el.setAttribute('role', 'img'), el.setAttribute('aria-label', o.label);
  else el.setAttribute('aria-hidden', 'true');
  for (const d of paths) {
    const p = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    p.setAttribute('d', d);
    el.appendChild(p);
  }
  return { el };
}
