package resolve

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// github520 读社区源那份"一个域名一行、一行一个 IP"的清单。
//
// 干什么用：社区一直在替这批域名挑地址，直接拿来当候选，常常比本机 DNS 给的可用——
//
//	这就是原型界面"IP 来源"那一行显示 GitHub520 的那一档。
//
// 什么情况下会错、出错时人看得见什么：
// ① 源本身会过期，它给的地址今天可能已经握不上手 → 所以这一档 NeedsProbe＝真探过才算数，方式自己不判好坏；
// ② 两支镜像都连不通 → 回 GAC-MODE-github520-001，界面显示"社区源连不通"；
// ③ 源里没这条域名 → 回 GAC-MODE-github520-002，界面显示"这一条没有社区来源"，都不演成"检测成功"。
//
// 怎么回来：镜像清单只在这个文件里，加一支、换一支都只改这一个文件（R6）。

var github520Spec = Spec{
	ID:              "github520",
	Title:           "GitHub520 社区源",
	Kinds:           []string{"github", "ai"},
	NeedsProbe:      true,
	DefaultPriority: 20,
	OfflineOK:       false,
	Rankable:        true,
}

// github520Sources 按 2026-10-22 本机实测排：前两支今天通（0.22 秒／0.47 秒），第三支是源站本身，今天连不通但它是原始出处。
// Kinds 也是量出来的：把源里那 40 条域名与菜单的 54 条对一遍，命中 github 10 条、ai 1 条，其余块零命中。
var github520Sources = []string{
	"https://raw.hellogithub.com/hosts",
	"https://gh-proxy.com/https://raw.githubusercontent.com/ineo6/hosts/master/hosts",
	"https://raw.githubusercontent.com/ineo6/hosts/master/hosts",
}

type github520 struct {
	// get 是"给我一个地址，还我一份文本"。真机上它就是 doc.go 里那支 http 取数，
	// 用例里换成假件——外网不可控，判据必须能钉死返回值。
	get func(ctx context.Context, url string) (string, error)
}

func newGithub520() Resolver { return &github520{get: fetchText} }

func init() { must(Register(github520Spec, newGithub520)) }

func (m *github520) Spec() Spec { return cloneSpec(github520Spec) }

func (m *github520) Resolve(ctx context.Context, q Query) (Set, error) {
	cctx, cancel, err := takeTimeout(ctx, github520Spec.ID)
	if err != nil {
		return Set{}, err
	}
	defer cancel()

	started := time.Now()
	var lastErr error
	for _, url := range github520Sources {
		text, err := m.get(cctx, url)
		if err != nil {
			if ctxErr := ctxError(github520Spec.ID, cctx); ctxErr != nil {
				return Set{}, ctxErr
			}
			lastErr = err
			continue // 这一源没给回内容，换下一源
		}
		ips := github520Lookup(text, q.Domain)
		if len(ips) > 0 {
			return Set{ModeID: github520Spec.ID, IPs: ips, Took: time.Since(started)}, nil
		}
	}

	if lastErr != nil {
		return Set{}, errs.New(errs.ModeCode(github520Spec.ID, 1), map[string]any{
			"domain": q.Domain, "sources": len(github520Sources),
		}, "社区源一支都没连上（最后一次：%s）", lastErr)
	}
	return Set{}, errs.New(errs.ModeCode(github520Spec.ID, 2), map[string]any{
		"domain": q.Domain,
	}, "社区源里没有 %s 这一条", q.Domain)
}

// github520Lookup 从一份 hosts 形状的文本里挑出这条域名的地址，顺序按源里给的，已去重。
// 看不懂的行跳过：这份清单是人维护的，偶尔一行写歪不该让整份清单作废。
func github520Lookup(text, domain string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		ip := fields[0]
		if net.ParseIP(ip) == nil {
			continue
		}
		for _, name := range fields[1:] {
			if !strings.EqualFold(name, domain) {
				continue
			}
			if !seen[ip] {
				seen[ip] = true
				out = append(out, ip)
			}
		}
	}
	return out
}
