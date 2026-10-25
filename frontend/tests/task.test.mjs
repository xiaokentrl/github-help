/* 任务这一行的显示字典。干什么用：队列里每一行（检测／应用／恢复）的三个显示（文字、状态点样式、行样式）
   只从 core/task/model.js 那三支函数取，这里把"检测类要说已检测、应用类要说完成"那套对应钉死。
   会错在哪：① 这套对应原来在原型里写了两遍（:2105 与 :2156），搬成一份之后如果映射漂了，
                队列行显示"完成"而详情徽标显示"已检测"——同一件事两种说法，用户不知道这条到底跑完没有；
             ② 上一版这里还测 makeTask／resetTaskToWaiting／itemStatusAfterChange 那三支"前端自己造一行、
                自己改一行状态"的算法。接了真服务之后那张表由服务给（契约 Task ＋ 事件里的整份快照），
                三支一起删了：留着测试反而会把人引回"界面自己改状态"那条路。
   怎么回来：红的时候看消息里那个 kind＋status；改判据要连 model.js 的表一起改，别只改测试。
             九个状态与三种种类的名单由 tests/contract.test.mjs 现读 internal/contract/contract.go 对账。 */

import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
  STATUS, KINDS, isDetectFinished, isApplyFinished,
  taskStatusText, taskStatusDot, taskStatusCls
} from '../core/task/model.js';

const task = (kind, status, extra = {}) => ({ id: 'x', kind, status, itemId: 'a', name: 'N', ...extra });

test('检测类的五个状态说"检测"的话，应用类说"完成"的话（原型 :1994-2013 那三张表）', () => {
  assert.equal(taskStatusText(task('detect', 'waiting')), '待检测');
  assert.equal(taskStatusText(task('detect', 'running')), '检测中');
  assert.equal(taskStatusText(task('detect', 'done')), '已检测');
  assert.equal(taskStatusText(task('detect', 'failed')), '检测失败');
  assert.equal(taskStatusText(task('detect', 'cancelled')), '已取消');
  assert.equal(taskStatusText(task('apply', 'done')), '完成');
  assert.equal(taskStatusText(task('apply', 'restored')), '已恢复');
  assert.equal(taskStatusCls(task('detect', 'done')), 'detected');
  assert.equal(taskStatusCls(task('apply', 'done')), 'done');
  assert.equal(taskStatusDot(task('detect', 'done')), 'applied');
});

test('认不出来的状态当场抛错，不给界面留一个空白行', () => {
  assert.throws(() => taskStatusText(task('apply', 'exploded')), /状态认不出来/);
  assert.throws(() => taskStatusDot(task('apply', 'exploded')), /状态认不出来/);
  assert.throws(() => taskStatusCls(task('apply', 'exploded')), /状态认不出来/);
  assert.equal(STATUS.length, 9);
  assert.deepEqual(KINDS, ['detect', 'apply', 'restore']);
});

test('结束没结束分两种判据——队列汇总与"清空已完成"用哪一条都在这里', () => {
  assert.equal(isDetectFinished(task('detect', 'cancelled')), true);
  assert.equal(isDetectFinished(task('detect', 'skipped')), false, '检测类没有跳过这一档');
  assert.equal(isDetectFinished(task('detect', 'running')), false);
  assert.equal(isApplyFinished(task('apply', 'skipped')), true);
  assert.equal(isApplyFinished(task('apply', 'rolledback')), true);
  assert.equal(isApplyFinished(task('apply', 'waiting')), false);
  assert.equal(isApplyFinished(task('restore', 'restored')), false, '恢复行的终态在 flows 那条 whenKindIdle 里判，不混进应用那一条');
});
