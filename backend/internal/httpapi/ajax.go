package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
)

// 本文件承接表单 AJAX 的响应信封（原 pages.go 的一部分）。
//
// 2026-10 前端重写删除了 SSR 模板与 pages.go，但登录/注册/二次验证/找回密码这些
// **表单 POST** 仍然返回 PHP 风格的 AJAX 信封（与 /api/* 的通用信封不同），
// 因此把这部分从 pages.go 拆出来单独保留。

// AjaxEnvelope 是登录/注册/重置密码等表单 AJAX 的响应信封。
//
// 逐字对应 PHP 侧 Base::_ajax_message()：
//
//	{"code":0,"message":"登录成功","extra":{"redirect":"/repositories"}}
//
// 与 API 通用信封 contract.Envelope 不同：这里**没有** request-id，也**没有** data。
type AjaxEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Extra   any    `json:"extra"`
}

// NewAjaxEnvelope 构造 AJAX 信封。
//
// extra 为 nil 时用空数组填充，以复刻 PHP 中 json_encode(array()) 的输出 ——
// 它是 [] 而不是 {}，前端依赖这一点。
func NewAjaxEnvelope(code int, message string, extra any) *AjaxEnvelope {
	if extra == nil {
		extra = []any{}
	}
	return &AjaxEnvelope{Code: code, Message: message, Extra: extra}
}

// WriteAjaxJSON 写出 AJAX 信封响应。
//
// 响应头与 PHP 侧保持一致：PHP 的 _ajax_message() 只是 echo，没有设置任何响应头，
// 因此 Content-Type 是 text/html; charset=UTF-8（**不是** application/json）；
// 响应体用 PHP 风格的 JSON 编码（非 ASCII 转 \uXXXX、斜杠转 "\/"）。
func WriteAjaxJSON(w http.ResponseWriter, env *AjaxEnvelope) error {
	payload, err := env.Encode()
	if err != nil {
		return fmt.Errorf("序列化 AJAX 信封失败: %w", err)
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=UTF-8")
	h.Set("Content-Length", strconv.Itoa(len(payload)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(payload))
	return nil
}

// Encode 按 PHP 的 json_encode 规则序列化信封（字段顺序 code → message → extra）。
func (e *AjaxEnvelope) Encode() (string, error) {
	extra, err := phpJSONEncode(e.Extra)
	if err != nil {
		return "", err
	}
	return `{"code":` + strconv.Itoa(e.Code) +
		`,"message":` + phpJSONString(e.Message) +
		`,"extra":` + extra + `}`, nil
}
