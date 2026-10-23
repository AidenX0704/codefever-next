package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/db"
	"codefever-next/backend/internal/gitsvc"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
	"codefever-next/backend/internal/session"
)

// ---------------------------------------------------------------------------
// 契约比对测试
//
// 它把 repository 域的只读端点逐条打一遍，再与
// backend/testdata/contract/repository.*.json（对着 PHP 后端采集的基线）
// 做**解析后结构**比较 —— 与 backend/cmd/contract-compare 的判定口径一致。
//
// 需要环境变量：
//
//	CODEFEVER_BASE_DIR      应用根目录（env.yaml 所在目录），必填
//	CODEFEVER_CONTRACT_DIR 契约基线目录，默认为 <base>/backend/testdata/contract
//
// 不设置 CODEFEVER_BASE_DIR 时跳过（保证 `go test ./...` 在任何环境都能跑）。
// ---------------------------------------------------------------------------

// rootUserKey 是演示库里的 root 用户。
const rootUserKey = "00000000000000000000000000000000"

// demoRepoKey 是演示库里的目标仓库。
const demoRepoKey = "1031c9342d6539fd36ff79f75ca4f8b7"

// contractCase 是一条契约用例。
type contractCase struct {
	ID     string
	Path   string
	Params map[string]string
}

func TestRepositoryContract(t *testing.T) {
	baseDir := os.Getenv("CODEFEVER_BASE_DIR")
	if baseDir == "" {
		t.Skip("未设置 CODEFEVER_BASE_DIR，跳过契约比对")
	}
	fixtureDir := os.Getenv("CODEFEVER_CONTRACT_DIR")
	if fixtureDir == "" {
		fixtureDir = filepath.Join(baseDir, "backend", "testdata", "contract")
	}

	cfg, err := config.Load(baseDir)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	sqlDB, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer sqlDB.Close()

	users := repo.NewUserRepo(sqlDB)
	sessions := session.NewStore(sqlDB, 10*24*time.Hour)

	srv := httpapi.New(cfg, sessions, users, slog.New(slog.NewTextHandler(io.Discard, nil)))
	repos := repo.NewRepositoryRepo(sqlDB)
	gitSvc := gitsvc.NewService(cfg.Env.CLI.Git, cfg.GitStoragePath())
	defer gitSvc.Close()
	NewRepositoryHandler(repos, gitSvc, NewGuard(repo.NewAccessRepo(sqlDB)),
		WithRepositorySiteConfig(cfg)).Register(srv)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 建一个 root 会话。
	sess, err := sessions.Create(context.Background(), rootUserKey, "127.0.0.1", "contract-test")
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	cookie := cfg.Env.Session.Name + "=" + url.QueryEscape(sess.Key)

	cases := []contractCase{
		{ID: "repository.list", Path: "/api/repository/list"},
		{ID: "repository.config", Path: "/api/repository/config",
			Params: map[string]string{"rKey": demoRepoKey}},
		{ID: "repository.activities", Path: "/api/repository/activities",
			Params: map[string]string{"repository": demoRepoKey, "category": "0", "keyword": "", "page": "1", "pagesize": "20"}},
		{ID: "repository.commitList", Path: "/api/repository/commitList",
			Params: map[string]string{"repository": demoRepoKey, "revision": "master", "path": "", "keyword": "", "page": "1", "pagesize": "10"}},
		{ID: "repository.commitDetail", Path: "/api/repository/commitDetail",
			Params: map[string]string{"repository": demoRepoKey, "commitSHA": "f67cfee73d75172d4e7afd77c3efa310b460fa1b"}},
		{ID: "repository.fileChanges", Path: "/api/repository/fileChanges",
			Params: map[string]string{"repository": demoRepoKey, "original": "c943e85a4f996856f19d53e0f59d47e6aa9e29ec", "modified": "f67cfee73d75172d4e7afd77c3efa310b460fa1b"}},
		{ID: "repository.refListContainSHA", Path: "/api/repository/refListContainSHA",
			Params: map[string]string{"repository": demoRepoKey, "hash": "f67cfee73d75172d4e7afd77c3efa310b460fa1b"}},
		{ID: "repository.object", Path: "/api/repository/object",
			Params: map[string]string{"repository": demoRepoKey, "parent": "master", "path": "", "revision": "master"}},
		{ID: "repository.pathStack", Path: "/api/repository/pathStack",
			Params: map[string]string{"repository": demoRepoKey, "revision": "master", "path": "README.md"}},
		{ID: "repository.mergeRequests", Path: "/api/repository/mergeRequests",
			Params: map[string]string{"repository": demoRepoKey, "status": "0", "keyword": "", "sort": "desc", "page": "1", "pagesize": "20"}},
		{ID: "repository.relatedMergeRequests", Path: "/api/repository/relatedMergeRequests",
			Params: map[string]string{"status": "0", "keyword": "", "sort": "desc", "page": "1", "pagesize": "20"}},
		{ID: "repository.mergeRequestCommits", Path: "/api/repository/mergeRequestCommits"},
		{ID: "repository.lastCommitLog", Path: "/api/repository/lastCommitLog",
			Params: map[string]string{"repository": demoRepoKey, "ref": "master", "path": "README.md"}},
		{ID: "repository.branchList", Path: "/api/repository/branchList",
			Params: map[string]string{"repository": demoRepoKey}},
		{ID: "repository.mergeRequestVersionList", Path: "/api/repository/mergeRequestVersionList"},
		{ID: "repository.blameInfo", Path: "/api/repository/blameInfo",
			Params: map[string]string{"repository": demoRepoKey, "revision": "master", "path": "README.md"}},
		{ID: "repository.mergeRequestDetail", Path: "/api/repository/mergeRequestDetail"},
		{ID: "repository.tagList", Path: "/api/repository/tagList",
			Params: map[string]string{"repository": demoRepoKey}},
		{ID: "repository.targetRepository", Path: "/api/repository/targetRepository",
			Params: map[string]string{"repository": demoRepoKey}},
	}

	var diffs int
	for _, c := range cases {
		got := requestContract(t, ts.URL, cookie, c)
		want := readFixture(t, fixtureDir, c.ID)
		if diff := compareJSON(want, got); diff != "" {
			diffs++
			t.Errorf("[%s] 与基线不一致:\n%s", c.ID, diff)
		}
	}
	if diffs > 0 {
		t.Errorf("共 %d 个端点与基线不一致", diffs)
	}
}

// requestContract 发起一次请求并返回归一化后的响应体。
func requestContract(t *testing.T, base, cookie string, c contractCase) map[string]any {
	t.Helper()

	target := base + c.Path
	if len(c.Params) > 0 {
		q := url.Values{}
		for k, v := range c.Params {
			q.Set(k, v)
		}
		target += "?" + q.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", cookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}

	var decoded any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("[%s] 响应不是 JSON (HTTP %d): %s", c.ID, resp.StatusCode, string(raw))
	}
	m, ok := normalizeTree(decoded, "").(map[string]any)
	if !ok {
		t.Fatalf("[%s] 响应不是 JSON 对象", c.ID)
	}
	return m
}

// readFixture 读取契约基线。
func readFixture(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	path := filepath.Join(dir, id+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取基线失败: %v", err)
	}
	var rec struct {
		Body map[string]any `json:"body"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("解析基线失败: %v", err)
	}
	return rec.Body
}

// compareJSON 比较两棵 JSON 树，返回差异描述（无差异时返回空串）。
func compareJSON(want, got any) string {
	wantText, err1 := json.Marshal(want)
	gotText, err2 := json.Marshal(got)
	if err1 != nil || err2 != nil {
		return fmt.Sprintf("序列化失败: %v / %v", err1, err2)
	}
	if string(wantText) == string(gotText) {
		return ""
	}
	return fmt.Sprintf("期望: %s\n实际: %s", wantText, gotText)
}

// 归一化规则与 backend/cmd/contract-collect 一致：
// R1 request-id → <REQUEST_ID>；R2 host → <HOST>；R3 绝对路径 → <BASE>；R4 时间戳 → <TS>。
const (
	placeholderRequestID = "<REQUEST_ID>"
	placeholderHost      = "<HOST>"
	placeholderBase      = "<BASE>"
	placeholderTS        = "<TS>"
	basePathPrefix       = "/data/www/codefever-community"
	tsMin                = 1700000000
	tsMax                = 2200000000
)

func normalizeTree(v any, key string) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeTree(val, k)
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, normalizeTree(item, ""))
		}
		return out
	case string:
		switch key {
		case "request-id":
			return placeholderRequestID
		case "host":
			return placeholderHost
		}
		return normalizeBasePath(t)
	case json.Number:
		if i, err := t.Int64(); err == nil && i >= tsMin && i <= tsMax {
			return placeholderTS
		}
		return t
	default:
		return v
	}
}

func normalizeBasePath(s string) string {
	if strings.HasPrefix(s, basePathPrefix) {
		return placeholderBase + strings.TrimPrefix(s, basePathPrefix)
	}
	return s
}
