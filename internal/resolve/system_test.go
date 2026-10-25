package resolve

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hlep/github-accel/internal/errs"
)

// 在线查询这一档的判据全在假件里：真 DNS 给什么不由我们定，但"给了怎么挑、问不到回哪个码"必须钉住。

func TestSystemResolveKeepsAddressShapesOnly(t *testing.T) {
	m := &system{lookup: func(context.Context, string) ([]string, error) {
		return []string{"140.82.113.26", "fe80::1%eth0", "140.82.113.26", "不是一个地址", "::1"}, nil
	}}

	got, err := m.Resolve(context.Background(), Query{Domain: "github.com"})
	if err != nil {
		t.Fatalf("Resolve 抛了：%v", err)
	}
	if strings.Join(got.IPs, ",") != "140.82.113.26,fe80::1,::1" {
		t.Errorf("该去重、剥掉作用域后缀、丢掉形状不对的，got %v", got.IPs)
	}
	if got.ModeID != "system" {
		t.Errorf("ModeID 要能对上方式自己那一档，got %q", got.ModeID)
	}
}

func TestSystemErrorsDistinguishNoAnswerFromBadShape(t *testing.T) {
	fail := &system{lookup: func(context.Context, string) ([]string, error) {
		return nil, errors.New("no such host")
	}}
	_, err := fail.Resolve(context.Background(), Query{Domain: "查不到的.example"})
	if code := errs.CodeOf(err); code != "GAC-MODE-system-001" {
		t.Errorf("本机答不出该回 001，实际 %s（%v）", code, err)
	}
	if p := errs.ParamsOf(err); p["domain"] != "查不到的.example" {
		t.Errorf("错误参数要带上是哪条域名，实际 %v", p)
	}

	// 答回来了但一条地址形状都没有：这是另一回事（例如被按名字给了乱码），码得分开，界面那句才不是说谎的。
	weird := &system{lookup: func(context.Context, string) ([]string, error) {
		return []string{"乱码", ""}, nil
	}}
	_, err = weird.Resolve(context.Background(), Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != "GAC-MODE-system-002" {
		t.Errorf("答回来的没一条是地址该回 002，实际 %s（%v）", code, err)
	}
}

func TestSystemPausedIsNotAFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()

	m := &system{lookup: func(context.Context, string) ([]string, error) { return []string{"1.1.1.1"}, nil }}
	_, err := m.Resolve(ctx, Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != errs.CodeTaskPaused {
		t.Errorf("该回 GAC-TASK-003，实际 %s（%v）", code, err)
	}
}

// 这一档的坑是"自己写的 hosts 会被自己读到"：它看起来永远成功，主用容易赖在它上面。
// 三道收口里的两道写在 Spec 的形状里，改了就等于把坑填回去——所以这里钉住形状，不钉注释。
func TestSystemCannotWinColdStartOrSkipProbe(t *testing.T) {
	s := specs(t, "system")[0]
	if !s.NeedsProbe {
		t.Errorf("system 的 NeedsProbe 被改成假了：它会把自己写进 hosts 的那一行当成成功，S 永远是真的，主用会赖着不动")
	}

	// 冷启动（无样本）按 DefaultPriority 排，它必须排在缓存、社区源、官方段之后。
	cold, err := Rank(RankInput{Candidates: specs(t, "cache", "github520", "segment", "system")})
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	if cold.Ranked {
		t.Errorf("一份样本都没有却说是按历史选的，界面那句就成了谎话")
	}
	if cold.Primary.ID != "cache" {
		t.Errorf("冷启动主用该是最便宜的 cache，实际 %q", cold.Primary.ID)
	}
	for _, id := range []string{"cache", "github520", "segment"} {
		other := specs(t, id)[0]
		if s.DefaultPriority <= other.DefaultPriority {
			t.Errorf("system 的默认顺序（%d）该排在 %s（%d）之后，否则它一开机就靠回声抢主用",
				s.DefaultPriority, id, other.DefaultPriority)
		}
	}
}
