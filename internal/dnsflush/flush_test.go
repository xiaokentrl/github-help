package dnsflush

import (
	"context"
	"os"
	"reflect"
	"testing"
)

// 只有一个方法这一条要钉住：多一个方法就得为两个平台各写一份实现，
// 而"平台差异只出现在按 build tag 分开的那两个文件里、调用点一条分叉都没有"是这一层的全部理由。
func TestFlusherHasExactlyOneMethod(t *testing.T) {
	typ := reflect.TypeOf((*Flusher)(nil)).Elem()
	if typ.NumMethod() != 1 {
		t.Fatalf("Flusher 只该有一个方法，实际 %d 个", typ.NumMethod())
	}
	m := typ.Method(0)
	if m.Name != "Flush" {
		t.Fatalf("方法名不该漂：%s", m.Name)
	}
}

func TestNewReturnsPlatformImpl(t *testing.T) {
	f := New()
	if f == nil {
		t.Fatal("New() 给了 nil——调用点会当场 panic，而这一层本该没有分支")
	}
	// 本机是 Linux：这一支该是"什么都不做而且算成功"。
	// 为什么它是正确的而不是偷懒：glibc 的 nss_files 每次解析都现读 hosts，没有需要清的缓存。
	if err := f.Flush(context.Background()); err != nil {
		t.Fatalf("Linux 上不刷不该报错：%v", err)
	}
	// 上下文已经撤掉也一样：这一支压根不发起任何外部命令，所以没有可被打断的东西。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.Flush(ctx); err != nil {
		t.Fatalf("got %v", err)
	}
}

// 下面是 ServiceName 那一条的判据（2026-10-23 联调轮：C5 要的"这一行由服务回，界面不猜"，
// 此前契约里没有它的家，界面只能继续显示一份写死的假值）。
// 三条用例都不碰真网络，也不碰真的 /etc/resolv.conf：喂的都是 t.TempDir() 里的假文件。

func writeResolv(t *testing.T, body string) string {
	t.Helper()
	p := t.TempDir() + "/resolv.conf"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestServiceNameFromResolv(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"resolved 存根", "nameserver 127.0.0.53\n# resolved\n", "systemd-resolved"},
		// 第一行 nameserver 说了算：后面那条备胎不改变"此刻谁在管解析"。
		{"第一行不是存根就不算", "nameserver 1.1.1.1\nnameserver 127.0.0.53\n", ""},
		{"注释里的地址不算", "# nameserver 127.0.0.53\nnameserver 10.0.0.1\n", ""},
		{"只有 search 没有 nameserver", "search example.com\n", ""},
		{"空文件", "", ""},
	}
	for _, c := range cases {
		got := serviceNameFrom(writeResolv(t, c.body))
		if got != c.want {
			t.Fatalf("%s：got=%q want=%q", c.name, got, c.want)
		}
	}
	// 文件压根不在（容器里、macOS 上都有这一档）：回空串而不是报错——
	// 空串在界面上是"—"（不知道），报错会让整块「关于我们」显示不出来。
	if got := serviceNameFrom(t.TempDir() + "/不在这里.conf"); got != "" {
		t.Fatalf("读不到文件该回空串，got=%q", got)
	}
}

// 这一条钉的是"取数据的那一步不许漂"：本机真读一遍，只读，不写、不刷、不发一个包。
// 为什么值得留一条依赖本机文件的用例：它红了就是"这台机器的判据变了"，
// 而不是"用例写错了"——判据改动必须让人看见（本机实测：resolv.conf 第一行 nameserver 127.0.0.53）。
func TestServiceNameOnThisMachineDoesNotPanic(t *testing.T) {
	got := ServiceName()
	if got != "" && got != "systemd-resolved" {
		t.Fatalf("这一支实现只认得那两种答案，got=%q", got)
	}
}
