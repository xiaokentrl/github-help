package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/ipc"
)

// bridge.go 是界面壳与后台服务之间的那一段桥。
//
// 干什么用：界面上的每一次点击都只能变成这一句 gac:invoke，壳把它转成一次 IPC 调用；
// 服务推进来的每一条事件，壳原样转成同名的窗口事件。桥只有三个名字：
// gac:invoke（界面→壳）、gac:reply（壳→界面，带 id）、gac:state（壳→界面，这条桥此刻通不通）。
// 这三个名字与走线的形状都不在这里写第二遍——家在同一份契约里（contract.Wire*／Request／Response／LinkState），
// 16 个业务方法名与 5 个事件名同理；壳只照着那份清单认名字，认不出来就不发。
//
// 什么情况下会错、出错时人看得见什么：
// ① 桥把界面上拼错的方法名照发不误 → 服务回一句 GAC-IPC-011，看起来跟"服务没这个功能"一模一样，
//
//	分不清是界面写错了还是服务缺注册。所以这里先按 contract.AllMethods 拦一道，并把拦下来的那句写清是"界面这条线写错了"。
//
// ② 界面上一个按钮都没有，因为壳连不上服务 → 这一档不能表现为"点了没反应"。
//
//	所以每次调用都回一句带码的话，另外连接一翻转就推一条 gac:state。
//
// ③ 服务重启之后界面还停在旧画面 → 事件是"变了什么"，重启这一段时间它一条也没收到。
//
//	所以接上之后推 gac:state{connected:true}，界面拿它去重新要一次 GetSnapshot。
//
// 怎么回来：①红了就改界面那头的调用点（名字仍只在契约里改一次）；②③红了先看 bridge_test.go 里那三张假服务，
// 它们能分开"桥没转"和"服务没答"这两种形状，别在桥里加兜底。

// callTimeout 给 10 秒。为什么够用：契约里那四个会跑久的方法（Detect／ApplyQueue／RestoreItem／RestoreHosts）
// 在服务侧是"立刻回任务号"，界面等的从来不是这一步；剩下的要么是纯读，要么只是试开一次文件。
// 为什么不能给太小：客户端一超时就把手里这条连接整个判废，同期还在飞的调用会跟着一起拿到"连不上"——
// 那正是界面突然全线报连不上的形状，比慢一点严重得多。
const callTimeout = 10 * time.Second

// retryEvery 给 1 秒：服务被人重启那一档，界面不该等人手忙脚乱地刷新。
// 为什么不做指数退避：这一条线连不上是常态（服务没起、权限没给），一秒一次的开销比"人多等 30 秒"轻。
const retryEvery = time.Second

// errLinkDropped 说的是"这条线半路断了"。为什么要跟"从来没连上过"分成两句：
// 断了意味着界面手里那份画面已经过期，要去重新要一次快照；没连上过则本来就没有画面可过期。
var errLinkDropped = errors.New(errs.CodeIPCDisconnect + ": 与后台服务之间的那条线断了")

// caller 是壳需要的那六件事。写成一小片接口只为了 bridge_test.go 能塞一张假服务进去：
// 三条铁律要求单元测试不碰真 hosts、不发真网络，真起一条 Unix 连接再加一个真服务就违背它了。
// 这里不另开一条分叉：真连接照旧走 *ipc.Client。
type caller interface {
	Connect(path string) error
	Close() error
	IsConnected() bool
	ActiveEndpoint() string
	Call(method string, params map[string]any, timeout time.Duration) (any, error)
	Events() <-chan contract.Event
	Dropped() int64
}

// 桥上走的那三种形状一律取自契约：页面→壳是 contract.Request，壳→页面是 contract.Response，
// 连接状态是 contract.LinkState。这里不再各立一份，否则同一句话有两个家，漂了谁也不报错。

// bridge 手里只有这一条连接，所以 Events() 的消费者天生只有一个（客户端那边写明只许一个）。
type bridge struct {
	endpoints []string
	client    caller
	timeout   time.Duration
	retry     time.Duration

	// emit 把一句话递给窗口。真壳里它就是 app.Event.Emit，测试里换成一张收集名单。
	emit func(name string, data any)
	logf func(format string, args ...any)

	// known 是 contract.AllMethods 那一份名单的集合形状：每次调用先照它认名字。
	known map[string]bool

	// connected／lastErr／dropped 只在 Run 这一条 goroutine 上读写，别的调用点不碰，所以不用锁。
	connected bool
	lastErr   string
	dropped   int64
}

func newBridge(endpoints []string, emit func(name string, data any)) *bridge {
	known := make(map[string]bool, len(contract.AllMethods))
	for _, m := range contract.AllMethods {
		known[m] = true
	}
	return &bridge{
		endpoints: endpoints,
		client:    ipc.NewClient(),
		timeout:   callTimeout,
		retry:     retryEvery,
		emit:      emit,
		logf:      log.Printf,
		known:     known,
	}
}

// handleInvoke 接住界面那一句，转成一次 IPC 调用，再把回执递回去。
// 为什么不在这里判断"服务大概还在跑"：连不上的那一句会由 client 当场回错，桥不替它编成功。
func (b *bridge) handleInvoke(raw any) {
	var req contract.Request
	if err := reMarshal(raw, &req); err != nil {
		b.emit(contract.WireReply, b.reply(contract.Response{OK: false,
			Code: errs.CodeIPCFrame, Err: "界面递过来的这句话看不懂：" + err.Error()}))
		return
	}
	if !b.known[req.Method] {
		// 一条也没发出去：界面上写着这个名字的地方要改，改完仍只在契约里对一次。
		b.emit(contract.WireReply, b.reply(contract.Response{ID: req.ID,
			Code: errs.CodeIPCNoHandler, Err: "契约里没有这个方法名，这条没发出去：" + req.Method}))
		return
	}

	data, err := b.client.Call(req.Method, req.Params, b.timeout)
	reply := contract.Response{ID: req.ID, OK: err == nil, Result: data}
	if err != nil {
		reply.Code, reply.Err = splitErr(err)
	}
	b.emit(contract.WireReply, b.reply(reply))
}

// currentEndpoint 给状态栏那一格"连的是哪条线"：连着的那条优先；一条都没连上就报第一条候选。
func (b *bridge) currentEndpoint() string {
	if ep := b.client.ActiveEndpoint(); ep != "" {
		return ep
	}
	if len(b.endpoints) > 0 {
		return b.endpoints[0]
	}
	return ""
}

// reply 给每一条回执补上发这一句这一侧的契约版本号。
// 干什么用：界面拿它对一句"你这份界面与外壳不是同一轮构建的"——这一档不写出来，
// 旧界面配新壳只会表现成某一行永远空着且不报错，正是这批账里最难查的那一种。
// 怎么会错：把这一句漏在调用点上＝总有一处回执漏了版本号；所以它是唯一的出口，回执都从这里出去。
func (b *bridge) reply(r contract.Response) contract.Response {
	r.Protocol = contract.ProtocolVersion
	return r
}

// Run 一直跑到窗口关掉：连不上就每秒再试一次，连上了就把服务推的事件原样转给窗口。
// 关掉时顺手把连接收掉——不留一条还在后台问话的桥，那正是"界面早关了、进程还赖着"的那一类。
func (b *bridge) Run(ctx context.Context) {
	ticker := time.NewTicker(b.retry)
	defer ticker.Stop()
	for {
		if !b.client.IsConnected() {
			if b.connected {
				// 线是半路断的（服务被重启、被杀掉）：先把这一句说出来，界面才知道手里那份画面已经过期。
				// 不说会怎样：按钮每一个都回一句"连不上"，而状态栏还挂着"在跑"——人只会以为工具自己坏了。
				b.setState(false, errLinkDropped)
			}
			// 权限模型定档（2026-10-25）后服务可能以 root 常驻在 /run/github-accel，
			// 也可能是开发者自己跑的那一份；每拍都把候选从头试一遍，不记住上一次连的是谁。
			var err error
			for _, ep := range b.endpoints {
				if err = b.client.Connect(ep); err == nil {
					break
				}
			}
			if err != nil {
				b.setState(false, err)
			} else {
				b.setState(true, nil)
			}
		}
		select {
		case <-ctx.Done():
			_ = b.client.Close()
			return
		case <-ticker.C:
		case ev := <-b.client.Events():
			b.forward(ev)
		}
	}
}

// forward 把一条服务事件整条交给窗口——连快照一起，不拆也不补。
// 干什么用：界面那五句订阅用的就是契约里那五个名字，桥不加前缀，名字就少一个家。
// 怎么会错：这里要是只转 payload 不转快照，界面就得自己攒着上一帧往前推，
// 丢一帧之后画出来的是不存在的那一半状态。
func (b *bridge) forward(ev contract.Event) {
	b.emit(ev.Name, ev)
	// Dropped 只在这条线上数过几帧，它一涨就写日志：丢一帧不影响正确性（下一条自带完整快照），
	// 但它一直涨说明窗口画不过来，这一句得留得下来，不许当没事。
	if n := b.client.Dropped(); n > b.dropped {
		b.logf("通道累计丢掉 %d 帧事件（每帧自带完整快照，界面看到的是此刻的真实全状态）", n)
		b.dropped = n
	}
}

// setState 只在"通↔不通"翻转、或者不通的那句错换了内容的那一刻推一条，不每秒重复推同一条。
// 为什么要连错的内容一起比：服务从"端点文件还没建出来"变成"连上了但答不了"是人要换招的信号，
// 只按通／不通推就会把后一句悄悄吞掉。
// 界面第一帧可能还没订阅上，所以它不指这一条起步：它自己第一次调用能不能成，才是它的第一手依据。
func (b *bridge) setState(connected bool, err error) {
	text := errText(err)
	if b.connected == connected && b.lastErr == text {
		return
	}
	b.connected = connected
	b.lastErr = text
	frame := contract.LinkState{Connected: connected, Endpoint: b.currentEndpoint(), Err: text, Dropped: b.client.Dropped()}
	b.emit(contract.WireState, frame)
}

// errText 把 nil 也变成一个能比的字符串，免得调用点到处写 if err != nil。
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// reMarshal 把通道上那份 any（解 JSON 出来的 map）转成本地结构体。
// 为什么不直接断言字段：断言只能一层一层抄，字段名一漂就静默拿到 nil；绕这一道 JSON，键名以 struct 标签为准。
func reMarshal(from any, to any) error {
	buf, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, to)
}

// splitErr 从一句错里把 "GAC-XXX-NNN" 这段码拿出来，剩下的原话留在 error 那一栏。
// 为什么要拆：界面那句人话是按码去语言文件换的（已定：D6 乙），整句塞进去等于让语言文件去猜一句它没见过的中文。
// 拆不出来时按两个哨兵错分档：连不上与形状对不上是两种完全不同的处理（前者重连，后者两边要同一份构建），
// 都写成"通道报错"就等于让人自己猜。
func splitErr(err error) (code, msg string) {
	msg = err.Error()
	if strings.HasPrefix(msg, "GAC-") {
		if i := strings.Index(msg, ":"); i > 0 {
			return msg[:i], strings.TrimLeft(msg[i+1:], " ")
		}
	}
	switch {
	case errors.Is(err, ipc.ErrNoConnection):
		return errs.CodeIPCDisconnect, msg
	case errors.Is(err, ipc.ErrVersionMismatch):
		return errs.CodeIPCShape, msg
	default:
		return errs.CodeIPCHandler, msg
	}
}
