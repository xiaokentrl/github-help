/* 把扫描到的一份份语言文件整理成 { 语言id: 键表 } 交给 t.js。放在这里是因为它不碰构建工具、不碰 DOM，
   能在 node --test 里直接喂假 scanned 跑。
   干什么用：`index.js` 管扫目录，本文件管认名字、查形状、保证回落那一份（zh-CN）在。
   会错在哪：① 语言文件名写成 `zh_CN.js` 或 `简体中文.js`：界面按 id 找不到就用回落那份，
                看着像"切了语言没反应"，而代码一句错也没报；
             ② 键表里混进非字符串（写成对象或函数）：t() 拿到对象往界面上贴，显示成 [object Object]；
             ③ 回落那份整个漏了：任何语言缺一个键就无处可退，界面上出现一片 ⟦键名⟧。
   怎么回来：三件都当场抛错，不静默。 */

const LANG = /^[a-z0-9]+(-[a-z0-9]+)*$/i;      // zh-CN / en / pt-BR 这种；下划线与中文一律不认

function bad(path, msg) {
  return new Error(`语言文件 ${path}：${msg}`);
}

export function collectLocales(scanned, fallback = 'zh-CN') {
  const entries = Object.entries(scanned);
  if (entries.length === 0) throw new Error('locales/ 下一个语言文件也没扫到：检查 index.js 的扫描路径还对不对');

  const locales = {};
  for (const [path, mod] of entries) {
    const id = (path.match(/^\.\/([^/]+)\.js$/) || [])[1];
    if (!id) throw bad(path, '不在 locales/<语言id>.js 这个形状上，扫描结果没法认出它是哪种语言');
    if (!LANG.test(id)) throw bad(path, `语言 id「${id}」得是 zh-CN 这种形状（小写字母数字加连字符），文件名就是界面里存的值`);
    const dict = mod && mod.default !== undefined ? mod.default : mod;
    if (!dict || typeof dict !== 'object' || Array.isArray(dict)) throw bad(path, '没有默认导出一份键表对象');
    for (const [key, value] of Object.entries(dict)) {
      if (typeof value !== 'string') throw bad(path, `键 ${key} 的值不是字符串（现在是 ${typeof value}），贴到界面上会显示成 [object Object]`);
    }
    locales[id] = dict;
  }

  if (!locales[fallback]) throw bad(`./${fallback}.js`, '回落语言这份不存在：任何语言缺一个键都会无处可退');
  return locales;
}
