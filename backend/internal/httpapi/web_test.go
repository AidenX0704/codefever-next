package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 本文件覆盖 web 控制器命名空间的三条路由判定规则。
//
// PHP 参考：
//   - application/libraries/api_controller.php:55-63（_remap → 501）
//   - application/libraries/api_controller.php:96-108（errorResponse 信封）
//   - application/config/routes.php:42（404_override = nopage）
//   - system/core/Router.php:_validate_request（控制器文件是否存在）

// newTestWebServer 造一个只含 web 控制器的服务器。
func newTestWebServer() *Server {
	srv := New(nil, nil, nil, nil)
	srv.RegisterWebController(&WebController{
		Name: "user",
		Methods: map[string]http.HandlerFunc{
			// 刻意用混合大小写的方法名，验证注册时会统一小写化。
			"resetPassword": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=UTF-8")
				_, _ = w.Write([]byte("reset-ok"))
			},
		},
	})
	srv.RegisterWebController(&WebController{
		Name: "home",
		Methods: map[string]http.HandlerFunc{
			"index": func(w http.ResponseWriter, r *http.Request) {
				Redirect(w, "/user/login")
			},
		},
	})
	return srv
}

// TestWebMethodNameIsCaseInsensitive 断言方法名大小写不敏感。
//
// PHP 的 method_exists() 大小写不敏感：
// 实测 GET /user/resetpassword 与 /user/RESETPASSWORD 都命中 resetPassword。
func TestWebMethodNameIsCaseInsensitive(t *testing.T) {
	srv := newTestWebServer()
	h := srv.Handler()

	for _, path := range []string{
		"/user/resetPassword",
		"/user/resetpassword",
		"/user/RESETPASSWORD",
		"/user/ResetPassword",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s 状态码 = %d, 期望 200", path, rec.Code)
		}
		if got := rec.Body.String(); got != "reset-ok" {
			t.Errorf("%s 响应体 = %q, 期望 %q", path, got, "reset-ok")
		}
	}
}

// TestWebUnknownMethodReturns501 断言"控制器存在但方法不存在"返回 501。
//
// 逐字对应 API_Controller::_remap → errorResponse(501, 'Unknown method')：
// 状态码 200、Content-Type application/json、响应体 49 字节。
func TestWebUnknownMethodReturns501(t *testing.T) {
	srv := newTestWebServer()
	h := srv.Handler()

	for _, path := range []string{"/user", "/user/", "/user/nope", "/user/xyz"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusOK {
			t.Errorf("%s 状态码 = %d, 期望 200（501 只在 body 的 code 字段里）", path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("%s Content-Type = %q, 期望 application/json", path, got)
		}
		want := `{"code":501,"message":"Unknown method","data":[]}`
		if got := rec.Body.String(); got != want {
			t.Errorf("%s 响应体 = %s, 期望 %s", path, got, want)
		}
	}
}

// TestWebUnknownControllerReturns404HTML 断言"控制器不存在"返回 HTML 404 页。
//
// 逐字对应 CI 的 404_override（routes.php: nopage）：控制器文件不存在时，
// CI 回落到 nopage/index.php，输出 HTML 页面而不是 JSON。
func TestWebUnknownControllerReturns404HTML(t *testing.T) {
	srv := newTestWebServer()
	h := srv.Handler()

	for _, path := range []string{"/nope", "/nope/x", "/USER/register", "/feature"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s 状态码 = %d, 期望 404", path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=UTF-8" {
			t.Errorf("%s Content-Type = %q, 期望 text/html; charset=UTF-8", path, got)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "<!DOCTYPE html>") {
			t.Errorf("%s 响应体不是 HTML 页面: %s", path, body)
		}
		if strings.Contains(body, `"code":501`) {
			t.Errorf("%s 不应返回 501 JSON: %s", path, body)
		}
	}
}

// TestWebDefaultController 断言路径为 "/" 时走 default_controller。
//
// 逐字对应 application/config/routes.php:41（default_controller = home）
// 与 CI_Router::_set_default_controller()。
func TestWebDefaultController(t *testing.T) {
	srv := newTestWebServer()
	h := srv.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusFound {
		t.Errorf("状态码 = %d, 期望 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/user/login" {
		t.Errorf("Location = %q, 期望 /user/login", got)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("响应体应为空, 实际 = %q", body)
	}
}

// TestSplitWebPath 断言路径分段规则与 CI 的 URI::_explode_segments 一致。
//
// CI 会丢弃空段；缺第二段时方法名是 "index"。
func TestSplitWebPath(t *testing.T) {
	cases := []struct {
		path   string
		ctrl   string
		method string
		ok     bool
	}{
		{path: "/", ctrl: "", method: "", ok: false},
		{path: "/user", ctrl: "user", method: "index", ok: true},
		{path: "/user/", ctrl: "user", method: "index", ok: true},
		{path: "/user/register", ctrl: "user", method: "register", ok: true},
		{path: "/user/resetPassword", ctrl: "user", method: "resetPassword", ok: true},
		{path: "/doc/cn/", ctrl: "doc", method: "cn", ok: true},
		{path: "/doc/cn/git/clone", ctrl: "doc", method: "cn", ok: true},
		{path: "//user//register//", ctrl: "user", method: "register", ok: true},
	}
	for _, tc := range cases {
		ctrl, method, ok := splitWebPath(tc.path)
		if ctrl != tc.ctrl || method != tc.method || ok != tc.ok {
			t.Errorf("splitWebPath(%q) = (%q, %q, %v), 期望 (%q, %q, %v)",
				tc.path, ctrl, method, ok, tc.ctrl, tc.method, tc.ok)
		}
	}
}

// TestPageRouteIsReachableCaseInsensitively 断言页面路由的大小写变体也能命中。
//
// PHP 侧 /user/login 就是 User::login，method_exists 大小写不敏感，
// 因此 /user/LOGIN 同样会渲染登录页。
func TestPageRouteIsReachableCaseInsensitively(t *testing.T) {
	srv := New(nil, nil, nil, nil)
	// 真实部署里 user 控制器一定存在（login 由页面路由提供）。
	srv.RegisterWebController(&WebController{
		Name:    "user",
		Methods: map[string]http.HandlerFunc{},
	})
	srv.RegisterPage("/user/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("login-page"))
	})
	h := srv.Handler()

	for _, path := range []string{"/user/login", "/user/LOGIN", "/user/Login"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Body.String(); got != "login-page" {
			t.Errorf("%s 响应体 = %q, 期望 %q", path, got, "login-page")
		}
	}
}
