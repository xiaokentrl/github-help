package resolve

import (
	"context"
	"time"
)

// constant 把一条域名钉死到固定地址，服务的是"去广告与屏蔽"那一块。
//
// 干什么用：屏蔽要的效果是"这个名字谁都连不上"，跟快慢、跟哪个地址能用都没关系——
//
//	它是这一档唯一能落地的形状，也是 block 那 3 条的来源（Kinds 只有 block，别的块碰不到它）。
//
// 什么情况下会错、出错时人看得见什么：
// ① 让它参与 S/V 比较 → 某条联网方式在屏蔽域名上"成功"给出一个公网 IP，屏蔽当场失效，
//
//	用户以为去广告功能坏了。所以 Rankable＝假：它不进两把尺子，第 6.4 节第 2 条说的就是这个。
//
// ② NeedsProbe＝假：钉死的这个地址不需要探，探它反而会把 127.0.0.1 记成"能服务"，
//
//	下一轮 S 就成了一条它本不该有的历史。
//
// 怎么回来：这个地址只写在这里这一处。要改成 0.0.0.0 或黑洞地址，改这一个常量，
//
//	不动菜单数据、不动 rank.go——那里也不许出现方式 id 去点名它（G11）。
var constantSpec = Spec{
	ID:              "constant",
	Title:           "固定屏蔽",
	Kinds:           []string{"block"},
	NeedsProbe:      false,
	DefaultPriority: 1,
	OfflineOK:       true,
	Rankable:        false,
}

// blockedAddress 是屏蔽条目的落点。它是这一档自己的"值"，不是取数上限，所以留在本文件而不是 budget.go。
const blockedAddress = "127.0.0.1"

type constant struct{}

func newConstant() Resolver { return &constant{} }

func init() { must(Register(constantSpec, newConstant)) }

func (m *constant) Spec() Spec { return cloneSpec(constantSpec) }

// Resolve 没有失败分支：不联网、不问人，给的就是那一个地址。
// 唯一要判的是"这一轮被叫停了"——那一句也得如实回，不能让收尾把屏蔽当成一次成功采样。
func (m *constant) Resolve(ctx context.Context, q Query) (Set, error) {
	if err := ctxError(constantSpec.ID, ctx); err != nil {
		return Set{}, err
	}
	started := time.Now()
	return Set{ModeID: constantSpec.ID, IPs: []string{blockedAddress}, Took: time.Since(started)}, nil
}
