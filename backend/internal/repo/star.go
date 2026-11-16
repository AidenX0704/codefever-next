package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// StarRepo 负责仓库标星（cc_repository_stars，013_repository_stars.sql）。
//
// 幂等约定：Star / Unstar 对"已经标过 / 没标过"的情况都返回 nil，
// 调用方不必先查后写，重复点击也不会报错。
type StarRepo struct {
	db *sql.DB
}

// NewStarRepo 创建标星仓储。
func NewStarRepo(db *sql.DB) *StarRepo { return &StarRepo{db: db} }

// Star 标星一个仓库。
//
// 主键 (u_key, r_key) 天然防重，用 INSERT IGNORE 实现幂等；
// 唯一可能撞上的就是这条主键约束，其它错误照常上抛。
func (r *StarRepo) Star(ctx context.Context, uKey, rKey string) error {
	if uKey == "" || rKey == "" {
		return ErrNotFound
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT IGNORE INTO cc_repository_stars (u_key, r_key) VALUES (?, ?)`,
		uKey, rKey); err != nil {
		return fmt.Errorf("写入仓库标星失败: %w", err)
	}
	return nil
}

// Unstar 取消标星一个仓库。
//
// DELETE 影响 0 行 = 本来就没标过，同样视为成功。
func (r *StarRepo) Unstar(ctx context.Context, uKey, rKey string) error {
	if uKey == "" || rKey == "" {
		return ErrNotFound
	}
	if _, err := r.db.ExecContext(ctx,
		"DELETE FROM cc_repository_stars WHERE u_key = ? AND r_key = ?", uKey, rKey); err != nil {
		return fmt.Errorf("删除仓库标星失败: %w", err)
	}
	return nil
}

// StarCounts 批量统计仓库的标星数。
//
// repository/list 与 repository/config 都用它：一条 GROUP BY 查询
// 取回整批仓库的计数，避免逐仓库 N+1。
func (r *StarRepo) StarCounts(ctx context.Context, rKeys []string) (map[string]int, error) {
	out := map[string]int{}
	if len(rKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(rKeys)
	rows, err := r.db.QueryContext(ctx, `
		SELECT r_key, COUNT(*) FROM cc_repository_stars
		WHERE r_key IN (`+strings.Join(placeholders, ", ")+`)
		GROUP BY r_key`, args...)
	if err != nil {
		return nil, fmt.Errorf("统计仓库标星数失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rKey string
		var n int
		if err := rows.Scan(&rKey, &n); err != nil {
			return nil, err
		}
		out[rKey] = n
	}
	return out, rows.Err()
}

// StarredKeys 返回用户在给定仓库集合里已标星的仓库集合。
//
// 用于 repository/list 的 starred 字段与 mode=starred 筛选。
// 给定集合为空时直接返回空映射，不发查询。
func (r *StarRepo) StarredKeys(ctx context.Context, uKey string, rKeys []string) (map[string]bool, error) {
	out := map[string]bool{}
	if uKey == "" || len(rKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(rKeys)
	args = append([]any{uKey}, args...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT r_key FROM cc_repository_stars
		WHERE u_key = ? AND r_key IN (`+strings.Join(placeholders, ", ")+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("查询用户标星仓库失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rKey string
		if err := rows.Scan(&rKey); err != nil {
			return nil, err
		}
		out[rKey] = true
	}
	return out, rows.Err()
}

// StarredRepositories 列出用户标星的全部仓库（按标星时间倒序）。
//
// mode=starred 的数据源：先取标星集合，再由调用方与"用户可见仓库"
// 求交集 —— 权限收缩后（被移出成员等）的星标行会残留在表里，
// 不能直接把残留行当可见仓库返回。
func (r *StarRepo) StarredRepositories(ctx context.Context, uKey string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r_key FROM cc_repository_stars WHERE u_key = ?
		ORDER BY s_created_at DESC`, uKey)
	if err != nil {
		return nil, fmt.Errorf("查询用户标星仓库失败: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var rKey string
		if err := rows.Scan(&rKey); err != nil {
			return nil, err
		}
		out = append(out, rKey)
	}
	return out, rows.Err()
}

// IsStarred 判断用户是否已标星某个仓库。
func (r *StarRepo) IsStarred(ctx context.Context, uKey, rKey string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `
		SELECT 1 FROM cc_repository_stars WHERE u_key = ? AND r_key = ? LIMIT 1`,
		uKey, rKey).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("查询仓库标星状态失败: %w", err)
	}
	return true, nil
}
