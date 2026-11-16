package handlers

import (
	"context"
	"database/sql"
	"errors"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// 合并请求评论。
//
// 读侧（mergeRequestComments）在 RepositoryHandler，
// 写侧（addMergeRequestComment）在 RepositoryWriteHandler，
// 与仓库域的既有分工一致，最终都注册进 "repository" 控制器。
//
// 权限约定：**能读到该 MR 即可评论** —— 复用 mergeRequestDetail 的
// 守卫语义（UAC_MR_READ），评论是讨论行为，不需要推送权限。

// WithRepositoryWriteComments 给写侧注入评论仓储与事件总线用的依赖。
func WithRepositoryWriteComments(comments *repo.MRCommentRepo) RepositoryWriteOption {
	return func(h *RepositoryWriteHandler) { h.Comments = comments }
}

// ---------------------------------------------------------------------------
// 读侧：mergeRequestComments
// ---------------------------------------------------------------------------

// registerMergeRequestCommentRead 在只读侧注册 mergeRequestComments。
//
// Comments 未注入时**不注册**（请求落 0x0105），与 registerTokenActions 一致。
func (h *RepositoryHandler) registerMergeRequestCommentRead(actions map[string]map[string]httpapi.ActionHandler) {
	if h.Comments == nil {
		return
	}
	actions["mergeRequestComments"] = map[string]httpapi.ActionHandler{"GET": h.mergeRequestComments}
}

// mrCommentView 是 mergeRequestComments 的元素。
//
// 作者字段同时给 avatar（原始头像 key，与 NormalizedUser.Icon 同源）与
// icon（别名）：前端两处历史代码分别用了两个名字，新端点一次给全，
// 免得对接时再猜。
type mrCommentView struct {
	ID        int64          `json:"id"`
	Body      string         `json:"body"`
	CreatedAt int64          `json:"createdAt"`
	Author    mrCommentOwner `json:"author"`
}

// mrCommentOwner 是评论作者的最小视图。
type mrCommentOwner struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Avatar *string `json:"avatar"`
	Icon   *string `json:"icon"`
}

// mergeRequestComments 实现 GET /api/repository/mergeRequestComments。
//
// 入参（query）：{mergeRequestId}（即 MR 主键 mr_key）。
// 输出：评论列表，按 c_id asc（时间正序，评论流从旧到新读）。
func (h *RepositoryHandler) mergeRequestComments(ctx context.Context, req *httpapi.Request) (any, error) {
	if h.Comments == nil {
		return nil, contract.NewError(contract.CodeEndpointNotFound)
	}
	uKey := req.UserKey()
	mrKey := trimSlashes(req.Parsed.QueryString("mergeRequestId"))
	if uKey == "" || mrKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	// MR 主键不带仓库前缀，守卫按 MR 的**目标仓库**判定 ——
	// 与 mergeRequestVersionList 的既有做法一致。
	mr, err := h.Repos.MergeRequestByKey(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	if err := h.Guard.RequireRepository(ctx, mr.TargetRKey, uKey, domain.UACMRRead); err != nil {
		return nil, err
	}

	list, err := h.Comments.List(ctx, mrKey)
	if err != nil {
		return nil, err
	}

	// 批量取作者：一条 IN 查询，评论再多也只有一条。
	uKeys := make([]string, 0, len(list))
	for i := range list {
		uKeys = append(uKeys, list[i].UKey)
	}
	users, err := h.Repos.UsersByKeys(ctx, uKeys)
	if err != nil {
		return nil, err
	}

	out := make([]mrCommentView, 0, len(list))
	for i := range list {
		c := &list[i]
		view := mrCommentView{
			ID:        c.ID,
			Body:      c.Body,
			CreatedAt: phpTimeOf(c.CreatedAt),
			Author:    mrCommentOwner{ID: c.UKey},
		}
		if u, ok := users[c.UKey]; ok && u != nil {
			view.Author.Name = u.Name
			view.Author.Avatar = u.Avatar
			view.Author.Icon = u.Avatar
		}
		out = append(out, view)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 写侧：addMergeRequestComment
// ---------------------------------------------------------------------------

// registerMergeRequestCommentWrite 在写侧注册 addMergeRequestComment。
//
// Comments 未注入时**不注册**（请求落 0x0105），与 registerTokenActions 一致。
func (h *RepositoryWriteHandler) registerMergeRequestCommentWrite(actions map[string]map[string]httpapi.ActionHandler) {
	if h.Comments == nil {
		return
	}
	actions["addMergeRequestComment"] = map[string]httpapi.ActionHandler{"POST": h.addMergeRequestComment}
}

// addMergeRequestComment 实现 POST /api/repository/addMergeRequestComment。
//
// 入参：{mergeRequestId, body(1..10000 字)}。
// 成功后向「MR 创建者 + 全体评审人 − 自己」投递站内通知
// （评论写入与评论事件同一事务入队，worker 侧异步落库）。
func (h *RepositoryWriteHandler) addMergeRequestComment(ctx context.Context, req *httpapi.Request) (any, error) {
	if h.Comments == nil {
		return nil, contract.NewError(contract.CodeEndpointNotFound)
	}
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	mrKey := domain.PHPTrim(bodyOrEmpty(p, "mergeRequestId"))
	body := bodyOrEmpty(p, "body")
	if !domain.PHPTruthy(mrKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !repo.MRCommentBodyAllowed(body) {
		// 空正文与超长正文都算 0x0202：与"评论必须是 1..10000 字"的
		// 参数约束一一对应，比 0x0201（缺参）更精确。
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}

	mr, err := h.Repos.GetMergeRequestByKey(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	if mr == nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	// 能读到该 MR 才能评论（复用 mergeRequestDetail 的守卫语义）。
	if err := h.Guard.RequireRepository(ctx, mr.TargetRKey, uKey, domain.UACMRRead); err != nil {
		return nil, err
	}

	// 性能整改 C5 的同款原则：事务外先取 groupKey，避免事务内另开池连接。
	targetGKey := h.groupKeyOf(ctx, mr.TargetRKey)

	var commentID int64
	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		id, err := tx.CreateMRComment(ctx, mrKey, uKey, body)
		if err != nil {
			return err
		}
		commentID = id
		// 评论事件与评论写入同一事务入队：评论落库则通知必发，
		// 评论回滚则事件消失（与 MR 创建/关闭等事件的既有约定一致）。
		return h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_COMMENT,
			domain.EventDataDescMERGE_REQUEST_COMMENT_D, map[string]any{
				"gKey":  targetGKey,
				"rKey":  mr.TargetRKey,
				"id":    mr.ID,
				"mrKey": mrKey,
				"body":  body,
			}, uKey)
	}); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeMissingParameter)
		}
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return map[string]any{"id": commentID}, nil
}
