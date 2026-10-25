/* 详情面板那七行指标里的一行（组件库规划.md 4.15 的 MetricRow）。干什么用：标签 + 值（等宽/弱化/带单位），
   忙的时候整格换成骨架屏。对应原型 :1894-1918 那段 DETAIL_METRICS + map。
   会错在哪：① 骨架屏必须换成同一批行、同一列宽，否则一进检测状态面板就多一行少一行、抖一下（那就是 C4 修过的病）；
             ② 值文本由调用方算好传进来（"延迟 128 ms" 那种拼接在 DetailView），件里不拼单位也不写中文——
                单位将来要跟语言走（"ms" 不换、"MB/s" 不换，但"尚未检测"要换）。
   怎么回来：只有 update(data)；busy 为真时 label 照旧显示、值那一格交给 Skeleton 件，
             件档位（'value'）和尺寸都在 skeleton.css 里引令牌，不在这儿写像素。 */

import { Skeleton } from '../../../components/skeleton/Skeleton.js';
import { el } from '../../../utils/dom.js';

/**
 * @param {object} [next]
 * @param {string} next.label  这一行叫什么（必填，调用方从 i18n 取）
 * @param {string} [next.value]  显示出来的值（已含单位）
 * @param {string} [next.cls]  值那一格的类名：'mono'/'dim'/…，字号那两档见 layout.css 的 .metric b.m-src/.m-time
 * @param {boolean} [next.busy]  检测/应用中：值那一格换成骨架屏，标签照旧
 */
export function MetricRow() {
  const label = el('label');
  const value = el('b');
  const skeleton = Skeleton({ from: 'value' });
  const node = el('div', { class: 'metric' }, [label, value]);

  let data = {};

  function paint() {
    if (data.label == null) throw new Error('MetricRow.update 少了 label：这一行没有名字，读屏念不出这一格是什么');
    label.textContent = data.label;
    if (data.busy) {
      value.className = '';
      if (!value.contains(skeleton.el)) {
        while (value.firstChild) value.removeChild(value.firstChild);
        value.appendChild(skeleton.el);
      }
      return;
    }
    // 骨架屏撤掉之后要连它外面那层 wrap 一起清干净，留一半就会看着像值没更新
    while (value.firstChild) value.removeChild(value.firstChild);
    value.className = data.cls || '';
    value.textContent = data.value == null ? '' : String(data.value);
  }

  return {
    el: node,
    update(next) { data = Object.assign(data, next); paint(); return this }
  };
}
