package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/detect"
	"github.com/hlep/github-accel/internal/dnsflush"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/hosts"
	"github.com/hlep/github-accel/internal/resolve"
	"github.com/hlep/github-accel/internal/settings"
)

// Service 是队列加状态的那一个本体。除了下面这几个字段，别处不许再存运行态。
type Service struct {
	cfg Config

	// mu 守下面所有可变字段。事件里的快照就在这把锁里组装：一次锁、一份完整状态。
	mu        sync.Mutex
	startedAt time.Time

	// order 是条目 id 的固定顺序（按菜单 order、再按条目在菜单里的先后）。
	// 快照里 items 的順序由它定，不让 map 的随机序跑到界面上（那是"每轮刷新都在重排"那种毛病的根）。
	order []string
	items map[string]*itemState
	// conflicts 与 unsupported 是清单里那两类不合格项，New 时一次算好。
	// 为什么要提前算：写入那一刻才知道"这个域名两条条目抢"就等于让用户先看一次假成功。
	conflicts   map[string][]string
	unsupported map[string][]string

	// samples 是各方式最近这些轮的样本（第 6.2 节两把尺子的原料）。
	// 它留在这儿而不是 store：store 本轮不建（技术方案第 7 节那条），而"当前状态"本来就不许从库里读〔已定：D3 甲〕。
	samples map[string][]resolve.Sample

	tasks   []*task
	taskNo  int
	batchNo int
	// running 是此刻正在跑的那一条；为 nil 表示队列空转。暂停时它照样能跑完，只是不再取下一条。
	running *task

	paused   bool
	stopping bool
	wake     chan struct{}
	stop     chan struct{}
	lane     sync.WaitGroup

	// writeHeld 是"此刻有写入在跑"，GetStatus 的 writeLockHeld 由它给真值（lock.go 管开关）。
	writeHeld bool
	// lastErr 那一句是给日志与控制台看的原文；下面两栏才是界面要的东西（契约 v9）。
	// 三样必须一起改，所以只许走 noteStateErr 那一个口子——分开写就会出现"新句子配旧码"。
	lastErr       string
	lastErrCode   string
	lastErrParams map[string]any

	// env 是这台机器上"界面要说得出名字但那三个数"：谁管解析、数据放哪儿、改的是哪份文件。
	// 在 New 里一次算好而不是每次快照现取：这三条在进程活着期间不变，
	// 而每条事件都带一份快照——每推一次就 ReadFile 一遍 resolv.conf 是白多一处系统调用。
	// 取不到的一律留空串，界面显示"—"（C5：宁可显示"不知道"，不许编一个名字）。
	env environment
}

// environment 是上面那三个数的形状。它不进契约（契约里是 State 那三个平铺的字段）：
// 这一层只是服务内部的暂存，加字段不必动契约，动契约得推版本号。
type environment struct {
	dnsService string
	dataDir    string
	hostsPath  string
}

// New 把依赖接好并认出全部条目。它不启动队列，也不碰磁盘以外的任何东西。
//
// 什么情况下会错、出错时人看得见什么：Catalog 或 Engine 没给＝编程错误，当场抛并指名缺哪一样，
// 不留下一个"起得来但什么都干不了"的服务；清单里一个条目也没有＝embed 漂了，同样当场抛。
func New(cfg Config) *Service {
	if cfg.Catalog == nil {
		panic("service: Config.Catalog 没给，服务一条条目都不认识")
	}
	if cfg.Engine == nil {
		panic("service: Config.Engine 没给，等于允许在方法里现造一个写死路径的引擎")
	}
	if cfg.Inspector == nil {
		cfg.Inspector = detect.NewChecker()
	}
	if cfg.Modes == nil {
		cfg.Modes = registryModes{}
	}
	if cfg.Flusher == nil {
		cfg.Flusher = dnsflush.New()
	}
	if cfg.Opener == nil {
		cfg.Opener = defaultOpener
	}
	if cfg.HostsPath == "" {
		cfg.HostsPath = hosts.DefaultPath()
	}

	s := &Service{
		cfg:         cfg,
		startedAt:   time.Now(),
		order:       []string{},
		items:       map[string]*itemState{},
		samples:     map[string][]resolve.Sample{},
		conflicts:   map[string][]string{},
		unsupported: map[string][]string{},
		wake:        make(chan struct{}, 1),
		stop:        make(chan struct{}),
	}
	for _, m := range cfg.Catalog.Menus() {
		for _, it := range m.Items {
			if _, dup := s.items[it.ID]; dup {
				panic(fmt.Sprintf("service: 条目 id %q 在两块菜单里重复出现", it.ID))
			}
			s.order = append(s.order, it.ID)
			s.items[it.ID] = &itemState{bestIPs: map[string]string{}}
		}
	}
	if len(s.order) == 0 {
		panic("service: 清单里一条条目也没有，菜单数据漂了")
	}
	s.env = environment{dnsService: dnsflush.ServiceName(), hostsPath: cfg.HostsPath}
	if dir, err := settings.Dir(); err == nil {
		s.env.dataDir = dir
	}
	for _, c := range cfg.Catalog.DomainConflicts() {
		s.conflicts[c.Domain] = append(s.conflicts[c.Domain], c.Items...)
	}
	for _, u := range cfg.Catalog.UnsupportedDomains() {
		s.unsupported[u.ItemID] = append(s.unsupported[u.ItemID], u.Value)
	}
	return s
}

// Start 放出那一条串行通道。它立刻返回，跑任务的是自己那道 goroutine。
//
// 为什么只有一条通道：写入必须串行（L3 的永久解），检测要并发就让它排在通道里慢慢跑——
// 本期"卡不卡"的判据还没给数（技术方案第 14 节那条仍开口的），先按最不容易出错的形状做。
func (s *Service) Start(ctx context.Context) {
	s.lane.Add(1)
	go func() {
		defer s.lane.Done()
		s.pump(ctx)
	}()
}

// Stop 叫停队列并等它把手上那条跑完。在飞的网络调用由 ctx 取消，不会留下半个写入。
func (s *Service) Stop() {
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
	close(s.stop)
	s.notify()
	s.lane.Wait()
}

// notify 提醒通道"状态变了，可以重新扫一遍"。
//
// 为什么用容量 1 的通道而不是 sync.Cond：唤醒要能和 stop 放在同一个 select 里等，
// 用 Cond 就得把等待挪到别的 goroutine，反而多出两处状态。
// 两次唤醒合成一次是安全的：醒来的人重新扫表，扫到的是此刻的真实状态，不是"当时那一下"。
func (s *Service) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// settingsPath 给出设置文件在哪儿。留空才现取，取法只在这一处。
func (s *Service) settingsPath() (string, error) {
	if s.cfg.SettingsPath != "" {
		return s.cfg.SettingsPath, nil
	}
	return settings.Path()
}

// loadSettings 读那 13 个键。文件还没写过＝默认值，不算错（这是"第一次用"那档）。
func (s *Service) loadSettings() (settings.Settings, error) {
	p, err := s.settingsPath()
	if err != nil {
		return settings.Settings{}, err
	}
	return settings.LoadFrom(p)
}

// state 给服务的状态那一块。Phase 用契约里那两条常量，别处不许现编字符串。
func (s *Service) stateLocked() contract.State {
	return contract.State{
		Name:            serviceName,
		Version:         s.cfg.Version,
		PID:             os.Getpid(),
		StartedAt:       s.startedAt.UTC().Format(time.RFC3339),
		Phase:           contract.ServiceRunning,
		QueuePaused:     s.paused,
		LastError:       s.lastErr,
		LastErrorCode:   s.lastErrCode,
		LastErrorParams: s.lastErrParams,
		DnsService:      s.env.dnsService,
		DataDir:         s.env.dataDir,
		HostsPath:       s.env.hostsPath,
	}
}

// noteStateErr 是状态栏那一句"为什么"唯一的写入口。调用方必须已持有 s.mu。
//
// 干什么用：状态栏那一格要说的话，和用户点开队列那一行看到的是同一件事故；
// 而这一格在界面那份语言文件里凑句子，凑句子要的是码与参数，不是一句已经拼死的中文〔已定：D6 乙〕。
// 什么情况下会错、出错时人看得见什么：只写原文（上一版的形状）→ 界面拿到一句服务侧的中文，
// 换不成用户那门语言；于是同一屏里队列那一行说的是本地话、状态栏那句还是中文，
// 看的人以为工具有两套说法，实际是契约少了两栏。
// 怎么回来：新增一栏时只改这里；调用点一律递一个 error（或一个 Caveat），
// 不许在外面各拼一遍 "%s: %v"——那是把第二个家的键名格式散到四处。
func (s *Service) noteStateErr(err error) {
	s.lastErr = err.Error()
	s.lastErrCode = errs.CodeOf(err)
	s.lastErrParams = errs.ParamsOf(err)
}

// noteStateCaveat 是"写成了但该知道"那一档写进状态栏的口：内容形状与 noteStateErr 一样（码＋参数＋一句原文），
// 只是原料来自 hosts.Caveat 而不是 error。为什么单开一个：Caveat 不是失败，
// 把它包成 error 就会让上面那一句"这不是主判断"的规矩变成假话。
func (s *Service) noteStateCaveat(c hosts.Caveat) {
	s.lastErr = fmt.Sprintf("%s: %v", c.Code, c.Params)
	s.lastErrCode = c.Code
	s.lastErrParams = c.Params
}

const serviceName = "github-accel-service"
