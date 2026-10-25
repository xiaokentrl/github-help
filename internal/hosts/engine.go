package hosts

import (
	"context"
	"os"
	"strings"

	"github.com/hlep/github-accel/internal/errs"
)

// Engine 是"碰这个文件"的唯一出口。做成接口只为一件事：用例要能换成内存假件，
// 而真引擎必须碰磁盘——两边不许共用一条代码路径（第 11.2 节那条"能被换掉的那一处边界"）。
type Engine interface {
	// Read 现读现算：GetStatus 里"这条域名此刻指向哪儿"永远来自这儿，不来自数据库〔已定：D3 甲〕。
	Read() (*Doc, error)
	// Apply 走完整五步并返回回执；一批改动是一个原子动作，不做"成了大半"。
	Apply(ctx context.Context, changes []Change) (Receipt, error)
	// Restore 撤掉这批域名的托管。单项恢复与"全部恢复默认"共用这一个执行体，参数决定范围〔已定〕。
	Restore(ctx context.Context, domains []string) (Receipt, error)
}

// Caveat 是"这次写成了，但有件事人该知道"。
// 为什么不做成 error，也不存整句中文：按 D6 乙，服务侧只回码与结构化参数，整句人话在前端语言文件那一个家。
// 混进 error 会让人以为这一次没生效——而"生效了没有"是这件事的主判断，不能被附属信息盖过去。
type Caveat struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
}

// Receipt 是一次写入的回执。
type Receipt struct {
	Path string `json:"path"`
	// ChangedLines 是"这次真动了几行"（改掉的加删掉的加写上的）。为 0 表示本来就没什么可改，幂等那条靠它验。
	ChangedLines int `json:"changedLines"`
	// RolledBack 为真表示写完重读对不上、已经用备份写回去了。界面上那条状态要回落到写之前的值。
	RolledBack bool `json:"rolledBack"`
	// BackupPath 是人能找到改动前那份内容的地方（存在配置目录里，不往 hosts 旁边丢文件）。
	BackupPath string `json:"backupPath,omitempty"`
	// Caveats 装 GAC-HOSTS-010/011 那两条"写成了但该知道"的事，见 Caveat 那段说明。
	Caveats []Caveat `json:"caveats,omitempty"`
}

// addCaveat 往回执上记一条附属信息。
func (r *Receipt) addCaveat(code string, params map[string]any) {
	r.Caveats = append(r.Caveats, Caveat{Code: code, Params: params})
}

// FileEngine 是真引擎。刷 DNS 那一步不在这里做——dnsflush 由服务在写完之后叫一次（第 8 节依赖方向）。
type FileEngine struct {
	path      string
	backupDir string
}

// NewFileEngine 用给定的 hosts 路径与备份目录造一个引擎。两个路径都由调用方给，界面永远不写死〔已定：C5〕：
// 服务启动时把 DefaultPath() 与 settings.BackupDir() 传进来，用例传临时目录。
// 所以这一份代码一辈子不需要"是不是在测试里"这种判断。
// backupDir 给空串等于"不存备份"：那条如实进回执的 Caveats，不会假装存过了。
func NewFileEngine(path, backupDir string) *FileEngine {
	return &FileEngine{path: path, backupDir: backupDir}
}

var _ Engine = (*FileEngine)(nil)

// Path 报告这个引擎写的是哪个文件。
func (e *FileEngine) Path() string { return e.path }

// Read 读文件并解析。文件不存在按空文档算：
// 用例指向一个还没建的临时路径时，"没内容"是真的而不是错——写下去就会建出来。
func (e *FileEngine) Read() (*Doc, error) {
	raw, err := e.readRaw()
	if err != nil {
		return nil, err
	}
	return Parse(string(raw)), nil
}

func (e *FileEngine) readRaw() ([]byte, error) {
	raw, err := os.ReadFile(e.path)
	if os.IsNotExist(err) {
		return []byte(nil), nil
	}
	if err != nil {
		return nil, errs.New(errs.CodeHostsRead, map[string]any{"path": e.path}, "读不到 hosts").WithCause(err)
	}
	return raw, nil
}

// Apply：备份 → 合并 → 写临时 → rename → 补属主与权限 → 重读校验 → 对不上就回滚。
func (e *FileEngine) Apply(ctx context.Context, changes []Change) (Receipt, error) {
	return e.run(ctx, changes)
}

// Restore 撤托管：先按现读的文件找出每条域名当初是哪个条目写的，再交给同一个执行体。
func (e *FileEngine) Restore(ctx context.Context, domains []string) (Receipt, error) {
	doc, err := e.Read()
	if err != nil {
		return Receipt{}, err
	}
	byDomain := map[string]string{}
	for _, l := range doc.Lines() {
		if l.Managed {
			for _, n := range l.Names {
				byDomain[n] = l.ItemID
			}
		}
	}
	changes := make([]Change, 0, len(domains))
	for _, d := range domains {
		changes = append(changes, Change{Domain: d, IP: "", ItemID: byDomain[d]})
	}
	return e.run(ctx, changes)
}

// run 是 Apply 与 Restore 共用的那一个执行体。
func (e *FileEngine) run(ctx context.Context, changes []Change) (Receipt, error) {
	original, err := e.readRaw()
	if err != nil {
		return Receipt{}, err
	}
	next, err := plan(Parse(string(original)), changes)
	if err != nil {
		return Receipt{Path: e.path}, err
	}
	want := next.Render()
	rec := Receipt{Path: e.path, ChangedLines: countDiffLines(string(original), want)}
	if string(original) == want {
		return rec, nil // 本来就长这样：一个字不写
	}
	if err := ctx.Err(); err != nil {
		return rec, errs.New(errs.CodeTaskPaused, nil, "这一批写入在动手前就被撤了").WithCause(err)
	}

	// 先把改动前那份存进配置目录：回滚和"手工放回"都拿它当依据。
	backupPath, bkErr := writeBackup(e.backupDir, e.path, original)
	rec.BackupPath = backupPath
	if bkErr != nil {
		// 备份没存下不是拦下写入的理由：写还是要写，但要把这件事带进回执，不许瞒。
		rec.addCaveat(errs.CodeHostsNoBackup, map[string]any{"dir": e.backupDir, "cause": bkErr.Error()})
	}

	mode, uid, gid := statFor(e.path)
	if err := e.replace(want, mode); err != nil {
		return rec, err
	}
	owned, ownErr := own(e.path, mode, uid, gid)
	if ownErr != nil {
		return e.finishFailed(rec, original, backupPath, "新文件的权限补不回原来那一份", ownErr)
	}
	if !owned {
		rec.addCaveat(errs.CodeHostsOwnerKept, map[string]any{"path": e.path})
	}

	if err := e.verify(changes); err != nil {
		return e.finishFailed(rec, original, backupPath, "写完重读对不上", err)
	}
	return rec, nil
}

// finishFailed 是"写完发现不对"那一步：还原，还原也失败就报最坏那一档。
// 码沿用 cause 自己的码（GAC-HOSTS-003／004），只把备份路径补进参数里，让人能找到依据。
func (e *FileEngine) finishFailed(rec Receipt, original []byte, backupPath, reason string, cause error) (Receipt, error) {
	if rbErr := e.rollback(original); rbErr != nil {
		return rec, rollbackFailed(e.path, backupPath, rbErr) // 这条分支下 RolledBack 保持 false：想还原而没还原成
	}
	rec.RolledBack = true
	rec.ChangedLines = 0 // 文件又回到动手前那一份，"动了几行"这个问题的答案重新是 0
	return rec, errs.New(errs.CodeOf(cause), map[string]any{"path": e.path, "backup": backupPath},
		"%s 没改成（%s），已经还原成写之前的样子", e.path, reason).WithCause(cause)
}

// rollback 把改动前那份原样写回。它自己失败是最坏那一档，见 rollbackFailed。
func (e *FileEngine) rollback(original []byte) error {
	mode, uid, gid := statFor(e.path)
	if err := e.replace(string(original), mode); err != nil {
		return err
	}
	_, err := own(e.path, mode, uid, gid)
	return err
}

func rollbackFailed(path, backupPath string, cause error) error {
	return errs.New(errs.CodeHostsRollback, map[string]any{"path": path, "backup": backupPath},
		"写完发现不对，还原也失败了——请手工把备份放回 %s", path).WithCause(cause)
}

// verify 重读文件，按域名逐条确认这一批改动真的落在文件里。
//
// 为什么按域名而不是按行号或整文件比：整文件比会碰上"别人这一刻也在改 hosts"，
// 那种情况不该判成我们写坏了；行号在 rename 之后没有意义。
// 问"这台机器现在按哪个地址解析这个域名"才是这件事的真正判据（取值口径与 Doc.Mappings 同一个家）。
func (e *FileEngine) verify(changes []Change) error {
	doc, err := e.Read()
	if err != nil {
		return errs.New(errs.CodeHostsVerify, map[string]any{"path": e.path}, "写完读回来核对失败").WithCause(err)
	}
	got := doc.Mappings()
	for _, c := range changes {
		if c.IP != "" {
			if got[c.Domain] != c.IP {
				return errs.New(errs.CodeHostsVerify, map[string]any{
					"domain": c.Domain, "want": c.IP, "got": got[c.Domain],
				}, "%s 现在指向 %s，不是要的 %s", c.Domain, orNone(got[c.Domain]), c.IP)
			}
			continue
		}
		// 撤托管：我们的那行必须不在了；应用前的地址要回来（要么原行放开，要么按 prev 补一行）。
		for _, l := range doc.Lines() {
			if l.Managed && hasName(l.Names, c.Domain) {
				return errs.New(errs.CodeHostsVerify, map[string]any{
					"domain": c.Domain, "itemId": l.ItemID,
				}, "%s 那行托管还留在文件里，没撤掉", c.Domain)
			}
		}
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "没有一行（本机 DNS 说了算）"
	}
	return s
}

// countDiffLines 数这次动了几行，给人一个能核对的数而不是"成功"两个字。
// 算法就一句：两边各数一遍出现次数，多出来和少掉的都是动过的。
func countDiffLines(before, after string) int {
	bc := lineCounts(before)
	ac := lineCounts(after)
	n := 0
	for line, in := range bc {
		out := ac[line]
		if in > out {
			n += in - out
		}
	}
	for line, out := range ac {
		in := bc[line]
		if out > in {
			n += out - in
		}
	}
	return n
}

func lineCounts(text string) map[string]int {
	out := map[string]int{}
	if text == "" {
		return out
	}
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		out[l]++
	}
	return out
}
