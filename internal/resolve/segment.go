package resolve

import (
	"context"
	"encoding/json"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// segment 把 GitHub 自己公布的 IP 段摊开成候选地址。
//
// 干什么用：社区源只给"每域名一个地址"，官方段给的是一整片地址；这一档负责后者，
//
//	交出去的那一批交给 detect 探出最优——它自己不判好坏。
//
// 什么情况下会错、出错时人看得见什么：
// ① 一个 /20 摊平是 4096 个地址，全探一遍就把用户网络跑死了 → 每个前缀只取 CandidatesPerPrefix 个，
//
//	整池再由 MaxCandidatesPerMode 截住，两个数都只有 budget.go 那一个家。
//
// ② meta 接口匿名只有 60 次/小时〔外部依据〕，一轮里反复读它必撞墙 → 段清单按 MetaCacheTTL 缓存一份。
// ③ 取数失败回 GAC-MODE-segment-001，段清单空得离谱回 002，界面显示"官方段没给回地址"。
//
// 怎么回来：这一档只认 web／api／git 三组段（实测够用：64 个 IPv4 前缀取 60 个候选，57 支握得上手）。
// 要加 Fastly 那类别的公布源，就照 R6 另加一个方式文件，别来改这个。

var segmentSpec = Spec{
	ID:              "segment",
	Title:           "官方 IP 段",
	Kinds:           []string{"github", "ai"},
	NeedsProbe:      true,
	DefaultPriority: 30,
	OfflineOK:       false,
	Rankable:        true,
}

const segmentMetaURL = "https://api.github.com/meta"

type segment struct {
	// prefixes 交回"官方此刻公布的段清单"。真机走 meta（带缓存），用例直接塞一批段，不靠外网。
	prefixes func(ctx context.Context) ([]string, error)
}

func newSegment() Resolver { return &segment{prefixes: officialPrefixes} }

func init() { must(Register(segmentSpec, newSegment)) }

func (m *segment) Spec() Spec { return cloneSpec(segmentSpec) }

func (m *segment) Resolve(ctx context.Context, q Query) (Set, error) {
	cctx, cancel, err := takeTimeout(ctx, segmentSpec.ID)
	if err != nil {
		return Set{}, err
	}
	defer cancel()

	started := time.Now()
	list, err := m.prefixes(cctx)
	if err != nil {
		if ctxErr := ctxError(segmentSpec.ID, cctx); ctxErr != nil {
			return Set{}, ctxErr
		}
		return Set{}, errs.New(errs.ModeCode(segmentSpec.ID, 1), map[string]any{"url": segmentMetaURL},
			"官方段清单没取到：%s", err)
	}

	ips := expandPrefixes(list, CandidatesPerPrefix, MaxCandidatesPerMode)
	if len(ips) == 0 {
		return Set{}, errs.New(errs.ModeCode(segmentSpec.ID, 2), map[string]any{"prefixes": len(list)},
			"官方段清单里一个能用的地址都没摊出来")
	}
	return Set{ModeID: segmentSpec.ID, IPs: ips, Took: time.Since(started)}, nil
}

// metaDoc 只取这三组段：web 41 条、api 27 条、git 62 条（2026-10-22 实测），
// 其余（packages、actions、codespaces 那几组）服务的是另一批域名，混进来只会摊出探不到的地址。
type metaDoc struct {
	Web []string `json:"web"`
	API []string `json:"api"`
	Git []string `json:"git"`
}

// metaCache 是全仓唯一一份官方段清单。
// 为什么缓存放在这个方式文件里：读它的只有这一档（第 6.5 节那条"别两处各读一遍"的护栏），
// 缓存跟着读者走，就不会长出第二个家；要复用它的第二档方式出现那天，这段就该搬进公共件并同轮改 G11。
var metaCache = struct {
	mu       sync.Mutex
	got      bool
	fetched  time.Time
	prefixes []string
}{mu: sync.Mutex{}}

// officialPrefixes 取段清单；没到 MetaCacheTTL 就直接复用手里那份。
func officialPrefixes(ctx context.Context) ([]string, error) {
	metaCache.mu.Lock()
	defer metaCache.mu.Unlock()

	if metaCache.got && time.Since(metaCache.fetched) < MetaCacheTTL {
		return metaCache.prefixes, nil
	}

	text, err := fetchText(ctx, segmentMetaURL)
	if err != nil {
		return nil, err
	}
	var doc metaDoc
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	list := append(append(append([]string(nil), doc.Web...), doc.API...), doc.Git...)
	metaCache.prefixes = list
	metaCache.fetched = time.Now()
	metaCache.got = len(list) > 0 // 一份空清单不算取到，下一轮还得再去问一次
	return list, nil
}

// expandPrefixes 把段摊成候选地址：每个前缀取最多 per 个（段首可用的那一个、主机位最高位单独置 1 的那一个、
// 段尾退一位的那一个），再按"先各段的第一个、再各段的第二个"轮转排开，最后截到 limit 个。
//
// 为什么要轮转：直接截前 limit 个地址等于只探前几条段，剩下的段一个没试；轮转保证截断之后仍是一片不同段的地址。
// 为什么每一步都确定性：同一个时刻拿到同一份段清单，两轮摊出来的候选必须一模一样。
func expandPrefixes(list []string, per, limit int) []string {
	parsed := make([]netip.Prefix, 0, len(list))
	for _, s := range list {
		p, err := netip.ParsePrefix(strings.TrimSpace(s))
		if err != nil || !p.IsValid() {
			continue // 读不懂的段跳过：官方接口偶尔加一种形状，不该让整池候选消失
		}
		parsed = append(parsed, p.Masked())
	}
	sort.SliceStable(parsed, func(i, j int) bool {
		return lessAddr(parsed[i].Addr(), parsed[j].Addr()) ||
			(parsed[i].Addr() == parsed[j].Addr() && parsed[i].Bits() < parsed[j].Bits())
	})

	slots := make([][]netip.Addr, len(parsed))
	for i, p := range parsed {
		slots[i] = prefixAddrs(p, per)
	}

	out := make([]string, 0, limit)
	seen := map[string]bool{}
	for k := 0; k < per; k++ { // 轮转：先取每段第 1 个，再取每段第 2 个
		for _, s := range slots {
			if k >= len(s) {
				continue
			}
			addr := s[k]
			if seen[addr.String()] {
				continue
			}
			seen[addr.String()] = true
			out = append(out, addr.String())
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

// prefixAddrs 给一条段造出至多 n 个可用地址。IPv4 大段里网络地址和广播地址不算可用，各退一格。
func prefixAddrs(p netip.Prefix, n int) []netip.Addr {
	base := p.Addr()
	hostBits := base.BitLen() - p.Bits()
	out := make([]netip.Addr, 0, n)

	push := func(a netip.Addr) {
		if len(out) >= n || !a.IsValid() || !p.Contains(a) {
			return
		}
		for _, have := range out {
			if have == a {
				return
			}
		}
		out = append(out, a)
	}

	if base.Is4() && hostBits > 1 {
		push(base.Next()) // 段首那一个是网络地址，跳过
	} else {
		push(base)
	}
	if hostBits > 1 {
		push(setHostBit(base, hostBits-1)) // 中间那一个：最高那一位主机位单独置 1
	}
	last := setHostBits(base, hostBits)
	if base.Is4() && hostBits > 1 {
		last = last.Prev() // 广播地址不可用
	}
	push(last)
	return out
}

// setHostBit 把主机区里从低往高数第 k 位置成 1（段首的基址主机位本来就是全 0）。
func setHostBit(base netip.Addr, k int) netip.Addr {
	b := base.AsSlice()
	if k/8 >= len(b) {
		return base
	}
	b[len(b)-1-k/8] |= 1 << uint(k%8)
	out, _ := netip.AddrFromSlice(b)
	return out
}

// setHostBits 把主机区全置成 1，得到这条段的最后一个地址。
func setHostBits(base netip.Addr, hostBits int) netip.Addr {
	b := base.AsSlice()
	if hostBits > len(b)*8 {
		hostBits = len(b) * 8 // 兜住 /0 这种整族段：置位下标不许越出地址本身
	}
	for k := 0; k < hostBits; k++ {
		b[len(b)-1-k/8] |= 1 << uint(k%8)
	}
	out, _ := netip.AddrFromSlice(b)
	return out
}

// lessAddr 按地址族（v4 在前）再按字节比大小——排序不靠字符串，免得 10.x 排到 9.x 后面。
func lessAddr(a, b netip.Addr) bool {
	if a.Is4() != b.Is4() {
		return a.Is4()
	}
	as, bs := a.AsSlice(), b.AsSlice()
	for i := range as {
		if as[i] != bs[i] {
			return as[i] < bs[i]
		}
	}
	return false
}
