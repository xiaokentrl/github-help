/* 界面这一侧的契约镜像：方法名、走线名、契约版本号，三个名单只有一个家。干什么用：
   core/api/client.js 与 core/api/wire.js 只准从这里取名字，别处再写一遍字符串就由门禁判红。
   会错在哪：这三组名字是界面与外壳之间唯一的接头。漂一个字母不会有人报错——
             方法名漂了，服务回一句 GAC-IPC-011（看起来像"服务没这个功能"）；
             走线名漂了，界面发出去的那一句根本没人听，症状是"每个按钮点下去都石沉大海"；
             版本号漂了，两侧字段少一栏，那一行永远空着且不报错。
   怎么回来：真源是 internal/contract/contract.go。frontend/tests/contract.test.mjs 每次现读那一份源码，
             把下面三个名单逐字比对，少一条、多一条、改一个字都当场红。 */

/** 契约里那 17 个方法（值＝Go 侧 Method* 常量的值）。 */
export const METHODS = {
  getStatus: 'GetStatus',
  detect: 'Detect',
  applyQueue: 'ApplyQueue',
  restoreItem: 'RestoreItem',
  restoreHosts: 'RestoreHosts',
  getSettings: 'GetSettings',
  saveSettings: 'SaveSettings',
  checkPermissions: 'CheckPermissions',
  checkUpdate: 'CheckUpdate',
  downloadUpdate: 'DownloadUpdate',
  getVersion: 'GetVersion',
  cancelTask: 'CancelTask',
  pauseQueue: 'PauseQueue',
  resumeQueue: 'ResumeQueue',
  getSnapshot: 'GetSnapshot',
  clearTasks: 'ClearTasks',
  openURL: 'OpenURL'
};

/** 壳与页面之间那三条线（值＝Go 侧 Wire* 常量的值）。业务事件那 5 条不在这里，家在 contract/events.js。 */
export const WIRE = {
  invoke: 'gac:invoke',
  reply: 'gac:reply',
  state: 'gac:state'
};

/** 这一份界面是按哪一版契约写的。
    干什么用：壳给的每一条回执都带它自己那一版号，两个数不等就说明界面与外壳不是同一轮构建出来的——
              这时候最省事的猜法"大概差不了多少"正是最难查的那种漂。
    会错在哪：Go 那边推了版本号、这里没跟着推：每一次调用都判成"版本不对"，界面全线点不动（有声，好查）；
              反过来这里独自推号：判成"对得上"，缺的那一栏却永远空着（无声，难查）。
    怎么回来：改 internal/contract/contract.go 的 ProtocolVersion 那一轮，同轮改这里这一个数。 */
export const EXPECTED_PROTOCOL = 10;
