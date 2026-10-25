// Package contract 是两个程序之间唯一的真源：界面和服务都从这里取方法名、事件名和字段名。
// 字段名是冻结契约，改一个字母就要同步改界面，所以这里只放常量与结构体，不放逻辑。
package contract

// 方法名。界面按这些名字调用，服务按这些名字分发。
// 一共 16 条：前 11 条与总纲第 2 节那张表逐字对齐，一条不多；后 5 条（下面逐条说明）是定档轮与联调轮补的，
// 那一张表还停在 11 条上——改代码这一轮要同步把它对齐，否则下一个人数方法的人只看到旧表。
// 第 12 条 CancelTask 是 2026-10-22 定档轮 D7 甲登记进来的：原型 :734 那颗「取消等待」、:1157 那个 cancelled 状态、
// :3123-3141 那段真会跑的取消逻辑都是界面已有、后端一直没名字，不补这个方法那三处永远接不上后端。
// 第 13、14 条走的是同一条判据，同一批实测：原型 :733 那颗 #btnPause、:1181 那个 state.paused、
// :3106-3115 那段点击逻辑（连同"将在当前任务完成后暂停"那句提示）、:2604 与 :2683 那两处真会读的暂停闸。
// 队列都暂停不了，M3 那句"暂停要能停在方式之间"就没有入口可接。
// 第 15 条 GetSnapshot 是 2026-10-23 联调轮补的，补它的实测依据是这一句：
// 快照此前只随事件推（Snapshot 只有服务进程里拿得到），界面开机或断线重连后**没有任何一口可以主动取**——
// 树会一直空着，直到下一次事件。事件不能替"我此刻要看一眼全部"这一问回答。
// 第 16 条 ClearTasks 是同一轮补的，补它的实测依据是这两句：
// ① 界面上「清空已完成」那颗按钮（原型 :2438）在契约里没有一个家——发不出去就只能本机把行抹掉，
// 而下一次事件又把那些行原样带回来，症状是"点了没反应"；
// ② 服务这一侧 s.tasks 只 append 不删（queue.go submit），一次开机跑几十条就一直堆到进程退出。
// 删掉终态那几行既不丢信息（跑完的行列已在条目状态里），又把这一处无上限增长收掉。
const (
	MethodGetStatus      = "GetStatus"
	MethodDetect         = "Detect"
	MethodApplyQueue     = "ApplyQueue"
	MethodRestoreItem    = "RestoreItem"
	MethodRestoreHosts   = "RestoreHosts"
	MethodGetSettings    = "GetSettings"
	MethodSaveSettings   = "SaveSettings"
	MethodCheckPerm      = "CheckPermissions"
	MethodCheckUpdate    = "CheckUpdate"
	MethodDownloadUpdate = "DownloadUpdate"
	MethodGetVersion     = "GetVersion"
	MethodCancelTask     = "CancelTask"
	MethodPauseQueue     = "PauseQueue"
	MethodResumeQueue    = "ResumeQueue"
	MethodGetSnapshot    = "GetSnapshot"
	MethodClearTasks     = "ClearTasks"
)

// AllMethods 是上面那 16 条的清单本身。为什么要有一份切片而不只有一批常量：
// 服务启动时要照着它核对"每个方法都注册了没有"（门禁 G3），用例也读这一份。
// 再抄第二份清单＝加一个方法只改一处，另一处悄悄缺一条，症状是界面调它报 GAC-IPC-011。
var AllMethods = []string{
	MethodGetStatus, MethodDetect, MethodApplyQueue, MethodRestoreItem, MethodRestoreHosts,
	MethodGetSettings, MethodSaveSettings, MethodCheckPerm, MethodCheckUpdate,
	MethodDownloadUpdate, MethodGetVersion, MethodCancelTask, MethodPauseQueue, MethodResumeQueue,
	MethodGetSnapshot, MethodClearTasks,
}

// 事件名。服务往界面推的这些名字，两侧必须逐字一致。
// 前四条＝契约 v0.1 那 4 个事件；detect:progress 是契约 v0.2 要加的那条「单项检测完成」（技术方案第 6 节第 1 条）。
const (
	EventQueueProgress    = "queue:progress"
	EventQueueDone        = "queue:done"
	EventDownloadProgress = "download:progress"
	EventNetworkChange    = "network:change"
	EventDetectProgress   = "detect:progress"
)

// EventsPending 是"名字已经冻在契约里、服务这一侧还没有生产者"那两条的名单（值＝什么时候才有）。
//
// 干什么用：让"这条线铺好了没人走"这件事有一处可核对的登记，而不是散在注释里。
// 2026-10-23 这一轮把整条链数了一遍，两处是零生产者：
//   - download:progress：要它得先有真的下载（更新通道，技术方案 M5 那一站）。
//     为什么不能先补一个"0%→100%"两个数了事：CheckUpdate／DownloadUpdate 今天如实回 GAC-UPDATE-501，
//     一个字节也没下过，推出去的进度就是说谎——界面上那条进度条正是靠它画的。
//   - network:change：要它得先有断网判定（GAC-DETECT-006，后端第 6.4 节第 1 条那笔欠账）。
//     状态栏第一格今天说的是另一件真事：那条线通不通（gac:state → state.connected），不是"网络正常"。
//
// 会错在哪：这一份名单必须与真生产者同步。接上那天不删这一条，用例 TestEveryEventHasProducerOrIsPending 当场红；
// 反过来删了这一条却没人发，同一条用例也红——两边都拦，才不会有"看着像接上了"的第三种状态。
//
// 怎么回来：接上那一站同轮做三件事：这里删一条、生产者那一处发出去、界面那边按已有订阅画（一条也不用改）。
var EventsPending = map[string]string{
	EventDownloadProgress: "M5 更新通道（CheckUpdate／DownloadUpdate 真跑起来那天才有进度可推）",
	EventNetworkChange:    "断网判定 GAC-DETECT-006（后端目录结构规划第 6.4 节第 1 条）",
}

// AllEvents 同上：清单只有一个家（AllMethods 那段说明）。
var AllEvents = []string{
	EventQueueProgress, EventQueueDone, EventDownloadProgress, EventNetworkChange, EventDetectProgress,
}

// 协议版本。两侧握手时比对，不一致就当场报错，不猜。
// 2＝加进第 12 个方法 CancelTask，并给 Item 补上 bestIp / errorCode / errorParams 三个字段。
// 3＝加 PauseQueue / ResumeQueue（第 13、14 条），并把界面那几行要显示的东西补进快照：
// Item 的 source / sourceRole / latency / lastCheck，Task 的 itemId / name / phase，State 的 queuePaused。
// 补这些不是为了好看——事件带的必须是自己画得出来的那一份，缺一个字段界面就有一行永远空着。
// 4＝加第 15 个方法 GetSnapshot，并给 State 补 dnsService / dataDir / hostsPath 三个数。
// 这三个数不是锦上添花：C5 定的就是"DNS 服务名与数据目录由服务回、界面不猜"，而契约里没有它们的家，
// 界面只能继续显示 mock 那两份假值（systemd-resolved / ~/.config/github-accel）——那一行看着像真话，其实是猜的。
// 5＝加第 16 个方法 ClearTasks（界面那颗「清空已完成」＋服务那一份只增不删的任务表）。
// 6＝把壳与页面之间那三条线的名字与形状也收进这一份契约（Wire*／LinkState，走线复用 Request 与 Response）。
// 这一档不是给服务读的，是给界面读的：界面那份构建（dist）与壳可能不是同一轮出来的，
// 名字漂了不会报错，症状是"每一次点击都回不来"。回执里带的 protocol 就是给界面比这一档用的。
// 7＝前端接真服务这一轮补的两个字段，每一个都对应界面上一处"接上真服务就点不动／永远空着"的断链：
// Item.managed——界面上那颗 ↺（还原到应用前）判的是"这一条有没有可还原的记录"，
// 而 prevAppliedIp 一个空串同时装着两件事："这一条不是我们写的"与"写之前 hosts 里本来没有这一条"
// （服务侧把后者也记成空串）。少了 managed 这一栏，界面只能把前者当后者判：
// 症状是刚应用好的那几条（也正是绝大多数：用户的 hosts 里本来就没有这些域名）↺ 根本不出来，还原点不动。
// Task.durationMs——队列那张表有一整列「耗时」，它的原料此前只有 mock 在本地掐表；
// 真服务的回执里没有一个时间，接上之后那一列永远是一个破折号，等于把这一列砍掉。
// 8＝给 Task 补 errorCode／errorParams 两栏，把队列那一行的「为什么」也接到 D6 乙那条规矩上：
// 此前只有 lastErr 那一句服务侧原文，界面拿不到码——语言文件凑不出那句话，
// 症状是队列里失败的那几行显示一串中文原文（切了语言还是中文），而条目那一行却是一句本地话，同一件事两种说法。
// 9＝给 State 补 lastErrorCode／lastErrorParams 两栏，补的是同一笔账的第三处断链（前两处＝Item 那两栏、Task 那两栏）：
// 状态栏那一句（本轮上限到了"剩下的排队"、读不到 hosts、刷 DNS 没成）此前只有 lastError 一句原文，
// 而它正是队列那一行同一个失败的第二次出声——行上换了语言、状态栏还是中文，一句话两种说法在一屏之内。
const ProtocolVersion = 9

const (
	// ServiceRunning 表示服务在线、可以接活。
	ServiceRunning = "running"
	// ServiceUnavailable 表示连不上服务，界面要把这句话如实显示出来。
	ServiceUnavailable = "unavailable"
)

// 条目那八档状态。名字与前端 core/meta.js 的 ItemMeta 逐字对齐，比对装在 frontend/tests/contract.test.mjs。
// 为什么要在 Go 侧也立一份常量而不是让 service 直接写字符串：同一批状态在 Go 里散成八条字面量，
// 改一个档位名就只改得动界面那侧，症状是那一档永远显示不出来（而且不报错）。
const (
	ItemUnset      = "unset"
	ItemDetecting  = "detecting"
	ItemDetected   = "detected"
	ItemApplying   = "applying"
	ItemApplied    = "applied"
	ItemStale      = "stale"
	ItemError      = "error"
	ItemRolledback = "rolledback"
)

// 队列那九档任务状态，同样与 core/meta.js 的 TaskMeta 逐字对齐。
const (
	TaskWaiting    = "waiting"
	TaskRunning    = "running"
	TaskDone       = "done"
	TaskFailed     = "failed"
	TaskSkipped    = "skipped"
	TaskRolledback = "rolledback"
	TaskCancelled  = "cancelled"
	TaskRestoring  = "restoring"
	TaskRestored   = "restored"
)

// 任务三种。core/task/model.js 的 KINDS 就是这三条，多一种界面就认不出来（它是抛错不是兜底）。
const (
	KindDetect  = "detect"
	KindApply   = "apply"
	KindRestore = "restore"
)

// 「IP 来源」那一行的两种身份。为什么要单独一个字段而不是把「（备用）」拼进 Source：
// ① 拼进去＝服务侧在写文案，顶掉技术方案第 15 节 D6 乙那一条；
// ② 拼进去以后这一串既当名字又当标记，界面判不了"这一轮是不是备用顶上的"，M2 那条"主用不许随便换人"也就没法验。
const (
	SourcePrimary  = "primary"
	SourceFallback = "fallback"
)

// 队列里一次写入走完的四个阶段（原型 :1162 那四行、:1168 那四行：文案两套、key 同一批，
// 所以 Go 侧只需要 key——文案的家在界面那侧，见 D6 乙）。
const (
	PhaseBackup = "backup"
	PhaseWrite  = "write"
	PhaseFlush  = "flush"
	PhaseVerify = "verify"
)

// Request 是一行一条的调用请求。
type Request struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	// Params 由各方法自己解释，通道层不碰。
	Params map[string]any `json:"params,omitempty"`
}

// Response 是一行一条的调用回执。
type Response struct {
	ID   int64  `json:"id"`
	OK   bool   `json:"ok"`
	Err  string `json:"err,omitempty"`
	Code string `json:"code,omitempty"`
	// Protocol 是发这条回执那一侧的契约版本号。上面那句"两侧握手时比对，不一致就当场报错"要靠它才有落实的地方：
	// 此前版本号只是常量，两侧真跑起来从没比过一次——旧服务配新界面会静默少字段（少的那行永远空着且不报错），
	// 正是这批账里最难查的一种。每一条回执都带，因为界面第一次问话就可能问到旧形状。
	Protocol int `json:"protocol"`
	// Params 是这一个码的结构化参数（域名、路径、第几条）。
	// 为什么码有了还要这一份：D6 乙定的就是"服务只回码与参数、整句人话在语言文件"，
	// 只回码的话界面拿不到"是哪一条域名坏了"，那句人话凑不出来，只能显示一个裸码。
	Params map[string]any `json:"params,omitempty"`
	// Result 是这份调用的返回内容，界面侧按方法名解到对应结构体。
	Result any `json:"result,omitempty"`
}

// 壳与页面之间那三条线。它们不是业务事件（不进 AllEvents、也不叫 Event*），
// 因为说的是"这句话走哪一条线"，而不是"服务那边发生了什么"。
// 为什么这一组名字也放进契约而不是留在 cmd/gui：它在两个程序里各写一遍正是最容易漂的那种形状——
// 漂了不报错，症状是"每一次点击都回不来"。对账装在 frontend/tests/contract.test.mjs（现读这一份源码）。
// 走线的形状直接复用上面那两个结构体：Request 是页面→壳那一句，Response 是壳→页面那一句
// （id 由页面自己编号，桥会把多次调用同时在飞，回执必须对得回它自己那一次点击；
// protocol 填发这一句那一侧的契约版本号，页面拿它判"界面与外壳不是同一轮构建的"这一档）。
const (
	WireInvoke = "gac:invoke" // 页面→壳：一次调用
	WireReply  = "gac:reply"  // 壳→页面：那一次调用的回执
	WireState  = "gac:state"  // 壳→页面：这条线此刻通不通，只说这一件事
)

// LinkState 是 gac:state 那一句的形状。它说的只是"这条桥通不通"，不是"服务健康不健康"：
// 服务那一份状态仍然只有 GetStatus 与事件里的 snapshot.state 说了算，界面不许拿这里的一句话去猜业务阶段。
type LinkState struct {
	Connected bool `json:"connected"`
	// Endpoint 是这条线要连的那一个文件路径。报"连不上"时要把路径一起报，否则人只看得到一句"连不上"。
	Endpoint string `json:"endpoint"`
	// Err 为空表示连着；不为空时带码前缀，理由同 Response.Err：整句中文不该由壳来拼〔已定：D6 乙〕。
	Err string `json:"err,omitempty"`
	// Dropped 是这条线累计丢掉的事件帧数。丢一帧不影响正确性（下一条自带完整快照），
	// 但它一直涨说明页面画不过来，这一句得让人看得见，不许当没事。
	Dropped int64 `json:"dropped"`
}

// StatusReply 是 GetStatus 的返回内容：界面靠它显示"服务在跑"还是"服务不可用"。
type StatusReply struct {
	Service State `json:"service"`
	// WriteLockHeld 为真表示此刻有 hosts 写入在跑；界面据此把会并发写的那几个按钮置灰。
	WriteLockHeld bool `json:"writeLockHeld"`
}

// State 是服务的状态快照。字段名冻结：是 appliedIp 不是 appliedIP。
type State struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	PID       int    `json:"pid"`
	StartedAt string `json:"startedAt"`
	Phase     string `json:"phase"`
	// QueuePaused 是队列的暂停闸此刻开着还是关着。它放在这里而不是只放 StatusReply：
	// 每条事件都自带完整快照，暂停这件事推给界面时也得带着同一份状态。
	// 只有 ResumeQueue 能把它关掉——收尾时顺手清掉就是 L2 那个"要点两下"（第 8 节那条）。
	QueuePaused bool `json:"queuePaused"`
	// LastError 为空表示一切正常；不为空时带错误码前缀。
	LastError string `json:"lastError,omitempty"`
	// LastErrorCode／LastErrorParams 是状态栏那一句"为什么"的码与参数（契约 v9）。
	// 为什么这一份也要有：上一轮给队列那一行（Task）补了码，而同一件事在状态栏还会出一次声——
	// 行上换成用户那门语言、状态栏还是服务侧原文，等于一屏之内两种说法。
	// 这一句原文（LastError）留着是给日志与控制台看的，界面上那句话由这两栏在语言文件凑〔已定：D6 乙〕。
	LastErrorCode   string         `json:"lastErrorCode,omitempty"`
	LastErrorParams map[string]any `json:"lastErrorParams,omitempty"`
	// DnsService 是这台机器此刻真正在管 DNS 解析的服务名（取值见 internal/dnsflush 那两份平台文件）。
	// 空＝认不出来，界面显示"—"；这里绝不留一个编好的名字：C5 定的就是"这一行由服务回，界面不猜"。
	DnsService string `json:"dnsService,omitempty"`
	// DataDir 是配置文件与 hosts 备份所在的目录（settings.Dir() 那一个出口给的就是它，不在两处各拼一遍）。
	DataDir string `json:"dataDir,omitempty"`
	// HostsPath 是这一份服务真正在写的那一个文件。
	// 界面「关于我们」那一格要写得出"改的是哪份文件"，而路径的判据只在服务侧有（Windows 与 Linux 不同）。
	HostsPath string `json:"hostsPath,omitempty"`
}

// Item 是一条域名加速项的状态快照。
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// AppliedIp 是当前写进 hosts 的地址。
	AppliedIp string `json:"appliedIp,omitempty"`
	// PrevAppliedIp 是应用之前的那个地址，还原靠它。
	PrevAppliedIp string `json:"prevAppliedIp,omitempty"`
	Status        string `json:"status"`
	// Managed 为真表示 hosts 里有我们为这一条写下的行（判据就是 itemStatus 用的那一个：文件里有行）。
	// 为什么光有 prevAppliedIp 不够：那一个空串装着两件事——"这一条不是我们写的"和
	// "写之前 hosts 里本来没有这一条"。界面那颗 ↺ 要分开的正是这两件事，分不开就点不动。
	// 来源只有文件（D3 甲），服务内存里没有第二份"是不是我们管的"。
	Managed bool `json:"managed"`
	// BestIp 是这一轮检测认为最能服务该条目的地址；没检测过就是空。
	// 界面拿它和 AppliedIp 比，才能分出"已生效"还是"有更快的没应用"（原型那两档就是这么干的）。
	BestIp string `json:"bestIp,omitempty"`
	// Source 是这一轮真给出 BestIp 那个方式的显示名——原型 :1900 那行「IP 来源」直接用它。
	// 名字由 resolve 的 Spec.Title 给：服务侧只给名字这一个数，不拼整句人话〔已定：D6 乙〕。
	Source string `json:"source,omitempty"`
	// SourceRole 说这个方式是"排第一的那个"还是"第一没选出来、第二顶上的"（取值见上面那两条常量）。
	SourceRole string `json:"sourceRole,omitempty"`
	// Latency 是 BestIp 的握手延迟，单位毫秒；没测出来是 0（原型 :1897 那行直接加 " ms"，所以这里给整数）。
	Latency int64 `json:"latency,omitempty"`
	// LastCheck 是这一条最近一次检测跑完的时间（UTC，RFC3339）。空＝还没检测过，界面那行显示"尚未检测"。
	// 时间的格式与含义分开冻：这里给可机读的串，"09:02:07"那种显示形状由界面决定。
	LastCheck string `json:"lastCheck,omitempty"`
	// ErrorCode 与 ErrorParams 是这条条目最近一次失败的原因。
	// 服务侧只回码和参数，整句人话在前端语言文件那一个家〔已定：技术方案第 15 节 D6 乙〕。
	ErrorCode   string         `json:"errorCode,omitempty"`
	ErrorParams map[string]any `json:"errorParams,omitempty"`
}

// Settings 是那 13 个偏好键的形状，键名逐字冻结（技术方案第 6 节第 6 条〔已定〕）。
// 内部包 settings 直接用这份结构体，不另立一个，否则同一批键名就长了两个家。
type Settings struct {
	SidebarWidth        int     `json:"sidebarWidth"`
	QueueHeight         int     `json:"queueHeight"`
	Zoom                float64 `json:"zoom"`
	Language            string  `json:"language"`
	Theme               string  `json:"theme"`
	TrayMinimize        bool    `json:"trayMinimize"`
	ShowTrayIcon        bool    `json:"showTrayIcon"`
	AutoStart           bool    `json:"autoStart"`
	StartMinimized      bool    `json:"startMinimized"`
	DetectFocusFollow   bool    `json:"detectFocusFollow"`
	ApplyFocusFollow    bool    `json:"applyFocusFollow"`
	AutoDetectOnStartup bool    `json:"autoDetectOnStartup"`
	AutoApplyOnStartup  bool    `json:"autoApplyOnStartup"`
}

// TaskIDReply 是四个异步方法（Detect／ApplyQueue／RestoreItem／RestoreHosts）的回执：
// 立刻给一个任务号，跑的过程靠事件推〔已定：契约硬性要求"立刻返回任务号"〕。
type TaskIDReply struct {
	TaskID string `json:"taskID"`
}

// CancelReply 是 CancelTask 的回执：只有等待中的任务能取消，正在跑的那一条不受影响（照原型 :3124 那句行为）。
// count 是这一次实际取消掉几条：顶部那颗「取消等待」不指名任务号，界面要拿这个数决定横幅上写"取消了 5 条"还是"没有可取消的"。
type CancelReply struct {
	Cancelled bool `json:"cancelled"`
	Count     int  `json:"count"`
}

// QueueReply 是 PauseQueue／ResumeQueue 的回执：回的是闸此刻的状态，不是"这次调用改了什么"。
// 为什么两个方法共用一份形状：暂停一个已经暂停的队列、恢复一个没暂停的队列都不该报错，
// 也不该回一个反过来的值——界面那颗按钮照这个数翻转文字（原型 :3106-3115 就是这么干的）。
type QueueReply struct {
	Paused bool `json:"paused"`
}

// VersionReply 是 GetVersion 的回执（形状照第 11.1 节那张表的 {current, buildDate}）。
type VersionReply struct {
	Current string `json:"current"`
	// BuildDate 由打包那一步（-ldflags）填进来。没填就如实留空，界面显示"—"：
	// 编一个日期进二进制，用户按它判断"我装的是哪一天的包"就完全是错的。
	BuildDate string `json:"buildDate,omitempty"`
}

// PermReply 是 CheckPermissions 的回执。
type PermReply struct {
	CanWrite   bool   `json:"canWrite"`
	DnsFlushed bool   `json:"dnsFlushed"`
	Detail     string `json:"detail,omitempty"`
}

// Event 是服务往界面推的一条事件。
// 每条事件都自带完整状态快照：界面不许靠"先收一半再猜另一半"画界面。
type Event struct {
	Name     string         `json:"name"`
	Snapshot Snapshot       `json:"snapshot"`
	Payload  map[string]any `json:"payload,omitempty"`
}

// Snapshot 是事件带出去的完整状态快照。
type Snapshot struct {
	State State  `json:"state"`
	Items []Item `json:"items"`
	// 写入任务在这里排队，界面只读不写。
	Tasks []Task `json:"tasks"`
}

// Task 是一条异步任务的回执形状。
type Task struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	// ItemID 是这一行属于哪条条目。RestoreHosts 那种"整份文件"的任务没有对应条目，所以留空。
	ItemID string `json:"itemId,omitempty"`
	// Name 是队列那一行显示的条目名。
	// 为什么要跟着快照走而不是让界面拿 itemId 去别处查：事件带的必须是自己画得出来的那一份，
	// 拆成两半就是"丢一个事件、那一行就空着"，而且查不到时界面不会报错。
	Name string `json:"name,omitempty"`
	// Phase 是这一步走到哪儿了（取值见上面那四条 Phase*）；不在跑就空。界面那行阶段文字拿它去语言文件换句子。
	Phase string `json:"phase,omitempty"`
	// Processed／Total 是全局进度条那两个数。
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
	LastErr   string `json:"lastErr,omitempty"`
	// ErrorCode／ErrorParams 是这一行没跑成的原因，规矩与 Item 那两栏完全一样〔已定：D6 乙〕：
	// 服务只给码与参数，整句人话在界面那份语言文件里凑。LastErr 那一句原文留着是给日志与控制台看的，
	// 界面那一列读的是这两栏——只剩原文的话，换了语言它还是中文，而同一个失败在条目那一行已经换成了本地话。
	ErrorCode   string         `json:"errorCode,omitempty"`
	ErrorParams map[string]any `json:"errorParams,omitempty"`
	// DurationMs 是这一行从取跑到收尾用了多少毫秒；还没跑完（含等待中、已取消、已跳过）就是 0。
	// 这里给可机读的毫秒数而不是"3.2s"那种串：显示形状归界面（同 Item.LastCheck 那条"格式与含义分开冻"），
	// 秒表只服务这一侧有——界面拿不到开始与结束那两个时刻，也就掐不出这个数。
	DurationMs int64 `json:"durationMs,omitempty"`
}
