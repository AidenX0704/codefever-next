package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestRetentionServiceRun 验证保留期清理的安全边界（性能整改 D2）。
//
// 需要真实 MySQL（涉及 DELETE ... LIMIT、NOT EXISTS 相关子查询与
// 多表/单表语义差异，mock 无法覆盖）：
//
//	$env:CODEFEVER_TEST_DSN = "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true"
//	go test ./internal/repo/ -run TestRetentionServiceRun -v
//
// 覆盖的关键不变量：
//   - 事件：只删**终态**（已处理 / 死信）且超期的行，待处理行绝不删；
//   - 死信保留期是已处理的两倍（便于排障）；
//   - 站内信：只删**已读**用户行；
//   - 站内信内容：只删 scope=2 且**已无任何用户行引用**的行；
//   - 仍有用户行引用的内容行必须保留（否则用户的通知会变成空壳）。
func TestRetentionServiceRun(t *testing.T) {
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

	const p = "cfret_"
	// 事件夹具：状态 + 年龄。
	events := []struct {
		key     string
		status  int
		ageDays int
	}{
		{p + "done_old", 3, 40},    // 已处理 + 超期      -> 删
		{p + "done_new", 3, 5},     // 已处理 + 未超期    -> 留
		{p + "pending_old", 1, 40}, // 待处理 + 超期      -> 留（绝不删）
		{p + "failed_old", 4, 70},  // 死信 + 超 60 天    -> 删
		{p + "failed_new", 4, 40},  // 死信 + 未超 60 天  -> 留
	}

	defer func() {
		for _, e := range events {
			_, _ = db.ExecContext(ctx, "DELETE FROM cc_events WHERE ev_key = ?", e.key)
		}
		_, _ = db.ExecContext(ctx, "DELETE FROM cc_notification_users WHERE nu_key IN (?, ?)", p+"nu_read", p+"nu_unread")
		_, _ = db.ExecContext(ctx, "DELETE FROM cc_notification_content WHERE nc_key IN (?, ?, ?)", p+"nc_orphan", p+"nc_kept", p+"nc_public")
	}()

	for _, e := range events {
		// ev_attempts 故意设为远超 events.MaxAttempts(5)：事件 worker 的领取
		// 查询带 `AND ev_attempts < MaxAttempts`，这样它们不会被**正在运行的
		// worker** 抢走并改写状态，本用例才能稳定地只衡量保留期逻辑
		// （早先不设该值时，待处理事件会被 worker 消费成"已处理"，
		//  随后被 30 天规则删掉，造成误报）。
		if _, err := db.ExecContext(ctx,
			"INSERT INTO cc_events (ev_key, ev_type, ev_data, ev_generated_by, ev_timestamp, ev_language, ev_status, ev_created, ev_processed, ev_attempts) "+
				"VALUES (?, '1', '{}', 'test', NOW(), 'zh-cn', ?, DATE_SUB(NOW(), INTERVAL ? DAY), NOW(), 999)",
			e.key, e.status, e.ageDays); err != nil {
			t.Fatalf("插入事件 %s 失败: %v", e.key, err)
		}
	}

	// 站内信内容：一个孤儿（可删）、一个仍被未读用户引用（必须保留）、
	// 一个广播类 scope=1（不在清理范围）。
	for _, row := range []struct {
		key   string
		scope int
		age   int
	}{
		{p + "nc_orphan", 2, 100},
		{p + "nc_kept", 2, 100},
		{p + "nc_public", 1, 400},
	} {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO cc_notification_content (nc_key, nc_type, g_key, r_key, nc_data, nc_scope, nc_created, nc_updated, nc_deleted, nc_status) "+
				"VALUES (?, '1', '', '', '{}', ?, DATE_SUB(NOW(), INTERVAL ? DAY), NOW(), NOW(), 1)",
			row.key, row.scope, row.age); err != nil {
			t.Fatalf("插入通知内容 %s 失败: %v", row.key, err)
		}
	}
	// 已读 + 超期 -> 删；未读 + 超期 -> 留（它的内容行也要留着）。
	if _, err := db.ExecContext(ctx,
		"INSERT INTO cc_notification_users (nu_key, u_key, nc_key, nu_is_read, nu_created, nu_updated, nu_deleted, nu_status) "+
			"VALUES (?, 'cfret_u1', ?, 1, DATE_SUB(NOW(), INTERVAL 100 DAY), NOW(), NOW(), 1)",
		p+"nu_read", p+"nc_orphan"); err != nil {
		t.Fatalf("插入已读用户行失败: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO cc_notification_users (nu_key, u_key, nc_key, nu_is_read, nu_created, nu_updated, nu_deleted, nu_status) "+
			"VALUES (?, 'cfret_u2', ?, 0, DATE_SUB(NOW(), INTERVAL 100 DAY), NOW(), NOW(), 1)",
		p+"nu_unread", p+"nc_kept"); err != nil {
		t.Fatalf("插入未读用户行失败: %v", err)
	}

	svc := NewRetentionService(db, RetentionConfig{
		EventDoneDays:    30,
		EventFailedDays:  60,
		NotificationDays: 90,
		WebhookLogDays:   90,
		ActivityDays:     0,
		BatchSize:        500,
	})
	start := time.Now()
	if _, err := svc.Run(ctx); err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	_ = start

	// 事件：只有 done_old 与 failed_old 应当消失。
	expect := map[string]bool{
		p + "done_old":    false,
		p + "done_new":    true,
		p + "pending_old": true,
		p + "failed_old":  false,
		p + "failed_new":  true,
	}
	for key, shouldExist := range expect {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cc_events WHERE ev_key = ?", key).Scan(&n); err != nil {
			t.Fatalf("查询事件 %s 失败: %v", key, err)
		}
		if (n > 0) != shouldExist {
			t.Errorf("事件 %s 存在=%v, 期望 %v（终态/年龄的保留策略不符）", key, n > 0, shouldExist)
		}
	}

	// 站内信内容：孤儿删除，被引用与广播保留。
	for key, shouldExist := range map[string]bool{
		p + "nc_orphan": false,
		p + "nc_kept":   true,
		p + "nc_public": true,
	} {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cc_notification_content WHERE nc_key = ?", key).Scan(&n); err != nil {
			t.Fatalf("查询内容 %s 失败: %v", key, err)
		}
		if (n > 0) != shouldExist {
			t.Errorf("通知内容 %s 存在=%v, 期望 %v", key, n > 0, shouldExist)
		}
	}

	// 用户行：已读删除，未读保留。
	for key, shouldExist := range map[string]bool{
		p + "nu_read":   false,
		p + "nu_unread": true,
	} {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cc_notification_users WHERE nu_key = ?", key).Scan(&n); err != nil {
			t.Fatalf("查询用户行 %s 失败: %v", key, err)
		}
		if (n > 0) != shouldExist {
			t.Errorf("站内信用户行 %s 存在=%v, 期望 %v", key, n > 0, shouldExist)
		}
	}

	// 全部保留天数配 0 时不应改动任何数据（相当于关闭）。
	off := NewRetentionService(db, RetentionConfig{BatchSize: 500})
	res, err := off.Run(ctx)
	if err != nil {
		t.Fatalf("关闭状态下的清理不应报错: %v", err)
	}
	if !res.Empty() {
		t.Errorf("全部保留期为 0 时不应删除任何行, 实际 %+v", res)
	}
}
