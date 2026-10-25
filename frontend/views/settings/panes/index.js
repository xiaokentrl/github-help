/* 设置面板清单的唯一生成处：扫 panes/<面板id>/pane.js，人不写清单。干什么用：把扫到的面板交给 collect.js 校验排序，
   SettingsView 只读这一个 panes——左侧导航那七条、内容段那七格、切面板那张表，全从这里长出来。
   会错在哪：这一行目录扫描是构建工具（Vite）的能力，纯浏览器直接打开认不出它——和 menu/index.js 一样，
             第 11 节第 1、3 条把这两件事绑在一起了。清单要是改回手写，"加一个面板文件夹、导航里没这一条"就再也不报错。
   怎么回来：id 与文件夹名不一致、order 撞号、组名写错，都在 collect.js 当场抛，不会给你一条缺格子的导航。 */
import { collectPanes } from './collect.js';

const scanned = import.meta.glob('./*/pane.js', { eager: true });

export const panes = collectPanes(scanned);
