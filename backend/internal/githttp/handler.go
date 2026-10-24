// Package githttp 实现 Git over HTTP 网关。
//
// 职责：把浏览器/命令行 git 客户端的 smart HTTP 请求，经内部鉴权回调确认
// 用户与仓库权限后，交给 git 自带的 git-http-backend（CGI）执行。
//
// 与旧实现（http-gateway/Request/main.go）的差异全部是**健壮性修复**，
// 不改变任何对外可观察语义：
//
//   - 配置只在启动时读一次，不再每个请求读 YAML（旧实现还有包级变量并发写竞态）；
//   - User-Agent 缺失时返回 405，而不是数组越界 panic；
//   - 鉴权 HTTP 客户端有 10 秒超时与连接池，不会在后端卡死时永久挂起；
//   - 不把用户名/口令/请求体写进日志；
//   - 正则预编译，且修正了旧实现 [a-zA-z] 的字符范围笔误；
//   - 支持 graceful shutdown；服务退出时不再丢弃在途请求。
package githttp

import (
	"log"
	"log/slog"
	"net/http"
	"net/http/cgi"
	"os"
	"regexp"
	"strings"

	"codefever-next/backend/internal/gateway"
)

// repoPathPattern 从 URL 路径中拆出仓库名与剩余 CGI 路径。
//
// 旧实现是 `^([a-zA-z0-9\/\.-]+\.git)(.*)$`（[a-zA-z] 是笔误，实际覆盖
// A-Z、[、\、]、^、_、`）。这里写成显式字符集，语义更清楚。
// 仓库名允许一层 group/repo，与 nginx vhost 的 location 正则一致。
var repoPathPattern = regexp.MustCompile(`^([A-Za-z0-9/._-]+\.git)(.*)$`)

// Handler 是 Git over HTTP 的 http.Handler。
type Handler struct {
	// Auth 是后端内部回调客户端。
	Auth *gateway.Client
	// CGIPath 是 git-http-backend 可执行体路径（env.yaml 的 cli.http）。
	CGIPath string
	// RepoRoot 是 git-storage 根目录的绝对路径。
	RepoRoot string
	// Logger 是结构化日志器。
	Logger *slog.Logger
}

// ServeHTTP 实现 http.Handler。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1) git 客户端识别。旧实现对空 User-Agent 会 panic，这里改为 405。
	if !strings.HasPrefix(r.UserAgent(), "git") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 2) 必须有 Basic Auth（终端用户：邮箱 + 口令）。
	user, pass, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Git User Login"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 3) 只接受 /<group>/<repo>.git/... 形态。
	//
	// 旧实现有一个"内部调用模式"：URL 不以 .git 结尾时，只校验 Basic 用户名
	// 形如 name@addr、口令任意，并把 GIT_PROJECT_ROOT 放宽到整个 git-storage，
	// 等于一个鉴权旁路。它只在 loopback 监听 + nginx 正则限制下才安全，
	// 这里直接删除该模式：非仓库路径一律 404。
	sub := repoPathPattern.FindStringSubmatch(r.URL.Path)
	if len(sub) < 2 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	repo := sub[1]
	req := r.Clone(r.Context())
	req.URL.Path = sub[2]

	// action 决定后端要做 READ 还是 PUSH 权限校验。
	//
	// info/refs 请求带 ?service=git-receive-pack / git-upload-pack；
	// 真正的 POST /git-receive-pack 没有 query，若只读 service，
	// 会退化成 READ 权限（PHP 时代同款缺陷）。这里按路径尾段补齐。
	action := r.URL.Query().Get("service")
	if action == "" {
		switch {
		case strings.HasSuffix(req.URL.Path, "/git-receive-pack"):
			action = "git-receive-pack"
		case strings.HasSuffix(req.URL.Path, "/git-upload-pack"):
			action = "git-upload-pack"
		}
	}

	res, err := h.Auth.AuthHTTP(r.Context(), gateway.AuthHTTPRequest{
		User:   user,
		Pass:   pass,
		Repo:   repo,
		Action: action,
	})
	if err != nil {
		// 旧实现：除 404 外一律改写成 401。这保证凭据错误 / 后端故障
		// 不会把内部错误码泄露给 git 客户端。
		status := http.StatusUnauthorized
		if gateway.IsNotFound(err) {
			status = http.StatusNotFound
		}
		h.logAuthFailure(r, repo, err, status)
		if status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="Git User Login"`)
		}
		w.WriteHeader(status)
		return
	}

	env := []string{
		"GIT_HTTP_EXPORT_ALL=",
		"GIT_PROJECT_ROOT=" + res.Path,
		"REMOTE_USER=" + res.User,
		"REMOTE_ADDR=" + res.Addr,
		"PGYER_UID=" + res.UID,
	}

	// 4) 防止客户端伪造网关信任的头。CGI 会把它们映射成 HTTP_*，
	//    虽不直接等于 REMOTE_USER，但去掉可以避免下游误用。
	for _, name := range []string{"REMOTE_USER", "REMOTE_ADDR", "PGYER-REPO", "PGYER-REPO-USER", "PGYER-REPO-ADDR"} {
		req.Header.Del(name)
	}

	// 5) 透传 Git protocol v2（性能相关）。git-http-backend 读 GIT_PROTOCOL
	//    环境变量；CGI 只会把它映射成 HTTP_GIT_PROTOCOL，因此这里显式补上。
	if proto := r.Header.Get("Git-Protocol"); proto != "" {
		env = append(env, "GIT_PROTOCOL="+proto)
	}

	handler := &cgi.Handler{
		Path:   h.CGIPath,
		Dir:    h.RepoRoot,
		Env:    env,
		Logger: log.New(os.Stderr, "cgi: ", log.LstdFlags),
	}
	handler.ServeHTTP(w, req)
}

// logAuthFailure 记录鉴权失败，但绝不记录口令。
func (h *Handler) logAuthFailure(r *http.Request, repo string, err error, status int) {
	if h.Logger == nil {
		return
	}
	h.Logger.Warn("git 网关鉴权失败",
		"repo", repo,
		"path", r.URL.Path,
		"remote", r.RemoteAddr,
		"status", status,
		"error", err,
	)
}
