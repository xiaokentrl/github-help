package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
)

// update.go 接"检查更新／下载更新"两条问话。数据源＝本仓库的 GitHub Release〔2026-10-25 接真〕：
//
// 干什么用：CheckUpdate 问 latest release 有没有比当前版本新；DownloadUpdate 把对应平台的安装包
// 拉到配置目录的 updates/ 下。**不做自动安装**（root 服务替用户跑 dpkg/rpm 越权了：
// 装哪个包、什么时候装归用户），回执里给落盘路径，界面如实转述。
//
// 什么情况下会错、出错时人看得见什么：
// ① 网络不通（这台机器访问 GitHub 普遍要走代理：Go 的 http 认 HTTP(S)_PROXY 环境变量，
//    systemd 起的服务没有代理环境时这里如实报"连不上"，界面显示服务原话）；
// ② 拿到的版本号不比当前新 → hasUpdate=false／"没有包可下载"，这不是错，如实说"已是最新"；
// ③ 下载中断/落盘失败 → 回 GAC-UPDATE-503，.part 半个文件当场删掉，不许留着装作下载完。
//
// 如实交底一条：DownloadUpdate 是同步下载，通道那头的调用上限是 10 秒（bridge 的 callTimeout）——
// 慢网上 GUI 那头会先报"没人答"，而服务这边的下载会继续跑完、文件照样落进 updates/。
// 界面那一侧的"下载中/进度条"要走 download:progress 事件（契约里还挂着待接登记），那是下一步。
//
// 怎么回来：版本比较的判据在 compareVersions（纯函数，有逐例用例）；取数走 updater 接口，
// 用例传假件，生产走 httpUpdater（真 http）。

// Updater 是取数口：检查一次 API 拉取、下载一次大文件拉取，分开两个动作，
// 用例才能各假各的（检查假、下载真写盘，两路互不拖累）。
// 导出而非包内私有：cmd/service 的接线用例要在真通道上喂假件（否则它真拨 GitHub）。
type Updater interface {
	// FetchJSON 拉一份 JSON 文本（检查用）。
	FetchJSON(url string) ([]byte, error)
	// FetchToFile 拉一个大文件落盘（下载用），回写到的字节数。
	FetchToFile(url, dest string) (int64, error)
}

// httpUpdater 是生产那份：超时上限各自写死，HTTP(S)_PROXY 环境变量 Go 自己认。
type httpUpdater struct{}

const (
	// 为什么不用 releases/latest:GitHub 的这个端点只返回正式版,prerelease 永远 404——
	// 本项目的 m 系列版本号全带连字符,发版时全部标 prerelease,用它等于永远"查无更新"〔用户实机撞到〕。
	// 改用列表端点取第一条:含 prerelease,按创建时间倒序,最新的一版就是它。
	updateAPIBase   = "https://api.github.com/repos/xiaokentrl/github-help/releases?per_page=1"
	updateTimeout   = 15 * time.Second
	downloadTimeout = 10 * time.Minute
)

func (httpUpdater) FetchJSON(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// GitHub API 要求带 UA；不带会被 403,而那一句会被用户读成"更新通道坏了"
	req.Header.Set("User-Agent", "github-accel-updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: updateTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 给了 %s，不是 200", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func (httpUpdater) FetchToFile(url, dest string) (int64, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "github-accel-updater")
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("GitHub 给了 %s，不是 200", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, resp.Body)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(tmp) // 半个文件不许留着装作下载完
		return 0, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return n, nil
}

// compareVersions 比较两份"0.0.0-m24"形状的版本号：点段按数字比，尾巴上的 -mN 也按数字比。
// 为什么不引一个语义版本库：就这仓库自己的发版形状（取号器只出这两种），一条函数说得清；
// 引库反而要在"m24 比 m9 大吗"这种自家规则上跟库的 prerelease 语义打架。
// 返回：a>b＝1，a=b＝0，a<b＝-1；解析不了的段按 0 算。
func compareVersions(a, b string) int {
	return compareSegments(splitVersion(a), splitVersion(b))
}

func splitVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v = strings.ReplaceAll(v, "-", ".")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		// "m25" 这类带字母前缀的段取它的数字尾巴（m25→25）；一个数字都没有的段按 0
		n := 0
		has := false
		for _, r := range p {
			if r >= '0' && r <= '9' {
				n = n*10 + int(r-'0')
				has = true
			}
		}
		if !has {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

func compareSegments(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

// releasePayload 是 GitHub latest release 回执里我们要的那几栏，别的不要。
type releasePayload struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// pickAsset 按平台挑安装包：Windows 挑 zip，Linux 挑 deb（deb 是 deb/rpm 两条系共有的发版形态；
// rpm 系用户装更新时拿 deb 装不上——如实交底：这一版 Linux 更新只挑 deb,rpm 用户走手动下载）。
func pickAsset(assets []struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"browser_download_url"`
}) string {
	suffix := ".deb"
	if runtime.GOOS == "windows" {
		suffix = ".zip"
	}
	for _, a := range assets {
		if strings.HasSuffix(strings.ToLower(a.Name), suffix) {
			return a.URL
		}
	}
	return ""
}

// changelogLines 把 release 正文折成界面"这一版更新了哪些内容"那几行：
// 只收以 - 或 * 起头的条目行，最多 20 行——正文里的标题、空行、交底段落不是更新内容。
func changelogLines(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			out = append(out, strings.TrimSpace(t[2:]))
			if len(out) >= 20 {
				break
			}
		}
	}
	return out
}

// hGetVersion 不需要网络，也不需要更新通道的任何东西。
//
// BuildDate 由打包那一步（-ldflags）填进来；没填就如实留空，界面显示"—"。
// 编一个日期进二进制，用户按它判断"我装的是哪一天的包"就完全是错的。
func (s *Service) hGetVersion(map[string]any) (any, error) {
	return contract.VersionReply{Current: s.cfg.Version, BuildDate: s.cfg.BuildDate}, nil
}

// hCheckUpdate 无参数：问 GitHub 的 release 列表（含 prerelease）取最新一条,比较版本,
// 回 {latest, hasUpdate, log}。
func (s *Service) hCheckUpdate(map[string]any) (any, error) {
	raw, err := s.cfg.Updater.FetchJSON(updateAPIBase)
	if err != nil {
		return nil, errs.New(errs.CodeUpdateCheckFailed, nil, "连不上更新源：%v", err)
	}
	var rels []releasePayload
	if err := json.Unmarshal(raw, &rels); err != nil {
		return nil, errs.New(errs.CodeUpdateCheckFailed, nil, "更新源回的那句看不懂：%v", err)
	}
	if len(rels) == 0 || rels[0].TagName == "" {
		// 仓库一个 release 都没有（列表为空/404）＝"没有可比较的最新版"，如实答，不是错
		return contract.UpdateCheckReply{Latest: "", HasUpdate: false, Log: []string{}}, nil
	}
	latest := strings.TrimPrefix(rels[0].TagName, "v")
	return contract.UpdateCheckReply{
		Latest:    latest,
		HasUpdate: compareVersions(latest, s.cfg.Version) > 0,
		Log:       changelogLines(rels[0].Body),
	}, nil
}

// hDownloadUpdate 无参数：先查一遍有没有更新，再把对应平台的包装到配置目录 updates/ 下。
// 没有更新还硬下载＝把旧包当新包装；如实回"已是最新"。
func (s *Service) hDownloadUpdate(map[string]any) (any, error) {
	check, err := s.hCheckUpdate(nil)
	if err != nil {
		return nil, err
	}
	c := check.(contract.UpdateCheckReply)
	if !c.HasUpdate {
		return nil, errs.New(errs.CodeUpdateCheckFailed, map[string]any{"latest": c.Latest},
			"已是最新版本（%s），没有包可下载", s.cfg.Version)
	}
	raw, err := s.cfg.Updater.FetchJSON(updateAPIBase)
	if err != nil {
		return nil, errs.New(errs.CodeUpdateCheckFailed, nil, "连不上更新源：%v", err)
	}
	var rels []releasePayload
	if err := json.Unmarshal(raw, &rels); err != nil {
		return nil, errs.New(errs.CodeUpdateCheckFailed, nil, "更新源回的那句看不懂：%v", err)
	}
	if len(rels) == 0 {
		return nil, errs.New(errs.CodeUpdateCheckFailed, nil, "更新源上还没有任何发布版")
	}
	url := pickAsset(rels[0].Assets)
	if url == "" {
		return nil, errs.New(errs.CodeUpdateCheckFailed, map[string]any{"platform": runtime.GOOS},
			"这一版没有适合本平台（%s）的安装包", runtime.GOOS)
	}
	name := filepath.Base(strings.Split(url, "?")[0])
	dest := filepath.Join(s.updatesDir(), name)
	if _, err := s.cfg.Updater.FetchToFile(url, dest); err != nil {
		return nil, errs.New(errs.CodeUpdateDownloadFailed, map[string]any{"url": url},
			"包没下载完：%v", err)
	}
	return contract.UpdateDownloadReply{SavedPath: dest}, nil
}

// updatesDir 放下载包：配置目录底下 updates/，卸载软件不清它（用户可能还没拿去装）。
func (s *Service) updatesDir() string {
	if s.cfg.SettingsPath == "" {
		return os.TempDir()
	}
	return filepath.Join(filepath.Dir(s.cfg.SettingsPath), "updates")
}
