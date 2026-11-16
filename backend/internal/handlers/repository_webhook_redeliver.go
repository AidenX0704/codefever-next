package handlers

import (
	"context"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
)

// Webhook 投递日志重发。
//
// 背景：webhook 投递有 5 次上限与阶梯退避，进入死信的事件不会再投。
// 运维/开发者需要一条"按投递日志手动重发"的通道 —— PHP 侧没有这个能力，
// 本端点把日志里存的请求载荷还原成事件重新入队，由常驻投递器按
// 既有流程发出（不绕过队列直接 POST）。
//
// 权限：与 getRepositoryWebhookLogs 相同 —— 能读到该 webhook 所属
// 仓库即可重发（UAC_REPO_READ）。

// WebhookRequeuer 是"按投递日志重新入队"的最小抽象。
//
// *webhook.MySQLStore 天然满足该接口；抽象出来是为了 handler 单测
// 不必构造真实的数据库队列。
type WebhookRequeuer interface {
	RequeueFromLog(ctx context.Context, webhookKey, requestJSON, fallbackUser string) (string, error)
}

// WithRepositoryWriteRequeuer 注入 webhook 重投依赖。
func WithRepositoryWriteRequeuer(r WebhookRequeuer) RepositoryWriteOption {
	return func(h *RepositoryWriteHandler) { h.Requeuer = r }
}

// registerWebhookRedeliverAction 把 redeliverWebhookLog 挂到 "repository" 控制器。
//
// Requeuer 未注入时**不注册**（请求落 0x0105），与 registerTokenActions 一致。
func (h *RepositoryWriteHandler) registerWebhookRedeliverAction(actions map[string]map[string]httpapi.ActionHandler) {
	if h.Requeuer == nil {
		return
	}
	actions["redeliverWebhookLog"] = map[string]httpapi.ActionHandler{"POST": h.redeliverWebhookLog}
}

// redeliverWebhookLog 实现 POST /api/repository/redeliverWebhookLog。
//
// 入参：{logId}（投递日志主键，即响应头 X-CodeFever-Id）。
// 输出：{eventKey}（新入队事件的主键，可用于在队列中追踪）。
func (h *RepositoryWriteHandler) redeliverWebhookLog(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	logID, _ := phpBodyArg(p, "logId")
	if !domain.PHPTruthy(logID) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	log, err := h.Repos.GetWebhookLogData(ctx, logID)
	if err != nil {
		return nil, err
	}
	// 日志不存在，或它对应的 webhook 已被删除 → 0x0405。
	// 与 deleteToken 的语义一致：不区分"没找到"与"操作失败"。
	if log == nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	webhook, _, err := h.Repos.GetWebhook(ctx, log.Webhook)
	if err != nil {
		return nil, err
	}
	if webhook == nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	if err := h.Guard.RequireRepository(ctx, webhook.RKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	// 重新入队：rwe_user 缺失时回退到当前操作者（webhook.MySQLStore.RequeueFromLog）。
	eventKey, err := h.Requeuer.RequeueFromLog(ctx, log.Webhook, log.Request, uKey)
	if err != nil {
		// 日志损坏（请求体解析不出来）属于目标数据问题，按参数错误呈现；
		// 队列写不进去才是 0x0405。
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return map[string]any{"eventKey": eventKey}, nil
}
