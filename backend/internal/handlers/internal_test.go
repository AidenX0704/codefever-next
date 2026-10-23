package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// internalTestToken 是测试用的网关令牌。
const internalTestToken = "test-internal-gateway-token"

// newInternalTestServer 装配一个只注册了 internal 域的测试服务器。
//
// 这些用例只走"不碰数据库"的分支（缺参、鉴权失败、动作不存在），
// 因此仓储可以挂在一个空连接上。
func newInternalTestServer(t *testing.T) *httpapi.Server {
	t.Helper()
	cfg := &config.Config{
		BaseDir: t.TempDir(),
		Env: config.Env{
			Path:    config.Path{Git: "git-storage"},
			Gateway: config.Gateway{Token: internalTestToken},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httpapi.New(cfg, nil, nil, logger)
	NewInternalHandler(
		cfg,
		NewGuard(repo.NewAccessRepo(nil)),
		repo.NewInternalRepo(nil),
		repo.NewUserRepo(nil),
	).Register(srv)
	return srv
}

// doInternalRequest 发一条内部网关请求并解析响应信封。
func doInternalRequest(t *testing.T, srv *httpapi.Server, method, path, body, token string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", token)
	// contract.Parse 会校验 Content-Length 与实际长度是否一致，
	// 而 httptest.NewRequest 只设置 Request.ContentLength 字段，不设置该头。
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	raw := rec.Body.String()
	var envelope map[string]any
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("响应不是 JSON (HTTP %d): %s", rec.Code, raw)
	}
	return rec.Code, envelope
}

// assertEnvelope 校验信封的状态码、业务码、消息与 data。
func assertEnvelope(t *testing.T, status int, envelope map[string]any, wantStatus, wantCode int, wantMessage string, wantData any) {
	t.Helper()
	if status != wantStatus {
		t.Fatalf("HTTP 状态 = %d, 期望 %d (body: %v)", status, wantStatus, envelope)
	}
	if got, ok := envelope["code"].(float64); !ok || int(got) != wantCode {
		t.Fatalf("code = %v, 期望 %d", envelope["code"], wantCode)
	}
	if got := envelope["message"]; got != wantMessage {
		t.Fatalf("message = %v, 期望 %q", got, wantMessage)
	}
	if got := envelope["data"]; !jsonEqual(got, wantData) {
		t.Fatalf("data = %#v, 期望 %#v", got, wantData)
	}
}

// jsonEqual 比较两个 JSON 值（走一次归一化，忽略数字类型差异）。
func jsonEqual(a, b any) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ab) == string(bb)
}

// TestInternalMissingParameters 对齐 PHP 契约基线里的"缺参"路径。
//
// 基准：backend/testdata/contract/internal.*.json
//
//	internal.authHTTP        → 400 / 0x0201 / data=[]
//	internal.authSSH         → 400 / 0x0201 / data=[]
//	internal.hooks           → 400 / 0x0201 / data={"remote":""}
//	internal.searchPublicKey → 400 / 0x0201 / data=[]
func TestInternalMissingParameters(t *testing.T) {
	srv := newInternalTestServer(t)

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   int
		wantData   any
	}{
		{
			name:       "authHTTP 缺参",
			method:     http.MethodGet,
			path:       "/apiv1/internal/authHTTP",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   513,
			wantData:   []any{},
		},
		{
			name:       "authHTTP 缺 repo",
			method:     http.MethodGet,
			path:       "/apiv1/internal/authHTTP",
			body:       `{"user":"root@codefever.cn","pass":"123456"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   513,
			wantData:   []any{},
		},
		{
			name:       "authSSH 缺参",
			method:     http.MethodGet,
			path:       "/apiv1/internal/authSSH",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   513,
			wantData:   []any{},
		},
		{
			name:       "authSSH 缺 action",
			method:     http.MethodGet,
			path:       "/apiv1/internal/authSSH",
			body:       `{"userID":"00000000000000000000000000000000","repo":"rootteam/demorepo.git"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   513,
			wantData:   []any{},
		},
		{
			name:       "hooks 缺 args",
			method:     http.MethodPost,
			path:       "/apiv1/internal/hooks",
			body:       `{"args":[]}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   513,
			wantData:   map[string]any{"remote": ""},
		},
		{
			name:       "hooks 空 body",
			method:     http.MethodPost,
			path:       "/apiv1/internal/hooks",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   513,
			wantData:   map[string]any{"remote": ""},
		},
		{
			name:       "searchPublicKey 缺参",
			method:     http.MethodGet,
			path:       "/apiv1/internal/searchPublicKey",
			body:       `{"fingerprint":"","publicKey":""}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   513,
			wantData:   []any{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, envelope := doInternalRequest(t, srv, tc.method, tc.path, tc.body, internalTestToken)
			assertEnvelope(t, status, envelope, tc.wantStatus, tc.wantCode, "Missing Parameters", tc.wantData)
		})
	}
}

// TestInternalBadToken 校验网关令牌错误时返回 0x0101。
//
// 逐字对应 APIAuth.php:47 的 Response::reject(0x0101)。
func TestInternalBadToken(t *testing.T) {
	srv := newInternalTestServer(t)
	status, envelope := doInternalRequest(t, srv, http.MethodGet, "/apiv1/internal/authHTTP", `{}`, "wrong-token")
	assertEnvelope(t, status, envelope, http.StatusForbidden, 257, "Invalid Login Credential", []any{})
}

// TestInternalUnknownAction 校验动作不存在时返回 0x0105。
//
// 逐字对应 application/controllers/apiv1/base.php:29 的 Response::reject(0x0105)。
func TestInternalUnknownAction(t *testing.T) {
	srv := newInternalTestServer(t)
	status, envelope := doInternalRequest(t, srv, http.MethodGet, "/apiv1/internal/nope", `{}`, internalTestToken)
	assertEnvelope(t, status, envelope, http.StatusNotFound, 261,
		"Endpoint Not Found Or Not Allowed Current Request Method", []any{})
}

// TestInternalMethodMismatch 校验方法不匹配时返回 0x0105。
//
// PHP 侧的动作名是 "<action>_<method>" 形式，方法不对就没有对应方法。
func TestInternalMethodMismatch(t *testing.T) {
	srv := newInternalTestServer(t)
	status, envelope := doInternalRequest(t, srv, http.MethodPost, "/apiv1/internal/authHTTP", `{}`, internalTestToken)
	assertEnvelope(t, status, envelope, http.StatusNotFound, 261,
		"Endpoint Not Found Or Not Allowed Current Request Method", []any{})
}

// TestAuthHTTPResultFieldOrder 锁定 authHTTP 响应体的键顺序。
//
// PHP 的 json_encode 按数组插入顺序输出键，Go 的 encoding/json 按结构体
// 字段顺序输出，两者必须一致才能得到逐字相同的响应体。
func TestAuthHTTPResultFieldOrder(t *testing.T) {
	raw, err := json.Marshal(authHTTPResult{
		Storage: "http://127.0.0.1:27555",
		UID:     "u|r",
		User:    "root",
		Addr:    "codefever.cn",
		Path:    "/git-storage/de7347",
	})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	want := `{"storage":"http://127.0.0.1:27555","uid":"u|r","user":"root","addr":"codefever.cn","path":"/git-storage/de7347"}`
	if string(raw) != want {
		t.Fatalf("响应体 = %s, 期望 %s", raw, want)
	}
}

// TestAuthSSHResultFieldOrder 锁定 authSSH 响应体的键顺序。
//
// 注意 PHP 侧**不输出** action 键。
func TestAuthSSHResultFieldOrder(t *testing.T) {
	raw, err := json.Marshal(authSSHResult{UID: "u|r", Path: "/git-storage/de7347"})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	want := `{"uid":"u|r","path":"/git-storage/de7347"}`
	if string(raw) != want {
		t.Fatalf("响应体 = %s, 期望 %s", raw, want)
	}
}

// TestProtectedBranchRemoteText 锁定保护分支拒绝提示的逐字文本。
//
// 这段文本会被 misc/hooks/main 原样打印给 git 客户端。
func TestProtectedBranchRemoteText(t *testing.T) {
	got := sprintfProtectedBranchRemote("master")
	want := "\n[CodeFever Next]: You are not allowed to push code to protected branch <master> !"
	if got != want {
		t.Fatalf("提示文本 = %q, 期望 %q", got, want)
	}
}

// TestSplitEmail 覆盖邮箱拆分。
func TestSplitEmail(t *testing.T) {
	cases := []struct {
		email string
		name  string
		addr  string
	}{
		{email: "root@codefever.cn", name: "root", addr: "codefever.cn"},
		{email: "a@b@c", name: "a", addr: "b"},
		{email: "noatsign", name: "", addr: ""},
		{email: "@codefever.cn", name: "", addr: "codefever.cn"},
		{email: "root@", name: "root", addr: ""},
	}
	for _, tc := range cases {
		name, addr := splitEmail(tc.email)
		if name != tc.name || addr != tc.addr {
			t.Fatalf("splitEmail(%q) = (%q, %q), 期望 (%q, %q)", tc.email, name, addr, tc.name, tc.addr)
		}
	}
}
