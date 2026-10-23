package config

import (
	"os"
	"path/filepath"
	"testing"

	"codefever-next/backend/internal/auth"
)

// writeEnv 写出一份最小可用的 env.yaml 与 config.yaml。
func writeEnv(t *testing.T, dir, envExtra string) {
	t.Helper()
	env := "env: 'development'\n" +
		"releaseNumber: '00000000000001'\n" +
		"mysql:\n  host: 127.0.0.1\n  port: 3306\n  username: root\n  password: 123456\n  db: codefever\n" +
		"session:\n  domain: ''\n  name: codefever_community\n  key: testkey\n" +
		"totp:\n  salt: testsalt\n" +
		"path:\n  file: file-storage\n  git: git-storage\n" +
		"cli:\n  git: git\n  shell: git-shell\n  http: git-http-backend\n" +
		"users:\n  www: git\n  git: git\n" +
		"gateway:\n  token: tok\n" +
		envExtra
	if err := os.WriteFile(filepath.Join(dir, "env.yaml"), []byte(env), 0o600); err != nil {
		t.Fatalf("写 env.yaml 失败: %v", err)
	}
	site := "host: 'http://localhost'\nallowRegister: true\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(site), 0o600); err != nil {
		t.Fatalf("写 config.yaml 失败: %v", err)
	}
}

// TestPasswordHashFormatDefaultIsLegacy 锁定共存期的核心默认值：
// 未显式配置时必须是 legacy（PHP 可验证），否则回滚目标会失效。
func TestPasswordHashFormatDefaultIsLegacy(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, "")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got := cfg.PasswordHashFormat(); got != auth.HashFormatLegacy {
		t.Fatalf("默认哈希格式 = %q, want %q", got, auth.HashFormatLegacy)
	}
}

// TestPasswordHashFormatFromEnvYAML 确认 env.yaml 里可以就地配置。
func TestPasswordHashFormatFromEnvYAML(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, "auth:\n  hash_format: argon2id\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got := cfg.PasswordHashFormat(); got != auth.HashFormatArgon2id {
		t.Fatalf("哈希格式 = %q, want %q", got, auth.HashFormatArgon2id)
	}
}

// TestPasswordHashFormatEnvOverride 确认环境变量优先于 env.yaml。
func TestPasswordHashFormatEnvOverride(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, "auth:\n  hash_format: legacy\n")

	t.Setenv("CODEFEVER_PASSWORD_HASH", "argon2id")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got := cfg.PasswordHashFormat(); got != auth.HashFormatArgon2id {
		t.Fatalf("环境变量覆盖后哈希格式 = %q, want %q", got, auth.HashFormatArgon2id)
	}
}

// TestPasswordHashFormatUnknownFallsBackToLegacy 确认无法识别的取值回落到 legacy。
func TestPasswordHashFormatUnknownFallsBackToLegacy(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, "auth:\n  hash_format: scrypt\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got := cfg.PasswordHashFormat(); got != auth.HashFormatLegacy {
		t.Fatalf("未知格式应回落到 legacy, got %q", got)
	}
}
