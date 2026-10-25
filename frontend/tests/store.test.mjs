/* store 那批查询辅助的判据。干什么用：界面所有"这条能不能勾／这一类是不是全勾／还原按钮该不该出现"都问这几个纯函数，
   这里把它们的边界钉住——尤其是「↺ 到底认哪一栏」这件事。
   会错在哪：上一版认的是 item.prevAppliedIp 是不是 undefined（原型 :1309-1310 那套界面自己记账的形状）。
             接了真服务之后那一栏是字符串、omitempty，空串装着两件事（"不是我们写的"与"写之前本来没有"），
             分不开的症状是：hosts 里明明还有我们那几行，↺ 却不出现，人只能去手动改文件。
             现在只认契约 Item.managed，判据由 internal/service 那一侧从文件里数出来，界面不再自己记。
   怎么回来：红的时候看断言消息里那条 id；改判据要连 core/store.js 那两句注释一起改。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
  state, resetState, allItems, findItem, findCategoryOfItem, categoryCheckState,
  checkedItems, canApply, canRestoreHosts, hasRestorableInCategory, allCheckedState,
  snapshotCollapsed, restoreCollapsed, collapseOnly, replaceCategories
} from '../core/store.js';

function seed() {
  resetState({
    categories: [
      { id: 'c1', name: '类目一', items: [
        { id: 'a', name: '条目a', checked: true },
        { id: 'b', name: '条目b', checked: false, bestIp: '1.1.1.1' }
      ] },
      { id: 'c2', name: '类目二', items: [
        { id: 'c', name: '条目c', checked: false }
      ] }
    ],
    queue: [], selectedId: 'b', lastDetect: ''
  });
  state.collapsed.clear();
}

test('查询：类目勾选三态与「全选」判据（原型 :1297 与 :1316）', () => {
  seed();
  assert.equal(categoryCheckState(state.categories[0]), 'indeterminate');
  assert.equal(categoryCheckState(state.categories[1]), 'unchecked');
  state.categories[1].items[0].checked = true;
  assert.equal(categoryCheckState(state.categories[1]), 'checked');
  assert.equal(allCheckedState(), false, 'b 还没勾，三条里勾了两条就不算全选');
  state.categories[0].items[1].checked = true;
  assert.equal(allCheckedState(), true);
});

test('查询：没有条目时「全选」说不选，别让空列表显示成已全选', () => {
  resetState({ categories: [] });
  assert.equal(allCheckedState(), false);
  assert.equal(categoryCheckState({ items: [] }), 'unchecked');
});

test('查询：条目找得到类目，找不到的返回 null（详情与队列都靠这一支把行对上）', () => {
  seed();
  assert.equal(findCategoryOfItem('c').id, 'c2');
  assert.equal(findCategoryOfItem('nope'), null);
  assert.equal(findItem('b').name, '条目b');
  assert.equal(findItem('nope'), null);
});

test('判据：canApply 只看有没有候选 IP；↺ 只认服务给的 managed（已定：D3 甲）', () => {
  seed();
  assert.equal(canApply({ bestIp: '' }), false);
  assert.equal(canApply({ bestIp: '1.1.1.1' }), true);

  assert.equal(canRestoreHosts({}), false, '服务没说这一条是托管行，↺ 不该出现');
  assert.equal(canRestoreHosts({ managed: true }), true);
  assert.equal(canRestoreHosts({ managed: false }), false);
  assert.equal(canRestoreHosts(null), false);
  // 界面自己记的那一栏再也不是凭据：prevAppliedIp 空串装着两件事，分不开
  assert.equal(canRestoreHosts({ prevAppliedIp: null }), false);
  assert.equal(canRestoreHosts({ prevAppliedIp: '1.2.3.4' }), false);

  assert.equal(hasRestorableInCategory(state.categories[0]), false);
  state.categories[0].items[0].managed = true;
  assert.equal(hasRestorableInCategory(state.categories[0]), true, '一类里只要有一条托管行，类目那颗 ↺ 就要在');
});

test('折叠：跑完批量动作要回到跑之前的样子，collapseOnly 只留当前那一类', () => {
  seed();
  state.collapsed.add('c1');
  const snap = snapshotCollapsed();
  collapseOnly('c2');
  assert.deepEqual([...state.collapsed], ['c1'], 'collapseOnly("c2") 把 c1 收着、c2 展开');
  restoreCollapsed(snap);
  assert.deepEqual([...state.collapsed], ['c1']);
});

test('换树：一次同步不能把用户正在看的那条弄丢，也认不得留一个不存在的选中', () => {
  seed();
  replaceCategories(state.categories);
  assert.equal(state.selectedId, 'b', '还在就接着选中');

  resetState({ selectedId: 'b' });
  replaceCategories([{ id: 'c1', name: '类目一', items: [{ id: 'a', name: '条目a', checked: false }] }]);
  assert.equal(state.selectedId, 'b', '找不到就留着，由详情面板自己说这条没了（服务端将来才换树，前端不猜）');

  resetState({ selectedId: null });
  assert.equal(state.selectedId, null);
});

test('checkedItems 只认真正勾上的，顺序跟树一致', () => {
  seed();
  state.categories[0].items[1].checked = true;
  assert.deepEqual(checkedItems().map(i => i.id), ['a', 'b']);
  assert.equal(allItems().length, 3);
});
