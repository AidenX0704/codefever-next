package domain

// 站点公开信息（统计 / 更新日志 / 公告）。
//
// 这些端点是本仓库**新增**的能力（PHP 侧没有对应实现），
// 因此没有"逐字对应 PHP"的约束，字段命名沿用 /api/* 的驼峰惯例：
// 主键叫 id，时间输出秒级时间戳（与 phpTimeOf 的全站口径一致）。

// SiteStats 是 GET /api/site/stats 的输出。
//
// 全部来自 DB count，不做任何 git 扫描 —— 首页/落地页的公开指标
// 必须在毫秒级返回，git 统计（commit 数之类）不在其中。
type SiteStats struct {
	RepositoryCount         int `json:"repositoryCount"`
	GroupCount              int `json:"groupCount"`
	MemberCount             int `json:"memberCount"`
	MergedMergeRequestCount int `json:"mergedMergeRequestCount"`
}

// Changelog 是 cc_changelogs 表的一行（对外形态见 ChangelogView）。
//
// ReleasedAt 保留原始字符串（DATE 列，形如 "2026-10-01"），
// 为 NULL 时是空串 —— 对外用 *string 区分"未设置"与"空"。
type Changelog struct {
	ID         int
	Version    string
	Title      string
	Body       string
	ReleasedAt string
	CreatedAt  string
}

// ChangelogView 是 GET /api/site/changelog 的元素。
type ChangelogView struct {
	ID         int     `json:"id"`
	Version    string  `json:"version"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	ReleasedAt *string `json:"releasedAt"`
	CreatedAt  int64   `json:"createdAt"`
}

// NormalizeChangelog 把 changelog 行转换为对外形态。
//
// releasedAt 用 *string：DATE 列为 NULL 时输出 null（未设置发布日期），
// 有值时输出 "YYYY-MM-DD" 原文（日期不需要 strtotime）。
func NormalizeChangelog(c *Changelog) ChangelogView {
	var released *string
	if c.ReleasedAt != "" {
		released = &c.ReleasedAt
	}
	return ChangelogView{
		ID:         c.ID,
		Version:    c.Version,
		Title:      c.Title,
		Body:       c.Body,
		ReleasedAt: released,
		CreatedAt:  PhpTime(c.CreatedAt),
	}
}

// AnnouncementType 是公告类型的合法取值（an_type 列的白名单）。
type AnnouncementType string

const (
	AnnouncementTypeSecurity AnnouncementType = "security"
	AnnouncementTypePrivacy  AnnouncementType = "privacy"
	AnnouncementTypeSupport  AnnouncementType = "support"
)

// AnnouncementTypeAllowed 判断公告类型是否在白名单内。
//
// an_type 列的默认值是 'support'（最通用的类型），写入前必须校验，
// 避免把任意字符串灌进列里。
func AnnouncementTypeAllowed(t string) bool {
	switch AnnouncementType(t) {
	case AnnouncementTypeSecurity, AnnouncementTypePrivacy, AnnouncementTypeSupport:
		return true
	}
	return false
}

// Announcement 是 cc_announcements 表的一行（对外形态见 AnnouncementView）。
type Announcement struct {
	ID        int
	Type      string
	Body      string
	Enabled   bool
	CreatedAt string
}

// AnnouncementView 是 GET /api/site/announcement 的元素。
type AnnouncementView struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Body      string `json:"body"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt"`
}

// NormalizeAnnouncement 把公告行转换为对外形态。
func NormalizeAnnouncement(a *Announcement) AnnouncementView {
	return AnnouncementView{
		ID:        a.ID,
		Type:      a.Type,
		Body:      a.Body,
		Enabled:   a.Enabled,
		CreatedAt: PhpTime(a.CreatedAt),
	}
}
