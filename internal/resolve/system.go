package resolve

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// system 问一次"这台机器现在认为这条域名在哪儿"，也就是原型界面那行"在线查询"。
//
// 干什么用：它是基准答案——不写 hosts 时用户拿到的就是它；有了它，界面上"当前地址"和"我们建议的地址"才摆得在一起。
//
// 什么情况下会错、出错时人看得见什么：这一档有一个别档没有的坑，必须写在这里——
//
//	本机解析器读的是 nsswitch 那一套，第一件事就是翻 /etc/hosts，
//	所以我们自己写进去的那一行，它会当成"在线查询的新答案"原样交回来。
//	后果不是写坏文件，而是名次：它看起来永远成功（S＝1），主用容易赖在它上面不动。
//
// 怎么回来：三道收口都在这一档自己的形状里——① NeedsProbe 是真，交回的地址照样探，探不上就掉 S；
//
//	② DefaultPriority 排在缓存、社区源、官方段之后，冷启动那次它不会顶在前面；
//	③ 界面那一行显示的是"在线查询"，用户看得见它给的是本机现在的看法，不是新查到的。
//
// 真要它绕开 hosts 只问 DNS，标准库做不到（net.Resolver 两种实现都翻 hosts），
//
//	那属于"另加一档走 UDP 直查 53 端口的方式"——照 R6 加一个文件就行，今天不塞进这一档。
var systemSpec = Spec{
	ID:              "system",
	Title:           "在线查询",
	Kinds:           []string{"ai", "cdn", "coursera", "custom", "gameplat", "gamespec", "github", "jetbrains", "ms", "steam"},
	NeedsProbe:      true,
	DefaultPriority: 40,
	OfflineOK:       true,
	Rankable:        true,
}

type system struct {
	// lookup 是"给我域名，还我这台机器现在的看法"。用例里换成假件，判据不靠真 DNS。
	lookup func(ctx context.Context, domain string) ([]string, error)
}

func newSystem() Resolver { return &system{lookup: localLookup} }

func init() { must(Register(systemSpec, newSystem)) }

func (m *system) Spec() Spec { return cloneSpec(systemSpec) }

func (m *system) Resolve(ctx context.Context, q Query) (Set, error) {
	cctx, cancel, err := takeTimeout(ctx, systemSpec.ID)
	if err != nil {
		return Set{}, err
	}
	defer cancel()

	started := time.Now()
	list, err := m.lookup(cctx, q.Domain)
	if err != nil {
		if ctxErr := ctxError(systemSpec.ID, cctx); ctxErr != nil {
			return Set{}, ctxErr
		}
		return Set{}, errs.New(errs.ModeCode(systemSpec.ID, 1), map[string]any{
			"domain": q.Domain,
		}, "本机解析答不出 %s：%s", q.Domain, err)
	}

	var out []string
	seen := map[string]bool{}
	for _, raw := range list {
		ip, _, _ := strings.Cut(raw, "%") // 带作用域后缀的 IPv6 写进 hosts 没意义
		if net.ParseIP(ip) == nil || seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	if len(out) == 0 {
		return Set{}, errs.New(errs.ModeCode(systemSpec.ID, 2), map[string]any{
			"domain": q.Domain,
		}, "本机解析给回 %s 的结果里没有地址形状", q.Domain)
	}
	return Set{ModeID: systemSpec.ID, IPs: out, Took: time.Since(started)}, nil
}

func localLookup(ctx context.Context, domain string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, domain)
}
