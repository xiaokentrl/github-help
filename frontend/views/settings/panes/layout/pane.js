/* 「布局」这一格（views/settings/panes/layout）。干什么用：把原型 :791-848 那四张卡片搬过来——
   主菜单宽度、队列高度这两档"选一档立刻生效"，检测／应用时焦点跟随这两个开关"点保存才生效"。
   会错在哪：① 宽度和高度的档位数字（240/340/480、180/252/360）的家在 core/settings.js 那两组预置里，
                拖动把手用的上下限也是它；这里再写一份，就会出现"分段说 200–600、拖到 600 就停"；
             ② 队列折叠着的时候改高度，界面不动（原型 :1592 那句先展开队列再写高度）——
                漏了这一步就是"点了大档，队列还是那条细缝"；
             ③ 那两个焦点跟随开关归流程那边读（core/flows.js 问 store 里的这两个值），
                本面板只负责把它写进设置，不在这里改队列行为；
             ④ 右边那行"240 px"的读数跟分段是两件东西：只在 onChange 里连着 sync() 补一次才会跟着变。
                漏了这一步＝宽度已经改了、旁边那行还写旧数，看着像没生效。
   怎么回来：立刻生效那两档走 ctx.set（写盘＋刷界面都在这一个出口），延后生效那两个开关走 ctx.setDraft（只改工作副本，
             点「保存」才落到 state.settings）。sync() 每次打开设置中心都要调，把外面（拖动把手）改出来的值补回分段。
             形状上两档卡片跟开关卡片不一样，这不是随手来的：开关走组件库规划第 5 节的 SettingRow（标题在左、控件在右，
             点标题就能聚焦那颗开关）；分段那一档的控件是"三个钮＋一个读数"，SettingRow 的 control 只收开关／复选／
             下拉／输入框，硬塞进去会让标题挂到分段那颗钮上（点标题＝改一次宽度），所以那两张按原型上下两截排。 */

import { Segmented } from '../../../../components/segmented/Segmented.js';
import { Switch } from '../../../../components/switch/Switch.js';
import { SettingRow } from '../../../../components/setting-row/SettingRow.js';
import { SIDEBAR_PRESETS, QUEUE_PRESETS, SIDEBAR_MIN_W, SIDEBAR_MAX_W } from '../../../../core/settings.js';
import { setCard, setRow } from '../card.js';
import { el } from '../../../../utils/dom.js';

const SIDEBAR_LABELS = ['settings.size.narrow', 'settings.size.standard', 'settings.size.wide'];
const QUEUE_LABELS = ['settings.size.small', 'settings.size.middle', 'settings.size.large'];

export default {
  id: 'layout',
  order: 1,
  group: 'prefs',
  icon: '▦',
  nameKey: 'settings.nav.layout',
  subKey: 'settings.sub.layout',

  build(ctx) {
    const { t } = ctx;
    const widthLabel = el('span', { class: 'update-status' });
    const heightLabel = el('span', { class: 'update-status' });

    const sidebar = Segmented({
      options: SIDEBAR_PRESETS.map((px, i) => ({ value: String(px), label: t(SIDEBAR_LABELS[i]) })),
      value: String(ctx.get('sidebarWidth')),
      ariaLabel: t('settings.layout.sidebar'),
      onChange: v => { ctx.set('sidebarWidth', Number(v)); sync(); }
    });
    const queue = Segmented({
      options: QUEUE_PRESETS.map((px, i) => ({ value: String(px), label: t(QUEUE_LABELS[i]) })),
      value: String(ctx.get('queueHeight')),
      ariaLabel: t('settings.layout.queue'),
      onChange: v => { ctx.set('queueHeight', Number(v)); ctx.expandQueue(); sync(); }
    });

    const detectFollow = Switch({
      checked: !!ctx.get('detectFocusFollow'),
      ariaLabel: t('settings.layout.detectFollow'),
      onChange: on => { ctx.setDraft('detectFocusFollow', on); }
    });
    const applyFollow = Switch({
      checked: !!ctx.get('applyFocusFollow'),
      ariaLabel: t('settings.layout.applyFollow'),
      onChange: on => { ctx.setDraft('applyFocusFollow', on); }
    });

    function sync() {
      sidebar.setValue(String(ctx.get('sidebarWidth')));
      queue.setValue(String(ctx.get('queueHeight')));
      widthLabel.textContent = `${ctx.get('sidebarWidth')} px`;
      heightLabel.textContent = `${ctx.get('queueHeight')} px`;
      detectFollow.setChecked(!!ctx.get('detectFocusFollow'));
      applyFollow.setChecked(!!ctx.get('applyFocusFollow'));
    }
    sync();

    return {
      el: el('div', { class: 'settings-pane' }, [
        el('h3', { class: 'pane-title', text: t('settings.layout.title') }),
        el('p', { class: 'pane-desc', text: t('settings.layout.desc') }),
        setCard({
          icon: '▤', title: t('settings.layout.sidebar'),
          desc: t('settings.layout.sidebarDesc', { min: SIDEBAR_MIN_W, max: SIDEBAR_MAX_W }),
          rows: [setRow([sidebar.el, widthLabel])]
        }),
        setCard({
          icon: '▥', title: t('settings.layout.queue'), desc: t('settings.layout.queueDesc'),
          rows: [setRow([queue.el, heightLabel])]
        }),
        el('h3', { class: 'pane-title mt', text: t('settings.layout.detectHead') }),
        el('p', { class: 'pane-desc', text: t('settings.layout.detectDesc') }),
        // SettingRow 只管那一行长什么样；边框、内边距、卡片之间的间距还是 .set-card 给的（原型 :374）
        setCard({ rows: [SettingRow({
          icon: '◎', title: t('settings.layout.detectFollow'), desc: t('settings.layout.detectFollowDesc'),
          control: detectFollow
        }).el] }),
        setCard({ rows: [SettingRow({
          icon: '◎', title: t('settings.layout.applyFollow'), desc: t('settings.layout.applyFollowDesc'),
          control: applyFollow
        }).el] })
      ]),
      sync
    };
  }
};
