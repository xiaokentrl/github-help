package detect

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
)

// probe.go 是指标一：带上正确域名发一次请求，看对面答不答。
//
// 干什么用：把连接钉到候选地址、把 SNI 与 Host 留给域名（curl --resolve 那一路），
//
//	所以问的是"这个 IP 真能不能服务这条域名"，不会退化成"这个 IP 有没有开 443"。
//
// 什么情况下会错、出错时人看得见什么：
// ① 只做 TLS 握手就当探上 → 握手成功但对面答非所问，写进 hosts 后浏览器仍然打不开，
//
//	界面却写着"成功"。所以这里一定要把请求发出去、把状态码拿回来。
//
// ② 按 IP 验证书名字 → GitHub 证书没有 IP SAN，一条都验不过，全批看起来全坏。
//
//	名字是拿去跟**域名**对的：验不过记 badCert（这个地址不是这条域名的正主），它不算阻断形状。
//
// ③ 对面答的不是 HTTP／回 5xx → 记 protocol，不当成"能用"。
//
// 怎么回来：Shape 那张表在本包 doc.go；哪个形状算能用、算阻断，判据分别在 rankips.go 与 blocked.go，各一处。

// prober 是指标一的真件。
//
// roots 为空就用系统那一份根证书：判据必须是这台机器真认的那套名字，
// 自己造一根等于"任何一张自签证书都算过"，那才是真的探不出好坏。
// 用例要塞进测试自签那一张，才能同时测到"名字对得上"与"名字对不上"两条分支——
// 换的是这一个入参，判的代码仍是同一份，不会出现"用例过的路径真机上没跑过"。
type prober struct {
	roots *x509.CertPool
}

func (p prober) ask(ctx context.Context, domain, ip string, lim Limits) Outcome {
	client := p.client(ip, lim.Port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+domain+"/", nil)
	if err != nil {
		return Outcome{IP: ip, Shape: ShapeOther, Detail: err.Error()}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Outcome{IP: ip, Shape: shapeOf(err), Detail: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	// 正文只数个数、不判内容：拿正文认拦截页缺实测依据（技术方案第 15 节 E6 那条），今天不写这一判据。
	body, _ := io.ReadAll(io.LimitReader(resp.Body, int64(lim.BodyBytes)))
	if resp.StatusCode >= http.StatusInternalServerError {
		return Outcome{IP: ip, Shape: ShapeProtocol, Status: resp.StatusCode, BodyBytes: len(body),
			Detail: "对面回了 " + strconv.Itoa(resp.StatusCode)}
	}
	return Outcome{IP: ip, CanServe: true, Shape: ShapeServed, Status: resp.StatusCode, BodyBytes: len(body)}
}

// client 造一个"连接钉到候选地址、名字留给域名"的客户端。
//
// 超时帽子由调用方给（传进来的 ctx 里已经装好 lim.ProbeTimeout），这里不再自己掐一次表——
// 两处各掐一次＝两个上限家，正是门禁要挡的那种形状。
func (p prober) client(ip string, port int) *http.Client {
	dialer := &net.Dialer{}
	target := addr(ip, port)
	transport := &http.Transport{
		// 名字（SNI 与证书校验对象）由 URL 里的域名给，这里只把"连到哪儿"换掉。
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, target)
		},
		TLSClientConfig: &tls.Config{RootCAs: p.roots},
		// 一支候选只用一次，不留连接池：留着下一轮还会拿到这条已经凉了的连接。
		DisableKeepAlives: true,
	}
	return &http.Client{Transport: transport}
}

// shapeOf 把一个失败原因归到表里那一档。归不进任何一档的一律记 other，不硬塞。
//
// 为什么非要分档：blocked.go 那句"全批一条都没答，疑似被阻断"只有靠形状才判得出来——
// 把 badCert（地址不服务这条域名）也算进阻断，一个源里配错的地址就会让整条域名被标成"换 IP 无效"，
// 从此不再重试，界面写着"被阻断"而其实只是清单写错了。
func shapeOf(err error) Shape {
	switch {
	case errors.Is(err, context.Canceled):
		return ShapeOther // 被叫停不是网络形状：它不该进任何统计
	case errors.Is(err, context.DeadlineExceeded):
		return ShapeTimeout
	case errors.Is(err, syscall.ECONNREFUSED):
		return ShapeRefused
	case errors.Is(err, syscall.ECONNRESET):
		return ShapeReset
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return ShapeNoRoute
	}

	// 证书那两条要在"是不是超时"之前认：一张名字不对的证书也是从 http.Client 里以 *url.Error 包出来的，
	// 先判成超时就会进阻断那张表，把"清单写错"演成"这条域名被封锁、换 IP 无效"。
	var certUnknown x509.UnknownAuthorityError
	if errors.As(err, &certUnknown) {
		return ShapeBadCert
	}
	var hostErr x509.HostnameError
	if errors.As(err, &hostErr) {
		return ShapeBadCert
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ShapeTimeout
	}
	if isResetText(err) {
		return ShapeReset
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return ShapeNoRoute
	}
	return ShapeOther
}

// isResetText 兜住 Windows 那一侧：那边的"连接被重置"是一个带文字的 socket 错误，
// 不一定能命中 syscall.ECONNRESET 这个具名错误〔外部依据：WSA 错误号在 Go 侧以 *net.OpError + 文本出现〕。
// 判据只认这一句里的固定说法，不参与别的猜测；认不出还是 other。
func isResetText(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "existing connection was forcibly closed") ||
		strings.Contains(msg, "connection was forcibly closed")
}
