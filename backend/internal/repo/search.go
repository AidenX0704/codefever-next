package repo

import (
	"context"
	"database/sql"
	"strings"

	"codefever-next/backend/internal/domain"
)

// SearchRepo 提供全局搜索（阶段一：元数据搜索）的只读查询。
//
// **可见性怎么算**（这里最容易出错，务必读）：仓库对"我"可见的判定与
// `RepositoryRepo.ListJoined` **完全一致**，三种来源取并集：
//
//	(1) 我是仓库所有者            → cc_repositories.u_key = 我
//	(2) 我是该仓库的直接成员      → cc_repository_members(u_key=我, rm_status=1)
//	(3) 该仓库属于我可见的仓库组  → 组的所有者是我，或我是该组成员
//
// 早先的实现只看了 (2)，结果**仓库所有者搜不到自己的仓库** —— 在真实实例上撞到过
// （`cc_repository_members` 为空、仓库靠 `u_key` 归属）。现在统一走
// `visibleRepositoryPredicate`，与列表页口径对齐。
//
// **为什么权限写在 SQL 而不是查完再过滤**：过滤发生在 LIMIT 之后会得出错误结论 ——
// 取 10 条里有 5 条无权，回给前端就是 5 条，"其实还有更多有权的结果"被吞掉。
//
// **为什么用 `LIKE '%x%'`**：本阶段关键词是"人记得住的名字片段"，前缀/分词索引
// 覆盖不了"记得中间几个字"；`LIKE '%x%'` 用不上索引，但当前量级（仓库/组/合并请求
// 都在万级以内）够用。升级路径见 `domain/search.go` 顶部注释。
type SearchRepo struct {
	db *sql.DB
}

// NewSearchRepo 创建搜索仓储。
func NewSearchRepo(db *sql.DB) *SearchRepo { return &SearchRepo{db: db} }

// visibleRepositoryPredicate 返回"仓库对我可见"的 SQL 片段与对应参数。
//
// alias 是 `cc_repositories` 在该查询里的别名（必须带：子查询里还有同表的其它别名）。
// 返回的片段自带外层括号，可直接拼进 `WHERE … AND <片段> AND …`。
//
// **参数顺序与返回的 args 严格一致**；`search_test.go` 里有占位符数量与参数数量
// 一致性的守护用例（这类拼接最容易出"少传一个 ?"，而它只在运行期报错）。
func visibleRepositoryPredicate(alias, uKey string) (string, []any) {
	fragment := `(` + alias + `.u_key = ?` +
		` OR EXISTS (SELECT 1 FROM cc_repository_members vrm` +
		`   WHERE vrm.r_key = ` + alias + `.r_key AND vrm.u_key = ? AND vrm.rm_status = ?)` +
		` OR EXISTS (SELECT 1 FROM cc_groups vg` +
		`   WHERE vg.g_key = ` + alias + `.g_key AND vg.g_status = ?` +
		`     AND (vg.u_key = ?` +
		`       OR EXISTS (SELECT 1 FROM cc_group_members vgm` +
		`         WHERE vgm.g_key = vg.g_key AND vgm.u_key = ? AND vgm.gm_status = ?))))`
	args := []any{
		uKey,                            // (1) 我是所有者
		uKey, domain.CommonStatusNormal, // (2) 我是直接成员
		domain.CommonStatusNormal, uKey, // (3) 组可用 + 组的所有者是我
		uKey, domain.CommonStatusNormal, // (3) 或我是该组成员
	}
	return fragment, args
}

// SearchRepositories 搜索我可见的仓库（名称 / 显示名 / 描述）。
//
// limit 由调用方传 `bucketLimit+1`，多取的那一条只用于判断"还有更多"。
func (r *SearchRepo) SearchRepositories(ctx context.Context, uKey, keyword string, limit int) ([]domain.SearchRepository, error) {
	if uKey == "" || keyword == "" || limit <= 0 {
		return []domain.SearchRepository{}, nil
	}
	visible, visibleArgs := visibleRepositoryPredicate("r", uKey)
	like := "%" + escapeLike(keyword) + "%"
	args := append([]any{domain.CommonStatusNormal}, visibleArgs...)
	args = append(args, like, like, like, limit)

	rows, err := r.db.QueryContext(ctx, `
		SELECT r.r_key, r.r_name, r.r_display_name, r.r_description, r.r_avatar,
		       CAST(r.r_updated AS CHAR), g.g_name, g.g_display_name
		FROM cc_repositories r
		LEFT JOIN cc_groups g ON g.g_key = r.g_key
		WHERE r.r_status = ? AND `+visible+`
		  AND (r.r_name LIKE ? OR r.r_display_name LIKE ? OR r.r_description LIKE ?)
		ORDER BY r.r_updated DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.SearchRepository{}
	for rows.Next() {
		var item domain.SearchRepository
		var avatar, updated, groupName, groupDisplay sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.DisplayName, &item.Description, &avatar,
			&updated, &groupName, &groupDisplay); err != nil {
			return nil, err
		}
		item.Avatar = nullToPtr(avatar)
		item.Updated = domain.PhpTime(updated.String)
		item.GroupName = groupName.String
		item.GroupDisplayName = groupDisplay.String
		out = append(out, item)
	}
	return out, rows.Err()
}

// SearchGroups 搜索我可见的仓库组（名称 / 显示名 / 描述）。
//
// 可见性口径与 `groupsOfUser` 一致：我拥有的 **或** 我是成员的组。
func (r *SearchRepo) SearchGroups(ctx context.Context, uKey, keyword string, limit int) ([]domain.SearchGroup, error) {
	if uKey == "" || keyword == "" || limit <= 0 {
		return []domain.SearchGroup{}, nil
	}
	like := "%" + escapeLike(keyword) + "%"
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.g_key, g.g_name, g.g_display_name, g.g_description, g.g_avatar,
		       CAST(g.g_updated AS CHAR),
		       (SELECT COUNT(*) FROM cc_repositories r2
		         WHERE r2.g_key = g.g_key AND r2.r_status = ?) AS repo_count
		FROM cc_groups g
		WHERE g.g_status = ?
		  AND (g.u_key = ?
		       OR EXISTS (SELECT 1 FROM cc_group_members gm
		                   WHERE gm.g_key = g.g_key AND gm.u_key = ? AND gm.gm_status = ?))
		  AND (g.g_name LIKE ? OR g.g_display_name LIKE ? OR g.g_description LIKE ?)
		ORDER BY g.g_updated DESC
		LIMIT ?`,
		domain.CommonStatusNormal,
		domain.CommonStatusNormal,
		uKey,
		uKey, domain.CommonStatusNormal,
		like, like, like,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.SearchGroup{}
	for rows.Next() {
		var item domain.SearchGroup
		var avatar, updated sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.DisplayName, &item.Description, &avatar,
			&updated, &item.RepositoryCount); err != nil {
			return nil, err
		}
		item.Avatar = nullToPtr(avatar)
		item.Updated = domain.PhpTime(updated.String)
		out = append(out, item)
	}
	return out, rows.Err()
}

// SearchMergeRequests 搜索我可见仓库下的合并请求标题。
//
// 只搜 `mr_title`：描述是自由文本、噪音极大，而阶段一按更新时间倒序（不做相关性排序），
// 把描述纳进来只会让结果更难用。
//
// 注意：`DISTINCT` + `ORDER BY` 必须排**已选中的列**，否则 MySQL 报
// 3065 "Expression #1 of ORDER BY clause is not in SELECT list"（真实实例上撞过）。
// 因此时间列取别名 `updated_at` 并按其排序；"YYYY-MM-DD HH:MM:SS" 字典序与时间序一致。
func (r *SearchRepo) SearchMergeRequests(ctx context.Context, uKey, keyword string, limit int) ([]domain.SearchMergeRequest, error) {
	if uKey == "" || keyword == "" || limit <= 0 {
		return []domain.SearchMergeRequest{}, nil
	}
	visible, visibleArgs := visibleRepositoryPredicate("vr", uKey)
	like := "%" + escapeLike(keyword) + "%"

	args := []any{domain.CommonStatusNormal, like, domain.CommonStatusNormal}
	args = append(args, visibleArgs...)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT mr.mr_key, mr.mr_source_r_key, mr.mr_source_branch,
		       mr.mr_target_r_key, mr.mr_target_branch, mr.mr_id, mr.mr_title,
		       mr.mr_proceed_status, CAST(mr.mr_created AS CHAR) AS created_at,
		       CAST(mr.mr_updated AS CHAR) AS updated_at, mr.u_key
		FROM cc_merge_requests mr
		WHERE mr.mr_status = ? AND mr.mr_title LIKE ?
		  AND EXISTS (SELECT 1 FROM cc_repositories vr
		               WHERE vr.r_key = mr.mr_target_r_key AND vr.r_status = ? AND `+visible+`)
		ORDER BY updated_at DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.SearchMergeRequest{}
	for rows.Next() {
		var item domain.SearchMergeRequest
		var created, updated sql.NullString
		if err := rows.Scan(&item.ID, &item.SourceRepository, &item.SourceBranch,
			&item.TargetRepository, &item.TargetBranch, &item.Number, &item.Title,
			&item.Status, &created, &updated, &item.Creator); err != nil {
			return nil, err
		}
		item.Create = domain.PhpTime(created.String)
		item.Update = domain.PhpTime(updated.String)
		// 与首页一致：搜索不填 reviewers / commit（填它们要逐条 git log）。
		item.Reviewers = []any{}
		out = append(out, item)
	}
	return out, rows.Err()
}

// TrimSearchKeyword 规范化搜索关键词。
//
// 只做两件事：去首尾空白、把连续空白压成一个空格（"a   b" 与 "a b" 等价；
// 全角空格 U+3000 也按 unicode 空白处理）。
// **不做长度截断** —— 截断会在用户不知情的情况下改变语义；超长关键词由 handler
// 按 errorcode 拒绝（见 handlers/search.go）。
func TrimSearchKeyword(raw string) string {
	return strings.Join(strings.Fields(raw), " ")
}
