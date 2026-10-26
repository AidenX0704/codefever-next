package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codefever-next/backend/internal/httpapi/publicapi"
)

// 本文件锁定**公开 API（/api/v1）与内部 /api/* 家族不互相干扰**。
//
// 背景（这是本项目公开 API 落地时最大的一个坑）：
//
// 内部 /api/* 的路径归一化中间件 normalizeAPIPath 会把 /api/ 开头的路径
// **截断到 3 段**（复刻 CodeIgniter 的「控制器 + 动作」两段式路由）。
// 公开 API 的路径形如 /api/v1/repos/{owner}/{repo}（4 段以上），
// 若不豁免就会被砍成 /api/v1/repos，然后被当成
// controller=v1 / action=repos 分发 —— 而控制器 "v1" 不存在，
// 最终落到 HTML 404 页，**静默失败**，排查成本极高。
//
// 因此这里用单元测试把三条不变量钉死：
//  1. 公开 API 路径不被截断；
//  2. 内部 /api/* 路径仍然按原规则截断（既有契约不受影响）；
//  3. 未装配公开 API 时，/api/v1/ 不会误落到内部分发器。

// TestNormalizeAPIPathExemptsPublicAPI 断言公开 API 路径不被截断。
func TestNormalizeAPIPathExemptsPublicAPI(t *testing.T) {
	cases := []string{
		"/api/v1/user",
		"/api/v1/user/repos",
		"/api/v1/users/someone",
		"/api/v1/repos/owner/repo",
		"/api/v1/repos/owner/repo/releases",
		"/api/v1/repos/owner/repo/releases/1/assets/2",
	}
	for _, path := range cases {
		var got string
		h := normalizeAPIPath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.URL.Path
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
		if got != path {
			t.Errorf("公开 API 路径被改写：%s → %s（期望原样保留）", path, got)
		}
	}
}

// TestNormalizeAPIPathStillTruncatesInternal 断言内部 /api/* 的截断规则不变。
//
// 既有契约要求 /api/user/info/ 归一化成 /api/user/info（否则会落到 HTML 404），
// 而 4 段以上的 /api/... 会被砍到 3 段。这条行为必须保持不变 ——
// 公开 API 的引入不能改动内部路由语义。
func TestNormalizeAPIPathStillTruncatesInternal(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/api/user/info/", "/api/user/info"},
		{"/api/user/info", "/api/user/info"},
		{"/api//user//info", "/api/user/info"},
		{"/api/a/b/c/d", "/api/a/b"},
	}
	for _, c := range cases {
		var got string
		h := normalizeAPIPath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.URL.Path
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, c.in, nil))
		if got != c.want {
			t.Errorf("normalizeAPIPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestIsPublicAPIPath 锁定公开 API 路径判定。
func TestIsPublicAPIPath(t *testing.T) {
	cases := map[string]bool{
		"/api/v1":                true,
		"/api/v1/":               true,
		"/api/v1/user":           true,
		"/api/v1/repos/o/r":      true,
		"/api/user/info":         false,
		"/api/v1x/user":          false,
		"/apiv1/internal/action": false,
		"/api":                   false,
		"/user/login":            false,
	}
	for path, want := range cases {
		if got := publicapi.IsPublicAPIPath(path); got != want {
			t.Errorf("IsPublicAPIPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestPublicAPIRouteNotShadowedByInternalDispatcher 断言公开 API 路径
// 不会被内部 /api/{ctrl}/{action} 分发器截胡。
//
// 这是本次改动的核心回归网：一旦有人把豁免逻辑删掉，
// 这个用例会立刻失败 —— 公开 API 会静默变成 HTML 404 页。
//
// 判别方式用「响应体形态」而不是状态码：公开 API 对不存在的仓库
// 本来就返回 404（JSON 错误体），而落到内部兜底时返回的是 HTML 404 页。
func TestPublicAPIRouteNotShadowedByInternalDispatcher(t *testing.T) {
	srv := New(nil, nil, nil, nil)

	// 装配一个假的公开 API 路由：只关心"请求有没有到达它"，
	// 不关心真实业务逻辑（那需要数据库）。
	srv.publicRouter = publicapi.NewRouter(publicapi.NewServer(
		nil, nil, nil, nil, nil, "http://localhost"))

	h := srv.Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/repos/owner/repo", nil)
	h.ServeHTTP(rec, req)

	// 响应必须是公开 API 的 JSON 错误体，而不是 HTML 404 页。
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("公开 API 响应 Content-Type = %q, want application/json; charset=utf-8"+
			"（落到内部兜底会是 text/html）", ct)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, `{"message":`) {
		t.Errorf("公开 API 响应体 = %s, want {\"message\":...} 形态（说明落到了内部兜底）", body)
	}
}
