/* 主菜单清单的唯一生成处：扫 menu/<菜单id>/module.js，人不写清单。
   干什么用：把扫到的模块交给 collect.js 排序校验，界面（views/menu/）只读这一个 menus。
   会错在哪：这一行 import.meta.glob 是构建工具（Vite）的能力，用纯浏览器 ES Module 直接打开会认不出它——第 11 节第 1、3 条把这两件事绑在一起了。
   怎么回来：目录少了 / order 撞号都会在构建或测试当场报错，不会给你一棵少菜单的树。 */
import { collectMenuModules } from './collect.js';

const scanned = import.meta.glob('./*/module.js', { eager: true });

export const menus = collectMenuModules(scanned);
