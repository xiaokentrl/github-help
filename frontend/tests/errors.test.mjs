/* 错误码 ↔ 那一句人话的对账闸（后端目录结构规划第 15 节那条 G13，2026-10-22 定档轮登记、这一轮实装）。
   干什么用：三件事——① 码的集合三方等值：internal/errs/codes.go 那份清单、core/api/errors.js 的参数名单、
   locales/zh-CN.js 里 err.* 那一句，少一条多一条都红；② 句子里用到的 {占位符} 必须在这个码的名单里；
   ③ 占位符还准保证每个出错现场都给得到——给不到的就当场不许用（只有 LOCAL_FILL 那两条例外：那两个数由界面自己补）。
   会错在哪：① Go 新增一个码而这边没跟着加：用户撞到它只剩一句"未知错误"，那正是最难查的一档；
             ② 表里留了一条没人发的句子：翻译的人照表白花力气，界面上永远走不到；
             ③ 句子带上了某个现场根本不给的数：屏幕上原地露出 {path}（t.js 少给参数时不换 undefined，见 core/i18n/t.js），
                这比显示码名更像"界面坏了"。
   怎么回来：红的时候看消息里那个码名。新增一码＝codes.go 加常量并塞进 Codes()、errors.js 的名单加一栏、
             zh-CN.js 配一句，三处同一轮改完；要句子带数，就把那个数在每个现场都递上。
             这一族不在 tests/i18n.test.mjs 的"没人用"扫描里——键名是 `err.${码}` 现场拼的，静态扫描永远扫不到。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { ERROR_PARAMS, errorKey, errorText, replyErrorText } from '../core/api/errors.js';
import zhCN from '../core/i18n/locales/zh-CN.js';
import { createI18n } from '../core/i18n/t.js';

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const REPO = path.resolve(FRONTEND, '..');
const CODES_GO = path.join(REPO, 'internal/errs/codes.go');
const ERRORS_JS = path.join(FRONTEND, 'core/api/errors.js');

/** 界面这一侧补参数的那几栏（与 errors.js 的 LOCAL_FILL 同一份名单——两条各写一遍就会漂，所以从源码里读） */
async function localFill() {
  const src = await readFile(ERRORS_JS, 'utf8');
  const body = src.slice(src.indexOf('const LOCAL_FILL'), src.indexOf('export function errorKey'));
  const out = new Map();
  for (const m of body.matchAll(/'([^']+)':\s*\{([^}]*)\}/g)) {
    out.set(m[1], [...m[2].matchAll(/([A-Za-z0-9_]+)\s*:/g)].map(k => k[1]));
  }
  if (!out.size) throw new Error('errors.js 里读不到 LOCAL_FILL——这一条闸的前提没了，去 core/api/errors.js 看那一支');
  return out;
}

/** codes.go：常量名 ↔ 码，以及 Codes() 清单里那一批 */
async function goCodes() {
  const src = await readFile(CODES_GO, 'utf8');
  const const2code = new Map();
  for (const m of src.matchAll(/\b(Code[A-Za-z0-9]+)\s*=\s*"([^"]+)"/g)) const2code.set(m[1], m[2]);
  const list = src.slice(src.indexOf('func Codes()'));
  const codes = [...list.matchAll(/\b(Code[A-Za-z0-9]+)\b/g)].map(m => m[1]).filter(n => const2code.has(n));
  return { const2code, registered: codes.map(n => const2code.get(n)) };
}

/** 从 map[string]any{...} 那个开括号往后数，只看最里层那一批 "键": ——嵌套的 map 在本项目没有，出现也要红 */
function paramKeysAt(text, openBrace) {
  const keys = [];
  let depth = 0;
  for (let i = openBrace; i < text.length; i += 1) {
    const c = text[i];
    if (c === '{') { depth += 1; continue; }
    if (c === '}') { depth -= 1; if (depth === 0) return keys; continue; }
    if (depth === 1) {
      const m = /^"([A-Za-z0-9_]+)"\s*:/.exec(text.slice(i));
      if (m) { keys.push(m[1]); i += m[0].length - 1; }
    }
  }
  throw new Error('map[string]any{ 没有收尾的花括号');
}

/** 全仓 Go 现场：每个码有一串"这一次给了哪几个数"。nil／裸 fmt.Errorf 都算"这一档一个数也不给"。 */
async function goSites() {
  const { const2code } = await goCodes();
  const files = [];
  const walk = async (dir) => {
    for (const e of await readdir(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (e.name === 'frontend' || e.name === '.git' || e.name === 'node_modules') continue;
        await walk(p);
      } else if (e.name.endsWith('.go') && !e.name.endsWith('_test.go') && p !== CODES_GO) files.push(p);
    }
  };
  await walk(REPO);
  const sites = new Map();   // 码 → [{file, keys:Set}]
  for (const file of files) {
    const text = await readFile(file, 'utf8');
    for (const m of text.matchAll(/errs\.New\(errs\.(Code[A-Za-z0-9]+)\s*,\s*(nil|map\[string\]any\{)/g)) {
      const code = const2code.get(m[1]);
      if (!code) continue;
      const keys = m[2] === 'nil' ? new Set() : new Set(paramKeysAt(text, m.index + m[0].length - 1));
      if (!sites.has(code)) sites.set(code, []);
      sites.get(code).push({ file: path.relative(REPO, file), keys });
    }
  }
  return sites;
}

const placeholders = sentence => [...sentence.matchAll(/\{([A-Za-z0-9_]+)\}/g)].map(m => m[1]);
const sorted = arr => [...new Set(arr)].sort();

test('G13①：codes.go 登记的码、errors.js 的名单、zh-CN 的句子，三份集合一模一样', async () => {
  const { registered } = await goCodes();
  assert.ok(registered.length >= 30, `codes.go 只解析出 ${registered.length} 条码，多半是那份文件的形状变了`);
  const go = sorted(registered);
  const table = sorted(Object.keys(ERROR_PARAMS));
  const locale = sorted(Object.keys(zhCN).filter(k => k.startsWith('err.')).map(k => k.slice(4)));
  assert.deepEqual(table, go, `与 Go 那份清单对不上：只有 Go 有＝${go.filter(c => !table.includes(c))}，只有名单有＝${table.filter(c => !go.includes(c))}`);
  assert.deepEqual(locale, go, `与 Go 那份清单对不上：只有 Go 有＝${go.filter(c => !locale.includes(c))}，只有句子有＝${locale.filter(c => !go.includes(c))}`);
});

test('G13②：每一句里的 {占位符} 都在这个码的参数名单里', () => {
  const bad = [];
  for (const [code, names] of Object.entries(ERROR_PARAMS)) {
    const sentence = zhCN[`err.${code}`];
    if (typeof sentence !== 'string') continue;    // 少一句由上一条闸点名，这里不重复报
    for (const p of placeholders(sentence)) if (!names.includes(p)) bad.push(`${code} 的句子用了 ${p}`);
  }
  assert.deepEqual(bad, [], `这些占位符这个码根本不带，界面上会原地露出花括号：${bad.join('、')}`);
});

test('G13③：句子带的数，这个码每一个 Go 现场都给得到（界面自己补的那两栏除外）', async () => {
  const sites = await goSites();
  const fill = await localFill();
  const bad = [];
  for (const [code, names] of Object.entries(ERROR_PARAMS)) {
    const sentence = zhCN[`err.${code}`];
    if (typeof sentence !== 'string') continue;
    const used = placeholders(sentence);
    if (!used.length) continue;
    const list = sites.get(code);
    const always = list
      ? used.filter(p => !list.every(s => s.keys.has(p)))
      : used;                          // Go 那侧一个 errs.New 现场都没有＝这码的参数只能由界面补
    const local = fill.get(code) || [];
    const miss = always.filter(p => !local.includes(p));
    if (miss.length) bad.push(`${code} 的 ${miss.join('/')} 有现场不给（Go 现场 ${list ? list.length : 0} 处）`);
  }
  assert.deepEqual(bad, [], `这些占位符不能保证每个现场都有，界面上会原地露出花括号：${bad.join('；')}`);
});

test('G13④：参数名单里每一栏真有人在发——Go 现场给过，或由 errors.js 的 LOCAL_FILL 补', async () => {
  const sites = await goSites();
  const fill = await localFill();
  const bad = [];
  for (const [code, names] of Object.entries(ERROR_PARAMS)) {
    if (!names.length) continue;
    const given = new Set([...(sites.get(code) || []).flatMap(s => [...s.keys]), ...(fill.get(code) || [])]);
    const extra = names.filter(n => !given.has(n));
    if (extra.length) bad.push(`${code} 登记了 ${extra.join('/')} 却没人给`);
  }
  assert.deepEqual(bad, [], `这些栏目是空登记（照着它写句子一定会露占位符）：${bad.join('；')}`);
});

test('认不出来的码落到 GAC-UNKNOWN-000 那一句，码名与原文都带进句子', () => {
  const t = createI18n({ 'zh-CN': zhCN });
  assert.equal(errorKey('GAC-MODE-doh-001'), 'err.GAC-UNKNOWN-000');
  const line = errorText(t, 'GAC-MODE-doh-001', {}, '取数那一步没答话');
  assert.match(line, /GAC-MODE-doh-001/);
  assert.match(line, /取数那一步没答话/);
  assert.ok(!line.includes('{'), `句子里还留着占位符：${line}`);
});

test('连不上后台服务那一句：端点给了就说端点，没给也不能露出花括号或空括号', () => {
  const t = createI18n({ 'zh-CN': zhCN });
  assert.match(errorText(t, 'GAC-IPC-040', { endpoint: '/run/user/1000/gac/ipc.sock' }), /\/run\/user\/1000\/gac\/ipc\.sock/);
  for (const params of [{}, { endpoint: '' }, undefined]) {
    const line = errorText(t, 'GAC-IPC-040', params);
    assert.ok(!line.includes('{') && !line.includes('：。'), `兜底没接上：${line}`);
  }
  assert.ok(!replyErrorText(t, { code: 'GAC-HOSTS-007', params: { domain: 'github.com' } }).includes('{'));
});

test('负向：真把一条句子改成用没登记的占位符，这条闸得红', async () => {
  const sites = await goSites();
  const fill = await localFill();
  const cut = { ...zhCN, 'err.GAC-PERM-002': '刷 DNS 没成功（{path}）' };
  const code = 'GAC-PERM-002';
  const list = sites.get(code);
  const always = placeholders(cut[`err.${code}`]).filter(p => !list || !list.every(s => s.keys.has(p)));
  assert.deepEqual(always.filter(p => !(fill.get(code) || []).includes(p)), ['path'],
    '这条漂移没红：G13③ 那条比对被改松了，去 tests/errors.test.mjs 看那一支');
});
