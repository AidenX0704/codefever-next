package events

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
)

// webhookHandler 把领域事件写进仓库 webhook 队列（cc_repository_webhook_events）。
//
// 逐字对应 application/event_handlers/WebhookHandler.php：
// 它监听 26 个事件，把事件数据按"该仓库订阅了哪些事件"分发到各个 webhook。
//
// 与 PHP 的差异：PHP 的投递由 controllers/backend/repository_webhook.php
// 用 cron 拉取队列后 Guzzle POST 出去；新实现同样**只负责入队**，
// 投递由独立的投递器负责（cc_repository_webhook_logs 记录投递结果）。
type webhookHandler struct {
	db  *sql.DB
	log *slog.Logger
}

// NewWebhookHandler 创建 webhook 处理器。
func NewWebhookHandler(db *sql.DB, logger *slog.Logger) Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &webhookHandler{db: db, log: logger}
}

// webhookEventTypes 是 WebhookHandler 关心的全部事件。
//
// 逐字对应 application/event_handlers/WebhookHandler.php:10-41 的
// $ListenedEventList（PHP 侧实际是 26 个：仓库 11 + 分支 6 + 标签 2 +
// 合并请求 6 + 仓库 push 1）。
func webhookEventTypes() []domain.EventType {
	return []domain.EventType{
		// 仓库事件（11 个）
		domain.EventTypeREPO_CREATE,
		domain.EventTypeREPO_FORK,
		domain.EventTypeREPO_UPDATE_AVATAR,
		domain.EventTypeREPO_UPDATE_NAME,
		domain.EventTypeREPO_UPDATE_DESCRIPTION,
		domain.EventTypeREPO_ADD_MEMBER,
		domain.EventTypeREPO_CHANGE_MEMBER_ROLE,
		domain.EventTypeREPO_REMOVE_MEMBER,
		domain.EventTypeREPO_CHANGE_OWNER,
		domain.EventTypeREPO_CHANGE_URL,
		domain.EventTypeREPO_REMOVE,

		// 分支事件（6 个）
		domain.EventTypeBRANCH_CREATE,
		domain.EventTypeBRANCH_REMOVE,
		domain.EventTypeDEFAULT_BRANCH_CHANGE,
		domain.EventTypePROTECTED_BRANCH_RULE_CREATE,
		domain.EventTypePROTECTED_BRANCH_RULE_CHANGE,
		domain.EventTypePROTECTED_BRANCH_RULE_REMOVE,

		// 标签事件（2 个）
		domain.EventTypeTAG_CREATE,
		domain.EventTypeTAG_REMOVE,

		// 合并请求事件（6 个）
		domain.EventTypeMERGE_REQUEST_CREATE,
		domain.EventTypeMERGE_REQUEST_CLOSE,
		domain.EventTypeMERGE_REQUEST_MERGE,
		domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE,
		domain.EventTypeMERGE_REQUEST_REVIEWER_DELETE,
		domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW,

		// 仓库 push 事件（1 个）
		domain.EventTypeHOOK_POST_RECEIVE,
	}
}

// Types 实现 Handler。
func (h *webhookHandler) Types() []domain.EventType { return webhookEventTypes() }

// Capture 实现 Handler。
func (h *webhookHandler) Capture(ctx context.Context, ev domain.EventPayload) error {
	rKey := eventDataString(ev.Data, "rKey")
	if rKey == "" {
		// PHP 侧 addRepositoryWebhookEvent() 对空主键直接返回 FALSE。
		return nil
	}

	// rwe_data 是事件数据的 JSON —— 逐字对应 PHP 的 `$event->data->getData()`
	// （EventData::getData() 就是 json_encode($this->_dataStream)）。
	payload, err := phpJSONEncode(ev.Data, false)
	if err != nil {
		return fmt.Errorf("序列化 webhook 事件数据失败: %w", err)
	}

	webhooks, err := h.repositoryWebhooks(ctx, rKey)
	if err != nil {
		return err
	}

	for _, hook := range webhooks {
		if handlerContextIsDone(ctx) {
			return ctx.Err()
		}
		// PHP 侧用 explode(',', $webhook['rw_events']) 判断是否订阅了该事件。
		if !webhookSubscribes(hook.events, ev.Type) {
			continue
		}
		if _, err := h.db.ExecContext(ctx, `
			INSERT INTO cc_repository_webhook_events (rwe_key, rwe_user, rw_key, rwe_type, rwe_data)
			VALUES (?, ?, ?, ?, ?)`,
			contract.NewKey(), ev.GeneratedBy, hook.key, ev.Type, payload); err != nil {
			return fmt.Errorf("写入 webhook 事件队列失败: %w", err)
		}
	}
	return nil
}

// repositoryWebhook 是一个仓库上的 webhook 订阅。
type repositoryWebhook struct {
	key    string
	events string
}

// repositoryWebhooks 列出仓库上的 webhook 订阅。
//
// 逐字对应 application/models/repository_model.php:2251-2262 的
// getRepositoryWebhooks()：只按 r_key 过滤，**不过滤 rw_active**
// （启用状态是在投递环节 getRepositoryWebhookEvents() 里判断的）。
func (h *webhookHandler) repositoryWebhooks(ctx context.Context, rKey string) ([]repositoryWebhook, error) {
	rows, err := h.db.QueryContext(ctx,
		"SELECT rw_key, rw_events FROM cc_repository_webhooks WHERE r_key = ?", rKey)
	if err != nil {
		return nil, fmt.Errorf("查询仓库 webhook 失败: %w", err)
	}
	defer rows.Close()

	out := []repositoryWebhook{}
	for rows.Next() {
		var hook repositoryWebhook
		if err := rows.Scan(&hook.key, &hook.events); err != nil {
			return nil, fmt.Errorf("扫描仓库 webhook 失败: %w", err)
		}
		out = append(out, hook)
	}
	return out, rows.Err()
}

// webhookSubscribes 判断 webhook 是否订阅了某个事件。
//
// 逐字对应 application/models/repository_model.php:2233-2237：
// $eventTypes = explode(',', $webhook['rw_events']); in_array($eventType, $eventTypes)
func webhookSubscribes(events, evType string) bool {
	for _, item := range strings.Split(events, ",") {
		if item == evType {
			return true
		}
	}
	return false
}
