/* 前端与 Go 服务之间的接头检查。干什么用：一条命令现读 Go 那边的常量与 json tag，跟界面这一侧的事件名清单、字段形状逐字对账；
   再量两件"名字都对但线没通"的：那 18 个方法有没有人调、那 5 条事件有没有人接。
   会错在哪：两侧名字漂了不会有人报错，只会界面上"队列跑完了还停在上一次"、某个字段读到 undefined 显示成空白；这类问题靠人眼查最慢。
             而"注册齐了却没人调用"连名字都不漂——2026-10-23 那轮查出的 GetSettings／SaveSettings 就是这样，
             设置存不进服务那一份文件，开机开关成了摆设，全套名字检查一路绿。
   怎么回来：少一条、多一条、拼错一个字母都当场红，消息里写清是哪一个。Go 侧自己的拼写钉在 internal/contract/contract_test.go。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const GO_CONTRACT = path.join(ROOT, 'internal', 'contract', 'contract.go');

async function goConstants(prefix) {
  const src = await readFile(GO_CONTRACT, 'utf8');
  const re = new RegExp(`\\b${prefix}[A-Za-z]+\\s+=\\s+"([^"]+)"`, 'g');
  return [...src.matchAll(re)].map(m => m[1]);
}

async function goJsonTags() {
  const src = await readFile(GO_CONTRACT, 'utf8');
  return [...src.matchAll(/`json:"([A-Za-z]+)(?:[^"]*)"`/g)].map(m => m[1]);
}

test('界面订阅的事件名 = Go 侧发射的事件名，一条不多一条不少', async () => {
  const { IPC_EVENTS } = await import('../core/contract/events.js');
  const go = await goConstants('Event');
  assert.ok(go.length >= 4, `Go 侧只解出 ${go.length} 个 Event 常量，八成是那边的写法变了，这个检查要跟着改`);
  assert.deepEqual(Object.values(IPC_EVENTS).sort(), go.slice().sort(),
    '事件名两边对不上：漂的那条不会报错，只会界面收不到');
  assert.equal(go.length, 5, 'Go 侧事件应为 5 条：冻结的 4 条 + 4A 之后的 detect:progress');
});

/* 界面将来要按这 18 个方法调用：11 条契约原表 + CancelTask + 队列暂停两条 + 联调轮补的 GetSnapshot、ClearTasks
   + 2026-10-25 补的 OpenURL（「关于我们」三颗入口钮接真）与 OpenHosts（工具栏「打开Hosts」）。
   数字写在这里不是为了让它好看：Go 那边加一条方法、这边不跟着改，界面那颗新按钮就点下去没反应（通道回 GAC-IPC-011）。 */
test('Go 侧方法应是冻结的 18 条（11 条契约 + CancelTask + 队列暂停两条 + GetSnapshot／ClearTasks + OpenURL＋OpenHosts）', async () => {
  const go = await goConstants('Method');
  assert.equal(go.length, 18, `界面将来要按这 18 个方法调用，Go 侧现在是 ${go.length} 个：${go.join(', ')}`);
  assert.equal(new Set(go).size, 18, 'Go 侧方法名有重复');
});

/* 方法名／走线名／版本号：界面那三个名单与 Go 那三个名单必须逐字相同。
   干什么用：methods.js 顶上写明"这三个名单由本文件比对"，可上一轮它一只字也没比——那句承诺是空的。
   会错在哪：Go 改了名（或加了方法）而这里没跟着改，症状分别是"每个按钮石沉大海"／"那一条永远空着且不报错"，
             都属于靠人眼最慢的那一类；版本号只在一侧推，则是缺的那一栏静默留空。
   怎么回来：红了就去改 internal/contract/contract.go 那一侧的真源，同轮把 methods.js 与这里对齐。 */
test('界面 METHODS／WIRE／版本号 与 Go 侧逐字相同（版本号漂了不许静默少一栏）', async () => {
  const src = await readFile(GO_CONTRACT, 'utf8');
  const { METHODS, WIRE, EXPECTED_PROTOCOL } = await import('../core/contract/methods.js');

  const goValues = prefix =>
    [...src.matchAll(new RegExp(`\\b${prefix}[A-Za-z]+\\s+=\\s+"([^"]+)"`, 'g'))].map(m => m[1]);
  const sorted = a => [...a].sort();
  const same = (a, b) => JSON.stringify(sorted(a)) === JSON.stringify(sorted(b));

  const goMethods = goValues('Method');
  assert.ok(goMethods.length === 18 && Object.keys(METHODS).length === 18,
    `方法两边数量不对：Go ${goMethods.length}／界面 ${Object.keys(METHODS).length}`);
  assert.ok(same(goMethods, Object.values(METHODS)),
    `方法名两边对不上：Go 有 ${sorted(goMethods).join(', ')}；界面有 ${sorted(Object.values(METHODS)).join(', ')}`);

  const goWire = goValues('Wire');
  assert.ok(same(goWire, Object.values(WIRE)),
    `走线名两边对不上：Go 有 ${sorted(goWire).join(', ')}；界面有 ${sorted(Object.values(WIRE)).join(', ')}`);

  const goVersion = Number(src.match(/\bconst ProtocolVersion\s*=\s*(\d+)\b/)?.[1]);
  assert.ok(Number.isInteger(goVersion), 'Go 侧没解出 ProtocolVersion 那个数，这个检查得跟着那边的写法改');
  assert.equal(EXPECTED_PROTOCOL, goVersion,
    `界面按 v${EXPECTED_PROTOCOL} 写的，服务是 v${goVersion}：少的那一栏在界面上永远空着且不报错`);
});

/**
 * 状态词表跨语言对账。干什么用：Go 那边写进快照的状态串，必须正好是界面 core/meta.js 与
 * core/task/model.js 认得的那一批——它是界面拿去查文案的 key。
 * 会错在哪：两边各改一个字母不会有人报错，只会那一档永远显示不出来（点"应用"卡片还写着"未替换"）。
 * 怎么回来：少一条、多一条、拼错一个字都当场红，红的那条写明是哪一组。
 */
test('Go 侧的状态串 = 界面状态字典的键名，四组逐一对上', async () => {
  const src = await readFile(GO_CONTRACT, 'utf8');
  const { ItemMeta, TaskMeta, PHASES } = await import('../core/meta.js');
  const { STATUS, KINDS } = await import('../core/task/model.js');

  const goGroup = prefix => [...src.matchAll(new RegExp(`\\b${prefix}[A-Za-z]+\\s+=\\s+"([^"]+)"`, 'g'))].map(m => m[1]);
  const jsKeys = o => Object.keys(o);
  const same = (a, b) => JSON.stringify([...a].sort()) === JSON.stringify([...b].sort());

  const pairs = [
    ['条目状态', goGroup('Item'), jsKeys(ItemMeta)],
    ['任务状态', goGroup('Task'), jsKeys(TaskMeta)],
    ['任务种类（model.js 的 STATUS）', goGroup('Task'), STATUS],
    ['任务种类（KINDS）', goGroup('Kind'), KINDS],
    ['写入阶段（PHASES 的 key）', goGroup('Phase'), PHASES.map(p => p.key)]
  ];
  for (const [name, go, js] of pairs) {
    assert.ok(go.length > 0 && js.length > 0, `${name} 这一组有一侧是空的，八成是解析方式变了`);
    assert.ok(same(go, js), `${name} 两边对不上：Go 有 ${go.join(', ')}；界面有 ${js.join(', ')}`);
  }
});

/* 契约那 18 个方法，每一个都得真有人在 core/api/ 之外叫它。干什么用：
   这一条挡的是 2026-10-23 那轮查出来的那种洞——GetSettings／SaveSettings 两边都注册好了、名字也对得上，
   可是界面一处也没调用：服务那一份设置文件于是永远没人写，高权限服务开机读的是默认值，
   界面上两把开机开关关了也白关。上面那几条"名字两边一致"全绿，一个字的错也没犯，功能却是死的。
   会错在哪：只看名字对不上是查不出"没人调用"的；方法名齐、动词齐、调用点为零，症状是"设置了但没生效"。
   怎么回来：新加一个契约方法那一轮，除了改三处名单，还要在这里点名它由谁调用——
             红了就是告诉你"这条线铺好了还没人走"，要么接上，要么别加。 */
test('契约那 18 个方法，每一个在 core/api/ 之外至少有一个调用点', async () => {
  const { METHODS } = await import('../core/contract/methods.js');
  const files = await jsSources(path.join(ROOT, 'frontend'));
  const callSites = name => {
    const re = new RegExp(`\\bapi\\.${name}\\s*\\(`);
    return files.filter(f => !f.rel.includes('core/api/') && re.test(f.src));
  };
  const dead = Object.keys(METHODS).filter(verb => callSites(verb).length === 0);
  assert.deepEqual(dead, [],
    `这些契约方法注册了却没人调用：${dead.join(', ')}——线铺好了没走，界面说的就是没生效的话`);
});

/* 五条业务事件每一条都得真有人接。干什么用：上一那条管"发出去"，这一条管"收进来"：
   事件名两边一致、名字也订阅了，但 main.js 那一段 handlers 少写一行，症状同样是"那一栏永远停在上一秒"。
   会错在哪：subscribe() 只认 core/contract/events.js 那一份名单，漏接的那条不会报错，
             因为"没人订"与"服务没发"在界面上是同一个样子。
   怎么回来：加一条事件那一轮，同轮在这里点名接它的那一支（现在五条全接在 flows.onEvent 上，
             因为每条事件自带整份快照）。 */
test('五条业务事件在 main.js 里全都有人接（订了名字不等于接了处理）', async () => {
  const { IPC_EVENTS } = await import('../core/contract/events.js');
  const mainSrc = await readFile(path.join(ROOT, 'frontend', 'main.js'), 'utf8');
  const missing = Object.keys(IPC_EVENTS).filter(key => !new RegExp(`\\b${key}:\\s*flows\\.onEvent`).test(mainSrc));
  assert.deepEqual(missing, [],
    `这些事件的名字订了但 main.js 没把处理函数接上：${missing.join(', ')}`);
});

/** 递归读 frontend/ 下所有 .js 源码（跳过 dist/ 那一份构建产物与 tests/ 这里自己）。
    为什么要现场读而不是 import：这里查的是"谁调用了谁"，import 进来的模块图只看得到真走了的那条路。 */
async function jsSources(dir, out = []) {
  const { readdir } = await import('node:fs/promises');
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    if (entry.name === 'dist' || entry.name === 'node_modules' || entry.name === 'tests') continue;
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) await jsSources(p, out);
    else if (entry.name.endsWith('.js')) out.push({ rel: path.relative(ROOT, p), src: await readFile(p, 'utf8') });
  }
  return out;
}

test('types.d.ts 的字段名与 Go 的 json tag 完全一致（appliedIp 不许写成 appliedIP）', async () => {
  const [dts, tags] = await Promise.all([
    readFile(path.join(ROOT, 'frontend', 'core', 'contract', 'types.d.ts'), 'utf8'),
    goJsonTags()
  ]);
  const declared = [...dts.matchAll(/^\s{2}([A-Za-z]+)\??\s*:/gm)].map(m => m[1]);
  assert.ok(declared.length >= 20, `types.d.ts 只解出 ${declared.length} 个字段，形状变了要跟着改这个解析`);
  const missing = tags.filter(t => !declared.includes(t));
  const extra = declared.filter(d => !tags.includes(d));
  assert.deepEqual(missing, [], `界面这一侧缺这些字段：${missing.join(', ')}（Go 那边有，界面读到会是 undefined）`);
  assert.deepEqual(extra, [], `界面这一侧多出来这些字段：${extra.join(', ')}（Go 那边没有，等于凭空造字段）`);
  assert.doesNotMatch(declared.join('\n'), /appliedIP|prevAppliedIP/,
    '界面这一侧出现了大写 IP 的字段名：后端给的是 appliedIp，第 52 块冻结的就是这个拼法');
});

/* 设置那 13 项的键名与默认值：界面一份、Go 契约结构体一份、Go 服务默认值一份，三份必须同一轮改。
   干什么用：设置有两个家（服务文件是真家、界面 localStorage 只是"先画对"），而这两份各写了一遍 13 个键。
   会错在哪：① 键名漂一个字母——服务那边收不到这一项，界面重开还是旧值，回执却照样说"保存好了"（不报错）；
             ② 默认值漂一个数——AutoDetectOnStartup 一侧 true、一侧 false，
                界面上那颗开关显示"开"，服务按"关"办，症状是"勾了没生效"，靠人眼最慢；
             ③ 加一项只改一侧——types.d.ts 那条比的是字段形状，比不出默认值，这一条补的就是那半个空。
   怎么回来：红了同轮改三处：frontend/core/settings.js 的 DEFAULTS、internal/contract/contract.go 的 Settings、
             internal/settings/settings.go 的 Defaults()。真源是最后一个，前两个跟着它。 */
test('设置 13 个键名与默认值：界面 DEFAULTS = Go 契约 Settings = Go Defaults()', async () => {
  const { DEFAULTS } = await import('../core/settings.js');
  const [contractSrc, settingsSrc] = await Promise.all([
    readFile(GO_CONTRACT, 'utf8'),
    readFile(path.join(ROOT, 'internal', 'settings', 'settings.go'), 'utf8')
  ]);

  const body = (src, head) => {
    const i = src.indexOf(head);
    assert.notEqual(i, -1, `Go 源码里找不到「${head}」这一段，这份对照得跟着那边的写法改`);
    const rest = src.slice(i + head.length);
    const end = rest.indexOf('\n}');
    assert.notEqual(end, -1, `「${head}」这一段找不到结尾`);
    return rest.slice(0, end);
  };

  // stored 结构体：Go 字段名 → json 键名（文件里与服务回执用的都是这一个拼法）
  const fieldOf = new Map();
  for (const m of body(settingsSrc, 'type stored struct {').matchAll(/^\s+([A-Z][A-Za-z]+)\s+\*?[^\s`]+\s+`json:"([A-Za-z]+)"/gm)) {
    fieldOf.set(m[1], m[2]);
  }
  assert.ok(fieldOf.size >= 13, `Go 的 stored 结构体只解出 ${fieldOf.size} 个带 json 标签的字段，解析方式要跟着改`);

  const goDefaults = new Map();
  for (const m of body(settingsSrc, 'func Defaults() Settings {').matchAll(/^\s+([A-Z][A-Za-z]+):\s+([^,\n]+),/gm)) {
    const key = fieldOf.get(m[1]);
    assert.ok(key, `Defaults() 里的 ${m[1]} 在 stored 结构体里没有对应的 json 标签`);
    const raw = m[2].trim();
    goDefaults.set(key, raw.startsWith('"') ? raw.slice(1, -1) : raw === 'true' ? true : raw === 'false' ? false : Number(raw));
  }
  assert.equal(goDefaults.size, 13, `Go 的 Defaults() 只解出 ${goDefaults.size} 项，应当是 13 项`);

  const contractKeys = [...body(contractSrc, 'type Settings struct {')
    .matchAll(/^\s+[A-Z][A-Za-z]+\s+[^\s`]+\s+`json:"([A-Za-z]+)"/gm)].map(m => m[1]);
  assert.equal(contractKeys.length, 13, `契约的 Settings 结构体只解出 ${contractKeys.length} 个键`);

  const js = Object.keys(DEFAULTS).sort();
  const same = (a, b) => JSON.stringify([...a].sort()) === JSON.stringify([...b].sort());
  assert.ok(same(js, contractKeys),
    `设置键名两边对不上：界面有 ${js.join(', ')}；契约有 ${contractKeys.slice().sort().join(', ')}（漂一个字母＝服务收不到那一项，界面重开还是旧值，回执却照样说保存好了）`);
  assert.ok(same(js, [...goDefaults.keys()]),
    `设置键名两边对不上：界面有 ${js.join(', ')}；服务默认值有 ${[...goDefaults.keys()].sort().join(', ')}`);

  for (const key of js) {
    assert.ok(goDefaults.has(key), `界面有 ${key}，Go 的 Defaults() 里没有那一项`);
    assert.equal(DEFAULTS[key], goDefaults.get(key),
      `${key} 的默认值两边不一样：界面 ${JSON.stringify(DEFAULTS[key])}／服务 ${JSON.stringify(goDefaults.get(key))}——`
      + '界面上那颗开关显示"开"、服务按"关"办，症状就是"勾了没生效"，而且一个字也不报错');
  }
});

/* SHELL_KEYS 那四项（trayMinimize／showTrayIcon／autoStart／startMinimized）今天没有任何人执行。
   干什么用：把"只有程序外壳能动、界面上只是存个勾"这件事钉成一条会红的检查。
   会错在哪：将来有人在 flows.js 或 views 里写 `if (state.settings.autoStart) {...}`，
             看着像实现了，其实那条通道（外壳与 systemd）根本不存在——本轮删掉的正是这类"名字都对但线没通"的东西；
             反过来，外壳真接上那天，这条会红，逼着写代码的人同轮把 main.js 顶上那句"还没接"改掉。
   怎么回来：接上那天把这四项从这份名单里删掉，并在 cmd/gui 那侧补一条真消费者的证据（用例消息里也是这么写的）。 */
test('那四把只有外壳能动的开关，界面里除设置那一格之外不许有人拿它做事', async () => {
  const { SHELL_KEYS } = await import('../core/settings.js');
  assert.deepEqual(SHELL_KEYS.slice().sort(),
    ['autoStart', 'showTrayIcon', 'startMinimized', 'trayMinimize'],
    'SHELL_KEYS 这份名单变了：要么外壳接上了其中一把（那要同轮改 main.js 顶上那句"还没接"），要么就是把不该进来的键混进来了');

  // 允许出现这份键的地方只有这几处：名单的家、设置外壳的提示那一句、那两格面板、语言表、main.js 的说明
  const ALLOW = [
    'frontend/core/settings.js',
    'frontend/views/settings/SettingsView.js',
    'frontend/views/settings/panes/tray/pane.js',
    'frontend/views/settings/panes/startup/pane.js',
    'frontend/core/i18n/locales/zh-CN.js',
    'frontend/main.js'
  ];
  const hits = [];
  for (const { rel, src } of await jsSources(path.join(ROOT, 'frontend'))) {
    if (ALLOW.includes(rel)) continue;
    // 认的是"这个键名出现在代码里"，连解构那种写法（const { autoStart } = state.settings）一起抓——
    // 只认 settings.autoStart 这一种写法的话，换一种拼法就漏过去了，那条闸等于没关。
    // 纯注释行（/* 、* 、// 开头）跳过：这里查的是有没有人拿它做事，不是有没有人提到它。
    for (const key of SHELL_KEYS) {
      for (const line of src.split('\n')) {
        const trimmed = line.trim();
        if (trimmed.startsWith('/*') || trimmed.startsWith('*') || trimmed.startsWith('//')) continue;
        if (!new RegExp(`\\b${key}\\b`).test(trimmed)) continue;
        hits.push(`${rel}: ${trimmed.slice(0, 120)}`);
      }
    }
  }
  assert.deepEqual(hits, [],
    `这些地方出现了那四把开关的读取或写入，可外壳那条通道今天还不存在：\n  ${hits.join('\n  ')}\n`
    + '要么把这把键从 SHELL_KEYS 里挪出去并补上真消费者，要么把这一处改回只走 ctx.setDraft（勾存起来、不动作）');
});
