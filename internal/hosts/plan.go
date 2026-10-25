package hosts

import (
	"net"
	"sort"
	"strings"

	"github.com/hlep/github-accel/internal/errs"
	"github.com/hlep/github-accel/internal/menu"
)

// Change 是"这一个域名该指到哪个地址"。IP 为空串表示撤掉我们的托管、恢复到写之前那样。
//
// 一次调用里同一域名只许出现一次：出现两次说明上层把两条条目算到同一个域名上了，
// 那正是门禁 N1 抓的那种重复（现在实测有一条 ubisoft.com），必须由这里拒掉而不是"后写的悄悄盖掉前写的"。
type Change struct {
	Domain string
	IP     string
	ItemID string
}

// Original 还给被我们停用那一行的原文。还原靠它，不靠记忆也不靠数据库。
func (l Line) Original() string {
	if !l.Disabled {
		return l.Raw
	}
	return l.orig
}

// Plan 把一批改动并成期望的文件内容。它是纯函数：不碰磁盘、不读时间，
// 所以"同一批改动每次写出的内容都一样"这条可以在 go test 里逐例钉住，不需要真文件。
func Plan(doc *Doc, changes []Change) (string, error) {
	out, err := plan(doc, changes)
	if err != nil {
		return "", err
	}
	return out.Render(), nil
}

// plan 是 Plan 与 Apply 共用的那一步，只返回新文档。
// 校验那一步不按"哪几行被动过"比，而是按域名比（见 engine.go 的 verify）——
// 那种问法才对得上"这台机器现在到底按哪个地址解析"，也不必把中间行号一路传下去。
func plan(doc *Doc, changes []Change) (*Doc, error) {
	// 先按域名把改动排一遍：改动顺序不该影响写出来的内容，否则同一份 hosts 每轮 diff 都在动。
	sorted := append([]Change(nil), changes...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Domain < sorted[j].Domain })

	seen := map[string]bool{}
	for _, c := range sorted {
		if seen[c.Domain] {
			return nil, errs.New(errs.CodeHostsConflict, map[string]any{
				"domain": c.Domain,
			}, "这一批改动里 %s 出现了两次，拒绝写入", c.Domain)
		}
		seen[c.Domain] = true
		if !menu.ValidDomain(c.Domain) {
			return nil, errs.New(errs.CodeHostsBadValue, map[string]any{
				"domain": c.Domain, "itemId": c.ItemID,
			}, "%s 不是域名形状，写不进 hosts", c.Domain)
		}
		if c.IP != "" && net.ParseIP(c.IP) == nil {
			return nil, errs.New(errs.CodeHostsBadValue, map[string]any{
				"domain": c.Domain, "ip": c.IP,
			}, "%s 不是地址形状，写不进 hosts", c.IP)
		}
	}

	lines := doc.Lines()
	for _, c := range sorted {
		next, err := applyOne(lines, c)
		if err != nil {
			return nil, err
		}
		lines = next
	}
	return &Doc{lines: lines}, nil
}

// applyOne 处理一条改动。行号在这里找，找不到就是"这域名我们没管过"。
func applyOne(lines []Line, c Change) ([]Line, error) {
	// 同一个域名挂着两行我们的标记，只可能是有人在外面复制过那一行。
	// 改一行留一行＝文件里同时存在两个地址，谁生效看顺序，这种事不该由"应用"悄悄决定。
	if n := countManaged(lines, c.Domain); n > 1 {
		return nil, errs.New(errs.CodeHostsConflict, map[string]any{
			"domain": c.Domain, "count": n,
		}, "%s 有 %d 行都带着我们的标记，先手工留一行再应用", c.Domain, n)
	}
	mine := indexOfManaged(lines, c.Domain)
	if c.IP != "" {
		return applySet(lines, c, mine)
	}
	return applyRestore(lines, c, mine)
}

func applySet(lines []Line, c Change, mine int) ([]Line, error) {
	if mine >= 0 {
		cur := lines[mine]
		// 我们的行上被人手工又加了一个名字：重写这一行会顺手把那个名字删掉，
		// 那是"私自改变别的功能"，所以和"用户原行里挤了两个名"同一档处理——拒写并给出原文。
		if len(cur.Names) > 1 {
			return nil, sharedLineErr(c, cur.Raw, cur.Names)
		}
		if cur.ItemID != c.ItemID {
			return nil, errs.New(errs.CodeHostsConflict, map[string]any{
				"domain": c.Domain, "itemId": c.ItemID, "heldBy": cur.ItemID,
			}, "%s 已经是条目 %s 写下的行，%s 不能抢", c.Domain, cur.ItemID, c.ItemID)
		}
		if cur.IP == c.IP {
			return lines, nil // 已经是这个地址了：幂等，一个字不改
		}
		lines[mine] = managedLine(c.IP, c.Domain, cur.ItemID, cur.PrevIP)
		return lines, nil
	}

	// 我们没写过这一行，那就看用户（或别的程序）有没有。
	owner, shared := indexOfUnmanaged(lines, c.Domain)
	if shared {
		return nil, sharedLineErr(c, lines[owner].Raw, lines[owner].Names)
	}

	// 应用前的地址：用户（或别的程序）那一行的地址，或者是"没有这一行"（记成 -，见 markerFor）。
	prev := ""
	if owner >= 0 {
		prev = lines[owner].IP
	}
	newLine := managedLine(c.IP, c.Domain, c.ItemID, prev)
	if owner >= 0 {
		// 停用整行会连带那一行里别的名一起停掉，所以能走到这里说明那一行只有这一个域名（shared 已在上面拒了）。
		out := make([]Line, 0, len(lines)+1)
		out = append(out, lines[:owner]...)
		out = append(out, disabledLine(lines[owner], c.ItemID), newLine)
		out = append(out, lines[owner+1:]...)
		return out, nil
	}

	return append(append([]Line(nil), lines...), newLine), nil
}

// sharedLineErr 是"这一行还带着别的名"那一条拒绝。参数里带上整行原文和别的名，
// 因为界面要给人看到"到底哪一行、还挤着谁"，只有一句"格式不对"等于让人猜。
func sharedLineErr(c Change, raw string, names []string) error {
	var others []string
	for _, n := range names {
		if !strings.EqualFold(n, c.Domain) {
			others = append(others, n)
		}
	}
	if len(others) == 0 {
		// 只剩一种可能：同一个名在这一行里写了两遍。它同样叫"这一行我们动不了"。
		others = []string{c.Domain + "（这一行里写了两遍）"}
	}
	return errs.New(errs.CodeHostsSharedLine, map[string]any{
		"domain": c.Domain, "itemId": c.ItemID, "line": raw, "otherNames": others,
	}, "%s 和 %s 挤在同一行（%s），停用整行会连带把它们一起停掉，所以这条拒写",
		c.Domain, strings.Join(others, "、"), raw)
}

func applyRestore(lines []Line, c Change, mine int) ([]Line, error) {
	var prev string
	var kept []Line
	if mine >= 0 {
		// 撤这一行会把它带着的别的名一起撤掉：跟"要写"那一档同一个理由，拒。
		if len(lines[mine].Names) > 1 {
			return nil, sharedLineErr(c, lines[mine].Raw, lines[mine].Names)
		}
		prev = lines[mine].PrevIP
		// 把这一行摘掉，其余行原样留着。
		kept = make([]Line, 0, len(lines)-1)
		for i, l := range lines {
			if i != mine {
				kept = append(kept, l)
			}
		}
	} else {
		kept = append([]Line(nil), lines...)
	}

	// 我们当初把哪一行注释掉了，就把它放回原处、原文一字不改。
	off := indexOfDisabled(kept, c.Domain, c.ItemID)
	if off >= 0 {
		back := parseLine(kept[off].Original())
		out := append([]Line(nil), kept[:off]...)
		out = append(out, back)
		out = append(out, kept[off+1:]...)
		return out, nil
	}

	// 原行不在了（被人在外面删了），但标记里存着应用前的地址：按那个地址补一行普通条目。
	// 补的内容里没有我们的标记——恢复到那一刻起，这一行就是用户自己的了。
	if mine >= 0 && prev != "" {
		out := append(kept, Line{Raw: prev + " " + c.Domain, IP: prev, Names: []string{c.Domain}})
		return out, nil
	}
	if mine < 0 {
		return lines, nil // 我们没写过、也没有停用的行：这一条本来就没动过
	}
	return kept, nil
}

// indexOfManaged 找我们为该域名写的那一行。返回 -1 表示没有。
func indexOfManaged(lines []Line, domain string) int {
	for i, l := range lines {
		if l.Managed && hasName(l.Names, domain) {
			return i
		}
	}
	return -1
}

// countManaged 数这个域名挂着几行我们的标记。与上面那一个共用同一句判断，两处不各写一遍规则。
func countManaged(lines []Line, domain string) int {
	n := 0
	for _, l := range lines {
		if l.Managed && hasName(l.Names, domain) {
			n++
		}
	}
	return n
}

// indexOfUnmanaged 找别人写的、带着这个域名的那一行。
// shared 为真表示那一行还带着别的名字——这一档要拒写，见 CodeHostsSharedLine 那段说明。
func indexOfUnmanaged(lines []Line, domain string) (int, bool) {
	for i, l := range lines {
		if l.Managed || l.Disabled || l.IP == "" {
			continue
		}
		if !hasName(l.Names, domain) {
			continue
		}
		return i, len(l.Names) > 1
	}
	return -1, false
}

// indexOfDisabled 找被我们停用的、原本带着这个域名的那一行。
// 标记里只有条目 id，域名在那段原文里，所以这里要解开原文再比一次。
func indexOfDisabled(lines []Line, domain, itemID string) int {
	for i, l := range lines {
		if !l.Disabled {
			continue
		}
		if itemID != "" && l.ItemID != itemID {
			continue
		}
		if hasName(parseLine(l.Original()).Names, domain) {
			return i
		}
	}
	return -1
}

func hasName(names []string, want string) bool {
	for _, n := range names {
		if strings.EqualFold(n, want) {
			return true
		}
	}
	return false
}
