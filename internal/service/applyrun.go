package service

import (
	"context"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/hosts"
	"github.com/hlep/github-accel/internal/menu"
)

// applyItem 把这一条条目要的几个域名写成各自的最优地址。
//
// 干什么用：一次点击（或开机那一次自动应用）里这一条的写入，走完四个阶段：备份 → 写入 → 刷 DNS → 核对。
// 什么情况下会错、出错时人看得见什么：
// ① 手上没有可用地址就直接写 → 写进去的是空串，整条域名解析不动。所以先补一次检测，
//
//	一个地址也补不出来就如实失败；补回来一半就写那一半（分法在 changesFor 第 ② 条）。
//
// ② 一个域名挂在两条条目上（清单里现在实测一条 ubisoft.com）却照写 → 后写的悄悄盖掉前写的，
//
//	界面上两条都显示"已生效"。这种在动手之前就拒（GAC-HOSTS-007），并把抢它的两条一起带回去。
//
// ③ 数据本身不是域名形状（清单里那三条）→ 同样在动手之前拒（GAC-HOSTS-008），不静默跳过；
// ④ 刷 DNS 没成 ≠ 没写成：hosts 已经落地，所以任务不失败，但那一句要出现在状态栏里（GAC-PERM-002）。
//
// 怎么回来：四个阶段的推进都推事件（setPhase），界面那一行永远说"此刻走到哪一步"，不演假进度。
func (s *Service) applyItem(ctx context.Context, t *task) error {
	item, _, ok := s.cfg.Catalog.ByItem(t.itemID)
	if !ok {
		return newBadItem(t.itemID)
	}
	best := s.bestIPsOf(item.ID)
	if len(best) == 0 && t.pick == "" {
		// 点应用就要它到位：这一条还没检测过，先在这一次任务里补一轮。
		detectErr := s.detectItem(ctx, t)
		best = s.bestIPsOf(item.ID)
		if detectErr != nil && len(best) == 0 {
			return detectErr
		}
		// 检测只救回一部分域名：救回的那几个照写（changesFor 会把没救回的那个留在状态栏里）。
	}
	if t.pick != "" {
		// 界面上选了某一个候选：这条条目里几个域名都用它（契约里一行只有一个 picks[条目id]→IP 就是这个形状）。
		for _, d := range item.Domains {
			best[d] = t.pick
		}
	}
	changes, err := s.changesFor(item, best)
	if err != nil {
		return err
	}

	// 阶段一 backup：下面这一次调用里的第一件事就是存备份。
	// 为什么 backup 与 write 之间没有单独的观察点：备份、写临时、rename、补属主、重读校验全在
	// hosts.Engine.Apply 这一次调用里（第 8 节：碰这个文件的出口只有一个），
	// 所以这里只能在它启动前标 backup、在它返回后标 write——四个阶段一个不少，只是前两个共用一次调用。
	s.setPhase(t, contract.PhaseBackup)
	var rec hosts.Receipt
	if err := s.withWriteLock(func() error {
		var e error
		rec, e = s.cfg.Engine.Apply(ctx, changes)
		return e
	}); err != nil {
		return err
	}
	s.setPhase(t, contract.PhaseWrite)
	s.publishReceipt(t, rec)

	// 阶段三 flush：Linux 那份是空实现（glibc 每次现读，没有缓存要刷），Windows 那份刷 Dnscache。
	s.setPhase(t, contract.PhaseFlush)
	if err := s.cfg.Flusher.Flush(ctx); err != nil {
		s.noteSideError(err) // 写成是主判断，这一句只进状态栏，不把任务判死
	}

	// 阶段四 verify：服务自己再读一遍整份文件，按域名核对"这台机器现在到底按哪个地址解析"。
	// 引擎里已经核对过一次，这里再核一次不是为了双保险，而是因为中间隔了一次刷 DNS，
	// 而且界面要说的是"服务看到的"那一份——它对上的就是文件此刻的真实内容〔已定：D3 甲〕。
	s.setPhase(t, contract.PhaseVerify)
	doc, err := s.cfg.Engine.Read()
	if err != nil {
		return err
	}
	got := doc.Mappings()
	for _, c := range changes {
		if got[c.Domain] != c.IP {
			return newVerifyMismatch(c.Domain, c.IP, got[c.Domain])
		}
	}
	return nil
}

// restoreItem 撤掉这一条条目的托管：单项恢复与「恢复默认」共用这一个执行体，参数决定范围〔已定〕。
//
// 干什么用：把这一条写的那几行删掉，用户原来那一行按当初存下的原文放回。
// 什么情况下会错、出错时人看得见什么：界面上是两个入口，后端要是两套实现就一定会漂——
// 所以这里跟 applyItem 走同一个引擎、同一把锁、同样的四个阶段，只是要写的地址是空串（空＝撤托管）。
// ① 自己一行都没写过却照写一次：「恢复默认」把清单里每一条都排一行，
//
//	为用户根本没写过的条目也要备份一次、重写一次、核对一次——那是拿用户的 hosts 文件凑行数。
//	所以先看文件里有没有自己那几行，没有就地收工（任务照样"已恢复"，因为它要撤的东西本来就不存在）。
//
// 怎么回来：这一条的判据只有 hasManagedLine 那一个函数，收尾核对用的也是它。
func (s *Service) restoreItem(ctx context.Context, t *task) error {
	item, _, ok := s.cfg.Catalog.ByItem(t.itemID)
	if !ok {
		return newBadItem(t.itemID)
	}
	doc, err := s.cfg.Engine.Read()
	if err != nil {
		return err
	}
	if !hasManagedLine(doc, item.ID) {
		return nil
	}
	// 撤托管也要先认"这个域名是不是两条条目抢着要"：照撤会连带把另一条那行一起撤掉——
	// 那正是"私自改变其他功能"。所以这道前置检查跟 changesFor 用同一个。
	if err := s.refuseUnfit(item); err != nil {
		return err
	}

	s.setPhase(t, contract.PhaseBackup)
	var rec hosts.Receipt
	if err := s.withWriteLock(func() error {
		var e error
		rec, e = s.cfg.Engine.Restore(ctx, item.Domains)
		return e
	}); err != nil {
		return err
	}
	s.setPhase(t, contract.PhaseWrite)
	s.publishReceipt(t, rec)

	s.setPhase(t, contract.PhaseFlush)
	if err := s.cfg.Flusher.Flush(ctx); err != nil {
		s.noteSideError(err)
	}

	s.setPhase(t, contract.PhaseVerify)
	doc, err = s.cfg.Engine.Read()
	if err != nil {
		return err
	}
	// 这一条条目的托管行还在文件里，就是没撤干净。别的条目自己的行不算——那是"不许私自改变其他功能"。
	if hasManagedLine(doc, item.ID) {
		return newVerifyLeft(item.ID)
	}
	// 恢复成功不代表这一条的检测白做了：最优地址留着，界面上那一行回到"已检测、未应用"那一档。
	return nil
}

// hasManagedLine 问文件里有没有这一条条目写下的行。
// 它是"这一条到底动过没有"唯一的判据：动手之前先问一次（省掉整次写入），收尾再问一次（核对撤净没有）。
func hasManagedLine(doc *hosts.Doc, itemID string) bool {
	if doc == nil {
		return false
	}
	for _, l := range doc.Lines() {
		if l.Managed && l.ItemID == itemID {
			return true
		}
	}
	return false
}

// changesFor 把"这条条目该指到哪儿"翻成引擎要的改动，并在动手之前拦下两类不合格数据。
//
// 干什么用：一条条目里几个域名各有自己的最优地址，这里按域名各出一条改动。
// 什么情况下会错、出错时人看得见什么：
// ① 一个地址都没有还照写 → 写进去的是空串，整条域名解析不动，所以当场拒（GAC-DETECT-001）；
// ② 有几个域名这一轮没检出地址就整条不写 → 本机实测到的正是这种（gist.github.com 不通、
//
//	github.com 与 api.github.com 通），用户点一次应用结果什么都没变。
//	所以有地址的照写，没地址的那个不动——它继续按这台机器自己的解析走，不算"私自改变其他功能"，
//	但那一句留在状态栏里：这一条只加速了一半这件事不该悄悄过去。
//
// 怎么回来：两类拦截都只在这一个函数里，"有没有地址"的分法跟快照那一侧（matchesAll）是同一句判据。
func (s *Service) changesFor(item menu.Item, best map[string]string) ([]hosts.Change, error) {
	if err := s.refuseUnfit(item); err != nil {
		return nil, err
	}
	out := make([]hosts.Change, 0, len(item.Domains))
	var missing []string
	for _, d := range item.Domains {
		ip := best[d]
		if ip == "" {
			missing = append(missing, d)
			continue
		}
		out = append(out, hosts.Change{Domain: d, IP: ip, ItemID: item.ID})
	}
	if len(out) == 0 {
		return nil, newNoAddress(item.ID, missing[0])
	}
	if len(missing) > 0 {
		s.noteSideError(newNoAddress(item.ID, missing[0]))
	}
	return out, nil
}

// refuseUnfit 拦下清单里那两类数据项：一个域名被两条条目抢、以及根本不是域名形状的值。
// 两类都只在写入这一刻拒，数据本身一个字不改〔已定：D4 甲、D5 甲〕。
func (s *Service) refuseUnfit(item menu.Item) error {
	for _, d := range item.Domains {
		s.mu.Lock()
		owners := append([]string(nil), s.conflicts[d]...)
		s.mu.Unlock()
		if len(owners) > 0 {
			return newConflict(d, owners)
		}
	}
	s.mu.Lock()
	values := append([]string(nil), s.unsupported[item.ID]...)
	s.mu.Unlock()
	if len(values) > 0 {
		return newBadValue(item.ID, values)
	}
	return nil
}

// bestIPsOf 取出这一条最近一轮的最优地址（拷贝，调用方改不动内存里那份）。
func (s *Service) bestIPsOf(itemID string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return copyIPs(s.itemStateOf(itemID).bestIPs)
}

// publishReceipt 把回执上那几个数推给界面：动了几个域名、备份在哪个路径、以及"写成了但该知道"的那几条。
//
// 为什么那些附属信息走 payload 而不是 error：见 hosts.Caveat 那段说明——
// 「这次生效了没有」是主判断，混进 error 会让人以为没生效。
func (s *Service) publishReceipt(t *task, rec hosts.Receipt) {
	payload := map[string]any{
		"taskId":       t.id,
		"batchId":      t.batch,
		"itemId":       t.itemID,
		"kind":         t.kind,
		"changedLines": rec.ChangedLines,
		"path":         rec.Path,
	}
	if rec.BackupPath != "" {
		payload["backupPath"] = rec.BackupPath
	}
	if len(rec.Caveats) > 0 {
		payload["caveats"] = rec.Caveats
		s.noteCaveat(rec.Caveats[0])
	}
	s.publish(contract.EventQueueProgress, payload)
}

// noteSideError 把"这件事没成，但它不是主判断"记进状态栏。今天只有一件：刷 DNS。
func (s *Service) noteSideError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noteStateErr(err)
}

// 这一节是写入链路上那几个码的构造处。码本身的家在 errs/codes.go，这里只造"码＋参数"这一对，
// 整句人话在前端语言文件那一个家〔已定：技术方案第 15 节 D6 乙〕。
// 为什么不散在调用点：同一个码在两个地方各拼一次参数，早晚会拼出两种键名，界面那一句话就凑不出来。

// newNoAddress 是"要点应用，手上却没有这一条域名该写的地址"。
// 它归 GAC-DETECT-001（候选为空）而不是 GAC-HOSTS 那一族：拦住写入的原因是没地址，不是文件写不动。
func newNoAddress(itemID, domain string) error {
	return errs.New(errs.CodeDetectEmpty, map[string]any{"itemId": itemID, "domain": domain},
		"这条域名还没有可用地址，先检测")
}

// newConflict 是一个域名挂在两条条目上（现在实测一条 ubisoft.com）。拒绝写入，并把抢它的两条一起带回去〔已定：D5 甲〕。
func newConflict(domain string, owners []string) error {
	return errs.New(errs.CodeHostsConflict, map[string]any{"domain": domain, "items": owners},
		"%s 同时挂在 %d 条条目上，不敢替人选一条", domain, len(owners))
}

// newBadValue 是这一条条目里有根本不是域名形状的数据项（清单里那三条）。照单登记、写入这一刻拒〔已定：D4 甲〕。
func newBadValue(itemID string, values []string) error {
	return errs.New(errs.CodeHostsBadValue, map[string]any{"itemId": itemID, "values": values},
		"这一条里有 %d 个值不是域名形状", len(values))
}

// newVerifyMismatch 是"服务自己重读整份文件，这个域名现在并不指向刚写的那个地址"。
// 码沿用 GAC-HOSTS-004（写完重读对不上）而不是新造一个：界面要说的是同一句话，差别只在参数。
func newVerifyMismatch(domain, want, got string) error {
	return errs.New(errs.CodeHostsVerify, map[string]any{"domain": domain, "want": want, "got": got},
		"%s 现在指向 %s，不是刚写下去的 %s", domain, got, want)
}

// newVerifyLeft 是"恢复之后这一条的托管行还留在文件里"。同样沿用 004。
func newVerifyLeft(itemID string) error {
	return errs.New(errs.CodeHostsVerify, map[string]any{"itemId": itemID},
		"条目 %s 的托管行没撤掉", itemID)
}

// newBadItem 是界面给的条目 id 在清单里找不到（GAC-TASK-004）。
func newBadItem(itemID string) error {
	return errs.New(errs.CodeTaskBadItem, map[string]any{"itemId": itemID}, "清单里没有这条条目")
}
