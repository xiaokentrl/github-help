package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// distIndex 是"这一份确实是构建出来的界面"的那一个凭据：vite 出来必有 index.html。
// 为什么只验它而不验资源文件名：产物名字带内容哈希，每轮构建都换，index.html 不换。
const distIndex = "index.html"

// resolveDist 决定界面文件从哪个目录读，按三条顺序找到第一条就停：
// ① -dist 指的那一处；② 与可执行文件同级的 web/dist（发布态）；③ 仓库里的 frontend/dist（开发态）。
//
// 干什么用：正式界面只有 frontend/ 那一棵树，构建产物只有 dist/ 那一份，壳从磁盘读它——
// 不在 cmd/gui 里再嵌第二份，否则改一次界面要记得两处，而上一版就是这么错的。
// 什么情况下会错、人看得见什么：三条都没命中时当场退出，并把找过的三处一次列全。
// 为什么不干脆开一个空窗口：空窗口和"界面脚本挂了"长得一模一样，人只会以为工具没作用；
// 一句写清在哪儿找过的错，才知道下一步是去 frontend/ 跑一次构建。
// 怎么回来：跑一次 npm run build（或按②那一条把产物摆到可执行文件旁边），也可以用 -dist 直接指过去。
func resolveDist(flagVal string) (string, error) {
	if flagVal != "" {
		abs, err := filepath.Abs(flagVal)
		if err != nil {
			return "", fmt.Errorf("界面目录的名字拼不出来: %w", err)
		}
		if !hasIndex(abs) {
			return "", fmt.Errorf("-dist 指的目录里没有 %s：%s（先去 frontend/ 跑一次 npm run build，或把这参数指到真正的构建产物）", distIndex, abs)
		}
		return abs, nil
	}
	tried := distCandidates()
	for i, cand := range tried {
		if abs, err := filepath.Abs(cand); err == nil {
			tried[i] = abs
			cand = abs
		}
		if hasIndex(cand) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("找不到界面文件，找过这几处：%s（开发态先在 frontend/ 跑 npm run build，或加 -dist 指一份）", strings.Join(tried, "、"))
}

// distCandidates 给出②③那三处。
// 为什么留③这一条：go run 把可执行文件放在临时目录里，那儿永远不会有 dist/，
// 只有当前工作目录这一条能接住"在仓库根直接 go run ./cmd/gui"这一种跑法。
func distCandidates() []string {
	exe, err := os.Executable()
	if err != nil {
		return []string{"web/dist", "frontend/dist"}
	}
	dir := filepath.Dir(exe)
	return []string{
		filepath.Join(dir, "web", "dist"),
		filepath.Join(dir, "frontend", "dist"),
		"frontend/dist",
	}
}

func hasIndex(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, distIndex))
	return err == nil && !st.IsDir()
}
