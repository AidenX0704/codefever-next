package repo

import (
	"context"
	"database/sql"
	"testing"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
)

// 本文件覆盖合并请求评论的**真实数据库**行为。
//
// 需要真实 MySQL：
//
//	$env:CODEFEVER_TEST_DSN = "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true"
//	go test ./internal/repo/ -run TestMRComment -v
//
// 覆盖的关键不变量：
//   - 评论经 RepositoryTx 写入（与事件同事务的路径），LastInsertId 回填；
//   - 列表按 c_id asc（时间正序）；
//   - Participants 取「创建者 + 评审人」，评审人只取 mrr_status = 1 的行。

// TestMRCommentTxAndList 验证事务内写评论与列表读取。
func TestMRCommentTxAndList(t *testing.T) {
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
	writes := NewRepositoryWriteRepo(db)
	comments := NewMRCommentRepo(db)

	mrKey := contract.NewKey()
	defer func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM cc_merge_request_comments WHERE mr_id = ?", mrKey)
	}()

	// 1) 事务内写两条评论（生产路径：与评论事件同一个事务）。
	var firstID int64
	if err := writes.WithTx(ctx, func(tx RepositoryTx, raw *sql.Tx) error {
		var txErr error
		firstID, txErr = tx.CreateMRComment(ctx, mrKey, "u-commenter", "第一条")
		return txErr
	}); err != nil {
		t.Fatalf("事务内写评论失败: %v", err)
	}
	if firstID == 0 {
		t.Fatal("评论主键应回填")
	}
	if err := writes.WithTx(ctx, func(tx RepositoryTx, raw *sql.Tx) error {
		_, txErr := tx.CreateMRComment(ctx, mrKey, "u-commenter-2", "第二条")
		return txErr
	}); err != nil {
		t.Fatalf("事务内写第二条评论失败: %v", err)
	}

	// 2) 列表按 c_id asc。
	list, err := comments.List(ctx, mrKey)
	if err != nil {
		t.Fatalf("列出评论失败: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("评论数 = %d, want 2", len(list))
	}
	if list[0].ID >= list[1].ID {
		t.Errorf("列表应按 c_id asc: %d >= %d", list[0].ID, list[1].ID)
	}
	if list[0].Body != "第一条" || list[0].MRKey != mrKey {
		t.Errorf("第一条评论内容不正确: %+v", list[0])
	}

	// 3) 列出空 MR 不报错。
	empty, err := comments.List(ctx, contract.NewKey())
	if err != nil {
		t.Fatalf("空列表失败: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("空 MR 的评论数 = %d, want 0", len(empty))
	}
}

// TestMRCommentParticipants 验证「创建者 + 评审人」的取人口径。
func TestMRCommentParticipants(t *testing.T) {
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
	comments := NewMRCommentRepo(db)

	mrKey := contract.NewKey()
	mrrKeys := make([]string, 0, 2)
	defer func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM cc_merge_request_comments WHERE mr_id = ?", mrKey)
		_, _ = db.ExecContext(ctx, "DELETE FROM cc_merge_request_reviewers WHERE mr_key = ?", mrKey)
		_, _ = db.ExecContext(ctx, "DELETE FROM cc_merge_requests WHERE mr_key = ?", mrKey)
	}()

	// 造一个 MR：创建者 u-creator，两个评审人（一个已评审、一个未评审）。
	if _, err := db.ExecContext(ctx, `
		INSERT INTO cc_merge_requests (mr_key, u_key, mr_source_r_key, mr_source_branch,
			mr_target_r_key, mr_target_branch, mr_start_hash, mr_id, mr_title,
			mr_created, mr_updated, mr_status)
		VALUES (?, 'u-creator', 'r-src', 'feature', 'r-dst', 'master', 'hash', 999999,
			'participants-probe', NOW(), NOW(), 1)`, mrKey); err != nil {
		t.Fatalf("插入探测 MR 失败: %v", err)
	}
	for i, reviewer := range []string{"u-reviewer-1", "u-reviewer-2"} {
		mrrKey := contract.NewKey()
		isReview := domain.GlobalFalse
		if i == 0 {
			isReview = domain.GlobalTrue
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO cc_merge_request_reviewers (mrr_key, mr_key, u_key, mrr_is_review, mrr_status)
			VALUES (?, ?, ?, ?, ?)`,
			mrrKey, mrKey, reviewer, isReview, domain.CommonStatusNormal); err != nil {
			t.Fatalf("插入探测评审人失败: %v", err)
		}
		mrrKeys = append(mrrKeys, mrrKey)
	}
	// 一条已软删除的评审人行：不应被算进受众。
	if _, err := db.ExecContext(ctx, `
		INSERT INTO cc_merge_request_reviewers (mrr_key, mr_key, u_key, mrr_is_review, mrr_status)
		VALUES (?, ?, 'u-reviewer-deleted', ?, ?)`,
		contract.NewKey(), mrKey, domain.GlobalFalse, domain.CommonStatusDelete); err != nil {
		t.Fatalf("插入已删除评审人失败: %v", err)
	}

	got, err := comments.Participants(ctx, mrKey)
	if err != nil {
		t.Fatalf("查询参与者失败: %v", err)
	}
	if got.Creator != "u-creator" {
		t.Errorf("创建者 = %q, want u-creator", got.Creator)
	}
	if len(got.Reviewers) != 2 {
		t.Fatalf("评审人数 = %d, want 2（软删除的不算）: %v", len(got.Reviewers), got.Reviewers)
	}
	set := map[string]bool{}
	for _, r := range got.Reviewers {
		set[r] = true
	}
	if !set["u-reviewer-1"] || !set["u-reviewer-2"] {
		t.Errorf("评审人集合不正确: %v", got.Reviewers)
	}

	// 不存在的 MR 返回 ErrNotFound。
	if _, err := comments.Participants(ctx, contract.NewKey()); err != ErrNotFound {
		t.Errorf("不存在的 MR 应返回 ErrNotFound, got %v", err)
	}
}
