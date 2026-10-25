package detect

import (
	"testing"

	"github.com/hlep/github-accel/internal/errs"
)

// 阻断这件事只能靠"失败形状"判，不能靠"失败条数"判——这一支文件钉的就是这张表。
// 判错方向的代价不对称：把清单写错当成被封锁，这条域名从此不再重试；
// 把真封锁当成地址不行，只是多跑几轮。所以表窄一点、宁可少标一条阻断。

func TestIsBlockedReadsShapeTableOnly(t *testing.T) {
	cases := []struct {
		name     string
		outcomes []Outcome
		want     bool
	}{
		{"空批", nil, false},
		{"一支到点不答", []Outcome{failAt("1.1.1.1", ShapeTimeout)}, true},
		{"一支被掐", []Outcome{failAt("1.1.1.1", ShapeReset)}, true},
		{"两种拦法掺一起", []Outcome{failAt("1.1.1.1", ShapeTimeout), failAt("2.2.2.2", ShapeReset)}, true},
		{"全是没人听", []Outcome{failAt("1.1.1.1", ShapeRefused)}, false},
		{"全是没路由", []Outcome{failAt("1.1.1.1", ShapeNoRoute)}, false},
		{"全不是正主", []Outcome{failAt("1.1.1.1", ShapeBadCert)}, false},
		{"答的不是HTTP", []Outcome{failAt("1.1.1.1", ShapeProtocol)}, false},
		{"认不出形状", []Outcome{failAt("1.1.1.1", ShapeOther)}, false},
		{"一支探上了", []Outcome{servedAt("1.1.1.1", 0), failAt("2.2.2.2", ShapeTimeout)}, false},
		{"探上那支延迟没测出来", []Outcome{servedNoRTT("1.1.1.1"), failAt("2.2.2.2", ShapeReset)}, false},
		{"一支到点不答掺一支不是正主", []Outcome{failAt("1.1.1.1", ShapeTimeout), failAt("2.2.2.2", ShapeBadCert)}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isBlocked(c.outcomes); got != c.want {
				t.Errorf("isBlocked = %v，要的是 %v", got, c.want)
			}
		})
	}
}

func TestFailCodeShapesCarryCounts(t *testing.T) {
	code, params := failCode("github.com", []Outcome{
		failAt("1.1.1.1", ShapeTimeout),
		failAt("2.2.2.2", ShapeTimeout),
		failAt("3.3.3.3", ShapeReset),
	})

	if code != errs.CodeDetectBlocked {
		t.Errorf("码是 %q，要的是 %q", code, errs.CodeDetectBlocked)
	}
	if params["domain"] != "github.com" {
		t.Errorf("参数里的域名是 %v", params["domain"])
	}
	if params["candidates"] != 3 {
		t.Errorf("参数里的候选数是 %v，要的是 3", params["candidates"])
	}
	shapes, ok := params["shapes"].(map[string]int)
	if !ok {
		t.Fatalf("形状清单不是 map[string]int，语言文件那句「都是什么形状」拿不到：%v", params["shapes"])
	}
	if shapes[string(ShapeTimeout)] != 2 || shapes[string(ShapeReset)] != 1 {
		t.Errorf("形状计数 got %v，要的是 timeout 2 支、reset 1 支", shapes)
	}
}

func TestFailCodeEmptyBatchIsCandidatePoolEmpty(t *testing.T) {
	code, params := failCode("github.com", nil)
	if code != errs.CodeDetectEmpty {
		t.Errorf("空批回的是 %q，要的是 %q：那是这一轮这个方式没给回地址，"+
			"标成 003 会让界面说「这批 IP 都不行」而其实一支都没探过", code, errs.CodeDetectEmpty)
	}
	if params["candidates"] != 0 {
		t.Errorf("空批的候选数是 %v", params["candidates"])
	}
}

func TestResultBlockedReadsCode(t *testing.T) {
	if !(Result{Code: errs.CodeDetectBlocked}).Blocked() {
		t.Error("004 的结果 Blocked() 为假：队列会照旧一轮一轮重试，界面一直显示「检测中」")
	}
	for _, code := range []string{"", errs.CodeDetectNoServe, errs.CodeDetectEmpty, errs.CodeDetectTimeout} {
		if (Result{Code: code}).Blocked() {
			t.Errorf("码 %q 也被当成阻断了：这一档该继续换下一批候选", code)
		}
	}
}
