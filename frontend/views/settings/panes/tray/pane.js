/* 「托盘」这一格（views/settings/panes/tray）。干什么用：原型 :896-912 那两张开关卡片。
   会错在哪：① 这两句现在只是"记下来＋送进服务那一份文件"（core/settings.js 那一层缓存 ＋ main.js 的 persist 送 SaveSettings），
                真正关窗口时该最小化还是退出、托盘图标画不画，归 GTK 壳那边（cmd/gui），那条通道今天还没接。
                所以保存那一句会补半句"本次运行看不出效果"（SettingsView 按 core/settings.js 的 SHELL_KEYS 判）——
                界面不许假装自己已经把窗口藏起来了；
             ② 两个开关都是"点保存才生效"那一类，别在这里顺手 ctx.set，否则「取消」按钮就变成了骗人的。
   怎么回来：值只走 ctx.setDraft；sync() 每次打开时从工作副本补一次，好让上一次没保存的拨动不留在卡上。 */

import { Switch } from '../../../../components/switch/Switch.js';
import { SettingRow } from '../../../../components/setting-row/SettingRow.js';
import { setCard } from '../card.js';
import { el } from '../../../../utils/dom.js';

export default {
  id: 'tray',
  order: 4,
  group: 'prefs',
  icon: '▣',
  nameKey: 'settings.nav.tray',
  subKey: 'settings.sub.tray',

  build(ctx) {
    const { t } = ctx;

    const minimize = Switch({
      checked: !!ctx.get('trayMinimize'),
      ariaLabel: t('settings.tray.minimize'),
      onChange: on => { ctx.setDraft('trayMinimize', on); }
    });
    const showIcon = Switch({
      checked: !!ctx.get('showTrayIcon'),
      ariaLabel: t('settings.tray.icon'),
      onChange: on => { ctx.setDraft('showTrayIcon', on); }
    });

    function sync() {
      minimize.setChecked(!!ctx.get('trayMinimize'));
      showIcon.setChecked(!!ctx.get('showTrayIcon'));
    }
    sync();

    return {
      el: el('div', { class: 'settings-pane' }, [
        el('h3', { class: 'pane-title', text: t('settings.tray.title') }),
        el('p', { class: 'pane-desc', text: t('settings.tray.desc') }),
        setCard({ rows: [SettingRow({
          icon: '▣', title: t('settings.tray.minimize'), desc: t('settings.tray.minimizeDesc'), control: minimize
        }).el] }),
        setCard({ rows: [SettingRow({
          icon: '▤', title: t('settings.tray.icon'), desc: t('settings.tray.iconDesc'), control: showIcon
        }).el] })
      ]),
      sync
    };
  }
};
