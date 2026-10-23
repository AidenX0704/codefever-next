package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
)

// 通知相关常量，逐字对应 application/config/constants.php。
const (
	// NotifyForPublic 是面向全体的系统通知作用域。
	NotifyForPublic = 1
	// NotifyForUser 是面向指定用户的通知作用域。
	NotifyForUser = 2
	// NotifyForDM 是直接消息作用域。
	NotifyForDM = 3

	// NotifyRead / NotifyUnread 是通知的已读 / 未读标记。
	NotifyRead   = 1
	NotifyUnread = 2

	// NotificationCategoryAll / Unread / System / MentionMe 是前端的通知分类。
	NotificationCategoryAll       = 0
	NotificationCategoryUnread    = 1
	NotificationCategorySystem    = 2
	NotificationCategoryMentionMe = 3

	// NotificationRefuseGroup / Repository 表示拒收通知的对象类型。
	NotificationRefuseGroup      = 1
	NotificationRefuseRepository = 2

	// NotificationStatusMR / Email 是用户通知设置的位掩码位。
	//
	// **位 = 1 表示关闭**（见 domain.NormalizeUser 的 notification 字段）。
	NotificationStatusMR    = 1
	NotificationStatusEmail = 2

	// CommonStatusDelete 是软删除状态值。
	CommonStatusDelete = 4
)

// NotificationRepo 负责 cc_notification_users / cc_notification_content / cc_notification_refuse。
type NotificationRepo struct {
	db *sql.DB
}

// NewNotificationRepo 创建通知仓储。
func NewNotificationRepo(db *sql.DB) *NotificationRepo { return &NotificationRepo{db: db} }

// Notification 是一条通知的归一化形态。
//
// 逐字对应 application/models/notification_model.php:11-27 的 normalizeNotifications()。
type Notification struct {
	ID         string
	Type       string
	Group      *string
	Repository *string
	// Data 是 JSON **字符串**（不是对象）——
	// 前端 www/view/src/helpers/VaribleHelper.js:248 会对其做 JSON.parse。
	Data    string
	Created int64
	IsRead  bool
}

// NotificationRefused 是拒收通知配置的归一化形态。
//
// 逐字对应 notification_model.php:29-41 的 normalizeNotificationsRefused()。
type NotificationRefused struct {
	ID     string
	Type   int
	Target string
}

// List 列出通知。
//
// 逐字对应 notification_model.php:43-78 的 listNotifications()：
//   - 分类 2（系统通知）查 cc_notification_content（不限用户），此时 nu_key 为空、isRead 恒为 true；
//   - 其余分类按用户查 cc_notification_users LEFT JOIN cc_notification_content；
//   - 按 nc_created 倒序，pagesize 固定 20（PHP 调用时未传 pagesize）。
func (r *NotificationRepo) List(ctx context.Context, uKey string, category, page, pagesize int) ([]Notification, error) {
	if uKey == "" {
		return nil, nil
	}

	var rows *sql.Rows
	var err error

	if category == NotificationCategorySystem {
		rows, err = r.db.QueryContext(ctx, `
			SELECT '' AS nu_key, nc_type, nc.g_key, nc.r_key, nc_data, nc_created, 0 AS nu_is_read
			FROM cc_notification_content AS nc
			WHERE nc_scope = ?
			ORDER BY nc_created DESC
			LIMIT ? OFFSET ?`, NotifyForPublic, pagesize, (page-1)*pagesize)
	} else {
		scope := NotifyForUser
		if category == NotificationCategoryMentionMe {
			scope = NotifyForDM
		}
		query := `
			SELECT nu.nu_key, nc.nc_type, nc.g_key, nc.r_key, nc_data, nc_created, nu.nu_is_read
			FROM cc_notification_users AS nu
			LEFT JOIN cc_notification_content AS nc ON nu.nc_key = nc.nc_key
			WHERE nu.u_key = ? AND nc.nc_scope = ? AND nu.nu_status = ?`
		args := []any{uKey, scope, domain.CommonStatusNormal}
		if category == NotificationCategoryUnread {
			query += " AND nu.nu_is_read = ?"
			args = append(args, NotifyUnread)
		}
		query += " ORDER BY nc_created DESC LIMIT ? OFFSET ?"
		args = append(args, pagesize, (page-1)*pagesize)
		rows, err = r.db.QueryContext(ctx, query, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Notification{}
	for rows.Next() {
		var n Notification
		var nuKey sql.NullString
		var ncType sql.NullString
		var gKey, rKey, ncData sql.NullString
		var created sql.NullTime
		var isRead sql.NullInt64

		if err := rows.Scan(&nuKey, &ncType, &gKey, &rKey, &ncData, &created, &isRead); err != nil {
			return nil, err
		}
		n.ID = nuKey.String
		n.Type = ncType.String
		n.Group = nullToPtrStr(gKey)
		n.Repository = nullToPtrStr(rKey)
		n.Data = ncData.String
		if created.Valid {
			n.Created = created.Time.Unix()
		}
		// PHP: $item['nu_is_read'] ? ($item['nu_is_read'] == NOTIFY_READ) : true
		// —— 未读标记为空（系统通知场景）时视为已读。
		if isRead.Valid && isRead.Int64 != 0 {
			n.IsRead = isRead.Int64 == NotifyRead
		} else {
			n.IsRead = true
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SetRead 标记通知为已读。
//
// 逐字对应 notification_model.php:122-141 的 setRead()。
func (r *NotificationRepo) SetRead(ctx context.Context, uKey string, all bool, nuKey string) error {
	if uKey == "" {
		return ErrNotFound
	}
	query := "UPDATE cc_notification_users SET nu_is_read = ?, nu_updated = ? WHERE u_key = ?"
	args := []any{NotifyRead, time.Now(), uKey}
	if !all {
		query += " AND nu_key = ?"
		args = append(args, nuKey)
	}
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// DeleteReaded 软删除已读通知。
//
// 逐字对应 notification_model.php:143-164 的 deleteReaded()：
// all=true 时**额外**限定 nu_is_read = NOTIFY_READ（只删已读的）。
func (r *NotificationRepo) DeleteReaded(ctx context.Context, uKey string, all bool, nuKey string) error {
	if uKey == "" {
		return ErrNotFound
	}
	query := "UPDATE cc_notification_users SET nu_deleted = ?, nu_status = ? WHERE u_key = ?"
	args := []any{time.Now(), CommonStatusDelete, uKey}
	if !all {
		query += " AND nu_key = ?"
		args = append(args, nuKey)
	} else {
		query += " AND nu_is_read = ?"
		args = append(args, NotifyRead)
	}
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// ListRefused 列出拒收通知的配置。
func (r *NotificationRepo) ListRefused(ctx context.Context, uKey string) ([]NotificationRefused, error) {
	if uKey == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT nr_key, nr_type, g_key, r_key
		FROM cc_notification_refuse WHERE u_key = ? AND nr_status = ?`,
		uKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []NotificationRefused{}
	for rows.Next() {
		var id string
		var typ int
		var gKey, rKey sql.NullString
		if err := rows.Scan(&id, &typ, &gKey, &rKey); err != nil {
			return nil, err
		}
		item := NotificationRefused{ID: id, Type: typ}
		if typ == NotificationRefuseGroup {
			item.Target = gKey.String
		} else {
			item.Target = rKey.String
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// AddRefused 新增一条拒收配置。
func (r *NotificationRepo) AddRefused(ctx context.Context, uKey string, typ int, target string) error {
	col := "r_key"
	if typ == NotificationRefuseGroup {
		col = "g_key"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cc_notification_refuse (nr_key, nr_type, u_key, `+col+`)
		VALUES (?, ?, ?, ?)`, contract.NewKey(), typ, uKey, target)
	return err
}

// DeleteRefused 删除一条拒收配置。
func (r *NotificationRepo) DeleteRefused(ctx context.Context, uKey string, typ int, target string) error {
	col := "r_key"
	if typ == NotificationRefuseGroup {
		col = "g_key"
	}
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM cc_notification_refuse
		WHERE u_key = ? AND nr_type = ? AND `+col+` = ?`, uKey, typ, target)
	return err
}

// HasRefused 判断某条拒收配置是否存在。
func (r *NotificationRepo) HasRefused(ctx context.Context, uKey string, typ int, target string) (bool, error) {
	col := "r_key"
	if typ == NotificationRefuseGroup {
		col = "g_key"
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM cc_notification_refuse
		WHERE u_key = ? AND nr_type = ? AND `+col+` = ? AND nr_status = ?`,
		uKey, typ, target, domain.CommonStatusNormal).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// SetRefused 关闭通知接收（拒收）。
//
// 逐字对应 notification_model.php:180-206 的 setGroupOrRepoRefused()：
// 关闭**仓库组**通知时，该组下所有尚未单独关闭的仓库也一并关闭（级联）。
func (r *NotificationRepo) SetRefused(ctx context.Context, uKey string, typ int, target string) error {
	if uKey == "" || typ == 0 || target == "" {
		return ErrNotFound
	}
	if err := r.AddRefused(ctx, uKey, typ, target); err != nil {
		return err
	}
	if typ != NotificationRefuseGroup {
		return nil
	}

	// 级联：组内尚未单独关闭的仓库一并关闭。
	repos, err := r.RepositoriesInGroup(ctx, target)
	if err != nil {
		return err
	}
	refused, err := r.ListRefused(ctx, uKey)
	if err != nil {
		return err
	}
	already := map[string]bool{}
	for _, item := range refused {
		if item.Type == NotificationRefuseRepository && item.Target != "" {
			already[item.Target] = true
		}
	}
	for _, rKey := range repos {
		if already[rKey] {
			continue
		}
		if err := r.AddRefused(ctx, uKey, NotificationRefuseRepository, rKey); err != nil {
			return err
		}
	}
	return nil
}

// SetReceive 恢复通知接收。
//
// 逐字对应 notification_model.php:225-243 的 setGroupOrRepoReceive()：
//   - 恢复**仓库组**通知 → 该组下所有仓库的关闭记录一并删除；
//   - 恢复**仓库**通知 → 同时恢复它所属组的关闭记录。
func (r *NotificationRepo) SetReceive(ctx context.Context, uKey string, typ int, target string) error {
	if uKey == "" || typ == 0 || target == "" {
		return ErrNotFound
	}
	if err := r.DeleteRefused(ctx, uKey, typ, target); err != nil {
		return err
	}

	if typ == NotificationRefuseGroup {
		repos, err := r.RepositoriesInGroup(ctx, target)
		if err != nil {
			return err
		}
		for _, rKey := range repos {
			if err := r.DeleteRefused(ctx, uKey, NotificationRefuseRepository, rKey); err != nil {
				return err
			}
		}
		return nil
	}

	// 恢复仓库通知时，同时恢复它所属组的关闭记录。
	var gKey sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT g_key FROM cc_repositories WHERE r_key = ?", target).Scan(&gKey)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !gKey.Valid {
		return ErrNotFound
	}
	return r.DeleteRefused(ctx, uKey, NotificationRefuseGroup, gKey.String)
}

// RepositoriesInGroup 列出组内的仓库主键。
func (r *NotificationRepo) RepositoriesInGroup(ctx context.Context, gKey string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT r_key FROM cc_repositories WHERE g_key = ? AND r_status = ?",
		gKey, domain.CommonStatusNormal)
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

// nullToPtrStr 把可空字符串转为指针（NULL → nil）。
func nullToPtrStr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}
