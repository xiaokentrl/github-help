/* 主题清单的唯一生成处：扫 styles/themes/<主题id>/theme.js，人不写清单。
   干什么用：把扫到的主题交给 collect.js 校验排序，托盘胶囊那六个圆点、主题弹窗那六张卡、
             设置中心的外观面板，全都只读这里的一份 themes。
   会错在哪：这一行 import.meta.glob 是构建工具（Vite）的能力，纯浏览器直接打开认不出它——
             和 menu/index.js 一样，第 11 节第 1、3 条把这两件事绑在一起了。
   怎么回来：少一套、id 撞号、order 撞号、预览四色缺支都在 collect.js 当场报错，
             不会给你一个少一个圆点的胶囊。 */
import { collectThemes } from './collect.js';

const scanned = import.meta.glob('./*/theme.js', { eager: true });

export const themes = collectThemes(scanned);
