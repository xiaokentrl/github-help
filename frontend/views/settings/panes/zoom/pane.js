/* 「界面缩放」这一格（views/settings/panes/zoom）。干什么用：原型 :852-869 那一张卡片——四档缩放，点一下立刻生效。
   会错在哪：① 那四档（0.9／1／1.1／1.25）的家在 core/settings.js 的 ZOOM_PRESETS，
                normalize() 认的就这四个值；这里另写一份，改档位的人会改漏另一处，
                结果是"面板上有 150% 这一档，存档里存进去却被打回 100%"；
             ② 百分比的字由数值算（Math.round(z*100)），不另写四个键——写了就有两份"110%"；
             ③ 读数那行要在 onChange 里连着 sync() 补一次（跟「布局」那格同一个坑）：
                只刷分段不刷读数，会出现"界面已经放大了、旁边还写 100%"。
   怎么回来：走 ctx.set('zoom', …)：写工作副本、写 state.settings、刷 document.body.style.zoom、落盘，四件在一个出口里做完；
             sync() 只把分段按钮的选中态与那行读数补成当前值。 */

import { Segmented } from '../../../../components/segmented/Segmented.js';
import { ZOOM_PRESETS } from '../../../../core/settings.js';
import { setCard, setRow } from '../card.js';
import { el } from '../../../../utils/dom.js';

const percent = z => `${Math.round(z * 100)}%`;

export default {
  id: 'zoom',
  order: 2,
  group: 'prefs',
  icon: '⤢',
  nameKey: 'settings.nav.zoom',
  subKey: 'settings.sub.zoom',

  build(ctx) {
    const { t } = ctx;
    const label = el('span', { class: 'update-status' });

    const zoom = Segmented({
      options: ZOOM_PRESETS.map(z => ({ value: String(z), label: percent(z) })),
      value: String(ctx.get('zoom')),
      ariaLabel: t('settings.zoom.ratio'),
      onChange: v => { ctx.set('zoom', Number(v)); sync(); }
    });

    function sync() {
      zoom.setValue(String(ctx.get('zoom')));
      label.textContent = percent(ctx.get('zoom'));
    }
    sync();

    return {
      el: el('div', { class: 'settings-pane' }, [
        el('h3', { class: 'pane-title', text: t('settings.zoom.title') }),
        el('p', { class: 'pane-desc', text: t('settings.zoom.desc') }),
        setCard({
          icon: '⤢', title: t('settings.zoom.ratio'), desc: t('settings.zoom.ratioDesc'),
          rows: [setRow([zoom.el, label])]
        })
      ]),
      sync
    };
  }
};
