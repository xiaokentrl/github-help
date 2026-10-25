package resolve

import (
	"fmt"
	"runtime"
	"sort"
)

// registration 是注册表里的一格。file 记下"是谁登记的"，撞号那条错误消息要能指名到文件。
type registration struct {
	spec        Spec
	newResolver func() Resolver
	file        string
}

// registrations 按 DefaultPriority、id 排好——注册顺序不参与，Go 里同包文件的 init() 顺序看着像文件名顺序，
// 但那是编译器实现细节；把它当顺序用，换一种构建方式就会变，所以这里每次取都现排。
var registrations []*registration

// Register 把一个方式交进注册表。各方式文件在自己的 init() 里调它，人不写清单（R6 的支点就在这儿）。
//
// 干什么用：它是"自动加载"唯一的落点——包里多一个 .go，清单就多一条，没人改第二个文件。
// 什么情况下会错、出错时人看得见什么：撞号（两个文件用同一个 id）和没声明适用范围（Kinds 为空）都在这里当场抛；
//
//	撞号如果让它悄悄盖过去，界面上少一种来源、日志里一个字没有，半年后没人查得出来。
//
// 怎么回来：错误消息带着先登记那一个的文件名；init 里用 must 把它变成"服务起不来"，不降级成警告。
func Register(spec Spec, newResolver func() Resolver) error {
	if spec.ID == "" {
		return fmt.Errorf("resolve: 方式没填 id（id 就是文件名去掉 .go）")
	}
	if spec.Title == "" {
		return fmt.Errorf("resolve: 方式 %s 没填界面显示名", spec.ID)
	}
	if len(spec.Kinds) == 0 {
		return fmt.Errorf("resolve: 方式 %s 没声明适用范围（Kinds 为空＝它会静默参选所有菜单块）", spec.ID)
	}
	if newResolver == nil {
		return fmt.Errorf("resolve: 方式 %s 没给构造函数", spec.ID)
	}

	_, file, _, _ := runtime.Caller(1)
	for _, r := range registrations {
		if r.spec.ID == spec.ID {
			return fmt.Errorf("resolve: 方式 id %q 撞号，%s 已经登记过它，%s 不能再登记一遍（换 id 或删掉一个文件）",
				spec.ID, r.file, file)
		}
	}

	registrations = append(registrations, &registration{spec: spec, newResolver: newResolver, file: file})
	sortSpecs()
	return nil
}

// All 给出参选清单（已排好，每轮顺序一样）。
//
// 什么情况下会错：调用方以为"顺序＝注册早晚"，于是靠顺序定主用 → 同分时主用换人。
// 怎么回来：谁参选由 service 按 Kinds 过滤，定主用/备用只准走 Rank。
func All() []Spec {
	out := make([]Spec, len(registrations))
	for i, r := range registrations {
		out[i] = cloneSpec(r.spec)
	}
	return out
}

// Get 按 id 取方式实现。
//
// 干什么用：第 6.1 节第 7 步"主用失败改选备用"要凭 id 把那个方式取出来再跑一遍。
// 什么情况下会错、出错时人看得见什么：取不到时返回 ok=false，调用方必须把这句写进快照——
// 静默当成零值继续跑，界面就显示"已经用备用方式了"，而它根本没跑。
func Get(id string) (Resolver, bool) {
	for _, r := range registrations {
		if r.spec.ID == id {
			return r.newResolver(), true
		}
	}
	return nil, false
}

// must 把 init() 里的错误变成"服务起不来"。
// 为什么不做成"记一条日志继续跑"：注册表少一个方式，用户看到的来源清单就是缺的，界面看不出异常，
// 这种"能跑但缺东西"最难查——让它起不来，人第一眼就知道。
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// sortSpecs 把注册表排成稳定顺序：先按 DefaultPriority，再按 id。
func sortSpecs() {
	sort.SliceStable(registrations, func(i, j int) bool {
		a, b := registrations[i].spec, registrations[j].spec
		if a.DefaultPriority != b.DefaultPriority {
			return a.DefaultPriority < b.DefaultPriority
		}
		return a.ID < b.ID
	})
}

func cloneSpec(s Spec) Spec {
	s.Kinds = append([]string(nil), s.Kinds...)
	return s
}
