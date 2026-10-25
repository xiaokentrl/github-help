/* 状态字典：每项加速、每个任务"处于哪一步"对应的中文与样式名。干什么用：菜单、详情、队列三处显示的同一批状态，名字和文字都从这里取。
   会错在哪：这四组原来挂在原型顶层（:1139-:1173），菜单和渲染器都要读，所以放在 core/（谁都不许再抄一份）；抄一份出去的后果是同一件事在界面上有两种说法。
   怎么回来：四组内容逐字搬自原型，一字未改（F1 的判据是行为一字不变）。加状态只改本文件，且菜单与渲染器同时看到。 */

/** 一项加速项的状态 → 界面文字 + 状态点样式 + 徽标样式（原型 :1139） */
export const ItemMeta = {
  unset:      { text: '未替换', dot: 'unset',    badge: 'unset' },
  detecting:  { text: '检测中', dot: 'pending',  badge: 'pending' },
  detected:   { text: '待应用', dot: 'detected', badge: 'detected' },
  applying:   { text: '应用中', dot: 'pending',  badge: 'pending' },
  applied:    { text: '已替换', dot: 'applied',  badge: 'applied' },
  stale:      { text: '待更新', dot: 'stale',    badge: 'stale' },
  error:      { text: '异常',   dot: 'error',    badge: 'error' },
  rolledback: { text: '已回滚', dot: 'error',    badge: 'error' }
};

/** 队列里一个任务的状态 → 界面文字 + 行样式 + 状态点样式（原型 :1150） */
export const TaskMeta = {
  waiting:    { text: '等待',   cls: 'waiting',    dot: 'unset' },
  running:    { text: '执行中', cls: 'running',    dot: 'pending' },
  done:       { text: '完成',   cls: 'done',       dot: 'applied' },
  failed:     { text: '失败',   cls: 'failed',     dot: 'error' },
  skipped:    { text: '跳过',   cls: 'skipped',    dot: 'unset' },
  rolledback: { text: '已回滚', cls: 'rolledback', dot: 'error' },
  cancelled:  { text: '已取消', cls: 'cancelled',  dot: 'unset' },
  restoring:  { text: '恢复中', cls: 'restoring',  dot: 'pending' },
  restored:   { text: '已恢复', cls: 'restored',   dot: 'stale' }
};

/** 应用时走的四个阶段（原型 :1162）。文字给状态行看，key 给逻辑判 */
export const PHASES = [
  { key: 'backup', text: '备份 hosts…' },
  { key: 'write',  text: '写入域名映射…' },
  { key: 'flush',  text: '刷新 DNS 缓存…' },
  { key: 'verify', text: '验证终端可用性…' }
];

/** 还原时走的四个阶段（原型 :1168）。与 PHASES 是两套，别合（第 6 节第 7 条：两种"还原"不是一回事） */
export const RESTORE_PHASES = [
  { key: 'backup', text: '备份当前 hosts…' },
  { key: 'write',  text: '写回应用前映射…' },
  { key: 'flush',  text: '刷新 DNS 缓存…' },
  { key: 'verify', text: '验证恢复结果…' }
];
