/* 顶栏那一行（views 层第五块）。干什么用：把外框里写死的那一段——logo、标题、权限徽标、「检查更新」那颗钮
   （原型 :667-680）——收成一个渲染器；四样东西的字、两句悬停说明、那个"有新版本"的红点全在这里，
   中文一句也不刻，全部走 t()。
   会错在哪：① 权限徽标的字与底色都跟着 state.adminGranted 走（原型 :2212-2221 是两段 innerHTML）。
                这里改成"只换那个文字节点 ＋ 切一个 .warn 类"，一次渲染判一次；别处再判一遍"要不要加 .warn"
                就成两份源，表现是"底色写着没权限、字还写着管理员权限"；
             ② 「检查更新」的红点是同一个按钮上的 .has-update 这一个类（样式在 layout.css :54-55），
                不是第二颗钮，也不用给它单独建元素；
             ③ 原型的 .win-btns（:679）不在这里：真窗口的最小化／最大化／关闭由 GTK 与 Windows 画，
                界面自己画三颗等于给用户三颗假的。
   怎么回来：容器由 main.js 递进来——views 这一层不许出现 getElementById，外框少一个壳由 main.js 当场抛错；
             这里刻了一句中文，tests/views-settings.test.mjs 那把尺就报红。 */

import { state } from '../../core/store.js';
import { Icon } from '../../components/icon/Icon.js';
import { el, replace } from '../../utils/dom.js';

/**
 * @param {object} d
 * @param {HTMLElement} d.root      外框那一条 .topbar
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.handlers       { onCheckUpdate, onPermClick }
 */
export function createTopbarView({ root, t, handlers }) {
  const permText = el('span', { text: t('topbar.permGranted') });
  const perm = el('div', { class: 'perm', title: t('topbar.permTitle') },
    [el('span', { class: 'dot' }), permText]);

  const updateBtn = el('button', { class: 'update-btn', title: t('topbar.checkUpdate'), attrs: { type: 'button' } }, [
    Icon({ name: 'refresh', size: 'sm' }).el,
    el('span', { class: 'lbl', text: t('topbar.checkUpdate') }),
    el('span', { class: 'upd-dot' })
  ]);

  perm.addEventListener('click', () => handlers.onPermClick());
  updateBtn.addEventListener('click', () => handlers.onCheckUpdate());

  // 「GH」是原型那枚角标字符（:668），不是文案：翻成任何语言都还是这两个字母
  replace(root, [
    el('div', { class: 'logo', text: 'GH' }),
    el('h1', { text: t('common.appName') }),
    el('div', { class: 'spacer' }),
    perm,
    updateBtn
  ]);

  function render() {
    permText.textContent = state.adminGranted ? t('topbar.permGranted') : t('topbar.permDenied');
    perm.classList.toggle('warn', !state.adminGranted);
    updateBtn.classList.toggle('has-update', !!state.update.hasUpdate);
  }

  render();
  return { el: root, render, updateBtn };
}
