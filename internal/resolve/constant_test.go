package resolve

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/hlep/github-accel/internal/errs"
)

// 固定屏蔽这一档没有取数、没有算法，判的全是"形状不许被人改坏"。

func TestConstantResolveAlwaysGivesTheBlockedAddress(t *testing.T) {
	got, err := newConstant().Resolve(context.Background(), Query{Domain: "ad.example"})
	if err != nil {
		t.Fatalf("这一档不联网、不问人，不该有失败分支：%v", err)
	}
	if got.ModeID != "constant" || len(got.IPs) != 1 || got.IPs[0] != blockedAddress {
		t.Errorf("屏蔽的落点只该是那一个地址，got %+v", got)
	}
	// 落点必须是 hosts 承载得动的形状：写进去一个读不懂的字符串，等于屏蔽悄悄失效。
	if net.ParseIP(blockedAddress) == nil {
		t.Errorf("blockedAddress %q 不是地址形状，写进 hosts 那一行会被当成乱码", blockedAddress)
	}
	if got.Took < 0 {
		t.Errorf("Took 不该是负数：%v", got.Took)
	}
}

func TestConstantPausedIsNotAFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()

	_, err := newConstant().Resolve(ctx, Query{Domain: "ad.example"})
	if code := errs.CodeOf(err); code != errs.CodeTaskPaused {
		t.Errorf("被叫停也要如实回 GAC-TASK-003，不能让收尾把屏蔽当成一次成功采样，实际 %s（%v）", code, err)
	}
}

// 两条命根子：不参与两把尺子（否则联网方式在屏蔽域名上"成功"一次，屏蔽就被顶掉）、不探（否则 127.0.0.1 被记成"能服务"）。
func TestConstantShapeKeepsBlockApart(t *testing.T) {
	s := specs(t, "constant")[0]
	if s.Rankable {
		t.Errorf("constant 一旦被改成参与比较，某条联网方式在屏蔽域名上给出一个公网 IP，屏蔽当场失效而界面看不出来")
	}
	if s.NeedsProbe {
		t.Errorf("constant 一旦被改成要探，127.0.0.1 会被记成\"能服务\"，下一轮 S 就多了一条它本不该有的历史")
	}
	if strings.Join(s.Kinds, ",") != "block" {
		t.Errorf("这一档只服务屏蔽那一块（实测菜单里 block 类目 3 条），got %v", s.Kinds)
	}

	// 屏蔽那块要一开机就赢过任何联网方式：默认顺序必须不比任何方式靠后，冷启动才不会被抢。
	for _, other := range All() {
		if other.ID == s.ID {
			continue
		}
		if other.DefaultPriority < s.DefaultPriority {
			t.Errorf("%s 的默认顺序（%d）比 constant（%d）还靠前，冷启动会把屏蔽顶掉",
				other.ID, other.DefaultPriority, s.DefaultPriority)
		}
	}
}

// 只有屏蔽方式参选时不许设备用：备用换人等于让屏蔽地址被联网方式顶掉。
func TestConstantOnlyChoiceHasNoBackup(t *testing.T) {
	c, err := Rank(RankInput{Candidates: specs(t, "constant")})
	if err != nil {
		t.Fatalf("Rank 抛了：%v", err)
	}
	if c.Primary.ID != "constant" || c.Backup.ID != "" || c.Ranked {
		t.Errorf("got %+v", c)
	}
}
