/* 「外观」这一格（views/settings/panes/appearance）。干什么用：原型 :871-894 那两张卡片——语言下拉与主题那一行。
   会错在哪：① 语言选项不许在这张面板里列名单。家是两个：可选的语言＝locales/ 扫出来的那几门（t.langs()），
                每门自己的写法＝core/i18n/languages.js。在这里补一句 en-US，切过去就会因为表里没有而当场抛错；
             ② 主题那六个名字的家在 styles/themes/<id>/theme.js，卡片和这句说明都读那一份扫描结果；
                这里再列一遍就是两份名单，加一套主题的人改不到这句句子；
             ③ 语言是"点保存才生效"，主题是"点一下就换"（原型 :1611-1630 与 :3321 那两处的分别）。
                搞反就会出现"选了主题按取消，界面还留着新配色"或"换了语言立刻重画，用户以为重启过了"。
   怎么回来：语言走 ctx.setDraft('language', …)；主题那颗钮走 ctx.openThemePicker()，
             换色由 theme-picker 那边写 state.settings.theme 并落盘，本面板 sync() 只把"当前：某某"这一行补上。 */

import { Select } from '../../../../components/select/Select.js';
import { Button } from '../../../../components/button/Button.js';
import { SettingRow } from '../../../../components/setting-row/SettingRow.js';
import { setCard, setRow } from '../card.js';
import { el } from '../../../../utils/dom.js';

export default {
  id: 'appearance',
  order: 3,
  group: 'prefs',
  icon: '◐',
  nameKey: 'settings.nav.appearance',
  subKey: 'settings.sub.appearance',

  build(ctx) {
    const { t } = ctx;
    const currentLabel = el('span', { class: 'update-status' });

    const language = Select({
      options: ctx.langs(),
      value: ctx.get('language'),
      ariaLabel: t('settings.appearance.language'),
      onChange: v => { ctx.setDraft('language', v); }
    });
    const pickBtn = Button({
      label: t('settings.appearance.pickTheme'),
      onClick: () => ctx.openThemePicker()
    });

    const names = ctx.themes.map(th => th.name);
    const langRow = SettingRow({
      icon: '◐', title: t('settings.appearance.language'), desc: t('settings.appearance.languageDesc'),
      control: language
    });

    function sync() {
      language.update({ value: ctx.get('language') });
      const theme = ctx.themes.find(th => th.id === ctx.state.settings.theme) || ctx.themes[0];
      currentLabel.textContent = t('settings.appearance.themeCurrent', { name: theme.name });
    }
    sync();

    return {
      el: el('div', { class: 'settings-pane' }, [
        el('h3', { class: 'pane-title', text: t('settings.appearance.title') }),
        el('p', { class: 'pane-desc', text: t('settings.appearance.desc') }),
        setCard({ rows: [langRow.el] }),
        setCard({
          icon: '◑', title: t('settings.appearance.theme'),
          desc: t('settings.appearance.themeDesc', { n: names.length, names: names.join('、') }),
          rows: [setRow([currentLabel, pickBtn.el])]
        })
      ]),
      sync
    };
  }
};
