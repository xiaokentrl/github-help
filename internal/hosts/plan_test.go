package hosts

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hlep/github-accel/internal/errs"
)

const header = "# 这是用户自己的说明行\n127.0.0.1\tlocalhost\n::1 localhost\n"

// apply 是这些用例共用的那一步：把改动并成内容，同时检查"解析读回来对得上"这条硬不变量。
// 为什么要每例都查：Plan 写的是文本，这台机器将来读的是文本——两边对不上就是"写成功了但没生效"，
// 而那正是五步校验要拦的事；把它提到纯函数这一层来查，一次工具运行就能覆盖全部形状。
func apply(t *testing.T, text string, changes ...Change) string {
	t.Helper()
	out, err := Plan(Parse(text), changes)
	if err != nil {
		t.Fatalf("这批改动本该写得进去，却拒了：%v", err)
	}
	doc := Parse(out)
	want := map[string]string{}
	for _, c := range changes {
		want[c.Domain] = c.IP
	}
	for d, ip := range want {
		if ip == "" {
			continue
		}
		if got := doc.Mappings()[d]; got != ip {
			t.Fatalf("%s 写完之后这台机器读到的还是 %q，不是 %q\n内容=%q", d, got, ip, out)
		}
	}
	return out
}

func TestPlanAppendsWhenDomainHasNoLine(t *testing.T) {
	out := apply(t, header, Change{Domain: "github.com", IP: "140.82.112.22", ItemID: "core-github"})
	want := header + "140.82.112.22 github.com # gac:core-github prev:-\n"
	if out != want {
		t.Fatalf("加行位置或标记形状不对：\n got=%q\nwant=%q", out, want)
	}
	l := Parse(out).Lines()[len(Parse(header).Lines())]
	if l.PrevIP != "" || !l.Managed {
		t.Fatalf("应用前没有这一行，prev 该是空：PrevIP=%q Managed=%v", l.PrevIP, l.Managed)
	}
}

func TestPlanDisablesUserLineAndRecordsPrev(t *testing.T) {
	text := header + "20.205.243.166 github.com\n" + "# 后面还有用户的说明\n"
	out := apply(t, text, Change{Domain: "github.com", IP: "140.82.112.22", ItemID: "core-github"})

	// 停用行必须紧挨在原位置，我们的行跟在它后面：位置不变才谈得上"没替用户重排"。
	if !strings.Contains(out, header+"# gac:off:core-github 20.205.243.166 github.com\n140.82.112.22 github.com # gac:core-github prev:20.205.243.166\n") {
		t.Fatalf("原行没在原位置被停用，或 prev 没记进去：\n%q", out)
	}
	if !strings.HasSuffix(out, "# 后面还有用户的说明\n") {
		t.Fatalf("原行之后的内容被挪动了：\n%q", out)
	}
}

func TestPlanIsIdempotent(t *testing.T) {
	changes := []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "core-github"}}
	first := apply(t, header, changes...)
	second := apply(t, first, changes...)
	if first != second {
		t.Fatalf("同样一批改动第二次写出了别的内容：\n 第一次=%q\n 第二次=%q", first, second)
	}
	if n := countDiffLines(first, second); n != 0 {
		t.Fatalf("幂等那一条要求动 0 行，实际动了 %d 行", n)
	}
}

func TestPlanIgnoresChangeOrder(t *testing.T) {
	// 一批两条：顺序换了内容必须一模一样，否则每次应用完 diff 里都在动，人看不出真改了什么。
	a := []Change{
		{Domain: "github.com", IP: "140.82.112.22", ItemID: "core-github"},
		{Domain: "raw.githubusercontent.com", IP: "185.199.108.133", ItemID: "assets"},
	}
	b := []Change{a[1], a[0]}
	if got, want := apply(t, header, b...), apply(t, header, a...); got != want {
		t.Fatalf("改动顺序影响了两字的内容：\n got=%q\nwant=%q", got, want)
	}
}

func TestPlanTakesOnlyTheFirstEffectiveLine(t *testing.T) {
	// 用户自己就有两条同一域名（第一条生效）。我们的行要停在第一条后面，prev 记第一条的地址。
	text := "1.1.1.1 github.com\n2.2.2.2 github.com\n"
	out := apply(t, text, Change{Domain: "github.com", IP: "140.82.112.22", ItemID: "core-github"})
	if !strings.Contains(out, "# gac:off:core-github 1.1.1.1 github.com\n140.82.112.22 github.com # gac:core-github prev:1.1.1.1\n") {
		t.Fatalf("prev 该取第一条生效行的地址：\n%q", out)
	}
	if !strings.Contains(out, "2.2.2.2 github.com\n") {
		t.Fatalf("第二条没生效的用户行也被我们停用了（那是私自改动）：\n%q", out)
	}
	if got := Parse(out).Mappings()["github.com"]; got != "140.82.112.22" {
		t.Fatalf("我们的行落在第一条之后却没生效：现在读到 %q", got)
	}
}

func TestPlanAcceptsIPv6(t *testing.T) {
	out := apply(t, header, Change{Domain: "github.com", IP: "::1", ItemID: "core-github"})
	if !strings.HasSuffix(out, "::1 github.com # gac:core-github prev:-\n") {
		t.Fatalf("IPv6 形状写歪了：\n%q", out)
	}
}

// 拒绝的那一批：每条都要有码，界面才知道该说哪句话〔已定：D6 乙——只回码与参数〕。
func TestPlanRefuses(t *testing.T) {
	for _, c := range []struct {
		name    string
		text    string
		changes []Change
		want    string
	}{
		{
			name: "一批里同一域名出现两次",
			text: header,
			changes: []Change{
				{Domain: "github.com", IP: "1.1.1.1", ItemID: "a"},
				{Domain: "github.com", IP: "2.2.2.2", ItemID: "b"},
			},
			want: errs.CodeHostsConflict,
		},
		{
			name:    "不是域名形状（无点）",
			text:    header,
			changes: []Change{{Domain: "localhost", IP: "1.1.1.1", ItemID: "a"}},
			want:    errs.CodeHostsBadValue,
		},
		{
			name:    "不是域名形状（大写）",
			text:    header,
			changes: []Change{{Domain: "GitHub.COM", IP: "1.1.1.1", ItemID: "a"}},
			want:    errs.CodeHostsBadValue,
		},
		{
			name:    "不是地址形状",
			text:    header,
			changes: []Change{{Domain: "github.com", IP: "140.82.112.999", ItemID: "a"}},
			want:    errs.CodeHostsBadValue,
		},
		{
			name:    "域名跟别的名挤在同一行",
			text:    "20.205.243.166 github.com codeload.github.com\n",
			changes: []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "a"}},
			want:    errs.CodeHostsSharedLine,
		},
		{
			name:    "同一个名在这一行里写了两遍",
			text:    "20.205.243.166 github.com github.com\n",
			changes: []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "a"}},
			want:    errs.CodeHostsSharedLine,
		},
		{
			name:    "别个条目已经占着这一行",
			text:    "1.1.1.1 github.com # gac:other prev:-\n",
			changes: []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "a"}},
			want:    errs.CodeHostsConflict,
		},
		{
			name:    "这一行上被人手工又加了一个名",
			text:    "1.1.1.1 github.com ssh.github.com # gac:a prev:-\n",
			changes: []Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "a"}},
			want:    errs.CodeHostsSharedLine,
		},
		{
			name: "同一个域名挂着两行我们的标记",
			text: "1.1.1.1 github.com # gac:a prev:-\n2.2.2.2 github.com # gac:a prev:-\n",
			changes: []Change{
				{Domain: "github.com", IP: "140.82.112.22", ItemID: "a"},
			},
			want: errs.CodeHostsConflict,
		},
		{
			name:    "还原时也一样：不许顺手删掉别人家的名",
			text:    "1.1.1.1 github.com ssh.github.com # gac:a prev:-\n",
			changes: []Change{{Domain: "github.com", IP: "", ItemID: "a"}},
			want:    errs.CodeHostsSharedLine,
		},
	} {
		_, err := Plan(Parse(c.text), c.changes)
		if err == nil {
			t.Errorf("%s：本该拒写，却写成功了", c.name)
			continue
		}
		if got := errs.CodeOf(err); got != c.want {
			t.Errorf("%s：码不对，got %s want %s（%v）", c.name, got, c.want, err)
		}
	}
}

func TestRefusalCarriesTheLineForTheHuman(t *testing.T) {
	// 界面要能指出"到底哪一行、还挤着谁"，只有一句"格式不对"等于让人自己翻文件。
	_, err := Plan(Parse("20.205.243.166 github.com codeload.github.com\n"),
		[]Change{{Domain: "github.com", IP: "140.82.112.22", ItemID: "a"}})
	p := errs.ParamsOf(err)
	if p["line"] != "20.205.243.166 github.com codeload.github.com" {
		t.Fatalf("参数里没带上那一行原文：%v", p)
	}
	if got := fmt.Sprint(p["otherNames"]); got != "[codeload.github.com]" {
		t.Fatalf("参数里没带上被牵连的名字：%v", p["otherNames"])
	}
}

func TestRestoreBringsTheOriginalBackByteForByte(t *testing.T) {
	// 故意带上缩进、制表符、多余空格和行尾注释：还原时要一字不差，
	// 因为"重新拼一遍"会把用户的写法改成我们的写法，那是改了不该改的东西。
	text := header + "   20.205.243.166\tgithub.com   # 用户自己写的注释\n"
	applied := apply(t, text, Change{Domain: "github.com", IP: "140.82.112.22", ItemID: "core-github"})
	if !strings.Contains(applied, "# gac:off:core-github    20.205.243.166\tgithub.com   # 用户自己写的注释\n") {
		t.Fatalf("停用行没带上原文的形状：%q", applied)
	}
	restored := apply(t, applied, Change{Domain: "github.com", IP: "", ItemID: "core-github"})
	if restored != text {
		t.Fatalf("还原之后跟原文不一样：\n got=%q\nwant=%q", restored, text)
	}
}

func TestRestoreRebuildsPrevLineWhenOriginalIsGone(t *testing.T) {
	// 应用之后，人在外面把那条被注释掉的原行删了。此时 prev 里存着的地址就是唯一的依据。
	applied := apply(t, header, Change{Domain: "github.com", IP: "140.82.112.22", ItemID: "core-github"})
	appliedWithPrev := apply(t, header+"9.9.9.9 github.com\n", Change{Domain: "github.com", IP: "140.82.112.22", ItemID: "core-github"})
	if !strings.Contains(appliedWithPrev, "prev:9.9.9.9") {
		t.Fatalf("没记上应用前的地址：%q", appliedWithPrev)
	}
	// 手工把停用行删干净，模拟"原行没了"。
	var kept []string
	for _, l := range strings.Split(appliedWithPrev, "\n") {
		if strings.HasPrefix(l, "# gac:off:") {
			continue
		}
		kept = append(kept, l)
	}
	broken := strings.Join(kept, "\n")

	out := apply(t, broken, Change{Domain: "github.com", IP: "", ItemID: "core-github"})
	if strings.Contains(out, "gac:") {
		t.Fatalf("还原之后这一行还带着我们的标记：%q", out)
	}
	if !strings.Contains(out, "9.9.9.9 github.com\n") {
		t.Fatalf("没按 prev 补回应用前的地址：%q", out)
	}
	if Parse(out).Mappings()["github.com"] != "9.9.9.9" {
		t.Fatalf("补回来的行不生效：%q", out)
	}
	// 顺手确认第一例：应用前压根没有这一行时，还原就是把这行删掉，不该留下 prev 补的行。
	out2 := apply(t, applied, Change{Domain: "github.com", IP: "", ItemID: "core-github"})
	if out2 != header {
		t.Fatalf("prev:- 的还原该只删不加：\n got=%q\nwant=%q", out2, header)
	}
}

func TestRestoreOfUntouchedDomainIsNoOp(t *testing.T) {
	out := apply(t, header, Change{Domain: "github.com", IP: "", ItemID: ""})
	if out != header {
		t.Fatalf("没动过的域名还原时改到了文件：%q", out)
	}
}

func TestPlanNeverDropsUserLines(t *testing.T) {
	// 底那一条：不管这批改几条，用户自己的每一行都必须还在（停用行算"换了个写法还在"，不许凭空少一行）。
	text := "# a\n#b\n\n127.0.0.1 localhost\n20.205.243.166 github.com\n185.199.108.133 raw.githubusercontent.com\n"
	out := apply(t, text,
		Change{Domain: "github.com", IP: "140.82.112.22", ItemID: "core"},
		Change{Domain: "raw.githubusercontent.com", IP: "185.199.108.153", ItemID: "assets"},
	)
	before := Parse(text).Lines()
	after := Parse(out).Lines()
	if len(after) != len(before)+2 { // 每个域名各多出一行：原行改成停用行，后面跟一行我们的
		t.Fatalf("行数变化不是 +2：%d → %d\n%q", len(before), len(after), out)
	}
	for _, l := range before {
		if l.IP == "" || l.Managed || l.Disabled {
			continue // 注释行与空行本来就不归这一层动
		}
		trimmed := strings.TrimSpace(l.Raw)
		// 要么原文还在原处，要么它变成我们的停用行留在原处（`# gac:off:<id> 原文`）。
		if !strings.Contains(out, l.Raw+"\n") && !strings.Contains(out, " "+trimmed+"\n") {
			t.Fatalf("用户这一行既不在原处也没当成停用行：%q\n内容=%q", l.Raw, out)
		}
	}
}
