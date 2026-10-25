package detect

import (
	"sort"
)

// rankips.go 是 IP 层那把尺子：一批读数里，哪个最优。
//
// 干什么用：把"能用且快"这句话落成一段能跑的代码——它是这件事在 IP 层唯一的家。
// 它是纯函数：不读时间、不读随机数、不碰磁盘，所以"同一批读数每次选出同一个地址"能在用例里逐例钉死。
//
// 什么情况下会错、出错时人看得见什么：
// ① 先比延迟再比"能不能服务" → 一支连不上的地址延迟记 0，会被排成最快当主用，
//
//	写进 hosts 后浏览器打不开，而界面上那行"已生效"看着没毛病。所以第一位永远是 CanServe。
//
// ② 同分不决胜 → 名次取决于切片初始顺序，等于把"自动选最优"做成"随机选一个"，
//
//	正是原型 :1281 那个"IP 来源天天变"的老毛病。所以决胜链固定：先延迟、再地址字符串字典序。
//
// ③ 延迟没测出来（RTTOK 为假）当成 0 → 同 ①。所以这类一律排在测出来的后面。
//
// 怎么回来：判据就三条（能服务 → 延迟小 → 地址字典序），一次都不许交换顺序；
//
//	"换 IP 有没有用"不在这儿判，那是 blocked.go 那张形状表的事。
func RankIPs(domain string, outcomes []Outcome) Result {
	ordered := orderOutcomes(outcomes)
	res := Result{Domain: domain, Outcomes: ordered}

	for _, o := range ordered {
		if o.CanServe {
			res.Best = o.IP
			if o.RTTOK {
				res.RTT = o.RTT
			}
			break
		}
	}
	if res.Best == "" {
		res.Code, res.Params = failCode(domain, ordered)
	}
	return res
}

// orderOutcomes 按名次排好一整批读数（探上的在前）。排的是拷贝，调用方传进来的切片改不动。
func orderOutcomes(outcomes []Outcome) []Outcome {
	out := append([]Outcome(nil), outcomes...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CanServe != out[j].CanServe {
			return out[i].CanServe
		}
		if out[i].CanServe {
			// 两个都比出延迟才比大小；只有一个比出来，比出来的那个在前。
			if out[i].RTTOK != out[j].RTTOK {
				return out[i].RTTOK
			}
			if out[i].RTT != out[j].RTT {
				return out[i].RTT < out[j].RTT
			}
		} else if out[i].Shape != out[j].Shape {
			return out[i].Shape < out[j].Shape // 失败的那批按形状聚堆，界面那行"探测明细"才好看清是怎么全灭的
		}
		return out[i].IP < out[j].IP
	})
	return out
}
