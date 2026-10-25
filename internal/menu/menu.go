// Package menu 是全项目域名清单在后端的家：一块主菜单一个目录，目录里只有数据、没有逻辑分叉。
//
// 那 11 份 JSON 由 tools/gen-backend-menu.mjs 从 frontend/menu/ 生成，人不手写也不许手改——
// 手改会被下一次生成盖掉，重新生成有没有变化由门禁 G4 现场比（frontend/tests/backendMenu.test.mjs）。
// 子菜单（条目）不建目录：实测条目身上除了"叫什么、管哪几个域名"再没有别的属性，54 个目录天生是空壳。
package menu

// Item 是一块菜单下面的一条二级条目，就是界面那棵树上的第二层。
type Item struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
}

// Menu 是一块主菜单。Order 决定它在界面上排第几，Common 来自同目录的 common/common.json。
type Menu struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Order  int        `json:"order"`
	Items  []Item     `json:"items"`
	Common MenuCommon `json:"-"`
}

// MenuCommon 是本块菜单下面所有条目共用的部分，只有父域归属这一样。
// 块级探测口径与还原口径此刻一块菜单也没有值，所以这里连字段都不声明：
// 数据里一旦出现没登记过的键，严格解码当场报错，不会静默忽略（那是"第二个家"最开始的形状）。
type MenuCommon struct {
	ParentDomains []string `json:"parentDomains"`
}

// Unsupported 是一条写不进 hosts 的数据项。它照旧留在清单里、界面上照旧显示，
// 但写入那一刻必须被拒绝并如实标出来——不是静默跳过（那样用户只会以为网络有问题）。
type Unsupported struct {
	MenuID string `json:"menuId"`
	ItemID string `json:"itemId"`
	Value  string `json:"value"`
}

// Conflict 是一个域名同时挂在两条条目下。写入与"还原"都必须先拒再说。
type Conflict struct {
	Domain string   `json:"domain"`
	Items  []string `json:"items"`
}

// Catalog 是一份校验过的只读清单。除了 Load 与下列方法，别处不许改它。
type Catalog struct {
	menus       []Menu
	byItem      map[string]itemRef
	domains     []string
	unsupported []Unsupported
	conflicts   []Conflict
}

type itemRef struct {
	item Item
	menu Menu
}

// Menus 给出按 Order 排好的全部菜单。返回的是拷贝，调用方改不动清单本体——
// 要改数据请回前端那份真源，重新生成一次。
func (c *Catalog) Menus() []Menu {
	out := make([]Menu, len(c.menus))
	for i, m := range c.menus {
		out[i] = cloneMenu(m)
	}
	return out
}

// cloneMenu 连条目与域名一起拷：只拷最外层结构的话，调用方改 Items[0].Domains[0] 就能改到清单里。
func cloneMenu(m Menu) Menu {
	out := m
	out.Items = make([]Item, len(m.Items))
	for i, it := range m.Items {
		out.Items[i] = it
		out.Items[i].Domains = append([]string(nil), it.Domains...)
	}
	out.Common.ParentDomains = append([]string(nil), m.Common.ParentDomains...)
	return out
}

// ByItem 由条目 id 反查条目和它属于哪块菜单。查不到时 ok 为 false，由调用方决定给用户看哪句人话。
func (c *Catalog) ByItem(itemID string) (Item, Menu, bool) {
	ref, ok := c.byItem[itemID]
	if !ok {
		return Item{}, Menu{}, false
	}
	return cloneItem(ref.item), cloneMenu(ref.menu), true
}

func cloneItem(it Item) Item {
	out := it
	out.Domains = append([]string(nil), it.Domains...)
	return out
}

// Domains 是全部受管域名的去重列表，"全部恢复默认"与 hosts 的覆盖检查靠它。
// 不含那几条写不进 hosts 的数据项（它们由 UnsupportedDomains 单独交出去）。
func (c *Catalog) Domains() []string {
	out := make([]string, len(c.domains))
	copy(out, c.domains)
	return out
}

// Common 取某块菜单的公共部分。没有该块时 ok 为 false——Load 已要求每块必有 common.json，
// 所以这里只会是"菜单 id 写错了"，不会是"那块还没配"。
func (c *Catalog) Common(menuID string) (MenuCommon, bool) {
	for _, m := range c.menus {
		if m.ID == menuID {
			return cloneMenu(m).Common, true
		}
	}
	return MenuCommon{}, false
}

// ParentDomains 是父域清单唯一的出口：校验与界面那句"这块菜单管哪些站"共用它，不在别处再数一遍。
func (c *Catalog) ParentDomains(menuID string) []string {
	mc, ok := c.Common(menuID)
	if !ok {
		return nil
	}
	return mc.ParentDomains
}

// UnsupportedDomains 照单列出那些不是合法域名形状的数据项（带路径的地址、中文占位）。
func (c *Catalog) UnsupportedDomains() []Unsupported {
	out := make([]Unsupported, len(c.unsupported))
	copy(out, c.unsupported)
	return out
}

// DomainConflicts 照单列出一个域名挂多条条目的情形。
func (c *Catalog) DomainConflicts() []Conflict {
	out := make([]Conflict, len(c.conflicts))
	for i, cf := range c.conflicts {
		out[i] = cf
		out[i].Items = append([]string(nil), cf.Items...)
	}
	return out
}
