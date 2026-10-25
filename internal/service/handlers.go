package service

import (
	"bytes"
	"encoding/json"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/ipc"
	"github.com/hlep/github-accel/internal/settings"
)

// handlers.go 把契约里那 16 个方法逐个挂到通道上。
//
// 干什么用：这一个文件是"界面能问什么"的全部答案。方法名、参数键名、返回形状的家都在
// internal/contract 与技术方案第 11.1 节那张表里，这里只写"接住参数→交给谁→回什么"。
//
// 什么情况下会错、出错时人看得见什么：
// ① 漏注册一个方法 → 编译过得去、服务起得来，界面上那颗按钮点下去什么也不动，
//
//	通道只会回一句 GAC-IPC-011。所以有一条用例照着 contract.AllMethods 数注册表（门禁 G3）。
//
// ② 把四个异步方法写成"跑完才回" → 界面卡住，进度事件全没了意义〔已定：契约硬性要求"立刻返回任务号"〕。
//
//	所以这四个都只调 submit，一个网络请求也不在这儿发。
//
// ③ 在回执里拼整句人话 → 顶掉已定的 D6 乙（服务只回码与参数）。这里回的全是结构体，没有一句文案。
//
// 怎么回来：①红了就补注册那一行；②③红了就把那一段挪回执行体里去，别在通道层加"聪明"的兜底。

// Register 挂上全部方法。origin 一律是"人点的"：从通道进来的都是界面动作。
// 开机自动化那条走 trigger.go，它调的是同一批 submit，只是登记的发起方不一样（L1 的解）。
func (s *Service) Register(h *ipc.Server) {
	h.Register(contract.MethodGetStatus, s.hGetStatus)
	h.Register(contract.MethodDetect, s.hDetect)
	h.Register(contract.MethodApplyQueue, s.hApplyQueue)
	h.Register(contract.MethodRestoreItem, s.hRestoreItem)
	h.Register(contract.MethodRestoreHosts, s.hRestoreHosts)
	h.Register(contract.MethodGetSettings, s.hGetSettings)
	h.Register(contract.MethodSaveSettings, s.hSaveSettings)
	h.Register(contract.MethodCheckPerm, s.hCheckPerm)
	h.Register(contract.MethodCheckUpdate, s.hCheckUpdate)
	h.Register(contract.MethodDownloadUpdate, s.hDownloadUpdate)
	h.Register(contract.MethodGetVersion, s.hGetVersion)
	h.Register(contract.MethodCancelTask, s.hCancelTask)
	h.Register(contract.MethodPauseQueue, s.hPauseQueue)
	h.Register(contract.MethodResumeQueue, s.hResumeQueue)
	h.Register(contract.MethodGetSnapshot, s.hGetSnapshot)
	h.Register(contract.MethodClearTasks, s.hClearTasks)
	h.Register(contract.MethodOpenURL, s.hOpenURL)
}

// registeredMethods 把注册表读出来给门禁 G3 用：它要的是"实际挂上去的名字"，不是"代码里写了几行 Register"。
// 为什么要再造一张表：ipc.Server 的 handlers 是私有字段，测试拿不到；
// 于是在这里按 contract.AllMethods 的顺序自己解一遍——两边一旦不等，用例当场红，不会静默少一个方法。
func (s *Service) registeredMethods() map[string]ipc.Handler {
	return map[string]ipc.Handler{
		contract.MethodGetStatus:      s.hGetStatus,
		contract.MethodDetect:         s.hDetect,
		contract.MethodApplyQueue:     s.hApplyQueue,
		contract.MethodRestoreItem:    s.hRestoreItem,
		contract.MethodRestoreHosts:   s.hRestoreHosts,
		contract.MethodGetSettings:    s.hGetSettings,
		contract.MethodSaveSettings:   s.hSaveSettings,
		contract.MethodCheckPerm:      s.hCheckPerm,
		contract.MethodCheckUpdate:    s.hCheckUpdate,
		contract.MethodDownloadUpdate: s.hDownloadUpdate,
		contract.MethodGetVersion:     s.hGetVersion,
		contract.MethodCancelTask:     s.hCancelTask,
		contract.MethodPauseQueue:     s.hPauseQueue,
		contract.MethodResumeQueue:    s.hResumeQueue,
		contract.MethodGetSnapshot:    s.hGetSnapshot,
		contract.MethodClearTasks:     s.hClearTasks,
		contract.MethodOpenURL:        s.hOpenURL,
	}
}

func (s *Service) hGetStatus(map[string]any) (any, error) {
	return s.status(), nil
}

// hGetSnapshot 是界面唯一一口能主动问到"54 条此刻各是什么状态、队列里此刻有哪几行"的地方。
//
// 干什么用：开机那一下与断线重连之后各取一次。事件推的是"变了什么"，问不出"现在是什么"——
// 少了这一个口，界面重连之后只能干等下一次有人点按钮，树会一直空着。
// 什么情况下会错：把读 hosts 失败当成"一条都没有"→ 界面把用户自己写的那几行说成没生效。
// 所以这里复用 Snapshot()：读不出来时那几列留空（＝不知道），并把读不动这件事带码写进 lastError。
// 怎么回来：这一问不排队、不改任何东西，纯读；红了只需要看 snapshot.go 那一份组装有没有漂。
func (s *Service) hGetSnapshot(map[string]any) (any, error) {
	return s.Snapshot(), nil
}

// hClearTasks 无参数：抹掉队列里已经跑完且不欠人一句话的那些行（失败与已回滚留下，好让「重试失败的那几条」有料），
// 回抹完之后那一份完整快照。
//
// 干什么用：界面上那颗「清空已完成」（原型 :2438）要有地方可发。
// 为什么回快照而不是回一个"抹了几行"的数：清完之后屏幕上要画的正是"还剩哪几行"，
// 回快照＝界面走的是与 GetSnapshot、与每一条事件同一条合并路径，不为本方法另立一种画法；
// 至于那句"清理了几条"，前端拿自己清之前那几行去数——那正是用户看见的行数。
// 什么情况下会错、出错时人看得见什么：只回个数、不回表 → 前端得自己再猜一次抹完长什么样，
// 于是"点了没反应"换成"点完行少了但批次进度那两个数还是抹之前的"，更难看。
// 怎么回来：判据（哪几行算跑完）在 queue.go 那一段，这里只负责接住与回表。
func (s *Service) hClearTasks(map[string]any) (any, error) {
	return s.clearFinished(), nil
}

// hDetect 收 {itemIDs}。不给 itemIDs＝全部检测（原型「全部检测」那颗按钮那一档）。
func (s *Service) hDetect(params map[string]any) (any, error) {
	return s.task(contract.KindDetect, stringsParam(params, "itemIDs"), nil)
}

// hApplyQueue 收 {itemIDs, picks}。picks 是界面上选中某个候选地址那一档：{条目id: IP}。
func (s *Service) hApplyQueue(params map[string]any) (any, error) {
	return s.task(contract.KindApply, stringsParam(params, "itemIDs"), picksParam(params, "picks"))
}

// hRestoreItem 收 {itemID}，一条。
func (s *Service) hRestoreItem(params map[string]any) (any, error) {
	id := stringParam(params, "itemID")
	if id == "" {
		return nil, newBadItem(id)
	}
	return s.task(contract.KindRestore, []string{id}, nil)
}

// hRestoreHosts 无参数：撤掉全部托管。它与 hRestoreItem 共用同一个执行体，只是范围不同〔已定〕，
// 所以这里传空列表，由队列展开成"每一条一行"——界面上那 51 行各自有没有撤掉，看得见。
func (s *Service) hRestoreHosts(map[string]any) (any, error) {
	return s.task(contract.KindRestore, nil, nil)
}

// task 是四个异步方法共用的那一步：入队、回任务号。一个网络请求、一次写盘都不在这儿发生。
func (s *Service) task(kind string, itemIDs []string, picks map[string]string) (any, error) {
	batch, err := s.submit(kind, itemIDs, originUser, picks)
	if err != nil {
		return nil, err
	}
	return contract.TaskIDReply{TaskID: batch}, nil
}

func (s *Service) hGetSettings(map[string]any) (any, error) {
	return s.loadSettings()
}

func (s *Service) hSaveSettings(params map[string]any) (any, error) {
	next, err := decodeSettings(params["settings"])
	if err != nil {
		return nil, err
	}
	p, err := s.settingsPath()
	if err != nil {
		return nil, err
	}
	if err := settings.SaveTo(p, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (s *Service) hCancelTask(params map[string]any) (any, error) {
	// taskID 可以不填：顶部那颗「取消等待」不针对某一行，原型 :734 那颗按钮给的就是这一档。
	// 上一版把它写成必填，那颗按钮就没有后端可对——这处偏离已登记在技术方案第 11.1 节那条。
	id := stringParam(params, "taskID")
	n, err := s.cancel(id)
	if err != nil {
		return nil, err
	}
	return contract.CancelReply{Cancelled: n > 0, Count: n}, nil
}

func (s *Service) hPauseQueue(map[string]any) (any, error) {
	return contract.QueueReply{Paused: s.pause()}, nil
}

// hCheckPerm 不进队列：界面上那盏权限灯要当场亮，等不起一轮检测。
// 它做的两件事都不动文件内容——按"不截断"的标志试开一次 hosts（perms.go 里那段说明），再刷一次 DNS。
func (s *Service) hCheckPerm(map[string]any) (any, error) {
	return s.checkPerms(), nil
}

func (s *Service) hResumeQueue(map[string]any) (any, error) {
	return contract.QueueReply{Paused: s.resume()}, nil
}

// stringsParam 把界面传来的数组解成字符串列表。缺键、传了别的东西都按"没给"算，
// 而不是当成空列表报错——「全部检测」那一档本来就靠不填这一键表达。
func stringsParam(params map[string]any, key string) []string {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stringParam(params map[string]any, key string) string {
	if v, ok := params[key].(string); ok {
		return v
	}
	return ""
}

// decodeSettings 把界面传来的那一份设置对象解成契约结构体。
//
// 为什么绕一次 JSON 而不是手抄 13 个键：手抄就是那批键名的第二个家，键名一漂这里必然漏一条。
// 为什么要 DisallowUnknownFields：已定的 E1 甲说"方式不许自带新键"，
// 多出来的键静默收下＝界面以为自己存上了，下一次读回来又没有——那句"设置存了没生效"就是这么长出来的。
func decodeSettings(raw any) (settings.Settings, error) {
	var out settings.Settings
	if raw == nil {
		return out, errs.New(errs.CodeSettingsParse, nil, "这组设置是空的，存不了")
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return out, errs.New(errs.CodeSettingsWrite, nil, "这组设置编码不出来").WithCause(err)
	}
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, errs.New(errs.CodeSettingsParse, map[string]any{"cause": err.Error()},
			"这组设置看不懂，没往里存").WithCause(err)
	}
	return out, nil
}

// picksParam 解 {条目id: IP}。值不是字符串的键直接丢——那是界面拼错了，
// 丢掉的后果是"这一条用它自己的最优地址"，比按一个看不懂的地址写进 hosts 轻。
func picksParam(params map[string]any, key string) map[string]string {
	raw, ok := params[key].(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]string{}
	for id, v := range raw {
		if ip, ok := v.(string); ok && ip != "" {
			out[id] = ip
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// hOpenURL 收 {url}：让系统把这条链接打开（「关于我们」那三颗入口钮）。
// 会错在哪：URL 不合法当场拒（opener.go 那道门卫，scheme 只认 http/https）；
// 打开器跑了但浏览器没起，服务这边如实回错——界面那半句"没打开成"的话有料可说。
func (s *Service) hOpenURL(params map[string]any) (any, error) {
	u := stringParam(params, "url")
	if err := validateOpenURL(u); err != nil {
		return nil, err
	}
	if err := s.cfg.Opener(u); err != nil {
		return nil, errs.New(errs.CodeIPCOpenURL, map[string]any{"url": u},
			"打开器交不出去这一句：%v", err)
	}
	return contract.OpenURLReply{Opened: true}, nil
}
