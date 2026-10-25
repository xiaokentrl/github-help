package hosts

import (
	"strings"
	"testing"
)

// 这一份文件里"每一行都还在"是 hosts 包的底，所以下面第一组用例逐条钉它。

func TestParseKeepsEveryLine(t *testing.T) {
	// 故意掺进各种读不懂的行：只有 IP 的、只有一段文字的、带制表符的、行尾注释的、我们那两种标记的。
	const text = "# /etc/hosts: local version\n" +
		"#\n" +
		"127.0.0.1\tlocalhost\n" +
		"127.0.1.1 myhost myhost.local # 我自己加的\n" +
		"::1\t\tlocalhost ip6-localhost ip6-loopback\n" +
		"\n" +
		"20.205.243.166 github.com\n" +
		"not-a-line-with-ip-only\n" +
		"!\n" +
		"140.82.112.22 raw.githubusercontent.com # gac:assets-prev:1.2.3.4\n" +
		"# gac:off:assets 5.6.7.8 raw.githubusercontent.com # 原来这行有注释\n"
	doc := Parse(text)
	if got := strings.Count(text, "\n"); len(doc.Lines()) != got {
		t.Fatalf("行数对不上：文件 %d 行，解析成 %d 行", got, len(doc.Lines()))
	}
	if doc.Render() != text {
		t.Fatalf("渲染回来的内容跟原文不一致：\n got=%q\nwant=%q", doc.Render(), text)
	}
}

func TestParseEmptyAndNoTrailingNewline(t *testing.T) {
	if got := Parse("").Render(); got != "" {
		t.Fatalf("空文档不该长出内容，got %q", got)
	}
	// 有人手工编辑时会把最后一个换行删掉；这一条不该变成两行黏在一起。
	doc := Parse("127.0.0.1 a.test")
	if len(doc.Lines()) != 1 {
		t.Fatalf("got %d 行", len(doc.Lines()))
	}
	if got := doc.Render(); got != "127.0.0.1 a.test\n" {
		t.Fatalf("补不回末尾换行，got %q", got)
	}
}

func TestMarkerRoundTrip(t *testing.T) {
	doc := Parse("140.82.112.22 github.com # gac:core-github prev:20.205.243.166\n")
	l := doc.Lines()[0]
	if !l.Managed || l.Disabled {
		t.Fatalf("这一行该认成我们写的：Managed=%v Disabled=%v", l.Managed, l.Disabled)
	}
	if l.ItemID != "core-github" {
		t.Fatalf("条目 id 解错了：%q", l.ItemID)
	}
	if l.PrevIP != "20.205.243.166" {
		t.Fatalf("prev 解错了：%q", l.PrevIP)
	}
	if l.IP != "140.82.112.22" || strings.Join(l.Names, ",") != "github.com" {
		t.Fatalf("地址或名字解错了：%q / %v", l.IP, l.Names)
	}

	// prev:- 表示"应用前这个域名压根没有行"，解出来必须是空串而不是 "-"。
	doc2 := Parse("1.2.3.4 b.test # gac:x prev:-\n")
	if got := doc2.Lines()[0].PrevIP; got != "" {
		t.Fatalf(`prev:- 该解成空串，got %q`, got)
	}

	off := Parse("# gac:off:core-github 5.6.7.8 github.com # 我原来的注释\n").Lines()[0]
	if !off.Disabled || off.ItemID != "core-github" {
		t.Fatalf("停用的行没认出来：Disabled=%v ItemID=%q", off.Disabled, off.ItemID)
	}
	if got := off.Original(); got != "5.6.7.8 github.com # 我原来的注释" {
		t.Fatalf("原文没原样留着：%q", got)
	}
}

func TestCommentWithoutOriginalStaysPlainComment(t *testing.T) {
	// 标记后面没有原文（人在外面删过或写错过）：这一行不能当"停用的用户行"，
	// 否则还原时会往文件里放回一条空行。
	l := Parse("# gac:off:core-github\n").Lines()[0]
	if l.Disabled {
		t.Fatal("这条残缺标记不该认成停用行")
	}
	if got := Parse("# gac:off:core-github\n").Render(); got != "# gac:off:core-github\n" {
		t.Fatalf("渲染该原样留着，got %q", got)
	}
}

func TestMappingsTakeFirstEffectiveLine(t *testing.T) {
	doc := Parse(strings.Join([]string{
		"1.1.1.1 dup.test",   // 生效的是这一条：文件里在前
		"2.2.2.2 dup.test",   // 在后面，不生效
		"# 9.9.9.9 off.test", // 注释行不算
		"# 8.8.8.8 off.test", // 用户自己的注释也不算
		"3.3.3.3 mine.test # gac:x prev:-",
	}, "\n") + "\n")
	m := doc.Mappings()
	if m["dup.test"] != "1.1.1.1" {
		t.Fatalf("该取第一条生效行，got %q", m["dup.test"])
	}
	if _, ok := m["off.test"]; ok {
		t.Fatalf("注释行不该进映射表：%v", m)
	}
	if m["mine.test"] != "3.3.3.3" {
		t.Fatalf("我们写的行也要生效（这才是这台机器现在的真实值），got %q", m["mine.test"])
	}
}

func TestDisabledLineIsCommentedOut(t *testing.T) {
	orig := parseLine("127.0.0.1  a.test # 手写")
	off := disabledLine(orig, "x1")
	if !strings.HasPrefix(off.Raw, "# ") {
		t.Fatalf("停用行必须以 # 起头，否则写出去是一行看不懂的内容：%q", off.Raw)
	}
	back := Parse(off.Raw + "\n").Lines()[0]
	if !back.Disabled || back.Original() != "127.0.0.1  a.test # 手写" {
		t.Fatalf("停用行解不回来：Disabled=%v 原文=%q", back.Disabled, back.Original())
	}
}

func TestLinesCopyDoesNotLeak(t *testing.T) {
	doc := Parse("127.0.0.1 a.test b.test\n")
	got := doc.Lines()
	got[0].Names[0] = "改了也不许影响文档"
	if doc.Lines()[0].Names[0] != "a.test" {
		t.Fatal("Lines() 给了内部切片的引用，调用方能改到文档本体")
	}
}

func TestCountDiffLines(t *testing.T) {
	for _, c := range []struct {
		name          string
		before, after string
		want          int
	}{
		{"没动", "a\nb\n", "a\nb\n", 0},
		{"加一行", "a\nb\n", "a\nb\nc\n", 1},
		{"改一行", "a\nb\n", "a\nB\n", 2}, // 删掉 b、加上 B 各算一次
		{"重复行只算多出来的那一份", "a\n", "a\na\n", 1},
	} {
		if got := countDiffLines(c.before, c.after); got != c.want {
			t.Errorf("%s：got %d want %d", c.name, got, c.want)
		}
	}
}
