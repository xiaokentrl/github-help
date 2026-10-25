package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
)

// jsonTags 取出一个结构体全部的 json 键名（把 ",omitempty" 这类尾巴剥掉）。
// 比对型检查一律两侧各解析出标识符再比集合，不许用 strings.Contains——改名不报红〔已定：那条门禁硬教训〕。
func jsonTags(t *testing.T, zero any) []string {
	t.Helper()
	typ := reflect.TypeOf(zero)
	if typ.Kind() != reflect.Struct {
		t.Fatalf("%T 不是结构体", zero)
	}
	out := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			t.Fatalf("%s 的字段 %s 没有 json 键名", typ.Name(), typ.Field(i).Name)
		}
		out = append(out, strings.Split(tag, ",")[0])
	}
	sort.Strings(out)
	return out
}

// stored 是文件里那一份，字段全是指针——形状跟契约那份不一样，但键名必须一字不差。
// 这一条是"设置存了没生效"那种翻车的唯一机器闸：Go 编得过，只有名字对不上时才会静默丢键。
func TestStoredKeysEqualContractKeys(t *testing.T) {
	got := jsonTags(t, stored{})
	want := jsonTags(t, contract.Settings{})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("设置文件的键名跟契约对不上：\n got=%v\nwant=%v", got, want)
	}
	if len(got) != 13 {
		t.Fatalf("键数不是 13（技术方案第 6 节那 13 个）：%d", len(got))
	}
}

// Defaults 的 13 个值要跟原型 :1199-1213 逐字相同。这里把那份原文抄成对照表：
// 原型是需求载体，它改了而这边没改，这条就该红。
func TestDefaultsMatchThePrototype(t *testing.T) {
	// 两个开机键（AutoDetectOnStartup／AutoApplyOnStartup）2026-10-25 经用户拍板默认关，
	// 原型里是开——这两格故意不再跟原型，其余 11 键仍逐字对齐原型默认。
	want := Settings{
		SidebarWidth: 240, QueueHeight: 252, Zoom: 1,
		Language: "zh-CN", Theme: "light",
		TrayMinimize: true, ShowTrayIcon: true, AutoStart: true, StartMinimized: false,
		DetectFocusFollow: false, ApplyFocusFollow: false,
		AutoDetectOnStartup: false, AutoApplyOnStartup: false,
	}
	if got := Defaults(); got != want {
		t.Fatalf("默认值跟原型对不上：\n got=%+v\nwant=%+v", got, want)
	}
	// 默认值只许这一个家：没写过文件时读出来的必须与它完全相同。
	got, err := LoadFrom(filepath.Join(t.TempDir(), "根本没这个文件.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("缺文件时的回落跟默认值不一样：\n got=%+v", got)
	}
}

// 指针那一步防的就是这一条：文件里只写了一个键，其余 12 个必须保持默认值，
// 而不是被零值盖掉。具体形状是"autoStart 开着开着自己关了"，而文件里看着什么都没动。
func TestPartialFileKeepsEveryUnwrittenDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"sidebarWidth":300}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.SidebarWidth = 300
	if got != want {
		t.Fatalf("没写的键被零值盖掉了：\n got=%+v\nwant=%+v", got, want)
	}
}

func TestExplicitFalseIsNotTreatedAsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"autoStart":false,"trayMinimize":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.AutoStart || got.TrayMinimize {
		t.Fatalf(`写了 false 却读回 true——指针那一层白写了：%+v`, got)
	}
}

func TestUnknownKeyIsAnError(t *testing.T) {
	// 多一个没登记的键要当场报错，不能静默忽略：忽略意味着"人改了设置而程序没反应"，
	// 那种情况界面永远显示成功，比报错难查得多。
	// 如实记一句边界：Go 的字段匹配不分大小写，所以 "sideBarWidth" 这种写法照样能读进 sidebarWidth——
	// 严格解码防住的是"没登记过的键"，不是"大小写写歪的键"，后者本来就不影响结果。
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"sidebarWidth":300,"tray":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if got := errs.CodeOf(err); got != errs.CodeSettingsParse {
		t.Fatalf("got %s（%v）", got, err)
	}
}

func TestRidiculousNumbersAreRejected(t *testing.T) {
	for _, body := range []string{
		`{"zoom":99}`, `{"sidebarWidth":0}`, `{"queueHeight":-5}`, `{"theme":""}`, `{"language":""}`,
		`{这不是 JSON}`,
	} {
		path := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadFrom(path); errs.CodeOf(err) != errs.CodeSettingsParse {
			t.Errorf("%s：该报错却给了 %v", body, err)
		}
	}
}

// 合法值清单的家在界面那边（主题靠扫 styles/、语言靠扫 locales/）。
// 这一条钉住"设置这一层不许长出第二份清单"：存了个界面没有的值，读回来得是成功的。
func TestUnlistedThemeAndLanguageStillLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dusk","language":"en-US"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("设置层不该拦合法值清单之外的东西：%v", err)
	}
	if got.Theme != "dusk" || got.Language != "en-US" {
		t.Fatalf("值没原样带回来：%+v", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Nested", "settings.json")
	s := Defaults()
	s.SidebarWidth = 340
	s.QueueHeight = 480
	s.Theme = "night"
	s.Zoom = 1.25
	s.AutoApplyOnStartup = false
	if err := SaveTo(path, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != s {
		t.Fatalf("存进去再读回来不是同一组设置：\n got=%+v\nwant=%+v", got, s)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("设置文件权限该 0600，实际 %v", fi.Mode().Perm())
	}
	// 原子写：换完之后不该在同目录留下 .tmp。
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("留下了没用完的临时文件：%v", err)
	}
}

func TestSaveRejectsBeforeWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := Defaults()
	s.Zoom = 7
	if got := errs.CodeOf(SaveTo(path, s)); got != errs.CodeSettingsParse {
		t.Fatalf("got %s", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("不该写出去的东西写出去了")
	}
}

// 文件里的键集合要正好是那 13 个——不多写（多写=往契约之外塞东西）、不少写（少写=读回来靠默认值兜）。
func TestSavedFileHasExactlyThirteenKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := SaveTo(path, Defaults()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if want := jsonTags(t, contract.Settings{}); !reflect.DeepEqual(keys, want) {
		t.Fatalf("写出去文件的键集合跟契约不一样：\n got=%v\nwant=%v", keys, want)
	}
}

func TestPathAndBackupDirShape(t *testing.T) {
	// 只钉形状钉两头的关系，不钉具体目录：那由 os.UserConfigDir() 说了算，本机值不算判据。
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "github-hlep" {
		t.Fatalf("配置目录该叫 github-hlep（与项目名一致），got %q", dir)
	}
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(p) != dir || filepath.Base(p) != "settings.json" {
		t.Fatalf("设置文件没落在配置目录里：%q", p)
	}
	bd, err := BackupDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(bd) != dir || bd == p {
		t.Fatalf("备份目录该与配置目录同级且不等于设置文件：%q vs %q", bd, p)
	}
}
