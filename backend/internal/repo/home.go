package repo

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"codefever-next/backend/internal/domain"
)

// HomeRepo 提供首页聚合所需的**只读**查询。
//
// 为什么不复用现有方法：
//   - `MergeRequests` 是按仓库集合 + 状态 + 关键词分页，表达不了
//     "指派给我且我还没评审"这种以**人**为主体的条件；
//   - `ListActivities` 是单仓库维度，首页需要"我可见的任意仓库 + 我所在的组"。
//
// 每条查询都在 `011_perf_index_home.sql` 里有对应索引：
//   - PendingReviews       → idx_mrr_user_review (u_key, mrr_is_review, mr_key)
//   - MyOpenMergeRequests  → idx_mr_creator_status (u_key, mr_proceed_status, mr_updated)
//   - RecentActivities     → idx_act_repo_created (a_relative_r_key, a_created)
//     idx_act_group_created (a_relative_g_key, a_created)
type HomeRepo struct {
	db *sql.DB
}

// NewHomeRepo 创建首页聚合仓储。
func NewHomeRepo(db *sql.DB) *HomeRepo { return &HomeRepo{db: db} }

// PendingReviews 列出"指派给我、我还没评审、MR 仍待合并"的合并请求。
//
// mrr_is_review：1 = 已评审，2 = 未评审（`domain.GlobalFalse`）。
// 越权过滤不在这里做 —— 仓库可见性含**组继承**，必须走 `RepositoryRepo.RolesFor`，
// SQL 里判断不了（见 handlers/home.go）。
func (r *HomeRepo) PendingReviews(ctx context.Context, uKey string, limit int) ([]domain.RepositoryMergeRequest, error) {
	if uKey == "" || limit <= 0 {
		return []domain.RepositoryMergeRequest{}, nil
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+repoCols(mergeRequestColumnsTpl, "mr")+" "+
			"FROM cc_merge_request_reviewers mrr "+
			"JOIN cc_merge_requests mr ON mr.mr_key = mrr.mr_key "+
			"WHERE mrr.u_key = ? AND mrr.mrr_is_review = ? AND mrr.mrr_status = ? "+
			"AND mr.mr_status = ? AND mr.mr_proceed_status = ? "+
			"ORDER BY mr.mr_updated DESC LIMIT ?",
		uKey, domain.GlobalFalse, domain.CommonStatusNormal,
		domain.CommonStatusNormal, domain.MergeRequestStatusOpen, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMergeRequests(rows)
}

// MyOpenMergeRequests 列出我发起、仍未合并的合并请求（`mr.u_key` 是发起人）。
func (r *HomeRepo) MyOpenMergeRequests(ctx context.Context, uKey string, limit int) ([]domain.RepositoryMergeRequest, error) {
	if uKey == "" || limit <= 0 {
		return []domain.RepositoryMergeRequest{}, nil
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+repoCols(mergeRequestColumnsTpl, "")+" FROM cc_merge_requests "+
			"WHERE u_key = ? AND mr_status = ? AND mr_proceed_status = ? "+
			"ORDER BY mr_updated DESC LIMIT ?",
		uKey, domain.CommonStatusNormal, domain.MergeRequestStatusOpen, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMergeRequests(rows)
}

// TodoCounts 返回 KPI 条用的两个"总数"（不是列表长度）。
//
// 为什么单独查总数：列表被 LIMIT 截断到 5 条，若直接拿 len(list) 当 KPI，
// 有 12 条待评审时首页会显示"5"，属于明着骗人。两个 COUNT 合成一条 SQL，
// 与索引 (u_key, mrr_is_review, mr_key) / (u_key, mr_proceed_status, mr_updated) 对齐。
func (r *HomeRepo) TodoCounts(ctx context.Context, uKey string) (pendingReviews, myOpen int, err error) {
	if uKey == "" {
		return 0, 0, nil
	}
	err = r.db.QueryRowContext(ctx,
		"SELECT"+
			" (SELECT COUNT(*) FROM cc_merge_request_reviewers mrr"+
			"   JOIN cc_merge_requests mr ON mr.mr_key = mrr.mr_key"+
			"   WHERE mrr.u_key = ? AND mrr.mrr_is_review = ? AND mrr.mrr_status = ?"+
			"     AND mr.mr_status = ? AND mr.mr_proceed_status = ?),"+
			" (SELECT COUNT(*) FROM cc_merge_requests"+
			"   WHERE u_key = ? AND mr_status = ? AND mr_proceed_status = ?)",
		uKey, domain.GlobalFalse, domain.CommonStatusNormal,
		domain.CommonStatusNormal, domain.MergeRequestStatusOpen,
		uKey, domain.CommonStatusNormal, domain.MergeRequestStatusOpen,
	).Scan(&pendingReviews, &myOpen)
	if err != nil {
		return 0, 0, err
	}
	return pendingReviews, myOpen, nil
}

// RecentActivities 列出"我可见范围内"的团队动态（仓库维度 + 仓库组维度）。
//
// **为什么不写成一条带 OR 子查询的 SQL**：实测 `WHERE (r_key IN (子查询) OR
// g_key IN (子查询))` 会让 MySQL 放弃索引、对 `cc_activities` 全表扫描
// （EXPLAIN 的 type=ALL）—— 因为 OR 跨两列 + `ORDER BY a_created`，优化器无法用
// 任何一个索引同时满足过滤与排序。活动表是全站最大的一张表，这条必须拆开：
//
//	① 先取"我参与的仓库主键"与"我所在的组主键"（两次走成员表单列索引的小查询）；
//	② 分别按 (a_relative_r_key, a_created) / (a_relative_g_key, a_created) 取最近 N 条；
//	③ 在 Go 侧合并去重后取前 N 条。
//
// 去重是必须的：仓库动态同时带 g_key（仓库所属组），若该组也在我的组列表里，
// 同一条活动会在两个分支里各出现一次。
//
// **可见仓库/组的取法**：与 `RepositoryRepo.ListJoined` 完全一致（所有者 ∪ 直接成员
// ∪ 可见组下的仓库）。早先只看 `cc_repository_members`，结果"自己拥有但没有成员行"
// 的仓库的动态不会出现 —— 真实实例上就有这种情况（成员表为空、仓库靠 u_key 归属），
// 2026-10-27 修正。
func (r *HomeRepo) RecentActivities(ctx context.Context, uKey string, limit int) ([]domain.RepositoryActivity, error) {
	if uKey == "" || limit <= 0 {
		return []domain.RepositoryActivity{}, nil
	}
	rKeys, err := r.visibleRepositoryKeys(ctx, uKey)
	if err != nil {
		return nil, err
	}
	gKeys, err := r.visibleGroupKeys(ctx, uKey)
	if err != nil {
		return nil, err
	}

	const activityCols = "a_key, a_type, u_key, a_relative_g_key, a_relative_r_key," +
		" a_relative_mr_key, a_data, CAST(a_created AS CHAR), a_status"

	merged := map[string]domain.RepositoryActivity{}
	if len(rKeys) > 0 {
		placeholders, args := inPlaceholders(rKeys)
		args = append(args, domain.CommonStatusNormal, limit)
		rows, qerr := r.db.QueryContext(ctx,
			"SELECT "+activityCols+" FROM cc_activities"+
				" WHERE a_relative_r_key IN ("+strings.Join(placeholders, ", ")+") AND a_status = ?"+
				" ORDER BY a_created DESC LIMIT ?", args...)
		if qerr != nil {
			return nil, qerr
		}
		list, serr := scanActivities(rows)
		rows.Close()
		if serr != nil {
			return nil, serr
		}
		for _, act := range list {
			merged[act.Key] = act
		}
	}
	if len(gKeys) > 0 {
		placeholders, args := inPlaceholders(gKeys)
		args = append(args, domain.CommonStatusNormal, limit)
		rows, qerr := r.db.QueryContext(ctx,
			"SELECT "+activityCols+" FROM cc_activities"+
				" WHERE a_relative_g_key IN ("+strings.Join(placeholders, ", ")+") AND a_status = ?"+
				" ORDER BY a_created DESC LIMIT ?", args...)
		if qerr != nil {
			return nil, qerr
		}
		list, serr := scanActivities(rows)
		rows.Close()
		if serr != nil {
			return nil, serr
		}
		for _, act := range list {
			if _, exists := merged[act.Key]; !exists {
				merged[act.Key] = act
			}
		}
	}

	out := make([]domain.RepositoryActivity, 0, len(merged))
	for _, act := range merged {
		out = append(out, act)
	}
	// Created 是 `CAST(a_created AS CHAR)` 的 "YYYY-MM-DD HH:MM:SS"，
	// 字典序与时间序一致，可以直接比较字符串（与既有 activity 查询的排序口径一致）。
	sort.Slice(out, func(i, j int) bool { return out[i].Created > out[j].Created })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// visibleRepositoryKeys 取"我可见的仓库"主键集合。
//
// 与 `RepositoryRepo.ListJoined` 同口径（三种来源取并集）：
//
//	(1) 我是仓库所有者
//	(2) 我是该仓库的直接成员
//	(3) 该仓库属于我可见的仓库组（组的所有者是我，或我是该组成员）
//
// 只取主键（不带整行），因为调用方只需要拼 IN 列表。
func (r *HomeRepo) visibleRepositoryKeys(ctx context.Context, uKey string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.r_key FROM cc_repositories r
		WHERE r.r_status = ?
		  AND (r.u_key = ?
		       OR EXISTS (SELECT 1 FROM cc_repository_members vrm
		                   WHERE vrm.r_key = r.r_key AND vrm.u_key = ? AND vrm.rm_status = ?)
		       OR EXISTS (SELECT 1 FROM cc_groups vg
		                   WHERE vg.g_key = r.g_key AND vg.g_status = ?
		                     AND (vg.u_key = ?
		                          OR EXISTS (SELECT 1 FROM cc_group_members vgm
		                                      WHERE vgm.g_key = vg.g_key AND vgm.u_key = ? AND vgm.gm_status = ?))))`,
		domain.CommonStatusNormal,
		uKey,
		uKey, domain.CommonStatusNormal,
		domain.CommonStatusNormal, uKey,
		uKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

// visibleGroupKeys 取"我可见的仓库组"主键集合：我拥有的 **或** 我是成员的组。
func (r *HomeRepo) visibleGroupKeys(ctx context.Context, uKey string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.g_key FROM cc_groups g
		WHERE g.g_status = ?
		  AND (g.u_key = ?
		       OR EXISTS (SELECT 1 FROM cc_group_members gm
		                   WHERE gm.g_key = g.g_key AND gm.u_key = ? AND gm.gm_status = ?))`,
		domain.CommonStatusNormal, uKey, uKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

// PlatformSnapshot 取平台概览（全站口径）。
//
// 四个 COUNT 合成一条 SQL 省掉 4 次往返；都是走 `*_status` 索引的计数，
// 表再大也是索引扫描，不随业务复杂度放大。
func (r *HomeRepo) PlatformSnapshot(ctx context.Context) (domain.HomePlatform, error) {
	var out domain.HomePlatform
	err := r.db.QueryRowContext(ctx,
		"SELECT"+
			" (SELECT COUNT(*) FROM cc_repositories WHERE r_status = ?),"+
			" (SELECT COUNT(*) FROM cc_groups WHERE g_status = ?),"+
			" (SELECT COUNT(*) FROM cc_users WHERE u_status = ?),"+
			" (SELECT COUNT(*) FROM cc_merge_requests WHERE mr_status = ? AND mr_proceed_status = ?)",
		domain.CommonStatusNormal,
		domain.CommonStatusNormal,
		domain.CommonStatusNormal,
		domain.CommonStatusNormal, domain.MergeRequestStatusOpen,
	).Scan(&out.Repositories, &out.Groups, &out.Members, &out.OpenMergeRequests)
	if err != nil {
		return domain.HomePlatform{}, err
	}
	return out, nil
}
