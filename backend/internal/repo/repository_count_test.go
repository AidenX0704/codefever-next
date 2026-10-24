package repo

import (
	"context"
	"database/sql"
	"testing"
)

// TestCountJoinedByUsersMatchesListJoined 验证性能整改 C2 的核心前提：
// 批量计数 CountJoinedByUsers 与逐用户 ListJoined 的**语义完全等价**。
//
// 为什么必须用集成测试守这条等价性：
// admin userList 原先对每个用户调一次 ListJoined（展开 5 条 SQL）再取 len()，
// 整改后改为一条 UNION ALL + COUNT(DISTINCT r_key)。两者口径一旦分叉，
// 管理端显示的"项目数"就会静默出错 —— 这是纯 SQL 逻辑，单元测试抓不到。
//
// 覆盖的边界：
//   - 自有组下的仓库（来源 1）
//   - 作为成员加入的组下的仓库（来源 2）
//   - 直接作为仓库成员（来源 3）
//   - 自有仓库（来源 4）
//   - **软删除过滤**：g_status/r_status/gm_status/rm_status 任一非正常都要排除
//   - **跨来源去重**：同一仓库既在自有组下又自己拥有，只能算 1 个
//   - **无归属仓库**：g_key 为空串的仓库不能因 JOIN 而丢失或误算
//   - **完全无仓库的用户**：必须得到 0，而不是缺失
//
// 需要真实 MySQL（与被测 SQL 的 JOIN/去重行为有关，无法用 mock 替代）：
//
//	$env:CODEFEVER_TEST_DSN = "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true"
//	go test ./internal/repo/ -run TestCountJoinedByUsersMatchesListJoined -v
func TestCountJoinedByUsersMatchesListJoined(t *testing.T) {
	dsn := testDSN()
	if dsn == "" {
		t.Skip("未设置 CODEFEVER_TEST_DSN，跳过需要真实 MySQL 的集成用例")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	r := NewRepositoryRepo(db)

	// 所有测试数据都带该前缀，便于清理且不会与真实数据冲突。
	const p = "cfperfc2_"
	users := []string{p + "u1", p + "u2", p + "u3", p + "u4", p + "u5"}

	groups := []struct {
		key, owner string
		status     int
	}{
		{p + "g1", users[0], 1}, // U1 拥有的正常组
		{p + "g2", users[2], 1}, // U3 拥有的正常组
		{p + "g3", users[0], 3}, // U1 拥有的已删除组 —— 其下仓库必须被排除
	}
	repos := []struct {
		key, owner, group string
		status            int
	}{
		{p + "r1", users[0], p + "g1", 1}, // 组 g1
		{p + "r2", users[0], p + "g1", 1}, // 组 g1
		{p + "r3", users[2], p + "g2", 1}, // 组 g2
		{p + "r4", users[0], p + "g3", 1}, // 已删除组下的仓库 —— 排除
		{p + "r5", users[1], "", 1},       // 无归属组的自有仓库
		{p + "r6", users[1], "", 3},       // 已删除的自有仓库 —— 排除
		{p + "r7", users[0], p + "g1", 1}, // 既在 g1 下、又被 U1 拥有 —— 只算 1 个
	}
	groupMembers := []struct {
		key, group, user string
		status           int
	}{
		{p + "gm1", p + "g1", users[1], 1}, // U2 是 g1 的正常成员
		{p + "gm2", p + "g1", users[3], 3}, // U4 是 g1 的已删除成员 —— 排除
	}
	repoMembers := []struct {
		key, repo, user string
		status          int
	}{
		{p + "rm1", p + "r1", users[4], 1}, // U5 直接是 r1 的成员
		{p + "rm2", p + "r2", users[4], 3}, // U5 对 r2 的成员资格已删除 —— 排除
	}

	// 清理：先删成员关系，再删仓库/组，最后删用户。
	defer func() {
		for _, m := range repoMembers {
			_, _ = db.ExecContext(ctx, "DELETE FROM cc_repository_members WHERE rm_key = ?", m.key)
		}
		for _, m := range groupMembers {
			_, _ = db.ExecContext(ctx, "DELETE FROM cc_group_members WHERE gm_key = ?", m.key)
		}
		for _, x := range repos {
			_, _ = db.ExecContext(ctx, "DELETE FROM cc_repositories WHERE r_key = ?", x.key)
		}
		for _, g := range groups {
			_, _ = db.ExecContext(ctx, "DELETE FROM cc_groups WHERE g_key = ?", g.key)
		}
		for _, u := range users {
			_, _ = db.ExecContext(ctx, "DELETE FROM cc_users WHERE u_key = ?", u)
		}
	}()

	for _, u := range users {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO cc_users (u_key, u_email, u_password, u_name, u_register_ip, u_created, u_updated, u_status) "+
				"VALUES (?, ?, 'x', ?, '127.0.0.1', NOW(), NOW(), 1)",
			u, u+"@perf.test", u); err != nil {
			t.Fatalf("插入用户 %s 失败: %v", u, err)
		}
	}
	for _, g := range groups {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO cc_groups (g_key, u_key, g_type, g_display_name, g_name, g_description, g_created, g_updated, g_status) "+
				"VALUES (?, ?, 1, ?, ?, '', NOW(), NOW(), ?)",
			g.key, g.owner, g.key, g.key, g.status); err != nil {
			t.Fatalf("插入组 %s 失败: %v", g.key, err)
		}
	}
	for _, x := range repos {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO cc_repositories (r_key, u_key, g_key, r_display_name, r_name, r_path, r_description, r_created, r_updated, r_status) "+
				"VALUES (?, ?, ?, ?, ?, ?, '', NOW(), NOW(), ?)",
			x.key, x.owner, x.group, x.key, x.key, x.key, x.status); err != nil {
			t.Fatalf("插入仓库 %s 失败: %v", x.key, err)
		}
	}
	for _, m := range groupMembers {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO cc_group_members (gm_key, g_key, u_key, gm_role, gm_created, gm_status) VALUES (?, ?, ?, 2, NOW(), ?)",
			m.key, m.group, m.user, m.status); err != nil {
			t.Fatalf("插入组成员 %s 失败: %v", m.key, err)
		}
	}
	for _, m := range repoMembers {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO cc_repository_members (rm_key, r_key, u_key, rm_role, rm_created, rm_status) VALUES (?, ?, ?, 2, NOW(), ?)",
			m.key, m.repo, m.user, m.status); err != nil {
			t.Fatalf("插入仓库成员 %s 失败: %v", m.key, err)
		}
	}

	got, err := r.CountJoinedByUsers(ctx, users)
	if err != nil {
		t.Fatalf("CountJoinedByUsers 失败: %v", err)
	}

	want := map[string]int{
		// U1: r1, r2, r7（g1 组下）+ r4（他拥有的仓库）。
		//
		// 注意 r4 虽然挂在**已删除的组** g3 下，但来源(4)"自己拥有的仓库"
		// 不看组状态，只看 r_status —— 这是 ListJoined 的既有语义。
		// 本用例把它固化下来，避免将来误"修正"成按组状态过滤。
		users[0]: 4,
		users[1]: 4, // g1 的 r1/r2/r7 + 自有 r5
		users[2]: 1, // g2 的 r3
		users[3]: 0, // 组成员资格已删除
		users[4]: 1, // 直接成员关系只有 r1
	}

	for _, u := range users {
		// 逐个与 ListJoined 的真实结果对比 —— 这是等价性的直接证据。
		list, err := r.ListJoined(ctx, u)
		if err != nil {
			t.Fatalf("ListJoined(%s) 失败: %v", u, err)
		}
		if len(list) != want[u] {
			t.Errorf("%s: ListJoined 返回 %d 个仓库, 期望 %d", u, len(list), want[u])
		}
		if got[u] != want[u] {
			t.Errorf("%s: CountJoinedByUsers = %d, 期望 %d", u, got[u], want[u])
		}
		if got[u] != len(list) {
			t.Errorf("%s: 批量计数(%d) 与 ListJoined(%d) 不一致 —— C2 改造破坏了语义",
				u, got[u], len(list))
		}
	}

	// 结果集必须覆盖全部入参用户（无仓库的用户得到 0，而不是缺失）。
	if len(got) != len(users) {
		t.Errorf("返回条目数 = %d, 期望 %d（无仓库的用户也应得到 0）", len(got), len(users))
	}

	// 空入参不应产生 SQL。
	empty, err := r.CountJoinedByUsers(ctx, nil)
	if err != nil {
		t.Fatalf("空入参应直接返回: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("空入参应返回空 map, 得到 %v", empty)
	}
}
