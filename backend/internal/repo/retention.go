package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// 本文件实现追加型表的保留期清理（性能整改 D2，2026-10）。
//
// ## 为什么需要
//
// 整改前全仓库只有一个清理任务：cc_sessions（cmd/codefever-api/main.go 的
// cleanupSessions）。而下列五张表都是"每次操作写一行、永不删除"：
//
//	cc_events                  事件队列（业务写同事务入队，worker 异步消费）
//	cc_activities              仓库动态流
//	cc_notification_content    站内信内容（**每个收件人一行**，见
//	                           events/notification.go 的 addWebSiteMessage）
//	cc_notification_users      站内信的已读状态（每个收件人一行）
//	cc_repository_webhook_logs webhook 投递日志
//
// cc_events 尤其危险：worker 每 2 秒按 ev_created 排序领取待处理事件，
// 已处理（ev_status=3）的行永久堆积会把这个候选集越拖越大。
//
// ## 安全边界（每条都刻意如此）
//
//   - cc_events 只删**终态**行：ev_status=3（已处理）按天数删，
//     ev_status=4（超过重试上限的死信）保留两倍天数，便于事后排障。
//     待处理(1)/处理中(2) 永远不会被删。
//   - cc_notification_users 只删**已读**行（nu_is_read=1），未读的站内信
//     不因为"时间久了"而消失。
//   - cc_notification_content 只删**已无任何用户行引用**且 nc_scope=2 的行
//     （scope=2 是 NotifyForUser，即 per-recipient 写入的那一类；
//     广播类通知不在此处删除，避免删掉所有人可见的公告）。
//   - cc_activities 是**用户可见的产品数据**（仓库动态），默认**不清理**，
//     天数配成 0 即关闭；需要保留策略时由运维显式开启。
//   - 所有删除都带 LIMIT 分批，并在批次之间检查 ctx，避免一次大删除
//     长时间持锁、把主从延迟拉爆。
type RetentionConfig struct {
	// EventDoneDays 是"已处理事件"的保留天数。<=0 表示不清理。
	EventDoneDays int
	// EventFailedDays 是"死信事件"的保留天数。<=0 表示不清理。
	EventFailedDays int
	// NotificationDays 是站内信（用户行 + 孤儿内容行）的保留天数。<=0 不清理。
	NotificationDays int
	// WebhookLogDays 是 webhook 投递日志的保留天数。<=0 不清理。
	WebhookLogDays int
	// ActivityDays 是仓库动态的保留天数。**默认 0（不清理）**，见上文说明。
	ActivityDays int
	// BatchSize 是单条 DELETE 的行数上限。
	BatchSize int
}

// RetentionResult 是一次清理的影响行数。
type RetentionResult struct {
	EventsDone          int64
	EventsFailed        int64
	NotificationUsers   int64
	NotificationContent int64
	WebhookLogs         int64
	Activities          int64
}

// Empty 判断本次清理是否什么都没删。
func (r RetentionResult) Empty() bool {
	return r.EventsDone == 0 && r.EventsFailed == 0 && r.NotificationUsers == 0 &&
		r.NotificationContent == 0 && r.WebhookLogs == 0 && r.Activities == 0
}

// RetentionService 负责追加型表的保留期清理。
type RetentionService struct {
	db  *sql.DB
	cfg RetentionConfig
}

// NewRetentionService 创建清理服务。
func NewRetentionService(db *sql.DB, cfg RetentionConfig) *RetentionService {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 2000
	}
	return &RetentionService{db: db, cfg: cfg}
}

// Run 执行一轮清理，返回影响行数。
//
// 各表的清理互相独立：单表失败会记在返回的 error 里，但不会阻止其它表，
// 避免一张表的问题把整套保留期策略一起卡住。
func (s *RetentionService) Run(ctx context.Context) (RetentionResult, error) {
	var res RetentionResult
	var firstErr error
	fail := func(what string, err error) {
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", what, err)
		}
	}

	if s.cfg.EventDoneDays > 0 {
		n, err := s.deleteBatched(ctx,
			"DELETE FROM cc_events WHERE ev_status = 3 AND ev_created < ? LIMIT ?",
			s.cutoff(s.cfg.EventDoneDays))
		res.EventsDone = n
		fail("清理已处理事件", err)
	}
	if s.cfg.EventFailedDays > 0 {
		n, err := s.deleteBatched(ctx,
			"DELETE FROM cc_events WHERE ev_status = 4 AND ev_created < ? LIMIT ?",
			s.cutoff(s.cfg.EventFailedDays))
		res.EventsFailed = n
		fail("清理死信事件", err)
	}
	if s.cfg.NotificationDays > 0 {
		cut := s.cutoff(s.cfg.NotificationDays)
		// 先删已读的用户行，再清孤儿内容行（顺序不能反，否则会误判引用关系）。
		n, err := s.deleteBatched(ctx,
			"DELETE FROM cc_notification_users WHERE nu_is_read = 1 AND nu_created < ? LIMIT ?", cut)
		res.NotificationUsers = n
		fail("清理已读站内信", err)

		n, err = s.deleteBatched(ctx,
			`DELETE FROM cc_notification_content
			  WHERE nc_scope = 2
			    AND nc_created < ?
			    AND NOT EXISTS (
			          SELECT 1 FROM cc_notification_users u
			           WHERE u.nc_key = cc_notification_content.nc_key
			    )
			  LIMIT ?`, cut)
		res.NotificationContent = n
		fail("清理孤儿站内信内容", err)
	}
	if s.cfg.WebhookLogDays > 0 {
		n, err := s.deleteBatched(ctx,
			"DELETE FROM cc_repository_webhook_logs WHERE rwl_created < ? LIMIT ?",
			s.cutoff(s.cfg.WebhookLogDays))
		res.WebhookLogs = n
		fail("清理 webhook 投递日志", err)
	}
	if s.cfg.ActivityDays > 0 {
		n, err := s.deleteBatched(ctx,
			"DELETE FROM cc_activities WHERE a_created < ? LIMIT ?",
			s.cutoff(s.cfg.ActivityDays))
		res.Activities = n
		fail("清理仓库动态", err)
	}

	return res, firstErr
}

// cutoff 返回"早于此刻即过期"的时间点。
func (s *RetentionService) cutoff(days int) time.Time {
	return time.Now().AddDate(0, 0, -days)
}

// deleteBatched 分批删除，直到某一批影响行数小于批次大小。
//
// 分批的意义：一次性 DELETE 几十万行会长时间持有行锁、拉高主从延迟，
// 还可能因为单个事务过大而写爆 undo log。
//
// 注意：MySQL **不支持**在多表 DELETE（`DELETE a FROM ... JOIN ...`）上使用
// LIMIT，因此这里的语句必须是单表 DELETE；"内容行是否还有人引用"的判断
// 用相关子查询（NOT EXISTS）表达，而不是 LEFT JOIN。
// （口径变更点：改为 NOT EXISTS 后不再受 LEFT JOIN 的 NULL 语义影响，
//
//	且 nc_key 为 NOT NULL，因此判定与原 LEFT JOIN 写法等价而更安全。）
func (s *RetentionService) deleteBatched(ctx context.Context, query string, args ...any) (int64, error) {
	var total int64
	batch := s.cfg.BatchSize
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		fullArgs := append(append([]any{}, args...), batch)
		result, err := s.db.ExecContext(ctx, query, fullArgs...)
		if err != nil {
			return total, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return total, err
		}
		total += n
		if n < int64(batch) {
			return total, nil
		}
	}
}
