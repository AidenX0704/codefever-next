package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"codefever-next/backend/internal/domain"
)

// MRCommentRepo 负责合并请求评论（cc_merge_request_comments，
// 014_mr_comments.sql）。
//
// mr_id 存的是 cc_merge_requests 的主键 mr_key —— 端点参数
// mergeRequestId 与既有 MR 写端点（mergeRequestClose 等）同源。
type MRCommentRepo struct {
	db *sql.DB
}

// NewMRCommentRepo 创建合并请求评论仓储。
func NewMRCommentRepo(db *sql.DB) *MRCommentRepo { return &MRCommentRepo{db: db} }

// MRComment 是一条合并请求评论。
type MRComment struct {
	ID        int64
	MRKey     string
	UKey      string
	Body      string
	CreatedAt string
}

// mrCommentMaxLength 是评论正文的长度上限（字符数，不是字节数）。
//
// text 列本身能存 64KB，但评论是给人读的，10000 字已经远超一次
// code review 意见的合理长度；限制在应用层做，避免超长正文拖垮
// 列表渲染与通知内容。
const mrCommentMaxLength = 10000

// MRCommentBodyAllowed 判断评论正文长度是否合法（1..10000 字）。
func MRCommentBodyAllowed(body string) bool {
	n := len([]rune(body))
	return n >= 1 && n <= mrCommentMaxLength
}

// List 按 MR 主键列出评论（按 c_id asc，即时间正序）。
func (r *MRCommentRepo) List(ctx context.Context, mrKey string) ([]MRComment, error) {
	if mrKey == "" {
		return []MRComment{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT c_id, mr_id, u_key, c_body, CAST(c_created_at AS CHAR)
		FROM cc_merge_request_comments
		WHERE mr_id = ?
		ORDER BY c_id ASC`, mrKey)
	if err != nil {
		return nil, fmt.Errorf("查询合并请求评论失败: %w", err)
	}
	defer rows.Close()

	out := []MRComment{}
	for rows.Next() {
		var c MRComment
		if err := rows.Scan(&c.ID, &c.MRKey, &c.UKey, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MRParticipants 是一个 MR 的"创建者 + 全体评审人"。
type MRParticipants struct {
	// Creator 是 MR 创建者的 u_key（可能为空：MR 行已被删）。
	Creator string
	// Reviewers 是全体评审人的 u_key（含未评审的）。
	Reviewers []string
}

// Participants 查询一个 MR 的创建者与评审人。
//
// 通知的受众就是这两类人（去掉评论者自己，由 events 侧的
// dedupeNotifyUsers 统一处理）。评审人只取 mrr_status = 1 的有效行，
// 与 ReviewPending 的口径一致。
func (r *MRCommentRepo) Participants(ctx context.Context, mrKey string) (MRParticipants, error) {
	out := MRParticipants{Reviewers: []string{}}
	if mrKey == "" {
		return out, nil
	}
	if err := r.db.QueryRowContext(ctx,
		"SELECT u_key FROM cc_merge_requests WHERE mr_key = ?", mrKey).Scan(&out.Creator); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, ErrNotFound
		}
		return out, fmt.Errorf("查询合并请求创建者失败: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT u_key FROM cc_merge_request_reviewers
		WHERE mr_key = ? AND mrr_status = ?`,
		mrKey, domain.CommonStatusNormal)
	if err != nil {
		return out, fmt.Errorf("查询合并请求评审人失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var uKey string
		if err := rows.Scan(&uKey); err != nil {
			return out, err
		}
		out.Reviewers = append(out.Reviewers, uKey)
	}
	return out, rows.Err()
}
