package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"codefever-next/backend/internal/domain"
)

// RepositoryRepo 负责 repository 域的数据读取。
//
// 与 PHP 侧 application/models/repository_model.php 的差异：
// PHP 的 normalize() 对每个仓库额外查 2 次 SQL（组信息 + fork 数），
// list_get() 又对每个仓库再查 2 次（成员角色 + 成员列表），是典型的 N+1。
// 这里把可批量的查询全部改成批量形式，**输出顺序与字段完全保持一致**。
type RepositoryRepo struct {
	db *sql.DB
}

// NewRepositoryRepo 创建仓库仓储。
func NewRepositoryRepo(db *sql.DB) *RepositoryRepo { return &RepositoryRepo{db: db} }

// 时间列一律用 CAST(... AS CHAR) 取 MySQL 的原始字符串。
//
// 原因：DSN 开了 parseTime，TIMESTAMP 列会被驱动直接转成 time.Time
// （还会按连接时区做一次换算），而 PHP 侧读的是**会话时区下的原始字符串**，
// 再由 strtotime() 解释。两者的时区基准不同，直接扫描会差一个时区偏移。
// CAST 之后拿到的字符串与 PHP 侧完全一致。
const (
	repositoryColumnsTpl = "%[1]s.r_key, %[1]s.u_key, %[1]s.g_key, %[1]s.r_fork_r_key, " +
		"%[1]s.r_display_name, %[1]s.r_name, %[1]s.r_path, %[1]s.r_description, %[1]s.r_avatar, " +
		"%[1]s.r_default_branch_name, CAST(%[1]s.r_created AS CHAR), CAST(%[1]s.r_updated AS CHAR), " +
		"%[1]s.r_status"

	repositoryGroupColumnsTpl = "%[1]s.g_key, %[1]s.u_key, %[1]s.g_type, %[1]s.g_display_name, " +
		"%[1]s.g_name, %[1]s.g_description, %[1]s.g_avatar, " +
		"CAST(%[1]s.g_created AS CHAR), CAST(%[1]s.g_updated AS CHAR), %[1]s.g_status"

	mergeRequestColumnsTpl = "%[1]s.mr_key, %[1]s.u_key, %[1]s.mr_merge_u_key, " +
		"%[1]s.mr_source_merged_hash, %[1]s.mr_source_r_key, %[1]s.mr_source_branch, " +
		"%[1]s.mr_target_merged_hash, %[1]s.mr_target_r_key, %[1]s.mr_target_branch, " +
		"%[1]s.mr_start_hash, %[1]s.mr_id, %[1]s.mr_title, %[1]s.mr_description, " +
		"%[1]s.mr_proceed_status, CAST(%[1]s.mr_created AS CHAR), " +
		"CAST(%[1]s.mr_merge_time AS CHAR), CAST(%[1]s.mr_close_time AS CHAR), " +
		"CAST(%[1]s.mr_updated AS CHAR), %[1]s.mr_status"
)

// repoCols 生成列清单；table 为空时不加表前缀。
func repoCols(tpl, table string) string {
	if table == "" {
		return strings.ReplaceAll(tpl, "%[1]s.", "")
	}
	return fmt.Sprintf(tpl, table)
}

// ---------------------------------------------------------------------------
// 仓库
// ---------------------------------------------------------------------------

// Get 读取一个仓库。
//
// 逐字对应 application/models/repository_model.php:409-425 的 get()：
// 只取 r_status = COMMON_STATUS_NORMAL 的行。
func (r *RepositoryRepo) Get(ctx context.Context, rKey string) (*domain.Repository, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+repoCols(repositoryColumnsTpl, "")+" FROM cc_repositories WHERE r_key = ? AND r_status = ?",
		rKey, domain.CommonStatusNormal)
	return scanRepository(row)
}

// RepositoryPath 返回仓库在 git-storage 下的相对路径（r_path）。
//
// 逐字对应 cc_repositories.r_path，形如 "/de7347a84f6236f06b0356ce6668c27d"。
// 绝对路径由 gitsvc.Service.RepoPath() 负责拼接并做路径穿越校验。
func (r *RepositoryRepo) RepositoryPath(ctx context.Context, rKey string) (string, error) {
	var p sql.NullString
	err := r.db.QueryRowContext(ctx, "SELECT r_path FROM cc_repositories WHERE r_key = ?", rKey).Scan(&p)
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

// ListDefaultBranches 列出全部正常仓库的默认分支与裸仓库相对路径。
//
// 启动时用它修复"HEAD 指向 master、但实际分支是 main"的历史仓库。
type RepositoryDefaultBranch struct {
	Key    string
	Path   string
	Branch string
}

// ListDefaultBranches 返回默认分支修复任务清单。
func (r *RepositoryRepo) ListDefaultBranches(ctx context.Context) ([]RepositoryDefaultBranch, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r_key, r_path, COALESCE(r_default_branch_name, '')
		FROM cc_repositories
		WHERE r_status = ? AND r_path IS NOT NULL AND r_path <> ''`,
		domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RepositoryDefaultBranch{}
	for rows.Next() {
		var item RepositoryDefaultBranch
		if err := rows.Scan(&item.Key, &item.Path, &item.Branch); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListJoined 列出用户可见的全部仓库。
//
// 逐字对应 application/models/repository_model.php:462-491 的 listJoined()：
//  1. 用户自己拥有的组 + 用户加入的组；
//  2. 这些组下的仓库；
//  3. 用户作为仓库成员直接加入的仓库；
//  4. 用户自己拥有的仓库；
//  5. 按 r_key 去重 —— **保留首次出现的位置，取最后一次出现的值**
//     （PHP 的 Helper::getUniqueItemList 语义）。
func (r *RepositoryRepo) ListJoined(ctx context.Context, uKey string) ([]domain.Repository, error) {
	if uKey == "" {
		return []domain.Repository{}, nil
	}

	groups, err := r.groupsOfUser(ctx, uKey)
	if err != nil {
		return nil, err
	}

	var collected []domain.Repository

	// (1)(2) 组下的仓库：按组顺序、组内按 r_key 排序，
	// 与 PHP 逐个 listInGroup() 的自然扫描顺序一致。
	//
	// 性能整改 D4/B15：排序改在 Go 侧做，SQL 里不再用 ORDER BY FIELD(...)。
	//
	// 原先的 `ORDER BY FIELD(g_key, ?,?,…), r_key` 有两个问题：
	//   1. FIELD() 是逐行函数，优化器无法用索引满足排序，必走 filesort；
	//   2. 它需要**第二份**与 IN 列表等长的占位符与参数（同一组 key 传两遍）。
	// 而这里的行数很小（一个用户可见的仓库数），在内存里排序更划算，
	// 也顺带让占位符只出现一次。
	if len(groups) > 0 {
		keys := make([]string, 0, len(groups))
		groupOrder := make(map[string]int, len(groups))
		for i, g := range groups {
			if _, ok := groupOrder[g.Key]; !ok {
				groupOrder[g.Key] = i
			}
			keys = append(keys, g.Key)
		}
		placeholders, args := inPlaceholders(keys)
		args = append(args, domain.CommonStatusNormal)

		rows, qerr := r.db.QueryContext(ctx,
			"SELECT "+repoCols(repositoryColumnsTpl, "")+" FROM cc_repositories WHERE g_key IN ("+
				strings.Join(placeholders, ", ")+") AND r_status = ?", args...)
		if qerr != nil {
			return nil, qerr
		}
		list, serr := scanRepositories(rows)
		if serr != nil {
			return nil, serr
		}
		// 复刻原 ORDER BY FIELD(g_key, …), r_key 的顺序：
		// 先按"组在 groups 中首次出现的位置"，再按仓库主键。
		sort.SliceStable(list, func(a, b int) bool {
			ga, gb := groupOrder[list[a].GroupKey], groupOrder[list[b].GroupKey]
			if ga != gb {
				return ga < gb
			}
			return list[a].Key < list[b].Key
		})
		collected = append(collected, list...)
	}

	// (3) 作为仓库成员直接加入的仓库。
	direct, err := r.db.QueryContext(ctx,
		"SELECT "+repoCols(repositoryColumnsTpl, "cc_repositories")+
			" FROM cc_repositories"+
			" LEFT JOIN cc_repository_members ON cc_repository_members.r_key = cc_repositories.r_key"+
			" WHERE cc_repository_members.u_key = ? AND cc_repository_members.rm_status = ?"+
			" AND cc_repositories.r_status = ?", uKey, domain.CommonStatusNormal, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	list, err := scanRepositories(direct)
	if err != nil {
		return nil, err
	}
	collected = append(collected, list...)

	// (4) 自己拥有的仓库。
	owned, err := r.db.QueryContext(ctx,
		"SELECT "+repoCols(repositoryColumnsTpl, "")+" FROM cc_repositories WHERE u_key = ? AND r_status = ?",
		uKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	list, err = scanRepositories(owned)
	if err != nil {
		return nil, err
	}
	collected = append(collected, list...)

	return uniqueRepositories(collected), nil
}

// uniqueRepositories 按 r_key 去重：保留首次出现的位置，取最后一次出现的值。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:49-57
// 的 getUniqueItemList()。
func uniqueRepositories(list []domain.Repository) []domain.Repository {
	index := map[string]int{}
	out := make([]domain.Repository, 0, len(list))
	for _, item := range list {
		if i, ok := index[item.Key]; ok {
			out[i] = item
			continue
		}
		index[item.Key] = len(out)
		out = append(out, item)
	}
	return out
}

// CountJoinedByUsers 批量统计每个用户"可见仓库"的数量。
//
// 性能整改 C2（2026-10）。
//
// ## 为什么需要它
//
// 管理端用户列表（POST /api/admin/userList）原先对**每个用户**调用一次
// ListJoined，而 ListJoined 展开成 5 条 SQL（自有组、加入组、组下仓库、
// 成员仓库、自有仓库）。默认每页 20 个用户 ⇒ 2 + 20×5 = 102 条查询，
// 且被查询的行数随用户仓库数线性增长。
//
// 更关键的是：userList 只用到 `len(projects)`（一个数字），
// 却把每个用户可见的**全部仓库行**都物化回进程并去重 —— 纯粹浪费。
//
// ## 口径必须与 ListJoined 完全一致
//
// ListJoined 取四个来源的并集，再按 r_key 去重后计数（见其函数注释）：
//
//	(1) 我拥有的正常组(g_status=1)下的正常仓库(r_status=1)
//	(2) 我作为成员(gm_status=1)加入的正常组下的正常仓库
//	(3) 我作为成员(rm_status=1)直接加入的正常仓库
//	(4) 我拥有的正常仓库
//
// 这里用一条 UNION ALL + COUNT(DISTINCT r_key) 复刻该并集，
// 因此等价于 len(ListJoined(uKey))。
//
// ## 复杂度
//
// 无论一页有多少用户，都只有 **1 条 SQL**（原先 5×N 条）。
func (r *RepositoryRepo) CountJoinedByUsers(ctx context.Context, uKeys []string) (map[string]int, error) {
	out := make(map[string]int, len(uKeys))
	if len(uKeys) == 0 {
		return out, nil
	}

	placeholders, keyArgs := inPlaceholders(uKeys)
	in := strings.Join(placeholders, ", ")

	// 参数必须严格按 SQL 中 ? 的出现顺序拼接，因此逐段累加而不是先拼 SQL 再补参。
	args := make([]any, 0, len(uKeys)*4+7)
	normal := domain.CommonStatusNormal

	args = append(args, normal, normal)
	args = append(args, keyArgs...)

	args = append(args, normal, normal, normal)
	args = append(args, keyArgs...)

	args = append(args, normal, normal)
	args = append(args, keyArgs...)

	args = append(args, normal)
	args = append(args, keyArgs...)

	query := `
		SELECT t.u_key, COUNT(DISTINCT t.r_key) FROM (
			SELECT g.u_key AS u_key, r.r_key AS r_key
			  FROM cc_repositories r
			  JOIN cc_groups g ON g.g_key = r.g_key
			 WHERE r.r_status = ? AND g.g_status = ? AND g.u_key IN (` + in + `)
			UNION ALL
			SELECT gm.u_key, r.r_key
			  FROM cc_repositories r
			  JOIN cc_groups g ON g.g_key = r.g_key
			  JOIN cc_group_members gm ON gm.g_key = g.g_key
			 WHERE r.r_status = ? AND g.g_status = ? AND gm.gm_status = ? AND gm.u_key IN (` + in + `)
			UNION ALL
			SELECT rm.u_key, r.r_key
			  FROM cc_repositories r
			  JOIN cc_repository_members rm ON rm.r_key = r.r_key
			 WHERE r.r_status = ? AND rm.rm_status = ? AND rm.u_key IN (` + in + `)
			UNION ALL
			SELECT r.u_key, r.r_key
			  FROM cc_repositories r
			 WHERE r.r_status = ? AND r.u_key IN (` + in + `)
		) t
		GROUP BY t.u_key`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var uKey string
		var n int
		if err := rows.Scan(&uKey, &n); err != nil {
			return nil, err
		}
		out[uKey] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 没有任何可见仓库的用户不会出现在结果集中，补 0 以免调用方区分「不存在」与「0」。
	for _, k := range uKeys {
		if _, ok := out[k]; !ok {
			out[k] = 0
		}
	}
	return out, nil
}

// groupsOfUser 返回用户自己拥有的组 + 用户加入的组。
//
// 逐字对应 application/models/group_model.php:117-134 的 list() 与 listJoined()。
func (r *RepositoryRepo) groupsOfUser(ctx context.Context, uKey string) ([]domain.RepositoryGroup, error) {
	owned, err := r.db.QueryContext(ctx,
		"SELECT "+repoCols(repositoryGroupColumnsTpl, "")+" FROM cc_groups WHERE u_key = ? AND g_status = ?",
		uKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	ownedList, err := scanRepositoryGroups(owned)
	if err != nil {
		return nil, err
	}

	joined, err := r.db.QueryContext(ctx,
		"SELECT "+repoCols(repositoryGroupColumnsTpl, "cc_groups")+
			" FROM cc_groups"+
			" LEFT JOIN cc_group_members ON cc_group_members.g_key = cc_groups.g_key"+
			" WHERE cc_group_members.u_key = ? AND cc_group_members.gm_status = ?"+
			" AND cc_groups.g_status = ?", uKey, domain.CommonStatusNormal, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	joinedList, err := scanRepositoryGroups(joined)
	if err != nil {
		return nil, err
	}

	return append(ownedList, joinedList...), nil
}

// GroupsByKeys 批量读取仓库组。
//
// 用于消除 normalize() 里"每个仓库查一次组"的 N+1。
func (r *RepositoryRepo) GroupsByKeys(ctx context.Context, gKeys []string) (map[string]*domain.RepositoryGroup, error) {
	out := map[string]*domain.RepositoryGroup{}
	if len(gKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(gKeys)
	rows, err := r.db.QueryContext(ctx,
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
func (r *RepositoryRepo) ForkCounts(ctx context.Context, rKeys []string) (map[string]int, error) {
	out := map[string]int{}
	if len(rKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(rKeys)
	args = append(args, domain.CommonStatusNormal)
	rows, err := r.db.QueryContext(ctx,
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
func (r *RepositoryRepo) MergeRequestCounts(ctx context.Context, rKeys []string) (map[string]domain.RepositoryMergeRequestCount, error) {
	out := map[string]domain.RepositoryMergeRequestCount{}
	if len(rKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(rKeys)
	args = append(args, domain.CommonStatusNormal)
	rows, err := r.db.QueryContext(ctx,
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

// ---------------------------------------------------------------------------
// 成员
// ---------------------------------------------------------------------------

// MembersOf 批量读取仓库成员。
//
// 逐字对应 application/models/repository_model.php:427-436 的 listMembers()：
// 只按 r_key 过滤，不带状态条件（已删除的成员也要出现，靠 deleteFlag 表达）。
func (r *RepositoryRepo) MembersOf(ctx context.Context, rKeys []string) (map[string][]domain.RepositoryMember, error) {
	out := map[string][]domain.RepositoryMember{}
	if len(rKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(rKeys)
	rows, err := r.db.QueryContext(ctx,
		"SELECT rm_key, r_key, u_key, rm_role, CAST(rm_created AS CHAR), rm_status FROM cc_repository_members"+
			" WHERE r_key IN ("+strings.Join(placeholders, ", ")+") ORDER BY rm_key", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m domain.RepositoryMember
		if err := rows.Scan(&m.Key, &m.RKey, &m.UKey, &m.Role, &m.Created, &m.Status); err != nil {
			return nil, err
		}
		out[m.RKey] = append(out[m.RKey], m)
	}
	return out, rows.Err()
}

// GroupMembersOf 批量读取仓库组成员。
//
// 逐字对应 application/models/group_model.php:110-115 的 listMembers()。
func (r *RepositoryRepo) GroupMembersOf(ctx context.Context, gKeys []string) (map[string][]domain.RepositoryGroupMember, error) {
	out := map[string][]domain.RepositoryGroupMember{}
	if len(gKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(gKeys)
	rows, err := r.db.QueryContext(ctx,
		"SELECT gm_key, g_key, u_key, gm_role, CAST(gm_created AS CHAR), gm_status FROM cc_group_members"+
			" WHERE g_key IN ("+strings.Join(placeholders, ", ")+") ORDER BY gm_key", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m domain.RepositoryGroupMember
		if err := rows.Scan(&m.Key, &m.GKey, &m.UKey, &m.Role, &m.Created, &m.Status); err != nil {
			return nil, err
		}
		out[m.GKey] = append(out[m.GKey], m)
	}
	return out, rows.Err()
}

// UsersByKeys 批量读取用户。
func (r *RepositoryRepo) UsersByKeys(ctx context.Context, uKeys []string) (map[string]*domain.User, error) {
	out := map[string]*domain.User{}
	if len(uKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(uKeys)
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+userColumns+" FROM cc_users WHERE u_key IN ("+strings.Join(placeholders, ", ")+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		u, serr := scanUser(rows)
		if serr != nil {
			return nil, serr
		}
		out[u.Key] = u
	}
	return out, rows.Err()
}

// CommitEmailsByUser 批量读取用户的已验证提交邮箱。
//
// 逐字对应 application/models/user_model.php:153-173 的 getCommitEmails()：
// 只取 ce_status 正常且 ce_is_check 为真的行，按 ce_created 倒序。
// 主邮箱由 domain.NormalizeUser 补到最前面。
func (r *RepositoryRepo) CommitEmailsByUser(ctx context.Context, uKeys []string) (map[string][]domain.CommitEmail, error) {
	out := map[string][]domain.CommitEmail{}
	if len(uKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(uKeys)
	args = append(args, domain.CommonStatusNormal)
	rows, err := r.db.QueryContext(ctx,
		"SELECT ce_key, u_key, ce_email, ce_is_check, ce_status, ce_created FROM cc_commit_emails"+
			" WHERE u_key IN ("+strings.Join(placeholders, ", ")+") AND ce_status = ?"+
			" AND ce_is_check = 1 ORDER BY ce_created DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e domain.CommitEmail
		var isCheck int
		if err := rows.Scan(&e.Key, &e.UKey, &e.Email, &isCheck, &e.Status, &e.Created); err != nil {
			return nil, err
		}
		e.IsCheck = isCheck == 1
		out[e.UKey] = append(out[e.UKey], e)
	}
	return out, rows.Err()
}

// RolesFor 批量解析用户在多个仓库上的角色。
//
// 逐字对应 application/models/repository_model.php:727-757 的 getMemberRole()：
// 仓库 owner → ROLE_OWNER；否则看 cc_repository_members.rm_role；
// 再否则回落到仓库所属组的角色（组 owner → ROLE_OWNER，否则 gm_role）。
func (r *RepositoryRepo) RolesFor(ctx context.Context, rKeys []string, uKey string) (map[string]int, error) {
	out := map[string]int{}
	if len(rKeys) == 0 || uKey == "" {
		return out, nil
	}
	placeholders, args := inPlaceholders(rKeys)

	// 仓库属主。
	ownerRows, err := r.db.QueryContext(ctx,
		"SELECT r_key, u_key FROM cc_repositories WHERE r_key IN ("+strings.Join(placeholders, ", ")+")",
		args...)
	if err != nil {
		return nil, err
	}
	owners := map[string]string{}
	for ownerRows.Next() {
		var key, owner string
		if err := ownerRows.Scan(&key, &owner); err != nil {
			ownerRows.Close()
			return nil, err
		}
		owners[key] = owner
	}
	ownerRows.Close()

	// 仓库成员角色。
	memberArgs := append([]any{uKey, domain.CommonStatusNormal}, args...)
	memberRows, err := r.db.QueryContext(ctx,
		"SELECT r_key, rm_role FROM cc_repository_members WHERE u_key = ? AND rm_status = ?"+
			" AND r_key IN ("+strings.Join(placeholders, ", ")+")", memberArgs...)
	if err != nil {
		return nil, err
	}
	memberRoles := map[string]int{}
	for memberRows.Next() {
		var key string
		var role int
		if err := memberRows.Scan(&key, &role); err != nil {
			memberRows.Close()
			return nil, err
		}
		memberRoles[key] = role
	}
	memberRows.Close()

	// 组角色（组属主与组成员）。
	groupRows, err := r.db.QueryContext(ctx,
		"SELECT DISTINCT g.g_key, g.u_key FROM cc_groups g"+
			" JOIN cc_repositories r ON r.g_key = g.g_key"+
			" WHERE r.r_key IN ("+strings.Join(placeholders, ", ")+")", args...)
	if err != nil {
		return nil, err
	}
	groupOwners := map[string]string{}
	for groupRows.Next() {
		var key, owner string
		if err := groupRows.Scan(&key, &owner); err != nil {
			groupRows.Close()
			return nil, err
		}
		groupOwners[key] = owner
	}
	groupRows.Close()

	gmArgs := append([]any{uKey, domain.CommonStatusNormal}, args...)
	gmRows, err := r.db.QueryContext(ctx,
		"SELECT gm.g_key, gm.gm_role FROM cc_group_members gm"+
			" JOIN cc_repositories r ON r.g_key = gm.g_key"+
			" WHERE gm.u_key = ? AND gm.gm_status = ?"+
			" AND r.r_key IN ("+strings.Join(placeholders, ", ")+")", gmArgs...)
	if err != nil {
		return nil, err
	}
	groupRoles := map[string]int{}
	for gmRows.Next() {
		var key string
		var role int
		if err := gmRows.Scan(&key, &role); err != nil {
			gmRows.Close()
			return nil, err
		}
		groupRoles[key] = role
	}
	gmRows.Close()

	// 仓库 → 所属组。
	relRows, err := r.db.QueryContext(ctx,
		"SELECT r_key, g_key FROM cc_repositories WHERE r_key IN ("+strings.Join(placeholders, ", ")+")",
		args...)
	if err != nil {
		return nil, err
	}
	repoGroups := map[string]string{}
	for relRows.Next() {
		var rKey, gKey string
		if err := relRows.Scan(&rKey, &gKey); err != nil {
			relRows.Close()
			return nil, err
		}
		repoGroups[rKey] = gKey
	}
	relRows.Close()

	for _, rKey := range rKeys {
		switch {
		case owners[rKey] == uKey:
			out[rKey] = int(domain.RoleOwner)
		case memberRoles[rKey] != 0:
			out[rKey] = memberRoles[rKey]
		default:
			gKey := repoGroups[rKey]
			switch {
			case gKey == "":
				out[rKey] = int(domain.RoleNoPermission)
			case groupOwners[gKey] == uKey:
				out[rKey] = int(domain.RoleOwner)
			default:
				out[rKey] = groupRoles[gKey]
			}
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 活动
// ---------------------------------------------------------------------------

// ListActivities 列出仓库的活动流。
//
// 逐字对应 application/models/repository_model.php:787-815 的 listActivities()：
// category 决定 a_type 的过滤区间，keyword 对 a_data 做 LIKE，
// 按 a_created 倒序并分页。
func (r *RepositoryRepo) ListActivities(ctx context.Context, rKey string, category int, keyword string, page, pagesize int) ([]domain.RepositoryActivity, error) {
	where := " WHERE a_relative_r_key = ? AND a_status = ?"
	args := []any{rKey, domain.CommonStatusNormal}

	switch category {
	case domain.ActivityCategoryCommit:
		where += " AND a_type IN (0x0401, 0x0402, 0x0501, 0x0502)"
	case domain.ActivityCategoryMergeRequest:
		where += " AND (a_type > 0x0700 AND a_type < 0x0800)"
	case domain.ActivityCategoryMember:
		where += " AND (a_type < 0x0600 AND a_type != 0x0401 AND a_type != 0x0402" +
			" AND a_type != 0x0501 AND a_type != 0x0502)"
	}
	if keyword != "" {
		// 性能整改 D3：转义 LIKE 通配符。
		// 整改前直接把用户输入拼进 "%…%"，`?keyword=%` 会变成裸通配符，
		// 把本来已经按 a_relative_r_key 收窄的查询重新退化成全表扫描；
		// `_` 同理。同包的其它检索（repo/group.go、repo/admin.go）一直用
		// escapeLike，这两处是漏网。
		where += " AND a_data LIKE ?"
		args = append(args, "%"+escapeLike(keyword)+"%")
	}
	// 性能整改 C1：此处是最后一道防线。handler 侧已用 domain.ClampPaging 收敛，
	// 这里再封一次顶，避免将来新增调用方绕过 handler 直接把用户输入透传进来。
	page, pagesize = domain.ClampPaging(page, pagesize)
	args = append(args, pagesize, (page-1)*pagesize)

	rows, err := r.db.QueryContext(ctx,
		"SELECT a_key, a_type, u_key, a_relative_g_key, a_relative_r_key, a_relative_mr_key,"+
			" a_data, CAST(a_created AS CHAR), a_status FROM cc_activities"+where+
			" ORDER BY a_created DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	return scanActivities(rows)
}

// ListMergeRequestActivities 列出某个合并请求的活动流。
//
// 逐字对应 application/models/repository_model.php:817-830 的 listMergeRequestActivities()。
func (r *RepositoryRepo) ListMergeRequestActivities(ctx context.Context, rKey, mrKey string) ([]domain.RepositoryActivity, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT a_key, a_type, u_key, a_relative_g_key, a_relative_r_key, a_relative_mr_key,"+
			" a_data, CAST(a_created AS CHAR), a_status FROM cc_activities"+
			" WHERE a_relative_r_key = ? AND a_relative_mr_key = ? AND a_status = ?"+
			" ORDER BY a_created DESC", rKey, mrKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	return scanActivities(rows)
}

// ---------------------------------------------------------------------------
// 合并请求
// ---------------------------------------------------------------------------

// MergeRequests 列出合并请求。
//
// 逐字对应 application/models/repository_model.php:1288-1325 的 getMergeRequests()：
// 目标仓库集合、进行状态、正常状态与标题/描述关键字过滤，按 mr_updated 排序分页。
func (r *RepositoryRepo) MergeRequests(ctx context.Context, targetRKeys []string, status int, keyword, sort string, page, pagesize int) ([]domain.RepositoryMergeRequest, error) {
	where := " WHERE mr_status = ?"
	args := []any{domain.CommonStatusNormal}

	if len(targetRKeys) > 0 {
		placeholders, pArgs := inPlaceholders(targetRKeys)
		where += " AND mr_target_r_key IN (" + strings.Join(placeholders, ", ") + ")"
		args = append(args, pArgs...)
	}
	if status != domain.MergeRequestStatusNone {
		where += " AND mr_proceed_status = ?"
		args = append(args, status)
	}
	if keyword != "" {
		// 性能整改 D3：转义 LIKE 通配符（理由同上，见本文件活动流分支的注释）。
		where += " AND (mr_title LIKE ? OR mr_description LIKE ?)"
		kw := "%" + escapeLike(keyword) + "%"
		args = append(args, kw, kw)
	}
	// 性能整改 C1：此处是最后一道防线。handler 侧已用 domain.ClampPaging 收敛，
	// 这里再封一次顶，避免将来新增调用方绕过 handler 直接把用户输入透传进来。
	page, pagesize = domain.ClampPaging(page, pagesize)
	if sort != "asc" {
		sort = "desc"
	}
	args = append(args, pagesize, (page-1)*pagesize)

	rows, err := r.db.QueryContext(ctx,
		"SELECT "+repoCols(mergeRequestColumnsTpl, "")+" FROM cc_merge_requests"+where+
			" ORDER BY mr_updated "+sort+" LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	return scanMergeRequests(rows)
}

// MergeRequestByKey 按主键读取合并请求。
//
// 逐字对应 application/models/repository_model.php:1180-1193 的 getMergeRequestByKey()。
func (r *RepositoryRepo) MergeRequestByKey(ctx context.Context, mrKey string) (*domain.RepositoryMergeRequest, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+repoCols(mergeRequestColumnsTpl, "")+" FROM cc_merge_requests WHERE mr_key = ? AND mr_status = ?",
		mrKey, domain.CommonStatusNormal)
	return scanMergeRequest(row)
}

// MergeRequestByIDAndKey 按业务编号与目标仓库读取合并请求。
//
// 逐字对应 application/models/repository_model.php:399-407 的 getMergeRequestByIdAndKey()。
func (r *RepositoryRepo) MergeRequestByIDAndKey(ctx context.Context, mID int, rKey string) (*domain.RepositoryMergeRequest, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+repoCols(mergeRequestColumnsTpl, "")+" FROM cc_merge_requests WHERE mr_id = ? AND mr_target_r_key = ?",
		mID, rKey)
	return scanMergeRequest(row)
}

// MergeRequestReviewers 批量读取合并请求的评审人。
//
// 逐字对应 application/models/repository_model.php:351-364 的 getMergeRequestReviewers()。
func (r *RepositoryRepo) MergeRequestReviewers(ctx context.Context, mrKeys []string) (map[string][]domain.RepositoryMergeRequestReviewer, error) {
	out := map[string][]domain.RepositoryMergeRequestReviewer{}
	if len(mrKeys) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(mrKeys)
	rows, err := r.db.QueryContext(ctx,
		"SELECT mrr_key, mr_key, u_key, mrr_is_review FROM cc_merge_request_reviewers"+
			" WHERE mr_key IN ("+strings.Join(placeholders, ", ")+") ORDER BY mrr_key", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rv domain.RepositoryMergeRequestReviewer
		var isReview int
		if err := rows.Scan(&rv.Key, &rv.MRKey, &rv.UKey, &isReview); err != nil {
			return nil, err
		}
		rv.IsReview = isReview == 1
		out[rv.MRKey] = append(out[rv.MRKey], rv)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// 保护分支规则
// ---------------------------------------------------------------------------

// ProtectedBranchRules 列出仓库的保护分支规则。
//
// 逐字对应 application/models/repository_model.php:832-844 的 listProtectedBranchRules()。
func (r *RepositoryRepo) ProtectedBranchRules(ctx context.Context, rKey string) ([]domain.ProtectedBranchRule, error) {
	rows, err := r.db.QueryContext(ctx,
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

// ---------------------------------------------------------------------------
// 扫描与工具
// ---------------------------------------------------------------------------

// scanRepository 扫描一行仓库记录。
func scanRepository(row scanner) (*domain.Repository, error) {
	var repo domain.Repository
	var forkFrom, avatar, defaultBranch sql.NullString
	err := row.Scan(&repo.Key, &repo.OwnerKey, &repo.GroupKey, &forkFrom, &repo.DisplayName,
		&repo.Name, &repo.Path, &repo.Description, &avatar, &defaultBranch,
		&repo.Created, &repo.Updated, &repo.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	repo.ForkFromKey = nullToPtr(forkFrom)
	repo.Avatar = nullToPtr(avatar)
	repo.DefaultBranch = nullToPtr(defaultBranch)
	return &repo, nil
}

// scanRepositories 扫描多行仓库记录。
func scanRepositories(rows *sql.Rows) ([]domain.Repository, error) {
	out := []domain.Repository{}
	for rows.Next() {
		var repo domain.Repository
		var forkFrom, avatar, defaultBranch sql.NullString
		if err := rows.Scan(&repo.Key, &repo.OwnerKey, &repo.GroupKey, &forkFrom, &repo.DisplayName,
			&repo.Name, &repo.Path, &repo.Description, &avatar, &defaultBranch,
			&repo.Created, &repo.Updated, &repo.Status); err != nil {
			return nil, err
		}
		repo.ForkFromKey = nullToPtr(forkFrom)
		repo.Avatar = nullToPtr(avatar)
		repo.DefaultBranch = nullToPtr(defaultBranch)
		out = append(out, repo)
	}
	return out, rows.Err()
}

// scanRepositoryGroups 扫描多行仓库组记录。
func scanRepositoryGroups(rows *sql.Rows) ([]domain.RepositoryGroup, error) {
	out := []domain.RepositoryGroup{}
	for rows.Next() {
		var g domain.RepositoryGroup
		var avatar sql.NullString
		if err := rows.Scan(&g.Key, &g.OwnerKey, &g.Type, &g.DisplayName, &g.Name,
			&g.Description, &avatar, &g.Created, &g.Updated, &g.Status); err != nil {
			return nil, err
		}
		g.Avatar = nullToPtr(avatar)
		out = append(out, g)
	}
	return out, rows.Err()
}

// scanActivities 扫描多行活动记录。
func scanActivities(rows *sql.Rows) ([]domain.RepositoryActivity, error) {
	out := []domain.RepositoryActivity{}
	for rows.Next() {
		var a domain.RepositoryActivity
		var gKey, rKey, mrKey, data sql.NullString
		if err := rows.Scan(&a.Key, &a.Type, &a.UKey, &gKey, &rKey, &mrKey,
			&data, &a.Created, &a.Status); err != nil {
			return nil, err
		}
		a.GroupKey = nullToPtr(gKey)
		a.Repository = nullToPtr(rKey)
		a.MergeReq = nullToPtr(mrKey)
		a.Data = nullToPtr(data)
		out = append(out, a)
	}
	return out, rows.Err()
}

// scanMergeRequest 扫描一行合并请求记录。
func scanMergeRequest(row scanner) (*domain.RepositoryMergeRequest, error) {
	var mr domain.RepositoryMergeRequest
	var mergeUKey, sourceHash, targetHash, description sql.NullString
	err := row.Scan(&mr.Key, &mr.UKey, &mergeUKey, &sourceHash, &mr.SourceRKey, &mr.SourceBranch,
		&targetHash, &mr.TargetRKey, &mr.TargetBranch, &mr.StartHash, &mr.ID, &mr.Title,
		&description, &mr.ProceedStatus, &mr.Created, &mr.MergeTime, &mr.CloseTime,
		&mr.Updated, &mr.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	mr.MergeUKey = nullToPtr(mergeUKey)
	mr.SourceHash = nullToPtr(sourceHash)
	mr.TargetHash = nullToPtr(targetHash)
	mr.Description = nullToPtr(description)
	return &mr, nil
}

// scanMergeRequests 扫描多行合并请求记录。
func scanMergeRequests(rows *sql.Rows) ([]domain.RepositoryMergeRequest, error) {
	out := []domain.RepositoryMergeRequest{}
	for rows.Next() {
		var mr domain.RepositoryMergeRequest
		var mergeUKey, sourceHash, targetHash, description sql.NullString
		if err := rows.Scan(&mr.Key, &mr.UKey, &mergeUKey, &sourceHash, &mr.SourceRKey, &mr.SourceBranch,
			&targetHash, &mr.TargetRKey, &mr.TargetBranch, &mr.StartHash, &mr.ID, &mr.Title,
			&description, &mr.ProceedStatus, &mr.Created, &mr.MergeTime, &mr.CloseTime,
			&mr.Updated, &mr.Status); err != nil {
			return nil, err
		}
		mr.MergeUKey = nullToPtr(mergeUKey)
		mr.SourceHash = nullToPtr(sourceHash)
		mr.TargetHash = nullToPtr(targetHash)
		mr.Description = nullToPtr(description)
		out = append(out, mr)
	}
	return out, rows.Err()
}

// inPlaceholders 生成 IN (?, ?, ...) 的占位符与参数。
//
// 性能整改 C4（2026-10）：对入参去重后再生成占位符。
//
// 原因：调用方普遍是这样攒 keys 的 ——
//
//	keys = append(keys, repo.OwnerKey)          // 仓库 owner
//	keys = append(keys, group.OwnerKey)         // 组 owner
//	for _, m := range members { keys = append(keys, m.UserKey) }   // 全部成员
//
// 同一个用户既可能是仓库 owner、又出现在组 owner、还出现在多个仓库的成员列表里，
// 于是同一个 u_key 会被重复压入多次。仓库列表接口（handlers/repository.go 的
// normalize）在 200 个仓库的规模下能攒出数百个重复占位符：
// 既让 SQL 文本变长、也白白多传参数。
//
// 去重发生在**占位符与参数同步**的位置，因此对所有调用方都是安全的：
// IN 语义与重复值无关。注意本函数不保证顺序稳定之外的任何语义 ——
// 需要依赖位置（如 ORDER BY FIELD(...)）的调用方必须自己构建另一份占位符，
// 不能复用本函数的返回值（ListJoined 就是这么做的）。
func inPlaceholders(keys []string) ([]string, []any) {
	placeholders := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, k)
	}
	return placeholders, args
}
