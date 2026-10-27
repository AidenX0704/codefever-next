package handlers

import (
	"testing"

	"codefever-next/backend/internal/domain"
)

// TestFilterMergeRequestsByRole 锁定首页"越权不返回"的安全语义。
//
// 背景：首页聚合把"待我评审""我发起的"直接拼进首屏。若漏掉角色过滤，
// 一个已被移出仓库（或角色被降为 guest）的人仍能在首页看到该仓库的 MR 标题与分支名
// —— 这是**信息泄露**，不是显示问题。
//
// 判定口径：看目标仓库的角色；`RoleGuest`（1）**及以下不返回**，其余返回。
func TestFilterMergeRequestsByRole(t *testing.T) {
	const (
		visibleKey = "visible-repo"
		guestKey   = "guest-repo"
		missingKey = "missing-repo"
	)

	list := []domain.RepositoryMergeRequest{
		{Key: "mr-a", TargetRKey: visibleKey, Title: "should stay"},
		{Key: "mr-b", TargetRKey: guestKey, Title: "guest must be hidden"},
		{Key: "mr-c", TargetRKey: missingKey, Title: "unknown role must be hidden"},
	}
	roles := map[string]int{
		visibleKey: int(domain.RoleDeveloper),
		guestKey:   int(domain.RoleGuest),
	}

	got := filterMergeRequestsByRole(list, roles)
	if len(got) != 1 {
		t.Fatalf("过滤后条数 = %d, 期望 1（只有目标仓库角色 > guest 的 MR 保留）", len(got))
	}
	if got[0].Key != "mr-a" {
		t.Fatalf("保留的 MR = %q, 期望 mr-a", got[0].Key)
	}

	// 空输入的边界：不应 panic，且返回非 nil 切片（JSON 序列化成 `[]` 而不是 `null`）。
	empty := filterMergeRequestsByRole(nil, nil)
	if empty == nil {
		t.Fatal("空输入返回 nil 切片，会被序列化成 null；前端按数组消费会炸")
	}
	if len(empty) != 0 {
		t.Fatalf("空输入过滤后条数 = %d, 期望 0", len(empty))
	}
}

// TestHomeMergeRequestViewsOmitCommit 锁定"首页不做 git 调用"的约定。
//
// 首页 view 刻意不填 `commit` / `reviewers`：填它们意味着逐条 `git log`，
// 5 条待办 = 5 次子进程。若有人为了"顺手补全"把它们加回来，这条用例会失败，
// 提醒他先看 docs/HOME_PAGE_PLAN_2026-10-27.md 的 D2 取舍。
func TestHomeMergeRequestViewsOmitCommit(t *testing.T) {
	views := homeMergeRequestViews([]domain.RepositoryMergeRequest{
		{Key: "mr-a", ID: 7, Title: "t", SourceRKey: "s", TargetRKey: "t2", ProceedStatus: 1},
	})
	if len(views) != 1 {
		t.Fatalf("条数 = %d, 期望 1", len(views))
	}
	view := views[0]
	if view.Number != 7 || view.Title != "t" {
		t.Fatalf("字段映射错误: number=%d title=%q", view.Number, view.Title)
	}
	if view.Commit.SHA != "" || view.Commit.Commit != "" || view.Commit.Email != "" {
		t.Fatalf("首页 view 不应携带 commit（会诱发逐条 git 调用）: %+v", view.Commit)
	}
	if len(view.Reviewers) != 0 {
		t.Fatalf("首页 view 不应携带 reviewers，实际 %d 条", len(view.Reviewers))
	}
}
