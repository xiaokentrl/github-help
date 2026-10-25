// Package resolve 回答的是"这条域名的 IP 该用哪种算法去弄"，不管"弄到的 IP 里哪个最好"（那是 detect）。
//
// 干什么用：一层放解析方式——一个方式一个文件，文件里自己 init() 注册自己（第 5.4 节 R3/R5/R6）。
// 加一种方式＝只新建一个 xxx.go，注册表、界面、别的方式文件都不用动。
// 另一层放四件公共件：registry（注册表）、rank（选主用/备用的纯函数）、health（两把尺子的算法）、budget（一轮取数的全部上限）。
// 六个方式文件都要用的那三件小东西（叫停判据、取数超时帽子、取一份文本）就落在本文件末尾——不为它们开第五个公共件，
// 第 5.4 节那句"四件"是数过的，多一个文件就多一处"谁许动谁不许动"的口径。
//
// 什么情况下会错、出错时人看得见什么：
// ① 加了文件忘写 init() 注册 → 它静静躺在包里，界面看不见 → 门禁 G8 当场红（包内方式文件集合 ⇄ 注册表 id 集合等值）。
// ② 为了"登记"它去改第二个文件（手写一份方式清单、改菜单数据）→ R6 作废 → G11 红（方式 id 的字面量只许出现在它自己那个文件里）。
// ③ 把上限数字抄第二份 → 一轮取数把用户网络跑死、界面全转圈 → G11 红（数字常量只在 budget.go 一处）。
// ④ 方式自己说"我这批 IP 能用" → 写进 hosts 后整条域名打不开 → 这一层不判能不能服务，判据在 detect；
//
//	本包不许 import detect（第 8 节第 7 条）。
//
// 怎么回来：三条门禁都是"测试当场红并指名到文件"，改的永远是新加的那一个文件，不许把断言放宽。
package resolve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// Resolver 是一种解析方式对外露出的全部形状：只有这两个方法。
//
// 为什么只给两个：加第三个（比如 Score()）＝把"该不该用我"的判定塞回方式自己手里，
// 第 6.2 节那两把尺子立刻混成一把，M2 验收那句"排名不再乱跳"当场作废，而代码看着还是对的。
// 这条形状由用例钉住（resolve_test.go 里数方法数）。
type Resolver interface {
	// Spec 交出这个方式的自我介绍，注册时用的就是它。
	Spec() Spec
	// Resolve 交出一批候选地址。它只负责"弄到地址"，不负责"弄到的地址好不好"。
	Resolve(ctx context.Context, q Query) (Set, error)
}

// Spec 是一个方式的自我介绍。它是数据，不是策略：谁参选由 service 按 Kinds 过滤，怎么排由 Rank 定。
type Spec struct {
	// ID 就是文件名去掉 .go：github520.go ⇄ id "github520"。这条对应关系由门禁 G8 两侧等值守着。
	ID string
	// Title 是界面"IP 来源"那一行显示的名字。
	// 它留在这里而不是服务侧的文案表里，是为了兑现 D6 乙那句"加一种方式只加一个文件"；
	// 代价如实记在这儿：这一支只有中文，以后要出英文界面，方式名是唯一得不补的东西。
	Title string
	// Kinds 是适用的主菜单块 id（menu.json 里那个 id）。空＝没声明适用范围，注册当场抛，不让它静默参选全部。
	Kinds []string
	// NeedsProbe 为真表示"这个方式交回的地址必须探过才知道好坏"，为假表示可以直接用（cache 命中、constant 屏蔽）。
	NeedsProbe bool
	// DefaultPriority 是没有历史样本时排老几，数字小的在前。冷启动那一次全靠它（第 6.6 节）。
	DefaultPriority int
	// OfflineOK 为真表示这条方式不占外网配额，断网也能交东西。
	OfflineOK bool
	// Rankable 为假表示这个方式不进 S/V 比较——只有固定屏蔽那种"钉死一个地址、不该被速度顶掉"的方式用false。
	//
	// 为什么要多这一个字段（第 0.1 节第 3 问）：G11 不许 rank.go 里出现方式 id 的字面量，
	// 所以"谁不参与选优"必须由方式自己声明，不能靠排名表里点名 constant。
	Rankable bool
}

// Query 是一次取数要知道的东西。只有域名是真的要用，别的字段都得有对手方才行。
type Query struct {
	// Domain 是要弄 IP 的那条域名。
	Domain string
	// Known 是上一轮检测确认过"能服务这条域名"的地址，由 service 喂进来。
	// 为什么要由外面喂：第 8 节第 8 条管住"resolve 不许 import store"——样本放哪儿归 service 管，这一层只吃参数。
	Known []string
}

// Set 是一个方式交回的一批候选地址。
type Set struct {
	// ModeID 是交回这批地址的方式 id，界面"IP 来源"那一行显示的就是它。
	ModeID string
	// IPs 按取数源给出的顺序，已去重。这里不排序：顺序由 detect 那层的两项指标定，不是由取数源定。
	IPs []string
	// Took 是这个方式自己花掉的时间，它是第 6.2 节那把尺子 V 的原料。
	// 注意它是"取清单的时间"，不是"这个 IP 的握手延迟"——两个数各管一层，混了界面延迟列就成了取数时间。
	Took time.Duration
}

// ctxError 把三件事分开：被叫停、太慢、还有得跑。
//
// 为什么要分开：暂停／取消那一刻，方式交回的不是"我弄不到 IP"，而是"没让我弄"。
// 两者混成一个码，service 就给这个方式记一条失败样本 → 一个只是被用户按停的方式在 S 上掉一档，
// 下一轮它真被名次顶下去，界面看不出任何原因。所以：
//
//	context.Canceled → GAC-TASK-003，service 见这个码不采样；
//	DeadlineExceeded（预算的帽子到点）→ GAC-DETECT-002，这就是一次失败，必须采样，
//	  否则一个每次都慢到掐表的方式能一直赖在名单里。
func ctxError(modeID string, ctx context.Context) error {
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		return errs.New(errs.CodeTaskPaused, map[string]any{"mode": modeID}, "取数被叫停：%s", ctx.Err())
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return errs.New(errs.CodeDetectTimeout, map[string]any{"mode": modeID, "limit": ModeTimeout.String()},
			"取数超过预算 %v，本轮这个方式没交回东西", ModeTimeout)
	}
	return nil
}

// takeTimeout 给一次取数戴上预算的帽子。数字只有一个家（budget.go 的 ModeTimeout），这里只是把它套到 ctx 上。
func takeTimeout(ctx context.Context, modeID string) (context.Context, func(), error) {
	if err := ctxError(modeID, ctx); err != nil {
		return ctx, func() {}, err
	}
	cctx, cancel := context.WithTimeout(ctx, ModeTimeout)
	return cctx, cancel, nil
}

// fetchText 取回一份文本：带上正确域名的事由各方式自己在请求里做，这里只管"发一次、拿回正文"。
//
// 什么情况下会错、出错时人看得见什么：状态码不是 200 一律算失败——坏掉的镜像最爱干的事是回一页 404 的 HTML，
// 那种内容喂进解析器只会得到"源里没这一条"，把"源坏了"说成"这条域名没来源"是误导。
// 正文只读到 FetchBodyBytes 为止：正常源实测 2.5 KB，对面给多少读多少＝一个坏源能把内存吃满。
// 它回的是普通错误，不带方式码——各方式拿到后自己包成 GAC-MODE-<自己>-001，别让共用的东西替谁编码。
func fetchText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s 给了 %s，不是 200", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, FetchBodyBytes))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
