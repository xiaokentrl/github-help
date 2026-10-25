/* 「关于我们」这一格（views/settings/panes/about）。干什么用：原型 :1004-1028 那两张卡片——
   上面那张是应用名、一句话简介和三个入口钮，下面那张是"键 值"四行（当前版本／构建日期／许可协议／数据目录）。
   会错在哪：① 版本与构建日期跟「检测升级」那一格读同一个 state.update，数据目录读 api.status() 回来的那一个——
                这里再写死一份，接上真服务之后两格会各说各的版本号；
             ② 那三颗钮原先是"回执一句（模拟）"（原型 :1717-1719 就是三句 toast）；
                2026-10-25 接成真的：契约第 17 个方法 OpenURL，服务用系统打开器开，scheme 只认 http(s)。
                URL 不写第二份家也不写在视图层之外——就在本文件这一个数组里，加一个入口钮只动这一处；
             ③ 打不开（离线、scheme 不对、打开器没跑成）不说"成功了"：按码换句子，服务原文跟在后面；
             ④ 这一格同样没有「保存」（hasSave:false，原型 :1551）。
   怎么回来：四行键值走 .set-row.tight ＋ .sr-key/.sr-val/.sr-code 这三个类（layout.css 第 11 节收的就是原型 :1023-1026 那四行行内样式），
             本面板只读不改，sync() 每次打开把 state.update 里的数补一遍。 */

import { Button } from '../../../../components/button/Button.js';
import { setCard, setRow } from '../card.js';
import { el } from '../../../../utils/dom.js';
import { api } from '../../../../core/api/client.js';

/** 三个入口与它们各自落点。域名随着仓库搬家改这里一个字，别处不动。 */
const LINKS = [
  ['settings.about.repo', 'settings.about.toastRepo', 'https://github.com/xiaokentrl/github-help'],
  ['settings.about.feedback', 'settings.about.toastFeedback', 'https://github.com/xiaokentrl/github-help/issues'],
  ['settings.about.licenseBtn', 'settings.about.toastLicense', 'https://github.com/xiaokentrl/github-help#readme']
];

/** 一行"键 值"：左边的键与右边的值各自的字色/等宽由 .sr-key/.sr-val/.sr-code 三个类给（layout.css 第 11 节） */
function infoRow(key, valueNode) {
  return el('div', { class: 'set-row tight' }, [el('span', { class: 'sr-key', text: key }), valueNode]);
}

export default {
  id: 'about',
  order: 7,
  group: 'info',
  icon: 'ⓘ',
  nameKey: 'settings.nav.about',
  subKey: 'settings.sub.about',
  hasSave: false,

  build(ctx) {
    const { t } = ctx;
    const u = ctx.state.update;

    const versionEl = el('b', { class: 'sr-val' });
    const buildEl = el('b', { class: 'sr-val' });
    const dirEl = el('code', { class: 'sr-code' });

    function sync() {
      versionEl.textContent = u.current;
      buildEl.textContent = u.buildDate || t('common.dash');
      dirEl.textContent = ctx.state.appStatus.dataDir || t('common.dash');
    }
    sync();

    const links = LINKS.map(([labelKey, toastKey, url]) => Button({
      label: t(labelKey),
      kind: 'ghost',
      onClick: async () => {
        try {
          await api.openURL(url);
          ctx.toast(t(toastKey));
        } catch (e) {
          // 打不开就如实说：码＋服务原文，不把"没打开成"画成"打开了"
          ctx.toast(`${t('settings.about.openFail')}${e && e.message ? '：' + e.message : ''}`);
        }
      }
    }));

    return {
      el: el('div', { class: 'settings-pane' }, [
        el('h3', { class: 'pane-title', text: t('settings.about.title') }),
        el('p', { class: 'pane-desc', text: t('settings.about.desc') }),
        setCard({
          rows: [el('div', { class: 'about-hero' }, [
            el('div', { class: 'about-logo-lg', text: 'GH' }),
            el('div', { class: 'about-hero-info' }, [
              el('h3', { text: t('settings.about.name') }),
              el('p', { text: t('settings.about.tagline') }),
              el('div', { class: 'about-links' }, links.map(b => b.el))
            ])
          ])]
        }),
        setCard({
          icon: 'ⓘ', title: t('settings.about.versionInfo'),
          rows: [
            infoRow(t('settings.about.rowCurrent'), versionEl),
            infoRow(t('settings.about.rowBuild'), buildEl),
            infoRow(t('settings.about.licenseBtn'), el('b', { text: t('settings.about.license') })),
            infoRow(t('settings.about.rowDataDir'), dirEl)
          ]
        })
      ]),
      sync
    };
  }
};
