package ipc

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
)

// Handler 处理一个方法调用，返回要写回去的结果。
// 报错只写一句人话，错误码前缀由通道层统一补，别让每个方法自己拼。
type Handler func(params map[string]any) (any, error)

// Server 是服务侧的通道：收请求、调方法、把事件推给每个连着的界面。
type Server struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	sessions map[*session]struct{}
	// requirePeer 判定这个连接能不能用：本机不是谁都能连上就算自己人。
	requirePeer func(net.Conn) error
}

// AllowAnyPeer 是"这条线不核身份"那一份判据的**唯一**写法。
// 它存在的意义是让读代码的人一眼分得清：这里到底是有意不核（用例里的临时端点，跑完就删），
// 还是漏了（真端点上出现这一行＝谁连上都能改 hosts）。
var AllowAnyPeer = func(net.Conn) error { return nil }

// NewServer 建通道。requirePeer 判"这个连接能不能用"：本机不是谁都能连上就算自己人。
// 给 nil 就是"这条线不核身份"——但那一句要写得出来才行，所以传 nil 这条路直接堵掉，
// 用例请显式用 AllowAnyPeer（少这一个字母，屏幕上什么都没变；多这一行 panic，漏检会被当场看见）。
func NewServer(requirePeer func(net.Conn) error) *Server {
	if requirePeer == nil {
		panic("ipc: requirePeer 给了 nil：核身份这一步不许空着，用例请显式传 ipc.AllowAnyPeer")
	}
	return &Server{
		handlers:    make(map[string]Handler),
		sessions:    make(map[*session]struct{}),
		requirePeer: requirePeer,
	}
}

// Register 挂一个方法。重名直接 panic——这是编程错误，不该等到线上才发现。
func (s *Server) Register(method string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.handlers[method]; dup {
		panic(fmt.Sprintf("ipc: 方法 %q 重复注册", method))
	}
	s.handlers[method] = h
}

// Serve 接一个监听器，一直服务到它被关掉。
func (s *Server) Serve(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go s.serveConn(c)
	}
}

func (s *Server) serveConn(c net.Conn) {
	defer c.Close()
	if s.requirePeer != nil {
		if err := s.requirePeer(c); err != nil {
			enc := newEncoder(c)
			_ = enc.write(contract.Response{OK: false, Code: errs.CodeIPCPeer, Err: err.Error()})
			return
		}
	}
	sess := &session{enc: newEncoder(c), srv: s}
	s.mu.Lock()
	s.sessions[sess] = struct{}{}
	s.mu.Unlock()
	defer sess.drop()
	dec := newDecoder(c)
	for {
		req, err := dec.next()
		if err != nil {
			return
		}
		sess.handle(req)
	}
}

// Broadcast 按契约里的事件名，把一份完整状态快照（外加这一条事件专属的那几个数）推给每个连着的界面。
// payload 给 nil 就是"这条事件只带状态"——进度类事件带的是"第几条／共几条／哪个方式"这类数，
// 它们不属于状态快照，但界面画那一行又要用，所以跟快照同一条行发出去，不分两次。
// 只发两个字段是这里唯一不许的做法——界面拿到就得是自己画得出来的那份。
//
// 事件这一侧带不带协议号？不带，是 deliberate：快照的解法由字段自己撑（认不出来的多余字段忽略、
// 缺的字段留空＝"不知道"），一条漂了的回执比一条不推更坏——界面会干等一条永不再来的推送。
// 版本对不上这件事由每一条回执里的 Protocol 兜住，界面开机第一次问话就撞得见。
func (s *Server) Broadcast(event string, snap contract.Snapshot, payload map[string]any) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for sess := range s.sessions {
		_ = sess.enc.write(contract.Event{Name: event, Snapshot: snap, Payload: payload})
	}
}

type session struct {
	enc *encoder
	srv *Server
}

func (sess *session) drop() {
	sess.srv.mu.Lock()
	delete(sess.srv.sessions, sess)
	sess.srv.mu.Unlock()
}

func (sess *session) handle(req *contract.Request) {
	resp := contract.Response{ID: req.ID, OK: true, Protocol: contract.ProtocolVersion}
	sess.srv.mu.RLock()
	h := sess.srv.handlers[req.Method]
	sess.srv.mu.RUnlock()

	switch {
	case h == nil:
		resp.OK = false
		resp.Code = errs.CodeIPCNoHandler
		resp.Err = "服务不认这个方法: " + req.Method
	default:
		out, err := h(req.Params)
		if err != nil {
			resp.OK = false
			// 码由错误自己带（errs.CodeOf），不是这里现编：方法里回的是 GAC-DETECT-005 还是 GAC-TASK-004，
			// 界面要说的是两句不同的话。认不出码的兜底才是 012，参数一起带出去（见 contract.Response.Params）。
			code := errs.CodeOf(err)
			if code == "" || code == errs.CodeUnknown {
				code = errs.CodeIPCHandler
			}
			resp.Code, resp.Err, resp.Params = code, err.Error(), errs.ParamsOf(err)
		} else {
			resp.Result = out
		}
	}
	_ = sess.enc.write(resp)
}
