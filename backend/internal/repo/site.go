package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"codefever-next/backend/internal/domain"
)

// SiteRepo 负责站点公开信息（统计 / 更新日志 / 公告）的读写。
//
// 对应表：cc_changelogs、cc_announcements（012_changelogs_announcements.sql）。
// 统计部分只对既有表做 COUNT，不引入新表。
type SiteRepo struct {
	db *sql.DB
}

// NewSiteRepo 创建站点公开信息仓储。
func NewSiteRepo(db *sql.DB) *SiteRepo { return &SiteRepo{db: db} }

// ---------------------------------------------------------------------------
// 统计
// ---------------------------------------------------------------------------

// Stats 统计站点公开指标。
//
// 口径说明（都是 COUNT，别扫 git）：
//   - repositoryCount：未软删除的仓库（r_status = 1）；
//   - groupCount：未软删除的仓库组（g_status = 1）；
//   - memberCount：未软删除的用户（u_status != 4，与登录侧"禁用即失效"的口径一致）；
//   - mergedMergeRequestCount：已合并的合并请求（mr_proceed_status = 2）。
//
// 四条查询合在一条里执行（子查询标量），一次往返拿全，公开端点不能多跑。
func (r *SiteRepo) Stats(ctx context.Context) (domain.SiteStats, error) {
	var stats domain.SiteStats
	err := r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM cc_repositories WHERE r_status = ?),
			(SELECT COUNT(*) FROM cc_groups WHERE g_status = ?),
			(SELECT COUNT(*) FROM cc_users WHERE u_status <> ?),
			(SELECT COUNT(*) FROM cc_merge_requests WHERE mr_proceed_status = ? AND mr_status = ?)`,
		domain.CommonStatusNormal, domain.CommonStatusNormal, domain.CommonStatusDelete,
		domain.MergeRequestStatusMerged, domain.CommonStatusNormal,
	).Scan(&stats.RepositoryCount, &stats.GroupCount, &stats.MemberCount, &stats.MergedMergeRequestCount)
	if err != nil {
		return domain.SiteStats{}, fmt.Errorf("统计站点指标失败: %w", err)
	}
	return stats, nil
}

// ---------------------------------------------------------------------------
// 更新日志
// ---------------------------------------------------------------------------

// changelogColumns 是 cc_changelogs 的列清单（与 scanChangelog 的顺序绑定）。
const changelogColumns = "cl_id, cl_version, cl_title, cl_body, " +
	"CAST(cl_released_at AS CHAR), CAST(cl_created_at AS CHAR)"

// ListChangelogs 分页列出更新日志。
//
// 排序 released_at desc, cl_id desc：同一发布日期里按创建先后倒排，
// 让"最新发布的条目"排在最前。NULL 的 released_at 在 MySQL 8 的
// `ORDER BY cl_released_at DESC` 里排在最后（DESC 时 NULL 最小），
// 即"未定发布日期的草稿"沉底 —— 这正是期望的行为。
func (r *SiteRepo) ListChangelogs(ctx context.Context, page, pagesize int) ([]domain.Changelog, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cc_changelogs").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计更新日志总数失败: %w", err)
	}
	if total == 0 {
		return []domain.Changelog{}, 0, nil
	}

	offset := (page - 1) * pagesize
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+changelogColumns+`
		FROM cc_changelogs
		ORDER BY cl_released_at DESC, cl_id DESC
		LIMIT ? OFFSET ?`, pagesize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询更新日志失败: %w", err)
	}
	defer rows.Close()

	out := []domain.Changelog{}
	for rows.Next() {
		c, err := scanChangelog(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *c)
	}
	return out, total, rows.Err()
}

// GetChangelog 按主键读取一条更新日志。
func (r *SiteRepo) GetChangelog(ctx context.Context, id int) (*domain.Changelog, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+changelogColumns+" FROM cc_changelogs WHERE cl_id = ?", id)
	c, err := scanChangelog(row)
	if errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("查询更新日志失败: %w", err)
	}
	return c, nil
}

// SaveChangelog 创建或更新一条更新日志（id 为 0 表示新建）。
//
// 管理端只有一个保存入口，用"有没有 id"区分新建与编辑，
// 避免前端维护两套提交逻辑。
func (r *SiteRepo) SaveChangelog(ctx context.Context, c *domain.Changelog) error {
	if c.ID == 0 {
		res, err := r.db.ExecContext(ctx, `
			INSERT INTO cc_changelogs (cl_version, cl_title, cl_body, cl_released_at)
			VALUES (?, ?, ?, ?)`,
			c.Version, c.Title, c.Body, nullableDateString(c.ReleasedAt))
		if err != nil {
			return fmt.Errorf("创建更新日志失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("读取更新日志主键失败: %w", err)
		}
		c.ID = int(id)
		return nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE cc_changelogs
		SET cl_version = ?, cl_title = ?, cl_body = ?, cl_released_at = ?
		WHERE cl_id = ?`,
		c.Version, c.Title, c.Body, nullableDateString(c.ReleasedAt), c.ID)
	if err != nil {
		return fmt.Errorf("更新更新日志失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteChangelog 物理删除一条更新日志。
//
// 返回 ErrNotFound 表示目标不存在，调用方决定如何呈现。
func (r *SiteRepo) DeleteChangelog(ctx context.Context, id int) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM cc_changelogs WHERE cl_id = ?", id)
	if err != nil {
		return fmt.Errorf("删除更新日志失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanChangelog 扫描一行 cc_changelogs。
//
// DATE 列为 NULL 时 CAST 出的字符串是 NULL → 空串，与 domain.Changelog
// 的"空串 = 未设置"约定一致。
func scanChangelog(row scanner) (*domain.Changelog, error) {
	var c domain.Changelog
	var released, created sql.NullString
	err := row.Scan(&c.ID, &c.Version, &c.Title, &c.Body, &released, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.ReleasedAt = released.String
	c.CreatedAt = created.String
	return &c, nil
}

// nullableDateString 把"空串 = 未设置"转换为数据库的 NULL。
func nullableDateString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ---------------------------------------------------------------------------
// 公告
// ---------------------------------------------------------------------------

// announcementColumns 是 cc_announcements 的列清单（与 scanAnnouncement 绑定）。
const announcementColumns = "an_id, an_type, an_body, an_enabled, CAST(an_created_at AS CHAR)"

// ListAnnouncements 列出启用的公告（an_id desc，最多 limit 条）。
//
// limit 由调用方传入（公开端点固定 10），<=0 视为 10。
func (r *SiteRepo) ListAnnouncements(ctx context.Context, limit int) ([]domain.Announcement, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+announcementColumns+`
		FROM cc_announcements
		WHERE an_enabled = 1
		ORDER BY an_id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("查询公告失败: %w", err)
	}
	defer rows.Close()

	out := []domain.Announcement{}
	for rows.Next() {
		a, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// ListAllAnnouncements 列出全部公告（管理端列表用，含停用的）。
func (r *SiteRepo) ListAllAnnouncements(ctx context.Context) ([]domain.Announcement, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+announcementColumns+`
		FROM cc_announcements
		ORDER BY an_id DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询公告失败: %w", err)
	}
	defer rows.Close()

	out := []domain.Announcement{}
	for rows.Next() {
		a, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// GetAnnouncement 按主键读取一条公告。
func (r *SiteRepo) GetAnnouncement(ctx context.Context, id int) (*domain.Announcement, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+announcementColumns+" FROM cc_announcements WHERE an_id = ?", id)
	a, err := scanAnnouncement(row)
	if errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("查询公告失败: %w", err)
	}
	return a, nil
}

// SaveAnnouncement 创建或更新一条公告（id 为 0 表示新建）。
func (r *SiteRepo) SaveAnnouncement(ctx context.Context, a *domain.Announcement) error {
	enabled := 0
	if a.Enabled {
		enabled = 1
	}
	if a.ID == 0 {
		res, err := r.db.ExecContext(ctx, `
			INSERT INTO cc_announcements (an_type, an_body, an_enabled)
			VALUES (?, ?, ?)`,
			a.Type, a.Body, enabled)
		if err != nil {
			return fmt.Errorf("创建公告失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("读取公告主键失败: %w", err)
		}
		a.ID = int(id)
		return nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE cc_announcements
		SET an_type = ?, an_body = ?, an_enabled = ?
		WHERE an_id = ?`,
		a.Type, a.Body, enabled, a.ID)
	if err != nil {
		return fmt.Errorf("更新公告失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAnnouncement 物理删除一条公告。
func (r *SiteRepo) DeleteAnnouncement(ctx context.Context, id int) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM cc_announcements WHERE an_id = ?", id)
	if err != nil {
		return fmt.Errorf("删除公告失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanAnnouncement 扫描一行 cc_announcements。
func scanAnnouncement(row scanner) (*domain.Announcement, error) {
	var a domain.Announcement
	var enabled int
	var created sql.NullString
	err := row.Scan(&a.ID, &a.Type, &a.Body, &enabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Enabled = enabled != 0
	a.CreatedAt = created.String
	return &a, nil
}
