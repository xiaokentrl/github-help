/* 可重复的随机数。干什么用：原型的 generateCandidates 那套假数据要靠它——同一个条目 id 每次刷出同一批候选 IP，界面才不会一跳一跳。
   会错在哪：这三个函数原本挂在原型顶层，谁都能直接调；只要界面上还有地方用它们，就是假候选地址漏到了真界面（真地址只准由服务回，契约 v9 的 candidates 那一栏）。
   怎么回来：mock 已在 2026-10-23 真服务接线那一轮删除，本文件此刻没有任何调用者（实测：全仓 grep 'utils/random' 只命中它自己）；
             #53 那轮候选地址改由服务回之后，本文件与 前端目录结构规划.md 里 utils 那一行一起删。 */

/** 逐字搬自原型 :1226：FNV-1a，拿条目 id 当种子 */
export function hashStr(s) {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
  return h >>> 0;
}

/** 逐字搬自原型 :1227：mulberry32，给定种子就出稳定序列 */
export function mulberry32(a) {
  return function () {
    a |= 0; a = a + 0x6D2B79F5 | 0;
    let t = Math.imul(a ^ a >>> 15, 1 | a);
    t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
    return ((t ^ t >>> 14) >>> 0) / 4294967296;
  };
}

/** 逐字搬自原型 :1228：四段地址，取值范围跟原型一致（1~254） */
export function randIP(rng) {
  const seg = () => 1 + Math.floor(rng() * 254);
  return `${seg()}.${seg()}.${seg()}.${seg()}`;
}
