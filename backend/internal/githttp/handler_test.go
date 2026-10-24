package githttp

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/gateway"
)

// newHandler 构造一个鉴权指向测试服务器的 Handler。
func newHandler(t *testing.T, authURL string) *Handler {
	t.Helper()
	client := gateway.NewClient(&config.Env{
		Gateway: config.Gateway{
			Token:    "t",
			AuthHTTP: authURL,
			AuthSSH:  authURL,
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &Handler{
		Auth:     client,
		CGIPath:  "/nonexistent/git-http-backend",
		RepoRoot: t.TempDir(),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// TestRejectsNonGitUserAgent 验证 UA 缺失时返回 405 而不是 panic。
func TestRejectsNonGitUserAgent(t *testing.T) {
	h := newHandler(t, "http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodGet, "/g/r.git/info/refs", nil)
	req.Header.Del("User-Agent")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("状态码 = %d, 期望 405", rec.Code)
	}
}

// TestRequiresBasicAuth 验证缺少 Basic Auth 时返回 401 并带 challenge。
func TestRequiresBasicAuth(t *testing.T) {
	h := newHandler(t, "http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodGet, "/g/r.git/info/refs", nil)
	req.Header.Set("User-Agent", "git/2.43.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("状态码 = %d, 期望 401", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("WWW-Authenticate"), "Basic") {
		t.Fatalf("缺少 WWW-Authenticate: %q", rec.Header().Get("WWW-Authenticate"))
	}
}

// TestAuthNotFoundPreserved 验证后端 404 被保留为 404。
func TestAuthNotFoundPreserved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":770,"message":"Repository Not Found","data":[]}`))
	}))
	defer srv.Close()

	h := newHandler(t, srv.URL)
	req := httptest.NewRequest(http.MethodGet, "/g/r.git/info/refs", nil)
	req.Header.Set("User-Agent", "git/2.43.0")
	req.SetBasicAuth("u@example.com", "p")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404", rec.Code)
	}
}

// TestAuthFailureBecomes401 验证除 404 外的失败一律改写成 401（旧语义）。
func TestAuthFailureBecomes401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":257,"message":"Invalid Login Credential","data":[]}`))
	}))
	defer srv.Close()

	h := newHandler(t, srv.URL)
	req := httptest.NewRequest(http.MethodGet, "/g/r.git/info/refs", nil)
	req.Header.Set("User-Agent", "git/2.43.0")
	req.SetBasicAuth("u@example.com", "p")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("状态码 = %d, 期望 401", rec.Code)
	}
}

// TestInternalFormRequiresUserAtHost 验证内部调用形态的用户名格式。
func TestInternalFormRequiresUserAtHost(t *testing.T) {
	h := newHandler(t, "http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodGet, "/info/refs", nil)
	req.Header.Set("User-Agent", "git/2.43.0")
	req.SetBasicAuth("no-at-sign", "uid")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404", rec.Code)
	}
}

// TestNonRepoPathRejected 确认旧的"内部模式"鉴权旁路已删除。
func TestNonRepoPathRejected(t *testing.T) {
	h := newHandler(t, "http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodGet, "/0123456789abcdef/info/refs", nil)
	req.Header.Set("User-Agent", "git/2.43.0")
	req.SetBasicAuth("a@b", "uid")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404", rec.Code)
	}
}

// TestReceivePackActionDerivedFromPath 确认 POST /git-receive-pack 会请求
// PUSH 权限，而不是退化成 READ。
func TestReceivePackActionDerivedFromPath(t *testing.T) {
	var got gateway.AuthHTTPRequest
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		_, _ = w.Write([]byte(`{"code":0,"message":"Success","data":{"uid":"u|r","user":"u","addr":"e","path":"` + t.TempDir() + `"}}`))
	}))
	defer authSrv.Close()

	h := newHandler(t, authSrv.URL)
	req := httptest.NewRequest(http.MethodPost, "/g/r.git/git-receive-pack", strings.NewReader("pack"))
	req.Header.Set("User-Agent", "git/2.43.0")
	req.SetBasicAuth("u@example.com", "p")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got.Action != "git-receive-pack" {
		t.Fatalf("action = %q, 期望 git-receive-pack", got.Action)
	}
}
