package detect

import (
	"context"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	nurl "net/url"
	"strconv"
	"testing"
	"time"
)

// 指标一判的是"这个地址带着这个域名答不答"，这条判据不能靠真网络钉。
// 用的是本机 TLS 测试服务：连接钉到 127.0.0.1，名字留给域名——与真机上同一份代码、同一套校验。

type testServer struct {
	ip    string
	port  int
	roots *x509.CertPool
	// name 是这张测试证书真认的那个名字；用例不许把域名写死，证书换了形状用例要跟着走。
	name string
}

func newTestServer(t *testing.T, handler http.HandlerFunc) *testServer {
	t.Helper()

	srv := httptest.NewUnstartedServer(handler)
	srv.StartTLS()
	t.Cleanup(srv.Close)

	u, err := nurl.Parse(srv.URL)
	if err != nil {
		t.Fatalf("测试服务地址读不懂：%v", err)
	}
	ip, portText, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("测试服务地址拆不开：%v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("端口不是数：%v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	return &testServer{ip: ip, port: port, roots: roots, name: srv.Certificate().DNSNames[0]}
}

func (s *testServer) limits() Limits {
	return Limits{ProbeTimeout: 2 * time.Second, RTTRepeats: 1, Concurrency: 1, BodyBytes: 4096, Port: s.port}
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

func TestProberServesWhenCertMatchesDomain(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(okHandler))
	p := prober{roots: srv.roots}

	got := p.ask(context.Background(), srv.name, srv.ip, srv.limits())
	if !got.CanServe || got.Shape != ShapeServed {
		t.Fatalf("名字对得上、对面答了 200，该算探上：%+v（%s）", got, got.Detail)
	}
	if got.Status != http.StatusOK {
		t.Errorf("状态码该带回来给界面看，got %d", got.Status)
	}
	if got.BodyBytes == 0 {
		t.Errorf("正文一个字节都没读回来，那这次探测等于没看回答")
	}
}

// 连接钉的是 IP，但 Host 与 SNI 必须是域名——写成 IP 就退化成"这台机器开了 443"，
// 而证书验的是 URL 里的名字，两者会一起对上，测不出问题。
func TestProberSendsDomainNotAddress(t *testing.T) {
	var seen string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen = req.Host
		_, _ = w.Write([]byte("ok"))
	}))
	p := prober{roots: srv.roots}

	got := p.ask(context.Background(), srv.name, srv.ip, srv.limits())
	if !got.CanServe {
		t.Fatalf("没探上：%+v（%s）", got, got.Detail)
	}
	if seen != srv.name {
		t.Errorf("Host 头该是域名 %s，实际 %q——发成地址就成了只问 443 开没开", srv.name, seen)
	}
}

// 证书只认 example.com：换成别的名字必须判"这个地址不服务这条域名"，
// 而且要落在 badCert 那一档（阻断那张表里没有它），否则清单里写错一条就会让整条域名被标成"换 IP 无效"。
func TestProberRejectsDomainTheCertDoesNotServe(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(okHandler))
	p := prober{roots: srv.roots}

	got := p.ask(context.Background(), "not-"+srv.name, srv.ip, srv.limits())
	if got.CanServe || got.Shape != ShapeBadCert {
		t.Errorf("名字对不上该记 badCert 且不算探上，got %+v（%s）", got, got.Detail)
	}
}

// 系统根认不出这张自签证书：同样是不服务这条域名，不许误记成超时／阻断。
func TestProberUnknownAuthorityIsNotATimeout(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(okHandler))
	p := prober{} // 不给 roots＝用系统那一份，判的代码同一条路径

	got := p.ask(context.Background(), srv.name, srv.ip, srv.limits())
	if got.CanServe || got.Shape != ShapeBadCert {
		t.Errorf("根认不出该记 badCert，got %+v（%s）", got, got.Detail)
	}
}

// 对面回了 5xx＝这条地址答非所问，不能算"能服务"（否则把源站故障写进 hosts 还标成功）。
func TestProberTreatsServerErrorAsNotServing(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	p := prober{roots: srv.roots}

	got := p.ask(context.Background(), srv.name, srv.ip, srv.limits())
	if got.CanServe || got.Shape != ShapeProtocol || got.Status != http.StatusBadGateway {
		t.Errorf("5xx 该记 protocol 并带回状态码，got %+v", got)
	}
}

// 4xx 算能服务：这条域名在这台边缘节点上是认的，只是这个路径没内容（比如根路径 404）。
func TestProberAcceptsClientError(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	p := prober{roots: srv.roots}

	got := p.ask(context.Background(), srv.name, srv.ip, srv.limits())
	if !got.CanServe || got.Status != http.StatusNotFound {
		t.Errorf("404 也是这条地址认这个域名的证据，got %+v", got)
	}
}

// BodyBytes 的上限真的生效：源再怎么灌，也只读回来 lim.BodyBytes 个。
func TestProberCapsBodyRead(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		buf := make([]byte, 4096)
		for i := range buf {
			buf[i] = 'a'
		}
		_, _ = w.Write(buf)
	}))
	p := prober{roots: srv.roots}
	lim := srv.limits()
	lim.BodyBytes = 8

	got := p.ask(context.Background(), srv.name, srv.ip, lim)
	if !got.CanServe || got.BodyBytes != 8 {
		t.Errorf("正文该截到上限 8 字节，got %+v", got)
	}
}

// 端口上没人听＝refused。它不进阻断那张表：本机没开这条服务不等于域名被封锁。
func TestProberRefusedShape(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("开不了本机端口：%v", err)
	}
	addrStr := ln.Addr().String()
	_ = ln.Close() // 立刻关掉，留一个没人听的端口

	ip, portText, err := net.SplitHostPort(addrStr)
	if err != nil {
		t.Fatalf("地址拆不开：%v", err)
	}
	port, _ := strconv.Atoi(portText)
	p := prober{}
	lim := Limits{ProbeTimeout: time.Second, RTTRepeats: 1, Concurrency: 1, BodyBytes: 64, Port: port}

	got := p.ask(context.Background(), "example.com", ip, lim)
	if got.CanServe || got.Shape != ShapeRefused {
		t.Errorf("没人听的端口该记 refused，got %+v（%s）", got, got.Detail)
	}
}

// 接了 TCP 但不回 TLS＝到点不答，记 timeout；这一档才是阻断表里的形状。
func TestProberSilentServerIsTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("开不了本机端口：%v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// 只握着，不发一个字节：像被中间设备半路拖住那样。
			go func() { time.Sleep(3 * time.Second); _ = conn.Close() }()
		}
	}()

	_, portText, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("地址拆不开：%v", err)
	}
	port, _ := strconv.Atoi(portText)
	p := prober{}
	lim := Limits{ProbeTimeout: 200 * time.Millisecond, RTTRepeats: 1, Concurrency: 1, BodyBytes: 64, Port: port}

	ctx, cancel := context.WithTimeout(context.Background(), lim.ProbeTimeout)
	defer cancel()
	got := p.ask(ctx, "example.com", "127.0.0.1", lim)
	if got.CanServe || got.Shape != ShapeTimeout {
		t.Errorf("对面不回话该记 timeout，got %+v（%s）", got, got.Detail)
	}
}

func TestShapeOfCancelledIsNotAFailureShape(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := shapeOf(ctx.Err()); got != ShapeOther {
		t.Errorf("被叫停不该进任何网络形状（它不该拉低稳定性），got %s", got)
	}
	// 同一只 ctx 到点才是超时形状：两个码在 resolve 那侧也是分开的（GAC-TASK-003 对 GAC-DETECT-002）。
	dead, cancelDead := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancelDead()
	time.Sleep(2 * time.Millisecond)
	if got := shapeOf(dead.Err()); got != ShapeTimeout {
		t.Errorf("到点该记 timeout，got %s", got)
	}
}
