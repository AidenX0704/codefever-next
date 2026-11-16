package repo

import (
	"context"
	"database/sql"
	"testing"

	"codefever-next/backend/internal/domain"
)

// 本文件覆盖站点公开信息仓储（changelog / 公告）的**真实数据库**行为。
//
// 需要真实 MySQL：
//
//	$env:CODEFEVER_TEST_DSN = "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true"
//	go test ./internal/repo/ -run TestSiteRepo -v
//
// 覆盖的关键不变量：
//   - Save 的"有没有 id"决定新建 / 更新，LastInsertId 正确回填；
//   - 列表排序 released_at desc, cl_id desc；NULL 发布日期沉底；
//   - 公告的 an_enabled=1 过滤与条数上限；
//   - 删除后读取返回 ErrNotFound。

// TestSiteRepoChangelogLifecycle 验证更新日志的完整生命周期。
func TestSiteRepoChangelogLifecycle(t *testing.T) {
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
	site := NewSiteRepo(db)

	// 1) 新建（无 releasedAt → NULL）。
	item := &domain.Changelog{Version: "v0.0.0-test", Title: "测试条目"}
	if err := site.SaveChangelog(ctx, item); err != nil {
		t.Fatalf("创建更新日志失败: %v", err)
	}
	if item.ID == 0 {
		t.Fatal("新建后主键应回填")
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM cc_changelogs WHERE cl_id = ?", item.ID)
	}()

	// 2) 读取。
	found, err := site.GetChangelog(ctx, item.ID)
	if err != nil {
		t.Fatalf("读取更新日志失败: %v", err)
	}
	if found.Title != "测试条目" || found.ReleasedAt != "" {
		t.Errorf("读取结果不正确: %+v", found)
	}

	// 3) 更新（带 id 与 releasedAt）。
	item.Title = "更新后的标题"
	item.ReleasedAt = "2026-10-01"
	if err := site.SaveChangelog(ctx, item); err != nil {
		t.Fatalf("更新更新日志失败: %v", err)
	}
	found, err = site.GetChangelog(ctx, item.ID)
	if err != nil {
		t.Fatalf("读取更新日志失败: %v", err)
	}
	if found.Title != "更新后的标题" || found.ReleasedAt != "2026-10-01" {
		t.Errorf("更新结果不正确: %+v", found)
	}

	// 4) 删除后读取必须 ErrNotFound。
	if err := site.DeleteChangelog(ctx, item.ID); err != nil {
		t.Fatalf("删除更新日志失败: %v", err)
	}
	if err := site.DeleteChangelog(ctx, item.ID); err != ErrNotFound {
		t.Errorf("重复删除应返回 ErrNotFound, got %v", err)
	}
	if _, err := site.GetChangelog(ctx, item.ID); err != ErrNotFound {
		t.Errorf("删除后读取应返回 ErrNotFound, got %v", err)
	}
}

// TestSiteRepoChangelogOrdering 验证列表排序与分页。
func TestSiteRepoChangelogOrdering(t *testing.T) {
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
	site := NewSiteRepo(db)

	// 造三条：A(2026-10-02)、B(2026-10-01)、C(无日期)。
	// 期望顺序 A、B、C（NULL 发布日期沉底）。
	ids := make([]int, 0, 3)
	defer func() {
		for _, id := range ids {
			_, _ = db.ExecContext(ctx, "DELETE FROM cc_changelogs WHERE cl_id = ?", id)
		}
	}()
	for _, spec := range []struct {
		title, released string
	}{
		{"排序A", "2026-10-02"},
		{"排序B", "2026-10-01"},
		{"排序C", ""},
	} {
		item := &domain.Changelog{Version: "v-test-order", Title: spec.title, ReleasedAt: spec.released}
		if err := site.SaveChangelog(ctx, item); err != nil {
			t.Fatalf("创建排序数据失败: %v", err)
		}
		ids = append(ids, item.ID)
	}

	list, total, err := site.ListChangelogs(ctx, 1, 100)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if total < 3 {
		t.Fatalf("总数 = %d, want >= 3", total)
	}
	// 找到我们造的三条在列表中的相对位置。
	pos := map[string]int{}
	for i, c := range list {
		if c.Version == "v-test-order" {
			pos[c.Title] = i
		}
	}
	if len(pos) != 3 {
		t.Fatalf("造出的三条没有全部出现在列表里: %v", pos)
	}
	if !(pos["排序A"] < pos["排序B"] && pos["排序B"] < pos["排序C"]) {
		t.Errorf("排序不正确: %v（期望 A < B < C，无日期沉底）", pos)
	}
}

// TestSiteRepoAnnouncementLifecycle 验证公告的启用过滤与删除。
func TestSiteRepoAnnouncementLifecycle(t *testing.T) {
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
	site := NewSiteRepo(db)

	enabled := &domain.Announcement{Type: "support", Body: "启用的公告", Enabled: true}
	disabled := &domain.Announcement{Type: "security", Body: "停用的公告", Enabled: false}
	for _, a := range []*domain.Announcement{enabled, disabled} {
		if err := site.SaveAnnouncement(ctx, a); err != nil {
			t.Fatalf("创建公告失败: %v", err)
		}
		defer func(id int) {
			_, _ = db.ExecContext(ctx, "DELETE FROM cc_announcements WHERE an_id = ?", id)
		}(a.ID)
	}

	// 公开列表只包含启用的公告。
	list, err := site.ListAnnouncements(ctx, 10)
	if err != nil {
		t.Fatalf("列出公告失败: %v", err)
	}
	foundEnabled, foundDisabled := false, false
	for _, a := range list {
		if a.ID == enabled.ID {
			foundEnabled = true
		}
		if a.ID == disabled.ID {
			foundDisabled = true
		}
	}
	if !foundEnabled {
		t.Error("启用的公告应出现在公开列表里")
	}
	if foundDisabled {
		t.Error("停用的公告不应出现在公开列表里")
	}

	// 管理列表（ListAllAnnouncements）两者都能看到。
	all, err := site.ListAllAnnouncements(ctx)
	if err != nil {
		t.Fatalf("管理列表失败: %v", err)
	}
	seen := 0
	for _, a := range all {
		if a.ID == enabled.ID || a.ID == disabled.ID {
			seen++
		}
	}
	if seen != 2 {
		t.Errorf("管理列表应同时包含启用与停用, got %d", seen)
	}

	// 更新（把停用的启用）。
	disabled.Enabled = true
	if err := site.SaveAnnouncement(ctx, disabled); err != nil {
		t.Fatalf("更新公告失败: %v", err)
	}
	list, err = site.ListAnnouncements(ctx, 10)
	if err != nil {
		t.Fatalf("列出公告失败: %v", err)
	}
	foundDisabled = false
	for _, a := range list {
		if a.ID == disabled.ID {
			foundDisabled = true
		}
	}
	if !foundDisabled {
		t.Error("启用后的公告应出现在公开列表里")
	}

	// 删除。
	if err := site.DeleteAnnouncement(ctx, disabled.ID); err != nil {
		t.Fatalf("删除公告失败: %v", err)
	}
	if err := site.DeleteAnnouncement(ctx, disabled.ID); err != ErrNotFound {
		t.Errorf("重复删除应返回 ErrNotFound, got %v", err)
	}
}
