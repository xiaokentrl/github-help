/* Tag 标签（组件库规划.md 4.7）：短促状态字——「最优」「检测」「应用」「恢复」。
   干什么用：把原型里两族小标（.tag 与 .q-kind）收成一个件，调用方只给文字和哪一档。
   会错在哪：① 文字写死在件里＝"新增一种语言＝只新增一个语言文件"当场作废；
             ② tone 拼错（比如写成 detct）不报错的话，那一格会退回默认底色，六套主题下看着都像"有点不对但说不上来"。
   怎么回来：未知 tone 这里直接抛错；旧字号 10px 只有这两族能引（--fs-legacy-tag），
             新用例必须传 size:'md' 走 11px 起那条规矩（第 10 节第 3 条）。 */

/** tone → 原型的类名；两族都收在这张表里，别处不再各写一遍 */
const TONES = {
  green: 'tag',
  detect: 'q-kind detect',
  apply: 'q-kind apply',
  restore: 'q-kind restore'
};

/**
 * @param {object} o
 * @param {string} o.text        标签上的字（调用方给）
 * @param {'green'|'detect'|'apply'|'restore'} [o.tone]  缺省 green＝候选表那颗「最优」
 * @param {'legacy'|'md'} [o.size]  'legacy'＝照原型 10px（只有这三族能用）；新用例用 'md'＝11px
 * @param {string} [o.title]
 */
/** tone → 原型那一族的类名；不认识就抛错，别让它悄悄退回默认底色 */
function toneOf(o) {
  const tone = o.tone || 'green';
  const base = TONES[tone];
  if (!base) throw new Error(`Tag 不认识 tone「${tone}」，能用的是：${Object.keys(TONES).join(' / ')}`);
  return base;
}

export function Tag(o) {
  // 先查一次参数再动 DOM：这样"tone 写错"这条守卫在 node 里也测得出来，不必起浏览器
  toneOf(o);
  const el = document.createElement('span');

  function paint() {
    const base = toneOf(o);
    el.className = base + (o.size === 'md' ? ' tag-md' : '');
    el.textContent = o.text == null ? '' : String(o.text);
    if (o.title) el.title = o.title; else el.removeAttribute('title');
  }

  paint();
  return {
    el,
    update(next = {}) { Object.assign(o, next); paint(); return this },
    setText(text) { o.text = text; paint(); return this }
  };
}
