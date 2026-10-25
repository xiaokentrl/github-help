/* 接口形状：字段名冻结处。干什么用：把服务回给界面的那些结构写成 TS 能检查的形状，`appliedIp` 写成 `appliedIP` 在编辑器里就报错。
   会错在哪：字段名是两侧唯一的接头（总纲第 2 节第 52 块硬性要求逐字一致）。这里跟 Go 漂了，界面读到 undefined，显示成空值而不报错——最难查的那种。
   怎么回来：真源是 internal/contract/contract.go 的结构体，本文件是它在界面这一侧的镜像；字段名单边由 internal/contract/contract_test.go 钉拼写，跨语言的名单比对在 frontend/tests/contract.test.mjs。

   下面每个字段名照 Go 的 json tag 抄，`?` 对应 Go 那边的 omitempty。 */

export interface Request {
  id: number;
  method: string;
  params?: Record<string, unknown>;
}

export interface Response {
  id: number;
  ok: boolean;
  err?: string;
  code?: string;
  /** 发这条回执那一侧的契约版本号；两侧握手时比对，不一致当场报错，不猜 */
  protocol: number;
  /** 这一个码的结构化参数；只回码界面凑不出那句人话 */
  params?: Record<string, unknown>;
  result?: unknown;
}

/** GetStatus 的返回内容 */
export interface StatusReply {
  service: State;
  /** true 表示此刻有 hosts 写入在跑，界面据此把会并发写的那几个按钮置灰 */
  writeLockHeld: boolean;
}

/** 服务的状态快照 */
export interface State {
  name: string;
  version: string;
  pid: number;
  startedAt: string;
  phase: string;
  /** 队列的暂停闸此刻开着还是关着。只有 ResumeQueue 能关掉它，收尾不许自己清（L2） */
  queuePaused: boolean;
  /** 空表示一切正常；不为空时带 GAC- 前缀的错误码 */
  lastError?: string;
  /** 上面那一句的码与参数（契约 v9）。为什么这一份也要有：队列那一行上一轮换成用户那门语言了，
      状态栏只拿到原文就换不动——同一屏两种说法，看的人以为工具自己坏了 */
  lastErrorCode?: string;
  lastErrorParams?: Record<string, unknown>;
  /** 这台机器此刻真正在管 DNS 解析的服务名；空＝认不出来，界面显示 —，绝不自己编一个（已定：C5） */
  dnsService?: string;
  /** 配置文件与 hosts 备份所在目录，由服务回（C5 同一句） */
  dataDir?: string;
  /** 这一份服务真正在写的那一个文件；Windows 与 Linux 的判据只在服务侧有 */
  hostsPath?: string;
}

/** 一条域名加速项。字段名冻结：是 appliedIp，不是 appliedIP */
export interface Item {
  id: string;
  name: string;
  appliedIp?: string;
  prevAppliedIp?: string;
  status: string;
  /** hosts 里有没有这一条自己那几行（我们写的）。界面上 ↺ 那一个按钮只在这一栏为真时才点得动 */
  managed?: boolean;
  bestIp?: string;
  /** 「IP 来源」那一行的显示名，由服务给（取的是方式的显示名，不是拼好的整句） */
  source?: string;
  /** 这个来源是排第一的那个（primary）还是第一没选出来、第二顶上的（fallback）。文案在 i18n，别在这儿拼 */
  sourceRole?: string;
  /** 最优 IP 的握手延迟，毫秒；0 表示没测出来 */
  latency?: number;
  /** 最近一次检测跑完的时间（UTC / RFC3339）；空＝尚未检测 */
  lastCheck?: string;
  /** 只回码与参数，整句人话在 i18n 那一个家 */
  errorCode?: string;
  errorParams?: Record<string, unknown>;
}

/** 那 13 个偏好键（键名逐字冻结，两侧都不许再加） */
export interface Settings {
  sidebarWidth: number;
  queueHeight: number;
  zoom: number;
  language: string;
  theme: string;
  trayMinimize: boolean;
  showTrayIcon: boolean;
  autoStart: boolean;
  startMinimized: boolean;
  detectFocusFollow: boolean;
  applyFocusFollow: boolean;
  autoDetectOnStartup: boolean;
  autoApplyOnStartup: boolean;
}

/** 事件带出去的完整状态快照：界面不许靠"先收一半再猜另一半"画界面 */
export interface Snapshot {
  /** 断网判定（netwatch）此刻的读数：状态栏第一格照它画 */
  networkOffline: boolean;
  state: State;
  items: Item[];
  tasks: Task[];
}

export interface Event {
  name: string;
  snapshot: Snapshot;
  payload?: Record<string, unknown>;
}

/** 一条异步任务的回执形状（检测/应用/还原改异步后的任务号靠它，你拍的 4A） */
export interface Task {
  id: string;
  kind: string;
  status: string;
  /** 这一行属于哪条条目；RestoreHosts 那种整份文件的任务没有，留空 */
  itemId?: string;
  /** 队列那一行显示的名字。跟着快照走，别让界面拿 itemId 去别处查 */
  name?: string;
  /** 正在跑的那一步（backup／write／flush／verify）；不在跑就空 */
  phase?: string;
  processed: number;
  total: number;
  /** 服务侧那一句原文，留给日志与控制台看；界面上那一列不许直接显示它 */
  lastErr?: string;
  /** 这一行没跑成的码与参数（契约 v8），队列那一列那句人话由它们凑 */
  errorCode?: string;
  errorParams?: Record<string, unknown>;
  /** 这一行跑了多久（毫秒）。没跑的行是 0，界面画破折号，不许显示 0.0s */
  durationMs?: number;
}

/** gac:state 那一句的形状：只说这条桥通不通，不说服务健康不健康（服务那一份看 snapshot.state） */
export interface LinkState {
  connected: boolean;
  /** 连不上时要连同这一个路径一起报，否则人只看得到一句"连不上" */
  endpoint: string;
  err?: string;
  /** 这条线累计丢掉的事件帧数；一直涨说明页面画不过来，要让人看得见 */
  dropped: number;
}

/** 四个异步方法（Detect／ApplyQueue／RestoreItem／RestoreHosts）立刻回的就是这个 */
export interface TaskIDReply {
  taskID: string;
}

/** CancelTask 的回执：只有等待中的能取消；count 是这一次实际取消掉几条 */
/** UpdateCheck 的回执：latest＝源上最新版本号；log＝release 正文的条目行（最多 20 行）。 */
export interface UpdateCheckReply {
  latest: string;
  hasUpdate: boolean;
  log: string[];
}

/** UpdateDownload 的回执：包落在哪（不做自动安装,装不装归用户）。 */
export interface UpdateDownloadReply {
  savedPath: string;
}

/** OpenURL 的回执：命令交给系统打开器了就是 true（窗口在用户那侧，服务只认得"交出去了"）。 */
export interface OpenURLReply {
  opened: boolean;
}

/** OpenHosts 的回执：Mode＝放开的权限字面量；Opened＝编辑器真被拉起来了（false＝没显示会话，如实说）。 */
export interface OpenHostsReply {
  mode: string;
  opened: boolean;
  note?: string;
}

export interface CancelReply {
  cancelled: boolean;
  count: number;
}

/** PauseQueue／ResumeQueue 的回执：回的是闸此刻的状态 */
export interface QueueReply {
  paused: boolean;
}

/** GetVersion 的回执；buildDate 打包时没填就是空，界面显示 — */
export interface VersionReply {
  current: string;
  buildDate?: string;
}

/** CheckPermissions 的回执 */
export interface PermReply {
  canWrite: boolean;
  dnsFlushed: boolean;
  detail?: string;
}
