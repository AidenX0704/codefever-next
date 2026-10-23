// Package events 实现领域事件总线。
//
// 与现有 PHP 实现的差异（有意为之）：
//
//	PHP：事件在请求线程内**同步内联**派发 —— 活动流写库、通知写库、
//	     甚至**发邮件**都在请求里完成，一个慢的 SMTP 会拖住整个 HTTP 响应。
//
//	新实现：事件与业务写入**同一事务**入队（cc_events 表），由后台 worker
//	     异步消费。这样业务写入的成功性与事件的存在性严格绑定，
//	     且邮件/webhook 的延迟不再影响 API 响应。
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
)

// Bus 是事件总线。
type Bus struct {
	db *sql.DB
	// Language 是事件语言标记（PHP 侧取自 HTTP_FRONT_END_LANG 头，默认 zh-cn）。
	Language string
}

// NewBus 创建事件总线。
func NewBus(db *sql.DB) *Bus {
	return &Bus{db: db, Language: domain.LanguageCN}
}

// Emit 发送一个事件。
//
// tx 必须与产生该事件的业务写入是**同一个事务**，从而保证：
// 业务提交 → 事件必在；业务回滚 → 事件消失。
//
// eventType 与 eventData 会被 domain.ValidateEventData 按事件注册表校验，
// 校验失败返回错误（对应 PHP 的异常 2001~2004）。
func (b *Bus) Emit(ctx context.Context, tx *sql.Tx, eventType domain.EventType, desc domain.EventDataDesc, data map[string]any, generatedBy string) error {
	if desc == "" {
		return &domain.EventError{Code: 2002, Msg: fmt.Sprintf("事件 %q 没有数据描述", eventType)}
	}
	normalized := domain.NormalizeEventData(data)
	if err := domain.ValidateEventData(desc, normalized); err != nil {
		return err
	}

	payload := domain.EventPayload{
		ID:          contract.NewKey(),
		Type:        string(eventType),
		Data:        normalized,
		GeneratedBy: generatedBy,
		Timestamp:   time.Now().Unix(),
		Language:    b.Language,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化事件失败: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO cc_events (ev_key, ev_type, ev_data, ev_generated_by, ev_timestamp, ev_language, ev_status, ev_created)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?)`,
		payload.ID, payload.Type, string(raw), payload.GeneratedBy, payload.Timestamp, payload.Language, time.Now())
	if err != nil {
		return fmt.Errorf("事件入队失败: %w", err)
	}
	return nil
}

// EmitSimple 是 Emit 的便捷封装：事件类型与描述成对给出。
func (b *Bus) EmitSimple(ctx context.Context, tx *sql.Tx, eventType domain.EventType, desc domain.EventDataDesc, data map[string]any, generatedBy string) error {
	return b.Emit(ctx, tx, eventType, desc, data, generatedBy)
}

// Handler 是事件处理器。
//
// 对应 PHP 侧 event_handlers/ 下的 ActivityHandler / UserNotificationHandler /
// WebhookHandler 的 capture() 方法。
type Handler interface {
	// Capture 处理一个事件。
	Capture(ctx context.Context, ev domain.EventPayload) error
	// Types 返回该 handler 关心的事件类型；空切片表示关心全部（对应 PHP 的 '*' 通配）。
	Types() []domain.EventType
}

// HandlerFunc 把普通函数适配为 Handler。
type HandlerFunc struct {
	// EventTypes 关心的事件类型；nil 表示全部。
	EventTypes []domain.EventType
	// Fn 处理函数。
	Fn func(ctx context.Context, ev domain.EventPayload) error
}

// Capture 实现 Handler。
func (h *HandlerFunc) Capture(ctx context.Context, ev domain.EventPayload) error {
	return h.Fn(ctx, ev)
}

// Types 实现 Handler。
func (h *HandlerFunc) Types() []domain.EventType { return h.EventTypes }
