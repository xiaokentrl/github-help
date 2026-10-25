package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestCodeOfPrefersWrappedError(t *testing.T) {
	e := New(CodeHostsWrite, map[string]any{"path": "/etc/hosts"}, "写不进去")
	if got := CodeOf(fmt.Errorf("上层包装: %w", e)); got != CodeHostsWrite {
		t.Fatalf("码要从包装里取出来，got %q", got)
	}
}

func TestCodeOfReadsLegacyPrefix(t *testing.T) {
	// ipc 那几条历史形状就是 `码: 文案`，不包 *Error；这条用例保证界面不会因为历史形状拿到 GAC-UNKNOWN-000。
	if got := CodeOf(errors.New("GAC-IPC-021: 监听失败")); got != "GAC-IPC-021" {
		t.Fatalf("got %q", got)
	}
}

func TestCodeOfRejectsLookalikes(t *testing.T) {
	cases := map[string]string{
		"打开文件失败: no such file":    CodeUnknown,            // 冒号前不是码
		"GAC: 太短的一段":              CodeUnknown,            // 缺前缀主体
		"GAC-IPC-021 监听失败无冒号":     CodeUnknown,            // 整段含空格
		"GAC-IPC-021: 后面又: 第二段":   "GAC-IPC-021",          // 取冒号前那一段就够
		"GAC-MODE-example-012: x": "GAC-MODE-example-012", // 方式那一族也要认得出来
	}
	for msg, want := range cases {
		if got := CodeOf(errors.New(msg)); got != want {
			t.Errorf("%q → got %q, want %q", msg, got, want)
		}
	}
}

func TestCodeOfNilAndUnknown(t *testing.T) {
	if got := CodeOf(nil); got != "" {
		t.Fatalf("没出错时不该有码，got %q", got)
	}
	if got := CodeOf(errors.New("纯文本一条")); got != CodeUnknown {
		t.Fatalf("got %q", got)
	}
}

func TestParamsCopyAndMissing(t *testing.T) {
	src := map[string]any{"domain": "github.com"}
	e := New(CodeHostsBadValue, src, "这条写不进去")
	src["domain"] = "被外面改了"
	if got := ParamsOf(e)["domain"]; got != "github.com" {
		t.Fatalf("参数要当场拷进去，不许共享同一张表，got %v", got)
	}
	if got := ParamsOf(errors.New("plain")); got == nil {
		t.Fatal("没参数时也要返回空表，调用方不必判空")
	}
}

func TestModeCodeShape(t *testing.T) {
	if got := ModeCode("GitHub520", 3); got != "GAC-MODE-github520-003" {
		t.Fatalf("方式 id 要转小写、序号补零，got %q", got)
	}
	if got := CodeOf(New(ModeCode("example", 12), nil, "取数失败")); got != "GAC-MODE-example-012" {
		t.Fatalf("got %q", got)
	}
}

func TestCodesListHasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Codes() {
		if seen[c] {
			t.Fatalf("码 %q 登记了两次", c)
		}
		seen[c] = true
		if !isCode(c) && c != CodeUnknown {
			t.Fatalf("码 %q 形状不对，CodeOf 认不出自己登记的码", c)
		}
	}
	if len(Codes()) < 30 {
		t.Fatalf("清单短得可疑：%d 条", len(Codes()))
	}
}
