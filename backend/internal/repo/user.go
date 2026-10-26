// Package repo 是 MySQL 数据访问层。
//
// 全部使用显式 SQL 与绑定参数，不引入 ORM：
// 现有 PHP 侧用 CI ActiveRecord，存在大量 N+1 与字符串拼接 LIKE，
// 显式 SQL 可以精确控制索引使用与查询形状。
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
)

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("记录不存在")

// UserRepo 负责 cc_users / cc_commit_emails / cc_ssh_keys 的读写。
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo 创建用户仓储。
func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

// userColumns 是 cc_users 的实际列集合。
//
// 注意：表中**没有** u_job / u_company 两列，尽管 PHP 侧 api/user.php 的
// update 白名单里列出了 job / company（写入这两个字段必然失败）——
// 那是原实现的一处潜在缺陷，新后端不再沿用该白名单。
const userColumns = "u_key, u_email, u_password, u_name, u_avatar, u_tel, u_calling_code, " +
	"u_team, u_role, u_location, u_notification_status, u_register_ip, u_2fa, u_admin, " +
	"u_created, u_updated, u_status"

// Get 按主键读取用户。
func (r *UserRepo) Get(ctx context.Context, uKey string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM cc_users WHERE u_key = ?", uKey)
	return scanUser(row)
}

// GetByEmail 按邮箱读取用户。
//
// 与 GetByTel 一致，过滤软删除（u_status = 4）的账号：
// PHP 的 User_model::getInfoByPwd() 也是这样，因此被禁用账号不能登录。
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM cc_users WHERE u_email = ? AND u_status <> ? LIMIT 1",
		email, domain.CommonStatusDelete)
	return scanUser(row)
}

// GetByName 按用户名读取用户。
//
// 用户名在 cc_users.u_name，公开 API 的 /api/v1/users/{username} 用它寻址
// （与公开 API 的 :username 路径段一致）。
//
// 与 GetByEmail 一致，过滤软删除（u_status = 4）的账号。
func (r *UserRepo) GetByName(ctx context.Context, name string) (*domain.User, error) {
	if name == "" {
		return nil, ErrNotFound
	}
	row := r.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM cc_users WHERE u_name = ? AND u_status <> ? LIMIT 1",
		name, domain.CommonStatusDelete)
	return scanUser(row)
}

// GetByTel 按手机号读取用户。
//
// 逐字对应 application/models/user_model.php:116-129：
// 只查 u_status != COMMON_STATUS_DELETE（软删除的行不算）。
func (r *UserRepo) GetByTel(ctx context.Context, tel string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM cc_users WHERE u_tel = ? AND u_status <> ? LIMIT 1",
		tel, domain.CommonStatusDelete)
	return scanUser(row)
}

// NewUser 是新增用户时需要写入的字段。
type NewUser struct {
	Key        string
	Email      string
	Tel        string
	Password   string
	Name       string
	Avatar     string
	RegisterIP string
}

// AddUser 插入一个新用户。
//
// 逐字对应 application/models/user_model.php:254-271（addUser）：
// PHP 只写入 insert 数组里出现过的列，其余列取数据库默认值
// （u_status 默认 1、u_notification_status 默认 0、u_admin 默认 0）。
//
// 注意 u_email 是 NOT NULL 且无默认值的列：PHP 侧用手机号注册时不会写这一列，
// MySQL 非严格模式下会落成空串，这里显式写空串，避免严格模式直接报错。
func (r *UserRepo) AddUser(ctx context.Context, u NewUser) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cc_users (u_key, u_email, u_tel, u_password, u_name, u_avatar,
			u_register_ip, u_created, u_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Key, u.Email, u.Tel, u.Password, u.Name, u.Avatar,
		u.RegisterIP, now, now)
	if err != nil {
		return fmt.Errorf("插入用户失败: %w", err)
	}
	return nil
}

// UpdateFields 更新用户表的指定列。
//
// 列名必须来自调用方的白名单，这里再做一次白名单校验，避免拼接风险。
func (r *UserRepo) UpdateFields(ctx context.Context, uKey string, fields map[string]any) error {
	allowed := map[string]bool{
		"u_name": true, "u_email": true, "u_team": true, "u_role": true,
		"u_tel": true, "u_calling_code": true,
		"u_location": true, "u_avatar": true, "u_admin": true, "u_2fa": true,
		"u_password": true, "u_notification_status": true, "u_status": true,
		"u_updated": true,
	}
	set := ""
	args := make([]any, 0, len(fields)+1)
	for _, col := range sortedKeys(fields) {
		if !allowed[col] {
			return fmt.Errorf("不允许更新列 %q", col)
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
	_, err := r.db.ExecContext(ctx, "UPDATE cc_users SET "+set+" WHERE u_key = ?", args...)
	return err
}

// CommitEmails 读取用户的提交邮箱。
//
// isCheck 为 true 时只返回已验证的邮箱（PHP 侧 $isCheck 参数）。
func (r *UserRepo) CommitEmails(ctx context.Context, uKey string, isCheck bool) ([]domain.CommitEmail, error) {
	query := "SELECT ce_key, u_key, ce_email, ce_is_check, ce_status, ce_created " +
		"FROM cc_commit_emails WHERE u_key = ? AND ce_status = ?"
	args := []any{uKey, domain.CommonStatusNormal}
	if isCheck {
		query += " AND ce_is_check = 1"
	}
	query += " ORDER BY ce_created DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.CommitEmail{}
	for rows.Next() {
		var e domain.CommitEmail
		var isCheckRaw int
		if err := rows.Scan(&e.Key, &e.UKey, &e.Email, &isCheckRaw, &e.Status, &e.Created); err != nil {
			return nil, err
		}
		e.IsCheck = isCheckRaw == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

// UnReadNotificationCount 统计未读通知数。
//
// 逐字对应 application/models/notification_model.php:80-86：
// 只查 cc_notification_users，条件为 nu_is_read = NOTIFY_UNREAD(2) 且 nu_status = COMMON_STATUS_NORMAL(1)。
func (r *UserRepo) UnReadNotificationCount(ctx context.Context, uKey string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM cc_notification_users
		WHERE u_key = ? AND nu_is_read = ? AND nu_status = ?`,
		uKey, NotifyUnread, domain.CommonStatusNormal).Scan(&n)
	return n, err
}

// SSHKeys 列出用户的 SSH 公钥。
func (r *UserRepo) SSHKeys(ctx context.Context, uKey string) ([]domain.SSHKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sk_key, u_key, sk_name, sk_key_type, sk_key_hash, sk_key_data, sk_status, sk_created, sk_updated
		FROM cc_ssh_keys WHERE u_key = ? ORDER BY sk_created DESC`, uKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.SSHKey{}
	for rows.Next() {
		var k domain.SSHKey
		if err := rows.Scan(&k.Key, &k.UKey, &k.Name, &k.KeyType, &k.KeyHash, &k.KeyData,
			&k.Status, &k.Created, &k.Updated); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// SSHKeyByID 按主键读取一条 SSH 公钥。
func (r *UserRepo) SSHKeyByID(ctx context.Context, skKey string) (*domain.SSHKey, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT sk_key, u_key, sk_name, sk_key_type, sk_key_hash, sk_key_data, sk_status, sk_created, sk_updated
		FROM cc_ssh_keys WHERE sk_key = ? LIMIT 1`, skKey)
	var k domain.SSHKey
	err := row.Scan(&k.Key, &k.UKey, &k.Name, &k.KeyType, &k.KeyHash, &k.KeyData,
		&k.Status, &k.Created, &k.Updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// SSHKeyByHash 按指纹查找 SSH 公钥（用于查重）。
func (r *UserRepo) SSHKeyByHash(ctx context.Context, hash string) (*domain.SSHKey, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT sk_key, u_key, sk_name, sk_key_type, sk_key_hash, sk_key_data, sk_status, sk_created, sk_updated
		FROM cc_ssh_keys WHERE sk_key_hash = ? LIMIT 1`, hash)
	var k domain.SSHKey
	err := row.Scan(&k.Key, &k.UKey, &k.Name, &k.KeyType, &k.KeyHash, &k.KeyData,
		&k.Status, &k.Created, &k.Updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// AddSSHKey 插入一条 SSH 公钥。
func (r *UserRepo) AddSSHKey(ctx context.Context, k *domain.SSHKey) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cc_ssh_keys (sk_key, u_key, sk_name, sk_key_type, sk_key_hash, sk_key_data, sk_status, sk_created, sk_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.Key, k.UKey, k.Name, k.KeyType, k.KeyHash, k.KeyData, k.Status, k.Created, k.Updated)
	return err
}

// DeleteSSHKey 删除一条 SSH 公钥。
func (r *UserRepo) DeleteSSHKey(ctx context.Context, skKey string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM cc_ssh_keys WHERE sk_key = ?", skKey)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// IsCommitEmailExists 判断提交邮箱是否已存在。
//
// 逐字对应 application/models/user_model.php:190-200。
func (r *UserRepo) IsCommitEmailExists(ctx context.Context, uKey, email string) (bool, error) {
	if uKey == "" || email == "" {
		return false, nil
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM cc_commit_emails
		WHERE u_key = ? AND ce_email = ? AND ce_status = ?`,
		uKey, email, domain.CommonStatusNormal).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// AddCommitEmail 新增一个提交邮箱（未验证状态）。
//
// 逐字对应 application/models/user_model.php 的 addCommitEmail()。
func (r *UserRepo) AddCommitEmail(ctx context.Context, uKey, email string) (string, error) {
	key := contract.NewKey()
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cc_commit_emails (ce_key, u_key, ce_email, ce_is_check, ce_status, ce_created, ce_updated)
		VALUES (?, ?, ?, 0, ?, ?, ?)`,
		key, uKey, email, domain.CommonStatusNormal, now, now)
	if err != nil {
		return "", err
	}
	return key, nil
}

// UpdateCommitEmail 更新一个提交邮箱。
func (r *UserRepo) UpdateCommitEmail(ctx context.Context, ceKey string, fields map[string]any) error {
	allowed := map[string]bool{
		"ce_is_check": true, "ce_status": true, "ce_deleted": true, "ce_updated": true,
	}
	set := ""
	args := make([]any, 0, len(fields)+1)
	for _, col := range sortedKeys(fields) {
		if !allowed[col] {
			return fmt.Errorf("不允许更新列 %q", col)
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
	args = append(args, ceKey)
	res, err := r.db.ExecContext(ctx, "UPDATE cc_commit_emails SET "+set+" WHERE ce_key = ?", args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner 抽象 *sql.Row 与 *sql.Rows 的公共 Scan 能力。
type scanner interface {
	Scan(dest ...any) error
}

// scanUser 扫描一行用户记录。
//
// 可空列用 sql.NullString 承接：PHP 的 row_array() 对 NULL 返回 NULL，
// JSON 编码后是 null 而不是空字符串，契约要求逐字一致。
func scanUser(row scanner) (*domain.User, error) {
	var u domain.User
	var adminRaw int
	var avatar, tel, callingCode, team, role, location, registerIP, twoFA sql.NullString
	err := row.Scan(&u.Key, &u.Email, &u.Password, &u.Name, &avatar, &tel, &callingCode,
		&team, &role, &location, &u.NotificationStatus, &registerIP, &twoFA, &adminRaw,
		&u.Created, &u.Updated, &u.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Admin = adminRaw != 0
	u.Avatar = nullToPtr(avatar)
	u.Tel = nullToPtr(tel)
	u.CallingCode = nullToPtr(callingCode)
	u.Team = nullToPtr(team)
	u.Role = nullToPtr(role)
	u.Location = nullToPtr(location)
	u.RegisterIP = nullToPtr(registerIP)
	u.TwoFA = nullToPtr(twoFA)
	return &u, nil
}

// nullToPtr 把 sql.NullString 转为 *string（NULL → nil）。
func nullToPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

// sortedKeys 返回有序的键列表，保证生成的 SQL 稳定可测。
func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
