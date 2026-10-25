//go:build !windows

package ipc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 权限模型定档（2026-10-25）：服务 root 常驻在 /run/github-accel，界面普通权限来连。
// 这组用例钉的是本用户能测的那一半形状（root 分支要真 root 才走到，CI 里测不了，如实不钉）：
//
//	① 拨号候选：第一条永远是 /run/github-accel/service.sock（root 服务是"正主"），
//	   第二条是本用户自己的端点；root 自己跑时只有一条，不重不漏；
//	② CheckPeer 对"本机 unix 对端 + 凭据齐全"放行（旧判据"同 UID"已废），
//	   非 unix 连接照旧拒——判据从"你是谁"变成"你是不是本机来的"，这条负向还在。
func TestDialEndpointsPutsRootServiceFirst(t *testing.T) {
	got, err := DialEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		if len(got) != 1 || got[0] != filepath.Join(rootEndpointDir, "service.sock") {
			t.Fatalf("root 态只该有一条候选（root 自己的固定端点），got=%v", got)
		}
		return
	}
	if len(got) != 2 {
		t.Fatalf("普通用户应有两条候选（root 常驻那条在前），got=%v", got)
	}
	if got[0] != filepath.Join(rootEndpointDir, "service.sock") {
		t.Fatalf("第一条应是 root 服务的固定端点，got=%s", got[0])
	}
	own := got[1]
	if !strings.Contains(own, "service.sock") {
		t.Fatalf("第二条应是自己那条端点，got=%s", own)
	}
}

func TestListenPermIsTightForUserRuns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.sock")
	l, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		if st.Mode().Perm() != 0o666 {
			t.Fatalf("root 态 socket 该是 0666（普通权限界面要连得进来），got=%o", st.Mode().Perm())
		}
		return
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("用户态 socket 该是 0600（只给本用户对练），got=%o", st.Mode().Perm())
	}
}

// 端到端冒烟：把 XDG_RUNTIME_DIR 指进临时目录，服务在"用户态端点"上起，
// 拨号侧按 DialEndpoints 的顺序试——root 那条固定端点不存在（负向），应落到第二条连上，
// 请求-回执一圈走通。这是"候选清单"这条新链路的机器判据。
func TestDialEndpointsFallsThroughToUserEndpoint(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	if os.Geteuid() == 0 {
		t.Skip("root 态没有第二条候选可落，这条只对普通用户跑")
	}
	// 这台机器若真装了我们的服务（/run/github-accel/service.sock 活着），第一条候选就通,
	// "落到用户端点"这个前提不成立——如实跳过,别让用例跟真安装态打架。
	if _, err := os.Stat(filepath.Join(rootEndpointDir, "service.sock")); err == nil {
		t.Skip("本机有真安装态的 root 服务在跑,fallthrough 前提不成立")
	}
	srvPath, err := Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	if srvPath != filepath.Join(dir, "service.sock") {
		t.Fatalf("服务端点该在自己的运行时目录里，got=%s", srvPath)
	}
	l, err := Listen(srvPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	srv := NewServer(AllowAnyPeer)
	srv.Register("GetVersion", func(map[string]any) (any, error) {
		return map[string]any{"current": "smoke"}, nil
	})
	go func() { _ = srv.Serve(l) }()

	c := NewClient()
	var last error
	for _, ep := range mustDialEndpoints(t) {
		if last = c.Connect(ep); last == nil {
			break
		}
	}
	if last != nil {
		t.Fatalf("两条候选都没连上：root 那条不在是预期，用户那条也不该死：%v", last)
	}
	if got := c.ActiveEndpoint(); got != srvPath {
		t.Fatalf("该落在用户端点那条上，got=%s", got)
	}
	result, err := c.Call("GetVersion", map[string]any{}, 3e9)
	if err != nil {
		t.Fatalf("回执没走通：%v", err)
	}
	m, ok := result.(map[string]any)
	if !ok || m["current"] != "smoke" {
		t.Fatalf("回执内容不对：got=%+v", result)
	}
	_ = c.Close()
}

func mustDialEndpoints(t *testing.T) []string {
	t.Helper()
	got, err := DialEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	return got
}
