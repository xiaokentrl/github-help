/* 主题弹窗里那一张配色卡（组件库规划.md 第 4 节列的第 5 个行级件）。干什么用：原型 :3325-3331 那段
   buildThemePicker 里的卡片模板——上面三条色块（背景占两格、面板色与主色、强调色）、下面名字与简介。
   会错在哪：① 名字、简介、四支预览色的家都在 styles/themes/<id>/theme.js，件里一个也不写死，
                加一套主题只多一个文件夹，这张卡跟着扫描结果多一张；
             ② 选中态只有 `.selected` 这一个类（勾是 layout.css 里 ::after 画的）。自己另加一个"对勾"节点，
                换主题时就会留下两枚勾；
             ③ 卡片是 div，键盘进不去——原型如此。这里补 role／tabindex／回车空格，是为了不改样式的前提下让它能聚焦。
   怎么回来：件只报"这张卡被点了"，改哪个主题由外面定（views/theme-picker 那一份接到 store 与主题钩子上）。 */

import { el } from '../../../utils/dom.js';

/**
 * @param {object} o
 * @param {object} o.theme       styles/themes 扫出来的那一条（id／name／desc／colors 四支）
 * @param {boolean} o.selected
 * @param {(id:string)=>void} o.onPick
 */
export function ThemeCard({ theme, selected, onPick }) {
  if (!Array.isArray(theme.colors) || theme.colors.length !== 4) {
    throw new Error(`主题 ${theme.id} 的预览色不是四支：这张卡按四支画，少一支就空一格`);
  }
  const [bg, surface, accent, extra] = theme.colors;

  const card = el('div', {
    class: 'theme-card' + (selected ? ' selected' : ''),
    dataset: { themeId: theme.id },
    attrs: { role: 'button', tabindex: '0', 'aria-pressed': selected ? 'true' : 'false' }
  }, [
    el('div', { class: 'theme-preview' }, [
      el('div', { class: 'theme-swatch theme-swatch-bg', style: `background:${bg}` }),
      el('div', { class: 'theme-swatch', style: `background:${surface}` }),
      el('div', { class: 'theme-swatch', style: `background:${accent}` }),
      el('div', { class: 'theme-swatch', style: `background:${extra}` })
    ]),
    el('div', { class: 'theme-name', text: theme.name }),
    el('div', { class: 'theme-desc', text: theme.desc })
  ]);

  card.addEventListener('click', () => onPick(theme.id));
  card.addEventListener('keydown', e => {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onPick(theme.id); }
  });
  return { el: card };
}
