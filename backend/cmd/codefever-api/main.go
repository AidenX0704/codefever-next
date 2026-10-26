// Command codefever-api 是 CodeFever Next 的 Go 后端服务入口。
//
// 它替换原有 PHP-FPM 承载的 application/ 全部逻辑，对外保持
// 108 个 /api/* 端点与 4 个 /apiv1/internal/* 回调的契约不变，
// 使前端产物 www/view/build 与两个 Go 网关无需改动即可继续工作。
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"codefever-next/backend/internal/appdir"
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

// defaultBaseDir 是容器内的应用根目录（保留给文档与兼容说明）。
//
// 实际默认值由 appdir.Resolve 计算：优先 CODEFEVER_BASE_DIR，其次从
// 可执行文件位置向上找 env.yaml。因此容器内 <base>/bin/codefever-api 与
// 源码安装 <src>/bin/codefever-api 都能自动定位到各自的根目录。
const defaultBaseDir = "/data/www/codefever-community"

// sessionTTL 与 PHP 侧 config.php 的 sess_expiration 保持一致（10 天）。
const sessionTTL = 10 * 24 * time.Hour

func main() {
	var (
		baseDir = flag.String("base", envOr("CODEFEVER_BASE_DIR", appdir.Resolve()), "应用根目录（env.yaml 所在目录）")
		addr    = flag.String("addr", envOr("CODEFEVER_ADDR", "127.0.0.1:9001"), "监听地址")
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

	// 内部网关令牌是 /apiv1/internal/* 的唯一凭据：空值会让
	// CompareToken("", "") 成立（fail-open），模板占位符则是公开值。
	// 生产环境必须显式配置随机令牌；Dockerfile 会在构建期生成。
	if err := validateGatewayToken(cfg); err != nil {
		return err
	}

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
	mail := mailer.FromEnv(cfg.SiteConfig().Email.From)
	sessions := session.NewStore(sqlDB, sessionTTL)
	// 过期会话清理：旧实现只在"恰好有人拿过期 Cookie 访问"时才删，
	// cc_sessions 会无限增长。这里每小时主动清一次。
	go cleanupSessions(sessions, logger)

	// 追加型表的保留期清理（性能整改 D2）。
	//
	// 整改前只有 cc_sessions 有清理任务，cc_events / cc_notification_* /
	// cc_repository_webhook_logs 全部永久增长；其中 cc_events 会被事件 worker
	// 每 2 秒按 ev_created 排序扫描，堆积后直接拖慢整个异步链路。
	//
	// 保留天数全部可用环境变量覆盖；配成 0 即关闭该项。
	// cc_activities 默认**关闭**：它是用户可见的仓库动态，属于产品数据，
	// 删除策略应由运维显式决定（见 repo/retention.go 的说明）。
	go cleanupRetention(sqlDB, logger)

	srv := httpapi.New(cfg, sessions, users, logger)

	// 公开 API（/api/v1）。
	//
	// 与内部 /api/* 家族完全隔离：不同路径前缀、不同响应信封、
	// 不同鉴权方式（访问令牌 vs 会话 Cookie）。
	//
	// 令牌表 cc_access_tokens 由 010_access_tokens.sql 迁移创建；
	// 令牌只存 SHA-256，明文仅在创建时返回一次。
	//
	// 站点 host 取自配置，用于拼出公开 API 里的仓库 HTML / clone 地址。
	httpapi.WithPublicAPI(
		users,
		repo.NewRepositoryRepo(sqlDB),
		repo.NewAccessTokenRepo(sqlDB),
		repo.NewAccessRepo(sqlDB),
		repo.NewGroupRepo(sqlDB),
		cfg.SiteConfig().Host,
	)(srv)

	// SPA 壳（新前端切换，批 7）。
	//
	// 解析顺序（第一个存在的即采用）：
	//  1. CODEFEVER_SPA_INDEX —— 显式覆盖（排障用）；
	//  2. <base>/www/app/index.html —— 镜像布局（Dockerfile 把 web/build 放进 www/app）；
	//  3. <base>/web/build/index.html —— 仓库内直接跑（vite build 的输出）；
	//  4. <base>/www/view/build/index.html —— 旧布局（切换期回滚用）。
	// 都找不到时不配置，各 GET 处理器回退到旧 SSR 页面。
	spaIndex := resolveSPAIndex(cfg.BaseDir)
	httpapi.ConfigureSPA(spaIndex)
	if spaIndex != "" {
		logger.Info("SPA 壳已启用", "path", spaIndex)
	} else {
		logger.Warn("未找到 SPA 壳，页面 GET 仍走服务端渲染")
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
	docHandler := handlers.NewDocHandler(cfg)
	docHandler.RegisterWeb(srv)
	// 文档中心的 JSON 端点（新前端 React 文档页用；注册为字面量路由，公开只读）。
	docHandler.RegisterAPI(srv)
	handlers.NewLangHandler(cfg).RegisterWeb(srv)
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

	// 后台修复历史仓库的裸仓库 HEAD。
	//
	// 早期仓库由 `git init --bare` 创建，HEAD 固定指向 master；如果用户
	// 首次推送 main，clone 会报 "remote HEAD refers to nonexistent ref"。
	// 这里按 DB 默认分支（或 main/master/第一个分支）重建 HEAD。
	go repairDefaultBranchHEADS(
		repo.NewRepositoryRepo(sqlDB),
		repo.NewRepositoryWriteRepo(sqlDB),
		gitSvc,
		logger,
	)

	// 仓库组域（13 个端点）。
	handlers.NewGroupHandler(
		repo.NewGroupRepo(sqlDB),
		guard,
		handlers.WithGroupUserRepo(users),
		handlers.WithGroupSiteConfig(cfg),
		handlers.WithGroupEventBus(eventBus),
		handlers.WithGroupGit(gitSvc, repo.NewRepositoryRepo(sqlDB)),
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
	// 管理端禁用账号 / 重置口令时需要吊销会话。
	admin.Sessions = sessions
	admin.Register(srv)

	// 内部网关回调（Go 网关与 git 钩子的唯一通道）。
	//
	// 注入事件总线是为了补齐 post-receive：push 成功后必须写入
	// HOOK_POST_RECEIVE 事件，否则活动流与 webhook 都会缺记录。
	handlers.NewInternalHandler(
		cfg,
		guard,
		repo.NewInternalRepo(sqlDB),
		users,
		handlers.WithInternalEventBus(sqlDB, eventBus),
		handlers.WithInternalGit(gitSvc),
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

	// 前端 Core Web Vitals 上报（性能整改 B1）。
	//
	// 走字面量 page route，优先级高于 /api/{ctrl}/{action} 通配路由，
	// 因此不会被 API 分发器截走；也就绕开了 contract.Parse 的
	// Accept / Content-Type 校验（sendBeacon 发的是 text/plain + */*）。
	handlers.NewWebVitalsHandler(logger).Register(srv)

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
// resolveSPAIndex 按「环境变量 → 镜像布局 → 仓库布局 → 旧布局」的顺序找 SPA 壳。
func resolveSPAIndex(baseDir string) string {
	candidates := []string{}
	if envPath := strings.TrimSpace(os.Getenv("CODEFEVER_SPA_INDEX")); envPath != "" {
		candidates = append(candidates, envPath)
	}
	candidates = append(candidates,
		filepath.Join(baseDir, "www", "app", "index.html"),
		filepath.Join(baseDir, "web", "build", "index.html"),
		filepath.Join(baseDir, "www", "view", "build", "index.html"),
	)
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

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
		// 前端切换后：/user/login 的 GET 由 SPA 渲染（表单 POST 仍走上面的分支）。
		if !httpapi.ServeSPA(w, r) {
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

// renderLoginPage 渲染登录页。
//
// 已登录用户直接跳转到 /repositories（对应 PHP 侧 user.php:66-71）。
//
// 判定"已登录"用的是服务端会话（session.Store + cc_sessions 表），
// 与 /api/* 的鉴权中间件同一套机制 —— 只看 Cookie 是否存在是不够的。

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

// repairDefaultBranchHEADS 修复历史裸仓库的 HEAD 指向。
//
// 只读 DB 列表 + 逐仓库 `git symbolic-ref`，不阻塞 API 启动；单个仓库失败
// 只记录日志，不影响其它仓库。
func repairDefaultBranchHEADS(repos *repo.RepositoryRepo, writes *repo.RepositoryWriteRepo, git *gitsvc.Service, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	list, err := repos.ListDefaultBranches(ctx)
	if err != nil {
		logger.Warn("读取仓库默认分支失败，跳过 HEAD 修复", "error", err)
		return
	}
	fixed := 0
	for _, item := range list {
		repoCtx, repoCancel := context.WithTimeout(ctx, 15*time.Second)
		repoPath, err := git.RepoPath(item.Path)
		if err != nil {
			repoCancel()
			logger.Warn("解析裸仓库路径失败", "repo", item.Key, "path", item.Path, "error", err)
			continue
		}
		// 种子数据里可能存在"有 DB 行、没有裸仓库"的仓库（例如演示库），
		// 这类记录直接跳过，不要每次启动都打 WARN。
		if _, statErr := os.Stat(repoPath); statErr != nil {
			repoCancel()
			continue
		}
		chosen, err := git.EnsureHEAD(repoCtx, repoPath, item.Branch)
		if err == nil && chosen != "" && chosen != item.Branch {
			if err := writes.SetDefaultBranch(repoCtx, item.Key, chosen); err != nil {
				logger.Warn("回写仓库默认分支失败", "repo", item.Key, "branch", chosen, "error", err)
			} else {
				fixed++
			}
		}
		repoCancel()
		if err != nil {
			logger.Warn("修复裸仓库 HEAD 失败", "repo", item.Key, "branch", item.Branch, "error", err)
		}
	}
	if fixed > 0 {
		logger.Info("已修复仓库默认分支 HEAD", "count", fixed)
	}
}

// cleanupSessions 每小时清理一次过期会话。
func cleanupSessions(store *session.Store, logger *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		n, err := store.Cleanup(ctx)
		cancel()
		if err != nil {
			logger.Warn("清理过期会话失败", "error", err)
			continue
		}
		if n > 0 {
			logger.Info("已清理过期会话", "count", n)
		}
	}
}

// cleanupRetention 每天执行一次追加型表的保留期清理。
//
// 启动后先等一个周期再跑（与 cleanupSessions 一致），避免和启动时的
// 迁移 / 数据初始化抢锁。
func cleanupRetention(sqlDB *sql.DB, logger *slog.Logger) {
	cfg := repo.RetentionConfig{
		// 事件队列：已处理 30 天即清；死信保留 60 天便于排障。
		EventDoneDays:   envInt("CODEFEVER_RETENTION_EVENT_DONE_DAYS", 30),
		EventFailedDays: envInt("CODEFEVER_RETENTION_EVENT_FAILED_DAYS", 60),
		// 站内信：已读用户行 + 其孤儿内容行保留 90 天。
		NotificationDays: envInt("CODEFEVER_RETENTION_NOTIFICATION_DAYS", 90),
		// webhook 投递日志：纯运维日志，90 天。
		WebhookLogDays: envInt("CODEFEVER_RETENTION_WEBHOOK_LOG_DAYS", 90),
		// 仓库动态是用户可见的产品数据 —— 默认关闭（0），
		// 由运维显式开启保留策略。
		ActivityDays: envInt("CODEFEVER_RETENTION_ACTIVITY_DAYS", 0),
	}
	service := repo.NewRetentionService(sqlDB, cfg)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		res, err := service.Run(ctx)
		cancel()
		if err != nil {
			logger.Warn("保留期清理未完全成功", "error", err,
				"events_done", res.EventsDone, "events_failed", res.EventsFailed,
				"notification_users", res.NotificationUsers,
				"notification_content", res.NotificationContent,
				"webhook_logs", res.WebhookLogs, "activities", res.Activities)
			continue
		}
		if !res.Empty() {
			logger.Info("保留期清理完成",
				"events_done", res.EventsDone, "events_failed", res.EventsFailed,
				"notification_users", res.NotificationUsers,
				"notification_content", res.NotificationContent,
				"webhook_logs", res.WebhookLogs, "activities", res.Activities)
		}
	}
}

// envInt 读取整数环境变量，缺失或非法时返回默认值。
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

// envOr 读取环境变量，没有则返回默认值。
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// validateGatewayToken 校验内部网关令牌。
//
// 空令牌会让 ConstantTimeCompare("", "") 成立，等价于关闭内部端点鉴权；
// 模板占位符是仓库里的公开值，生产环境同样不可接受。
// 本地开发（env: development）允许占位符，但会打印告警。
func validateGatewayToken(cfg *config.Config) error {
	token := strings.TrimSpace(cfg.Env.Gateway.Token)
	if token == "" {
		return errors.New("env.yaml 的 gateway.token 不能为空：内部网关端点将无法安全鉴权")
	}
	if token == "<codefever_community_api_token>" {
		if cfg.Env.Env == "development" {
			slog.Warn("gateway.token 仍是模板占位符，仅限本地开发使用")
			return nil
		}
		return errors.New("env.yaml 的 gateway.token 仍是模板占位符，请生成随机令牌（openssl rand -hex 32）")
	}
	return nil
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
