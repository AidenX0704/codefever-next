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

	// PageDataProvider 构造服务端渲染页面所需的数据。
	//
	// 由 main 在装配时注入；缺省时 404 页退回到自包含的 HTML 实现。
	PageDataProvider func() PageData
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

	return s.recoverMiddleware(s.logMiddleware(mux))
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
