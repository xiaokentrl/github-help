package detect

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// 编排那一段（并发摊开、两项指标各拿一份超时帽子、探测次数记账、空地址丢掉、结果按名次排）
// 用假件钉：真件在 probe.go／rtt.go 各自那支文件里对着本机服务测过，这里不重复联网。

// fakeProbe 交回一个"按地址表给读数、其余都算没探上"的指标一。
func fakeProbe(byIP map[string]Outcome) func(context.Context, string, string, Limits) Outcome {
	return func(_ context.Context, domain, ip string, lim Limits) Outcome {
		if o, ok := byIP[ip]; ok {
			o.IP = ip
			return o
		}
		return Outcome{IP: ip, Shape: ShapeOther, Detail: "假件没这支的地址"}
	}
}

func TestCheckCombinesBothMetrics(t *testing.T) {
	probe := fakeProbe(map[string]Outcome{
		"1.1.1.1": {CanServe: true, Shape: ShapeServed, Status: 200},
		"2.2.2.2": {CanServe: true, Shape: ShapeServed, Status: 200},
	})
	measured := map[string]time.Duration{"1.1.1.1": 40 * time.Millisecond, "2.2.2.2": 9 * time.Millisecond}

	c := &Checker{probe: probe, measure: func(_ context.Context, ip string, _ Limits) (time.Duration, bool) {
		d, ok := measured[ip]
		return d, ok
	}}
	res := c.Check(context.Background(), "github.com", []string{"1.1.1.1", "2.2.2.2"}, Limits{ProbeTimeout: time.Second, Concurrency: 2})

	if res.Best != "2.2.2.2" {
		t.Errorf("Best = %q，要的是延迟 9ms 那支：两项指标没合起来排", res.Best)
	}
	if res.RTT != 9*time.Millisecond {
		t.Errorf("RTT = %v", res.RTT)
	}
	if res.Code != "" || res.Params != nil {
		t.Errorf("探上了还带着 %q 和参数：那是给「全批没探上」用的", res.Code)
	}
	if res.Outcomes[0].Status != 200 {
		t.Errorf("明细里第一名的状态码是 %d，指标一的读数该跟着它一起交给界面", res.Outcomes[0].Status)
	}
}

// 指标二只给指标一答上的那支量：省的是预算，不是名次（这三条判据各钉一笔）。
func TestCheckSkipsHandshakeForCandidatesThatDidNotAnswer(t *testing.T) {
	var mu sync.Mutex
	var measured []string
	c := &Checker{
		probe: fakeProbe(map[string]Outcome{
			"1.1.1.1": {CanServe: true, Shape: ShapeServed, Status: 200},
		}),
		measure: func(_ context.Context, ip string, _ Limits) (time.Duration, bool) {
			mu.Lock()
			measured = append(measured, ip)
			mu.Unlock()
			return 5 * time.Millisecond, true
		},
	}
	res := c.Check(context.Background(), "github.com", []string{"1.1.1.1", "2.2.2.2"},
		Limits{ProbeTimeout: time.Second, RTTRepeats: 3, Concurrency: 2})

	if len(measured) != 1 || measured[0] != "1.1.1.1" {
		t.Errorf("握了 %v 的手：指标一没答上的那支不该去握手，它永远当不了 Best，握一次白吃一次预算", measured)
	}
	if res.ProbeCount != 5 {
		t.Errorf("ProbeCount = %d，要的是 5（两支指标一各一次，加答上的那支重复 3 次握手）：service 记这本账只读这个数，"+
			"它漂了整轮上限就按错的读数截断", res.ProbeCount)
	}
	if res.Best != "1.1.1.1" || res.Outcomes[1].RTTOK {
		t.Errorf("Best = %q，没握手的那支不该带测出来的延迟进名次", res.Best)
	}
}

func TestCheckKeepsConcurrencyUnderCap(t *testing.T) {
	var active, peak int64
	c := &Checker{
		probe: func(_ context.Context, _, ip string, _ Limits) Outcome {
			now := atomic.AddInt64(&active, 1)
			for {
				old := atomic.LoadInt64(&peak)
				if now <= old || atomic.CompareAndSwapInt64(&peak, old, now) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt64(&active, -1)
			return Outcome{IP: ip, CanServe: true, Shape: ShapeServed}
		},
		measure: func(context.Context, string, Limits) (time.Duration, bool) { return time.Millisecond, true },
	}

	ips := []string{"1.1.1.1", "1.1.1.2", "1.1.1.3", "1.1.1.4", "1.1.1.5", "1.1.1.6"}
	c.Check(context.Background(), "github.com", ips, Limits{ProbeTimeout: time.Second, Concurrency: 2})

	if p := atomic.LoadInt64(&peak); p > 2 {
		t.Errorf("并发上限给的是 2，同时探了 %d 支：预算里那句「一轮最多同时发几支」是 service 记账的依据，"+
			"这里越过去整轮墙钟与时延就都不按 budget.go 走了", p)
	}
	if p := atomic.LoadInt64(&peak); p == 0 {
		t.Error("一支都没探")
	}
}

func TestCheckZeroConcurrencyStillProbes(t *testing.T) {
	// 并发给 0 是最容易被配错的一个值：按信号量写就会一支都发不出去，整轮挂在这儿。
	var probed int64
	c := &Checker{
		probe: func(_ context.Context, _, ip string, _ Limits) Outcome {
			atomic.AddInt64(&probed, 1)
			return Outcome{IP: ip, CanServe: true, Shape: ShapeServed}
		},
		measure: func(context.Context, string, Limits) (time.Duration, bool) { return time.Millisecond, true },
	}
	done := make(chan Result, 1)
	go func() {
		done <- c.Check(context.Background(), "github.com", []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, Limits{ProbeTimeout: time.Second})
	}()

	select {
	case res := <-done:
		if atomic.LoadInt64(&probed) != 3 {
			t.Errorf("探了 %d 支，三支都该探到", atomic.LoadInt64(&probed))
		}
		if res.Best == "" {
			t.Error("并发给 0 就一个都没探上")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Concurrency 给 0 时整轮挂住了：信号量一支都发不出去")
	}
}

func TestCheckDropsEmptyAndDuplicateAddresses(t *testing.T) {
	var seen []string
	c := &Checker{
		probe: func(_ context.Context, _, ip string, _ Limits) Outcome {
			seen = append(seen, ip)
			return Outcome{IP: ip, CanServe: true, Shape: ShapeServed}
		},
		measure: func(context.Context, string, Limits) (time.Duration, bool) { return time.Millisecond, true },
	}
	res := c.Check(context.Background(), "github.com",
		[]string{"", "1.1.1.1", "1.1.1.1", "", "2.2.2.2"}, Limits{ProbeTimeout: time.Second, Concurrency: 1})

	if len(seen) != 2 {
		t.Errorf("真探了 %v：空地址（上游算错了）与重复地址各算一次就够了", seen)
	}
	for _, ip := range seen {
		if ip == "" {
			t.Error("拿空地址去拨号：症状是一条都探不上，看着像「这批 IP 都不行」")
		}
	}
	if res.Outcomes[0].IP == "" {
		t.Error("结果里还有空地址那一条")
	}
}

func TestMeasureAllReturnsInInputOrder(t *testing.T) {
	c := &Checker{
		probe: func(_ context.Context, _, ip string, _ Limits) Outcome {
			if ip == "1.1.1.1" {
				time.Sleep(10 * time.Millisecond) // 慢的那支先进场，回来的位置也该是它
			}
			return Outcome{IP: ip, CanServe: true, Shape: ShapeServed}
		},
		measure: func(context.Context, string, Limits) (time.Duration, bool) { return time.Millisecond, true },
	}
	ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	out, _ := c.measureAll(context.Background(), "github.com", ips, Limits{ProbeTimeout: time.Second, Concurrency: 3})

	for i, o := range out {
		if o.IP != ips[i] {
			t.Errorf("第 %d 位回来的是 %q，读数的位置要跟进去的顺序一致，名次只能由 RankIPs 定", i, o.IP)
		}
	}
}

func TestEachMetricGetsItsOwnTimeoutHat(t *testing.T) {
	const hat = 200 * time.Millisecond
	c := &Checker{
		probe: func(context.Context, string, string, Limits) Outcome {
			time.Sleep(150 * time.Millisecond) // 指标一把帽子用得差不多满
			return Outcome{IP: "1.1.1.1", CanServe: true, Shape: ShapeServed}
		},
		measure: func(ctx context.Context, _ string, _ Limits) (time.Duration, bool) {
			dl, ok := ctx.Deadline()
			if !ok {
				t.Error("指标二没拿到超时帽子：它会一路挂到整轮结束")
				return 0, false
			}
			if left := time.Until(dl); left < hat/2 {
				t.Errorf("指标二只剩下 %v：帽子被前一次探测吃光了，握手延迟会被记成超时，"+
					"一支好地址就因为这个掉到备选", left)
			}
			return 3 * time.Millisecond, true
		},
	}
	res := c.Check(context.Background(), "github.com", []string{"1.1.1.1"},
		Limits{ProbeTimeout: hat, RTTRepeats: 1, Concurrency: 1, BodyBytes: 4096, Port: 443})

	if res.Best != "1.1.1.1" {
		t.Errorf("Best = %q，Code = %q", res.Best, res.Code)
	}
}

func TestCheckEmptyBatchProbesNothing(t *testing.T) {
	var probed int64
	c := &Checker{
		probe: func(_ context.Context, _, ip string, _ Limits) Outcome {
			atomic.AddInt64(&probed, 1)
			return Outcome{IP: ip, CanServe: true, Shape: ShapeServed}
		},
		measure: func(context.Context, string, Limits) (time.Duration, bool) { return 0, false },
	}
	res := c.Check(context.Background(), "github.com", nil, Limits{ProbeTimeout: time.Second})

	if atomic.LoadInt64(&probed) != 0 {
		t.Errorf("一批地址都没给还探了 %d 次", atomic.LoadInt64(&probed))
	}
	if res.ProbeCount != 0 {
		t.Errorf("ProbeCount = %d：一支都没发出去，这本账却是非零的——service 会照着它把预算吃掉", res.ProbeCount)
	}
	if res.Code != errs.CodeDetectEmpty {
		t.Errorf("码是 %q，要的是 GAC-DETECT-001：一支都没探过就说「全都不能服务」是编出来的话", res.Code)
	}
	if res.Blocked() {
		t.Error("空批被标成阻断：这条域名从此不再重试，界面写着「被封锁」而其实只是这一轮没取到地址")
	}
}
