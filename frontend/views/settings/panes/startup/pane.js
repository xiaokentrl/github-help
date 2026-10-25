/* 「开机自启动」这一格（views/settings/panes/startup）。干什么用：原型 :915-963 那四张开关卡片，
   外加「自动检测已选」关掉时把「自动应用队列」连卡片一起灰掉那一条联动。
   会错在哪：① 那条联动有三半：开关本身禁掉＋那一行连说明一起灰（SettingRow.setDisabled，读屏才知道这句不能改）
                ＋整张卡片加 .disabled（原型 :1521-1528 做了前两件事的另一半）。
                只做前半就会出现"说明那三行还亮着，像还能改"；
             ② 「自动应用」依赖「自动检测」这条规矩的家在 core/settings.js 的 normalize()（第 63 行那一句）。
                这里画的是同一件事的另一半：界面提前灰掉，别等保存之后才发现被 normalization 改回关闭；
             ③ 「登录时自动启动」与「启动后最小化」这两句归 GTK 壳与 systemd 那边，界面这里只把勾存下来
                （缓存＋送服务那一份文件，同 tray 那一格），壳那条通道今天还没接，所以保存那一句会补半句
                "本次运行看不出效果"（core/settings.js 的 SHELL_KEYS）。
                另外两把（自动检测／自动应用）不一样：它们的主人就是这个界面——
                frontend/core/flows.js 的 runStartupAutomation 在开机问到快照、拉回设置之后照这两个值跑，
                而且只跑已勾选的那几条。服务那侧曾经自己起过一整批（2026-10-23 删掉，见 internal/service/trigger.go），
                那时的表现是"界面上开关关着，hosts 每次开机还是被整批改写"。
   怎么回来：四个开关全部 ctx.setDraft（点「保存」才生效）；sync() 从工作副本补四个勾，并跑一次那条联动。 */

import { Switch } from '../../../../components/switch/Switch.js';
import { SettingRow } from '../../../../components/setting-row/SettingRow.js';
import { setCard } from '../card.js';
import { el } from '../../../../utils/dom.js';

export default {
  id: 'startup',
  order: 5,
  group: 'prefs',
  icon: '▶',
  nameKey: 'settings.nav.startup',
  subKey: 'settings.sub.startup',

  build(ctx) {
    const { t } = ctx;

    const autoStart = Switch({
      checked: !!ctx.get('autoStart'),
      ariaLabel: t('settings.startup.autoStart'),
      onChange: on => { ctx.setDraft('autoStart', on); }
    });
    const minimized = Switch({
      checked: !!ctx.get('startMinimized'),
      ariaLabel: t('settings.startup.minimized'),
      onChange: on => { ctx.setDraft('startMinimized', on); }
    });
    const autoDetect = Switch({
      checked: !!ctx.get('autoDetectOnStartup'),
      ariaLabel: t('settings.startup.autoDetect'),
      onChange: on => { ctx.setDraft('autoDetectOnStartup', on); syncDepend(); }
    });
    const autoApply = Switch({
      checked: !!ctx.get('autoApplyOnStartup'),
      ariaLabel: t('settings.startup.autoApply'),
      onChange: on => { ctx.setDraft('autoApplyOnStartup', on); }
    });

    const applyRow = SettingRow({
      icon: '◎', title: t('settings.startup.autoApply'), desc: t('settings.startup.autoApplyDesc'), control: autoApply
    });
    const applyCard = setCard({ rows: [applyRow.el] });

    /** 原型 :1516-1530：检测没开，应用那颗就既按不动、整张卡片也该灰 */
    function syncDepend() {
      const on = !!ctx.get('autoDetectOnStartup');
      if (!on) ctx.setDraft('autoApplyOnStartup', false);
      autoApply.setChecked(on && !!ctx.get('autoApplyOnStartup'));
      autoApply.setDisabled(!on);
      applyRow.setDisabled(!on);            // 这一句管那一行：aria-disabled 与说明一起灰
      applyCard.classList.toggle('disabled', !on);
    }

    function sync() {
      autoStart.setChecked(!!ctx.get('autoStart'));
      minimized.setChecked(!!ctx.get('startMinimized'));
      autoDetect.setChecked(!!ctx.get('autoDetectOnStartup'));
      syncDepend();
    }
    sync();

    return {
      el: el('div', { class: 'settings-pane' }, [
        el('h3', { class: 'pane-title', text: t('settings.startup.title') }),
        el('p', { class: 'pane-desc', text: t('settings.startup.desc') }),
        setCard({ rows: [SettingRow({
          icon: '▶', title: t('settings.startup.autoStart'), desc: t('settings.startup.autoStartDesc'), control: autoStart
        }).el] }),
        setCard({ rows: [SettingRow({
          icon: '▾', title: t('settings.startup.minimized'), desc: t('settings.startup.minimizedDesc'), control: minimized
        }).el] }),
        setCard({ rows: [SettingRow({
          icon: '◎', title: t('settings.startup.autoDetect'), desc: t('settings.startup.autoDetectDesc'), control: autoDetect
        }).el] }),
        applyCard
      ]),
      sync
    };
  }
};
