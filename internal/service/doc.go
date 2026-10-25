// Package service 是后台服务里唯一持有可变状态的地方：队列、任务状态机、写入串行锁、快照组装，
// 以及一轮检测里把"选方式"（resolve 那一层）和"选 IP"（detect 那一层）接起来的那一处。
//
// 干什么用：界面上每一个动作（检测、应用、恢复、暂停、取消）最后都要落到这里的一条队列上，
// 由它决定"现在跑哪一条、下一条跑哪一条、跑完把什么推回界面"。
//
// 什么情况下会错、出错时人看得见什么：
// ① 谁来发起都拦在同一道判定上 → 开机自动检测被自己人拦掉，界面永远不出结果（L1）。
//
//	所以 trigger.go 只管"谁来发起"，它不参与任何判定，跑不跑由队列定。
//
// ② 收尾时顺手把暂停标志清掉 → 用户点一次暂停没停住，要点两下（L2）。
//
//	所以"暂停"是队列的一个正式状态，只有恢复那一个动作能把它关掉。
//
// ③ 还原那 900 毫秒里三个按钮都能点，两条写入真同时起跑 → 后写的把前写的盖掉（L3）。
//
//	所以写入串行锁在这儿（lock.go），界面上的置灰只挡手点，不当判据。
//
// ④ 事件只带两个字段 → 界面靠猜补全，快照跟文件对不上。
//
//	所以每条事件都过 snapshot.go 这一个组装处，快照里的"当前写入 IP"永远现读 hosts 算。
//
// 怎么回来：这四件事各有一条用例钉着（trigger／pause／lock／snapshot 四组），红了就改回那一个文件，
// 不许把断言放宽。
package service

import (
	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/detect"
	"github.com/hlep/github-accel/internal/dnsflush"
	"github.com/hlep/github-accel/internal/hosts"
	"github.com/hlep/github-accel/internal/menu"
	"github.com/hlep/github-accel/internal/resolve"
)

// Broadcaster 是本包唯一往外推事件的出口，由 ipc.Server 实现。
//
// 为什么必须是接口：队列那几条用例要收到事件就得造一条真通道，测试会变得又慢又脆；
// 换成假件之后"每条事件都带完整快照"这一条反而是最好验的（技术方案第 11.2 节列的六个接口之一）。
type Broadcaster interface {
	Broadcast(event string, snap contract.Snapshot, payload map[string]any)
}

// ModeSource 是"这一轮有哪些解析方式参选"的出口，默认就是 resolve 自己的注册表。
//
// 为什么把注册表包一层而注册表本身不做接口（第 11.2 节那条刻意的不对称）：
// 方式必须能加（R6），所以调用点不许认死方式；但"清单本身"只有注册表一份实现，
// 给它再做一个接口就等于给方式清单开第二个家。
// 这一层的唯一用处是让检测那几条用例能钉死返回值——外网不可控，真注册表进用例就等于把测试挂在网络上。
type ModeSource interface {
	// Specs 给出全部已登记的方式（顺序稳定）。
	Specs() []resolve.Spec
	// Resolver 按 id 取方式实现；取不到时 ok 为假，调用方必须如实报，不许静默换人。
	Resolver(id string) (resolve.Resolver, bool)
}

// registryModes 是生产用的那一份：只做转发，不含任何判断。
type registryModes struct{}

func (registryModes) Specs() []resolve.Spec { return resolve.All() }

func (registryModes) Resolver(id string) (resolve.Resolver, bool) { return resolve.Get(id) }

// Config 是一次组装要用的全部依赖。每一个都由调用方给，本包不许在方法里现取。
//
// 什么情况下会错、出错时人看得见什么：依赖在方法里现取（比如 handler 里直接 hosts.NewFileEngine(写死路径)）
// → 用例碰不到第二条路径，界面写死路径那个老毛病（已定的 C5）就从后门回来了。
// 怎么回来：除 Catalog 与 Engine 之外都允许留空，留空就是取本机的真实现；
// 取的口径只有 New 这一处，别处不许再写一遍"空就换默认"。
type Config struct {
	// Catalog 是校验过的域名清单，必填——没有它整个服务一条条目都不认识。
	Catalog *menu.Catalog
	// Engine 是碰 hosts 的唯一出口，必填——用例传内存假件，cmd/service 传真引擎。
	Engine hosts.Engine
	// Inspector 是探测那一步（指标一＋指标二），留空用 detect.NewChecker()。
	Inspector detect.Inspector
	// Modes 是参选方式清单，留空用 resolve 的注册表。
	Modes ModeSource
	// Flusher 是刷本机 DNS 那一步，留空用 dnsflush.New()。
	Flusher dnsflush.Flusher
	// Opener 是"打开外部页面"那一步，留空用 opener.go 的系统打开器。用例传假件记 URL，不真拉浏览器。
	Opener Opener
	// Broadcaster 是事件出口，留空就是不往外推（cmd 侧没建通道时的形状）。
	Broadcaster Broadcaster
	// HostsPath 只给权限自检那一句话用（写不写得动问的是路径，不是引擎）。留空取 hosts.DefaultPath()。
	HostsPath string
	// SettingsPath 是那 13 个偏好键的文件。留空取 settings.Path()。
	SettingsPath string
	// Version 与 BuildDate 原样进 GetVersion 的回执。BuildDate 没填就如实留空。
	Version   string
	BuildDate string
}
