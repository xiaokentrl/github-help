package menu

// 校验层：服务启动那一刻把清单里所有"能当场判定的错"判完。
//
// 干什么用：分两类处理。形状类（撞号、顺序断档、条目没有域名、父域清单对不上）是数据坏了，
// 直接返回错误让服务起不来；内容类（不是合法域名形状的字符串、一个域名挂两条条目）照单登记在清单上，
// 由写入那一刻拒绝并如实标出来〔已定：D4 甲，54 条数据一个字不改〕——静默跳过最坏，
// 用户只会以为网络有问题。
//
// 会错在哪：父域那份清单如果只信文件不重算，它跟 items 一漂就没人知道，界面上那句
// "这块菜单管哪些站"会变成假话；所以这里用 items 现场重算再逐字比。
//
// 怎么回来：每条错误消息带菜单 id 与条目 id，改的是前端真源，跑一次生成器就对齐。

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const labelPattern = `[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?`

// 与 tools/gen-backend-menu.mjs 里那条 DOMAIN_SHAPE 同一口径：至少两段，只有小写字母、数字、连字符，
// 段首段尾不许是连字符，末段以字母起头，总长 1–253。两边任一侧改了，menu_test.go 的对照表当场红。
var domainRe = regexp.MustCompile(`^` + labelPattern + `(?:\.` + labelPattern + `)*\.[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidDomain 是"这串是不是能写进 hosts 的域名形状"这把尺子唯一的家，给别的包用的那一个出口。
// hosts 写入前要拿它拒掉那几条不是域名的数据项〔已定：D4 甲——数据不改、写入那一刻拒绝并如实标出来〕；
// 那边不许自己再写一份判定，否则两把尺子迟早量出两个结果（domain-cases.json 那张对照表只钉得住这一处）。
func ValidDomain(s string) bool { return isDomain(s) }

func isDomain(s string) bool {
	return len(s) >= 1 && len(s) <= 253 && domainRe.MatchString(s)
}

// parentOf 取合法域名的最后两段。不合格的没有父域可言（返回空串，调用方跳过）。
func parentOf(s string) string {
	if !isDomain(s) {
		return ""
	}
	parts := strings.Split(s, ".")
	return strings.Join(parts[len(parts)-2:], ".")
}

// index 摊平条目、域名与两类内容问题；撞号在这里当场抛。
func (c *Catalog) index() error {
	seenItem := map[string]string{}
	domainOwner := map[string][]string{}
	domains := map[string]bool{}

	for _, m := range c.menus {
		if m.Name == "" {
			return fmt.Errorf("menu/%s：菜单名是空的", m.ID)
		}
		if len(m.Items) == 0 {
			return fmt.Errorf("menu/%s：这块菜单一条条目也没有。加一块菜单＝加数据，不许只加目录", m.ID)
		}
		for _, it := range m.Items {
			if prev, dup := seenItem[it.ID]; dup {
				return fmt.Errorf("条目 id 重复：%q 同时出现在 menu/%s 与 menu/%s", it.ID, prev, m.ID)
			}
			seenItem[it.ID] = m.ID
			if it.Name == "" {
				return fmt.Errorf("menu/%s 的条目 %q 名字是空的", m.ID, it.ID)
			}
			if len(it.Domains) == 0 {
				return fmt.Errorf("menu/%s 的条目 %q 一个域名也没有：这条在界面上点不动，也不许当空条目留着", m.ID, it.ID)
			}
			c.byItem[it.ID] = itemRef{item: it, menu: m}

			inside := map[string]bool{}
			for _, d := range it.Domains {
				if inside[d] {
					return fmt.Errorf("menu/%s 的条目 %q 里域名 %q 写了两次", m.ID, it.ID, d)
				}
				inside[d] = true
				if !isDomain(d) {
					c.unsupported = append(c.unsupported, Unsupported{MenuID: m.ID, ItemID: it.ID, Value: d})
					continue
				}
				domains[d] = true
				domainOwner[d] = append(domainOwner[d], it.ID)
			}
		}
	}

	for _, m := range c.menus {
		for _, it := range m.Items {
			for _, d := range it.Domains {
				if owners := domainOwner[d]; len(owners) > 1 {
					c.appendConflict(Conflict{Domain: d, Items: owners})
				}
			}
		}
	}

	sorted := make([]string, 0, len(domains))
	for d := range domains {
		sorted = append(sorted, d)
	}
	sort.Strings(sorted)
	c.domains = sorted
	return nil
}

// validate 查形状类问题：撞号、顺序断档、父域清单漂了。
func (c *Catalog) validate() error {
	seenID := map[string]bool{}
	seenOrder := map[int]string{}
	for _, m := range c.menus {
		if seenID[m.ID] {
			return fmt.Errorf("菜单 id 重复：%q", m.ID)
		}
		seenID[m.ID] = true
		if prev, dup := seenOrder[m.Order]; dup {
			return fmt.Errorf("菜单 order 撞号：第 %d 位同时是 %q 与 %q，界面上的菜单顺序会变成掷硬币", m.Order, prev, m.ID)
		}
		seenOrder[m.Order] = m.ID
		if err := c.validateCommon(m); err != nil {
			return err
		}
	}
	for i := 1; i <= len(c.menus); i++ {
		if _, ok := seenOrder[i]; !ok {
			return fmt.Errorf("菜单 order 从 1 到 %d 中间断了第 %d 位：要么补上要么往前挪，不许留洞", len(c.menus), i)
		}
	}
	return nil
}

// validateCommon 管公共层（G12）：与 items 现场重算的父域集合逐字等值，顺序也算。
// 三条"不许"里只有这一条真需要查：
//
//	「不许出现 items 键或没登记过的键」由严格解码在读取那一刻拦（见 load.go 的 decodeEmbedded）；
//	「不许出现条目 id」「每条必须正好两段」由等值天然管住——条目 id 不是任何域名的最后两段，
//	而现算出来的父域本来就都是两段。写两条检查管同一件事，正是本文件要防的那种重复。
func (c *Catalog) validateCommon(m Menu) error {
	computed := map[string]bool{}
	for _, it := range m.Items {
		for _, d := range it.Domains {
			if p := parentOf(d); p != "" {
				computed[p] = true
			}
		}
	}
	want := make([]string, 0, len(computed))
	for p := range computed {
		want = append(want, p)
	}
	sort.Strings(want)

	if !equalStrings(m.Common.ParentDomains, want) {
		return fmt.Errorf("menu/%s/common/common.json 里的 parentDomains 与 items 现算的结果对不上：现算 %v，文件里写的是 %v。"+
			"这一份由生成器算，手改会被下一次生成盖掉", m.ID, want, m.Common.ParentDomains)
	}
	return nil
}

func (c *Catalog) appendConflict(n Conflict) {
	for i := range c.conflicts {
		if c.conflicts[i].Domain == n.Domain {
			return
		}
	}
	c.conflicts = append(c.conflicts, n)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
