// Package db 负责数据库连接与版本化迁移。
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"codefever-next/backend/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Open 建立数据库连接池。
func Open(cfg *config.Config) (*sql.DB, error) {
	sqlDB, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	// 及时回收空闲连接，避免 MySQL 侧 wait_timeout 先断开后客户端仍复用。
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	return sqlDB, nil
}

// Migrate 应用所有未执行的迁移。
//
// 迁移文件按文件名前缀的数字顺序执行，执行记录写入 cc_schema_migrations。
// 每个迁移在独立事务中执行，失败即回滚并中止后续迁移。
//
// ## 并发保护
//
// codefever-api 与 codefever-worker **都会在启动时调用本函数**，两者同时起来时
// 会出现经典竞态：双方都查到「该迁移未执行」→ 双方都执行 → 双方都 INSERT 记录，
// 后提交的一方撞上 `Duplicate entry` 并直接退出（表现为 worker 启动失败、
// 由 supervisor 重启一次才恢复）。
//
// 因此整个迁移过程在 **MySQL 命名锁**（GET_LOCK）下串行化：拿到锁的进程执行完并
// 释放后，后到的进程重新查询即可正确跳过已执行的迁移。命名锁是会话级的，
// 所以必须全程绑定**同一条连接**。
func Migrate(ctx context.Context, sqlDB *sql.DB) error {
	const lockName = "codefever_schema_migrate"
	const lockWaitSeconds = 60

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取迁移专用连接失败: %w", err)
	}
	defer conn.Close()

	var locked sql.NullInt64
	if err := conn.QueryRowContext(ctx,
		"SELECT GET_LOCK(?, ?)", lockName, lockWaitSeconds).Scan(&locked); err != nil {
		return fmt.Errorf("获取迁移锁失败: %w", err)
	}
	if !locked.Valid || locked.Int64 != 1 {
		return fmt.Errorf("获取迁移锁失败：等待 %d 秒仍未获得锁（另一个进程可能仍在迁移）", lockWaitSeconds)
	}
	defer func() {
		// 释放锁必须用同一条连接；连接归还连接池时 MySQL 也会自动释放会话锁，
		// 但这里显式释放，避免锁意外滞留到连接被复用。
		relCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(relCtx, "SELECT RELEASE_LOCK(?)", lockName)
	}()

	if _, err := sqlDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS cc_schema_migrations (
			version VARCHAR(128) NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (version)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied int
		if err := sqlDB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM cc_schema_migrations WHERE version = ?", name).Scan(&applied); err != nil {
			return fmt.Errorf("查询迁移状态失败: %w", err)
		}
		if applied > 0 {
			continue
		}

		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("读取迁移 %s 失败: %w", name, err)
		}
		if err := applyMigration(ctx, sqlDB, name, string(body)); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration 在单个事务中执行一个迁移文件。
func applyMigration(ctx context.Context, sqlDB *sql.DB, name, body string) error {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启迁移事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range splitStatements(body) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("迁移 %s 执行失败 (%s): %w", name, truncate(stmt, 120), err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO cc_schema_migrations (version) VALUES (?)", name); err != nil {
		return fmt.Errorf("记录迁移 %s 失败: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交迁移 %s 失败: %w", name, err)
	}
	return nil
}

// splitStatements 按分号切分 SQL 脚本，忽略注释与空语句。
func splitStatements(body string) []string {
	var out []string
	for _, raw := range strings.Split(body, ";") {
		var lines []string
		for _, line := range strings.Split(raw, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			lines = append(lines, line)
		}
		stmt := strings.TrimSpace(strings.Join(lines, "\n"))
		if stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}

// truncate 截断字符串用于错误信息。
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
