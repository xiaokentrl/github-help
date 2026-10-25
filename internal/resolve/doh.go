package resolve

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// doh 往公共 DoH 端点问一次解析。
//
// 干什么用：本机 DNS 常被按名字给假地址，DoH 走 HTTPS 问一次，能给出一批不一样的候选。
//
// 什么情况下会错、出错时人看得见什么：这一档最容易"看起来永远失败"，因为端点本身就是被拦的对象——
//
//	2026-10-22 在本机重量：dns.google 在 6 秒整超时，cloudflare-dns.com 的 TLS 被对端重置，
//	连 8.8.8.8:443 的 TCP 都握不上；也就是说这一档今天必然交回失败，界面上显示的是
//	"GAC-MODE-doh-001 DoH 端点都没连上"，不是"这条域名没法加速"。
//	端点通了却答不出这条域名，回 002。
//
// 怎么回来：端点清单只在这个文件里；M2 第一天要按技术方案第 8.2 节末那句重新量一次通不通，
//
//	量完把读数写进技术方案第 15 节 E6 那一条——不许因为"今天不通"就把这一档删掉，
//	它失败会被 S 记下来，Rank 自然把它排到最后，界面也看得见它为什么排最后。
var dohSpec = Spec{
	ID:              "doh",
	Title:           "DoH 优选",
	Kinds:           []string{"ai", "cdn", "coursera", "custom", "gameplat", "gamespec", "github", "jetbrains", "ms", "steam"},
	NeedsProbe:      true,
	DefaultPriority: 50,
	OfflineOK:       false,
	Rankable:        true,
}

// dohEndpoints 两支公共端点，都要 JSON 形式的回答（application/dns-json）。
// 只问 A 记录：这批菜单的域名实测以 IPv4 为多（官方段那 130 条里 IPv4 64 条），
// 多问一次 AAAA 就是多占一次取数预算，不值。
var dohEndpoints = []string{
	"https://dns.google/resolve",
	"https://cloudflare-dns.com/dns-query",
}

type doh struct {
	get func(ctx context.Context, url string) (string, error)
}

func newDoH() Resolver { return &doh{get: fetchText} }

func init() { must(Register(dohSpec, newDoH)) }

func (m *doh) Spec() Spec { return cloneSpec(dohSpec) }

func (m *doh) Resolve(ctx context.Context, q Query) (Set, error) {
	cctx, cancel, err := takeTimeout(ctx, dohSpec.ID)
	if err != nil {
		return Set{}, err
	}
	defer cancel()

	started := time.Now()
	var lastErr error
	for _, endpoint := range dohEndpoints {
		text, err := m.get(cctx, dohQuery(endpoint, q.Domain))
		if err != nil {
			if ctxErr := ctxError(dohSpec.ID, cctx); ctxErr != nil {
				return Set{}, ctxErr
			}
			lastErr = err
			continue
		}
		ips := dohAnswers(text, q.Domain)
		if len(ips) > 0 {
			return Set{ModeID: dohSpec.ID, IPs: ips, Took: time.Since(started)}, nil
		}
	}
	if lastErr != nil {
		return Set{}, errs.New(errs.ModeCode(dohSpec.ID, 1), map[string]any{
			"domain": q.Domain, "endpoints": len(dohEndpoints),
		}, "DoH 端点一个都没连上（最后一次：%s）", lastErr)
	}
	return Set{}, errs.New(errs.ModeCode(dohSpec.ID, 2), map[string]any{
		"domain": q.Domain,
	}, "DoH 答不出 %s 的地址", q.Domain)
}

func dohQuery(endpoint, domain string) string {
	return endpoint + "?name=" + url.QueryEscape(domain) + "&type=A"
}

// dohAnswer 是 DoH JSON 的一条回答。type 是数字，这里不写数字常量去比它——
// 判"这条回答是不是一个地址"用 net.ParseIP 就够，CNAME 那类 data 是名字，一律进不来。
type dohAnswer struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type dohReply struct {
	Answer []dohAnswer `json:"Answer"`
}

func dohAnswers(text, domain string) []string {
	var reply dohReply
	if err := json.Unmarshal([]byte(text), &reply); err != nil {
		return nil // 回答不是预期形状＝这个端点这次没答上，换下一支
	}
	var out []string
	seen := map[string]bool{}
	for _, a := range reply.Answer {
		name := strings.TrimSuffix(strings.TrimSpace(a.Name), ".")
		if !strings.EqualFold(name, domain) {
			continue
		}
		ip := strings.TrimSpace(a.Data)
		if net.ParseIP(ip) == nil || seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	return out
}
