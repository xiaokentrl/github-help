package ipc

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/contract"
)

func startServer(t *testing.T) (net.Listener, *Server) {
	t.Helper()
	path := t.TempDir() + "/svc.sock"
	l, err := Listen(path)
	if err != nil {
		t.Fatalf("建端点失败: %v", err)
	}
	srv := NewServer(CheckPeer)
	srv.Register(contract.MethodGetStatus, func(map[string]any) (any, error) {
		return contract.StatusReply{
			Service: contract.State{Name: "test", Phase: contract.ServiceRunning, PID: 4242},
		}, nil
	})
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = l.Close() })
	return l, srv
}

// 干什么：查一次状态，界面拿它决定显示"服务在跑"还是"服务不可用"。
func TestStatusWhileRunning(t *testing.T) {
	l, _ := startServer(t)
	c := NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上自己刚起的端点: %v", err)
	}
	got, err := c.Status()
	if err != nil {
		t.Fatalf("取状态失败: %v", err)
	}
	if got.Service.Phase != contract.ServiceRunning {
		t.Fatalf("服务在跑时不该报别的，got=%+v", got.Service)
	}
}

// 怎么会错：服务被杀掉以后，界面若还显示"服务在跑"就是假进度。
// 判据：端点关掉后，Status 必须当场返回错、并把 phase 写成 unavailable。
func TestStatusAfterServiceGone(t *testing.T) {
	l, _ := startServer(t)
	path := l.Addr().String()
	c := NewClient()
	if err := c.Connect(path); err != nil {
		t.Fatalf("第一次连不上: %v", err)
	}
	if _, err := c.Status(); err != nil {
		t.Fatalf("服务还好着，不该出错: %v", err)
	}
	_ = l.Close()
	c.discard()
	if _, err := c.Status(); err == nil {
		t.Fatal("服务已经没了，Status 还当它活着")
	}
	got, _ := c.Status()
	if got.Service.Phase != contract.ServiceUnavailable {
		t.Fatalf("连不上时要如实写 unavailable，got=%q", got.Service.Phase)
	}
}

// 一条线上事件和回执混着来：先推一条快照，再回一条回执，调用方必须拿到回执那条。
func TestEventAndResponseShareOneLine(t *testing.T) {
	l, srv := startServer(t)
	c := NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上: %v", err)
	}
	// 先跑一次调用，确保服务侧这个连接已经登记好，下面的事件才一定推得到这个客户端。
	if _, err := c.Status(); err != nil {
		t.Fatalf("第一次取状态失败: %v", err)
	}
	srv.Broadcast(contract.EventQueueProgress, contract.Snapshot{State: contract.State{Phase: contract.ServiceRunning, PID: 7}}, nil)
	// 上面这条事件还没被读走，紧接着的调用必须穿过它拿到自己的回执。
	got, err := c.Status()
	if err != nil {
		t.Fatalf("被事件挡了一下就没回执了: %v", err)
	}
	if got.Service.PID != 4242 {
		t.Fatalf("拿错了对象，got=%+v", got.Service)
	}
}

// 不认的方法要当场报错，不能沉默返回一个空结果让界面以为"服务说没问题"。
func TestUnknownMethodRejects(t *testing.T) {
	l, _ := startServer(t)
	c := NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上: %v", err)
	}
	if _, err := c.Call("NoSuchMethod", nil, 2*time.Second); err == nil {
		t.Fatal("不认的方法居然返回了成功")
	}
}

// 2026-10-23 联调轮补的三条。这一轮之前的形状是"每次调用现起一个读的人、读到事件就丢掉"，
// 症状：服务那 5 条事件全推给了空气，进度条永远不动，而且一个字也不报。
// 所以下面这三条里，第一条是那一笔欠账的封口——它挂了，就等于事件出口又没了。

// 事件必须真的交到人手里，而不是"读了但没人接"。
func TestEventsReachTheSubscriber(t *testing.T) {
	l, srv := startServer(t)
	c := NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上: %v", err)
	}
	// 先问一句：确保服务端已经把这个连接登记进 sessions，否则 Broadcast 找不到人推。
	if _, err := c.Status(); err != nil {
		t.Fatalf("取状态失败: %v", err)
	}
	srv.Broadcast(contract.EventDetectProgress,
		contract.Snapshot{State: contract.State{Phase: contract.ServiceRunning, PID: 9}},
		map[string]any{"itemID": "github-raw"})

	select {
	case ev := <-c.Events():
		if ev.Name != contract.EventDetectProgress {
			t.Fatalf("事件名漂了：%q", ev.Name)
		}
		if ev.Snapshot.State.PID != 9 {
			t.Fatalf("快照没跟着事件走，got=%+v", ev.Snapshot.State)
		}
		if ev.Payload["itemID"] != "github-raw" {
			t.Fatalf("payload 丢了，got=%v", ev.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("事件没交到人手里——这就是老那份 client.go 的形状（读到事件 continue 丢掉）")
	}
	if got := c.Dropped(); got != 0 {
		t.Fatalf("一条也没该丢，dropped=%d", got)
	}
}

// 回执一路混着事件也得找到自己的调用；而 Snapshot() 那一问要真的把 items／tasks 带回来。
func TestSnapshotCarriesItemsAndTasks(t *testing.T) {
	path := t.TempDir() + "/svc.sock"
	l, err := Listen(path)
	if err != nil {
		t.Fatalf("建端点失败: %v", err)
	}
	srv := NewServer(AllowAnyPeer)
	srv.Register(contract.MethodGetSnapshot, func(map[string]any) (any, error) {
		return contract.Snapshot{
			State: contract.State{Phase: contract.ServiceRunning, DnsService: "systemd-resolved", HostsPath: "/tmp/h"},
			Items: []contract.Item{{ID: "github-raw", Name: "raw.githubusercontent.com", Status: contract.ItemApplied, AppliedIp: "1.2.3.4"}},
			Tasks: []contract.Task{{ID: "batch-1", Kind: contract.KindApply, Status: contract.TaskDone, Processed: 1, Total: 1}},
		}, nil
	})
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = l.Close() })

	c := NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上: %v", err)
	}
	snap, err := c.Snapshot()
	if err != nil {
		t.Fatalf("取快照失败: %v", err)
	}
	if len(snap.Items) != 1 || snap.Items[0].AppliedIp != "1.2.3.4" {
		t.Fatalf("items 没带回来，got=%+v", snap.Items)
	}
	if len(snap.Tasks) != 1 || snap.Tasks[0].Processed != 1 {
		t.Fatalf("tasks 没带回来，got=%+v", snap.Tasks)
	}
	// 环境那三个数也必须在同一份快照里：C5 要的是"这一行由服务回，界面不猜"。
	if snap.State.DnsService != "systemd-resolved" || snap.State.HostsPath != "/tmp/h" {
		t.Fatalf("环境那几个数丢了，got=%+v", snap.State)
	}
}

// 两侧契约版本对不上时必须当场报错，而不是"返回一个看着成功的空形状"。
// 这一条为什么值得单开：旧服务配新界面不会崩，只会让新字段永远空着——那是最难查的一种错。
func TestProtocolMismatchFailsLoud(t *testing.T) {
	path := t.TempDir() + "/old.sock"
	l, err := Listen(path)
	if err != nil {
		t.Fatalf("建端点失败: %v", err)
	}
	// 这里不用 ipc.Server：要的就是一条"回旧版本号"的线，模拟装了旧包的那台机器。
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		dec := newDecoder(conn)
		req, err := dec.next()
		if err != nil {
			return
		}
		enc := newEncoder(conn)
		_ = enc.write(contract.Response{ID: req.ID, OK: true, Protocol: contract.ProtocolVersion - 1,
			Result: contract.StatusReply{}})
	}()
	t.Cleanup(func() { _ = l.Close() })

	c := NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上: %v", err)
	}
	_, err = c.Status()
	if err == nil {
		t.Fatal("版本差一版却当成功返回了——界面接下来就会拿空字段画出一副没事的样子")
	}
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("报错该认出是版本对不上，got=%v", err)
	}
}

// 超时把连接判废之后，重连要还能问得出话；而等待中的那一次调用必须当场拿到一句实话，不许永远挂着。
func TestTimeoutDiscardsAndReconnectWorks(t *testing.T) {
	path := t.TempDir() + "/slow.sock"
	l, err := Listen(path)
	if err != nil {
		t.Fatalf("建端点失败: %v", err)
	}
	// 只卡第一问。为什么用 once 而不是"读一个通道"：用例中途失败时 cleanup 也要放行那一条，
	// 同一次放行不能被 close 两遍（那会 panic 在收尾里，把真正的失败盖掉）。
	gate := make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	var holdFirst atomic.Bool
	holdFirst.Store(true)
	srv := NewServer(AllowAnyPeer)
	srv.Register(contract.MethodGetStatus, func(map[string]any) (any, error) {
		if holdFirst.CompareAndSwap(true, false) {
			<-gate
		}
		return contract.StatusReply{Service: contract.State{Phase: contract.ServiceRunning}}, nil
	})
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(release)
	t.Cleanup(func() { _ = l.Close() })

	c := NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上: %v", err)
	}
	start := time.Now()
	if _, err := c.Status(); err == nil {
		t.Fatal("服务没回话，Status 却报了成功")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("等超时等了 %s：连接判废这一步没生效", elapsed)
	}
	// 放行那一问（它回给的是已经作废的旧连接，这里不接它），再另开一条连接问一遍。
	release()
	c2 := NewClient()
	if err := c2.Connect(l.Addr().String()); err != nil {
		t.Fatalf("重连不上: %v", err)
	}
	if _, err := c2.Status(); err != nil {
		t.Fatalf("重连后问不出话: %v", err)
	}
}

// 老那份 client.go 里"每次调用各起一个读的人"还藏着一个更坏的形状：
// 上一次的读的人还没退出，这一次的回执可能被它拿走。
// 这一条按顺序并发问四句，四句都必须拿到自己那一条回执——回执串门在这四条里当场就红。
func TestConcurrentCallsGetTheirOwnReply(t *testing.T) {
	l, srv := startServer(t)
	srv.Register(contract.MethodGetVersion, func(map[string]any) (any, error) {
		return contract.VersionReply{Current: "v1.0.0"}, nil
	})
	c := NewClient()
	if err := c.Connect(l.Addr().String()); err != nil {
		t.Fatalf("连不上: %v", err)
	}
	var wg sync.WaitGroup
	errsOut := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 两种问话混着发：回执形状不同，拿错了当场看得出来。
			if _, err := c.Call(contract.MethodGetVersion, nil, 3*time.Second); err != nil {
				errsOut <- err
				return
			}
			if _, err := c.Status(); err != nil {
				errsOut <- err
			}
		}()
	}
	wg.Wait()
	close(errsOut)
	for err := range errsOut {
		t.Fatalf("并发调用出了错（回执串门就长这样）：%v", err)
	}
}
