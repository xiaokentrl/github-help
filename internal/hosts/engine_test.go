package hosts

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// 这一整组用例都写临时文件，一次也不碰 /etc/hosts〔已定：他不点，我们一个字不碰〕。
// 路径由 NewFileEngine 传进来，所以引擎里根本没有"真路径"这个常量可走。

type fixture struct {
	dir       string
	hostsPath string
	backupDir string
}

// newFixture 造一间只有 hosts 和备份目录的临时房子。
func newFixture(t *testing.T, initial string) *fixture {
	t.Helper()
	dir := t.TempDir()
	f := &fixture{dir: dir, hostsPath: filepath.Join(dir, "hosts"), backupDir: filepath.Join(dir, "backups")}
	if initial != "" {
		if err := os.WriteFile(f.hostsPath, []byte(initial), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f *fixture) engine() *FileEngine { return NewFileEngine(f.hostsPath, f.backupDir) }

func (f *fixture) content(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(f.hostsPath)
	if err != nil {
		t.Fatalf("读回 hosts 失败：%v", err)
	}
	return string(raw)
}

func (f *fixture) backups(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(f.backupDir, "hosts-backup-*"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

// hasCaveat 按码找，不按句子找——服务侧只回码与参数〔已定：D6 乙〕，所以这才是可验的形状。
func hasCaveat(rec Receipt, code string) bool {
	for _, c := range rec.Caveats {
		if c.Code == code {
			return true
		}
	}
	return false
}

func TestApplyWritesFileKeepsModeAndTakesBackup(t *testing.T) {
	const initial = "# 说明行\n127.0.0.1\tlocalhost\n20.205.243.166 github.com\n"
	f := newFixture(t, initial)
	rec, err := f.engine().Apply(context.Background(),
		[]Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}})
	if err != nil {
		t.Fatalf("这一批该写成功：%v", err)
	}
	want, err := Plan(Parse(initial), []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.content(t); got != want {
		t.Fatalf("文件内容不是合并出来的那一份：\n got=%q\nwant=%q", got, want)
	}
	// 原行改成停用行是"少一行 + 多两行"，一共动了 3 行。
	if rec.ChangedLines != 3 {
		t.Fatalf("动了几行数错了：got %d want 3", rec.ChangedLines)
	}
	if len(rec.Caveats) != 0 {
		t.Fatalf("这一步不该有附属提醒：%v", rec.Caveats)
	}

	fi, err := os.Stat(f.hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("权限没补回来，现在是 %v（hosts 不可读等于整台机器解析不了）", fi.Mode().Perm())
	}

	bk := f.backups(t)
	if len(bk) != 1 {
		t.Fatalf("备份该有 1 份，实际 %d 份", len(bk))
	}
	raw, err := os.ReadFile(bk[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != initial {
		t.Fatalf("备份存的不是改动前那一份：\n got=%q\nwant=%q", raw, initial)
	}
	if fi, err := os.Stat(bk[0]); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Fatalf("备份里是整份 hosts，权限该 0600，实际 %v", fi.Mode().Perm())
	}
	if rec.BackupPath != bk[0] {
		t.Fatalf("回执给的备份路径跟实际存的不是同一个：%q vs %q", rec.BackupPath, bk[0])
	}
}

func TestApplySameChangesTwiceWritesNothing(t *testing.T) {
	f := newFixture(t, "# 说明行\n127.0.0.1\tlocalhost\n")
	changes := []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}}
	first, err := f.engine().Apply(context.Background(), changes)
	if err != nil {
		t.Fatal(err)
	}
	before := f.content(t)
	mtimeBefore := statModTime(t, f.hostsPath)

	second, err := f.engine().Apply(context.Background(), changes)
	if err != nil {
		t.Fatalf("第二次应用同一批改动不该报错：%v", err)
	}
	if second.ChangedLines != 0 {
		t.Fatalf("幂等那一条要求动 0 行，实际 %d", second.ChangedLines)
	}
	if f.content(t) != before {
		t.Fatal("第二次应用改到了文件")
	}
	if !statModTime(t, f.hostsPath).Equal(mtimeBefore) {
		t.Fatal("第二次应用真的动笔写了文件（mtime 变了）——幂等那一步短路没生效")
	}
	if len(f.backups(t)) != 1 {
		t.Fatalf("没写文件就不该多一份备份，实际 %d 份", len(f.backups(t)))
	}
	if first.ChangedLines != 1 {
		t.Fatalf("第一次只该加一行：got %d", first.ChangedLines)
	}
}

func TestApplyRefusesWithoutTouchingAnything(t *testing.T) {
	// 拒写的那一档必须在动笔之前就拦下：文件、备份两边都不许有痕迹。
	const initial = "20.205.243.166 github.com codeload.github.com\n"
	f := newFixture(t, initial)
	_, err := f.engine().Apply(context.Background(),
		[]Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}})
	if got := errs.CodeOf(err); got != errs.CodeHostsSharedLine {
		t.Fatalf("码不对：got %s want %s（%v）", got, errs.CodeHostsSharedLine, err)
	}
	if f.content(t) != initial {
		t.Fatalf("拒写了还动文件：%q", f.content(t))
	}
	if n := len(f.backups(t)); n != 0 {
		t.Fatalf("拒写了却存了 %d 份备份", n)
	}
	if _, err := os.Stat(f.backupDir); !os.IsNotExist(err) {
		t.Fatal("拒写了连备份目录都不该建出来")
	}
}

func TestApplyHonoursCancelledContext(t *testing.T) {
	f := newFixture(t, "# 说明行\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.engine().Apply(ctx, []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}})
	if got := errs.CodeOf(err); got != errs.CodeTaskPaused {
		t.Fatalf("撤掉的一批要在动笔前停下：got %s（%v）", got, err)
	}
	if f.content(t) != "# 说明行\n" {
		t.Fatalf("撤掉了却把文件写了：%q", f.content(t))
	}
}

func TestApplyCreatesMissingFileAndMissingDirFails(t *testing.T) {
	// 文件不存在但目录在：写下去就会建出来，权限取 0644（hosts 必须人人可读）。
	f := newFixture(t, "")
	rec, err := f.engine().Apply(context.Background(),
		[]Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}})
	if err != nil {
		t.Fatalf("新建 hosts 该成功：%v", err)
	}
	if fi, err := os.Stat(f.hostsPath); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != 0o644 {
		t.Fatalf("新建的文件权限是 %v，不是 0644", fi.Mode().Perm())
	}
	if rec.BackupPath == "" {
		t.Fatal("改动前那一份是空的也要照常存一份：不然大小说不出差别，出事时人找不到依据")
	}
	if raw, err := os.ReadFile(rec.BackupPath); err != nil {
		t.Fatal(err)
	} else if len(raw) != 0 {
		t.Fatalf("备份存的该是那份空内容，实际 %q", raw)
	}

	// 连目录都没有：这不是"权限不够"那种可以重试的事，但同样是写不进去，要给 GAC-HOSTS-003。
	bad := NewFileEngine(filepath.Join(t.TempDir(), "没有这一层", "hosts"), t.TempDir())
	_, err = bad.Apply(context.Background(), []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}})
	if got := errs.CodeOf(err); got != errs.CodeHostsWrite {
		t.Fatalf("got %s（%v）", got, err)
	}
}

func TestApplySucceedsWithEmptyBackupDirButSaysSo(t *testing.T) {
	// 备份目录指到一个普通文件下面：MkdirAll 一定失败。写入本身还是要做完，
	// 但这一件事必须进回执的提醒里，不许瞒。
	f := newFixture(t, "# 说明行\n")
	blocker := filepath.Join(f.dir, "blocker")
	if err := os.WriteFile(blocker, []byte("我不是目录"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := NewFileEngine(f.hostsPath, filepath.Join(blocker, "backups"))
	rec, err := eng.Apply(context.Background(),
		[]Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}})
	if err != nil {
		t.Fatalf("备份存不下不该拦下写入：%v", err)
	}
	if !hasCaveat(rec, errs.CodeHostsNoBackup) {
		t.Fatalf("回执里没带上这件事：%+v", rec.Caveats)
	}
	if !strings.Contains(f.content(t), "140.82.112.22 github.com # gac:core prev:-") {
		t.Fatalf("文件却没写成：%q", f.content(t))
	}
}

func TestReadOnlyDirectoryFailsAndLeavesOriginalUntouched(t *testing.T) {
	const initial = "# 说明行\n127.0.0.1 localhost\n"
	f := newFixture(t, initial)
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("以 root 跑时目录权限拦不住写入；Windows 上权限位根本不由 chmod 表达——两边这条判据都没有意义")
	}
	// 把整间房子设成只读（备份目录也在里面，所以那条 GAC-HOSTS-011 会一起出现）。
	if err := os.Chmod(f.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.dir, 0o700) })

	_, err := f.engine().Apply(context.Background(),
		[]Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}})
	if got := errs.CodeOf(err); got != errs.CodeHostsWrite {
		t.Fatalf("写不进去要给 GAC-HOSTS-003，got %s（%v）", got, err)
	}
	if f.content(t) != initial {
		t.Fatalf("失败了却把原文件动了：%q", f.content(t))
	}
}

func TestVerifyReadsTheFileNotThePlan(t *testing.T) {
	f := newFixture(t, "# 说明行\n")
	changes := []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}}
	if _, err := f.engine().Apply(context.Background(), changes); err != nil {
		t.Fatal(err)
	}
	// 现在人在外面把地址改成别的了：校验问的是"这台机器现在读到什么"，它必须回答"对不上"。
	if err := os.WriteFile(f.hostsPath, []byte("8.8.8.8 github.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := f.engine().verify(changes)
	if got := errs.CodeOf(err); got != errs.CodeHostsVerify {
		t.Fatalf("got %s（%v）", got, err)
	}
	p := errs.ParamsOf(err)
	if p["got"] != "8.8.8.8" || p["want"] != "140.82.112.22" {
		t.Fatalf("校验的错误里要带现在读到什么和要什么，实际参数=%v", p)
	}
}

func TestVerifyRejectsLeftoverManagedLineOnRestore(t *testing.T) {
	f := newFixture(t, "20.205.243.166 github.com\n")
	if _, err := f.engine().Apply(context.Background(),
		[]Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}}); err != nil {
		t.Fatal(err)
	}
	err := f.engine().verify([]Change{{Domain: "github.com", IP: "", ItemID: "core"}})
	if got := errs.CodeOf(err); got != errs.CodeHostsVerify {
		t.Fatalf("撤托管时我们的行还在，校验要拦：got %s（%v）", got, err)
	}
}

func TestFinishFailedRollsBackToTheOriginal(t *testing.T) {
	const initial = "# 说明行\n127.0.0.1 localhost\n"
	f := newFixture(t, initial)
	eng := f.engine()
	if _, err := eng.Apply(context.Background(), []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}}); err != nil {
		t.Fatal(err)
	}
	written := f.content(t)
	if written == initial {
		t.Fatal("前一步没写进去，后面测不了回滚")
	}

	rec := Receipt{Path: f.hostsPath, ChangedLines: countDiffLines(initial, written)}
	cause := errs.New(errs.CodeHostsVerify, map[string]any{"domain": "github.com"}, "假装校验没过")
	out, err := eng.finishFailed(rec, []byte(initial), "/tmp/hosts-backup-测试", "假装的事", cause)
	if !out.RolledBack {
		t.Fatal("还原成功了却没报告已回滚")
	}
	if out.ChangedLines != 0 {
		t.Fatalf("文件已经回到动手前那一份，动了几行该重新是 0，got %d", out.ChangedLines)
	}
	if got := f.content(t); got != initial {
		t.Fatalf("回滚没把原文件放回去：\n got=%q\nwant=%q", got, initial)
	}
	if got := errs.CodeOf(err); got != errs.CodeHostsVerify {
		t.Fatalf("回滚之后要沿用校验那条码，got %s", got)
	}
	if p := errs.ParamsOf(err); p["backup"] != "/tmp/hosts-backup-测试" {
		t.Fatalf("错误参数里要带上备份路径，实际 %v", p)
	}
	// 权限也得跟着回来：回滚走的同一条写入路径，不该顺手把文件改成 0600。
	if fi, err := os.Stat(f.hostsPath); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != 0o644 {
		t.Fatalf("回滚之后权限成了 %v", fi.Mode().Perm())
	}
}

func TestRollbackFailureIsTheWorstCase(t *testing.T) {
	f := newFixture(t, "# 说明行\n127.0.0.1 localhost\n")
	eng := f.engine()
	if _, err := eng.Apply(context.Background(), []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}}); err != nil {
		t.Fatal(err)
	}
	const initial = "# 说明行\n127.0.0.1 localhost\n"
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("以 root 跑时目录权限拦不住还原；Windows 上同理——两边这条判据都没有意义")
	}
	if err := os.Chmod(f.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.dir, 0o700) })

	out, err := eng.finishFailed(Receipt{Path: f.hostsPath}, []byte(initial), "备份在别处", "假装校验没过",
		errs.New(errs.CodeHostsVerify, nil, "假装校验没过"))
	if got := errs.CodeOf(err); got != errs.CodeHostsRollback {
		t.Fatalf("还原也失败是最坏那一档，要给 GAC-HOSTS-005，got %s（%v）", got, err)
	}
	if out.RolledBack {
		t.Fatal("没还原成却报告已回滚——这条会让人以为文件已经安全了")
	}
	p := errs.ParamsOf(err)
	if p["backup"] != "备份在别处" {
		t.Fatalf("最坏那一档必须把备份路径给出去，实际 %v", p)
	}
}

func TestRestoreOnDiskRoundTrip(t *testing.T) {
	const initial = "# 说明行\n127.0.0.1\tlocalhost\n20.205.243.166 github.com\n"
	f := newFixture(t, initial)
	eng := f.engine()
	if _, err := eng.Apply(context.Background(), []Change{
		{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"},
		{Domain: "raw.githubusercontent.com", IP: "185.199.108.153", ItemID: "assets"},
	}); err != nil {
		t.Fatal(err)
	}
	doc, err := eng.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Lines()) == 0 {
		t.Fatal("Read 读回空文档")
	}
	rec, err := eng.Restore(context.Background(), []string{"github.com", "raw.githubusercontent.com"})
	if err != nil {
		t.Fatalf("恢复默认该成功：%v", err)
	}
	if got := f.content(t); got != initial {
		t.Fatalf("全部恢复之后跟原文不一样：\n got=%q\nwant=%q", got, initial)
	}
	// 恢复之前 5 行（停用行 + 两行我们的行），之后 3 行原行：
	// 少掉 3 行（停用行与两行我们的行）、加回 1 行原行，一共动了 4 行。
	if rec.ChangedLines != 4 {
		t.Fatalf("动了几行：got %d want 4", rec.ChangedLines)
	}
}

func TestRestoreKnowsTheItemFromTheFileItself(t *testing.T) {
	// Restore 只拿到域名，条目 id 是从现读的标记里认出来的——这条链撑着"没有数据库也能恢复"〔已定：D3 甲〕。
	f := newFixture(t, "20.205.243.166 github.com\n")
	eng := f.engine()
	if _, err := eng.Apply(context.Background(), []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Restore(context.Background(), []string{"github.com"}); err != nil {
		t.Fatal(err)
	}
	if got := f.content(t); got != "20.205.243.166 github.com\n" {
		t.Fatalf("恢复出来的原行不对：%q", got)
	}
}

func TestReadMissingFileIsAnEmptyDoc(t *testing.T) {
	eng := NewFileEngine(filepath.Join(t.TempDir(), "hosts"), t.TempDir())
	doc, err := eng.Read()
	if err != nil {
		t.Fatalf("文件不存在该算空文档而不是错：%v", err)
	}
	if len(doc.Lines()) != 0 {
		t.Fatalf("got %d 行", len(doc.Lines()))
	}
	if got, err := eng.readRaw(); err != nil || got != nil {
		t.Fatalf("readRaw 该给空字节，got %q / %v", got, err)
	}
}

func TestDefaultPathShape(t *testing.T) {
	// 只钉形状不钉平台分支的真值：本机不是 Windows，那边照不了〔待核：M5 要在真 Windows 上核〕。
	p := DefaultPath()
	if p == "" || filepath.Base(p) != "hosts" {
		t.Fatalf("DefaultPath 该给一个以 hosts 结尾的路径，got %q", p)
	}
	if !filepath.IsAbs(p) {
		t.Fatalf("路径必须是绝对的，界面才没处写死相对路径：%q", p)
	}
}

func TestBackupPruneKeepsTheNewestTen(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 30; i++ {
		name := "hosts-backup-20261022-" + pad(i) + "00000"
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// 人自己放进去的文件不归这个包管，删它就是删用户的东西。
	keep := filepath.Join(dir, "我自己备份的 hosts")
	if err := os.WriteFile(keep, []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	prune(dir)
	matches, err := filepath.Glob(filepath.Join(dir, "hosts-backup-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != backupKeep {
		t.Fatalf("只该留 %d 份，实际 %d 份", backupKeep, len(matches))
	}
	if matches[backupKeep-1] != filepath.Join(dir, "hosts-backup-20261022-02900000") {
		t.Fatalf("留下的该是最新那几份，最新一份是 %q", matches[backupKeep-1])
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("动了人不归我们管的文件：%v", err)
	}
}

func pad(i int) string {
	s := []byte{'0', '0', '0'}
	n := 0
	for i > 0 {
		s[2-n] = byte('0' + i%10)
		i /= 10
		n++
	}
	return string(s)
}

func statModTime(t *testing.T, path string) time.Time {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.ModTime()
}
