package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindUpwards 验证从子目录向上定位 env.yaml。
func TestFindUpwards(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "env.yaml"), []byte("env: testing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "bin", "nested")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	got, ok := FindUpwards(child)
	if !ok {
		t.Fatal("FindUpwards 未找到 env.yaml")
	}
	if got != root {
		t.Fatalf("FindUpwards = %q, 期望 %q", got, root)
	}
}

// TestFindUpwardsMissing 验证找不到时返回 false，而不是死循环。
func TestFindUpwardsMissing(t *testing.T) {
	dir := t.TempDir()
	if _, ok := FindUpwards(dir); ok {
		t.Fatal("FindUpwards 在空目录里不应返回 true")
	}
}

// TestResolveEnvOverride 验证环境变量优先级最高。
func TestResolveEnvOverride(t *testing.T) {
	want := t.TempDir()
	t.Setenv("CODEFEVER_BASE_DIR", want)
	if got := Resolve(); got != want {
		t.Fatalf("Resolve = %q, 期望 %q", got, want)
	}
}
