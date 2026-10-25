/* 语言清单的唯一生成处：扫 core/i18n/locales/<语言id>.js，人不写清单。干什么用：把扫到的一份份键表交给 t.js，
   界面与 core 只认 t(key) 这一个入口；新增一种语言＝往这个目录放一个文件，这里与 t.js 一个字都不用改。
   会错在哪：① 这一行 import.meta.glob 是构建工具（Vite）的能力，用纯浏览器 ES Module 直接打开会认不出它——
                和 menu/index.js、styles/themes/index.js 一样。所以 node 里的测试不碰这个文件，
                只 import locales/zh-CN.js 那一份键表（tests/i18n.test.mjs 就是这么做的）。
             ② 这个目录是平铺的（不像 menu/ 与 themes/ 有子目录形状可靠），扫描会把邻居的工具文件也扫进来：
                collect.js 就叫「collect」——长得完全像一种语言，于是它导出的那些函数被当成键表，
                启动当场抛「键 collectLocales 的值不是字符串」。所以排除名单必须跟目录里的工具文件同步，
                这一条由 tests/i18n.test.mjs 那条"不是语言文件就得在排除名单里"当场看着。
   怎么回来：文件命名即语言 id（zh-CN.js → zh-CN）；键表不是对象、或默认导出漏了，collect.js 当场报错。 */
import { collectLocales } from './collect.js';

const scanned = import.meta.glob(['./*.js', '!./index.js', '!./collect.js'], { eager: true });

export const locales = collectLocales(scanned);
