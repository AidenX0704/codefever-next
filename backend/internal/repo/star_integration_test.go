package repo

import (
	"context"
	"database/sql"
	"testing"

	"codefever-next/backend/internal/contract"
)

// 本文件覆盖仓库标星仓储的**真实数据库**行为。
//
// 需要真实 MySQL（涉及主键去重与 GROUP BY 统计，mock 无法覆盖）：
//
//	$env:CODEFEVER_TEST_DSN = "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true"
//	go test ./internal/repo/ -run TestStarRepo -v
//
// 覆盖的关键不变量：
//   - Star / Unstar 都是**幂等**的：重复标星不报错，取消未标的星也不报错；
//   - 计数与状态查询只反映真实行。

// TestStarRepoIdempotency 验证标星 / 取消标星的幂等语义。
func TestStarRepoIdempotency(t *testing.T) {
	dsn := testDSN()
	if dsn == "" {
		t.Skip("未设置 CODEFEVER_TEST_DSN，跳过需要真实 MySQL 的集成用例")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	stars := NewStarRepo(db)

	uKey := contract.NewKey()
	rKey := contract.NewKey()

	// 1) 首次标星。
	if err := stars.Star(ctx, uKey, rKey); err != nil {
		t.Fatalf("标星失败: %v", err)
	}
	// 2) 重复标星必须幂等（INSERT IGNORE 主键冲突被吞掉）。
	if err := stars.Star(ctx, uKey, rKey); err != nil {
		t.Fatalf("重复标星不应报错: %v", err)
	}
	// 3) 计数与状态。
	counts, err := stars.StarCounts(ctx, []string{rKey})
	if err != nil {
		t.Fatalf("统计标星数失败: %v", err)
	}
	if counts[rKey] != 1 {
		t.Errorf("重复标星后计数 = %d, want 1（主键去重）", counts[rKey])
	}
	starred, err := stars.IsStarred(ctx, uKey, rKey)
	if err != nil {
		t.Fatalf("查询标星状态失败: %v", err)
	}
	if !starred {
		t.Error("标星后应查询为已标星")
	}
	// 4) 批量状态查询。
	keys, err := stars.StarredKeys(ctx, uKey, []string{rKey, contract.NewKey()})
	if err != nil {
		t.Fatalf("批量查询标星状态失败: %v", err)
	}
	if !keys[rKey] || len(keys) != 1 {
		t.Errorf("批量状态 = %v, want 只含 rKey", keys)
	}
	// 5) 列出全部标星仓库。
	all, err := stars.StarredRepositories(ctx, uKey)
	if err != nil {
		t.Fatalf("列出标星仓库失败: %v", err)
	}
	if len(all) != 1 || all[0] != rKey {
		t.Errorf("标星列表 = %v, want [rKey]", all)
	}
	// 6) 取消标星（两次，幂等）。
	if err := stars.Unstar(ctx, uKey, rKey); err != nil {
		t.Fatalf("取消标星失败: %v", err)
	}
	if err := stars.Unstar(ctx, uKey, rKey); err != nil {
		t.Fatalf("取消未标的星不应报错: %v", err)
	}
	starred, err = stars.IsStarred(ctx, uKey, rKey)
	if err != nil {
		t.Fatalf("查询标星状态失败: %v", err)
	}
	if starred {
		t.Error("取消后不应再是已标星")
	}
	// 7) 清理验证：计数归零。
	counts, err = stars.StarCounts(ctx, []string{rKey})
	if err != nil {
		t.Fatalf("统计标星数失败: %v", err)
	}
	if counts[rKey] != 0 {
		t.Errorf("取消后计数 = %d, want 0", counts[rKey])
	}
}
