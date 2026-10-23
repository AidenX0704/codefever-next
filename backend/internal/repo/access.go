package repo

import (
	"context"
	"database/sql"
	"errors"

	"codefever-next/backend/internal/domain"
)

// AccessRepo 负责角色解析：用户在仓库组与仓库上的角色。
//
// 角色继承链（逐字对应 PHP 侧）：
//
//	仓库组：组 owner → ROLE_OWNER；否则查 cc_group_members.gm_role；否则 ROLE_NO_PERMISSION
//	  （application/models/group_model.php:136-155）
//
//	仓库：仓库 owner → ROLE_OWNER；否则查 cc_repository_members.rm_role；否则回落到仓库所属组的角色
//	  （application/models/repository_model.php:727-757）
//
// 注意：仓库不存在时返回 ROLE_NO_PERMISSION，而不是 ROLE_NO_BODY ——
// 这与 PHP 一致，且会导致权限校验失败后返回 0x0106（实测确认）。
type AccessRepo struct {
	db *sql.DB
}

// NewAccessRepo 创建访问控制仓储。
func NewAccessRepo(db *sql.DB) *AccessRepo { return &AccessRepo{db: db} }

// GroupRole 解析用户在仓库组上的角色。
func (r *AccessRepo) GroupRole(ctx context.Context, gKey, uKey string) (domain.Role, error) {
	if gKey == "" || uKey == "" {
		return domain.RoleNoPermission, nil
	}

	var owner sql.NullString
	err := r.db.QueryRowContext(ctx, "SELECT u_key FROM cc_groups WHERE g_key = ?", gKey).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RoleNoPermission, nil
	}
	if err != nil {
		return domain.RoleNoPermission, err
	}
	if owner.Valid && owner.String == uKey {
		return domain.RoleOwner, nil
	}

	var role int
	err = r.db.QueryRowContext(ctx, `
		SELECT gm_role FROM cc_group_members
		WHERE g_key = ? AND u_key = ? AND gm_status = ? LIMIT 1`,
		gKey, uKey, domain.CommonStatusNormal).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RoleNoPermission, nil
	}
	if err != nil {
		return domain.RoleNoPermission, err
	}
	return domain.Role(role), nil
}

// RepositoryRole 解析用户在仓库上的角色。
func (r *AccessRepo) RepositoryRole(ctx context.Context, rKey, uKey string) (domain.Role, error) {
	if rKey == "" || uKey == "" {
		return domain.RoleNoPermission, nil
	}

	var owner sql.NullString
	var gKey sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT u_key, g_key FROM cc_repositories WHERE r_key = ?", rKey).Scan(&owner, &gKey)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RoleNoPermission, nil
	}
	if err != nil {
		return domain.RoleNoPermission, err
	}
	if owner.Valid && owner.String == uKey {
		return domain.RoleOwner, nil
	}

	var role int
	err = r.db.QueryRowContext(ctx, `
		SELECT rm_role FROM cc_repository_members
		WHERE r_key = ? AND u_key = ? AND rm_status = ? LIMIT 1`,
		rKey, uKey, domain.CommonStatusNormal).Scan(&role)
	if err == nil && role != 0 {
		return domain.Role(role), nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.RoleNoPermission, err
	}

	// 回落到仓库所属组的角色。
	if !gKey.Valid {
		return domain.RoleNoPermission, nil
	}
	return r.GroupRole(ctx, gKey.String, uKey)
}

// RepositoryGroupKey 返回仓库所属的仓库组主键。
func (r *AccessRepo) RepositoryGroupKey(ctx context.Context, rKey string) (string, error) {
	var gKey sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT g_key FROM cc_repositories WHERE r_key = ?", rKey).Scan(&gKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if !gKey.Valid {
		return "", ErrNotFound
	}
	return gKey.String, nil
}

// RepositoryOwner 返回仓库所有者的用户主键。
func (r *AccessRepo) RepositoryOwner(ctx context.Context, rKey string) (string, error) {
	var owner sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT u_key FROM cc_repositories WHERE r_key = ?", rKey).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return owner.String, nil
}

// RepositoryPath 返回仓库在 git-storage 下的相对路径（r_path）。
func (r *AccessRepo) RepositoryPath(ctx context.Context, rKey string) (string, error) {
	var path sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT r_path FROM cc_repositories WHERE r_key = ?", rKey).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if !path.Valid {
		return "", ErrNotFound
	}
	return path.String, nil
}
