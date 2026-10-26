package repo

import (
	"context"
	"database/sql"
	"testing"

	"codefever-next/backend/internal/contract"
)

// 本文件覆盖访问令牌仓储的**真实数据库**行为。
//
// 需要真实 MySQL（涉及唯一索引、软删除与 JOIN 语义，mock 无法覆盖）：
//
//	$env:CODEFEVER_TEST_DSN = "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true"
//	go test ./internal/repo/ -run TestAccessTokenRepo -v
//
// 覆盖的关键不变量：
//   - 明文**不入库**：库里只有 SHA-256；
//   - 按摘要能找回令牌（鉴权主路径）；
//   - 吊销后按摘要**找不回来**（否则吊销形同虚设）；
//   - 不能吊销别人的令牌（u_key 不匹配时不产生任何影响）。

// TestAccessTokenRepoLifecycle 验证令牌的完整生命周期。
func TestAccessTokenRepoLifecycle(t *testing.T) {
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
	repo := NewAccessTokenRepo(db)

	// 造一个用户（令牌必须挂在真实用户上）。
	uKey := contract.NewKey()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO cc_users (u_key, u_email, u_password, u_name, u_register_ip, u_admin,
		                      u_notification_status, u_created, u_updated, u_status)
		VALUES (?, ?, 'x', ?, '127.0.0.1', 0, 0, NOW(), NOW(), 1)`,
		uKey, "token-probe-"+uKey+"@codefever.cn", "probe-"+uKey); err != nil {
		t.Fatalf("插入探测用户失败: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM cc_users WHERE u_key = ?", uKey)
	}()

	// 1) 创建令牌。
	token, plaintext, err := repo.Create(ctx, uKey, "ci-token")
	if err != nil {
		t.Fatalf("创建令牌失败: %v", err)
	}
	if plaintext == "" {
		t.Fatal("创建令牌应返回明文")
	}
	if token.SHA256 == plaintext {
		t.Fatal("明文不得原样存入 SHA256 字段")
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM cc_access_tokens WHERE at_key = ?", token.Key)
	}()

	// 2) 库里不能出现明文。
	var stored string
	if err := db.QueryRowContext(ctx,
		"SELECT at_sha256 FROM cc_access_tokens WHERE at_key = ?", token.Key).Scan(&stored); err != nil {
		t.Fatalf("读取令牌失败: %v", err)
	}
	if stored == plaintext {
		t.Error("数据库中出现了令牌明文")
	}
	if stored != HashTokenPlaintext(plaintext) {
		t.Errorf("库中摘要 = %s, want %s", stored, HashTokenPlaintext(plaintext))
	}

	// 3) 按摘要能找回令牌（鉴权主路径）。
	found, err := repo.BySHA256(ctx, HashTokenPlaintext(plaintext))
	if err != nil {
		t.Fatalf("按摘要查找令牌失败: %v", err)
	}
	if found.Key != token.Key || found.UKey != uKey {
		t.Errorf("查回的令牌不匹配: key=%s u_key=%s", found.Key, found.UKey)
	}

	// 4) 错误摘要查不到。
	if _, err := repo.BySHA256(ctx, HashTokenPlaintext("wrong-token")); err == nil {
		t.Error("错误摘要应当查不到令牌")
	}

	// 5) 列表能看到令牌。
	list, err := repo.List(ctx, uKey)
	if err != nil {
		t.Fatalf("列出令牌失败: %v", err)
	}
	foundInList := false
	for _, item := range list {
		if item.Key == token.Key {
			foundInList = true
		}
	}
	if !foundInList {
		t.Error("列表里应包含刚创建的令牌")
	}

	// 6) 名字重复应被察觉。
	exists, err := repo.NameExists(ctx, uKey, "ci-token")
	if err != nil {
		t.Fatalf("检查重名失败: %v", err)
	}
	if !exists {
		t.Error("重名检查应返回 true")
	}

	// 7) 不能吊销别人的令牌。
	otherKey := contract.NewKey()
	if err := repo.Delete(ctx, otherKey, token.Key); err == nil {
		t.Error("吊销他人令牌应当失败")
	}

	// 8) 吊销自己的令牌。
	if err := repo.Delete(ctx, uKey, token.Key); err != nil {
		t.Fatalf("吊销令牌失败: %v", err)
	}
	// 吊销后按摘要必须找不回来 —— 否则吊销形同虚设。
	if _, err := repo.BySHA256(ctx, HashTokenPlaintext(plaintext)); err == nil {
		t.Error("已吊销的令牌仍能被按摘要查到")
	}
	// 列表里也不该再出现。
	list, err = repo.List(ctx, uKey)
	if err != nil {
		t.Fatalf("列出令牌失败: %v", err)
	}
	for _, item := range list {
		if item.Key == token.Key {
			t.Error("已吊销的令牌仍出现在列表里")
		}
	}
}
