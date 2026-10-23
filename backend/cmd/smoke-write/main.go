// Command smoke-write 验证**写端点的成功路径**。
//
// 为什么需要它：契约基线（contract-collect）按设计**不写业务数据** ——
// 所有写端点只覆盖"缺参 / 资源不存在"这类失败分支（0x0201、0x0106）。
// 因此"建组 / 建库 / 建分支 / 建标签 / 建合并请求 / 加成员 / 加 webhook /
// 管理端建用户与改配置"这些**成功路径**在基线里是空白。
//
// 本工具驱动真实 HTTP API，逐个走成功路径并断言结果，
// 最后把创建出来的数据全部清理干净。
//
// ⚠️ 必须跑在**隔离环境**上（独立的数据库 + 独立的 git-storage），
//
//	它会真的创建仓库、用户与配置。
//
// 用法：
//
//	smoke-write -base http://127.0.0.1:9004 -email root@codefever.cn -password 123456 \
//	           -dsn "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true" \
//	           -gitstorage /path/to/git-storage
package main

import (
	"bytes"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type client struct {
	base string
	http *http.Client
}

func main() {
	var (
		base       = flag.String("base", "http://127.0.0.1:9004", "后端地址")
		email      = flag.String("email", "root@codefever.cn", "登录邮箱")
		password   = flag.String("password", "123456", "登录口令")
		dsn        = flag.String("dsn", "", "可选：数据库 DSN，用于校验事件入队与数据落库")
		gitStorage = flag.String("gitstorage", "", "可选：git-storage 根目录，用于校验裸仓库是否真的建出")
		gitBin     = flag.String("git", "git", "git 可执行文件路径")
		keep       = flag.Bool("keep", false, "保留创建出来的数据（调试用）")
	)
	flag.Parse()

	jar, _ := cookiejar.New(nil)
	c := &client{base: strings.TrimRight(*base, "/"), http: &http.Client{Jar: jar, Timeout: 60 * time.Second}}

	var db *sql.DB
	if *dsn != "" {
		var err error
		db, err = sql.Open("mysql", *dsn)
		if err != nil {
			fail("打开数据库失败: %v", err)
		}
		defer db.Close()
	}

	r := &runner{c: c, db: db, gitStorage: *gitStorage, gitBin: *gitBin, keep: *keep}
	r.run(*email, *password)

	fmt.Println()
	fmt.Printf("结果: 通过 %d / 失败 %d\n", r.passed, r.failed)
	if r.failed > 0 {
		os.Exit(1)
	}
}

type runner struct {
	c          *client
	db         *sql.DB
	gitStorage string
	gitBin     string
	keep       bool
	passed     int
	failed     int

	createdGroup    string
	createdGroupKey string
	createdRepo     string
	createdUser     string
	createdWebhook  string
}

func (r *runner) run(email, password string) {
	fmt.Println("=== 写端点成功路径回归 ===")

	// 1) 登录
	body, code := r.postForm("/user/login", map[string]string{"email": email, "password": password})
	r.check("登录", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))

	// 1b) 回滚安全断言：登录**不得**改写库里的口令哈希。
	//
	// PHP 后端只会用 md5(md5($password)) 校验口令。Go 若在登录时把哈希
	// 透明升级成 argon2id，该账号就无法再在 PHP 侧登录 —— "秒级回滚"随之失效。
	// 默认（legacy 格式）下必须保持原样。
	if r.db != nil {
		var stored string
		err := r.db.QueryRow("SELECT u_password FROM cc_users WHERE u_email = ?", email).Scan(&stored)
		want := legacyHash(password)
		r.check("登录未改写口令哈希（回滚安全）",
			err == nil && stored == want,
			fmt.Sprintf("stored=%s want=%s err=%v", trunc(stored), want, err))
	}

	// 2) 建组
	groupName := "smokegrp" + shortRand()
	body, code = r.postForm("/api/group/create", map[string]string{
		"displayName": "Smoke Group " + groupName,
		"name":        groupName,
		"type":        "1",
		"description": "smoke write regression",
	})
	r.check("建组 group.create", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))
	// 后续接口要的是**主键**（g_key / r_key），不是名字。
	// 建组响应里 id 即 g_key。
	r.createdGroupKey = r.pick(body, "id")
	r.createdGroup = r.pick(body, "name")

	// 查组（确认可读）
	body, code = r.get("/api/group/config?gKey=" + url.QueryEscape(r.createdGroupKey))
	r.check("查组 group.config", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))

	// 3) 建库
	repoName := "smokerepo" + shortRand()
	body, code = r.postForm("/api/repository/create", map[string]string{
		"displayName": "Smoke Repo " + repoName,
		"name":        repoName,
		"group":       r.createdGroupKey,
		"description": "smoke write regression",
	})
	r.check("建库 repository.create", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))
	// 后续接口要的是 r_key（建库响应里的 id）。
	r.createdRepo = r.pick(body, "id")

	// 建库后仓库是空的（没有 master 分支），
	// 因此先推一个初始提交，后续的建分支/建标签/建合并请求才有基线。
	// 这一步用 git CLI 完成，等价于用户第一次 push。
	r.seedInitialCommit(r.createdRepo, "initial commit for smoke regression")

	// 4) 建分支（PHP 侧此处逻辑反转导致"成功反而报 0x040C"，新实现应成功）
	//
	// 分支名**先确定再调用**：建分支的响应是空数组，取不到名字，
	// 若事后随机补一个就会引用到不存在的分支。
	branch := "feature-smoke-" + shortRand()
	body, code = r.postForm("/api/repository/createBranch", map[string]string{
		"repository": r.createdRepo,
		"name":       branch,
		"origin":     "master",
	})
	r.check("建分支 createBranch（修复点）", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))

	// 合并请求要求源分支相对目标分支**有新提交**（PHP 同语义，否则 0x040C），
	// 因此给分支再推一个提交。
	r.pushCommitToBranch(r.createdRepo, branch, "commit on branch for smoke regression")

	// 5) 建标签
	body, code = r.postForm("/api/repository/createTag", map[string]string{
		"repository":  r.createdRepo,
		"name":        "v001smoke" + shortRand() + "x",
		"origin":      "master",
		"description": "smoke tag",
	})
	r.check("建标签 createTag（修复点）", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))

	// 6) 建合并请求
	body, code = r.postForm("/api/repository/createMergeRequest", map[string]string{
		"sourceRepository": r.createdRepo,
		"targetRepository": r.createdRepo,
		"sourceBranch":     branch,
		"targetBranch":     "master",
		"title":            "Smoke MR " + shortRand(),
		"description":      "smoke merge request",
	})
	r.check("建合并请求 createMergeRequest", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))

	// 7) 加成员 + 改角色（用刚建的另一个账号）
	smokeEmail := "smoke-" + shortRand() + "@codefever.cn"
	body, code = r.postForm("/api/admin/addUser", map[string]string{
		"name":  "smoke user " + shortRand(),
		"email": smokeEmail,
	})
	r.check("管理端建用户 addUser", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))
	r.createdUser = smokeEmail

	// 7b) 回滚安全断言：新账号的口令哈希必须是 PHP 侧可验证的 md5(md5(x))。
	//
	// 背景：PHP 后端只会用 md5(md5($password)) 校验口令。Go 若写成 argon2id，
	// 该账号就无法再在 PHP 侧登录 —— "秒级回滚"承诺随之失效。
	// 这条断言把该不变量钉在写操作回归里。
	if r.db != nil {
		initial := r.pick(body, "password")
		var stored string
		err := r.db.QueryRow("SELECT u_password FROM cc_users WHERE u_email = ?", smokeEmail).Scan(&stored)
		want := legacyHash(initial)
		r.check("新账号口令哈希 PHP 可验证（回滚安全）",
			err == nil && stored == want,
			fmt.Sprintf("stored=%s want=%s err=%v", trunc(stored), want, err))
	}

	body, code = r.postForm("/api/repository/addMember", map[string]string{
		"repository": r.createdRepo,
		"email":      smokeEmail,
	})
	r.check("加成员 addMember", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))

	// 8) 加 webhook
	body, code = r.postForm("/api/repository/editWebhook", map[string]string{
		"repository": r.createdRepo,
		"url":        "http://127.0.0.1:19999/hook",
		"secret":     "smoke-secret",
		"events":     "mergeRequest:create,hook:postReceive",
		"active":     "1",
	})
	r.check("加 webhook editWebhook", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))

	// 9) 管理端写配置（P0 修复点：必须原子写 + 备份）
	if !r.keep {
		body, code = r.postForm("/api/admin/config", map[string]string{
			"data": `{"host":"http://localhost:9080","ssh":"git@localhost:","allowRegister":true,"email":{"name":"CodeFever Next","from":"git@mail.sender"}}`,
		})
		r.check("管理端写配置 admin.config（P0 修复点）", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))
	}

	// 10) 数据层校验
	r.verifyDB()

	// 11) 清理
	if !r.keep {
		r.cleanup()
	}
}

// verifyDB 校验数据确实落库、事件确实入队。
func (r *runner) verifyDB() {
	if r.db == nil {
		return
	}
	var n int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM cc_repositories WHERE r_key = ?", r.createdRepo).Scan(&n); err == nil {
		r.check("数据库里有新建的仓库", n == 1, fmt.Sprintf("count=%d", n))
	}
	if err := r.db.QueryRow("SELECT COUNT(*) FROM cc_events WHERE ev_type LIKE 'repo:%'").Scan(&n); err == nil {
		r.check("事件已入队（repo:*）", n > 0, fmt.Sprintf("count=%d", n))
	}
	if r.gitStorage != "" && r.createdRepo != "" {
		// 裸仓库目录名是 r_path，需从库里取。
		var path string
		if err := r.db.QueryRow("SELECT r_path FROM cc_repositories WHERE r_key = ?", r.createdRepo).Scan(&path); err == nil {
			dir := filepath.Join(r.gitStorage, strings.TrimPrefix(path, "/"))
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				r.check("裸仓库目录已创建", true, dir)
			} else {
				r.check("裸仓库目录已创建", false, dir+" 不存在")
			}
		}
	}
}

// cleanup 删除本次创建的所有数据。
func (r *runner) cleanup() {
	if r.createdRepo != "" {
		body, code := r.postForm("/api/repository/deleteRepository", map[string]string{"repository": r.createdRepo})
		r.check("清理：删库", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))
	}
	if r.createdGroupKey != "" {
		body, code := r.postForm("/api/group/deleteGroup", map[string]string{"group": r.createdGroupKey})
		r.check("清理：删组", code == 0, fmt.Sprintf("code=%d body=%s", code, trunc(body)))
	}
}

// seedInitialCommit 给新建的空仓库推一个初始提交。
//
// 新建的裸仓库没有 master 分支，直接建分支/建标签/建合并请求都会失败
// （这与 PHP 侧行为一致，不是缺陷）。这里模拟用户第一次 push，
// 让后续的写端点有可操作的基线。
func (r *runner) seedInitialCommit(rKey, message string) {
	if r.db == nil || r.gitStorage == "" || rKey == "" {
		return
	}
	var rPath string
	if err := r.db.QueryRow("SELECT r_path FROM cc_repositories WHERE r_key = ?", rKey).Scan(&rPath); err != nil {
		r.check("准备初始提交：取 r_path", false, err.Error())
		return
	}
	bare := filepath.Join(r.gitStorage, strings.TrimPrefix(rPath, "/"))

	tmp, err := os.MkdirTemp("", "smoke-seed-*")
	if err != nil {
		r.check("准备初始提交：建临时目录", false, err.Error())
		return
	}
	defer os.RemoveAll(tmp)

	git := r.gitBin
	run := func(args ...string) error {
		cmd := exec.Command(git, args...)
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=smoke", "GIT_AUTHOR_EMAIL=smoke@codefever.cn",
			"GIT_COMMITTER_NAME=smoke", "GIT_COMMITTER_EMAIL=smoke@codefever.cn")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := run("init", "-q", "-b", "master", "."); err != nil {
		r.check("准备初始提交：git init", false, err.Error())
		return
	}
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("smoke regression seed\n"), 0o644); err != nil {
		r.check("准备初始提交：写文件", false, err.Error())
		return
	}
	if err := run("add", "README.md"); err != nil {
		r.check("准备初始提交：git add", false, err.Error())
		return
	}
	if err := run("commit", "-q", "-m", message); err != nil {
		r.check("准备初始提交：git commit", false, err.Error())
		return
	}
	if err := run("push", "-q", bare, "master"); err != nil {
		r.check("准备初始提交：git push", false, err.Error())
		return
	}
	r.check("准备初始提交：推送 master", true, bare)
}

// pushCommitToBranch 给指定分支推一个新提交。
func (r *runner) pushCommitToBranch(rKey, branch, message string) {
	if r.db == nil || r.gitStorage == "" || rKey == "" || branch == "" {
		return
	}
	var rPath string
	if err := r.db.QueryRow("SELECT r_path FROM cc_repositories WHERE r_key = ?", rKey).Scan(&rPath); err != nil {
		r.check("准备分支提交：取 r_path", false, err.Error())
		return
	}
	bare := filepath.Join(r.gitStorage, strings.TrimPrefix(rPath, "/"))

	tmp, err := os.MkdirTemp("", "smoke-branch-*")
	if err != nil {
		r.check("准备分支提交：建临时目录", false, err.Error())
		return
	}
	defer os.RemoveAll(tmp)

	git := r.gitBin
	run := func(args ...string) error {
		cmd := exec.Command(git, args...)
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=smoke", "GIT_AUTHOR_EMAIL=smoke@codefever.cn",
			"GIT_COMMITTER_NAME=smoke", "GIT_COMMITTER_EMAIL=smoke@codefever.cn")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := run("clone", "-q", bare, "."); err != nil {
		r.check("准备分支提交：clone", false, err.Error())
		return
	}
	if err := run("checkout", "-q", "-B", branch); err != nil {
		r.check("准备分支提交：checkout", false, err.Error())
		return
	}
	if err := os.WriteFile(filepath.Join(tmp, "branch.txt"), []byte("smoke branch commit\n"), 0o644); err != nil {
		r.check("准备分支提交：写文件", false, err.Error())
		return
	}
	if err := run("add", "branch.txt"); err != nil {
		r.check("准备分支提交：git add", false, err.Error())
		return
	}
	if err := run("commit", "-q", "-m", message); err != nil {
		r.check("准备分支提交：git commit", false, err.Error())
		return
	}
	if err := run("push", "-q", "origin", branch); err != nil {
		r.check("准备分支提交：git push", false, err.Error())
		return
	}
	r.check("准备分支提交：推送到 "+branch, true, bare)
}

// check 记录一条断言结果。
func (r *runner) check(name string, ok bool, detail string) {
	if ok {
		r.passed++
		fmt.Printf("  ✅ %-40s %s\n", name, detail)
		return
	}
	r.failed++
	fmt.Printf("  ❌ %-40s %s\n", name, detail)
}

// postForm 以 multipart/form-data 提交（前端所有写操作的形态）。
func (r *runner) postForm(path string, fields map[string]string) (string, int) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	_ = w.Close()

	req, err := http.NewRequest(http.MethodPost, r.c.base+path, &buf)
	if err != nil {
		return err.Error(), -1
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	return r.do(req)
}

// get 发起 GET 请求。
func (r *runner) get(path string) (string, int) {
	req, err := http.NewRequest(http.MethodGet, r.c.base+path, nil)
	if err != nil {
		return err.Error(), -1
	}
	req.Header.Set("Accept", "application/json")
	return r.do(req)
}

// do 执行请求并解析响应信封。
func (r *runner) do(req *http.Request) (string, int) {
	resp, err := r.c.http.Do(req)
	if err != nil {
		return err.Error(), -1
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return string(raw), -1
	}
	return string(raw), env.Code
}

// pick 从响应里取一个字段（兼容 data 是对象或数组的情况）。
func (r *runner) pick(raw, field string) string {
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(env.Data, &obj); err == nil {
		if v, ok := obj[field].(string); ok {
			return v
		}
	}
	return ""
}

// legacyHash 复刻 PHP 的 md5(md5($password))。
//
// 用于断言"Go 写下的口令哈希仍然能被 PHP 后端验证"——
// 双后端共存期的硬要求，见 internal/auth 的 HashFormat 说明。
func legacyHash(password string) string {
	first := md5.Sum([]byte(password))
	second := md5.Sum([]byte(hex.EncodeToString(first[:])))
	return hex.EncodeToString(second[:])
}

// shortRand 生成短随机串。
func shortRand() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%100000000)
}

// trunc 截断长文本。
func trunc(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

// fail 直接退出。
func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
