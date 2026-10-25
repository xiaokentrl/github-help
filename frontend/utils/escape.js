/* 往界面里塞文字之前先转义。干什么用：域名、条目名这些字符串要进 innerHTML 模板，不转义的话一个 `<` 就能把整棵树的骨架顶坏。
   会错在哪：漏一处转义，界面上少一行或多一行，运气差时脚本还能被当内容执行。
   怎么回来：渲染类的代码统一从这里取这两个函数，第 8 节那条 grep 门禁会盯 components/ 与 parts/ 里的裸 innerHTML 拼装。 */

/** 逐字搬自原型 :1231，字符表和顺序都没动 */
export function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, m => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[m]));
}

/** 逐字搬自原型 :1232，搜索框里的正则元字符靠它 */
export function escapeRegExp(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
