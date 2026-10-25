/* 详情页的显示规则（2026-10-26 用户拍"没有数据就没有数据,有数据立刻同步,能收起"）:
   ① 未选中 → 空态(请选择左侧任意项);
   ② 选中但没有数据(没检测过、不在跑) → "这一条还没有数据"占位,不给骨架屏凭空摆破折号;
   ③ 选中且有数据(lastCheck/bestIp)或在跑 → 详情壳,字段从 state 即时取;
   ④ 收起不在视图里——它在 main.js 的 onSelect(再点一次已选中＝selectedId 置空),视图只管按 state 画。 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { installDom } from './lib/dom-stub.mjs';

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

installDom();

const { createI18n } = await import(pathToFileURL(path.join(FRONTEND, 'core/i18n/t.js')).href);
const { state } = await import(pathToFileURL(path.join(FRONTEND, 'core/store.js')).href);
const { createDetailView } = await import(pathToFileURL(path.join(FRONTEND, 'views/detail/DetailView.js')).href);

const zhCN = (await import(pathToFileURL(path.join(FRONTEND, 'core/i18n/locales/zh-CN.js')).href)).default;
const t = createI18n({ 'zh-CN': zhCN });

function seed(items) {
  state.categories = [{ id: 'c1', name: '类目', items }];
  state.collapsed = new Set();
  state.selectedId = null;
}

function build() {
  const root = document.createElement('div');
  root.id = 'detail';
  document.body.appendChild(root);
  const view = createDetailView({ root, t, handlers: {} });
  return { root, view };
}

const bodyText = root => root.textContent || '';

test('详情：未选中＝空态占位', () => {
  const restoreState = { ...state };
  try {
    seed([{ id: 'i1', name: '条目', checked: false, status: 'unset', domains: ['example.com'], candidates: [] }]);
    const { root, view } = build();
    state.selectedId = null;
    view.render();
    assert.ok(bodyText(root).includes('请选择左侧任意项'), '未选中该显示空态');
  } finally { Object.assign(state, restoreState); }
});

test('详情：没检测过＝"这一条还没有数据",不摆骨架屏', () => {
  seed([{ id: 'i1', name: '条目', checked: false, status: 'unset', domains: ['example.com'], candidates: [] }]);
  const { root, view } = build();
  state.selectedId = 'i1';
  view.render();
  assert.ok(bodyText(root).includes('这一条还没有数据'), '无数据该给占位句');
  assert.ok(!root.querySelector('.crumb'), '无数据不该摆详情骨架');
});

test('详情：有数据（lastCheck）＝详情壳即时呈现', () => {
  seed([{ id: 'i1', name: '条目', checked: false, status: 'unset', domains: ['example.com'], candidates: [], lastCheck: '2026-10-26T10:00:00Z', bestIp: '1.2.3.4' }]);
  const { root, view } = build();
  state.selectedId = 'i1';
  view.render();
  assert.ok(root.querySelector('.crumb'), '有数据该摆详情壳');
  assert.ok(bodyText(root).includes('条目'), '面包屑要有条目名');
});

test('详情：在跑（detecting）＝显示详情壳,让进度看得见', () => {
  seed([{ id: 'i1', name: '条目', checked: false, status: 'detecting', domains: ['example.com'], candidates: [] }]);
  const { root, view } = build();
  state.selectedId = 'i1';
  view.render();
  assert.ok(root.querySelector('.crumb'), '在跑的条目该显示详情壳');
});

test('详情：数据到了,同一支 render 再跑一遍立刻换内容（即时同步）', () => {
  seed([{ id: 'i1', name: '条目', checked: false, status: 'unset', domains: ['example.com'], candidates: [] }]);
  const { root, view } = build();
  state.selectedId = 'i1';
  view.render();
  assert.ok(bodyText(root).includes('这一条还没有数据'));
  // 检测完成:数据到位,不换实例、不重建,同一支 render 就得把壳换出来
  state.categories[0].items[0].lastCheck = '2026-10-26T10:00:00Z';
  state.categories[0].items[0].bestIp = '1.2.3.4';
  state.categories[0].items[0].candidates = state.categories[0].items[0].candidates || [];
  view.render();
  assert.ok(root.querySelector('.crumb'), '数据到位后该立刻切成详情壳');
});
