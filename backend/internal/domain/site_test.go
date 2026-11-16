package domain

import "testing"

// 站点公开信息（更新日志 / 公告）的纯逻辑用例。

// TestAnnouncementTypeAllowed 锁定公告类型的白名单。
func TestAnnouncementTypeAllowed(t *testing.T) {
	for _, allowed := range []string{"security", "privacy", "support"} {
		if !AnnouncementTypeAllowed(allowed) {
			t.Errorf("类型 %q 应当合法", allowed)
		}
	}
	for _, rejected := range []string{"", "Security", "other", "0"} {
		if AnnouncementTypeAllowed(rejected) {
			t.Errorf("类型 %q 不应当合法", rejected)
		}
	}
}

// TestNormalizeChangelog 锁定 changelog 的输出形态。
//
// releasedAt 的 NULL / 有值语义必须区分：未设置 → null，
// 有值 → "YYYY-MM-DD" 原文。
func TestNormalizeChangelog(t *testing.T) {
	view := NormalizeChangelog(&Changelog{
		ID:         3,
		Version:    "v1.2.0",
		Title:      "标题",
		Body:       "正文",
		ReleasedAt: "2026-10-01",
		CreatedAt:  "2026-10-01 10:00:00",
	})
	if view.ID != 3 || view.Version != "v1.2.0" || view.Title != "标题" {
		t.Errorf("字段映射不正确: %+v", view)
	}
	if view.ReleasedAt == nil || *view.ReleasedAt != "2026-10-01" {
		t.Errorf("releasedAt = %v, want 2026-10-01", view.ReleasedAt)
	}
	if view.CreatedAt != 1790848800 {
		t.Errorf("createdAt = %d（应为秒级时间戳，与全站 phpTimeOf 口径一致）", view.CreatedAt)
	}

	// 未设置发布日期 → null（不是空串）。
	draft := NormalizeChangelog(&Changelog{ID: 1, Title: "草稿"})
	if draft.ReleasedAt != nil {
		t.Errorf("未设置 releasedAt 时应输出 null, got %v", *draft.ReleasedAt)
	}
}

// TestNormalizeAnnouncement 锁定公告的输出形态。
func TestNormalizeAnnouncement(t *testing.T) {
	view := NormalizeAnnouncement(&Announcement{
		ID:        7,
		Type:      "security",
		Body:      "内容",
		Enabled:   true,
		CreatedAt: "2026-10-01 10:00:00",
	})
	if view.ID != 7 || view.Type != "security" || !view.Enabled {
		t.Errorf("字段映射不正确: %+v", view)
	}
	if view.CreatedAt != 1790848800 {
		t.Errorf("createdAt = %d", view.CreatedAt)
	}
}
