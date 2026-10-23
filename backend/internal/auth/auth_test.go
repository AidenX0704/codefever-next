package auth

import (
	"crypto/md5"
	"encoding/base32"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestLegacyHash(t *testing.T) {
	// 该值来自 misc/db_data.sql 中种子管理员的口令哈希，
	// 必须与 PHP 侧 md5(md5('123456')) 逐字一致。
	const want = "14e1b600b1fd579f47433b88e8d85291"
	if got := LegacyHash("123456"); got != want {
		t.Fatalf("LegacyHash(\"123456\") = %q, want %q", got, want)
	}
}

// TestHashPasswordForFormatLegacyIsPHPCompatible 锁定共存期的核心不变量：
// legacy 格式写出的哈希必须与 PHP 侧 md5(md5(x)) **逐字相同**，
// 否则 PHP 回滚目标将无法验证该账号。
func TestHashPasswordForFormatLegacyIsPHPCompatible(t *testing.T) {
	got, err := HashPasswordForFormat("s3cret-pass", HashFormatLegacy)
	if err != nil {
		t.Fatalf("HashPasswordForFormat 失败: %v", err)
	}
	if want := LegacyHash("s3cret-pass"); got != want {
		t.Fatalf("legacy 格式哈希 = %q, want %q", got, want)
	}
	// PHP 侧的校验方式：md5(md5($password)) === u_password
	// 这里用标准库独立复刻一遍，避免"用被测函数验证被测函数"。
	first := md5.Sum([]byte("s3cret-pass"))
	second := md5.Sum([]byte(hex.EncodeToString(first[:])))
	if want := hex.EncodeToString(second[:]); got != want {
		t.Fatalf("legacy 格式哈希 = %q, PHP 侧算出的 md5(md5(x)) = %q", got, want)
	}
}

// TestHashPasswordForFormatArgon2id 确认 argon2id 仍然可用（PHP 下线后切换）。
func TestHashPasswordForFormatArgon2id(t *testing.T) {
	got, err := HashPasswordForFormat("s3cret-pass", HashFormatArgon2id)
	if err != nil {
		t.Fatalf("HashPasswordForFormat 失败: %v", err)
	}
	if !strings.HasPrefix(got, "$argon2id$") {
		t.Fatalf("argon2id 格式哈希应以 $argon2id$ 开头, got %q", got)
	}
	if ok, _ := VerifyPassword(got, "s3cret-pass"); !ok {
		t.Fatal("argon2id 哈希应能校验通过")
	}
}

// TestParseHashFormatDefault 确认无法识别的配置回落到 legacy（更安全的一侧）。
func TestParseHashFormatDefault(t *testing.T) {
	cases := map[string]HashFormat{
		"":            HashFormatLegacy,
		"legacy":      HashFormatLegacy,
		"LEGACY":      HashFormatLegacy,
		" argon2id ":  HashFormatArgon2id,
		"argon2id":    HashFormatArgon2id,
		"scrypt":      HashFormatLegacy,
		"unknown-xxx": HashFormatLegacy,
	}
	for in, want := range cases {
		if got := ParseHashFormat(in); got != want {
			t.Fatalf("ParseHashFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVerifyPasswordLegacy(t *testing.T) {
	const stored = "14e1b600b1fd579f47433b88e8d85291"

	ok, upgrade := VerifyPassword(stored, "123456")
	if !ok {
		t.Fatal("期望校验通过")
	}
	if !upgrade {
		t.Fatal("旧哈希应标记为需要升级")
	}

	ok, _ = VerifyPassword(stored, "wrong-password")
	if ok {
		t.Fatal("错误口令不应通过")
	}
}

func TestHashPasswordRoundTrip(t *testing.T) {
	hashed, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if !strings.HasPrefix(hashed, "$argon2id$") {
		t.Fatalf("哈希格式不正确: %q", hashed)
	}

	ok, upgrade := VerifyPassword(hashed, "s3cret-pass")
	if !ok {
		t.Fatal("期望校验通过")
	}
	if upgrade {
		t.Fatal("argon2id 哈希不应标记为需要升级")
	}

	ok, _ = VerifyPassword(hashed, "other")
	if ok {
		t.Fatal("错误口令不应通过")
	}

	// 两次哈希的盐不同，因此结果不同。
	other, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if hashed == other {
		t.Fatal("不同盐应产生不同哈希")
	}
}

// TestTOTPRFC6238Vectors 用 RFC 6238 官方测试向量校验 TOTP 实现。
//
// 注意 RFC 的测试向量是 8 位数字，本实现的 Digits=6，因此这里用同一套
// secret 与时间点自行推导 6 位结果 —— 通过与 RFC 8 位结果的一致性来验证
// HOTP 核心算法正确。
func TestTOTPRFC6238Vectors(t *testing.T) {
	// RFC 6238 附录 B 使用的 secret："12345678901234567890"（ASCII）。
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))

	// RFC 6238 的 8 位期望值。
	rfcVectors := []struct {
		unix int64
		want string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}

	for _, v := range rfcVectors {
		totp := &TOTP{Secret: secret, Period: 30 * time.Second, Digits: 8, Window: 1}
		got, err := totp.codeForCounter(uint64(v.unix / 30))
		if err != nil {
			t.Fatalf("t=unix(%d) 计算失败: %v", v.unix, err)
		}
		if got != v.want {
			t.Errorf("t=unix(%d): got %q, want %q", v.unix, got, v.want)
		}
	}
}

func TestTOTPVerify(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	totp := &TOTP{Secret: secret, Period: 30 * time.Second, Digits: 6, Window: 1}

	at := time.Unix(1111111109, 0)
	code, err := totp.Code(at)
	if err != nil {
		t.Fatalf("生成验证码失败: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("验证码长度应为 6，实际 %d", len(code))
	}
	if !totp.Verify(code, at) {
		t.Fatal("当前时刻的验证码应校验通过")
	}
	if totp.Verify(code, at.Add(10*time.Minute)) {
		t.Fatal("超出时间窗的验证码不应通过")
	}
	if totp.Verify("000000", at) && code != "000000" {
		t.Fatal("错误验证码不应通过")
	}
}

func TestCompareToken(t *testing.T) {
	if !CompareToken("secret", "secret") {
		t.Fatal("相同令牌应通过")
	}
	if CompareToken("secret", "secrer") {
		t.Fatal("不同令牌不应通过")
	}
	if CompareToken("secret", "secret-longer") {
		t.Fatal("不同长度令牌不应通过")
	}
}

func TestGenerateEmailCode(t *testing.T) {
	at := time.Unix(1700000000, 0)
	code := GenerateEmailCode("user@example.com", at, "salt")
	if len(code) != 6 {
		t.Fatalf("验证码长度应为 6，实际 %d", len(code))
	}
	if !VerifyEmailCode("user@example.com", code, at, "salt") {
		t.Fatal("验证码应校验通过")
	}
	// 盐值或邮箱不同则验证码不同 —— 验证码与目标邮箱绑定。
	if VerifyEmailCode("other@example.com", code, at, "salt") {
		t.Fatal("验证码不应在其他邮箱上通过")
	}
	if VerifyEmailCode("user@example.com", code, at, "other-salt") {
		t.Fatal("验证码不应在其他盐值下通过")
	}
}
