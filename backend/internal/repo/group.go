package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
)

// commonStatusDelete 对应 PHP 常量 COMMON_STATUS_DELETE。
const commonStatusDelete = 4

// GroupRepo 负责 cc_groups / cc_group_members / cc_activities /
// cc_merge_requests / cc_merge_request_reviewers 的读写。
//
// 全部使用显式 SQL 与绑定参数，查询形状逐字对齐
// application/models/group_model.php 与 application/models/repository_model.php。
type GroupRepo struct {
	db *sql.DB
	// tx 非空时，所有语句都在该事务里执行。
	tx *sql.Tx
}

// NewGroupRepo 创建仓库组仓储。
func NewGroupRepo(db *sql.DB) *GroupRepo { return &GroupRepo{db: db} }

// querier 抽象 *sql.DB 与 *sql.Tx 的公共执行能力。
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// runner 返回当前生效的语句执行句柄。
func (r *GroupRepo) runner() querier {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// WithTx 在一个事务里执行 fn。
//
// fn 收到的 GroupRepo 绑定到该事务；fn 返回错误即回滚。
//
// PHP 侧的写操作没有事务保护（多步写入可能只落一半，例如 changeOwner
// 要改三次），这里显式补上。
func (r *GroupRepo) WithTx(ctx context.Context, fn func(tx *GroupRepo) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(&GroupRepo{db: r.db, tx: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// WithTxBoth 在事务中执行 fn，同时交出**事务绑定的仓储**与**原生事务句柄**。
//
// 用途：领域事件必须与业务写入**同事务**入队（cc_events），
// 而 WithTx 只交出仓储包装、拿不到 *sql.Tx。
//
// 用法：
//
//	err := repo.WithTxBoth(ctx, func(groups *GroupRepo, raw *sql.Tx) error {
//	    // groups 上的写与 raw 上的事件入队处于同一事务
//	    return nil
//	})
func (r *GroupRepo) WithTxBoth(ctx context.Context, fn func(groups *GroupRepo, raw *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(&GroupRepo{db: r.db, tx: tx}, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// groupColumns 是 cc_groups 的实际列集合。
const groupColumns = "g_key, u_key, g_type, g_display_name, g_name, g_description, " +
	"g_avatar, g_created, g_updated, g_status"

// groupWritableColumns 是允许被 Update 写入的列白名单。
var groupWritableColumns = map[string]bool{
	"g_display_name": true,
	"g_name":         true,
	"g_description":  true,
	"g_avatar":       true,
	"u_key":          true,
	"g_status":       true,
	"g_deleted":      true,
}

// Get 按主键读取仓库组（只取未删除的）。
//
// 逐字对应 application/models/group_model.php:91-108 的 get()：
// 条件为 g_key 相等且 g_status = COMMON_STATUS_NORMAL。
func (r *GroupRepo) Get(ctx context.Context, gKey string) (*domain.Group, error) {
	row := r.runner().QueryRowContext(ctx,
		"SELECT "+groupColumns+" FROM cc_groups WHERE g_key = ? AND g_status = ?",
		gKey, domain.CommonStatusNormal)
	return scanGroup(row)
}

// ListOwned 列出用户创建的仓库组。
//
// 逐字对应 application/models/group_model.php:117-123 的 list()。
func (r *GroupRepo) ListOwned(ctx context.Context, uKey string) ([]domain.Group, error) {
	rows, err := r.runner().QueryContext(ctx,
		"SELECT "+groupColumns+" FROM cc_groups WHERE u_key = ? AND g_status = ?",
		uKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGroups(rows)
}

// ListJoined 列出用户加入的仓库组。
//
// 逐字对应 application/models/group_model.php:125-134 的 listJoined()：
// 以 cc_group_members 左连接 cc_groups，只取 gm_status 与 g_status 都正常的行。
func (r *GroupRepo) ListJoined(ctx context.Context, uKey string) ([]domain.Group, error) {
	rows, err := r.runner().QueryContext(ctx, `
		SELECT cc_groups.g_key, cc_groups.u_key, cc_groups.g_type, cc_groups.g_display_name,
		       cc_groups.g_name, cc_groups.g_description, cc_groups.g_avatar, cc_groups.g_created,
		       cc_groups.g_updated, cc_groups.g_status
		FROM cc_groups
		LEFT JOIN cc_group_members ON cc_group_members.g_key = cc_groups.g_key
		WHERE cc_group_members.u_key = ?
		  AND cc_group_members.gm_status = ?
		  AND cc_groups.g_status = ?`,
		uKey, domain.CommonStatusNormal, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGroups(rows)
}

// ListMembers 列出仓库组的全部成员行（含已删除的）。
//
// 逐字对应 application/models/group_model.php:110-115 的 listMembers()：
// PHP 侧不过滤 gm_status，已删除成员也要参与 deleteFlag 的计算。
func (r *GroupRepo) ListMembers(ctx context.Context, gKey string) ([]domain.GroupMember, error) {
	rows, err := r.runner().QueryContext(ctx, `
		SELECT gm_key, g_key, u_key, gm_role, gm_created, gm_status
		FROM cc_group_members WHERE g_key = ?`, gKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.GroupMember{}
	for rows.Next() {
		var m domain.GroupMember
		var created sql.NullTime
		if err := rows.Scan(&m.Key, &m.GKey, &m.UKey, &m.Role, &created, &m.Status); err != nil {
			return nil, err
		}
		m.Created = phpTime(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// SearchByName 按名称查找仓库组。
//
// 逐字对应 application/models/group_model.php:178-186 的 searchByName()：
// 名称先按 " \t\n\r\0\x0B'\"/" 做首尾裁剪，再做精确匹配。
func (r *GroupRepo) SearchByName(ctx context.Context, name string) (*domain.Group, error) {
	row := r.runner().QueryRowContext(ctx,
		"SELECT "+groupColumns+" FROM cc_groups WHERE g_name = ? AND g_status = ? LIMIT 1",
		phpTrimGroupName(name), domain.CommonStatusNormal)
	return scanGroup(row)
}

// SearchByDisplayName 按显示名查找仓库组。
//
// 逐字对应 application/models/group_model.php:188-195 的 searchByDisplayName()。
func (r *GroupRepo) SearchByDisplayName(ctx context.Context, displayName string) (*domain.Group, error) {
	row := r.runner().QueryRowContext(ctx,
		"SELECT "+groupColumns+" FROM cc_groups WHERE g_display_name = ? AND g_status = ? LIMIT 1",
		displayName, domain.CommonStatusNormal)
	return scanGroup(row)
}

// Create 创建一个仓库组。
//
// 逐字对应 application/models/group_model.php:157-176 的 create()：
// 只写 g_key / u_key / g_display_name / g_name / g_type / g_description，
// 其余列走数据库默认值（g_avatar 为 NULL、g_created 为当前时间、
// g_updated 为 '0000-00-00 00:00:00'、g_status 为 1）。
func (r *GroupRepo) Create(ctx context.Context, g *domain.Group) error {
	_, err := r.runner().ExecContext(ctx, `
		INSERT INTO cc_groups (g_key, u_key, g_display_name, g_name, g_type, g_description)
		VALUES (?, ?, ?, ?, ?, ?)`,
		g.Key, g.OwnerKey, g.DisplayName, g.Name, g.Type, g.Description)
	return err
}

// Update 更新仓库组的指定列。
//
// 列名必须来自调用方的白名单，这里再校验一次，避免拼接风险。
func (r *GroupRepo) Update(ctx context.Context, gKey string, fields map[string]any) error {
	set := ""
	args := make([]any, 0, len(fields)+1)
	for _, col := range sortedKeys(fields) {
		if !groupWritableColumns[col] {
			return errors.New("不允许更新列 " + col)
		}
		if set != "" {
			set += ", "
		}
		set += col + " = ?"
		args = append(args, fields[col])
	}
	if set == "" {
		return nil
	}
	args = append(args, gKey)
	_, err := r.runner().ExecContext(ctx, "UPDATE cc_groups SET "+set+" WHERE g_key = ?", args...)
	return err
}

// AddMember 把用户加入仓库组。
//
// 逐字对应 application/models/group_model.php:263-283 的 addMember()：
// 已存在则把角色重置为 ROLE_GUEST 并恢复 gm_status，不存在则插入新行。
func (r *GroupRepo) AddMember(ctx context.Context, uKey, gKey string) error {
	return r.SetMemberRole(ctx, uKey, gKey, int(domain.RoleGuest))
}

// SetMemberRole 设置用户在仓库组内的角色。
//
// 逐字对应 application/models/group_model.php:285-310 的 setMemberRole()：
// 已存在则更新 gm_role 并把 gm_status 恢复为 COMMON_STATUS_NORMAL；
// 不存在则插入新行（gm_status 走数据库默认值 1）。
func (r *GroupRepo) SetMemberRole(ctx context.Context, uKey, gKey string, role int) error {
	exists, err := r.memberExists(ctx, uKey, gKey)
	if err != nil {
		return err
	}
	if exists {
		_, err = r.runner().ExecContext(ctx, `
			UPDATE cc_group_members SET gm_role = ?, gm_status = ?
			WHERE u_key = ? AND g_key = ?`,
			role, domain.CommonStatusNormal, uKey, gKey)
		return err
	}
	_, err = r.runner().ExecContext(ctx, `
		INSERT INTO cc_group_members (gm_key, u_key, g_key, gm_role)
		VALUES (?, ?, ?, ?)`,
		contract.NewKey(), uKey, gKey, role)
	return err
}

// RemoveMember 把成员移出仓库组（软删除）。
//
// 逐字对应 application/models/group_model.php:312-322 的 removeMember()。
func (r *GroupRepo) RemoveMember(ctx context.Context, uKey, gKey string) error {
	_, err := r.runner().ExecContext(ctx, `
		UPDATE cc_group_members
		SET gm_role = ?, gm_status = ?, gm_deleted = ?
		WHERE u_key = ? AND g_key = ?`,
		int(domain.RoleGuest), commonStatusDelete, phpNow(), uKey, gKey)
	return err
}

// RemoveGroup 删除仓库组（软删除）。
//
// 逐字对应 application/models/group_model.php:324-332 的 removeGroup()。
func (r *GroupRepo) RemoveGroup(ctx context.Context, gKey string) error {
	_, err := r.runner().ExecContext(ctx, `
		UPDATE cc_groups SET g_status = ?, g_deleted = ? WHERE g_key = ?`,
		commonStatusDelete, phpNow(), gKey)
	return err
}

// RepositoriesInGroup 统计仓库组内未删除的仓库数量。
//
// 逐字对应 application/models/repository_model.php:450-460 的 listInGroup()，
// 控制器侧只用它的条数判断"非空组"（0x040A）。
func (r *GroupRepo) RepositoriesInGroup(ctx context.Context, gKey string) (int, error) {
	var n int
	err := r.runner().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM cc_repositories WHERE g_key = ? AND r_status = ?`,
		gKey, domain.CommonStatusNormal).Scan(&n)
	return n, err
}

// MergeRequests 列出仓库组内的合并请求。
//
// 逐字对应 application/models/group_model.php:197-224 的 getMergeRequests()：
// 以 cc_repositories 左连接 cc_merge_requests（按目标仓库），只取
// 目标仓库属于该组、仓库与合并请求都未删除的行。
func (r *GroupRepo) MergeRequests(ctx context.Context, gKey string, status int, keyword, sort string, page, pagesize int) ([]domain.MergeRequest, error) {
	query := `
		SELECT mr.mr_key, mr.u_key, mr.mr_source_r_key, mr.mr_source_branch,
		       mr.mr_target_r_key, mr.mr_target_branch, mr.mr_id, mr.mr_title,
		       mr.mr_proceed_status, mr.mr_created, mr.mr_updated
		FROM cc_merge_requests mr
		LEFT JOIN cc_repositories rp ON mr.mr_target_r_key = rp.r_key`
	// 性能整改 C1：分页参数的最终收敛点（见 domain.ClampPaging）。
	page, pagesize = domain.ClampPaging(page, pagesize)
	args := []any{}
	if status != 0 {
		query += " WHERE mr.mr_proceed_status = ?"
		args = append(args, status)
	}
	if keyword != "" {
		// PHP 侧用 escape_like_str 转义后再拼 like。
		query += " AND (mr.mr_title LIKE ? OR mr.mr_description LIKE ?)"
		like := "%" + escapeLike(keyword) + "%"
		args = append(args, like, like)
	}
	query += " AND rp.g_key = ? AND rp.r_status = ? AND mr.mr_status = ?"
	args = append(args, gKey, domain.CommonStatusNormal, domain.CommonStatusNormal)
	query += " ORDER BY mr.mr_created " + phpSortDirection(sort)
	query += " LIMIT ? OFFSET ?"
	args = append(args, pagesize, domain.GroupPagingOffset(page, pagesize))

	rows, err := r.runner().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.MergeRequest{}
	for rows.Next() {
		var m domain.MergeRequest
		var created, updated sql.NullTime
		if err := rows.Scan(&m.Key, &m.UKey, &m.SourceRKey, &m.SourceBranch,
			&m.TargetRKey, &m.TargetBranch, &m.ID, &m.Title,
			&m.ProceedStatus, &created, &updated); err != nil {
			return nil, err
		}
		m.Created = phpTime(created)
		m.Updated = phpTime(updated)
		out = append(out, m)
	}
	return out, rows.Err()
}

// MergeRequestReviewers 列出合并请求的评审人。
//
// 逐字对应 application/models/repository_model.php:351-364 的 getMergeRequestReviewers()。
func (r *GroupRepo) MergeRequestReviewers(ctx context.Context, mrKey string) ([]domain.MergeRequestReviewer, error) {
	rows, err := r.runner().QueryContext(ctx, `
		SELECT mrr_key, mr_key, u_key, mrr_is_review
		FROM cc_merge_request_reviewers WHERE mr_key = ? AND mrr_status = ?`,
		mrKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.MergeRequestReviewer{}
	for rows.Next() {
		var rv domain.MergeRequestReviewer
		var isReview int
		if err := rows.Scan(&rv.Key, &rv.MRKey, &rv.UKey, &isReview); err != nil {
			return nil, err
		}
		rv.IsReview = isReview == 1
		out = append(out, rv)
	}
	return out, rows.Err()
}

// Activities 列出仓库组的动态。
//
// 逐字对应 application/models/group_model.php:226-254 的 listActivities()：
// 分类过滤见 domain.GroupActivityCategoryWhere，关键词匹配 a_data，
// 最后按 a_created 倒序。
func (r *GroupRepo) Activities(ctx context.Context, gKey string, category int, keyword string, page, pagesize int) ([]domain.Activity, error) {
	query := `
		SELECT a_key, a_type, u_key, a_relative_g_key, a_relative_r_key, a_relative_mr_key,
		       a_data, a_created, a_status
		FROM cc_activities`
	// 性能整改 C1：分页参数的最终收敛点（见 domain.ClampPaging）。
	page, pagesize = domain.ClampPaging(page, pagesize)
	args := []any{}
	if where, params := domain.GroupActivityCategoryWhere(category); where != "" {
		query += " WHERE " + where
		args = append(args, params...)
	}
	if keyword != "" {
		if len(args) > 0 {
			query += " AND a_data LIKE ?"
		} else {
			query += " WHERE a_data LIKE ?"
		}
		args = append(args, "%"+escapeLike(keyword)+"%")
	}
	if len(args) > 0 {
		query += " AND a_relative_g_key = ?"
	} else {
		query += " WHERE a_relative_g_key = ?"
	}
	args = append(args, gKey)
	query += " AND a_status = ?"
	args = append(args, domain.CommonStatusNormal)
	query += " ORDER BY a_created DESC LIMIT ? OFFSET ?"
	args = append(args, pagesize, domain.GroupPagingOffset(page, pagesize))

	rows, err := r.runner().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Activity{}
	for rows.Next() {
		var a domain.Activity
		var relativeG, relativeR, relativeMR, data sql.NullString
		var created sql.NullTime
		if err := rows.Scan(&a.Key, &a.Type, &a.UKey, &relativeG, &relativeR, &relativeMR,
			&data, &created, &a.Status); err != nil {
			return nil, err
		}
		a.GroupKey = nullToPtr(relativeG)
		a.Repository = nullToPtr(relativeR)
		a.MergeReq = nullToPtr(relativeMR)
		a.Data = nullToPtr(data)
		a.Created = phpTime(created)
		out = append(out, a)
	}
	return out, rows.Err()
}

// memberExists 判断成员行是否存在（不区分状态）。
func (r *GroupRepo) memberExists(ctx context.Context, uKey, gKey string) (bool, error) {
	var one int
	err := r.runner().QueryRowContext(ctx,
		"SELECT 1 FROM cc_group_members WHERE u_key = ? AND g_key = ? LIMIT 1", uKey, gKey).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// scanGroup 扫描一行仓库组记录。
func scanGroup(row scanner) (*domain.Group, error) {
	var g domain.Group
	var avatar sql.NullString
	var created, updated sql.NullTime
	err := row.Scan(&g.Key, &g.OwnerKey, &g.Type, &g.DisplayName, &g.Name,
		&g.Description, &avatar, &created, &updated, &g.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.Avatar = nullToPtr(avatar)
	g.Created = phpTime(created)
	g.Updated = phpTime(updated)
	return &g, nil
}

// scanGroups 扫描多行仓库组记录。
func scanGroups(rows *sql.Rows) ([]domain.Group, error) {
	out := []domain.Group{}
	for rows.Next() {
		var g domain.Group
		var avatar sql.NullString
		var created, updated sql.NullTime
		if err := rows.Scan(&g.Key, &g.OwnerKey, &g.Type, &g.DisplayName, &g.Name,
			&g.Description, &avatar, &created, &updated, &g.Status); err != nil {
			return nil, err
		}
		g.Avatar = nullToPtr(avatar)
		g.Created = phpTime(created)
		g.Updated = phpTime(updated)
		out = append(out, g)
	}
	return out, rows.Err()
}

// phpTime 把驱动解析出的时间转成 time.Time，
// 其 Unix() 与 PHP 的 strtotime 结果逐字一致。
//
// MySQL 的零值日期 '0000-00-00 00:00:00' 在 PHP 里是 -62169984000，
// 而 Go 的 time.Time 零值（0001-01-01）会得到 -62135596800，
// 因此这里统一走 domain.PHPTimestamp 换算。
// NULL 列在 PHP 侧是 strtotime(null) = false，即 0。
func phpTime(t sql.NullTime) time.Time {
	if !t.Valid {
		return time.Unix(0, 0).UTC()
	}
	return time.Unix(domain.PHPTimestamp(t.Time), 0).UTC()
}

// phpTrimGroupName 复刻 PHP 的 trim($name, " \t\n\r\0\x0B'\"/")。
func phpTrimGroupName(s string) string {
	return strings.Trim(s, " \t\n\r\x00\x0b'\"/")
}

// phpNow 返回 PHP 侧的当前时间字符串（date('Y-m-d H:i:s')，时区为 UTC）。
func phpNow() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

// phpSortDirection 把排序方向收敛为白名单内的关键字，杜绝拼接风险。
func phpSortDirection(sort string) string {
	if sort == "asc" {
		return "ASC"
	}
	return "DESC"
}

// escapeLike 转义 LIKE 通配符。
//
// PHP 侧用 CI 的 escape_like_str（转义符为 !），生成的 SQL 却不带 ESCAPE 子句，
// 因此这里按 MySQL 默认转义符 \ 处理 % 与 _，语义等价且更安全。
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
