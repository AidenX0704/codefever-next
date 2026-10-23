package repo

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// testDSN 返回集成测试用的 DSN；未设置时跳过。
//
// 这些用例需要真实 MySQL（验证事务与回滚的**真实**行为），
// 因此默认跳过，用 CODEFEVER_TEST_DSN 指定实例后运行：
//
//	$env:CODEFEVER_TEST_DSN = "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true"
//	go test ./internal/repo/ -run TestGroupRepoWithTxBoth -v
func testDSN() string { return os.Getenv("CODEFEVER_TEST_DSN") }

// TestGroupRepoWithTxBoth 验证 WithTxBoth 同时交出事务绑定的仓储与原生事务，
// 且回调返回错误时**真的回滚**。
//
// 这条机制是为领域事件服务的：事件必须与业务写入同事务入队
// （见 internal/events/bus.go 的 Emit 说明），
// 因此仓储必须能同时给出「绑定事务的仓储」与「原生 *sql.Tx」。
func TestGroupRepoWithTxBoth(t *testing.T) {
	dsn := testDSN()
	if dsn == "" {
		t.Skip("未设置 CODEFEVER_TEST_DSN，跳过需要真实 MySQL 的集成用例")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	repo := NewGroupRepo(db)
	ctx := context.Background()

	// 回调收到的事务必须非空。
	var gotRaw *sql.Tx
	var gotBound *GroupRepo
	if err := repo.WithTxBoth(ctx, func(groups *GroupRepo, raw *sql.Tx) error {
		gotRaw = raw
		gotBound = groups
		return nil
	}); err != nil {
		t.Fatalf("WithTxBoth 失败: %v", err)
	}
	if gotRaw == nil {
		t.Error("回调应收到原生事务")
	}
	if gotBound == nil {
		t.Error("回调应收到事务绑定的仓储")
	}

	// 回调返回错误 → 必须回滚（从事务外看不到未提交的数据）。
	const probe = "cf_tx_probe_should_not_exist"
	rollbackErr := repo.WithTxBoth(ctx, func(groups *GroupRepo, raw *sql.Tx) error {
		_, err := raw.ExecContext(ctx,
			"INSERT INTO cc_groups (g_key, u_key, g_type, g_display_name, g_name, g_description, g_avatar, g_created, g_updated, g_status) "+
				"VALUES (?, ?, 1, 'probe', ?, 'probe', '', NOW(), NOW(), 1)",
			probe, probe, probe)
		if err != nil {
			return err
		}
		return os.ErrInvalid
	})
	if rollbackErr == nil {
		t.Fatal("回调返回错误时 WithTxBoth 应当返回该错误")
	}

	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cc_groups WHERE g_key = ?", probe).Scan(&n); err != nil {
		t.Fatalf("校验回滚失败: %v", err)
	}
	if n != 0 {
		t.Error("回调报错后事务应被回滚，探针行不应残留")
	}
}
