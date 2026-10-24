// Command codefever-http-gateway 是 Git over HTTP 网关。
//
// 它监听 loopback（默认 127.0.0.1:27555），由 nginx 的仓库 location 反向代理。
// 请求先经后端 /apiv1/internal/authHTTP 鉴权，再交给 git-http-backend（CGI）
// 执行真正的 smart HTTP 协议处理。
//
// 该进程与 codefever-api / codefever-worker 分开运行（supervisor 管理），
// 但源码属于同一个 Go module，共享配置解析、内部回调客户端与日志约定。
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"codefever-next/backend/internal/appdir"
	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/gateway"
	"codefever-next/backend/internal/githttp"
)

func main() {
	base := flag.String("base", envOr("CODEFEVER_BASE_DIR", appdir.Resolve()), "应用根目录（env.yaml 所在目录）")
	addr := flag.String("addr", envOr("CODEFEVER_GATEWAY_ADDR", "127.0.0.1:27555"), "监听地址")
	logLvl := flag.String("log-level", envOr("CODEFEVER_LOG_LEVEL", "info"), "日志级别 debug|info|warn|error")
	flag.Parse()

	logger := newLogger(*logLvl)
	slog.SetDefault(logger)

	if err := run(*base, *addr, logger); err != nil {
		logger.Error("HTTP 网关启动失败", "error", err)
		os.Exit(1)
	}
}

// run 校验配置并启动 HTTP 服务。
func run(baseDir, addr string, logger *slog.Logger) error {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return err
	}

	env, err := config.LoadEnv(absBase)
	if err != nil {
		return err
	}

	auth := gateway.NewClient(env, logger)
	if err := auth.ValidateHTTP(); err != nil {
		return err
	}
	if env.CLI.HTTP == "" {
		return errors.New("env.yaml 缺少 cli.http（git-http-backend 路径）")
	}
	if _, err := os.Stat(env.CLI.HTTP); err != nil {
		return errors.New("git-http-backend 不存在或不可执行: " + env.CLI.HTTP)
	}

	repoRoot := env.GitStoragePath(absBase)
	if _, err := os.Stat(repoRoot); err != nil {
		logger.Warn("git-storage 目录尚不存在，将在首次建仓时创建", "path", repoRoot, "error", err)
	}

	handler := &githttp.Handler{
		Auth:     auth,
		CGIPath:  env.CLI.HTTP,
		RepoRoot: repoRoot,
		Logger:   logger,
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		// 不设置 ReadTimeout / WriteTimeout：git clone/push 可能传输数 GB，
		// 固定超时会在大仓库上误杀连接。慢速攻击面由 loopback 监听与
		// nginx 前置层收敛。
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("Git over HTTP 网关已启动",
			"addr", addr,
			"base", absBase,
			"git_storage", repoRoot,
			"git_http_backend", env.CLI.HTTP,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("收到退出信号，开始优雅关闭")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
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
