package resolve

import (
	"context"
	"strings"
	"testing"

	"github.com/hlep/github-accel/internal/errs"
)

// 本地缓存这一档自己不记地址（第 8 节第 8 条：本包不许 import store），
// 样本由 service 从外面喂进来，所以用例只管"喂进来的这批怎么挑"。

func TestCacheResolveKeepsAddressShapesOnly(t *testing.T) {
	m := newCache()
	got, err := m.Resolve(context.Background(), Query{
		Domain: "github.com",
		Known: []string{"20.205.243.166", " fe80::1%eth0 ", "20.205.243.166",
			"这一条被外部改坏了", "", "::1", "10.0.0.255"},
	})
	if err != nil {
		t.Fatalf("Resolve 抛了：%v", err)
	}
	want := "20.205.243.166,fe80::1,::1,10.0.0.255"
	if strings.Join(got.IPs, ",") != want {
		t.Errorf("该去掉作用域后缀、按来路顺序去重、丢掉形状不对的，got %v, want %s", got.IPs, want)
	}
	if got.ModeID != "cache" {
		t.Errorf("ModeID 要能对上方式自己那一档，got %q", got.ModeID)
	}
}

func TestCacheEmptyIsExplicitNotSilent(t *testing.T) {
	m := newCache()
	_, err := m.Resolve(context.Background(), Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != "GAC-MODE-cache-001" {
		t.Errorf("第一次检测／这条域名还没被写过，该如实回 001 而不是交回一份空名单，实际 %s（%v）", code, err)
	}
	if p := errs.ParamsOf(err); p["domain"] != "github.com" {
		t.Errorf("错误参数要带上是哪条域名，实际 %v", p)
	}

	// 全是坏形状＝等于没有，也一样要报 001，不能让 service 拿到一个"成功但空"的名单去写 hosts。
	_, err = m.Resolve(context.Background(), Query{Domain: "github.com", Known: []string{"乱码", "999.1.1.1"}})
	if code := errs.CodeOf(err); code != "GAC-MODE-cache-001" {
		t.Errorf("喂进来的没一条是地址该回 001，实际 %s", code)
	}
}

func TestCachePausedIsNotAFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()

	_, err := newCache().Resolve(ctx, Query{Domain: "github.com", Known: []string{"1.1.1.1"}})
	if code := errs.CodeOf(err); code != errs.CodeTaskPaused {
		t.Errorf("该回 GAC-TASK-003，实际 %s（%v）", code, err)
	}
}

// 这一档最要紧的一条不在算法里，在 Spec 的形状里：命中缓存也必须探。
// NeedsProbe 一旦被改成假，缓存的比例永远是真的 1，主用会一辈子赖在它上面，
// 网络换了之后工具看着"一切正常"其实写的都是探不到的地址——这种烂法界面看不见。
func TestCacheStillProbesHits(t *testing.T) {
	s := specs(t, "cache")[0]
	if !s.NeedsProbe {
		t.Errorf("cache 的 NeedsProbe 被改成假了：命中缓存不再探，主用会永久赖在 cache 上，过期了界面也没人看得见")
	}
	if !s.Rankable {
		t.Errorf("cache 不参与两把尺子就没法被过期结果挤下去")
	}
}
