package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"codefever-next/backend/internal/domain"
	"time"
)

// 事件状态。
const (
	// EventStatusPending 待处理。
	EventStatusPending = 1
	// EventStatusProcessing 处理中（已被 worker 领取）。
	EventStatusProcessing = 2
	// EventStatusDone 已处理。
	EventStatusDone = 3
	// EventStatusFailed 超过最大重试次数，进入死信。
	EventStatusFailed = 4
)

// MaxAttempts 是单个事件的最大处理次数，超过后进入死信状态。
const MaxAttempts = 5

// EventLease 是"处理中"事件的租约时长。
//
// worker 崩溃后，超过租约的 Processing 事件会被其它 worker 重新领取，
// 避免事件永久卡在 Processing（activity/notification/webhook 静默丢失）。
const EventLease = 2 * time.Minute

// Worker 消费事件队列并派发给各个 handler。
//
// 对应 PHP 侧的 service\Event\EventDispatcher：按事件类型匹配 handler
// （支持 '*' 通配与 "主事件:子事件" 的前缀匹配），全部匹配的 handler 都会被执行。
type Worker struct {
	db       *sql.DB
	handlers []Handler
	log      *slog.Logger

	// PollInterval 是轮询间隔。
	PollInterval time.Duration
	// BatchSize 是每轮领取的事件数。
	BatchSize int
}

// NewWorker 创建事件 worker。
func NewWorker(db *sql.DB, handlers []Handler, log *slog.Logger) *Worker {
	if log == nil {
		log = slog.Default()
	}
	return &Worker{
		db:           db,
		handlers:     handlers,
		log:          log,
		PollInterval: 2 * time.Second,
		BatchSize:    50,
	}
}

// Run 持续消费事件队列，直到 ctx 被取消。
func (w *Worker) Run(ctx context.Context) error {
	w.log.Info("事件 worker 已启动", "handlers", len(w.handlers), "interval", w.PollInterval)
	for {
		select {
		case <-ctx.Done():
			w.log.Info("事件 worker 收到退出信号")
			return nil
		default:
		}

		n, err := w.ProcessOnce(ctx)
		if err != nil {
			w.log.Error("处理事件批次失败", "error", err)
		}
		if n == 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(w.PollInterval):
			}
		}
	}
}

// queuedEvent 是一条待处理的事件。
type queuedEvent struct {
	key       string
	evType    string
	raw       string
	generated string
	timestamp int64
	language  string
}

// ProcessOnce 处理一批事件，返回本轮处理的事件数。
//
// 领取过程在一个事务里完成，并使用 `FOR UPDATE SKIP LOCKED`：
// 并发的多个 worker 会互相跳过已被锁定的行，从而**不会重复消费同一条事件**
// （早期版本先 SELECT 再 UPDATE，两个 worker 会拿到同一批行，导致活动流出现重复行）。
func (w *Worker) ProcessOnce(ctx context.Context) (int, error) {
	batch, err := w.claim(ctx)
	if err != nil {
		return 0, err
	}
	if len(batch) == 0 {
		return 0, nil
	}

	processed := 0
	for _, q := range batch {
		if err := w.handleOne(ctx, q.key, q.evType, q.raw, q.generated, q.timestamp, q.language); err != nil {
			w.log.Error("事件处理失败", "event", q.evType, "key", q.key, "error", err)
			continue
		}
		processed++
	}
	return processed, nil
}

// claim 原子地领取一批待处理事件。
//
// 事务内 `SELECT ... FOR UPDATE SKIP LOCKED` 锁定行，再逐条把状态改为"处理中"
// 并累加重试次数，最后提交。被别的 worker 锁住的行会被直接跳过。
func (w *Worker) claim(ctx context.Context) ([]queuedEvent, error) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT ev_key, ev_type, ev_data, ev_generated_by, ev_timestamp, ev_language
		FROM cc_events
		WHERE (ev_status = ?
		       OR (ev_status = ? AND (ev_lease_until IS NULL OR ev_lease_until < NOW())))
		  AND ev_attempts < ?
		ORDER BY ev_created ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED`, EventStatusPending, EventStatusProcessing, MaxAttempts, w.BatchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batch []queuedEvent
	for rows.Next() {
		var q queuedEvent
		if err := rows.Scan(&q.key, &q.evType, &q.raw, &q.generated, &q.timestamp, &q.language); err != nil {
			return nil, err
		}
		batch = append(batch, q)
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

	// 逐条置为"处理中"并累加重试次数。
	//
	// 注意占位符与参数的对应关系：IN 列表里每个 key 一个占位符，
	// 因此参数必须**逐个展开**（args...），不能把切片当单个参数传 ——
	// 后者会得到 "unsupported type []interface {}"，导致事件永远无法被消费。
	args := make([]any, 0, len(batch)+2)
	args = append(args, EventStatusProcessing, time.Now().Add(EventLease))
	for _, q := range batch {
		args = append(args, q.key)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE cc_events
		SET ev_status = ?, ev_attempts = ev_attempts + 1, ev_lease_until = ?
		WHERE ev_key IN (`+placeholders(len(batch))+`)`, args...); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return batch, nil
}

// handleOne 处理单个事件。
//
// 带 panic 兜底：某个 handler 的 bug 不应让事件永久停在 Processing，
// 也不应让整个 worker 进程退出（supervisor 重启会造成下一批事件也中断）。
func (w *Worker) handleOne(ctx context.Context, key, evType, raw, generated string, timestamp int64, language string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("处理事件时 panic: %v", r)
			w.markFailed(ctx, key, evType, err)
		}
	}()

	var payload domain.EventPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		// 载荷损坏无法恢复，直接进死信。
		w.markFailed(ctx, key, evType, fmt.Errorf("事件载荷损坏: %w", err))
		return err
	}
	payload.ID = key
	payload.Type = evType
	payload.GeneratedBy = generated
	payload.Timestamp = timestamp
	payload.Language = language

	var failures []error
	matched := 0
	for _, h := range w.handlers {
		if !handlerMatches(h, evType) {
			continue
		}
		matched++
		if err := h.Capture(ctx, payload); err != nil {
			failures = append(failures, fmt.Errorf("handler %T: %w", h, err))
		}
	}

	if len(failures) > 0 {
		err := errors.Join(failures...)
		w.markFailed(ctx, key, evType, err)
		return err
	}

	// 没有匹配到 handler 时同样视为已处理（对应 PHP：没有 handler 订阅则什么都不做）。
	_ = matched
	if _, err := w.db.ExecContext(ctx,
		"UPDATE cc_events SET ev_status = ?, ev_processed = ?, ev_lease_until = NULL WHERE ev_key = ?",
		EventStatusDone, time.Now(), key); err != nil {
		return err
	}
	return nil
}

// markFailed 标记事件失败；超过重试上限则进入死信。
func (w *Worker) markFailed(ctx context.Context, key, evType string, cause error) {
	var attempts int
	if err := w.db.QueryRowContext(ctx,
		"SELECT ev_attempts FROM cc_events WHERE ev_key = ?", key).Scan(&attempts); err != nil {
		w.log.Error("读取事件重试次数失败", "key", key, "error", err)
		return
	}
	status := EventStatusPending
	if attempts >= MaxAttempts {
		status = EventStatusFailed
	}
	if _, err := w.db.ExecContext(ctx,
		"UPDATE cc_events SET ev_status = ?, ev_lease_until = NULL WHERE ev_key = ?", status, key); err != nil {
		w.log.Error("更新事件状态失败", "key", key, "error", err)
	}
	if status == EventStatusFailed {
		w.log.Error("事件进入死信", "event", evType, "key", key, "attempts", attempts, "error", cause)
	}
}

// handlerMatches 判断 handler 是否关心该事件类型。
//
// 对应 PHP 侧 EventDispatcher::_getEventHandler 的三级匹配：
//   - '*' 通配 → 全部匹配；
//   - "主事件" 前缀匹配（"group" 匹配 "group:create"）；
//   - 完全匹配。
func handlerMatches(h Handler, evType string) bool {
	types := h.Types()
	if len(types) == 0 {
		// 未声明关心范围 → 视为通配（对应 PHP 的 '*'）。
		return true
	}
	for _, t := range types {
		if string(t) == "*" || string(t) == evType {
			return true
		}
		// 主事件前缀匹配：PHP 侧用 "主事件.*" 模式，此处等价为 "主事件:" 前缀。
		if len(evType) > len(t) && evType[:len(t)] == string(t) && evType[len(t)] == ':' {
			return true
		}
	}
	return false
}

// placeholders 生成 n 个逗号分隔的占位符。
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, 0, n*2-1)
	for i := 0; i < n; i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '?')
	}
	return string(buf)
}
