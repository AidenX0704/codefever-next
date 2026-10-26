package publicapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestRouter 造一个只装了路由的公开 API（依赖全空）。
//
// 空依赖足以覆盖路由层的行为：需要令牌的端点会因"令牌服务未启用"回 401，
// 公开端点会因仓储为空回 404 —— 两者都不影响"方法是否被放行""路径是否命中"的判定。
func newTestRouter() *Router {
	return NewRouter(NewServer(nil, nil, nil, nil, nil, ""))
}

// assertJSONError 断言响应是公开 API 的错误体形态（`{message,url}`）。
func assertJSONError(t *testing.T, rec *httptest.ResponseRecorder) ErrorBody {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json（公开 API 的错误体必须是 JSON）", ct)
	}
	var body ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是合法 JSON: %q (%v)", rec.Body.String(), err)
	}
	if body.Message == "" || body.URL == "" {
		t.Errorf("错误体缺少字段: %+v", body)
	}
	return body
}

// TestRouterRejectsNonGet 锁定"公开 API 只接受 GET / HEAD"这条契约。
//
// 此前路由是纯路径模式，方法完全不设限：带合法令牌的 POST /api/v1/user 会正常
// 返回 200 与数据，而文档写的是 GET —— 同一件事有两种说法。
func TestRouterRejectsNonGet(t *testing.T) {
	r := newTestRouter()

	for _, method := range []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	} {
		t.Run(method, func(t *testing.T) {
			for _, path := range []string{
				"/api/v1/user",
				"/api/v1/user/repos",
				"/api/v1/users/someone",
				"/api/v1/repos/dev/hello",
			} {
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, httptest.NewRequest(method, path, nil))

				if rec.Code != http.StatusMethodNotAllowed {
					t.Errorf("%s %s 状态码 = %d, want 405", method, path, rec.Code)
				}
				// 方法错误不能先撞上鉴权：否则调用方会误以为是凭据问题。
				body := assertJSONError(t, rec)
				if strings.Contains(body.Message, "令牌") {
					t.Errorf("%s %s 的 405 走成了鉴权错误: %q", method, path, body.Message)
				}
				if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
					t.Errorf("%s %s 的 Allow = %q, want %q", method, path, allow, "GET, HEAD")
				}
			}
		})
	}
}

// TestRouterAllowsHead 锁定 HEAD 被放行到 handler（探活脚本常用）。
//
// 放行的证据：HEAD 走到 requireUser，因依赖为空返回 401，而不是 405。
func TestRouterAllowsHead(t *testing.T) {
	r := newTestRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/api/v1/user", nil))
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("HEAD 被 405 拦掉了，want 放行")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("HEAD 状态码 = %d, want 401（已穿过方法闸门，撞上鉴权）", rec.Code)
	}
}

// TestRouterUnknownPathReturnsJSON404 锁定"未命中的公开 API 路径回 JSON 404"。
//
// Go 的 ServeMux 默认回 text/plain 的 "404 page not found"，调用方按
// {message,url} 解析会失败；而"路径拼错"是最常见的调用错误，必须可解析。
func TestRouterUnknownPathReturnsJSON404(t *testing.T) {
	r := newTestRouter()

	for _, path := range []string{
		"/api/v1",         // 命名空间根
		"/api/v1/",        // 带尾斜杠的根
		"/api/v1/nope",    // 不存在的端点
		"/api/v1/user/xy", // 端点前缀多一段
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s 状态码 = %d, want 404", path, rec.Code)
		}
		assertJSONError(t, rec)
	}
}
