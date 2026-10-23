// Command codefever-worker 是后台事件处理进程。
//
// 它从 cc_events 队列表取事件，派发给各个 handler：
//
//	活动流（cc_activities）
//	站内通知 + 邮件（cc_notification_content / cc_notification_users）
//	仓库 webhook（cc_repository_webhook_events → 投递）
//
// 与 PHP 侧的区别：PHP 把这些动作**同步内联**在 HTTP 请求里执行
// （连发邮件都在请求线程内），新实现把它们挪到独立进程，
// 因此慢的 SMTP / webhook 投递不再拖住 API 响应。
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/db"
	"codefever-next/backend/internal/events"
	"codefever-next/backend/internal/webhook"
)

func main() {
	var (
		baseDir = flag.String("base", envOr("CODEFEVER_BASE_DIR", "/data/www/codefever-community"), "应用根目录")
		logLvl  = flag.String("log-level", envOr("CODEFEVER_LOG_LEVEL", "info"), "日志级别 debug|info|warn|error")
	)
	flag.Parse()

	logger := newLogger(*logLvl)
	slog.SetDefault(logger)

	if err := run(*baseDir, logger); err != nil {
		logger.Error("worker 启动失败", "error", err)
		os.Exit(1)
	}
}

// run 装配并运行 worker。
func run(baseDir string, logger *slog.Logger) error {
	cfg, err := config.Load(baseDir)
	if err != nil {
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

	worker := events.NewWorker(sqlDB, events.BuildHandlers(sqlDB, cfg, logger), logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 投递器需要队列表里的投递状态列（rwe_status / rwe_attempts / rwe_next_time），
	// 缺列时按需补齐（幂等）。
	if err := webhook.EnsureSchema(ctx, sqlDB); err != nil {
		return err
	}

	// webhook 投递与事件消费并行跑：事件消费负责入队，
	// 投递器负责把已入队的事件真正发出去（对应 PHP 的 cron 控制器）。
	deliverer := webhook.NewDeliverer(sqlDB, cfg.Site.Host, webhook.NewHTTPSender(webhook.DefaultTimeout), logger)
	go func() {
		if err := deliverer.Run(ctx); err != nil {
			logger.Error("webhook 投递器退出", "error", err)
		}
	}()

	return worker.Run(ctx)
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
