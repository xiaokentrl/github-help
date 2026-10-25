/* 首次使用那条横幅（views 层第六块）。干什么用：把外框里写死的那一段（原型 :682-687）收成一个渲染器——
   圆底「i」角标、中间那句带强调的建议、右边「知道了」那颗钮；显示与收掉只有 show()／hide() 两个出口。
   会错在哪：① 那句话被 <b> 断成三段（原型 :683 就是一个 <b> 夹在中间），所以语言文件里也是三段
                （banner.pre / banner.steps / banner.rest）。合成一句就把强调弄丢了，拆开时又少一句；
             ② 显示靠 .show 这一个类（layout.css :62 那条 .banner.show{display:flex}），
                不是 display 内联样式——写内联就把深色主题那几条覆盖关系搅乱了；
             ③ 那颗「知道了」只在本次运行里收掉（跟原型 :3356-3357 一致，没落盘）：
                下次开机还会提一次。要改成"记住不再提示"属于加功能，得先点名。
   怎么回来：容器由 main.js 递进来；这里刻一句中文，tests/views-settings.test.mjs 那把尺报红。 */

import { el, replace } from '../../utils/dom.js';

/**
 * @param {object} d
 * @param {HTMLElement} d.root      外框那一条 #banner
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.handlers       { onClose }
 */
export function createBannerView({ root, t, handlers }) {
  const close = el('button', { text: t('banner.close'), attrs: { type: 'button' } });
  close.addEventListener('click', () => handlers.onClose());

  replace(root, [
    el('span', { class: 'ico', attrs: { 'aria-hidden': 'true' }, text: 'i' }),
    el('span', {}, [
      document.createTextNode(t('banner.pre')),
      el('b', { text: t('banner.steps') }),
      document.createTextNode(t('banner.rest'))
    ]),
    el('div', { class: 'spacer' }),
    close
  ]);

  return {
    el: root,
    show() { root.classList.add('show'); return this; },
    hide() { root.classList.remove('show'); return this; },
    isOpen() { return root.classList.contains('show'); }
  };
}
