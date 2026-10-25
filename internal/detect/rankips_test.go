package detect

import (
	"reflect"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// 这一支文件钉的是 IP 层那把尺子本身：判据不联网，输入是一批读数，
// 所以"同一批读数每次选出同一个地址"这句话能在用例里逐例钉死，不用等今天网络上运气如何。

func servedAt(ip string, rtt time.Duration) Outcome {
	return Outcome{IP: ip, CanServe: true, Shape: ShapeServed, Status: 200, RTT: rtt, RTTOK: true}
}

func servedNoRTT(ip string) Outcome {
	return Outcome{IP: ip, CanServe: true, Shape: ShapeServed, Status: 200}
}

func failAt(ip string, shape Shape) Outcome {
	return Outcome{IP: ip, Shape: shape}
}

func TestRankIPsPutsServableFirstEvenWhenSlower(t *testing.T) {
	res := RankIPs("github.com", []Outcome{
		failAt("1.1.1.1", ShapeTimeout),
		servedAt("2.2.2.2", 40*time.Millisecond),
	})

	if res.Outcomes[0].IP != "2.2.2.2" {
		t.Errorf("第一名是 %s，要的是能服务那支 2.2.2.2：先比延迟就会把连不上的地址排成最快，"+
			"写进 hosts 后浏览器打不开而界面写着「已生效」", res.Outcomes[0].IP)
	}
	if res.Best != "2.2.2.2" {
		t.Errorf("Best = %q", res.Best)
	}
	if res.Code != "" {
		t.Errorf("探上了一支还带着错误码 %q", res.Code)
	}
}

func TestRankIPsUnmeasuredRTTCannotWinByZero(t *testing.T) {
	// 1.1.1.1 地址字典序在前，但它延迟没测出来；不靠 RTTOK 拦住它，0 毫秒会直接抢走主用。
	res := RankIPs("github.com", []Outcome{
		servedNoRTT("1.1.1.1"),
		servedAt("2.2.2.2", 30*time.Millisecond),
	})

	if res.Best != "2.2.2.2" {
		t.Errorf("Best = %q，要的是真量到 30ms 那支：没测出来当成 0 毫秒＝把一支连不上的地址当最快", res.Best)
	}
	if res.Outcomes[1].IP != "1.1.1.1" {
		t.Errorf("第二名的位置是 %s", res.Outcomes[1].IP)
	}
}

func TestRankIPsTiesBreakByAddress(t *testing.T) {
	left := []Outcome{servedAt("9.9.9.9", 10*time.Millisecond), servedAt("3.3.3.3", 10*time.Millisecond)}
	right := []Outcome{servedAt("3.3.3.3", 10*time.Millisecond), servedAt("9.9.9.9", 10*time.Millisecond)}

	got, want := rank(resOf(t, left)), rank(resOf(t, right))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("同一批读数换了进去的顺序就换了名次：\n %v\n vs %v\n"+
			"＝「自动选最优」其实变成了「随机选一个」，正是原型 :1281 那个「IP 来源天天变」的老毛病", got, want)
	}
	if got[0] != "3.3.3.3" {
		t.Errorf("同分时的第一名是 %s，判据是地址字典序", got[0])
	}
}

func TestRankIPsDoesNotReorderCallerSlice(t *testing.T) {
	in := []Outcome{servedAt("9.9.9.9", 10*time.Millisecond), failAt("8.8.8.8", ShapeReset)}
	before := append([]Outcome(nil), in...)
	_ = RankIPs("github.com", in)

	if !reflect.DeepEqual(in, before) {
		t.Error("RankIPs 把调用方传进来的切片排序了：上游那一轮样本会被打乱，" +
			"下一轮读同一批候选拿到的顺序就不是取数那侧给的顺序")
	}
}

func TestRankIPsGroupsFailuresByShape(t *testing.T) {
	res := RankIPs("github.com", []Outcome{
		failAt("1.1.1.1", ShapeTimeout),
		failAt("2.2.2.2", ShapeRefused),
		failAt("3.3.3.3", ShapeBadCert),
	})

	var got []Shape
	for _, o := range res.Outcomes {
		got = append(got, o.Shape)
	}
	want := []Shape{ShapeBadCert, ShapeRefused, ShapeTimeout} // 形状名字典序（blocked.go 那张表里的三种）
	if !reflect.DeepEqual(got, want) {
		t.Errorf("全灭那批排成 %v，要的是按形状聚堆 %v：界面那行「探测明细」要靠这个看清是怎么全灭的", got, want)
	}
}

func TestRankIPsCodeWhenNoneServes(t *testing.T) {
	cases := []struct {
		name     string
		outcomes []Outcome
		wantCode string
	}{
		{"一批都没有", nil, errs.CodeDetectEmpty},
		{"全是到点不答", []Outcome{failAt("1.1.1.1", ShapeTimeout)}, errs.CodeDetectBlocked},
		{"到点不答加被掐", []Outcome{failAt("1.1.1.1", ShapeTimeout), failAt("2.2.2.2", ShapeReset)}, errs.CodeDetectBlocked},
		{"掺一支没人听", []Outcome{failAt("1.1.1.1", ShapeTimeout), failAt("2.2.2.2", ShapeRefused)}, errs.CodeDetectNoServe},
		{"掺一支不是正主", []Outcome{failAt("1.1.1.1", ShapeReset), failAt("2.2.2.2", ShapeBadCert)}, errs.CodeDetectNoServe},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := RankIPs("github.com", c.outcomes)
			if res.Best != "" {
				t.Errorf("一支都没探上却有 Best %q", res.Best)
			}
			if res.RTT != 0 {
				t.Errorf("没有 Best 还带着延迟 %v", res.RTT)
			}
			if res.Code != c.wantCode {
				t.Errorf("码是 %q，要的是 %q", res.Code, c.wantCode)
			}
			if res.Params == nil {
				t.Fatal("没探上却没给参数：语言文件那句「探了几支、都是什么形状」就没法拼出来")
			}
		})
	}
}

func TestRankIPsResultRTTFollowsBest(t *testing.T) {
	res := RankIPs("github.com", []Outcome{servedAt("2.2.2.2", 30*time.Millisecond), servedAt("1.1.1.1", 12*time.Millisecond)})
	if res.Best != "1.1.1.1" || res.RTT != 12*time.Millisecond {
		t.Errorf("Best/RTT 是 %q / %v，界面那行会写着「1.1.1.1 之外的人最快」——两个数必须来自同一支", res.Best, res.RTT)
	}

	unmeasured := RankIPs("github.com", []Outcome{servedNoRTT("1.1.1.1")})
	if unmeasured.Best != "1.1.1.1" || unmeasured.RTT != 0 {
		t.Errorf("唯一一支没量到延迟时 got %q / %v，要的是照旧选中它、延迟留 0 但 Code 为空", unmeasured.Best, unmeasured.RTT)
	}
	if unmeasured.Code != "" {
		t.Errorf("探上了却回码 %q", unmeasured.Code)
	}
}

func resOf(t *testing.T, outcomes []Outcome) Result {
	t.Helper()
	return RankIPs("github.com", outcomes)
}

func rank(res Result) []string {
	var out []string
	for _, o := range res.Outcomes {
		out = append(out, o.IP)
	}
	return out
}
