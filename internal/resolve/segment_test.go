package resolve

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// 官方段这一档的算法全在 expandPrefixes／prefixAddrs 里，用例不碰外网：段清单直接塞进去。

func TestExpandPrefixesSpreadsAcrossPrefixes(t *testing.T) {
	// 四条 /24：每段取 3 个，limit 给到 12 就是全取；limit 收紧到 4 时必须是"每段各一个"，
	// 而不是"前一条段取满三个再加下一条"——后者等于只探了第一段。
	list := []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"}
	full := expandPrefixes(list, 3, 12)
	if len(full) != 12 {
		t.Fatalf("四条段各 3 个该有 12 个地址，实际 %d：%v", len(full), full)
	}

	few := expandPrefixes(list, 3, 4)
	var prefixes []string
	for _, ip := range few {
		prefixes = append(prefixes, strings.Join(strings.Split(ip, ".")[:3], "."))
	}
	if len(few) != 4 || strings.Join(prefixes, ",") != "10.0.0,10.0.1,10.0.2,10.0.3" {
		t.Errorf("截断后必须仍是一片不同段的地址，got %v", few)
	}
}

func TestExpandPrefixesIsDeterministic(t *testing.T) {
	list := []string{"192.30.252.0/22", "140.82.112.0/20", "185.199.108.0/22", "143.55.64.0/20"}
	// 同一批段，换了来路顺序也必须摊出同一个池：段清单要先排过再摊，不然同一时刻两轮结果不一样。
	a := expandPrefixes(list, CandidatesPerPrefix, MaxCandidatesPerMode)
	b := expandPrefixes([]string{list[2], list[0], list[3], list[1]}, CandidatesPerPrefix, MaxCandidatesPerMode)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("段清单顺序换了，候选池就换了：\n%v\n%v", a, b)
	}
	if len(a) != len(b) {
		t.Fatalf("两轮数量都不等")
	}
	for i := range a {
		if a[i] == b[i] {
			continue
		}
		t.Fatalf("两轮内容不同（第 %d 个 %s / %s）", i, a[i], b[i])
	}
}

func TestExpandPrefixesSkipsUnusableAndKeepsBothFamilies(t *testing.T) {
	got := expandPrefixes([]string{"读不懂的一段", "10.0.0.0/24", "fd00::/56", "", "10.0.0.0/24"}, 3, 20)
	if len(got) == 0 {
		t.Fatal("一段都没摊出来")
	}
	seen := map[string]bool{}
	var v4, v6 int
	for _, ip := range got {
		if seen[ip] {
			t.Errorf("候选池里有重复地址：%s", ip)
		}
		seen[ip] = true
		if strings.Contains(ip, ":") {
			v6++
		} else {
			v4++
		}
	}
	if v4 == 0 || v6 == 0 {
		t.Errorf("两个地址族都该有（v4 %d 个、v6 %d 个），got %v", v4, v6, got)
	}

	if out := expandPrefixes([]string{"读不懂的一段"}, 3, 20); len(out) != 0 {
		t.Errorf("全是坏段该交回空池，让 Resolve 报 002，实际 %v", out)
	}
}

func TestPrefixAddrsSkipsNetworkAndBroadcast(t *testing.T) {
	got := addrStrings(prefixAddrs(parsePrefix(t, "10.0.0.0/24"), 3))
	want := []string{"10.0.0.1", "10.0.0.128", "10.0.0.254"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("大段里网络地址与广播地址都不许写进 hosts：got %v, want %v", got, want)
	}
}

func TestPrefixAddrsHandlesTinyPrefixes(t *testing.T) {
	cases := []struct {
		prefix string
		want   string
	}{
		{"10.0.0.0/31", "10.0.0.0,10.0.0.1"}, // 点对点这一段两个地址都可用
		{"10.0.0.7/32", "10.0.0.7"},          // 单个地址：不许造出第二个来
		{"fd00::/127", "fd00::,fd00::1"},     // IPv6 同理
		{"fd00::/128", "fd00::"},             // 单个地址
	}
	for _, c := range cases {
		got := addrStrings(prefixAddrs(parsePrefix(t, c.prefix), CandidatesPerPrefix))
		if strings.Join(got, ",") != c.want {
			t.Errorf("%s：got %v, want %s", c.prefix, got, c.want)
		}
	}
}

func TestSegmentResolveUsesInjectedPool(t *testing.T) {
	m := &segment{prefixes: func(context.Context) ([]string, error) {
		return []string{"10.0.0.0/24", "10.0.1.0/24"}, nil
	}}

	got, err := m.Resolve(context.Background(), Query{Domain: "github.com"})
	if err != nil {
		t.Fatalf("Resolve 抛了：%v", err)
	}
	if got.ModeID != "segment" || len(got.IPs) == 0 {
		t.Errorf("got %+v", got)
	}
}

func TestSegmentErrors(t *testing.T) {
	down := &segment{prefixes: func(context.Context) ([]string, error) { return nil, errors.New("403 配额用完") }}
	_, err := down.Resolve(context.Background(), Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != "GAC-MODE-segment-001" {
		t.Errorf("取不到段清单该回 001，实际 %s（%v）", code, err)
	}
	if p := errs.ParamsOf(err); p["url"] != segmentMetaURL {
		t.Errorf("错误参数要带上是哪个地址问不到，实际 %v", p)
	}

	empty := &segment{prefixes: func(context.Context) ([]string, error) { return []string{"不是段"}, nil }}
	_, err = empty.Resolve(context.Background(), Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != "GAC-MODE-segment-002" {
		t.Errorf("摊不出地址该回 002，实际 %s（%v）", code, err)
	}
}

func TestSegmentPausedIsNotAFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()

	m := &segment{prefixes: func(context.Context) ([]string, error) { return []string{"10.0.0.0/24"}, nil }}
	_, err := m.Resolve(ctx, Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != errs.CodeTaskPaused {
		t.Errorf("该回 GAC-TASK-003，实际 %s（%v）", code, err)
	}
}

// 段清单必须只有一份。这条查的是"第二次问走没走缓存"——一轮里三档方式同读一份段清单，
// 不共用就会各问一次 meta，配额 60 次/小时几下就撞墙（第 6.5 节那句）。
func TestOfficialPrefixesCachedWithinTTL(t *testing.T) {
	// 用例只测"缓存这段逻辑"，不测真取数：这里直接查缓存的初始形状与 TTL 的关系，
	// 真取数那一支（fetchText）由 github520 那边的假件覆盖，不重复一遍。
	if MetaCacheTTL <= 0 {
		t.Fatalf("TTL 不是正数，缓存等于没有")
	}
	if metaCache.got && time.Since(metaCache.fetched) > MetaCacheTTL {
		t.Errorf("手里那份已经过期（%v）还标着有效，下一轮会拿旧段清单去探", time.Since(metaCache.fetched))
	}
}

func parsePrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("%s 读不懂：%v", s, err)
	}
	return p
}

func addrStrings(list []netip.Addr) []string {
	out := make([]string, len(list))
	for i, a := range list {
		out[i] = a.String()
	}
	return out
}
