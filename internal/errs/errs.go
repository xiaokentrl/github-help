// Package errs 是全项目错误码唯一的家。
//
// 干什么用：服务侧出错时只回「一个码 + 一组结构化参数」，整句人话在前端语言文件那一个家（技术方案第 15 节 D6 乙）。
// 什么情况下会错、出错时人看得见什么：错误只剩一句自由文本时，界面只能整句显示，用户不知道该点「重试」还是「重装服务」；
// 码名改了而语言文件没改，界面会把码本身当文案显示出来。
// 怎么回来：码只在本文件登记（GAC-MODE- 那一族例外，由方式 id 现场拼），
// 界面侧按键 `err.<码>` 取句子，码清单与语言键的集合等值由门禁 G13 管（前端接线那一站实装）。
package errs

import (
	"errors"
	"fmt"
	"strings"
)

// Error 是一条带码的错误。Msg 是给日志和开发者看的，不许当成给用户看的那句话。
type Error struct {
	Code   string
	Params map[string]any
	Msg    string
}

func (e *Error) Error() string {
	if e.Msg == "" {
		return e.Code
	}
	return e.Code + ": " + e.Msg
}

// New 造一条带码错误。params 可为 nil，但凡是给用户看的数（域名、路径、第几条）都要放进来，
// 否则界面拿到码也凑不出那句话。
func New(code string, params map[string]any, format string, args ...any) *Error {
	e := &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
	if params != nil {
		e.Params = make(map[string]any, len(params))
		for k, v := range params {
			e.Params[k] = v
		}
	}
	return e
}

// WithCause 把底层错误并进消息里，码不变——界面认的是码，不是这句尾巴。
func (e *Error) WithCause(cause error) *Error {
	if cause != nil {
		e.Msg = fmt.Sprintf("%s（%v）", e.Msg, cause)
	}
	return e
}

// CodeOf 取一条错误的码；不是带码错误时返回 CodeUnknown，绝不返回空串。
// 调用方（ipc 那侧）靠它填 Response.Code，不许靠读文案前缀猜类型。
func CodeOf(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) && e.Code != "" {
		return e.Code
	}
	// 没包成 *Error 的历史形状（比如 ipc 里那几条 fmt.Errorf）：码就是冒号前那一段。
	head, _, found := strings.Cut(err.Error(), ":")
	if found && isCode(head) {
		return head
	}
	return CodeUnknown
}

// ParamsOf 取一条错误的参数；没有参数时返回空表，调用方不必判空。
func ParamsOf(err error) map[string]any {
	var e *Error
	if errors.As(err, &e) && e.Params != nil {
		out := make(map[string]any, len(e.Params))
		for k, v := range e.Params {
			out[k] = v
		}
		return out
	}
	return map[string]any{}
}

// ModeCode 拼一条方式专属的码：GAC-MODE-<方式id>-<三位数>。
// 这是唯一允许"现编"的形状，为的是 R6 那句「加一种解析方式只加一个 xxx.go、不动其他任何文件」——
// 若新方式必须来这里登记一个码，R6 当场破。除 resolve 外别的包不许用它（门禁 G11 管）。
func ModeCode(modeID string, n int) string {
	return fmt.Sprintf("GAC-MODE-%s-%03d", strings.ToLower(modeID), n)
}

// isCode 判一段文本像不像本项目的码：GAC-<一到七个大写字母> 开头，且整段不含空格。
func isCode(s string) bool {
	if !strings.HasPrefix(s, "GAC-") || strings.ContainsAny(s, " ") {
		return false
	}
	body := strings.TrimPrefix(s, "GAC-")
	for _, r := range body {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r >= 'a' && r <= 'z') {
			return false
		}
	}
	return len(body) >= 4
}
