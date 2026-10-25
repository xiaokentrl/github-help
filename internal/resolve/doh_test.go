package resolve

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hlep/github-accel/internal/errs"
)

// DoH 这一档今天在本机必然失败（文件顶上那三个读数）。用例因此全走假件：
// 判的是"连上了怎么挑答案、没连上回哪个码"，不是"端点通不通"。

const dohReplySample = `{"Status":0,"name":"github.com.",
  "Answer":[{"name":"github.com.","type":1,"TTL":60,"data":"20.205.243.166"},
            {"name":"github.com.","type":5,"TTL":60,"data":"codeload.github.com."},
            {"name":"github.com.","type":1,"TTL":60,"data":"20.205.243.168"},
            {"name":"github.com.","type":1,"TTL":60,"data":"20.205.243.166"},
            {"name":"api.github.com.","type":1,"TTL":60,"data":"198.18.0.1"}]}`

func TestDoHAnswersKeepsOnlyRealAddressesForThatName(t *testing.T) {
	got := dohAnswers(dohReplySample, "github.com")
	if strings.Join(got, ",") != "20.205.243.166,20.205.243.168" {
		t.Errorf("CNAME（data 是个名字）与别的域名都该被挡掉，重复的只留第一次，got %v", got)
	}

	if got := dohAnswers(dohReplySample, "GITHUB.COM"); len(got) != 2 {
		t.Errorf("域名大小写不一样也该认（hosts 本来就不分大小写），got %v", got)
	}
	if got := dohAnswers(`{"Answer":[{"name":"github.com.","data":"20.205.243.166"}]}`, "api.github.com"); len(got) != 0 {
		t.Errorf("回答里没这条域名就不该给地址：%v", got)
	}
	if got := dohAnswers("这不是 JSON", "github.com"); got != nil {
		t.Errorf("回答形状不对＝这一支这次没答上，该交回空，got %v", got)
	}
}

func TestDoHResolveUsesFirstWorkingEndpoint(t *testing.T) {
	m := &doh{get: fakeGet(map[string]string{
		dohQuery(dohEndpoints[0], "github.com"): dohReplySample,
		dohQuery(dohEndpoints[1], "github.com"): `{"Answer":[{"name":"github.com.","data":"1.1.1.1"}]}`,
	}, nil)}

	got, err := m.Resolve(context.Background(), Query{Domain: "github.com"})
	if err != nil {
		t.Fatalf("Resolve 抛了：%v", err)
	}
	if got.ModeID != "doh" || strings.Join(got.IPs, ",") != "20.205.243.166,20.205.243.168" {
		t.Errorf("两支都通时该用排在前面那一支，got %+v", got)
	}
}

func TestDoHResolveFallsToNextEndpoint(t *testing.T) {
	m := &doh{get: func(_ context.Context, url string) (string, error) {
		if url == dohQuery(dohEndpoints[0], "github.com") {
			return "", errors.New("TLS 被对端重置")
		}
		return `{"Answer":[{"name":"github.com.","data":"1.1.1.1"}]}`, nil
	}}

	got, err := m.Resolve(context.Background(), Query{Domain: "github.com"})
	if err != nil {
		t.Fatalf("第一支连不通该换第二支，实际抛了：%v", err)
	}
	if strings.Join(got.IPs, ",") != "1.1.1.1" {
		t.Errorf("got %+v", got)
	}
}

func TestDoHErrorsDistinguishEndpointsDownFromNoAnswer(t *testing.T) {
	down := &doh{get: fakeGet(nil, errors.New("connection refused"))}
	_, err := down.Resolve(context.Background(), Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != "GAC-MODE-doh-001" {
		t.Errorf("一支都连不上该回 001，实际 %s（%v）", code, err)
	}
	p := errs.ParamsOf(err)
	if p["domain"] != "github.com" {
		t.Errorf("错误参数要带上是哪条域名，实际 %v", p)
	}

	// 连上了却答不出这条域名＝另一回事，界面那句不能写成"端点都没连上"。
	// 前面那一支也得答得上，否则"最后一次失败"会盖住"这一支只是没答案"。
	noAnswer := &doh{get: fakeGet(map[string]string{
		dohQuery(dohEndpoints[0], "github.com"): `{"Status":3,"Authority":[]}`,
		dohQuery(dohEndpoints[1], "github.com"): `{"Status":3,"Authority":[]}`,
	}, nil)}
	_, err = noAnswer.Resolve(context.Background(), Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != "GAC-MODE-doh-002" {
		t.Errorf("答不出这条域名该回 002，实际 %s（%v）", code, err)
	}
}

func TestDoHPausedIsNotAFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()

	m := &doh{get: fakeGet(map[string]string{dohQuery(dohEndpoints[0], "github.com"): dohReplySample}, nil)}
	_, err := m.Resolve(ctx, Query{Domain: "github.com"})
	if code := errs.CodeOf(err); code != errs.CodeTaskPaused {
		t.Errorf("该回 GAC-TASK-003，实际 %s（%v）", code, err)
	}
}

// 端点清单不许在外面再抄一份：dohQuery 拼出来的地址必须是清单里那一支加上查询串。
func TestDoHQueryAppendsToEndpointList(t *testing.T) {
	for _, endpoint := range dohEndpoints {
		got := dohQuery(endpoint, "github.com")
		if !strings.HasPrefix(got, endpoint+"?") {
			t.Errorf("%s 拼出来的查询串不像同一支端点：%s", endpoint, got)
		}
	}
	if got := dohQuery("https://dns.google/resolve", "a b.example"); strings.Contains(got, " ") {
		t.Errorf("域名里带空格要转义，got %s", got)
	}
}
