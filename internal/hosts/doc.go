// Package hosts 是全项目唯一被允许碰 hosts 文件的包。
//
// 干什么用：把 hosts 读成一份"每行都还在"的文档，合并出期望内容（纯函数，不碰磁盘），
// 再走五步：备份 → 写临时文件 → rename 盖过去 → 补回原属主与权限 → 重读逐条校验；对不上就用备份写回去。
// 什么情况下会错、出错时人看得见什么：解析丢行＝用户自己加的条目被我们"应用"时顺手删了；
// Linux 上 rename 会换 inode，属主和权限跟着变，补不回来下次普通用户就改不动它；
// 写坏了整台机器的解析，最坏连 GitHub 都上不去——所以校验不过必须回滚，界面那条状态回落到写之前的值。
// 怎么回来：凡是这一包改过的行都带标记（见 marker 常量），还原靠标记逐行撤，不靠记忆；
// 回滚失败（最坏那一档）会回 GAC-HOSTS-005 并把备份路径一起给出去，人还能手工放回原文件。
package hosts

import (
	"fmt"
	"strings"
)

// 标记只有一个形状定义处。界面与文档都不许自己拼这个串。
const (
	markerPrefix = "gac:"                // 我们写的行：`<ip> <domain> # gac:<条目id> prev:<应用前的地址或->`
	offPrefix    = markerPrefix + "off:" // 被我们停用的用户原行（整行以 # 起头，原文跟在后面）
	sepPrev      = " prev:"
	fieldSep     = " # "
	commentHead  = "# " // 停用行必须真的是一条注释，否则写出去就成了一行看不懂的内容
)

// Line 是文件里的一行。Raw 永远保存原文——渲染时未被改动的行原样写回，这是"不丢用户内容"的底。
type Line struct {
	Raw string

	// IP 与 Names 只有真条目行才有值；注释行、空行两者为零。
	IP    string
	Names []string

	// Comment 是条目行后面那段行尾注释（不含我们那段标记）。
	Comment string

	Managed  bool   // 这一行是我们写的
	ItemID   string // 这一行属于哪条条目
	PrevIP   string // 应用之前这个域名指向哪儿；空串表示"应用前它没有行"
	Disabled bool   // 这一行是被我们停用的用户原行

	// orig 只有停用行才有值：那条用户原行的一字原文。还原时放回它，不重新拼——
	// 重新拼会把用户的空格、大小写、行尾注释改掉。
	orig string
}

// Doc 是一份 hosts 文档。它是"现读"的结果，不是缓存：GetStatus 每次都重新读它再算状态。
type Doc struct {
	lines []Line
}

// Lines 给出一份拷贝，调用方改不动文档本体。
func (d *Doc) Lines() []Line {
	out := make([]Line, len(d.lines))
	for i, l := range d.lines {
		out[i] = l
		out[i].Names = append([]string(nil), l.Names...)
	}
	return out
}

// Render 把文档拼回文本内容。行与行之间一律用一个换行，末尾保留一个换行。
func (d *Doc) Render() string {
	if len(d.lines) == 0 {
		return ""
	}
	var b strings.Builder
	for _, l := range d.lines {
		b.WriteString(l.Raw)
		b.WriteByte('\n')
	}
	return b.String()
}

// Mappings 给出"此刻这台机器上每个域名指向哪个地址"。
// 取法就一条：按文件顺序，跳过被停用的行，第一个带着这个域名的真条目行算——这跟 glibc nss_files 的做法一致。
// 我们写的行和用户写的行在这里不加区别：位置才是生效的根据，"这行是谁写的"不是。
func (d *Doc) Mappings() map[string]string {
	out := map[string]string{}
	for _, l := range d.lines {
		if l.Disabled || l.IP == "" {
			continue
		}
		for _, n := range l.Names {
			if _, seen := out[n]; !seen {
				out[n] = l.IP
			}
		}
	}
	return out
}

// Managed 只给出我们写的那些行，"这条条目现在是不是我们管的、prev 是什么"全靠它现算。
func (d *Doc) Managed() []Line {
	var out []Line
	for _, l := range d.lines {
		if l.Managed {
			cp := l
			cp.Names = append([]string(nil), l.Names...)
			out = append(out, cp)
		}
	}
	return out
}

// ItemIDToIPs 按条目 id 归集我们写进去的地址，快照里 appliedIp 由它给。
func (d *Doc) ItemIDToIPs() map[string]string {
	out := map[string]string{}
	for _, l := range d.Managed() {
		if len(l.Names) == 1 {
			out[l.ItemID] = l.IP
		}
	}
	return out
}

// Parse 把文本读成文档。它不报错也不丢行：看不懂的行原样留着（Raw 还在），
// 因为把一行注释掉的判断该由 Plan 做，而不是由"这行我读不懂"来做。
func Parse(text string) *Doc {
	d := &Doc{}
	if text == "" {
		return d
	}
	for _, raw := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		d.lines = append(d.lines, parseLine(raw))
	}
	return d
}

func parseLine(raw string) Line {
	l := Line{Raw: raw}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return l
	}

	// 注释行里只有我们那一种带标记的形状要单独认，其余一律当普通注释原样留着。
	if strings.HasPrefix(trimmed, "#") {
		code := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		if !strings.HasPrefix(code, offPrefix) {
			return l
		}
		rest := strings.TrimPrefix(code, offPrefix)
		id, tail, found := strings.Cut(rest, " ")
		if !found || strings.TrimSpace(tail) == "" {
			return l // 标记后面没有原文：这一行不是我们停用的用户行，当普通注释留着
		}
		l.Disabled = true
		l.ItemID = id
		// tail 不修剪：用户原来那行开头的空格与制表符也要原样回得去。
		l.orig = tail
		return l
	}

	// 条目行：先剥掉行尾注释，剩下的按空白切成 "IP 名字..."。
	body, comment := trimmed, ""
	if i := strings.Index(trimmed, "#"); i >= 0 {
		body = strings.TrimSpace(trimmed[:i])
		comment = strings.TrimSpace(trimmed[i+1:])
	}
	fields := strings.Fields(body)
	if len(fields) < 2 {
		return l // 读不懂的行原样留着，不猜
	}
	l.IP = fields[0]
	l.Names = fields[1:]
	l.Comment = comment

	// 行尾注释里有我们的标记，这一行就是我们写的。
	if i := strings.Index(comment, markerPrefix); i >= 0 {
		body := comment[i+len(markerPrefix):] // 形如 `<条目id> prev:<地址或->`
		head, tail, _ := strings.Cut(body, sepPrev)
		l.Managed = true
		l.ItemID = head
		l.PrevIP = normalizePrev(tail)
	}
	return l
}

// normalizePrev 把标记里的 prev 段收成干净值：`-` 表示应用前这域名没有行。
func normalizePrev(tail string) string {
	tail = strings.TrimSpace(tail)
	if tail == "-" || tail == "" {
		return ""
	}
	// prev 段后面可能还跟着别的注释文字，取到第一个空白为止。
	head, _, _ := strings.Cut(tail, " ")
	return head
}

// markerFor 拼出我们那行的行尾标记。
func markerFor(itemID, prevIP string) string {
	prev := "-"
	if prevIP != "" {
		prev = prevIP
	}
	return fmt.Sprintf("%s%s%s%s", markerPrefix, itemID, sepPrev, prev)
}

// managedLine 造一行"我们写的"内容，并同步把解析字段填好，省得 Plan 之后再解一次。
func managedLine(ip, domain, itemID, prevIP string) Line {
	body := ip + " " + domain
	mark := markerFor(itemID, prevIP)
	raw := body + fieldSep + mark
	return Line{
		Raw: raw, IP: ip, Names: []string{domain}, Comment: mark,
		Managed: true, ItemID: itemID, PrevIP: prevIP,
	}
}

// disabledLine 把一条用户原行整行注释掉，原文一字不动地跟在标记后面。
func disabledLine(orig Line, itemID string) Line {
	return Line{
		Raw:      commentHead + offPrefix + itemID + " " + orig.Raw,
		Disabled: true,
		ItemID:   itemID,
		orig:     orig.Raw, // 原文一字不改，连开头的空格和行尾注释都带着
	}
}
