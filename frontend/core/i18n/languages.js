/* 每门语言自己的写法（简体中文、English、日本語……）。干什么用：设置中心「外观」那一格的下拉要把语言 id 显示成人看得懂的样子，
   这里就是那份对照。名单不在这里——能选哪几门由 core/i18n/locales/ 扫出来的那几份键表决定（t.langs()）。
   会错在哪：① 在这里列一门 locales/ 里没有的语言，切过去会因为表里没有而当场抛错（t.js 第 37 行那句），
                所以"多一门语言"永远是先往 locales/ 放文件，这里再补它的自称；
             ② 自称（endonym）不翻译：English 在中文界面里也写 English，翻成"英文"就变成另一种东西了。
   怎么回来：认不出的 id 就直接把 id 亮出来（宁可看见 zh-HK 这种生面孔，也不要显示成空白让人以为程序坏了）。 */

/** 那份自称表。导出它是为了让用例能逐门查"locales/ 里有这门、这里没写"（tests/views-settings.test.mjs）。 */
export const NATIVE_NAMES = {
  'zh-CN': '简体中文',      // 原型 :877
  'zh-TW': '繁體中文',      // :878
  'en-US': 'English',      // :879
  'ja-JP': '日本語'         // :880
};

/** 给一门语言 id，回它自己的写法；表里没有就用 id 本身 */
export function languageName(id) {
  return NATIVE_NAMES[id] || id;
}

/** 下拉那一批选项：能选哪几门＝locales/ 扫出来的那几门 */
export function languageOptions(langs) {
  return langs.map(id => ({ value: id, label: languageName(id) }));
}
