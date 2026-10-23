package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/httpapi"
)

// repoDocDir 是仓库根目录下的 doc/ 目录（相对本包目录）。
//
// 仓库根目录 = backend/internal/handlers → ../../..
const repoDocDir = "../../../doc"

// TestDocHandlerRendersRepoContent 用仓库里真实的 doc/ 目录验证文档页。
//
// 容器里没有部署 doc/ 目录，所以线上比对时 /doc/cn/ 只会看到 404；
// 但实现必须能按仓库内容正确工作 —— 这里直接用仓库里的 doc/zh-cn 验证：
//   - GET /doc/cn/  → 渲染文档详情页（左侧菜单来自 doc/zh-cn/index.md）；
//   - GET /doc/cn/git/clone → 渲染对应的 Markdown。
func TestDocHandlerRendersRepoContent(t *testing.T) {
	if _, err := os.Stat(filepath.Join(repoDocDir, "zh-cn", "index.md")); err != nil {
		t.Skipf("仓库里没有 %s/zh-cn/index.md: %v", repoDocDir, err)
	}

	base, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("解析仓库根目录失败: %v", err)
	}
	h := NewDocHandler(&config.Config{BaseDir: base})

	t.Run("目录页重定向到首个文档", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/doc/cn/", nil)
		h.RegisterWeb(newTestServer())
		h.cn(rec, req)

		// 逐字对应 doc.php:85-112：目录按 index.md 的第一条菜单逐级下钻，
		// 最终 302 到第一个 .md 文档（doc/zh-cn/start/welcome.md）。
		if rec.Code != http.StatusFound {
			t.Fatalf("状态码 = %d, 期望 302", rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "/doc/cn/start/welcome.md" {
			t.Errorf("Location = %q, 期望 /doc/cn/start/welcome.md", got)
		}
	})

	t.Run("详情页", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/doc/cn/start/welcome.md", nil)
		h.cn(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("状态码 = %d, 期望 200", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "var doc = `") {
			t.Errorf("详情页缺少文档正文: %s", body)
		}
		// 正文里的反引号被 PHP 侧转义成 \`（doc.php:117）。
		if !strings.Contains(body, "欢迎使用 CodeFever Next") {
			t.Errorf("详情页正文不正确: %s", body)
		}
	})

	t.Run("目录穿越被拒绝", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/doc/cn/../../etc/passwd", nil)
		h.cn(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码 = %d, 期望 404", rec.Code)
		}
	})
}

// TestDocRedirectFollowsPHPConstructor 断言 Doc 构造函数的路径重定向。
//
// 逐字对应 application/controllers/doc.php:9-22：
// 没有扩展名且路径不以 '/' 结尾 → 302 "<path>/"。
func TestDocRedirectFollowsPHPConstructor(t *testing.T) {
	base, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("解析仓库根目录失败: %v", err)
	}
	h := NewDocHandler(&config.Config{BaseDir: base})

	cases := []struct {
		path     string
		location string
	}{
		{path: "/doc", location: "/doc/cn/"},
		{path: "/doc/", location: "/doc/cn/"},
		{path: "/doc/xx", location: "/doc/xx/"},
		{path: "/doc/cn", location: "/doc/cn/"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if !h.docRedirect(rec, req) {
			t.Errorf("%s 应当被重定向", tc.path)
			continue
		}
		if rec.Code != http.StatusFound {
			t.Errorf("%s 状态码 = %d, 期望 302", tc.path, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != tc.location {
			t.Errorf("%s Location = %q, 期望 %q", tc.path, got, tc.location)
		}
	}
}

// newTestServer 造一个只用于注册控制器的空服务器。
func newTestServer() *httpapi.Server {
	return httpapi.New(&config.Config{}, nil, nil, nil)
}
