package detect

import (
	"context"
	"net"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// 指标二判的是三件事：同一支地址真测了几次、几次里取最低那一个、测不出来时如实说测不出来。
// 对手方是本机监听器——握手是真的，但读数不靠公网运气（判据不能"今天网络好就绿"）。

// listenLocal 在本机开一个只接不应的监听器，交回地址、端口和"握了几次手"的计数器。
func listenLocal(t *testing.T) (string, int, *atomic.Int64) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("本机监听起不来：%v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	handshakes := &atomic.Int64{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // Cleanup 关掉监听器，这个循环就该结束
			}
			handshakes.Add(1)
			_ = conn.Close()
		}
	}()

	ip, portText, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("监听地址拆不开：%v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("端口不是数：%v", err)
	}
	return ip, port, handshakes
}

// closedPort 交回一个此刻没人听的端口（占完就关）。
// 万一这一瞬间别的程序把端口占走了，这条用例会报"测出来了"而不是悄悄放过。
func closedPort(t *testing.T) (string, int) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占个端口都占不上：%v", err)
	}
	ip, portText, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("端口地址拆不开：%v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("端口不是数：%v", err)
	}
	_ = ln.Close()
	return ip, port
}

// waitHandshakes 等本机监听器把连接数记到 want（记不满就交回当下这个数，让用例照着它报错）。
func waitHandshakes(t *testing.T, counter *atomic.Int64, want int64) int64 {
	t.Helper()
	for i := 0; i < 100; i++ {
		if got := counter.Load(); got >= want {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	return counter.Load()
}

func TestLowRTTPicksMinimumWithoutReorderingInput(t *testing.T) {
	samples := []time.Duration{7 * time.Millisecond, 2 * time.Millisecond, 5 * time.Millisecond}
	before := append([]time.Duration(nil), samples...)

	got, ok := lowRTT(samples)
	if !ok {
		t.Fatal("有三个读数却说「没测出来」")
	}
	if got != 2*time.Millisecond {
		t.Errorf("取到 %v，要的是最低那一个 2ms：取错＝偶发一次排队就能把一支地址排到最后，界面那行「最快 IP」天天换人", got)
	}
	if !reflect.DeepEqual(samples, before) {
		t.Errorf("传进来的切片被改了：%v → %v", before, samples)
	}
}

func TestLowRTTEmptySaysNotMeasured(t *testing.T) {
	got, ok := lowRTT(nil)
	if ok || got != 0 {
		t.Errorf("空读数交回了 (%v, %v)：「没测出来」必须是 (0,false)，只交回 0 会被调用方当成「零毫秒最快」", got, ok)
	}
}

func TestMeasureRTTDialsOncePerRepeat(t *testing.T) {
	ip, port, handshakes := listenLocal(t)
	lim := Limits{ProbeTimeout: 2 * time.Second, RTTRepeats: 3, Concurrency: 1, BodyBytes: 4096, Port: port}

	got, ok := measureRTT(context.Background(), ip, lim)
	if !ok {
		t.Fatal("本机监听器就在听着，却报「测不出来」")
	}
	if got <= 0 {
		t.Errorf("握手延迟读成 %v：0 与「极快」分不开，界面会把它排到第一", got)
	}
	// 拨号那侧在 connect 就返回了，服务端记数要慢一拍，所以给它一点时间追上来再判。
	h := waitHandshakes(t, handshakes, 3)
	if h != 3 {
		t.Errorf("要求重复 3 次，真握了 %d 次：只测一次就会被一次偶发排队定死名次", h)
	}
}

func TestMeasureRTTWithoutListenerSaysNotMeasured(t *testing.T) {
	ip, port := closedPort(t)
	lim := Limits{ProbeTimeout: time.Second, RTTRepeats: 2, Concurrency: 1, BodyBytes: 4096, Port: port}

	got, ok := measureRTT(context.Background(), ip, lim)
	if ok || got != 0 {
		t.Errorf("没人听的端口交回了 (%v, %v)：这类地址必须靠 RTTOK 排在测出来的后面，"+
			"当成 0 毫秒就会被选成主用写进 hosts", got, ok)
	}
}

func TestMeasureRTTStopsWhenCalledOff(t *testing.T) {
	ip, port, handshakes := listenLocal(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, ok := measureRTT(ctx, ip, Limits{ProbeTimeout: time.Second, RTTRepeats: 5, Port: port})
	if h := handshakes.Load(); h != 0 {
		t.Errorf("已经叫停还握了 %d 次手：整轮取消之后继续发连接，队列会拖到超时才回来", h)
	}
	if ok {
		t.Error("叫停这一轮不该交回一个「测出来的延迟」")
	}
}

func TestMeasureRTTZeroRepeatSaysNotMeasured(t *testing.T) {
	ip, port, handshakes := listenLocal(t)

	_, ok := measureRTT(context.Background(), ip, Limits{ProbeTimeout: time.Second, RTTRepeats: 0, Port: port})
	if ok {
		t.Error("重复次数给成 0 却报测出来了")
	}
	if h := handshakes.Load(); h != 0 {
		t.Errorf("重复 0 次还握了 %d 次手", h)
	}
}
