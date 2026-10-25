/* 任务这一行的显示字典。干什么用：队列里每一行（检测 / 应用 / 恢复）的三个显示（文字、状态点样式、行样式）
   只从这里的三个函数取——原来它们在原型里各写一遍（:2105 与 :2156）。
   会错在哪：① 这套对应要是漂了，队列行显示"完成"而详情徽标显示"已检测"，同一件事两种说法，用户不知道这条到底跑完没有；
             ② 行的数据本身不在这里造：上一版有 makeTask／resetTaskToWaiting 那几支"前端自己造一行、自己改一行状态"的算法，
                接了真服务之后那一张表由服务给（契约的 Task 形状＋事件里的整份快照），
                前端再造一次就是同一张表长了两个家——留着的症状是"点完跳一下再退回原样"。
   怎么回来：状态词表由 tests/contract.test.mjs 现读 internal/contract/contract.go 对账（少一条多一条都红）；
             三个映射函数是纯函数，tests/task.test.mjs 直接喂对象测。 */

import { TaskMeta } from '../meta.js';

/** 服务那张表可能出现的九个状态（与 contract.Task* 逐字相同，由 tests/contract.test.mjs 对账） */
export const STATUS = ['waiting', 'running', 'done', 'failed', 'skipped',
  'rolledback', 'cancelled', 'restoring', 'restored'];

/** 三种任务种类（与 contract.Kind* 逐字相同） */
export const KINDS = ['detect', 'apply', 'restore'];

/** 检测类任务借用同一批状态码，但界面上的说法要跟"检测"对齐（原型 :1994-2013 那三张表） */
const DETECT_TEXT = { waiting: '待检测', running: '检测中', done: '已检测', failed: '检测失败', cancelled: '已取消' };
const DETECT_DOT = { waiting: 'unset', running: 'pending', done: 'applied', failed: 'error', cancelled: 'unset' };
const DETECT_CLS = { waiting: 'waiting', running: 'detecting', done: 'detected', failed: 'detect-failed', cancelled: 'cancelled' };

function metaOf(task) {
  const m = TaskMeta[task.status];
  if (!m) throw new Error(`任务 ${task.id} 的状态认不出来：${task.status}（core/task/model.js 的 STATUS 里没这一档）`);
  return m;
}

export function taskStatusText(task) {
  if (task.kind === 'detect' && DETECT_TEXT[task.status]) return DETECT_TEXT[task.status];
  return metaOf(task).text;
}

export function taskStatusDot(task) {
  if (task.kind === 'detect' && DETECT_DOT[task.status]) return DETECT_DOT[task.status];
  return metaOf(task).dot;
}

export function taskStatusCls(task) {
  if (task.kind === 'detect' && DETECT_CLS[task.status]) return DETECT_CLS[task.status];
  return metaOf(task).cls;
}

/** 一行任务是结束还是没结束——队列汇总、"清空已完成"、全局进度条三处共用这一个判据 */
export function isDetectFinished(task) {
  return ['done', 'failed', 'cancelled'].includes(task.status);
}

export function isApplyFinished(task) {
  return ['done', 'failed', 'skipped', 'rolledback'].includes(task.status);
}
