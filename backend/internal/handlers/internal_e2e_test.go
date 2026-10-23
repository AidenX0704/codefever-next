package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/db"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// 演示数据（与 backend/contract/manifest.yaml 的 demo 段一致）。
const (
	e2eUserKey  = "00000000000000000000000000000000"
	e2eUserMail = "root@codefever.cn"
	e2eUserPass = "123456"
	e2eRepoKey  = "1031c9342d6539fd36ff79f75ca4f8b7"
	e2eRepoPath = "de7347a84f6236f06b0356ce6668c27d"
)

// newInternalE2EServer 装配一个连真实数据库的 internal 域服务器。
//
// 数据库不可用时直接跳过 —— 这些用例是"有库才有意义"的集成验证。
func newInternalE2EServer(t *testing.T) (*httpapi.Server, *config.Config) {
	t.Helper()

	baseDir, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("解析应用根目录失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "env.yaml")); err != nil {
		t.Skipf("找不到 env.yaml（应用根目录 %s），跳过集成用例", baseDir)
	}

	cfg, err := config.Load(baseDir)
	if err != nil {
		t.Skipf("加载配置失败: %v", err)
	}

	sqlDB, err := db.Open(cfg)
	if err != nil {
		t.Skipf("连接数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Ping(); err != nil {
		t.Skipf("数据库不可用: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httpapi.New(cfg, nil, repo.NewUserRepo(sqlDB), logger)
	NewInternalHandler(
		cfg,
		NewGuard(repo.NewAccessRepo(sqlDB)),
		repo.NewInternalRepo(sqlDB),
		repo.NewUserRepo(sqlDB),
	).Register(srv)
	return srv, cfg
}

// doE2ERequest 发一条带 JSON body 的内部网关请求。
func doE2ERequest(t *testing.T, srv *httpapi.Server, token, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", token)
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

// TestE2EAuthHTTPSuccess 校验 authHTTP 的成功响应。
//
// 对齐 PHP 基线 backend/testdata/contract/internal.authHTTP.json：
//
//	{"storage":"http://127.0.0.1:27555",
//	 "uid":"<u_key>|<r_key>",
//	 "user":"root","addr":"codefever.cn",
//	 "path":"<git-storage>/de7347a84f6236f06b0356ce6668c27d"}
//
// uid 的 "<u_key>|<r_key>" 是跨进程硬契约。
func TestE2EAuthHTTPSuccess(t *testing.T) {
	srv, cfg := newInternalE2EServer(t)

	body := `{"user":"` + e2eUserMail + `","pass":"` + e2eUserPass + `",` +
		`"repo":"/rootteam/demorepo.git","action":"git-upload-pack"}`
	status, envelope := doE2ERequest(t, srv, cfg.Env.Gateway.Token, http.MethodGet, "/apiv1/internal/authHTTP", body)

	if status != http.StatusOK {
		t.Fatalf("HTTP 状态 = %d, 期望 200 (body: %v)", status, envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 不是对象: %#v", envelope["data"])
	}

	if got := data["storage"]; got != "http://127.0.0.1:27555" {
		t.Fatalf("storage = %v", got)
	}
	if got := data["uid"]; got != e2eUserKey+"|"+e2eRepoKey {
		t.Fatalf("uid = %v, 期望 %q", got, e2eUserKey+"|"+e2eRepoKey)
	}
	if got := data["user"]; got != "root" {
		t.Fatalf("user = %v, 期望 root", got)
	}
	if got := data["addr"]; got != "codefever.cn" {
		t.Fatalf("addr = %v, 期望 codefever.cn", got)
	}
	path, _ := data["path"].(string)
	// path = <git-storage 根目录> + r_path；r_path 自带前导斜杠，
	// 因此拼接后一定是 "…/git-storage/de7347a8…"（与 PHP 侧一致）。
	if !strings.HasSuffix(path, "git-storage/"+e2eRepoPath) {
		t.Fatalf("path = %q, 期望以 %q 结尾", path, "git-storage/"+e2eRepoPath)
	}
}

// TestE2EAuthSSHSuccess 校验 authSSH 的成功响应。
//
// 对齐 PHP 基线 backend/testdata/contract/internal.authSSH.json：
// data 只有 uid 与 path 两个键（PHP 侧不输出 action）。
func TestE2EAuthSSHSuccess(t *testing.T) {
	srv, cfg := newInternalE2EServer(t)

	// repo 带单引号 —— ssh-gateway 收到的就是这种形态。
	body := `{"userID":"` + e2eUserKey + `","repo":"'rootteam/demorepo.git'","action":"git-upload-pack"}`
	status, envelope := doE2ERequest(t, srv, cfg.Env.Gateway.Token, http.MethodGet, "/apiv1/internal/authSSH", body)

	if status != http.StatusOK {
		t.Fatalf("HTTP 状态 = %d, 期望 200 (body: %v)", status, envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 不是对象: %#v", envelope["data"])
	}
	if len(data) != 2 {
		t.Fatalf("data 的键数 = %d, 期望 2（PHP 侧不输出 action）: %#v", len(data), data)
	}
	if got := data["uid"]; got != e2eUserKey+"|"+e2eRepoKey {
		t.Fatalf("uid = %v, 期望 %q", got, e2eUserKey+"|"+e2eRepoKey)
	}
	path, _ := data["path"].(string)
	// path = <git-storage 根目录> + r_path；r_path 自带前导斜杠，
	// 因此拼接后一定是 "…/git-storage/de7347a8…"（与 PHP 侧一致）。
	if !strings.HasSuffix(path, "git-storage/"+e2eRepoPath) {
		t.Fatalf("path = %q, 期望以 %q 结尾", path, "git-storage/"+e2eRepoPath)
	}
}

// TestE2EAuthHTTPRepositoryNotFound 校验"仓库/分组不存在"返回 404 / 0x0302。
//
// 这是 Go 网关唯一会原样透出的失败码（http-gateway/Request/main.go:144-150
// 会把其余失败码改写成 401），因此必须用 404。
func TestE2EAuthHTTPRepositoryNotFound(t *testing.T) {
	srv, cfg := newInternalE2EServer(t)

	body := `{"user":"` + e2eUserMail + `","pass":"` + e2eUserPass + `",` +
		`"repo":"/rootteam/nosuchrepo.git","action":"git-upload-pack"}`
	status, envelope := doE2ERequest(t, srv, cfg.Env.Gateway.Token, http.MethodGet, "/apiv1/internal/authHTTP", body)

	if status != http.StatusNotFound {
		t.Fatalf("HTTP 状态 = %d, 期望 404 (body: %v)", status, envelope)
	}
	if got, _ := envelope["code"].(float64); int(got) != 770 {
		t.Fatalf("code = %v, 期望 770 (0x0302)", envelope["code"])
	}
}

// TestE2EAuthHTTPBadCredential 校验口令错误返回 403 / 0x0101。
func TestE2EAuthHTTPBadCredential(t *testing.T) {
	srv, cfg := newInternalE2EServer(t)

	body := `{"user":"` + e2eUserMail + `","pass":"definitely-not-the-password",` +
		`"repo":"/rootteam/demorepo.git","action":"git-upload-pack"}`
	status, envelope := doE2ERequest(t, srv, cfg.Env.Gateway.Token, http.MethodGet, "/apiv1/internal/authHTTP", body)

	if status != http.StatusForbidden {
		t.Fatalf("HTTP 状态 = %d, 期望 403 (body: %v)", status, envelope)
	}
	if got, _ := envelope["code"].(float64); int(got) != 257 {
		t.Fatalf("code = %v, 期望 257 (0x0101)", envelope["code"])
	}
}

// TestE2EAuthHTTPUnknownAction 校验未知动作返回 403 / 0x0106。
//
// PHP 侧 REPO_ACTION_MAPPING 里没有该动作 → 权限判定不通过。
func TestE2EAuthHTTPUnknownAction(t *testing.T) {
	srv, cfg := newInternalE2EServer(t)

	body := `{"user":"` + e2eUserMail + `","pass":"` + e2eUserPass + `",` +
		`"repo":"/rootteam/demorepo.git","action":"upload"}`
	status, envelope := doE2ERequest(t, srv, cfg.Env.Gateway.Token, http.MethodGet, "/apiv1/internal/authHTTP", body)

	if status != http.StatusForbidden {
		t.Fatalf("HTTP 状态 = %d, 期望 403 (body: %v)", status, envelope)
	}
	if got, _ := envelope["code"].(float64); int(got) != 262 {
		t.Fatalf("code = %v, 期望 262 (0x0106)", envelope["code"])
	}
}

// TestE2ESearchPublicKeyNotFound 校验未登记的公钥返回 403 / 0x0101。
func TestE2ESearchPublicKeyNotFound(t *testing.T) {
	srv, cfg := newInternalE2EServer(t)

	body := `{"fingerprint":"whatever","publicKey":"bm90LWEtcmVnaXN0ZXJlZC1wdWJsaWMta2V5"}`
	status, envelope := doE2ERequest(t, srv, cfg.Env.Gateway.Token, http.MethodGet, "/apiv1/internal/searchPublicKey", body)

	if status != http.StatusForbidden {
		t.Fatalf("HTTP 状态 = %d, 期望 403 (body: %v)", status, envelope)
	}
	if got, _ := envelope["code"].(float64); int(got) != 257 {
		t.Fatalf("code = %v, 期望 257 (0x0101)", envelope["code"])
	}
}
