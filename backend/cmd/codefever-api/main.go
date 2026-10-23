// Command codefever-api 是 CodeFever Next 的 Go 后端服务入口。
//
// 它替换原有 PHP-FPM 承载的 application/ 全部逻辑，对外保持
// 108 个 /api/* 端点与 4 个 /apiv1/internal/* 回调的契约不变，
// 使前端产物 www/view/build 与两个 Go 网关无需改动即可继续工作。
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/db"
	"codefever-next/backend/internal/events"
	"codefever-next/backend/internal/gitsvc"
	"codefever-next/backend/internal/handlers"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/mailer"
	"codefever-next/backend/internal/repo"
	"codefever-next/backend/internal/session"
	"codefever-next/backend/internal/storage"
)

// defaultBaseDir 是容器内的应用根目录。
//
// 该路径不可变更：Go 网关通过 <base>/http-gateway/main 与
// <base>/ssh-gateway/shell/main 的**相对路径**读取 <base>/env.yaml。
const defaultBaseDir = "/data/www/codefever-community"

// sessionTTL 与 PHP 侧 config.php 的 sess_expiration 保持一致（10 天）。
const sessionTTL = 10 * 24 * time.Hour

func main() {
	var (
		baseDir = flag.String("base", envOr("CODEFEVER_BASE_DIR", defaultBaseDir), "应用根目录（env.yaml 所在目录）")
		addr    = flag.String("addr", envOr("CODEFEVER_ADDR", ":9001"), "监听地址")
		logLvl  = flag.String("log-level", envOr("CODEFEVER_LOG_LEVEL", "info"), "日志级别 debug|info|warn|error")
	)
	flag.Parse()

	logger := newLogger(*logLvl)
	slog.SetDefault(logger)

	if err := run(*baseDir, *addr, logger); err != nil {
		logger.Error("服务启动失败", "error", err)
		os.Exit(1)
	}
}

// run 装配并启动服务。
func run(baseDir, addr string, logger *slog.Logger) error {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return err
	}

	cfg, err := config.Load(absBase)
	if err != nil {
		return err
	}
	logger.Info("配置已加载",
		"base", absBase,
		"mysql", cfg.Env.MySQL.Host+":"+itoa(cfg.Env.MySQL.Port)+"/"+cfg.Env.MySQL.DB,
		"env", cfg.Env.Env,
	)

	sqlDB, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	migrateCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := db.Migrate(migrateCtx, sqlDB); err != nil {
		return err
	}
	logger.Info("数据库迁移完成")

	users := repo.NewUserRepo(sqlDB)
	notifications := repo.NewNotificationRepo(sqlDB)
	disk := storage.NewDisk(cfg.FileStoragePath())
	mail := mailer.NewLocalMailer()
	sessions := session.NewStore(sqlDB, sessionTTL)

	srv := httpapi.New(cfg, sessions, users, logger)

	// 服务端渲染页面的公共数据。
	srv.PageDataProvider = func() httpapi.PageData {
		return httpapi.PageData{
			Lang:          httpapi.NewTranslations("zh-cn"),
			Host:          cfg.Site.Host,
			ReleaseNumber: cfg.Env.ReleaseNumber,
			AppName:       handlers.AppName,
			AllowRegister: cfg.Site.AllowRegister,
		}
	}

	// 业务域端点。
	handlers.NewUserHandler(cfg, users, notifications, disk, mail, sessions).Register(srv)
	fileHandler := handlers.NewFileHandler(disk)
	fileHandler.Register(srv)

	// web 控制器命名空间（PHP 侧 application/controllers/ 下的顶层控制器）。
	//
	// 这些路径由 nginx 反向代理到后端（/user/*、/file/*、/doc/*、/lang/*、
	// /captcha/*、/feature 等），响应信封与 /api/* 不同（见 httpapi/web.go）。
	handlers.NewWebUserHandler(cfg, users, sessions, disk, mail).RegisterWeb(srv)
	fileHandler.RegisterWeb(srv)
	handlers.NewDocHandler(cfg).RegisterWeb(srv)
	handlers.NewLangHandler(cfg).RegisterWeb(srv)
	handlers.NewCaptchaHandler().RegisterWeb(srv)
	handlers.NewHomeHandler(cfg, users, sessions).RegisterWeb(srv)
	handlers.NewNopageHandler(cfg).RegisterWeb(srv)
	handlers.RegisterBaseController(srv)

	// 权限守卫：仓库组 / 仓库 / 内部网关共用同一套角色解析与权限判定。
	guard := handlers.NewGuard(repo.NewAccessRepo(sqlDB))

	// Git 服务层（argv 调用，不经 shell）与事件总线。
	//
	// 事件与业务写入在同一事务内入队，保证"业务提交则事件在、业务回滚则事件消失"。
	gitSvc := gitsvc.NewService(cfg.Env.CLI.Git, cfg.GitStoragePath())
	defer gitSvc.Close()
	eventBus := events.NewBus(sqlDB)

	// 仓库组域（13 个端点）。
	handlers.NewGroupHandler(
		repo.NewGroupRepo(sqlDB),
		guard,
		handlers.WithGroupUserRepo(users),
		handlers.WithGroupSiteConfig(cfg),
	).Register(srv)

	// 仓库域：只读端点 + 写端点（同一控制器）。
	handlers.NewRepositoryHandler(
		repo.NewRepositoryRepo(sqlDB),
		gitSvc,
		guard,
		handlers.WithRepositorySiteConfig(cfg),
	).Register(srv)
	handlers.NewRepositoryWriteHandler(
		repo.NewRepositoryWriteRepo(sqlDB),
		users,
		guard,
		gitSvc,
		eventBus,
		handlers.WithRepositoryWriteSiteConfig(cfg),
	).Register(srv)

	// 管理域（19 个端点）。
	//
	// 构造函数返回 error：启动自检发现 config.yaml 不可解析时**拒绝启动**，
	// 而不是让每个请求都 500（这是 PHP 侧 P0 缺陷的根因）。
	admin, err := handlers.NewAdminHandler(
		cfg,
		repo.NewAdminRepo(sqlDB),
		users,
		repo.NewGroupRepo(sqlDB),
		repo.NewRepositoryRepo(sqlDB),
		disk,
	)
	if err != nil {
		return err
	}
	admin.Register(srv)

	// 内部网关回调（Go 网关与 git 钩子的唯一通道）。
	handlers.NewInternalHandler(
		cfg,
		guard,
		repo.NewInternalRepo(sqlDB),
		users,
	).Register(srv)

	// 预先声明全部控制器名。
	//
	// PHP 侧"控制器存在但动作不存在"返回 JSON 0x0105，而"控制器不存在"
	// 返回 HTML 404 页。未移植动作的控制器必须先声明，否则会错误地
	// 返回 HTML 404，与契约不符。
	for _, name := range []string{"user", "admin", "group", "repository", "internal"} {
		srv.DeclareController(name)
	}

	// 页面路由（响应信封与 /api/* 不同）。
	login := handlers.NewLoginHandler(cfg, users, sessions)
	srv.RegisterPage("/user/login", loginPageHandler(login, cfg))
	srv.RegisterPage("/user/logout", logoutHandler(login))

	handler := srv.Handler()

	// pprof：仅在 CODEFEVER_PPROF=1 时挂载。
	//
	// 它是**运维诊断通道**（采样 goroutine 数与堆占用，判断长跑是否有泄漏），
	// 不属于产品 API 面，因此默认关闭；开启时只监听与业务同一个端口，
	// 但路径前缀是 /debug/pprof/，与 /api/*、/apiv1/* 完全不重叠，
	// 不会改变契约基线。
	if os.Getenv("CODEFEVER_PPROF") == "1" {
		handler = withPprof(handler)
		logger.Info("pprof 已开启", "path", "/debug/pprof/")
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP 服务已启动", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-stop:
		logger.Info("收到退出信号，开始优雅关闭")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	return httpServer.Shutdown(shutdownCtx)
}

// loginPageHandler 处理 /user/login。
//
// GET  → 渲染登录页（已登录则 302 /repositories）
// POST → 执行登录并返回 {code, message, extra} 信封
func loginPageHandler(login *handlers.LoginHandler, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			result, err := login.Login(r.Context(), w, r)
			if err != nil {
				slog.Error("登录处理失败", "error", err)
				w.Header().Set("Content-Type", "text/html; charset=UTF-8")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// PHP 侧 _ajax_message() 不设置 Content-Type，
			// 因此响应头是 text/html; charset=UTF-8（实测确认）。
			// 响应体用 PHP 风格的 JSON 编码（非 ASCII 转 \uXXXX、斜杠转 "\/"），
			// 与 PHP 的 json_encode 默认行为逐字节一致。
			if err := httpapi.WriteAjaxJSON(w, httpapi.NewAjaxEnvelope(result.Code, result.Message, result.Extra)); err != nil {
				slog.Error("写出登录响应失败", "error", err)
			}
			return
		}
		renderLoginPage(w, r, cfg, login)
	}
}

// renderLoginPage 渲染登录页。
//
// 已登录用户直接跳转到 /repositories（对应 PHP 侧 user.php:66-71）。
//
// 判定"已登录"用的是服务端会话（session.Store + cc_sessions 表），
// 与 /api/* 的鉴权中间件同一套机制 —— 只看 Cookie 是否存在是不够的。
func renderLoginPage(w http.ResponseWriter, r *http.Request, cfg *config.Config, login *handlers.LoginHandler) {
	if login.LoggedIn(r.Context(), r) {
		httpapi.Redirect(w, "/repositories")
		return
	}
	data := httpapi.PageData{
		Lang:          httpapi.NewTranslations("zh-cn"),
		Host:          cfg.Site.Host,
		ReleaseNumber: cfg.Env.ReleaseNumber,
		AppName:       handlers.AppName,
		AllowRegister: cfg.Site.AllowRegister,
		Email:         strings.TrimSpace(r.URL.Query().Get("email")),
	}
	if err := httpapi.RenderLogin(w, data); err != nil {
		slog.Error("渲染登录页失败", "error", err)
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// logoutHandler 处理 /user/logout。
//
// 逐字对应 application/controllers/user.php:314-318：
// 清会话 + 302 /user/login（响应体为空）。
func logoutHandler(login *handlers.LoginHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		login.Logout(r.Context(), w, r)
	}
}

// withPprof 在既有 handler 前挂载 pprof 路由。
//
// 只放行 /debug/pprof/ 前缀，其余路径原样透传，因此不会影响
// 业务路由与契约行为。
func withPprof(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/debug/pprof/") {
			mux.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newLogger 创建结构化日志器。
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

// envOr 读取环境变量，没有则返回默认值。
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// itoa 把整数转成字符串。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
