package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
)

// RepositoryWriteRepo 负责 repository 域的**写**操作。
//
// 与只读的 RepositoryRepo 分开的原因：
//
//  1. 写操作必须跑在事务里（WithTx），而只读仓储没有事务概念；
//  2. 写操作需要"列白名单"来防止拼接风险，与只读仓储的查询风格不同；
//  3. 事件必须与业务写入在**同一个 *sql.Tx** 里入队，因此 WithTx
//     把裸事务一并交给调用方。
//
// PHP 侧的写操作**没有事务保护**（例如 changeOwner 要写三次，
// 中途失败会留下半成品），这里显式补上。
type RepositoryWriteRepo struct {
	db *sql.DB
	tx *sql.Tx
}

// NewRepositoryWriteRepo 创建仓库写仓储。
func NewRepositoryWriteRepo(db *sql.DB) *RepositoryWriteRepo { return &RepositoryWriteRepo{db: db} }

// runner 返回当前生效的语句执行句柄。
func (r *RepositoryWriteRepo) runner() querier {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// RepositoryTx 是**事务内**可用的仓库写操作集合。
//
// 抽成接口有两个目的：
//
//  1. 单元测试可以注入假的实现，覆盖"建仓失败要回滚"这类分支；
//  2. 把"哪些操作必须在事务里"显式化 —— 处理器拿到的永远是事务内的仓储。
type RepositoryTx interface {
	Create(ctx context.Context, repo *domain.Repository) error
	Update(ctx context.Context, rKey string, fields map[string]any) error
	RemoveRepository(ctx context.Context, rKey string) error

	AddMember(ctx context.Context, uKey, rKey string) error
	SetMemberRole(ctx context.Context, uKey, rKey string, role int) error
	RemoveMember(ctx context.Context, uKey, rKey string) error

	CreateProtectedBranchRule(ctx context.Context, rKey, rule string, rolePush, roleMerge int) error
	UpdateProtectedBranchRule(ctx context.Context, pbrKey string, fields map[string]any) error

	CreateWebhook(ctx context.Context, w *domain.RepositoryWebhook) (string, error)
	UpdateWebhook(ctx context.Context, rwKey string, fields map[string]any) (int64, error)
	DeleteWebhook(ctx context.Context, rwKey string) (int64, error)
	DeleteWebhookEventsByRwKey(ctx context.Context, rwKey string) error
	DeleteWebhookLogsByRwKey(ctx context.Context, rwKey string) error

	CreateMergeRequest(ctx context.Context, mr *domain.RepositoryMergeRequest) error
	MergeRequestReview(ctx context.Context, mrKey, uKey string) error
	AssignReviewer(ctx context.Context, mrKey, uKey string) (string, error)
	DeleteReviewer(ctx context.Context, mrKey, uKey string) error
	MergeRequestClose(ctx context.Context, uKey, mrKey, sourceSHA, targetSHA string) error
	MergeRequestMerge(ctx context.Context, uKey, mrKey, sourceSHA, targetSHA string) error
}

// WithTx 在一个事务里执行 fn。
//
// fn 收到的仓储绑定到该事务；raw 是同一个事务对象，调用方用它把领域事件
// 发到事件总线 —— 从而保证"业务提交则事件在、业务回滚则事件消失"。
//
// fn 返回错误即回滚。
func (r *RepositoryWriteRepo) WithTx(ctx context.Context, fn func(tx RepositoryTx, raw *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(&RepositoryWriteRepo{db: r.db, tx: tx}, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// repositoryWritableColumns 是允许被 Update 写入的列白名单。
//
// 刻意**不含** u_job / u_company —— cc_users 表里没有这两列
// （PHP 侧 api/user.php 的白名单里却有，写入必然失败）。
var repositoryWritableColumns = map[string]bool{
	"r_display_name":        true,
	"r_name":                true,
	"r_description":         true,
	"r_avatar":              true,
	"r_default_branch_name": true,
	"u_key":                 true,
	"r_fork_r_key":          true,
	"r_status":              true,
	"r_deleted":             true,
}

// protectedBranchWritableColumns 是保护分支规则表的列白名单。
var protectedBranchWritableColumns = map[string]bool{
	"pbr_rule":              true,
	"pbr_push_min_role_id":  true,
	"pbr_merge_min_role_id": true,
	"pbr_status":            true,
	"pbr_deleted":           true,
}

// webhookWritableColumns 是 webhook 表的列白名单。
var webhookWritableColumns = map[string]bool{
	"rw_url":     true,
	"rw_secret":  true,
	"rw_events":  true,
	"rw_active":  true,
	"rw_updated": true,
}

// ---------------------------------------------------------------------------
// 仓库
// ---------------------------------------------------------------------------

// Create 插入一行仓库记录。
//
// 逐字对应 application/models/repository_model.php:910-938 的 create()：
// 只写 r_key / u_key / g_key / r_display_name / r_name / r_path /
// r_description / r_updated，其余列走数据库默认值。
func (r *RepositoryWriteRepo) Create(ctx context.Context, repo *domain.Repository) error {
	_, err := r.runner().ExecContext(ctx, `
		INSERT INTO cc_repositories
			(r_key, u_key, g_key, r_fork_r_key, r_display_name, r_name, r_path,
			 r_description, r_avatar, r_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		repo.Key, repo.OwnerKey, repo.GroupKey, repo.ForkFromKey,
		repo.DisplayName, repo.Name, repo.Path, repo.Description, repo.Avatar)
	return err
}

// Update 更新仓库的指定列。
//
// 列名必须来自调用方的白名单，这里再校验一次，避免拼接风险。
func (r *RepositoryWriteRepo) Update(ctx context.Context, rKey string, fields map[string]any) error {
	set, args, err := buildUpdateSet(fields, repositoryWritableColumns)
	if err != nil {
		return err
	}
	if set == "" {
		return nil
	}
	args = append(args, rKey)
	_, err = r.runner().ExecContext(ctx, "UPDATE cc_repositories SET "+set+" WHERE r_key = ?", args...)
	return err
}

// RemoveRepository 软删除仓库。
//
// 逐字对应 application/models/repository_model.php:566-574 的 removeRepository()：
// 把 r_status 置为 COMMON_STATUS_DELETE（4）并写入 r_deleted。
//
// 注意：**不删除磁盘上的裸仓库** —— 与 PHP 一致；仓库是软删除，
// 目录仍可能被恢复使用。
func (r *RepositoryWriteRepo) RemoveRepository(ctx context.Context, rKey string) error {
	_, err := r.runner().ExecContext(ctx, `
		UPDATE cc_repositories SET r_status = ?, r_deleted = NOW() WHERE r_key = ?`,
		domain.CommonStatusDelete, rKey)
	return err
}

// Get 读取一个仓库（与只读仓储同语义，但绑定当前事务）。
func (r *RepositoryWriteRepo) Get(ctx context.Context, rKey string) (*domain.Repository, error) {
	row := r.runner().QueryRowContext(ctx,
		"SELECT "+repoCols(repositoryColumnsTpl, "")+" FROM cc_repositories WHERE r_key = ? AND r_status = ?",
		rKey, domain.CommonStatusNormal)
	return scanRepository(row)
}

// RepositoryPath 返回仓库在 git-storage 下的相对路径（r_path）。
//
// 逐字对应 cc_repositories.r_path，形如 "/de7347a84f6236f06b0356ce6668c27d"。
// 绝对路径由 gitsvc.Service.RepoPath() 负责拼接并做路径穿越校验。
func (r *RepositoryWriteRepo) RepositoryPath(ctx context.Context, rKey string) (string, error) {
	var p sql.NullString
	err := r.runner().QueryRowContext(ctx, "SELECT r_path FROM cc_repositories WHERE r_key = ?", rKey).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if !p.Valid {
		return "", ErrNotFound
	}
	return p.String, nil
}

// SearchByName 在指定仓库组内按仓库名查仓库。
//
// 逐字对应 application/models/repository_model.php:759-771 的 searchByName()。
// 查不到时返回 (nil, nil) —— 对应 PHP 侧的 FALSE。
func (r *RepositoryWriteRepo) SearchByName(ctx context.Context, name, gKey string) (*domain.Repository, error) {
	if name == "" || gKey == "" {
		return nil, nil
	}
	row := r.runner().QueryRowContext(ctx,
		"SELECT "+repoCols(repositoryColumnsTpl, "")+
			" FROM cc_repositories WHERE r_name = ? AND g_key = ? AND r_status = ? LIMIT 1",
		name, gKey, domain.CommonStatusNormal)
	repo, err := scanRepository(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// SearchByDisplayName 在指定仓库组内按显示名查仓库。
//
// 逐字对应 application/models/repository_model.php:773-785 的 searchByDisplayName()。
func (r *RepositoryWriteRepo) SearchByDisplayName(ctx context.Context, displayName, gKey string) (*domain.Repository, error) {
	if displayName == "" || gKey == "" {
		return nil, nil
	}
	row := r.runner().QueryRowContext(ctx,
		"SELECT "+repoCols(repositoryColumnsTpl, "")+
			" FROM cc_repositories WHERE r_display_name = ? AND g_key = ? AND r_status = ? LIMIT 1",
		displayName, gKey, domain.CommonStatusNormal)
	repo, err := scanRepository(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// ---------------------------------------------------------------------------
// 成员
// ---------------------------------------------------------------------------

// AddMember 把用户加入仓库。
//
// 逐字对应 application/models/repository_model.php:493-517 的 addMember()：
// 已是成员则把角色重置为 ROLE_GUEST，否则插入新行。
func (r *RepositoryWriteRepo) AddMember(ctx context.Context, uKey, rKey string) error {
	return r.SetMemberRole(ctx, uKey, rKey, int(domain.RoleGuest))
}

// SetMemberRole 设置用户在仓库内的角色。
//
// 逐字对应 application/models/repository_model.php:519-548 的 setMemberRole()：
// 已是成员则更新 rm_role 并恢复 rm_status；否则插入新行。
func (r *RepositoryWriteRepo) SetMemberRole(ctx context.Context, uKey, rKey string, role int) error {
	if uKey == "" || rKey == "" || role == 0 {
		return errors.New("仓库成员参数不完整")
	}
	exists, err := r.repositoryMemberExists(ctx, uKey, rKey)
	if err != nil {
		return err
	}
	if exists {
		_, err = r.runner().ExecContext(ctx, `
			UPDATE cc_repository_members SET rm_role = ?, rm_status = ?
			WHERE u_key = ? AND r_key = ?`,
			role, domain.CommonStatusNormal, uKey, rKey)
		return err
	}
	_, err = r.runner().ExecContext(ctx, `
		INSERT INTO cc_repository_members (rm_key, u_key, r_key, rm_role)
		VALUES (?, ?, ?, ?)`,
		contract.NewKey(), uKey, rKey, role)
	return err
}

// RemoveMember 移除仓库成员（软删除 + 角色降为 ROLE_GUEST）。
//
// 逐字对应 application/models/repository_model.php:550-564 的 removeMember()。
func (r *RepositoryWriteRepo) RemoveMember(ctx context.Context, uKey, rKey string) error {
	if uKey == "" || rKey == "" {
		return errors.New("仓库成员参数不完整")
	}
	_, err := r.runner().ExecContext(ctx, `
		UPDATE cc_repository_members
		SET rm_role = ?, rm_status = ?, rm_deleted = NOW()
		WHERE u_key = ? AND r_key = ?`,
		int(domain.RoleGuest), domain.CommonStatusDelete, uKey, rKey)
	return err
}

// repositoryMemberExists 判断用户是否已有仓库成员行（不看状态）。
func (r *RepositoryWriteRepo) repositoryMemberExists(ctx context.Context, uKey, rKey string) (bool, error) {
	var one int
	err := r.runner().QueryRowContext(ctx,
		"SELECT 1 FROM cc_repository_members WHERE u_key = ? AND r_key = ? LIMIT 1", uKey, rKey).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// 保护分支规则
// ---------------------------------------------------------------------------

// ListProtectedBranchRules 列出仓库的保护分支规则。
//
// 逐字对应 application/models/repository_model.php:832-844 的
// listProtectedBranchRules()：按 pbr_updated 倒序。
func (r *RepositoryWriteRepo) ListProtectedBranchRules(ctx context.Context, rKey string) ([]domain.ProtectedBranchRule, error) {
	if rKey == "" {
		return nil, nil
	}
	rows, err := r.runner().QueryContext(ctx,
		"SELECT pbr_key, r_key, pbr_rule, pbr_push_min_role_id, pbr_merge_min_role_id,"+
			" CAST(pbr_updated AS CHAR), pbr_status"+
			" FROM cc_proteced_branch_rules WHERE r_key = ? AND pbr_status = ?"+
			" ORDER BY pbr_updated DESC", rKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ProtectedBranchRule{}
	for rows.Next() {
		var rule domain.ProtectedBranchRule
		if err := rows.Scan(&rule.Key, &rule.RKey, &rule.Rule, &rule.PushRole, &rule.MergeRole,
			&rule.Updated, &rule.Status); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// GetProtectedBranchRule 按规则串读取一条保护分支规则。
//
// 逐字对应 application/models/repository_model.php:846-860 的
// getProtectedBranchRule()。查不到时返回 (nil, nil)。
func (r *RepositoryWriteRepo) GetProtectedBranchRule(ctx context.Context, rKey, rule string) (*domain.ProtectedBranchRule, error) {
	if rKey == "" || rule == "" {
		return nil, nil
	}
	row := r.runner().QueryRowContext(ctx,
		"SELECT pbr_key, r_key, pbr_rule, pbr_push_min_role_id, pbr_merge_min_role_id,"+
			" CAST(pbr_updated AS CHAR), pbr_status"+
			" FROM cc_proteced_branch_rules"+
			" WHERE r_key = ? AND pbr_status = ? AND pbr_rule = ? LIMIT 1",
		rKey, domain.CommonStatusNormal, rule)
	var out domain.ProtectedBranchRule
	err := row.Scan(&out.Key, &out.RKey, &out.Rule, &out.PushRole, &out.MergeRole, &out.Updated, &out.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateProtectedBranchRule 新建一条保护分支规则。
//
// 逐字对应 application/models/repository_model.php:862-878 的
// createProtectedBranchRule()。
func (r *RepositoryWriteRepo) CreateProtectedBranchRule(ctx context.Context, rKey, rule string, rolePush, roleMerge int) error {
	if rKey == "" || rule == "" || rolePush == 0 || roleMerge == 0 {
		return errors.New("保护分支规则参数不完整")
	}
	_, err := r.runner().ExecContext(ctx, `
		INSERT INTO cc_proteced_branch_rules
			(pbr_key, r_key, pbr_rule, pbr_push_min_role_id, pbr_merge_min_role_id)
		VALUES (?, ?, ?, ?, ?)`,
		contract.NewKey(), rKey, rule, rolePush, roleMerge)
	return err
}

// UpdateProtectedBranchRule 更新一条保护分支规则。
//
// 逐字对应 application/models/repository_model.php:880-889 的
// updateProtectedBranchRule()。
func (r *RepositoryWriteRepo) UpdateProtectedBranchRule(ctx context.Context, pbrKey string, fields map[string]any) error {
	set, args, err := buildUpdateSet(fields, protectedBranchWritableColumns)
	if err != nil {
		return err
	}
	if set == "" {
		return nil
	}
	args = append(args, pbrKey)
	_, err = r.runner().ExecContext(ctx,
		"UPDATE cc_proteced_branch_rules SET "+set+" WHERE pbr_key = ?", args...)
	return err
}

// ---------------------------------------------------------------------------
// Webhook
// ---------------------------------------------------------------------------

// GetWebhook 按主键读取一条 webhook。
//
// 逐字对应 application/models/repository_model.php:2144-2149 的 getWebhook()。
// 注意它**没有 JOIN cc_users**，因此返回的 UserName 为 nil。
func (r *RepositoryWriteRepo) GetWebhook(ctx context.Context, rwKey string) (*domain.RepositoryWebhook, *string, error) {
	if rwKey == "" {
		return nil, nil, nil
	}
	row := r.runner().QueryRowContext(ctx,
		"SELECT rw_key, r_key, u_key, rw_url, rw_secret, rw_events, rw_active,"+
			" CAST(rw_updated AS CHAR) FROM cc_repository_webhooks WHERE rw_key = ? LIMIT 1", rwKey)
	w, err := scanWebhook(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return w, nil, nil
}

// ListWebhooks 列出仓库的全部 webhook（附带创建者名称）。
//
// 逐字对应 application/models/repository_model.php:2151-2158 的 getWebhooks()：
// LEFT JOIN cc_users 取 u_name。
func (r *RepositoryWriteRepo) ListWebhooks(ctx context.Context, rKey string) ([]domain.RepositoryWebhook, map[string]*string, error) {
	if rKey == "" {
		return nil, nil, nil
	}
	rows, err := r.runner().QueryContext(ctx,
		"SELECT rw.rw_key, rw.r_key, rw.u_key, rw.rw_url, rw.rw_secret, rw.rw_events, rw.rw_active,"+
			" CAST(rw.rw_updated AS CHAR), u.u_name"+
			" FROM cc_repository_webhooks AS rw"+
			" LEFT JOIN cc_users AS u ON rw.u_key = u.u_key"+
			" WHERE rw.r_key = ?", rKey)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := []domain.RepositoryWebhook{}
	users := map[string]*string{}
	for rows.Next() {
		var w domain.RepositoryWebhook
		var name sql.NullString
		if err := rows.Scan(&w.Key, &w.RKey, &w.UKey, &w.URL, &w.Secret, &w.Events, &w.Active,
			&w.Updated, &name); err != nil {
			return nil, nil, err
		}
		if name.Valid {
			v := name.String
			users[w.Key] = &v
		}
		out = append(out, w)
	}
	return out, users, rows.Err()
}

// CreateWebhook 新建一条 webhook，返回新建的主键。
//
// 逐字对应 application/models/repository_model.php:2160-2171 的 createWebhook()。
func (r *RepositoryWriteRepo) CreateWebhook(ctx context.Context, w *domain.RepositoryWebhook) (string, error) {
	if w.UKey == "" || w.RKey == "" || w.URL == "" || w.Events == "" {
		return "", errors.New("webhook 参数不完整")
	}
	key := contract.NewKey()
	_, err := r.runner().ExecContext(ctx, `
		INSERT INTO cc_repository_webhooks
			(rw_key, r_key, u_key, rw_url, rw_secret, rw_events, rw_active, rw_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW())`,
		key, w.RKey, w.UKey, w.URL, w.Secret, w.Events, w.Active)
	if err != nil {
		return "", err
	}
	return key, nil
}

// UpdateWebhook 更新一条 webhook，返回受影响的行数。
//
// 逐字对应 application/models/repository_model.php:2173-2185 的 updateWebhook()。
func (r *RepositoryWriteRepo) UpdateWebhook(ctx context.Context, rwKey string, fields map[string]any) (int64, error) {
	set, args, err := buildUpdateSet(fields, webhookWritableColumns)
	if err != nil {
		return 0, err
	}
	if set == "" {
		return 0, nil
	}
	args = append(args, rwKey)
	res, err := r.runner().ExecContext(ctx,
		"UPDATE cc_repository_webhooks SET "+set+" WHERE rw_key = ?", args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteWebhook 删除一条 webhook，返回受影响的行数。
//
// 逐字对应 application/models/repository_model.php:2187-2197 的 deleteWebhook()。
func (r *RepositoryWriteRepo) DeleteWebhook(ctx context.Context, rwKey string) (int64, error) {
	res, err := r.runner().ExecContext(ctx,
		"DELETE FROM cc_repository_webhooks WHERE rw_key = ?", rwKey)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteWebhookEventsByRwKey 删除某条 webhook 的全部待投递事件。
//
// 逐字对应 application/models/repository_model.php:2199-2208。
func (r *RepositoryWriteRepo) DeleteWebhookEventsByRwKey(ctx context.Context, rwKey string) error {
	_, err := r.runner().ExecContext(ctx,
		"DELETE FROM cc_repository_webhook_events WHERE rw_key = ?", rwKey)
	return err
}

// DeleteWebhookLogsByRwKey 删除某条 webhook 的全部投递日志。
//
// 逐字对应 application/models/repository_model.php:2210-2219。
func (r *RepositoryWriteRepo) DeleteWebhookLogsByRwKey(ctx context.Context, rwKey string) error {
	_, err := r.runner().ExecContext(ctx,
		"DELETE FROM cc_repository_webhook_logs WHERE rw_key = ?", rwKey)
	return err
}

// ListWebhookLogs 列出某条 webhook 的投递日志（按创建时间倒序）。
//
// 逐字对应 application/models/repository_model.php:2337-2350 的
// getRepositoryWebhookLogs()。
func (r *RepositoryWriteRepo) ListWebhookLogs(ctx context.Context, rwKey string) ([]domain.RepositoryWebhookLog, error) {
	if rwKey == "" {
		return nil, nil
	}
	rows, err := r.runner().QueryContext(ctx,
		"SELECT rwl_id, rw_key, rwl_request, rwl_response, rwl_start, rwl_end, rwl_status,"+
			" CAST(rwl_created AS CHAR) FROM cc_repository_webhook_logs"+
			" WHERE rw_key = ? ORDER BY rwl_created DESC", rwKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.RepositoryWebhookLog{}
	for rows.Next() {
		var l domain.RepositoryWebhookLog
		if err := rows.Scan(&l.ID, &l.Webhook, &l.Request, &l.Response, &l.Start, &l.End,
			&l.Status, &l.Created); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetWebhookLogData 按主键读取一条投递日志的请求/响应内容。
//
// 逐字对应 application/models/repository_model.php:2306-2317 的
// getRepositoryWebhookLogData()。查不到时返回 (nil, nil)。
func (r *RepositoryWriteRepo) GetWebhookLogData(ctx context.Context, id string) (*domain.RepositoryWebhookLog, error) {
	if id == "" {
		return nil, nil
	}
	row := r.runner().QueryRowContext(ctx,
		"SELECT rwl_id, rw_key, rwl_request, rwl_response, rwl_start, rwl_end, rwl_status,"+
			" CAST(rwl_created AS CHAR) FROM cc_repository_webhook_logs WHERE rwl_id = ? LIMIT 1", id)
	var l domain.RepositoryWebhookLog
	err := row.Scan(&l.ID, &l.Webhook, &l.Request, &l.Response, &l.Start, &l.End, &l.Status, &l.Created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// ---------------------------------------------------------------------------
// 合并请求
// ---------------------------------------------------------------------------

// GetMergeRequestByKey 按主键读取一个合并请求。
//
// 逐字对应 application/models/repository_model.php:1180-1193 的
// getMergeRequestByKey()：只取 mr_status = COMMON_STATUS_NORMAL 的行。
func (r *RepositoryWriteRepo) GetMergeRequestByKey(ctx context.Context, mrKey string) (*domain.RepositoryMergeRequest, error) {
	if mrKey == "" {
		return nil, nil
	}
	row := r.runner().QueryRowContext(ctx,
		"SELECT "+repoCols(mergeRequestColumnsTpl, "")+
			" FROM cc_merge_requests WHERE mr_key = ? AND mr_status = ? LIMIT 1",
		mrKey, domain.CommonStatusNormal)
	mr, err := scanMergeRequest(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mr, nil
}

// MergeRequestExists 判断同一对分支之间是否已有**待合并**的合并请求。
//
// 逐字对应 application/models/repository_model.php:1195-1211 的
// isMergeRequestExist()。
func (r *RepositoryWriteRepo) MergeRequestExists(ctx context.Context, sourceRKey, sourceBranch, targetRKey, targetBranch string) (bool, error) {
	if sourceRKey == "" || sourceBranch == "" || targetRKey == "" || targetBranch == "" {
		// PHP 侧参数不全时直接返回 TRUE（视为已存在 → 0x0406）。
		return true, nil
	}
	var one int
	err := r.runner().QueryRowContext(ctx,
		"SELECT 1 FROM cc_merge_requests"+
			" WHERE mr_source_r_key = ? AND mr_source_branch = ?"+
			" AND mr_target_r_key = ? AND mr_target_branch = ?"+
			" AND mr_proceed_status = ? AND mr_status = ? LIMIT 1",
		sourceRKey, sourceBranch, targetRKey, targetBranch,
		domain.MergeRequestStatusOpen, domain.CommonStatusNormal).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// NextMergeRequestID 计算目标仓库的下一个合并请求编号。
//
// 逐字对应 application/models/repository_model.php:254-257：
// 取该目标仓库下最大的 mr_id 再加一；一个都没有时从 1 开始。
func (r *RepositoryWriteRepo) NextMergeRequestID(ctx context.Context, targetRKey string) (int, error) {
	var last sql.NullInt64
	err := r.runner().QueryRowContext(ctx,
		"SELECT mr_id FROM cc_merge_requests WHERE mr_target_r_key = ? ORDER BY mr_id DESC LIMIT 1",
		targetRKey).Scan(&last)
	if errors.Is(err, sql.ErrNoRows) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	if !last.Valid {
		return 1, nil
	}
	return int(last.Int64) + 1, nil
}

// CreateMergeRequest 新建一个合并请求。
//
// 逐字对应 application/models/repository_model.php:244-281 的
// createMergeRequest()。
func (r *RepositoryWriteRepo) CreateMergeRequest(ctx context.Context, mr *domain.RepositoryMergeRequest) error {
	_, err := r.runner().ExecContext(ctx, `
		INSERT INTO cc_merge_requests
			(mr_key, u_key, mr_source_r_key, mr_source_branch, mr_target_r_key, mr_target_branch,
			 mr_start_hash, mr_title, mr_description, mr_id, mr_proceed_status,
			 mr_created, mr_updated, mr_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), ?)`,
		mr.Key, mr.UKey, mr.SourceRKey, mr.SourceBranch, mr.TargetRKey, mr.TargetBranch,
		mr.StartHash, mr.Title, mr.Description, mr.ID, mr.ProceedStatus, domain.CommonStatusNormal)
	return err
}

// MergeRequestReview 把当前用户标记为"已评审"。
//
// 逐字对应 application/models/repository_model.php:283-295 的
// mergeRequestReview()：把 mrr_is_review 置为 GLOBAL_TRUE（1）。
func (r *RepositoryWriteRepo) MergeRequestReview(ctx context.Context, mrKey, uKey string) error {
	if mrKey == "" || uKey == "" {
		return errors.New("合并请求评审参数不完整")
	}
	_, err := r.runner().ExecContext(ctx, `
		UPDATE cc_merge_request_reviewers SET mrr_is_review = ?
		WHERE mr_key = ? AND u_key = ? AND mrr_status = ?`,
		domain.GlobalTrue, mrKey, uKey, domain.CommonStatusNormal)
	return err
}

// AssignReviewer 给合并请求指派评审人，返回评审记录主键。
//
// 逐字对应 application/models/repository_model.php:297-325 的 assignReviewer()：
// 若已有一条"未评审"的记录，先删掉再插入新的；已评审过的则返回 FALSE。
func (r *RepositoryWriteRepo) AssignReviewer(ctx context.Context, mrKey, uKey string) (string, error) {
	if mrKey == "" || uKey == "" {
		return "", errors.New("合并请求评审人参数不完整")
	}
	var existingKey string
	var isReview int
	err := r.runner().QueryRowContext(ctx,
		"SELECT mrr_key, mrr_is_review FROM cc_merge_request_reviewers"+
			" WHERE mr_key = ? AND mrr_status = ? LIMIT 1",
		mrKey, domain.CommonStatusNormal).Scan(&existingKey, &isReview)
	switch {
	case err == nil:
		if isReview == domain.GlobalTrue {
			// 已评审过 → PHP 侧返回 FALSE（0x0405）。
			return "", nil
		}
		if _, derr := r.runner().ExecContext(ctx,
			"DELETE FROM cc_merge_request_reviewers WHERE mr_key = ? AND u_key = ?",
			mrKey, uKey); derr != nil {
			return "", derr
		}
	case errors.Is(err, sql.ErrNoRows):
		// 没有记录，继续插入。
	default:
		return "", err
	}

	key := contract.NewKey()
	if _, err := r.runner().ExecContext(ctx, `
		INSERT INTO cc_merge_request_reviewers (mrr_key, mr_key, u_key, mrr_status)
		VALUES (?, ?, ?, ?)`,
		key, mrKey, uKey, domain.CommonStatusNormal); err != nil {
		return "", err
	}
	return key, nil
}

// DeleteReviewer 删除合并请求的评审人。
//
// 逐字对应 application/models/repository_model.php:327-349 的 deleteReviewer()：
// 若该评审人**已评审**过则拒绝删除（返回 FALSE）。
func (r *RepositoryWriteRepo) DeleteReviewer(ctx context.Context, mrKey, uKey string) error {
	if mrKey == "" || uKey == "" {
		return errors.New("合并请求评审人参数不完整")
	}
	var isReview int
	err := r.runner().QueryRowContext(ctx,
		"SELECT mrr_is_review FROM cc_merge_request_reviewers"+
			" WHERE mr_key = ? AND u_key = ? AND mrr_status = ? LIMIT 1",
		mrKey, uKey, domain.CommonStatusNormal).Scan(&isReview)
	if errors.Is(err, sql.ErrNoRows) {
		// 没有记录 → PHP 侧的 delete 影响 0 行仍返回 TRUE。
		return nil
	}
	if err != nil {
		return err
	}
	if isReview == domain.GlobalTrue {
		return errors.New("该评审人已评审，不能移除")
	}
	_, err = r.runner().ExecContext(ctx,
		"DELETE FROM cc_merge_request_reviewers WHERE mr_key = ? AND u_key = ?", mrKey, uKey)
	return err
}

// ReviewPending 判断是否还有**未评审**的评审人。
//
// 逐字对应 application/models/repository_model.php:366-380 的 getReviewStatus()：
// 存在 mrr_is_review = GLOBAL_FALSE（2）的评审人时返回 FALSE（不可合并）。
func (r *RepositoryWriteRepo) ReviewPending(ctx context.Context, mrKey string) (bool, error) {
	if mrKey == "" {
		return false, errors.New("合并请求主键不能为空")
	}
	var one int
	err := r.runner().QueryRowContext(ctx,
		"SELECT 1 FROM cc_merge_request_reviewers"+
			" WHERE mr_key = ? AND mrr_status = ? AND mrr_is_review = ? LIMIT 1",
		mrKey, domain.CommonStatusNormal, domain.GlobalFalse).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// MergeRequestClose 关闭一个合并请求。
//
// 逐字对应 application/models/repository_model.php:1327-1354 的 mergeRequestClose()。
func (r *RepositoryWriteRepo) MergeRequestClose(ctx context.Context, uKey, mrKey, sourceSHA, targetSHA string) error {
	_, err := r.runner().ExecContext(ctx, `
		UPDATE cc_merge_requests
		SET mr_merge_u_key = ?, mr_source_merged_hash = ?, mr_target_merged_hash = ?,
			mr_close_time = NOW(), mr_proceed_status = ?
		WHERE mr_key = ?`,
		uKey, sourceSHA, targetSHA, domain.MergeRequestStatusClosed, mrKey)
	return err
}

// MergeRequestMerge 把一个合并请求标记为已合并。
//
// 逐字对应 application/models/repository_model.php:382-397 的 mergeRequestMerge()。
func (r *RepositoryWriteRepo) MergeRequestMerge(ctx context.Context, uKey, mrKey, sourceSHA, targetSHA string) error {
	_, err := r.runner().ExecContext(ctx, `
		UPDATE cc_merge_requests
		SET mr_source_merged_hash = ?, mr_target_merged_hash = ?, mr_proceed_status = ?,
			mr_merge_u_key = ?, mr_merge_time = NOW()
		WHERE mr_key = ?`,
		sourceSHA, targetSHA, domain.MergeRequestStatusMerged, uKey, mrKey)
	return err
}

// ---------------------------------------------------------------------------
// 内部工具
// ---------------------------------------------------------------------------

// GroupsByKeys 批量读取仓库组。
//
// 与只读仓储的同名方法同语义（application/models/repository_model.php:24-49
// 的 normalize() 需要组信息），这里绑定当前事务，供归一化使用。
func (r *RepositoryWriteRepo) GroupsByKeys(ctx context.Context, gKeys []string) (map[string]*domain.RepositoryGroup, error) {
	out := map[string]*domain.RepositoryGroup{}
	if len(gKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(gKeys)
	rows, err := r.runner().QueryContext(ctx,
		"SELECT "+repoCols(repositoryGroupColumnsTpl, "")+" FROM cc_groups WHERE g_key IN ("+
			strings.Join(placeholders, ", ")+")", args...)
	if err != nil {
		return nil, err
	}
	list, err := scanRepositoryGroups(rows)
	if err != nil {
		return nil, err
	}
	for i := range list {
		g := list[i]
		out[g.Key] = &g
	}
	return out, nil
}

// ForkCounts 批量统计每个仓库的 fork 数。
//
// 逐字对应 application/models/repository_model.php:576-584 的 countForks()。
func (r *RepositoryWriteRepo) ForkCounts(ctx context.Context, rKeys []string) (map[string]int, error) {
	out := map[string]int{}
	if len(rKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(rKeys)
	args = append(args, domain.CommonStatusNormal)
	rows, err := r.runner().QueryContext(ctx,
		"SELECT r_fork_r_key, COUNT(*) FROM cc_repositories WHERE r_fork_r_key IN ("+
			strings.Join(placeholders, ", ")+") AND r_status = ? GROUP BY r_fork_r_key", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			return nil, err
		}
		out[key] = n
	}
	return out, rows.Err()
}

// MergeRequestCounts 批量统计每个仓库的合并请求数。
//
// 逐字对应 application/models/repository_model.php:586-617 的 countMergeRequests()。
func (r *RepositoryWriteRepo) MergeRequestCounts(ctx context.Context, rKeys []string) (map[string]domain.RepositoryMergeRequestCount, error) {
	out := map[string]domain.RepositoryMergeRequestCount{}
	if len(rKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(rKeys)
	args = append(args, domain.CommonStatusNormal)
	rows, err := r.runner().QueryContext(ctx,
		"SELECT mr_target_r_key, mr_proceed_status, COUNT(mr_key) FROM cc_merge_requests"+
			" WHERE mr_target_r_key IN ("+strings.Join(placeholders, ", ")+") AND mr_status = ?"+
			" GROUP BY mr_target_r_key, mr_proceed_status", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var status, n int
		if err := rows.Scan(&key, &status, &n); err != nil {
			return nil, err
		}
		c := out[key]
		switch status {
		case domain.MergeRequestStatusOpen:
			c.Open = n
		case domain.MergeRequestStatusMerged:
			c.Merged = n
		case domain.MergeRequestStatusClosed:
			c.Closed = n
		}
		out[key] = c
	}
	for key, c := range out {
		c.All = c.Open + c.Merged + c.Closed
		out[key] = c
	}
	return out, rows.Err()
}

// buildUpdateSet 由字段映射构造 UPDATE 的 SET 子句与参数列表。
//
// 列名必须来自白名单；字段按字典序排列，保证同一组字段总是生成同一条 SQL。
func buildUpdateSet(fields map[string]any, allowed map[string]bool) (string, []any, error) {
	if len(fields) == 0 {
		return "", nil, nil
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		if !allowed[name] {
			return "", nil, errors.New("不允许更新列 " + name)
		}
		names = append(names, name)
	}
	sortStrings(names)

	set := ""
	args := make([]any, 0, len(names))
	for i, name := range names {
		if i > 0 {
			set += ", "
		}
		set += name + " = ?"
		args = append(args, fields[name])
	}
	return set, args, nil
}

// sortStrings 对字符串切片做字典序排序（避免引入 sort 泛型技巧）。
func sortStrings(list []string) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j] < list[j-1]; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

// scanWebhook 扫描一行 webhook。
func scanWebhook(row *sql.Row) (*domain.RepositoryWebhook, error) {
	var w domain.RepositoryWebhook
	err := row.Scan(&w.Key, &w.RKey, &w.UKey, &w.URL, &w.Secret, &w.Events, &w.Active, &w.Updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}
