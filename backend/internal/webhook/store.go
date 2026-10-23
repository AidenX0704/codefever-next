package webhook

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"codefever-next/backend/internal/domain"
)

// Event 是一条待投递的 webhook 事件。
type Event struct {
	// Key 是事件主键（rwe_key）。
	Key string
	// RepoKey 是仓库主键（cc_repository_webhooks.r_key）。
	RepoKey string
	// Webhook 是 webhook 主键（rw_key）。
	Webhook string
	// Type 是事件类型（rwe_type）。
	Type string
	// Data 是事件数据（rwe_data，JSON 文本）。
	Data string
	// URL 是投递地址（rw_url）。
	URL string
	// Secret 是签名密钥（rw_secret）。
	Secret string
	// UserKey 是触发者主键（rwe_user）。
	UserKey string
	// RepoName 是仓库名（r_name）。
	RepoName string
	// GroupName 是仓库组名（g_name）。
	GroupName string
	// UserName 是触发者用户名（u_name）。
	UserName string
	// Attempts 是本轮投递的尝试次数（领取时已 +1）。
	Attempts int
}

// Log 是一条投递日志（对应 cc_repository_webhook_logs）。
type Log struct {
	// ID 是日志主键（rwl_id），与请求头 X-CodeFever-Id 相同。
	ID string
	// Webhook 是 webhook 主键（rw_key）。
	Webhook string
	// Request 是请求内容（rwl_request，JSON 文本）。
	Request string
	// Response 是响应内容（rwl_response，JSON 文本）。
	Response string
	// Start 是开始时间（rwl_start，Unix 秒）。
	Start float64
	// End 是结束时间（rwl_end，Unix 秒）。
	End float64
	// Status 是 HTTP 状态码（rwl_status），0 表示没有拿到响应。
	Status int
}

// Store 是 webhook 事件队列的存储抽象。
//
// 抽象出来是为了让投递逻辑（退避、死信、并发）可以脱离数据库单测，
// MySQL 实现见 MySQLStore。
type Store interface {
	// Claim 原子地领取一批待投递事件。
	//
	// 领取成功的行会被标记为"投递中"并累加尝试次数，
	// 租约（leaseUntil）到期前不会被再次领取。
	// 已经耗尽投递次数的行也会被领出来（由投递器直接标记死信），
	// 否则它们会永远卡在队列里。
	Claim(ctx context.Context, limit int, leaseUntil time.Time) ([]Event, error)
	// Delete 删除已成功投递的事件。
	Delete(ctx context.Context, eventKey string) error
	// Reschedule 记录一次失败：安排下次投递时间，dead 为真时标记为死信。
	Reschedule(ctx context.Context, eventKey string, next time.Time, dead bool) error
	// AddLog 写入一条投递日志。
	AddLog(ctx context.Context, l *Log) error
}

// MySQLStore 是基于 cc_repository_webhook_events / cc_repository_webhook_logs 的实现。
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore 创建 MySQL 存储。
func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

// Claim 原子地领取一批待投递事件。
//
// 逐字对齐 PHP 的 getRepositoryWebhookEvents()：只取启用（rw_active = 1）的
// webhook 的事件，按创建时间排序。
// 差别在于 PHP 是"查出来再逐条投递"，没有行锁 —— 多个投递器会重复投递同一条；
// 这里在事务里用 `FOR UPDATE SKIP LOCKED` 锁住行，被别的投递器占住的行直接跳过，
// 因此**不会重复投递**（与 internal/events.Worker 的领取方式一致）。
func (s *MySQLStore) Claim(ctx context.Context, limit int, leaseUntil time.Time) ([]Event, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("开启 webhook 领取事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT e.rwe_key, e.rwe_user, e.rwe_type, e.rwe_data, e.rwe_attempts,
		       rw.rw_key, rw.r_key, rw.rw_url, rw.rw_secret,
		       r.r_name, g.g_name, u.u_name
		FROM cc_repository_webhook_events AS e
		JOIN cc_repository_webhooks AS rw ON rw.rw_key = e.rw_key
		LEFT JOIN cc_repositories AS r ON r.r_key = rw.r_key AND r.r_status = ?
		LEFT JOIN cc_groups AS g ON g.g_key = r.g_key AND g.g_status = ?
		LEFT JOIN cc_users AS u ON u.u_key = e.rwe_user
		WHERE rw.rw_active = ?
		  AND e.rwe_status IN (?, ?)
		  AND (e.rwe_next_time IS NULL OR e.rwe_next_time <= NOW())
		ORDER BY e.rwe_created ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED`,
		domain.CommonStatusNormal, domain.CommonStatusNormal, domain.CommonStatusNormal,
		StatusPending, StatusDelivering, limit)
	if err != nil {
		return nil, fmt.Errorf("查询待投递 webhook 事件失败: %w", err)
	}
	defer rows.Close()

	var batch []Event
	for rows.Next() {
		var ev Event
		var attempts int
		var repoName, groupName, userName sql.NullString
		if err := rows.Scan(&ev.Key, &ev.UserKey, &ev.Type, &ev.Data, &attempts,
			&ev.Webhook, &ev.RepoKey, &ev.URL, &ev.Secret,
			&repoName, &groupName, &userName); err != nil {
			return nil, fmt.Errorf("扫描待投递 webhook 事件失败: %w", err)
		}
		ev.RepoName = repoName.String
		ev.GroupName = groupName.String
		ev.UserName = userName.String
		ev.Attempts = attempts + 1
		batch = append(batch, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(batch) == 0 {
		return nil, tx.Commit()
	}

	// 逐条标记为"投递中"并累加尝试次数，同时写入租约到期时间。
	//
	// 占位符与参数必须**逐个展开**：IN 列表里每个主键一个占位符。
	args := make([]any, 0, len(batch)+2)
	args = append(args, StatusDelivering, leaseUntil)
	for _, ev := range batch {
		args = append(args, ev.Key)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE cc_repository_webhook_events
		SET rwe_status = ?, rwe_attempts = rwe_attempts + 1, rwe_next_time = ?
		WHERE rwe_key IN (`+placeholders(len(batch))+`)`, args...); err != nil {
		return nil, fmt.Errorf("标记 webhook 事件为投递中失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交 webhook 领取事务失败: %w", err)
	}
	return batch, nil
}

// Delete 删除已成功投递的事件。
//
// 逐字对应 PHP 的 deleteRepositoryWebhookEvent()。
func (s *MySQLStore) Delete(ctx context.Context, eventKey string) error {
	if _, err := s.db.ExecContext(ctx,
		"DELETE FROM cc_repository_webhook_events WHERE rwe_key = ?", eventKey); err != nil {
		return fmt.Errorf("删除 webhook 事件失败: %w", err)
	}
	return nil
}

// Reschedule 记录一次投递失败。
//
// dead 为真时标记为死信（不再投递），否则安排下次投递时间（退避）。
func (s *MySQLStore) Reschedule(ctx context.Context, eventKey string, next time.Time, dead bool) error {
	if dead {
		if _, err := s.db.ExecContext(ctx, `
			UPDATE cc_repository_webhook_events
			SET rwe_status = ?, rwe_next_time = NULL
			WHERE rwe_key = ?`, StatusDead, eventKey); err != nil {
			return fmt.Errorf("标记 webhook 事件为死信失败: %w", err)
		}
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE cc_repository_webhook_events
		SET rwe_status = ?, rwe_next_time = ?
		WHERE rwe_key = ?`, StatusPending, next, eventKey); err != nil {
		return fmt.Errorf("安排 webhook 事件重投失败: %w", err)
	}
	return nil
}

// AddLog 写入一条投递日志。
//
// 逐字对应 PHP 的 addRepositoryWebhookLog()；
// rwl_created 用表默认值（当前时间）。
func (s *MySQLStore) AddLog(ctx context.Context, l *Log) error {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO cc_repository_webhook_logs
			(rwl_id, rw_key, rwl_request, rwl_response, rwl_start, rwl_end, rwl_status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.Webhook, l.Request, l.Response, l.Start, l.End, l.Status); err != nil {
		return fmt.Errorf("写入 webhook 投递日志失败: %w", err)
	}
	return nil
}

// schemaColumns 是投递器需要的队列表附加列。
var schemaColumns = []struct {
	name string
	ddl  string
}{
	{"rwe_status", "`rwe_status` tinyint(3) NOT NULL DEFAULT '1' COMMENT '投递状态：1 待投递 2 投递中 3 死信'"},
	{"rwe_attempts", "`rwe_attempts` int(11) NOT NULL DEFAULT '0' COMMENT '已投递次数'"},
	{"rwe_next_time", "`rwe_next_time` datetime NULL DEFAULT NULL COMMENT '下次可投递时间（退避到期或租约到期）'"},
}

// EnsureSchema 补齐队列表的投递状态列。
//
// cc_repository_webhook_events 是 PHP 侧建的表，原本只有入队所需的列
// （rwe_key / rwe_user / rw_key / rwe_type / rwe_data / rwe_created），
// 没有承载"投递状态 / 重试次数 / 下次可投递时间"的列。
//
// 这里按需追加三列，与 cc_events 的 ev_status / ev_attempts 同构：
//
//	rwe_status     1 待投递 / 2 投递中 / 3 死信
//	rwe_attempts   已投递次数
//	rwe_next_time  下次可投递时间（退避到期或租约到期）
//
// 三列都有默认值，PHP 侧不写这些列，因此**不影响 PHP 的读写**。
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	existing, err := existingColumns(ctx, db, "cc_repository_webhook_events")
	if err != nil {
		return err
	}

	missing := make([]string, 0, len(schemaColumns))
	for _, col := range schemaColumns {
		if !existing[col.name] {
			missing = append(missing, col.ddl)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	if _, err := db.ExecContext(ctx,
		"ALTER TABLE cc_repository_webhook_events ADD COLUMN "+strings.Join(missing, ", ADD COLUMN ")); err != nil {
		return fmt.Errorf("补齐 webhook 队列表列失败: %w", err)
	}
	return nil
}

// existingColumns 读出表里已有的列名。
func existingColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT COLUMN_NAME FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table)
	if err != nil {
		return nil, fmt.Errorf("查询表结构失败: %w", err)
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("扫描表结构失败: %w", err)
		}
		out[name] = true
	}
	return out, rows.Err()
}

// placeholders 生成 n 个逗号分隔的占位符。
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
