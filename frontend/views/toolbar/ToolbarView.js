/* 工具栏那一行（views 层第七块）。干什么用：把外框里写死的那一排（原型 :689-704）收成一个渲染器——
   两颗互斥钮（全选／折叠）、一条竖分隔、搜索框、模式下拉、 Spacer、三颗大钮，
   末尾再挂上那条全局进度线（件在 main.js 建，位置在这一行末尾：它靠 .toolbar 下沿绝对定位）。
   会错在哪：① 三颗大钮的禁用条件只在 core/task/gate.js 的 isBusy() 里算一次（已定：C2 就是当年判漏了）。
                这里再写一遍"执行中就禁用"，将来加一种忙碌态就有一处漏；
             ② 两颗互斥钮的文字与悬停说明跟着"当前是不是全选／全折叠"走，判据在 core/store.js
                （allCheckedState／allCollapsedState），本文件只读结果、不算算法；
             ③ 模式下拉那两项"系统代理／PAC"是 disabled 的占位项（原型 :697-698），本文件不给它们接事件：
                解析方式换档属后端那一侧（技术方案第 12 节），界面上今天点不动是对的；
             ④ 进度线在这里只是"贴在末尾"，它显示什么数由队列喂（views/queue 那一块）。
                谁都能 appendChild 一次＝同一行里两根线。
   怎么回来：容器由 main.js 递进来；搜索框输入的过滤延迟也留在 main.js（原型 :3097 那 150 毫秒）；
             这里刻一句中文，tests/views-settings.test.mjs 那把尺报红。 */

import { state, allCheckedState, allCollapsedState } from '../../core/store.js';
import { isBusy } from '../../core/task/gate.js';
import { Button } from '../../components/button/Button.js';
import { TextField } from '../../components/text-field/TextField.js';
import { Select } from '../../components/select/Select.js';
import { el, replace } from '../../utils/dom.js';

/**
 * @param {object} d
 * @param {HTMLElement} d.root         外框那一条 #toolbar
 * @param {HTMLElement} [d.progress]   ProgressBar({kind:'global'}) 建的那根线（件实例由 main.js 持有，因为队列要喂它）
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.handlers          { onSelectToggle, onInvert, onFoldToggle, onSearch, onDetectAll, onApplyQueue, onRestore }
 */
export function createToolbarView({ root, progress, t, handlers }) {
  const selectToggle = Button({ label: t('toolbar.selectAll'), title: t('toolbar.selectAllTitle'), onClick: () => handlers.onSelectToggle() });
  const invert = Button({ label: t('toolbar.invert'), onClick: () => handlers.onInvert() });
  const foldToggle = Button({ label: t('toolbar.fold'), title: t('toolbar.foldTitle'), onClick: () => handlers.onFoldToggle() });

  const search = TextField({
    placeholder: t('toolbar.searchPlaceholder'),
    clearTitle: t('toolbar.searchClearTitle'),
    onInput: value => handlers.onSearch(value)
  });

  const mode = Select({
    size: 'mode',
    value: 'hosts',
    ariaLabel: t('toolbar.modeLabel'),
    options: [
      { value: 'hosts', label: t('toolbar.modeHosts') },
      { value: 'proxy', label: t('toolbar.modeProxy'), disabled: true },
      { value: 'pac', label: t('toolbar.modePac'), disabled: true }
    ]
  });

  const detectAll = Button({ label: t('toolbar.detectAll'), onClick: () => handlers.onDetectAll() });
  const applyQueue = Button({ label: t('toolbar.applyQueue'), kind: 'primary', onClick: () => handlers.onApplyQueue() });
  const restore = Button({ label: t('toolbar.restore'), kind: 'danger', onClick: () => handlers.onRestore() });

  replace(root, [
    selectToggle.el, invert.el, foldToggle.el,
    el('div', { class: 'divider-v' }),
    search.el, mode.el,
    el('div', { class: 'spacer' }),
    detectAll.el, applyQueue.el, restore.el
  ]);
  if (progress) root.appendChild(progress);

  function render() {
    const busy = isBusy(state);
    detectAll.update({ disabled: busy });
    applyQueue.update({ disabled: busy });
    restore.update({ disabled: busy });

    const allChecked = allCheckedState();
    selectToggle.update({
      label: allChecked ? t('toolbar.selectNone') : t('toolbar.selectAll'),
      title: allChecked ? t('toolbar.selectNoneTitle') : t('toolbar.selectAllTitle')
    });

    const allFolded = allCollapsedState();
    foldToggle.update({
      label: allFolded ? t('toolbar.unfold') : t('toolbar.fold'),
      title: allFolded ? t('toolbar.unfoldTitle') : t('toolbar.foldTitle')
    });
  }

  render();
  return { el: root, render, search, mode, buttons: { selectToggle, invert, foldToggle, detectAll, applyQueue, restore } };
}
