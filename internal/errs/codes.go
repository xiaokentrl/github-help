package errs

// 这一份清单是全项目错误码唯一的家。用法：每个码都要有真发它的那个人（不许登记占位码），
// 前端语言文件按 `err.<码>` 配句子，两边集合等值由门禁 G13 管。
// GAC-MODE-<方式id>-xxx 那一族故意不在这里，它由 ModeCode() 现场拼——见 errs.go 顶上那段说明。

// GAC-IPC-xxx：通道层（对端身份、帧、请求分发、客户端连接）。
const (
	CodeIPCFrame      = "GAC-IPC-001" // 收到的行不是合法 JSON
	CodeIPCNoMethod   = "GAC-IPC-002" // 请求没有方法名
	CodeIPCWrite      = "GAC-IPC-003" // 回执写不出去
	CodeIPCPeer       = "GAC-IPC-010" // 对端身份核不过
	CodeIPCNoHandler  = "GAC-IPC-011" // 服务不认这个方法
	CodeIPCHandler    = "GAC-IPC-012" // 方法跑挂了（码由 handler 的错误决定，这是兜底）
	CodeIPCRuntimeDir = "GAC-IPC-020" // 建不出运行时目录
	CodeIPCStale      = "GAC-IPC-023" // 旧端点清不掉
	CodeIPCListen     = "GAC-IPC-021" // 监听失败
	CodeIPCChmod      = "GAC-IPC-022" // 端点权限没改成 0600
	CodeIPCPlatform   = "GAC-IPC-030" // 本平台通道未实现（Windows 命名管道属 M1）
	CodeIPCDisconnect = "GAC-IPC-040" // 连不上后台服务
	CodeIPCTimeout    = "GAC-IPC-041" // 等回执超时
	CodeIPCReply      = "GAC-IPC-042" // 收到的回执不是合法 JSON
	CodeIPCShape      = "GAC-IPC-043" // 服务给的状态看不懂
)

// GAC-HOSTS-xxx：hosts 文件这一侧（读、合并、写、校验、回滚）。
const (
	CodeHostsRead     = "GAC-HOSTS-001" // 读不到 hosts
	CodeHostsParse    = "GAC-HOSTS-002" // 解析不了（内容本身没救，不会静默丢行）
	CodeHostsWrite    = "GAC-HOSTS-003" // 写不进去（权限不足／文件被占用）
	CodeHostsVerify   = "GAC-HOSTS-004" // 写完重读对不上，已用备份还原
	CodeHostsRollback = "GAC-HOSTS-005" // 还原本身也失败——最坏那一档，界面要给出备份路径
	CodeHostsBusy     = "GAC-HOSTS-006" // 已有写入在跑（L3 的全局串行锁给的就是这个码）
	CodeHostsConflict = "GAC-HOSTS-007" // 一个域名挂两条条目，拒绝（D5 甲）
	CodeHostsBadValue = "GAC-HOSTS-008" // 这条数据不是域名形状，拒绝（D4 甲）
	// CodeHostsSharedLine：要管的域名跟别的域名挤在同一行里。
	// 为什么单开一个码：停用整行会连带把别人家的域名一起停掉，那是"私自改变其他功能"；
	// 拆开那一行又要替用户重排他的写法。两样都不做——拒写、把这一行原文还给界面看。
	CodeHostsSharedLine = "GAC-HOSTS-009"
	// 下面两条是"写成了，但有件事人该知道"那一档：它们不走 error，
	// 因为"这一次生效了没有"是主判断，把附属信息混进 error 会让人以为没生效。
	CodeHostsOwnerKept = "GAC-HOSTS-010" // 新文件的属主／ACL 没补回原来那一份（当前身份不是 root，或 Windows 那半要到 M5）
	CodeHostsNoBackup  = "GAC-HOSTS-011" // 改动前那份没存下备份（写入照做，但出问题时要人自己找依据）
)

// GAC-SETTINGS-xxx：那 13 个偏好键的读写。
// 为什么不复用 GAC-HOSTS 那一族：那三条的话术全在说"hosts 没改成"，
// 设置没存上借它的码，界面就把一句"你的主题没换过来"演成"你的 hosts 写坏了"——这是误导，不是省码。
const (
	CodeSettingsDir   = "GAC-SETTINGS-001" // 取不到配置目录（连文件该放哪儿都不知道）
	CodeSettingsRead  = "GAC-SETTINGS-002" // 读不到设置文件（存在但读不动，跟"还没写过设置"是两件事）
	CodeSettingsParse = "GAC-SETTINGS-003" // 设置文件看不懂／里面的数不像话
	CodeSettingsWrite = "GAC-SETTINGS-004" // 设置写不出去（界面上改完会退回原值）
)

// GAC-DETECT-xxx：IP 层与方式层的取数、探测。
const (
	CodeDetectEmpty   = "GAC-DETECT-001" // 候选池为空
	CodeDetectTimeout = "GAC-DETECT-002" // 取数超时（本轮这个方式没交回东西）
	CodeDetectNoServe = "GAC-DETECT-003" // 交回的 IP 全都不能服务该域名
	CodeDetectBlocked = "GAC-DETECT-004" // 疑似被针对性阻断，换 IP 无效
	CodeDetectBudget  = "GAC-DETECT-005" // 超预算，本轮只算了前 N 条，剩下排队
	CodeDetectOffline = "GAC-DETECT-006" // 这条方式要联网而现在连不通
)

// GAC-PERM-xxx：权限自检。
const (
	CodePermNoWrite = "GAC-PERM-001" // 当前身份写不动 hosts
	CodePermFlush   = "GAC-PERM-002" // 刷 DNS 没成功（Windows 上 Dnscache 要权限）
)

// GAC-TASK-xxx：队列与任务状态机。
// 这一族是本轮新登记的前缀（第 9.10 节那张表里没有它）：加第 12 个方法 CancelTask 之后，
// 「任务号查不到」「已跑完没法取消」这两件事必须有码可回，塞进 GAC-HOSTS 会把两件事混成一件。
const (
	CodeTaskNotFound = "GAC-TASK-001" // 任务号不存在
	CodeTaskFinal    = "GAC-TASK-002" // 任务已在终态，取消不动它（照原型 :3124 那句行为）
	CodeTaskPaused   = "GAC-TASK-003" // 队列暂停中，等「恢复」这个动作
	// CodeTaskBadItem：界面传来的条目 id 在菜单清单里找不到。
	// 为什么单开一条：把它当成"这一条没检出结果"来跑，症状是队列跑完、那一行什么也没发生，
	// 人会以为加速失败了；它其实是清单里没这一条，得说成另一句话。
	CodeTaskBadItem = "GAC-TASK-004"
)

// GAC-UPDATE-xxx：自动更新（M5 才接线，今天这三条是"如实说没接线"用的）。
const (
	CodeUpdateNotWired = "GAC-UPDATE-501" // 这一站还没做，界面显示"本期不提供"，不许演成功
)

// CodeUnknown 是 CodeOf 认不出码时的兜底：宁可显示"未知错误 + 原始消息"，也不静默当成某个已知码。
const CodeUnknown = "GAC-UNKNOWN-000"

// Codes 给出本文件登记的码清单（不含 GAC-MODE- 那一族）。
// 它是门禁 G13 的Go 侧对手方：两侧各自解析出标识符再做集合比较，不许用 strings.Contains。
func Codes() []string {
	return []string{
		CodeIPCFrame, CodeIPCNoMethod, CodeIPCWrite, CodeIPCPeer, CodeIPCNoHandler, CodeIPCHandler,
		CodeIPCRuntimeDir, CodeIPCStale, CodeIPCListen, CodeIPCChmod, CodeIPCPlatform,
		CodeIPCDisconnect, CodeIPCTimeout, CodeIPCReply, CodeIPCShape,
		CodeHostsRead, CodeHostsParse, CodeHostsWrite, CodeHostsVerify, CodeHostsRollback,
		CodeHostsBusy, CodeHostsConflict, CodeHostsBadValue, CodeHostsSharedLine,
		CodeHostsOwnerKept, CodeHostsNoBackup,
		CodeSettingsDir, CodeSettingsRead, CodeSettingsParse, CodeSettingsWrite,
		CodeDetectEmpty, CodeDetectTimeout, CodeDetectNoServe, CodeDetectBlocked,
		CodeDetectBudget, CodeDetectOffline,
		CodePermNoWrite, CodePermFlush,
		CodeTaskNotFound, CodeTaskFinal, CodeTaskPaused, CodeTaskBadItem,
		CodeUpdateNotWired,
		CodeUnknown,
	}
}
