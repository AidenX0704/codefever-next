package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// shellHTML 是测试用的最小 SPA 壳。
const shellHTML = "<!doctype html><html><body>shell</body></html>"

// configureTestShell 把 SPA 壳指向一个临时 index.html，并在用例结束时复位。
func configureTestShell(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	index := filepath.Join(dir, "index.html")
	if err := os.WriteFile(index, []byte(shellHTML), 0o600); err != nil {
		t.Fatalf("写临时壳失败: %v", err)
	}
	ConfigureSPA(index)
	t.Cleanup(func() { ConfigureSPA("") })
}

// TestPageRouteServesSPA 断言"注册为页面路由的站内页"以 **200** 下发 SPA 壳。
//
// 为什么需要这条断言：未注册的路径走 handleFallback → web 控制器分发器 →
// "控制器不存在" → handleNotFound，最终以 **404 状态码**下发 SPA 壳
// （nopage 的等价行为）。也就是说"页面能打开"并不代表"路由注册对了"——
// 浏览器与爬虫会把它当成不存在的页面。
//
// 因此每个新增站内页面（`/help` 就是这样加的，见 cmd/codefever-api/main.go）
// 都必须在 main 里 RegisterPage + ServeSPA，由本用例锁定这条约定。
func TestPageRouteServesSPA(t *testing.T) {
	configureTestShell(t)

	srv := New(nil, nil, nil, nil)
	srv.RegisterPage("/help", func(w http.ResponseWriter, r *http.Request) {
		if !ServeSPA(w, r) {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	h := srv.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/help", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /help 状态码 = %d, 期望 200（注册页面路由时应以 200 下发壳）", rec.Code)
	}
	if got := rec.Body.String(); got != shellHTML {
		t.Errorf("GET /help 响应体 = %q, 期望壳内容", got)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("壳的 Cache-Control = %q, 期望 no-cache（壳引用带哈希的资源名）", cc)
	}

	// 对照：未注册的普通路径仍以 404 下发壳（前端 NotFound 渲染）。
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope-page", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("未注册路径状态码 = %d, 期望 404", rec.Code)
	}
}

// TestPageRouteSPAHeadHasNoBody 断言 HEAD 请求下发壳时没有响应体。
//
// ServeSPA 对 HEAD 只写响应头：监控探测不应把整份壳当正文读走。
func TestPageRouteSPAHeadHasNoBody(t *testing.T) {
	configureTestShell(t)

	srv := New(nil, nil, nil, nil)
	srv.RegisterPage("/help", func(w http.ResponseWriter, r *http.Request) {
		if !ServeSPA(w, r) {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	h := srv.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/help", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD /help 状态码 = %d, 期望 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("HEAD /help 响应体长度 = %d, 期望 0", rec.Body.Len())
	}
}
