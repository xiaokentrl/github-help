/* 「源码里哪些是字符串字面量」这一把尺。干什么用：把注释原地挖成等长空白，再把剩下的引号串一个个抓出来，
   附上"它前面紧贴的是什么"好判断这是哪类字面量。件层（tests/components.test.mjs）与视图层（tests/views-settings.test.mjs）
   量的都是"有没有把界面文案刻死在代码里"这一件事，所以只用这一把尺。
   会错在哪：① 正则一刀切会两头错：「// 键盘没有"松手"这一下」这种行尾注释里的引号会被当成真字面量（误报），
                'https://…' 里的 // 会被当成注释、把后面整段吞掉（漏报）；
             ② 挖掉注释时如果把行也删了，报出来的行号就指错地方，改的人按行号去找找不到。
   怎么回来：按源码位置走一遍：先认串、再认注释，两边都不会看错；挖出来的空白保持原来的长度，行号不动。
             新增需要这把尺的检查层时，从这里 import，别在测试文件里再抄一份。 */

const CJK = /[\u3400-\u9fff]/;

/** 收尾引号的下标：单双引号串不许跨行（跨了就把整份文件吃进去），模板串一路吃到下一个反引号 */
function strEnd(a, i) {
  const q = a[i];
  for (let j = i + 1; j < a.length; j++) {
    if (a[j] === '\\') { j++; continue; }
    if (q !== '`' && a[j] === '\n') return j;
    if (a[j] === q) return j;
  }
  return a.length;
}

/** 把注释原地挖成等长空白（保住行号） */
export function maskComments(src) {
  const a = src.split('');
  const blank = (j, k) => { for (; j < k; j++) if (a[j] !== '\n') a[j] = ' '; };
  let i = 0;
  while (i < a.length) {
    const c = a[i];
    if (c === "'" || c === '"' || c === '`') { i = strEnd(a, i) + 1; continue; }
    if (c === '/' && a[i + 1] === '/') {
      let e = i; while (e < a.length && a[e] !== '\n') e++;
      blank(i, e); i = e; continue;
    }
    if (c === '/' && a[i + 1] === '*') {
      const e = src.indexOf('*/', i + 2);
      const to = e < 0 ? a.length : e + 2;
      blank(i, to); i = to; continue;
    }
    i++;
  }
  return a.join('');
}

/** 剥掉注释后，把剩下的字符串/模板字面量一个个抓出来 */
export function literals(src) {
  const code = maskComments(src);
  const out = [];
  let i = 0;
  while (i < code.length) {
    const q = code[i];
    if (q !== "'" && q !== '"' && q !== '`') { i++; continue; }
    const j = strEnd(code, i);
    out.push({
      body: code.slice(i + 1, j),
      before: code.slice(Math.max(0, i - 14), i),
      line: code.slice(0, i).split('\n').length,        // 注释挖成等长空白，所以这个行号还是源文件的行号
    });
    i = j + 1;
  }
  return out;
}

/** 这一句是不是给开发者看的字（不进界面，改语言用不着动它）：
    ① throw new Error(...)；② 三个 collect.js 共用的那个错误工厂 bad(path, ...)；③ console.log/warn/error。
    除了紧贴括号的那一个参数，还要认下已经填进去的前几个参数（bad(path, 那句就落在第二个位置），
    只认「左边正好是个左括号」会把这类开发用的字当成界面文案报红。
   除此之外，源码里出现汉字字面量就算把界面文案刻死了。 */
export function isDevCopy(lit) {
  return /(?:new Error|\bbad|console\.\w+)\s*\(\s*(?:[\w$]+\s*,\s*)?$/.test(lit.before.replace(/\s+$/, ''));
}

export function cjkLiterals(src) {
  return literals(src).filter(l => CJK.test(l.body) && !isDevCopy(l));
}

export { CJK };
