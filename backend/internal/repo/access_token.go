package repo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
)

// AccessTokenRepo 负责 cc_access_tokens 的读写。
//
// 安全模型：
//   - **只存 SHA-256**，明文仅在创建时返回一次；
//   - 校验时对传入明文取 SHA-256 后按唯一索引比对；
//   - 令牌 ≡ 用户（权限继承所属用户），scope 列预留但当前恒为空串。
type AccessTokenRepo struct {
	db *sql.DB
}

// NewAccessTokenRepo 创建访问令牌仓储。
func NewAccessTokenRepo(db *sql.DB) *AccessTokenRepo { return &AccessTokenRepo{db: db} }

// accessTokenColumnsTpl 是 cc_access_tokens 的列清单。
const accessTokenColumnsTpl = "at_key, u_key, at_name, at_sha256, at_scope, " +
	"CAST(at_last_used AS CHAR), CAST(at_created AS CHAR), CAST(at_updated AS CHAR), at_status"

// GenerateTokenPlaintext 生成一个高熵的令牌明文。
//
// 32 字节随机数 → base64url（无 padding），共 43 个字符。
// 明文只在这里生成一次，调用方负责在创建响应里返回给用户。
func GenerateTokenPlaintext() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashTokenPlaintext 计算令牌明文的 SHA-256 十六进制摘要。
func HashTokenPlaintext(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// Create 创建一个访问令牌。
//
// 返回值包含令牌主键与**明文** —— 明文只在此刻存在一次，
// 数据库里只保存它的 SHA-256。调用方必须把明文透传给用户。
func (r *AccessTokenRepo) Create(ctx context.Context, uKey, name string) (*domain.AccessToken, string, error) {
	if uKey == "" || name == "" {
		return nil, "", ErrNotFound
	}
	plaintext, err := GenerateTokenPlaintext()
	if err != nil {
		return nil, "", err
	}
	now := time.Now()
	token := &domain.AccessToken{
		Key:     contract.NewKey(),
		UKey:    uKey,
		Name:    name,
		SHA256:  HashTokenPlaintext(plaintext),
		Scope:   "",
		Created: now.Format("2006-01-02 15:04:05"),
		Updated: now.Format("2006-01-02 15:04:05"),
		Status:  domain.CommonStatusNormal,
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO cc_access_tokens (at_key, u_key, at_name, at_sha256, at_scope, at_status, at_created, at_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		token.Key, token.UKey, token.Name, token.SHA256, token.Scope, token.Status, now, now)
	if err != nil {
		return nil, "", err
	}
	return token, plaintext, nil
}

// BySHA256 按令牌摘要查找一条有效令牌。
//
// 找不到时返回 ErrNotFound —— 调用方据此返回 401。
func (r *AccessTokenRepo) BySHA256(ctx context.Context, sha string) (*domain.AccessToken, error) {
	if sha == "" {
		return nil, ErrNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+accessTokenColumnsTpl+`
		FROM cc_access_tokens WHERE at_sha256 = ? AND at_status = ? LIMIT 1`,
		sha, domain.CommonStatusNormal)
	return scanAccessToken(row)
}

// ByKey 按主键读取一条令牌（用于列表页与吊销）。
func (r *AccessTokenRepo) ByKey(ctx context.Context, atKey string) (*domain.AccessToken, error) {
	if atKey == "" {
		return nil, ErrNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+accessTokenColumnsTpl+`
		FROM cc_access_tokens WHERE at_key = ? LIMIT 1`, atKey)
	return scanAccessToken(row)
}

// List 列出某个用户的全部有效令牌（按创建时间倒序）。
func (r *AccessTokenRepo) List(ctx context.Context, uKey string) ([]domain.AccessToken, error) {
	if uKey == "" {
		return []domain.AccessToken{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+accessTokenColumnsTpl+`
		FROM cc_access_tokens WHERE u_key = ? AND at_status = ? ORDER BY at_created DESC`,
		uKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.AccessToken{}
	for rows.Next() {
		var t domain.AccessToken
		if err := rows.Scan(&t.Key, &t.UKey, &t.Name, &t.SHA256, &t.Scope,
			&t.LastUsed, &t.Created, &t.Updated, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Touch 更新令牌的最后使用时间。
//
// 失败**不阻断**请求 —— 这只是统计信息，写不进去不应让调用方拿不到令牌。
func (r *AccessTokenRepo) Touch(ctx context.Context, atKey string) error {
	if atKey == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE cc_access_tokens SET at_last_used = ? WHERE at_key = ?", time.Now(), atKey)
	return err
}

// Delete 吊销一个令牌（软删除，与仓库既有约定一致）。
//
// 只能吊销**自己名下**的令牌 —— uKey 不匹配时返回 ErrNotFound，
// 这样调用方不会泄露"该令牌存在但不属于我"这一事实。
func (r *AccessTokenRepo) Delete(ctx context.Context, uKey, atKey string) error {
	if uKey == "" || atKey == "" {
		return ErrNotFound
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE cc_access_tokens SET at_status = ?, at_deleted = ?
		WHERE at_key = ? AND u_key = ? AND at_status = ?`,
		domain.CommonStatusDelete, time.Now(), atKey, uKey, domain.CommonStatusNormal)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// NameExists 判断用户在某个名字下是否已有令牌（创建时查重）。
func (r *AccessTokenRepo) NameExists(ctx context.Context, uKey, name string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM cc_access_tokens
		WHERE u_key = ? AND at_name = ? AND at_status = ?`,
		uKey, name, domain.CommonStatusNormal).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// scanAccessToken 扫描一行 cc_access_tokens。
func scanAccessToken(row *sql.Row) (*domain.AccessToken, error) {
	var t domain.AccessToken
	err := row.Scan(&t.Key, &t.UKey, &t.Name, &t.SHA256, &t.Scope,
		&t.LastUsed, &t.Created, &t.Updated, &t.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}
