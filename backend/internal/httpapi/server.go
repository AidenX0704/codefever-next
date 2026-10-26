// Package httpapi 是 HTTP 接入层：路由、鉴权中间件、分发与响应。
//
// 分发顺序严格复刻 PHP 侧的 CodeIgniter 流程，
// 因为顺序差异会直接改变前端可见的错误码：
//
//	路由解析（未知 controller → HTML 404 页，不是 JSON）
//	  → 请求头校验（0x0102 / 0x0103 / 0x0104）
//	  → 鉴权（0x0101）
//	  → 动作查找（0x0105）
//	  → 业务处理
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/httpapi/publicapi"
	"codefever-next/backend/internal/repo"
	"codefever-next/backend/internal/session"
)

// AuthLevel 表示端点的鉴权等级，对应 PHP 侧的 APIAuth 常量。
type AuthLevel string

const (
	// AuthWebUser 要求已登录的普通用户（CI session）。
	AuthWebUser AuthLevel = "web_user"
	// AuthAdminWebUser 要求管理员。
	AuthAdminWebUser AuthLevel = "web_admin_user"
	// AuthInternalGateway 要求内部网关令牌。
	AuthInternalGateway AuthLevel = "internal_gateway"
)

// ActionHandler 处理一个具体端点。
//
// 返回值会被序列化为响应信封的 data 字段；返回 error 则序列化为错误信封。
type ActionHandler func(ctx context.Context, req *Request) (any, error)

// Controller 对应 PHP 侧的 api/<name>.php。
type Controller struct {
	// Name 是控制器名，对应 URL 中的 <controller> 段。
	Name string
	// Auth 是该控制器要求的鉴权等级。
	Auth AuthLevel
	// Actions 的键是动作名，值是「HTTP 方法 → 处理器」的映射。
	//
	// PHP 侧的动作名与方法名是拼接关系（<action>_<method>），
	// 这里拆开表达以获得编译期检查。
	Actions map[string]map[string]ActionHandler
}

// Server 是 HTTP 服务。
type Server struct {
	cfg      *config.Config
	sessions *session.Store
	users    *repo.UserRepo
	log      *slog.Logger

	controllers map[string]*Controller
	// controllerNames 声明"哪些控制器是存在的"。
	//
	// 这一层区分是契约的关键：PHP 侧 CodeIgniter 路由失败时会回落到
	// nopage/index.php，返回 **HTML 404 页**；而"控制器存在、动作不存在"
	// 走 JSON 信封的 0x0105。
	//
	// 实测：
	//   GET /api/nope/nope   → 404 + Content-Type: text/html（controller 不存在）
	//   GET /api/user/nope   → 404 + JSON code=261（0x0105）（controller 存在）
	//
	// 因此新后端必须**预先声明全部控制器名**，即使其动作尚未实现，
	// 否则未实现的端点会错误地返回 HTML 而不是 JSON 0x0105。
	controllerNames map[string]bool
	pageRoutes      map[string]http.HandlerFunc
	pagePatterns    map[string]http.HandlerFunc

	// webControllers 是 PHP 侧 application/controllers/ 下的顶层控制器
	// （user / file / doc / lang / captcha / home / nopage / base）。
	//
	// 键是控制器名，**区分大小写**（CI 用 file_exists 探测控制器文件，
	// Linux 文件系统区分大小写）；值里的方法名统一小写化
	// （PHP 的 method_exists 大小写不敏感）。
	webControllers map[string]*WebController
	// pageRoutesCI 是 pageRoutes 的小写索引，用于让 /user/LOGIN 这类
	// 大小写变体命中同一个页面处理器。
	pageRoutesCI map[string]http.HandlerFunc

	// publicRouter 是公开 API（/api/v1）的路由表。
	//
	// 它与内部 /api/* 家族**完全隔离**：不同的路径前缀、不同的响应信封、
	// 不同的鉴权方式（令牌 vs 会话 Cookie）。
	// 未装配时（测试或未启用）保持 nil，请求会落到兜底 404。
	publicRouter *publicapi.Router
}

// PublicAPIOption 是 Server 的可选依赖。
type PublicAPIOption func(*Server)

// WithPublicAPI 装配公开 API（/api/v1）。
//
// 依赖齐备才装配 —— 缺任何一个仓储都不装配（公开 API 直接 404），
// 避免运行期出现"路由存在但鉴权必然失败"的半可用状态。
func WithPublicAPI(users *repo.UserRepo, repos *repo.RepositoryRepo, tokens *repo.AccessTokenRepo,
	access *repo.AccessRepo, groups *repo.GroupRepo, siteURL string) PublicAPIOption {
	return func(s *Server) {
		if users == nil || repos == nil || tokens == nil || access == nil || groups == nil {
			return
		}
		api := publicapi.NewServer(users, repos, tokens, access, groups, siteURL)
		publicapi.SetDocURL(siteURL)
		s.publicRouter = publicapi.NewRouter(api)
	}
}

// New 创建 HTTP 服务。
func New(cfg *config.Config, sessions *session.Store, users *repo.UserRepo, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		cfg:             cfg,
		sessions:        sessions,
		users:           users,
		log:             log,
		controllers:     map[string]*Controller{},
		controllerNames: map[string]bool{},
		pageRoutes:      map[string]http.HandlerFunc{},
		pagePatterns:    map[string]http.HandlerFunc{},
		webControllers:  map[string]*WebController{},
		pageRoutesCI:    map[string]http.HandlerFunc{},
	}
}

// Register 注册一个控制器。
func (s *Server) Register(c *Controller) {
	s.controllers[c.Name] = c
	s.controllerNames[c.Name] = true
}

// DeclareController 声明一个控制器存在，但暂不提供任何动作实现。
//
// 用于"该控制器的动作尚未移植"的阶段：此时访问其动作会得到 JSON 0x0105，
// 与 PHP 侧"控制器存在但方法不存在"的响应一致。
func (s *Server) DeclareController(name string) {
	s.controllerNames[name] = true
}

// RegisterPattern 注册一个带通配段的路由。
//
// 用于 /file/fetch/{storage}/{key} 这类路径参数路由。
func (s *Server) RegisterPattern(pattern string, handler http.HandlerFunc) {
	s.pagePatterns[pattern] = handler
}

// RegisterPage 注册一个非 API 的页面路由。
//
// 这些路径由 PHP 侧的 web 控制器提供（/user/login、/user/logout 等），
// 它们的响应信封与 /api/* 不同，因此不走统一分发器。
//
// 路径本身**区分大小写**（Go 的 ServeMux 语义），但 web 控制器分发器会
// 用小写索引再查一次，因此 /user/LOGIN 也能命中同一个处理器
// （PHP 侧 method_exists 大小写不敏感）。
func (s *Server) RegisterPage(path string, handler http.HandlerFunc) {
	s.pageRoutes[path] = handler
	s.pageRoutesCI[strings.ToLower(path)] = handler
}

// Controller 返回已注册的控制器。
func (s *Server) Controller(name string) (*Controller, bool) {
	c, ok := s.controllers[name]
	return c, ok
}

// maxRequestBodyBytes 是单个请求体的硬上限。
//
// nginx 侧 app location 的 client_max_body_size 是 20m；这里留 2MB 余量给
// multipart 边界与表单字段。MaxBytesReader 让 `contract.Parse` 的
// io.ReadAll 在超限时立即失败，避免匿名请求把进程内存打满。
const maxRequestBodyBytes = 22 << 20

// Handler 返回可挂载到 http.Server 的处理器。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// API 端点：/api/<controller> 与 /api/<controller>/<action>。
	//
	// 用不带方法前缀的模式注册，由分发器自己判定动词 —— 因为 PHP 侧对
	// 不支持的动词返回 0x0102（JSON 信封），而 Go 的 ServeMux 会返回
	// 405 且不带信封，两者不一致。
	mux.HandleFunc("/api/{ctrl}", s.handleAPI)
	mux.HandleFunc("/api/{ctrl}/{action}", s.handleAPI)

	// 内部网关回调：/apiv1/internal/<action>。
	mux.HandleFunc("/apiv1/internal/{action}", s.handleInternal)

	// 公开 API：/api/v1/ 子树。
	//
	// 【为什么用 Wrap 而不是 mux.Handle("/api/v1/", ...)】
	//
	// Go 的 ServeMux 拒绝歧义模式：主 mux 已有 /api/{ctrl}/{action}，
	// 再注册 /api/v1/ 会 panic（两者都能匹配 "/api/v1/action"，
	// 但谁都不比谁更具体）。因此公开 API 在**进入主 mux 之前**分流，
	// 由它自己的 mux 处理 —— 主 mux 完全看不到 /api/v1 请求。
	//
	// 注意与 /apiv1/internal（内部网关回调前缀）无关，两者不可混用。

	// 健康检查。
	mux.HandleFunc("/healthz", s.handleHealth)

	// 页面路由（/user/login、/user/logout 等）。
	for path, handler := range s.pageRoutes {
		mux.HandleFunc(path, handler)
	}
	for pattern, handler := range s.pagePatterns {
		mux.HandleFunc(pattern, handler)
	}

	// 其余路径：交给 web 控制器分发器 —— 它复刻 CI 的三段判定
	// （控制器存在 → 控制器不存在 → 404 HTML 页），见 web.go。
	mux.HandleFunc("/", s.handleFallback)

	// 公开 API 在进入主 mux **之前**分流（原因见上方注释）。
	//
	// 分流放在 normalizeAPIPath **外面**，这样公开 API 的多段路径
	// 连段数截断都不会碰到 —— 双保险，避免将来有人调整中间件顺序时
	// 又把公开 API 打成静默 404。
	handler := http.Handler(mux)
	if s.publicRouter != nil {
		handler = s.publicRouter.Wrap(handler)
	}

	return s.recoverMiddleware(s.logMiddleware(limitRequestBody(normalizeAPIPath(handler))))
}

// normalizeAPIPath 复刻 CodeIgniter URI 的段处理：
//
//   - 丢弃空段（尾斜杠、连续斜杠）；
//   - 多余段作为"方法参数"传入（PHP 用户函数允许多余实参，实际被忽略）。
//
// 不处理的话，`/api/user/info/` 会落到兜底 HTML 404，而 PHP 返回 200 JSON。
// 只改路径，不碰 query 与大小写（控制器名仍区分大小写）。
//
// 【公开 API 豁免】/api/v1/ 前缀**不做段数截断**。
//
// 截断规则来自 PHP 侧「控制器 + 动作」两段式路由，对公开 API 不适用：
// /api/v1/repos/{owner}/{repo} 有 4 段以上，若被截到 3 段就会变成
// /api/v1/repos，被当成 controller=v1 / action=repos 分发，最终静默 404。
// 公开 API 由独立的子树路由接管（见 publicapi.Router），段数不受限制。
func normalizeAPIPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && !publicapi.IsPublicAPIPath(r.URL.Path) {
			segments := make([]string, 0, 3)
			for _, seg := range strings.Split(r.URL.Path, "/") {
				if seg != "" {
					segments = append(segments, seg)
				}
			}
			if len(segments) > 3 {
				segments = segments[:3]
			}
			normalized := "/" + strings.Join(segments, "/")
			if normalized != r.URL.Path {
				r.URL.Path = normalized
				r.URL.RawPath = ""
			}
		}
		next.ServeHTTP(w, r)
	})
}

// limitRequestBody 限制请求体大小，并在请求结束后清理 multipart 临时文件。
func limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		}
		next.ServeHTTP(w, r)
		// ParseMultipartForm 会把大文件落到临时目录；Go 不保证自动清理，
		// 这里显式清掉，避免 /tmp 随上传次数累积。
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	})
}

// handleHealth 返回服务健康状态。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","time":"` + time.Now().Format(time.RFC3339) + `"}`))
}

// logMiddleware 记录访问日志。
func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestIDFrom(r.Context()),
			"remote", r.RemoteAddr,
		)
	})
}

// recoverMiddleware 捕获 panic，避免单个请求打挂整个进程。
//
// PHP 侧没有全局异常处理器：致命错误会输出截断的 JSON（Content-Length 已设但
// body 不全），前端解析失败。这里改为返回完整、可解析的错误信封。
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic recovered",
					"error", rec,
					"path", r.URL.Path,
					"request_id", requestIDFrom(r.Context()),
				)
				id := requestIDFrom(r.Context())
				if id == "" {
					id = newRequestID()
				}
				writeInternalError(w, id, rec)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder 记录响应状态码，用于访问日志。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader 记录状态码。
func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
