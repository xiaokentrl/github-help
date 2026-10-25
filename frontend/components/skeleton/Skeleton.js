/* Skeleton 骨架屏（组件库规划.md 4.10）：等后端首帧时先占个位，别让用户对着空白看。
   干什么用：原型只在详情指标那一格用过（:1916），且宽高是手填的 60×12——手填的毛病是
             真实值一变宽（比如 IP 变长），骨架跟内容对不上，一替换整块就跳版（详情面板那格跳动的老毛病同类，C4 修过）。
   会错在哪：件里允许 `width:任意像素`，早晚又回到"骨架与内容两套尺寸"，一到替换就跳。
   怎么回来：这里只认三个档位名，每一档的宽高都引真实件的那几支尺寸令牌（见 skeleton.css 注释），
             档位名不认识当场抛错；要第四档就先在样式里加一档，逼人来这儿登记一次。 */

/** 三个档位 = 三个真实买家的尺寸，全部引尺寸令牌，不接受手填像素 */
const PRESETS = {
  value: 'skeleton skeleton-value',   // 详情指标那一格：高走 --fs-lg，宽 4 个等宽字（原型 :1916 用的就是这个）
  line: 'skeleton skeleton-line',     // 一整行文字：高走 --fs-sm，宽跟着容器
  block: 'skeleton skeleton-block'    // 整块占位：高走 --ctl-lg，宽跟着容器
};

/**
 * @param {object} o
 * @param {'value'|'line'|'block'} [o.from]  缺省 'value'
 * @param {number} [o.lines]  要几行（一块占位连着排），缺省 1
 * @param {string} [o.ariaLabel] 给读屏的那句"正在加载"（由调用方给，件不持有文案）
 */
/** 档位名 → 类名；不认识就抛错（要第四档，先在 skeleton.css 里加一档，尺寸全引令牌） */
function presetOf(o) {
  const cls = PRESETS[o.from || 'value'];
  if (!cls) throw new Error(`Skeleton 不认识档位「${o.from}」，能用的是：${Object.keys(PRESETS).join(' / ')}`);
  return cls;
}

export function Skeleton(o = {}) {
  // 先查档位再动 DOM：这条守卫在 node 里也测得出来
  presetOf(o);
  const el = document.createElement('span');
  const parts = [];

  function paint() {
    const cls = presetOf(o);
    // line/block 这两档的宽写的是"跟着容器"，但外层是 inline-flex（按内容收缩），
    // 子层的 width:100% 算出来是 0——骨架等于没画。这两档改成占满块的 flex；
    // value 那一档保持 inline，免得把同一行的标签挤到下一行去。
    const fluid = (o.from || 'value') !== 'value';
    el.className = 'skeleton-wrap' + (fluid ? ' skeleton-wrap-fluid' : '');
    parts.length = 0;
    const n = Math.max(1, o.lines || 1);
    for (let i = 0; i < n; i++) {
      const p = document.createElement('span');
      p.className = cls;
      parts.push(p);
    }
    el.replaceChildren(...parts);
    el.setAttribute('role', 'status');
    el.setAttribute('aria-live', 'polite');
    if (o.ariaLabel) el.setAttribute('aria-label', o.ariaLabel);
  }

  paint();
  return {
    el,
    update(next = {}) { Object.assign(o, next); paint(); return this },
    setLines(n) { o.lines = n; paint(); return this }
  };
}
