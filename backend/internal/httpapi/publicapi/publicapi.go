// Package publicapi 实现面向外部的公开 REST API（/api/v1）。
//
// # 为什么独立于 contract.Envelope
//
// 内部 /api/* 家族的信封是 **PHP 兼容契约**：
//
//	{"request-id":"...","code":0,"message":"Success","data":{...}}
//
// 它被 123 个契约基线快照锁定，任何字段改动都会让回归网失效。
// 公开 API 面向的是外部脚本与 CI（GitHub Actions 等常见客户端），
// 它们期望的是「裸 JSON + {message,url} 错误体」这种形态。
//
// 因此这里**不复用** contract.Envelope，也不共用 contract.Parse：
// 公开 API 有自己的参数解析（标准 query / JSON body）、自己的错误码、
// 自己的分页语义。两者只是共用 repo 层与鉴权判定。
//
// # 与既有路由的隔离
//
// 既有 /api/* 由 httpapi.Server 的分发器处理，路径形如
// /api/{controller}/{action}；公开 API 形如 /api/v1/repos/{owner}/{repo}，
// 段数不定。因此：
//
//   - httpapi.normalizeAPIPath 对 /api/v1/ 前缀**豁免**段数截断；
//   - 公开 API 用自己的 ServeMux（/api/v1/ 子树）注册，不与 /api/{ctrl}/{action} 混用。
//
// 注意：/apiv1/internal（内部网关回调前缀）与 /api/v1 是两回事，不可混用。
package publicapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ErrorBody 是公开 API 的错误响应体。
//
// 错误体形态与 GitHub 等公开 API 惯例一致：
//
//	{"message":"...","url":"https://<host>/api/v1"}
//
// url 字段指向 API 文档，便于调用方自助排查。
type ErrorBody struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}

// WriteJSON 写出成功响应（裸 JSON，不带信封）。
func WriteJSON(w http.ResponseWriter, status int, body any) {
	payload, err := json.Marshal(body)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "响应序列化失败")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-cache, must-revalidate")
	h.Set("Content-Length", strconv.Itoa(len(payload)))
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

// WriteError 写出错误响应。
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorBody{Message: message, URL: docsURL})
}

// docsURL 是错误体里的文档地址。
//
// 取站点根地址即可 —— 公开 API 文档挂载在站点根的 /api/v1 说明页，
// 精确路径由部署方决定，这里给出稳定可解析的前缀。
var docsURL = "https://localhost/api/v1"

// SetDocURL 覆盖错误体里的文档地址（由 Server 在装配时按站点 host 设置）。
func SetDocURL(u string) {
	if u != "" {
		docsURL = strings.TrimSuffix(u, "/") + "/api/v1"
	}
}

// BearerToken 从 Authorization 头中提取访问令牌明文。
//
// 支持两种写法（并额外兼容 GitHub 客户端）：
//
//	Authorization: token <明文>
//	Authorization: Bearer <明文>
//
// 也支持 query 参数 ?access_token=<明文> —— 部分 CI 场景无法设置请求头。
func BearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth != "" {
		fields := strings.Fields(auth)
		if len(fields) == 2 {
			switch strings.ToLower(fields[0]) {
			case "token", "bearer":
				return fields[1]
			}
		}
	}
	return strings.TrimSpace(r.URL.Query().Get("access_token"))
}
