/* 界面读数的共用量具（量具本身只能在浏览器里"跑"：它的函数一调用就要碰 document 与 CSSOM。
   但"import 它"在 node 里是安全的——本文件顶层不取 DOM，所以 tests/lib/screens.mjs 与 pin.mjs
   能在 node 里把它连同 tests/views/screens.probe.js 一起读进来，拿那份唯一的豁免名单。
   干什么用：两份实测脚本——件层 tests/components/state-matrix.probe.js 与界面层
             tests/views/screens.probe.js——都要算同一批数：任意颜色串归一成 [r,g,b,a]、
             文字底下真正压着的那块底色往上找、WCAG 相对亮度与对比度、盒子的像素、字号行高。
             算式只留这一份，两边都用它。
   会错在哪：① 同一套公式写两处迟早漂：件层判「过线」、界面层判「不过线」，两份基线自相打架，
                而人只会以为主题坏了；所以这次把件层那 5 个私有函数搬到这里；
             ② 搬动这份文件＝两份基线的读数算法都换了，tests/lib/matrix.mjs 的源码指纹把这里
                一起摘要进去（那边 sourceFiles() 的第 4 条），改一行守档立刻红，逼人把两份基线都重量重钉；
             ③ contrastOf 收的是 toRgba 归一之后的数组，不是 'rgb(...)' 字符串——字符串那份在
                tests/lib/matrix.mjs 里另有一份（node 那边要拿记录里的 fg/bg 现算，防人改基线）；
                两份各管一头是故意的，别合。
   怎么回来：改完这里，按两份量具各自文件头那三步各跑一次（件层一份、界面层一份），再
             `node tests/lib/pin.mjs <读数>` 与 `node tests/lib/pin.mjs --kind screens <读数>` 重钉；
             只改这里没重钉，node --test 那两份基线全红。 */

/** 任意 CSS 颜色串 → [r,g,b,a]。十六进制/rgb 直接读，其余（oklch 这类）借 canvas 归一，读不出来返回 null */
export function toRgba(value) {
  if (!value || value === 'transparent') return null;
  const m = value.match(/^rgba?\(([^)]+)\)$/);
  if (m) {
    const p = m[1].split(/[,\s/]+/).filter(Boolean).map(Number);
    if (p.length >= 3 && p.slice(0, 3).every(Number.isFinite)) return [p[0], p[1], p[2], p.length > 3 ? p[3] : 1];
  }
  const c = document.createElement('canvas');
  c.width = c.height = 1;
  const g = c.getContext('2d');
  g.fillStyle = '#000';
  g.fillStyle = value;
  const back = String(g.fillStyle);
  if (g.fillStyle instanceof Object || !/^#|^rgb/.test(back)) return null;
  if (back[0] === '#') {
    const n = parseInt(back.slice(1), 16);
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255, 1];
  }
  return toRgba(back);
}

export function alphaOf(c) { return c ? c[3] : 0; }

/** 往上找第一个真有底色的祖先——件自己不设背景时，字其实压在外层卡片上 */
export function bgOf(el) {
  for (let n = el; n; n = n.parentElement) {
    const c = toRgba(getComputedStyle(n).backgroundColor);
    if (c && alphaOf(c) > 0.5) return c;
  }
  const page = toRgba(getComputedStyle(document.body).backgroundColor);
  return page && alphaOf(page) > 0.5 ? page : [255, 255, 255, 1];
}

/** WCAG 相对亮度：跟 tests/lib/matrix.mjs 里那一份逐字同一套公式，两边算出来必须一模一样 */
function luminance(c) {
  const s = c.slice(0, 3).map(v => {
    const x = v / 255;
    return x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * s[0] + 0.7152 * s[1] + 0.0722 * s[2];
}

/** 对比度：两个入参都是 toRgba 的产物（数组），不是颜色串 */
export function contrastOf(fg, bg) {
  const a = luminance(fg);
  const b = luminance(bg);
  return +((Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)).toFixed(2);
}

/** 声明值换像素：'50%' 对着宿主的对应边，'12px' 直接取，'100%' 就是宿主那么长，auto 当 0 */
export function lenOf(value, ref) {
  if (!value || value === 'auto') return 0;
  if (value.endsWith('%')) return (parseFloat(value) / 100) * ref;
  return parseFloat(value) || 0;
}

/** 盒子：屏幕坐标下的宽高，四舍五入到整像素（亚像素在两份画面之间本来就对不齐，量出来只是噪声） */
export function boxOf(el) {
  const r = el.getBoundingClientRect();
  return { w: Math.round(r.width), h: Math.round(r.height) };
}

/** 字：字号、字重、行高（normal 记 null：那是"由浏览器定"，不是某个数）、字体族第一个名字 */
export function fontOf(el) {
  const s = getComputedStyle(el);
  return {
    size: parseFloat(s.fontSize) || 0,
    weight: Number(s.fontWeight) || 0,
    line: s.lineHeight === 'normal' ? null : parseFloat(s.lineHeight) || 0,
    family: (s.fontFamily || '').split(',')[0].trim().replace(/^["']|["']$/g, '')
  };
}
