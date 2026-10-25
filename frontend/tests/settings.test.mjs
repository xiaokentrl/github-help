/* 设置这一层的判据。干什么用：默认值、范围、存档里翻出旧值时怎么办，全在 normalize/loadSettings 这两个纯函数里，
   这里喂假存档测——面板还没建，先保证"面板将来怎么写都写不出非法值"。
   会错在哪：① 默认值在面板里再写一份，就会出现"说明写 240、打开是 340"；
             ② 旧档里那些已经不认的值（主题「深色」、语言「中文」）不能崩、也不能瞎用，得回落到默认；
             ③ 「自动应用」在「自动检测」关掉时还留着 true，界面上一勾一闪自己变回去，看着像坏了。
   怎么回来：红的时候看消息里那个键。改默认值要连原型的 DEFAULT_SETTINGS 一起看（:1199-1213），
             这里是"合法范围与回落"的家，默认值本身只有 settings.js 那一份。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
  DEFAULTS, SETTINGS_KEY, SIDEBAR_MIN_W, SIDEBAR_MAX_W, QUEUE_MIN_H,
  normalize, loadSettings, saveSettings, queueMaxHeight
} from '../core/settings.js';

const THEMES = ['light', 'midnight', 'oled', 'forest', 'ocean', 'sakura'];

/** 假存档：只有这两个方法，读写都记账，不让测试碰本机 localStorage */
function fakeStore(initial) {
  const data = new Map(initial ? [[SETTINGS_KEY, initial]] : []);
  const writes = [];
  return {
    writes,
    getItem: k => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => { data.set(k, v); writes.push([k, v]); },
    removeItem: k => data.delete(k)
  };
}

test('默认值只有一份：13 个键，界面那 13 个设置项一一对上（原型 :1199-1213）', () => {
  assert.equal(Object.keys(DEFAULTS).length, 13);
  assert.deepEqual(normalize(null, THEMES), DEFAULTS);
  assert.deepEqual(normalize(undefined, THEMES), DEFAULTS);
});

test('范围：宽度越界夹回来，缩放只认四档，别的值一律回落默认', () => {
  const small = normalize({ sidebarWidth: 60, queueHeight: 20 }, THEMES);
  assert.equal(small.sidebarWidth, SIDEBAR_MIN_W);
  assert.equal(small.queueHeight, QUEUE_MIN_H);

  const big = normalize({ sidebarWidth: 99999 }, THEMES);
  assert.equal(big.sidebarWidth, SIDEBAR_MAX_W);

  assert.equal(normalize({ sidebarWidth: 'abc' }, THEMES).sidebarWidth, DEFAULTS.sidebarWidth);
  for (const z of [0.9, 1, 1.1, 1.25]) assert.equal(normalize({ zoom: z }, THEMES).zoom, z);
  assert.equal(normalize({ zoom: 1.5 }, THEMES).zoom, 1, '缩放只认那四档，别的值不能让界面变成没验证过的样子');
});

test('主题与语言：认不出来的主题回落，别留一个没有配方的 id', () => {
  assert.equal(normalize({ theme: 'dark' }, THEMES).theme, 'light', '旧档里那个"深色"已经不认了');
  assert.equal(normalize({ theme: 'forest' }, THEMES).theme, 'forest');
  assert.equal(normalize({ language: 'en' }, THEMES).language, 'en');
  assert.equal(normalize({ language: '' }, THEMES).language, DEFAULTS.language);
});

test('联动：自动检测没开，自动应用必须落回关闭（原型 :1516-1530 那段灰掉联动的判据）', () => {
  const off = normalize({ autoDetectOnStartup: false, autoApplyOnStartup: true }, THEMES);
  assert.equal(off.autoDetectOnStartup, false);
  assert.equal(off.autoApplyOnStartup, false);
  assert.equal(normalize({ autoDetectOnStartup: true, autoApplyOnStartup: true }, THEMES).autoApplyOnStartup, true);
});

test('布尔项：存档里写字符串 true 不算 true，缺项就用默认，不许 undefined 混进界面', () => {
  const out = normalize({ trayMinimize: 'true', showTrayIcon: false }, THEMES);
  assert.equal(out.trayMinimize, DEFAULTS.trayMinimize);
  assert.equal(out.showTrayIcon, false);
});

test('读写存档：坏 JSON、没这个键、写不进去三种情况都不该把界面卡住', () => {
  const broken = fakeStore('{"theme":');
  assert.equal(loadSettings(THEMES, broken).theme, 'light');

  assert.deepEqual(loadSettings(THEMES, fakeStore(null)), DEFAULTS);
  const stored = fakeStore(JSON.stringify({ theme: 'oled', sidebarWidth: 300 }));
  const loaded = loadSettings(THEMES, stored);
  assert.equal(loaded.theme, 'oled');
  assert.equal(loaded.sidebarWidth, 300);

  saveSettings(loaded, stored);
  assert.equal(stored.writes.length, 1);
  assert.equal(JSON.parse(stored.writes[0][1]).theme, 'oled');

  const exploding = { getItem: () => { throw new Error('隐私模式'); }, setItem: () => { throw new Error('配额满'); } };
  assert.deepEqual(loadSettings(THEMES, exploding), DEFAULTS);
  assert.doesNotThrow(() => saveSettings(DEFAULTS, exploding));
});

test('队列高度上限：留够上面那一片，窗口再矮也不许把树压没', () => {
  assert.equal(queueMaxHeight(1000), 700);
  assert.equal(queueMaxHeight(240), 40, '矮窗口按 innerHeight-200 走');
  assert.equal(queueMaxHeight(150), -50, '矮到不如干脆不给，调用方自己夹下限');
});
