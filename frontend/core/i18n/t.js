/* 把键换成话。干什么用：core 与界面只说键名（gate.busy.queue），这里负责按当前语言去 locales 里取那一句，
   并把句子里的 {n}、{name} 换成调用方给的参数。
   会错在哪：① 键写了但表里没有——界面会显示成空白或 [object Object]，半天看不出是哪儿少了；
             所以这里缺键就大声：控制台点名下键＋当场语言＋回落语言，界面上留一对 ⟦…⟧ 让人一眼看见哪儿空了。
          ② 参数少给一个：{n} 会原样留在句子里（比静默替换成 undefined 好——看的人立刻知道传漏了）。
          ③ 切到一个表里根本没有的语言：静默用回落那份，界面上"切了没反应"，最难查。所以这里直接抛错。
   怎么回来：本文件不碰构建工具、不碰 DOM，能在 node --test 里直接喂假 locales 跑（tests/i18n.test.mjs）；
             真实的 locales 由 main.js 从 locales/index.js 拿过来注进来，所以 core 层测试不需要 Vite。 */

const MARK = (key) => `\u27e6${key}\u27e7`;   // ⟦键名⟧：界面上看到这对括号，就是这里缺一条

function interpolate(text, params) {
  if (!params) return text;
  return text.replace(/\{([a-zA-Z0-9_]+)\}/g, (whole, name) =>
    params[name] === undefined ? whole : String(params[name]));
}

/** locales 的形状：{ 'zh-CN': { 键: 句子 }, ... }，由 locales/index.js 扫目录得到 */
export function createI18n(locales, { lang = 'zh-CN', fallback = 'zh-CN' } = {}) {
  if (!locales[fallback]) throw new Error(`i18n：回落语言 ${fallback} 不在清单里，可用的是 ${Object.keys(locales).join(', ') || '（空）'}`);
  if (!locales[lang]) throw new Error(`i18n：语言 ${lang} 不在清单里，可用的是 ${Object.keys(locales).join(', ')}`);

  let current = lang;

  function t(key, params) {
    const here = locales[current];
    if (typeof here[key] === 'string') return interpolate(here[key], params);
    const back = locales[fallback][key];
    if (typeof back !== 'string') {
      console.error(`i18n：键 ${key} 在 ${current} 和回落 ${fallback} 里都没有——界面上会显示成 ⟦${key}⟧，去 locales/${fallback}.js 补一句`);
      return MARK(key);
    }
    console.error(`i18n：键 ${key} 在 ${current} 里缺，本次先用回落 ${fallback} 的那句——请在 locales/${current}.js 补上`);
    return interpolate(back, params);
  }

  t.setLang = (id) => {
    if (!locales[id]) throw new Error(`i18n：要切到 ${id}，但 locales/ 下没有 ${id}.js`);
    current = id;
  };
  t.getLang = () => current;
  t.langs = () => Object.keys(locales).sort();

  return t;
}
