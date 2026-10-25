/* 域名形状的判定口径只有一个家。干什么用：判一个字符串是不是能写进 hosts 的合法域名，并算出它的父域（最后两段）。
   为什么单独一个文件：前端门禁、后端 Go、生成器三边都要用同一把尺子，写在谁里面另外两边就得抄一遍——
   抄的那一份迟早会跟原本不一致，那时"两边判定一致"这句话就变成假话。
   会错在哪：判得太宽 → 带路径的地址混进 hosts，那一行无效，用户以为加速生效了；判得太严 → 正常域名被当成垃圾登记。
   怎么回来：用例表在 tools/domain-cases.json（也只有一份），Go 与 JS 两边各自的测试都读它，任一侧改了判定就红。 */

/** 合法域名形状：只有小写字母、数字、连字符，至少两段，段首段尾不许是连字符，末段以字母起头，总长 1–253。
    与 internal/menu/validate.go 那条 domainRe 同一口径；改这里必须同时改那边，两边的对照用例见 tools/domain-cases.json。 */
const DOMAIN_SHAPE = /^(?=.{1,253}$)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$/;

export const isDomain = d => DOMAIN_SHAPE.test(d);

/** 父域＝合法域名的最后两段。不合格的形状没有父域可言，整条跳过（否则公共目录里长出垃圾）。
    量出来的不合格一共三条：block 那条带路径的地址、custom 那块的「自定义域名」和另一条带路径的地址。 */
export function parentDomains(domains) {
  const out = new Set();
  for (const d of domains) {
    if (!isDomain(d)) continue;
    const parts = d.split('.');
    out.add(parts.slice(-2).join('.'));
  }
  return [...out].sort();
}
