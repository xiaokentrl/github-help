/* 服务往界面推的事件名，前端这一侧的唯一清单。干什么用：界面订阅事件、Go 侧发射事件，两边都从这里／那里的常量取值，名字不许在别处再写一遍字符串。
   会错在哪：名字漂了不会报错，只会"队列跑完了界面还停在上一次"——这种问题最难查。前 4 条是总纲第 2 节第 52 块冻结的那 4 个；第 5 条 detect:progress 是你拍 4A（检测改异步）之后要有的"单项检测完成"。
   怎么回来：frontend/tests/contract.test.mjs 每次现读 internal/contract/contract.go 的 Event 常量比对，两边少一条、多一条、拼错一个字都当场红。 */

export const IPC_EVENTS = {
  queueProgress: 'queue:progress',
  queueDone: 'queue:done',
  downloadProgress: 'download:progress',
  networkChange: 'network:change',
  detectProgress: 'detect:progress'
};
