package repo

import (
	"strings"
	"testing"
)

// 本文件覆盖公开 API 访问令牌的**纯逻辑**部分（不需要数据库）。
//
// 需要真实 MySQL 的部分（创建 / 查询 / 吊销）见 access_token_integration_test.go；
// 这里的用例锁定的是"令牌明文与摘要的关系"这一安全不变量 ——
// 它决定了令牌能否被从数据库反推出来。

// TestGenerateTokenPlaintextUniqueness 断言令牌明文随机且互不相同。
func TestGenerateTokenPlaintextUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		plaintext, err := GenerateTokenPlaintext()
		if err != nil {
			t.Fatalf("生成令牌失败: %v", err)
		}
		if len(plaintext) != 43 {
			t.Errorf("令牌明文长度 = %d, want 43（32 字节 base64url 无 padding）", len(plaintext))
		}
		// base64url 字符集：不含 + / = 等 URL 不安全字符。
		if strings.ContainsAny(plaintext, "+/=") {
			t.Errorf("令牌明文含非 URL 安全字符: %q", plaintext)
		}
		if seen[plaintext] {
			t.Fatalf("令牌明文重复: %q", plaintext)
		}
		seen[plaintext] = true
	}
}

// TestHashTokenPlaintextIsDeterministic 断言摘要计算是确定性的。
//
// 鉴权时要用同一个明文算出同一个摘要去比对，
// 因此摘要必须是纯函数（无随机盐）。
func TestHashTokenPlaintextIsDeterministic(t *testing.T) {
	const plaintext = "dGVzdC10b2tlbi12YWx1ZQ"
	first := HashTokenPlaintext(plaintext)
	second := HashTokenPlaintext(plaintext)
	if first != second {
		t.Errorf("同一明文的摘要不一致: %s vs %s", first, second)
	}
	if len(first) != 64 {
		t.Errorf("摘要长度 = %d, want 64（SHA-256 十六进制）", len(first))
	}
	// 已知值锁定算法：必须是**纯 SHA-256**（无盐、无迭代）。
	// 一旦有人改成加盐哈希，这个用例会立刻失败 ——
	// 因为鉴权是拿传入明文现算摘要去比对，加盐后无法比对。
	const want = "3a5b86159ff0dcf7c71baab5c35895df8c898f7550bf2859e4d309c293b6ef7e"
	if first != want {
		t.Errorf("摘要 = %s, want %s（算法必须是纯 SHA-256）", first, want)
	}
}

// TestHashTokenPlaintextDistinguishes 断言不同明文得到不同摘要。
func TestHashTokenPlaintextDistinguishes(t *testing.T) {
	if HashTokenPlaintext("token-a") == HashTokenPlaintext("token-b") {
		t.Error("不同明文得到了相同摘要")
	}
}
