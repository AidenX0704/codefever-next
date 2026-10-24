package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"codefever-next/backend/internal/domain"
)

// InternalRepo 负责内部网关回调端点（/apiv1/internal/*）的数据查询。
//
// 这些查询逐字对应 PHP 侧的模型方法：
//
//	Group_model::searchByName            application/models/group_model.php:178-186
//	Repository_model::searchByName       application/models/repository_model.php:759-771
//	Repository_model::checkBranchProteced application/models/repository_model.php:2047-2050
//	User_sshkey_model::searchAuthKey     application/models/user_sshkey_model.php:62-76
//
// 之所以不并入 AccessRepo：这些查询服务于 internal 域，
// 与权限矩阵无关，混在一起会让 AccessRepo 承担两个不相关的职责。
type InternalRepo struct {
	db *sql.DB
}

// NewInternalRepo 创建内部网关回调仓储。
func NewInternalRepo(db *sql.DB) *InternalRepo { return &InternalRepo{db: db} }

// GroupKeyByName 按组名查组主键。
//
// 逐字对应 application/models/group_model.php:178-186 的 searchByName：
// 组名先 trim 掉首尾的空白、单双引号与斜杠，再按 g_name 精确匹配，
// 且只取 g_status = COMMON_STATUS_NORMAL 的记录。
//
// 查不到时返回空串（不是错误）—— 对应 PHP 侧 row_array() 返回空数组。
func (r *InternalRepo) GroupKeyByName(ctx context.Context, name string) (string, error) {
	name = strings.Trim(name, " \t\n\r\x00\x0b'\"/")
	if name == "" {
		return "", nil
	}
	var gKey sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT g_key FROM cc_groups WHERE g_name = ? AND g_status = ? LIMIT 1",
		name, domain.CommonStatusNormal).Scan(&gKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !gKey.Valid {
		return "", nil
	}
	return gKey.String, nil
}

// RepositoryByName 在指定仓库组内按仓库名查仓库。
//
// 逐字对应 application/models/repository_model.php:759-771 的 searchByName：
// 按 r_name + g_key 精确匹配，且只取 r_status = COMMON_STATUS_NORMAL 的记录。
//
// 查不到时返回 (nil, nil) —— 对应 PHP 侧返回 FALSE。
func (r *InternalRepo) RepositoryByName(ctx context.Context, name, gKey string) (*domain.InternalRepository, error) {
	if name == "" || gKey == "" {
		return nil, nil
	}
	var (
		repo  domain.InternalRepository
		rPath sql.NullString
		rName sql.NullString
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT r_key, r_name, r_path FROM cc_repositories
		WHERE r_name = ? AND g_key = ? AND r_status = ? LIMIT 1`,
		name, gKey, domain.CommonStatusNormal).Scan(&repo.Key, &rName, &rPath)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if rName.Valid {
		repo.Name = rName.String
	}
	if rPath.Valid {
		repo.Path = rPath.String
	}
	return &repo, nil
}

// RepositoryPath 返回仓库在 git-storage 下的相对路径（r_path）。
func (r *InternalRepo) RepositoryPath(ctx context.Context, rKey string) (string, error) {
	if rKey == "" {
		return "", nil
	}
	var path sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT r_path FROM cc_repositories WHERE r_key = ?", rKey).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return path.String, nil
}

// DefaultBranch 返回仓库的默认分支名；未设置时返回空串。
func (r *InternalRepo) DefaultBranch(ctx context.Context, rKey string) (string, error) {
	if rKey == "" {
		return "", nil
	}
	var branch sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT r_default_branch_name FROM cc_repositories WHERE r_key = ?", rKey).Scan(&branch)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return branch.String, nil
}

// SetDefaultBranch 更新仓库默认分支。
func (r *InternalRepo) SetDefaultBranch(ctx context.Context, rKey, branch string) error {
	if rKey == "" || branch == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE cc_repositories SET r_default_branch_name = ? WHERE r_key = ?", branch, rKey)
	return err
}

// ProtectedBranchRules 读取仓库的保护分支规则。
//
// 逐字对应 application/models/repository_model.php:2047-2050：
// 只取 r_key 匹配且 pbr_status = COMMON_STATUS_NORMAL 的规则。
func (r *InternalRepo) ProtectedBranchRules(ctx context.Context, rKey string) ([]domain.ProtectedBranchRule, error) {
	if rKey == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT pbr_key, pbr_rule, pbr_push_min_role_id, pbr_merge_min_role_id
		FROM cc_proteced_branch_rules
		WHERE r_key = ? AND pbr_status = ?`,
		rKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.ProtectedBranchRule{}
	for rows.Next() {
		var rule domain.ProtectedBranchRule
		if err := rows.Scan(&rule.Key, &rule.Rule, &rule.PushRole, &rule.MergeRole); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// ActiveSSHKeyByHash 按指纹查找未删除的 SSH 公钥。
//
// 逐字对应 application/models/user_sshkey_model.php:62-76 的 searchAuthKey：
// 指纹 = md5(base64_decode(公钥))，且只取 sk_status = COMMON_STATUS_NORMAL 的记录。
//
// 注意 repo.UserRepo.SSHKeyByHash 不带状态过滤，因此这里单独实现。
func (r *InternalRepo) ActiveSSHKeyByHash(ctx context.Context, hash string) (*domain.SSHKey, error) {
	if hash == "" {
		return nil, ErrNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT sk_key, u_key, sk_name, sk_key_type, sk_key_hash, sk_key_data, sk_status, sk_created, sk_updated
		FROM cc_ssh_keys WHERE sk_key_hash = ? AND sk_status = ? LIMIT 1`,
		hash, domain.CommonStatusNormal)
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
