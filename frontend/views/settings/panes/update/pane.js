/* 「检测升级」这一格（views/settings/panes/update）。干什么用：原型 :965-1002 那三张卡片——
   三个版本读数＋那颗检查更新、发现新版本才露出来的那张（更新内容清单与下载进度）、底下那张版本历史。
   会错在哪：① 这一格的所有数只从 state.update 读（当前版本、构建日期、最新、checking/downloading/downloaded、log、进度、checkError）。
                状态的家在 core/store.js，改它的是 main.js 那两个动词；面板自己留一份副本，就会出现"底下说下载中、顶栏那颗已经亮了"；
             ② 版本历史那条"当前版本"不写死版本号：它跟着 state.update 走，
                这里再抄一份 v1.0.0，接上真服务之后界面上下两格说的就不是同一个版本；
             ③ 这一格没有「保存」（原型 :1551：update 与 about 两格把那颗钮藏起来）——它没有延后生效的项，
                留一颗保存反而像"这些勾还没存"；
             ④ 服务说过的话以外，这一格一个字也不许多造。本期 CheckUpdate／DownloadUpdate 如实回
                GAC-UPDATE-501（更新通道要到技术方案 M5 那一站才有），所以：
                  · 查过一次而失败：那一格说的是服务回的那句人话（u.checkError），不许退回「尚未检查」——
                    服务明明答过话，界面说"没查过"就是界面在说假话（原型 :1668 那一句只在真没查过时才出现）；
                  · 版本历史不摆 v0.9.0／v0.5.0 那两条编出来的行，也不摆「本次更新内容」那样的假清单：
                    这台机器上只有一个版本读数，就是 GetVersion 回的那一个。
   怎么回来：检查与下载都走 ctx.checkUpdate() / ctx.downloadUpdate()，跑完之后 main.js 调一次 sync()；
             本面板不改 state，也不自己发请求（发请求归 core/api/client.js 那一个出口）。
             真接上更新通道那天，需要动的只有两处：core/store.js 的 state.update 补上服务回的那几个字段，
             以及这一格把 checkError 那一档换成"有历史就列历史"——编出来的句子不许有第二个家。 */

import { Button } from '../../../../components/button/Button.js';
import { setCard, setRow } from '../card.js';
import { el } from '../../../../utils/dom.js';

export default {
  id: 'update',
  order: 6,
  group: 'info',
  icon: '⭯',
  nameKey: 'settings.nav.update',
  subKey: 'settings.sub.update',
  hasSave: false,
  badge: true,

  build(ctx) {
    const { t } = ctx;
    const u = ctx.state.update;

    const curVerEl = el('div', { class: 'ver-value' });
    const buildEl = el('div', { class: 'ver-value sub' });
    const latestEl = el('div', { class: 'ver-value' });
    const statusEl = el('span', { class: 'update-status' });
    const changelogEl = el('ul', { class: 'changelog' });
    const barEl = el('i');
    const pctEl = el('span');
    const speedEl = el('span');
    const dlProgress = el('div', { class: 'dl-progress' }, [
      el('div', { class: 'dl-bar' }, [barEl]),
      el('div', { class: 'dl-text' }, [pctEl, speedEl])
    ]);

    const checkBtn = Button({
      label: t('settings.update.check'), kind: 'primary',
      onClick: async () => {
        const r = await ctx.checkUpdate();
        if (!r) return;   // 已经在查了：ctx 回 null，这一句就不多说（原型 :1641 那次点击同样没有第二句 toast）
        ctx.toast(r.hasUpdate ? t('settings.update.toastFound', { v: r.latest })
          : t('settings.update.toastLatest', { v: u.current }));
      }
    });
    const dlBtn = Button({
      label: t('settings.update.download'), kind: 'primary',
      onClick: () => ctx.downloadUpdate()
    });

    const detailCard = setCard({
      icon: '✦', title: t('settings.update.changelog'),
      rows: [changelogEl, el('div', { class: 'dl-area' }, [setRow([dlBtn.el]), dlProgress])]
    });

    const historyEl = el('div', { class: 'ver-history' });

    function statusText() {
      if (u.checking) return [t('settings.update.checkingNow'), ''];
      if (u.downloading) return [t('settings.update.downloading'), 'has-new'];
      if (u.downloaded) return [t('settings.update.ready'), 'ok'];
      if (u.hasUpdate) return [t('settings.update.found', { v: u.latest }), 'has-new'];
      if (u.latest) return [t('settings.update.isLatest'), 'ok'];
      // 查过而服务回了那句"本期不提供"：把那句摆在这一格（它排在「尚未检查」之前——查过一次就不算没查）
      if (u.checkError) return [u.checkError, 'err'];
      return [t('settings.update.idle'), ''];
    }

    function sync() {
      curVerEl.textContent = u.current;
      buildEl.textContent = u.buildDate || t('common.dash');
      if (u.checking) latestEl.textContent = t('settings.update.checking');
      else if (!u.latest) latestEl.textContent = t('common.dash');
      else latestEl.textContent = u.latest;
      latestEl.className = 'ver-value' + (u.latest ? ' ' + (u.hasUpdate ? 'latest-new' : 'latest-ok') : '');

      const [text, cls] = statusText();
      statusEl.textContent = text;
      statusEl.className = 'update-status' + (cls ? ' ' + cls : '');
      checkBtn.update({ label: u.checking ? t('settings.update.checking') : t('settings.update.check') });
      checkBtn.setDisabled(u.checking || u.downloading);

      detailCard.style.display = u.hasUpdate ? '' : 'none';
      if (u.hasUpdate) {
        changelogEl.replaceChildren(...u.log.map(line => el('li', { text: line })));
        dlBtn.setDisabled(u.downloading);
        dlBtn.update({
          label: u.downloading ? t('settings.update.downloadingBtn')
            : u.downloaded ? t('settings.update.relaunch') : t('settings.update.download')
        });
        dlProgress.classList.toggle('show', u.downloading || u.downloaded);
        barEl.style.width = `${u.progress}%`;
        pctEl.textContent = `${u.progress}%`;
        if (u.downloading) speedEl.textContent = `${u.speed} MB/s`;   // 下载停下来之后那句速率留在最后一次，跟原型一致
      }

      // 版本历史：这台机器上只有 GetVersion 回的那一个读数，更早的历史归更新通道（本期不给，见文件头④）
      const rows = [
        el('div', { class: 'ver-h-item' }, [
          el('span', { class: 'ver-h-tag current', text: u.current || t('common.dash') }),
          el('span', { class: 'ver-h-date', text: u.buildDate || t('common.dash') }),
          el('span', { class: 'ver-h-desc', text: t('settings.update.hCurrent') })
        ])
      ];
      rows.push(el('div', { class: 'ver-h-item' }, [
        el('span', { class: 'ver-h-desc', text: t('settings.update.historyNone') })
      ]));
      historyEl.replaceChildren(...rows);
    }
    sync();

    return {
      el: el('div', { class: 'settings-pane' }, [
        el('h3', { class: 'pane-title', text: t('settings.update.title') }),
        el('p', { class: 'pane-desc', text: t('settings.update.desc') }),
        setCard({
          rows: [
            el('div', { class: 'ver-grid' }, [
              el('div', { class: 'ver-block' }, [el('div', { class: 'ver-label', text: t('settings.update.current') }), curVerEl]),
              el('div', { class: 'ver-block' }, [el('div', { class: 'ver-label', text: t('settings.update.buildDate') }), buildEl]),
              el('div', { class: 'ver-block' }, [el('div', { class: 'ver-label', text: t('settings.update.latest') }), latestEl])
            ]),
            el('div', { class: 'update-action' }, [checkBtn.el, statusEl])
          ]
        }),
        detailCard,
        setCard({ icon: '▤', title: t('settings.update.history'), rows: [historyEl] })
      ]),
      sync
    };
  }
};
