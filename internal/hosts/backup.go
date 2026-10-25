package hosts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hlep/github-accel/internal/errs"
)

// backupKeep 是备份目录里最多留几份。
// 为什么要这个数：这个函数每次写入都存一份改动前的全文，不留上限就是在配置目录里越堆越多，
// 半年后没人知道自己攒了几个 G 的 hosts 副本。取 10 是因为"最近十次"够人往回找了。
const backupKeep = 10

// backupNameFmt 决定文件名形状。它是这个目录唯一的辨认依据：
// 只有按这个前缀扫出来的文件才归我们管，别的名字一律不碰——包括人自己放进去的备份。
const backupNameFmt = "hosts-backup-%s"

// writeBackup 把"动手之前那一份"存进配置目录，返回存到哪儿了。
//
// 干什么用：回滚失败时人靠它手工放回；平时它也是"上一次改了什么"的唯一凭据。
// 为什么存配置目录而不是 hosts 旁边：往 /etc 里塞一个文件是往用户的系统目录丢东西，
// 而且那个文件本身会被下一次解析读进文档，等于自己给自己造脏数据。
// 什么情况下会错：目录建不出来、磁盘满、权限不够——都如实报错，但**不拦下写入**：
// 写不写是主判断，备份是附属信息，混在一起会让人以为这一次没生效（那种误判比丢一份备份更贵）。
// 怎么回来：错误进 Receipt.Note，人看得见；备份目录里最老的会被删掉，只留最近 backupKeep 份。
func writeBackup(backupDir, hostsPath string, content []byte) (string, error) {
	if backupDir == "" {
		return "", nil // 调用方没给目录：不猜、不往 hosts 旁边丢文件，只报告"这份没存下"
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", errs.New(errs.CodeHostsWrite, map[string]any{"dir": backupDir}, "建不出备份目录").WithCause(err)
	}
	name := fmt.Sprintf(backupNameFmt, time.Now().Format("20060102-150405.000"))
	path := filepath.Join(backupDir, name)
	// 0o600：里面是整份 hosts，谁都能读等于把这台机器的内网解析表公开了。
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return "", errs.New(errs.CodeHostsWrite, map[string]any{"path": path, "hosts": hostsPath},
			"备份存不下").WithCause(err)
	}
	prune(backupDir)
	return path, nil
}

// prune 只删按上面那个前缀扫出来的、超出 backupKeep 的那几个最老的。
// 时间戳在名字里且格式定长，所以字典序就是时间序——这是当初把日期写成 20060102- 这个形状换来的。
func prune(dir string) {
	matches, err := filepath.Glob(filepath.Join(dir, "hosts-backup-*"))
	if err != nil || len(matches) <= backupKeep {
		return
	}
	sort.Strings(matches)
	for _, old := range matches[:len(matches)-backupKeep] {
		_ = os.Remove(old)
	}
}
