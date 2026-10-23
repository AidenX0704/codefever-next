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

// AdminRepo 负责管理域（/api/admin/*）的数据访问。
//
// 与其它仓储一样使用显式 SQL 与绑定参数，不引入 ORM。
// 查询形状逐字对齐 application/models/user_model.php、
// application/models/group_model.php 与 application/models/repository_model.php
// 中被 admin 控制器用到的那几个方法。
type AdminRepo struct {
	db *sql.DB
	// tx 非空时，所有语句都在该事务里执行。
	tx *sql.Tx
}

// NewAdminRepo 创建管理域仓储。
func NewAdminRepo(db *sql.DB) *AdminRepo { return &AdminRepo{db: db} }

// runner 返回当前生效的语句执行句柄。
func (r *AdminRepo) runner() querier {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// WithTx 在一个事务里执行 fn。
//
// fn 收到的 AdminRepo 绑定到该事务；fn 返回错误即回滚。
//
// PHP 侧的 admin 写操作**没有任何事务保护**（多步写入可能只落一半），
// 这里显式补上。
func (r *AdminRepo) WithTx(ctx context.Context, fn func(tx *AdminRepo) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(&AdminRepo{db: r.db, tx: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// adminUserColumns 是管理域用到的用户列集合。
//
// 注意 u_created 用 CAST(... AS CHAR) 取原始字符串：
// 管理页要展示的是 "2022-01-01 00:00:00" 这样的原始值，
// 与 PHP 侧 row_array() 的返回一致（见 admin.userList 的 "created" 字段）。
const adminUserColumns = "u_key, u_email, u_name, u_avatar, u_2fa, " +
	"CAST(u_created AS CHAR), u_status, u_admin"

// adminUserWritableColumns 是管理域允许写入的用户列白名单。
var adminUserWritableColumns = map[string]bool{
	"u_name":        true,
	"u_email":       true,
	"u_avatar":      true,
	"u_2fa":         true,
	"u_password":    true,
	"u_admin":       true,
	"u_status":      true,
	"u_updated":     true,
	"u_register_ip": true,
}

// ---------------------------------------------------------------------------
// 用户
// ---------------------------------------------------------------------------

// ListUsers 列出管理域的用户列表。
//
// 逐字对应 application/models/user_model.php:311-333 的 getUserList()：
//  1. category 决定过滤条件（见 domain.AdminUserCategoryWhere）；
//  2. keyword 命中 u_name 或 u_email；
//  3. sort 命中白名单时按该列**倒序**；
//  4. 分页。
//
// 注意：PHP 侧**不过滤 u_status**（"已禁用"是独立页签），
// 因此这里也不加状态条件。
func (r *AdminRepo) ListUsers(ctx context.Context, category int, keyword, sort string, page, pagesize int) ([]domain.AdminUser, error) {
	query := "SELECT " + adminUserColumns + " FROM cc_users"
	where, args := domain.AdminUserCategoryWhere(category)
	if where != "" {
		query += " WHERE " + where
	}
	if kw, params := domain.AdminUserKeywordWhere(keyword); kw != "" {
		if where != "" {
			query += " AND " + kw
		} else {
			query += " WHERE " + kw
		}
		args = append(args, params...)
	}
	if column := domain.AdminUserSortColumn(sort); column != "" {
		query += " ORDER BY " + column + " DESC"
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, pagesize, domain.AdminPagingOffset(page, pagesize))

	rows, err := r.runner().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.AdminUser{}
	for rows.Next() {
		var u domain.AdminUser
		var avatar, twoFA sql.NullString
		var adminRaw int
		if err := rows.Scan(&u.Key, &u.Email, &u.Name, &avatar, &twoFA,
			&u.Created, &u.Status, &adminRaw); err != nil {
			return nil, err
		}
		u.Avatar = nullToPtr(avatar)
		u.TwoFA = nullToPtr(twoFA)
		u.Admin = adminRaw != 0
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountUsers 统计管理域的用户总数。
//
// 逐字对应 application/models/user_model.php:323-325 的 count 分支：
// 过滤条件与 ListUsers 完全一致，但不分页。
func (r *AdminRepo) CountUsers(ctx context.Context, category int, keyword string) (int, error) {
	query := "SELECT COUNT(*) FROM cc_users"
	where, args := domain.AdminUserCategoryWhere(category)
	if where != "" {
		query += " WHERE " + where
	}
	if kw, params := domain.AdminUserKeywordWhere(keyword); kw != "" {
		if where != "" {
			query += " AND " + kw
		} else {
			query += " WHERE " + kw
		}
		args = append(args, params...)
	}
	var n int
	err := r.runner().QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

// AddUser 新建一个用户。
//
// 逐字对应 application/models/user_model.php:254-271 的 addUser()：
// 写入 u_key / u_password / u_name / u_email / u_avatar，
// 并补上 u_register_ip 与 u_created / u_updated。
//
// 其余列走数据库默认值（u_calling_code = 86、u_admin = 0、u_status = 1）。
func (r *AdminRepo) AddUser(ctx context.Context, u *domain.AdminUser, password, registerIP string) error {
	now := adminNow()
	_, err := r.runner().ExecContext(ctx, `
		INSERT INTO cc_users (u_key, u_email, u_password, u_name, u_avatar, u_register_ip, u_created, u_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Key, u.Email, password, u.Name, u.Avatar, registerIP, now, now)
	return err
}

// UpdateUserFields 更新用户的指定列。
//
// 列名必须来自调用方的白名单，这里再校验一次，避免拼接风险。
//
// PHP 侧的 updateUser() 还会顺带写 u_updated，这里同样处理。
func (r *AdminRepo) UpdateUserFields(ctx context.Context, uKey string, fields map[string]any) error {
	if _, ok := fields["u_updated"]; !ok {
		fields["u_updated"] = adminNow()
	}
	set := ""
	args := make([]any, 0, len(fields)+1)
	for _, col := range sortedKeys(fields) {
		if !adminUserWritableColumns[col] {
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
	args = append(args, uKey)
	_, err := r.runner().ExecContext(ctx, "UPDATE cc_users SET "+set+" WHERE u_key = ?", args...)
	return err
}

// ---------------------------------------------------------------------------
// 仓库组
// ---------------------------------------------------------------------------

// ListGroups 列出管理域的仓库组列表。
//
// 逐字对应 application/models/group_model.php:344-361 的 getGroupList()：
// keyword 命中 g_display_name，只取 g_status 正常的组，
// 命中排序白名单时按该列倒序，最后分页。
func (r *AdminRepo) ListGroups(ctx context.Context, keyword, sort string, page, pagesize int) ([]domain.Group, error) {
	where, args := adminListWhere("g_display_name", keyword, "g_status", domain.CommonStatusNormal)

	query := "SELECT " + groupColumns + " FROM cc_groups WHERE " + where
	if column := domain.AdminGroupSortColumn(sort); column != "" {
		query += " ORDER BY " + column + " DESC"
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, pagesize, domain.AdminPagingOffset(page, pagesize))

	rows, err := r.runner().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGroups(rows)
}

// CountGroups 统计管理域的仓库组总数。
func (r *AdminRepo) CountGroups(ctx context.Context, keyword string) (int, error) {
	where, args := adminListWhere("g_display_name", keyword, "g_status", domain.CommonStatusNormal)

	var n int
	err := r.runner().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cc_groups WHERE "+where, args...).Scan(&n)
	return n, err
}

// ---------------------------------------------------------------------------
// 仓库
// ---------------------------------------------------------------------------

// ListRepositories 列出管理域的仓库列表。
//
// 逐字对应 application/models/repository_model.php:2081-2098 的 getRepositoryList()：
// keyword 命中 r_display_name，只取 r_status 正常的仓库，
// 命中排序白名单时按该列倒序，最后分页。
func (r *AdminRepo) ListRepositories(ctx context.Context, keyword, sort string, page, pagesize int) ([]domain.Repository, error) {
	where, args := adminListWhere("r_display_name", keyword, "r_status", domain.CommonStatusNormal)

	query := "SELECT " + repoCols(repositoryColumnsTpl, "") + " FROM cc_repositories WHERE " + where
	if column := domain.AdminRepositorySortColumn(sort); column != "" {
		query += " ORDER BY " + column + " DESC"
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, pagesize, domain.AdminPagingOffset(page, pagesize))

	rows, err := r.runner().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRepositories(rows)
}

// CountRepositories 统计管理域的仓库总数。
func (r *AdminRepo) CountRepositories(ctx context.Context, keyword string) (int, error) {
	where, args := adminListWhere("r_display_name", keyword, "r_status", domain.CommonStatusNormal)

	var n int
	err := r.runner().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cc_repositories WHERE "+where, args...).Scan(&n)
	return n, err
}

// adminListWhere 拼出列表查询的 WHERE 子句。
//
// keyword 条件可选（PHP 侧的 $this->db->like），状态条件恒存在，
// 两者之间是 AND 关系。
func adminListWhere(keywordColumn, keyword, statusColumn string, statusValue int) (string, []any) {
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if keyword != "" {
		clauses = append(clauses, keywordColumn+" LIKE ?")
		args = append(args, "%"+escapeLike(keyword)+"%")
	}
	clauses = append(clauses, statusColumn+" = ?")
	args = append(args, statusValue)
	return strings.Join(clauses, " AND "), args
}

// ---------------------------------------------------------------------------
// 仓库成员
// ---------------------------------------------------------------------------

// AddRepositoryMember 把用户加入仓库。
//
// 逐字对应 application/models/repository_model.php:493-517 的 addMember()：
// 已有成员行则把角色重置为 ROLE_GUEST 并恢复 rm_status，
// 否则插入新行（rm_role = ROLE_GUEST）。
//
// 与 PHP 的差别：PHP 忽略 insert 的结果并恒返回 TRUE，
// 新实现把真实的数据库错误抛出去（由控制器转成 0x0405）。
func (r *AdminRepo) AddRepositoryMember(ctx context.Context, uKey, rKey string) error {
	exists, err := r.repositoryMemberExists(ctx, uKey, rKey)
	if err != nil {
		return err
	}
	if exists {
		_, err = r.runner().ExecContext(ctx, `
			UPDATE cc_repository_members SET rm_role = ?, rm_status = ?
			WHERE u_key = ? AND r_key = ?`,
			int(domain.RoleGuest), domain.CommonStatusNormal, uKey, rKey)
		return err
	}
	_, err = r.runner().ExecContext(ctx, `
		INSERT INTO cc_repository_members (rm_key, u_key, r_key, rm_role)
		VALUES (?, ?, ?, ?)`,
		contract.NewKey(), uKey, rKey, int(domain.RoleGuest))
	return err
}

// SetRepositoryMemberRole 设置用户在仓库内的角色。
//
// 逐字对应 application/models/repository_model.php:519-548 的 setMemberRole()：
// 已有成员行则更新 rm_role 并把 rm_status 恢复为 COMMON_STATUS_NORMAL；
// 否则插入新行（rm_status 走数据库默认值 1）。
func (r *AdminRepo) SetRepositoryMemberRole(ctx context.Context, uKey, rKey string, role int) error {
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

// RemoveRepositoryMember 把成员移出仓库（软删除）。
//
// 逐字对应 application/models/repository_model.php:550-564 的 removeMember()：
// 角色置为 ROLE_GUEST、状态置为 COMMON_STATUS_DELETE、记录删除时间。
func (r *AdminRepo) RemoveRepositoryMember(ctx context.Context, uKey, rKey string) error {
	_, err := r.runner().ExecContext(ctx, `
		UPDATE cc_repository_members
		SET rm_role = ?, rm_status = ?, rm_deleted = ?
		WHERE u_key = ? AND r_key = ?`,
		int(domain.RoleGuest), commonStatusDelete, adminNow(), uKey, rKey)
	return err
}

// repositoryMemberExists 判断仓库成员行是否存在（不区分状态）。
func (r *AdminRepo) repositoryMemberExists(ctx context.Context, uKey, rKey string) (bool, error) {
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
// 系统用量
// ---------------------------------------------------------------------------

// UsageCounts 统计正常状态的用户、仓库与仓库组数量。
//
// 逐字对应 application/controllers/api/admin.php:362-374。
func (r *AdminRepo) UsageCounts(ctx context.Context) (domain.AdminUsage, error) {
	var out domain.AdminUsage

	if err := r.runner().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cc_users WHERE u_status = ?", domain.CommonStatusNormal).Scan(&out.User); err != nil {
		return out, err
	}
	if err := r.runner().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cc_repositories WHERE r_status = ?", domain.CommonStatusNormal).Scan(&out.Repository); err != nil {
		return out, err
	}
	if err := r.runner().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cc_groups WHERE g_status = ?", domain.CommonStatusNormal).Scan(&out.Group); err != nil {
		return out, err
	}
	return out, nil
}

// adminNow 返回 PHP 侧的当前时间字符串（date('Y-m-d H:i:s')，时区为 UTC）。
func adminNow() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}
