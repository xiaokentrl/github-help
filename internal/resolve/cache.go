package resolve

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// cache 交回"上一轮检测确认能服务这条域名"的地址。
//
// 干什么用：一轮检测里最便宜的来源是刚验过的那个地址——它已经服务成功过一次，取它不花任何网络开销。
//
// 什么情况下会错、出错时人看得见什么：手里没有上一轮结果时（第一次点检测、或这条域名还没被写过）
//
//	它回 GAC-MODE-cache-001"还没有上一轮结果"，Rank 那一步本来就把它排在有结果的方式后面，界面上一个字不差。
//
// 另一个坑更要紧：网络今天变了、昨天能用的地址今天不能用了，如果"命中缓存就跳过探测"，
//
//	这个方式的比例永远是 1，主用会一辈子赖在 cache 上不动，工具看着能用其实早就过期。
//	所以 NeedsProbe 是真——命中缓存也照样探一次，只是候选就一个地址，代价小。
//
// 怎么回来：地址由 service 从样本里喂进来（Query.Known）。本包不许 import store（第 8 节第 8 条），
//
//	所以这一档自己一个地址都不记；要查"缓存为什么不命中"，问 service 手上那份样本。
var cacheSpec = Spec{
	ID:              "cache",
	Title:           "本地缓存",
	Kinds:           []string{"ai", "cdn", "coursera", "custom", "gameplat", "gamespec", "github", "jetbrains", "ms", "steam"},
	NeedsProbe:      true,
	DefaultPriority: 10,
	OfflineOK:       true,
	Rankable:        true,
}

type cache struct{}

func newCache() Resolver { return &cache{} }

func init() { must(Register(cacheSpec, newCache)) }

func (m *cache) Spec() Spec { return cloneSpec(cacheSpec) }

func (m *cache) Resolve(ctx context.Context, q Query) (Set, error) {
	if err := ctxError(cacheSpec.ID, ctx); err != nil {
		return Set{}, err
	}

	started := time.Now()
	var out []string
	seen := map[string]bool{}
	for _, raw := range q.Known {
		// 样本是落在磁盘上的，可能被外部改出空格；带作用域后缀的（fe80::1%eth0）写进 hosts 没意义，先剥掉再判形状。
		ip, _, _ := strings.Cut(strings.TrimSpace(raw), "%")
		if net.ParseIP(ip) == nil || seen[ip] {
			continue // 形状不对的（样本被外部改坏）丢掉，不写进 hosts
		}
		seen[ip] = true
		out = append(out, ip)
	}
	if len(out) == 0 {
		return Set{}, errs.New(errs.ModeCode(cacheSpec.ID, 1), map[string]any{
			"domain": q.Domain,
		}, "%s 还没有上一轮检测留下的地址", q.Domain)
	}
	return Set{ModeID: cacheSpec.ID, IPs: out, Took: time.Since(started)}, nil
}
