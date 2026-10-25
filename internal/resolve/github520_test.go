package resolve

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// 社区源这一档的用例全走假件：外网不可控，判据必须钉得住返回值（第 11 节那句"引擎不知道谁调我"）。

func fakeGet(replies map[string]string, err error) func(context.Context, string) (string, error) {
	return func(_ context.Context, url string) (string, error) {
		if err != nil {
			return "", err
		}
		text, ok := replies[url]
		if !ok {
			return "", errors.New("用例里没有这一支源：" + url)
		}
		return text, nil
	}
}

const github520Sample = "# GitHub520 Host Start\n" +
	"140.82.113.26                 alive.github.com\n" +
	"20.205.243.168                api.github.com\n" +
	"185.199.108.133               avatars.githubusercontent.com\n" +
	"\n" +
	"not-an-ip  github.com\n" +
	"20.205.243.166  github.com  # 这一行带行尾注释\n" +
	"1.1.1.1  other.example\n" +
	"# GitHub520 Host End\n"

func TestGithub520LookupTakesOnlyThatDomain(t *testing.T) {
	got := github520Lookup(github520Sample, "github.com")
	want := []string{"20.205.243.166"} // 那一行形状不对（IP 不是地址）的不算答案
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}

	if got := github520Lookup(github520Sample, "API.GITHUB.com"); strings.Join(got, ",") != "20.205.243.168" {
		t.Errorf("域名大小写不一样也该认（hosts 本来就不分大小写），got %v", got)
	}
	if got := github520Lookup(github520Sample, "没这一条.example"); len(got) != 0 {
		t.Errorf("源里没有的域名不该给出地址：%v", got)
	}
}

func TestGithub520LookupDeduplicatesKeepingSourceOrder(t *testing.T) {
	text := "9.9.9.9 a.example\n8.8.8.8 a.example\n9.9.9.9 a.example\n"
	got := github520Lookup(text, "a.example")
	if strings.Join(got, ",") != "9.9.9.9,8.8.8.8" {
		t.Errorf("该按源里的顺序去重，got %v", got)
	}
}

func TestGithub520ResolveUsesFirstWorkingMirror(t *testing.T) {
	m := &github520{get: fakeGet(map[string]string{
		github520Sources[0]: "1.1.1.1 github.com\n",
		github520Sources[1]: "2.2.2.2 github.com\n",
	}, nil)}

	got, err := m.Resolve(context.Background(), Query{Domain: "github.com"})
	if err != nil {
		t.Fatalf("Resolve 抛了：%v", err)
	}
	if got.ModeID != "github520" || strings.Join(got.IPs, ",") != "1.1.1.1" {
		t.Errorf("两支镜像都通时该用排在前面那一支，got %+v", got)
	}
	if got.Took < 0 {
		t.Errorf("Took 是 V 那把尺子的原料，不该是负数：%v", got.Took)
	}
}

// 排在前面的镜像今天连不通（实测 raw.githubusercontent.com 就是这一档），必须自己换下一支。
func TestGithub520ResolveFallsToNextMirror(t *testing.T) {
	m := &github520{get: func(_ context.Context, url string) (string, error) {
		if url == github520Sources[0] {
			return "", errors.New("连不通")
		}
		return "2.2.2.2 github.com\n", nil
	}}

	got, err := m.Resolve(context.Background(), Query{Domain: "github.com"})
	if err != nil {
		t.Fatalf("第一支失败该换第二支，实际抛了：%v", err)
	}
	if strings.Join(got.IPs, ",") != "2.2.2.2" {
		t.Errorf("got %+v", got)
	}
}

// 源里没这条域名 ≠ 源坏了：两个码得分开，界面那句才不是说谎的。
func TestGithub520ErrorsDistinguishSourcesDownFromNoEntry(t *testing.T) {
	down := &github520{get: fakeGet(nil, errors.New("connection refused"))}
	_, err := down.Resolve(context.Background(), Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != "GAC-MODE-github520-001" {
		t.Errorf("全都连不通该回 001，实际 %s（%v）", code, err)
	}

	empty := &github520{get: fakeGet(map[string]string{
		github520Sources[0]: "# 一份还没更新的清单\n",
		github520Sources[1]: "# 同样是空的\n",
		github520Sources[2]: "# 也是空的\n",
	}, nil)}
	_, err = empty.Resolve(context.Background(), Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != "GAC-MODE-github520-002" {
		t.Errorf("源里没有这一条该回 002，实际 %s（%v）", code, err)
	}
	if p := errs.ParamsOf(err); p["domain"] != "github.com" {
		t.Errorf("错误参数要带上是哪条域名，界面那句才指得明，实际 %v", p)
	}
}

// 被叫停 ≠ 失败：这一轮不许给这个方式记一笔 S。
func TestGithub520PausedIsNotAFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()

	m := &github520{get: fakeGet(map[string]string{github520Sources[0]: "1.1.1.1 github.com\n"}, nil)}
	_, err := m.Resolve(ctx, Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != errs.CodeTaskPaused {
		t.Fatalf("该回 GAC-TASK-003，实际 %s（%v）", code, err)
	}
}

// 超时帽子由预算给：假件里睡过 ModeTimeout，必须是被帽子掐掉而不是无限等。
func TestGithub520HonoursBudgetTimeout(t *testing.T) {
	m := &github520{get: func(ctx context.Context, _ string) (string, error) {
		select {
		case <-time.After(ModeTimeout + time.Second):
			return "1.1.1.1 github.com\n", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}

	started := time.Now()
	_, err := m.Resolve(context.Background(), Query{Domain: "github.com"})
	if err == nil {
		t.Fatal("源睡过了预算，不该当成成功")
	}
	if took := time.Since(started); took > ModeTimeout+time.Second {
		t.Errorf("没按 ModeTimeout 掐：等了 %v", took)
	}
	// 帽子到点是一次失败（要采样），被用户按停才是"没让我弄"（不采样）——两个码混了，
	// 一个每次都慢到掐表的方式就能一直赖在名单里。
	if code := errs.CodeOf(err); code != errs.CodeDetectTimeout {
		t.Errorf("超预算该回 GAC-DETECT-002，实际 %s（%v）", code, err)
	}
}
