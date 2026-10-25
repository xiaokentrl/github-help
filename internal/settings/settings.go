// Package settings 是那 13 个偏好键的家——只有读写与"防坏文件"这一层，不管这些值什么意思。
//
// 干什么用：读一个 JSON 配置文件（Linux 落 XDG 配置目录、Windows 落 %APPDATA%），
// 文件里没写的键保持默认值，写了但没登记过的键当场报错。
// 什么情况下会错、出错时人看得见什么：键名漂一个字母不会编译失败，只会"设置存了没生效"——
// 所以键名以 contract.Settings 那 13 个冻结字段名为准，测试用 reflect 双向比两边的名字集合；
// 文件坏了要如实报错，不许静默回到默认值（那种"看起来一切正常"的回落会让用户以为改设置没生效）。
// 怎么回来：这个文件是人可编辑的，删掉它就回到默认值；卸载留档（本期口径：不跟着 hosts 一起还原）。
package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
)

// Settings 直接复用契约里那一份结构体：键名只有一个家，服务侧与界面侧读的是同一张形状。
type Settings = contract.Settings

// Defaults 给出与原型 :1199-1213 逐字相同的 13 个默认值。
// 默认值只许在这一个函数里定义——散到读文件与写文件两处，迟早给出不一样的那一份。
func Defaults() Settings {
	return Settings{
		SidebarWidth:      240,
		QueueHeight:       252,
		Zoom:              1,
		Language:          "zh-CN",
		Theme:             "light",
		TrayMinimize:      true,
		ShowTrayIcon:      true,
		AutoStart:         true,
		StartMinimized:    false,
		DetectFocusFollow: false,
		ApplyFocusFollow:  false,
		// 开机自动化默认关〔已定：2026-10-25 用户拍板，与 frontend/core/settings.js 的 DEFAULTS 同轮改〕。
		AutoDetectOnStartup: false,
		AutoApplyOnStartup:  false,
	}
}

// stored 是文件里那一份的形状，每个字段都是指针。
// 为什么非要指针：Go 的解码分不出"文件里写了 false"和"文件里没写这个键"，
// 用值类型合并会让 8 个布尔键里没写的那些从默认值 true 掉成零值 false——
// 具体形状就是"自动启动开着开着自己关了"，而且文件里看着没动过。
type stored struct {
	SidebarWidth        *int     `json:"sidebarWidth"`
	QueueHeight         *int     `json:"queueHeight"`
	Zoom                *float64 `json:"zoom"`
	Language            *string  `json:"language"`
	Theme               *string  `json:"theme"`
	TrayMinimize        *bool    `json:"trayMinimize"`
	ShowTrayIcon        *bool    `json:"showTrayIcon"`
	AutoStart           *bool    `json:"autoStart"`
	StartMinimized      *bool    `json:"startMinimized"`
	DetectFocusFollow   *bool    `json:"detectFocusFollow"`
	ApplyFocusFollow    *bool    `json:"applyFocusFollow"`
	AutoDetectOnStartup *bool    `json:"autoDetectOnStartup"`
	AutoApplyOnStartup  *bool    `json:"autoApplyOnStartup"`
}

// Dir 给出这个程序的用户数据目录（配置文件、hosts 备份都放它下面）。
// 单开这一个出口是为了别处不必自己再拼一遍 os.UserConfigDir()——拼两份迟早拼出两个家。
// 取值顺序：GAC_CONFIG_DIR（显式指定，systemd 单元用它在无 HOME 的环境里钉一个家，
// 2026-10-26 实测：系统服务没有这套环境变量，不钉则设置存不了、备份目录拿不到、
// 状态栏"数据目录"显示"—"）＞ os.UserConfigDir()（普通用户与开发态）。
func Dir() (string, error) {
	if explicit := os.Getenv("GAC_CONFIG_DIR"); explicit != "" {
		return filepath.Join(explicit, "github-hlep"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", errs.New(errs.CodeSettingsDir, nil, "取不到配置目录").WithCause(err)
	}
	return filepath.Join(dir, "github-hlep"), nil
}

// Path 给出配置文件在哪儿。用标准库那一个函数，Linux/Windows 各给各的目录，
// 因此不需要规划里那对 path.go / path_windows.go——少一条按平台分叉的分支（总纲第 0.1 节第 4 问）。
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// BackupDir 给出 hosts 备份放哪儿（服务启动时把它交给 hosts.NewFileEngine）。
// 放在这一层是因为"往哪儿存"归设置管，而 hosts 包不许 import 设置（第 8 节依赖方向）：
// 需要目录的人把路径传进去，不让引擎自己伸手拿。
func BackupDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hosts-backups"), nil
}

// Load 读用户那一份。
func Load() (Settings, error) {
	p, err := Path()
	if err != nil {
		return Defaults(), err
	}
	return LoadFrom(p)
}

// LoadFrom 从指定路径读，用例靠它注入临时目录，不需要动用户机器上的配置。
func LoadFrom(path string) (Settings, error) {
	out := Defaults()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, errs.New(errs.CodeSettingsRead, map[string]any{"path": path}, "读不到设置文件").WithCause(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var s stored
	if err := dec.Decode(&s); err != nil {
		return out, errs.New(errs.CodeSettingsParse, map[string]any{"path": path}, "设置文件看不懂，界面上的设置不会跟着改").WithCause(err)
	}
	s.mergeInto(&out)
	if err := check(out); err != nil {
		return out, errs.New(errs.CodeSettingsParse, map[string]any{"path": path}, "设置文件里的数不像话").WithCause(err)
	}
	return out, nil
}

// mergeInto 只把文件里真写了的那些键盖到 out 上；没写的保持默认值。
func (s stored) mergeInto(out *Settings) {
	if s.SidebarWidth != nil {
		out.SidebarWidth = *s.SidebarWidth
	}
	if s.QueueHeight != nil {
		out.QueueHeight = *s.QueueHeight
	}
	if s.Zoom != nil {
		out.Zoom = *s.Zoom
	}
	if s.Language != nil {
		out.Language = *s.Language
	}
	if s.Theme != nil {
		out.Theme = *s.Theme
	}
	if s.TrayMinimize != nil {
		out.TrayMinimize = *s.TrayMinimize
	}
	if s.ShowTrayIcon != nil {
		out.ShowTrayIcon = *s.ShowTrayIcon
	}
	if s.AutoStart != nil {
		out.AutoStart = *s.AutoStart
	}
	if s.StartMinimized != nil {
		out.StartMinimized = *s.StartMinimized
	}
	if s.DetectFocusFollow != nil {
		out.DetectFocusFollow = *s.DetectFocusFollow
	}
	if s.ApplyFocusFollow != nil {
		out.ApplyFocusFollow = *s.ApplyFocusFollow
	}
	if s.AutoDetectOnStartup != nil {
		out.AutoDetectOnStartup = *s.AutoDetectOnStartup
	}
	if s.AutoApplyOnStartup != nil {
		out.AutoApplyOnStartup = *s.AutoApplyOnStartup
	}
}

// Save 原子写用户那一份：先写同目录临时文件，再 rename 盖过去。
// 中途断电或磁盘满时，原来那份要么完整旧、要么完整新，不会停在半行 JSON 上。
func Save(s Settings) error {
	p, err := Path()
	if err != nil {
		return err
	}
	return SaveTo(p, s)
}

// SaveTo 写进指定路径（用例注入临时目录用）。
func SaveTo(path string, s Settings) error {
	if err := check(s); err != nil {
		return errs.New(errs.CodeSettingsParse, map[string]any{"path": path}, "这组设置存不了").WithCause(err)
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return errs.New(errs.CodeSettingsWrite, map[string]any{"path": path}, "设置编码不出来").WithCause(err)
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errs.New(errs.CodeSettingsWrite, map[string]any{"path": path}, "建不出配置目录").WithCause(err)
	}
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return errs.New(errs.CodeSettingsWrite, map[string]any{"path": path}, "写设置失败").WithCause(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return errs.New(errs.CodeSettingsWrite, map[string]any{"path": path}, "换设置文件失败").WithCause(err)
	}
	return nil
}

// check 只做"防坏文件"这一层：像素数为正、缩放落在看得过去的区间、两个字符串非空。
// 故意不校验 theme 与 language 的取值——合法值清单的家在界面那边（主题靠扫 styles/、语言靠扫 locales/），
// 在这儿再列一份就是那批清单的第二个家。存了个界面没有的值，界面会照它已定的口径回落并大声报错。
func check(s Settings) error {
	if s.SidebarWidth <= 0 || s.SidebarWidth > 8192 {
		return fmt.Errorf("sidebarWidth=%d 不在能画出来的范围里", s.SidebarWidth)
	}
	if s.QueueHeight <= 0 || s.QueueHeight > 8192 {
		return fmt.Errorf("queueHeight=%d 不在能画出来的范围里", s.QueueHeight)
	}
	if s.Zoom < 0.5 || s.Zoom > 2 {
		return fmt.Errorf("zoom=%v 不在能画出来的范围里", s.Zoom)
	}
	if s.Language == "" {
		return fmt.Errorf("language 是空的")
	}
	if s.Theme == "" {
		return fmt.Errorf("theme 是空的")
	}
	return nil
}
