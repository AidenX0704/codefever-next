// Package session 实现服务端会话存储。
//
// 现有 PHP 后端把会话放进 CI2 的加密 Cookie（mcrypt，键取自 env.yaml 的
// session.key），存在三个问题：
//   - 会话内容在客户端可见（虽已加密，但密钥与实现耦合）；
//   - cookie_secure=FALSE，10 天有效期的登录态可经明文 HTTP 传输；
//   - 无法服务端主动吊销。
//
// 新实现改为服务端存储：Cookie 里只放一个 32 位随机 session id，
// 其余状态在 cc_sessions 表。Cookie 名保持 codefever_community 不变，
// 因此前端（credentials:'same-origin'）完全无感。
package session

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"codefever-next/backend/internal/contract"
)

// ErrNotFound 表示会话不存在或已过期。
var ErrNotFound = errors.New("会话不存在或已过期")

// Session 是一次登录会话。
type Session struct {
	Key       string
	UKey      string
	IP        string
	UserAgent string
	Created   time.Time
	Expires   time.Time
}

// Store 是会话仓储。
type Store struct {
	db  *sql.DB
	ttl time.Duration
}

// NewStore 创建会话仓储。
//
// ttl 对应 PHP 侧的 sess_expiration = 3600 * 24 * 10（10 天）。
func NewStore(db *sql.DB, ttl time.Duration) *Store {
	return &Store{db: db, ttl: ttl}
}

// TTL 返回会话有效期。
func (s *Store) TTL() time.Duration { return s.ttl }

// Create 创建一个新会话并返回其 session id。
func (s *Store) Create(ctx context.Context, uKey, ip, userAgent string) (*Session, error) {
	key := contract.NewKey()
	now := time.Now()
	expires := now.Add(s.ttl)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cc_sessions (s_key, u_key, s_ip, s_user_agent, s_created, s_expires, s_last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key, uKey, ip, userAgent, now, expires, now)
	if err != nil {
		return nil, err
	}
	return &Session{Key: key, UKey: uKey, IP: ip, UserAgent: userAgent, Created: now, Expires: expires}, nil
}

// Get 按 session id 读取会话。
//
// 过期会话会被视为不存在并顺带清理。
func (s *Store) Get(ctx context.Context, key string) (*Session, error) {
	if key == "" {
		return nil, ErrNotFound
	}
	var sess Session
	err := s.db.QueryRowContext(ctx, `
		SELECT s_key, u_key, s_ip, s_user_agent, s_created, s_expires
		FROM cc_sessions WHERE s_key = ?`, key).
		Scan(&sess.Key, &sess.UKey, &sess.IP, &sess.UserAgent, &sess.Created, &sess.Expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(sess.Expires) {
		_ = s.Delete(ctx, key)
		return nil, ErrNotFound
	}
	return &sess, nil
}

// Touch 刷新会话的最后访问时间与过期时间（滑动续期）。
func (s *Store) Touch(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE cc_sessions SET s_last_seen = ?, s_expires = ? WHERE s_key = ?`,
		time.Now(), time.Now().Add(s.ttl), key)
	return err
}

// Delete 删除一个会话（登出）。
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM cc_sessions WHERE s_key = ?", key)
	return err
}

// DeleteByUser 删除某用户的全部会话（禁用账号时使用）。
func (s *Store) DeleteByUser(ctx context.Context, uKey string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM cc_sessions WHERE u_key = ?", uKey)
	return err
}

// Cleanup 清理过期会话。
func (s *Store) Cleanup(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM cc_sessions WHERE s_expires < ?", time.Now())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CookieOptions 是下发会话 Cookie 时的选项。
type CookieOptions struct {
	Name   string
	Domain string
	Secure bool
	Path   string
	MaxAge time.Duration
}

// WriteCookie 把会话 id 写入 Cookie。
//
// 与 PHP 侧的行为差异（均为安全增强，不影响前端）：
//   - 增加 HttpOnly：前端本来就不读这个 Cookie（依赖 credentials:'same-origin'）；
//   - 增加 SameSite=Lax；
//   - 生产环境启用 Secure。
func WriteCookie(w http.ResponseWriter, opts CookieOptions, value string) {
	if opts.Path == "" {
		opts.Path = "/"
	}
	c := &http.Cookie{
		Name:     opts.Name,
		Value:    value,
		Path:     opts.Path,
		Domain:   opts.Domain,
		MaxAge:   int(opts.MaxAge.Seconds()),
		HttpOnly: true,
		Secure:   opts.Secure,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, c)
}

// ClearCookie 清除会话 Cookie（登出）。
func ClearCookie(w http.ResponseWriter, opts CookieOptions) {
	if opts.Path == "" {
		opts.Path = "/"
	}
	c := &http.Cookie{
		Name:     opts.Name,
		Value:    "",
		Path:     opts.Path,
		Domain:   opts.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   opts.Secure,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, c)
}

// ReadCookie 从请求中读取会话 id。
func ReadCookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
