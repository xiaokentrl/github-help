/* 样式装载顺序的唯一执行者。干什么用：按「tokens → base → layout → 组件 → 主题」这五步把 CSS 引进来，
   谁后装谁覆盖——主题放最后，所以主题里那一份变量能盖住 tokens 的浅色默认值；组件放布局之后，
   所以件自己的样式能纠正 layout 里那批公共骨架。
   会错在哪：这条顺序以前只写在文档里（前端目录结构规划.md 第 7 节），没有执行者；
             一旦有人在某个 .js 里单独 import 了一个 component.css，Vite 就按"第一次被引用"的位置插进去，
             顺序变成随机的，表现是"浅色主题下某块底色不对，切一下主题又好了"——最难查那种。
   怎么回来：所有 CSS 只从本文件进界面（main.js 只 import 这一个），别处出现 `.css'` 的 import 就是破了规矩；
             装载了哪几路交给下面导出的 loaded 名单，tests/styles.test.mjs 拿它跟磁盘上的 .css 清单一比，
             少装一路、多装一路都当场红；新增一个组件不用改这里（组件那一路是扫出来的）。 */

import './tokens.css';
import './base.css';
import './layout.css';

const componentCss = import.meta.glob('../components/*/*.css', { eager: true });
const themeCss = import.meta.glob('./themes/*/theme.css', { eager: true });

export const loaded = {
  components: Object.keys(componentCss).sort(),
  themes: Object.keys(themeCss).sort()
};
