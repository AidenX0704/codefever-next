package gitsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestEnsureHEADPrefersMain 复现并锁定 clone 的 HEAD bug：
// 裸仓库由 git init --bare 创建（HEAD -> master），首次推送 main 后，
// HEAD 必须被修正到 main，否则 copy 会 warning 且工作区为空。
func TestEnsureHEADPrefersMain(t *testing.T) {
	if !gitAvailable() {
		t.Skip("环境里没有 git")
	}

	src := t.TempDir()
	gitRun(t, src, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, src, "add", "README.md")
	gitRun(t, src, "commit", "-q", "-m", "first")
	gitRun(t, src, "remote", "add", "origin", src)

	bare := filepath.Join(t.TempDir(), "repo.git")
	gitRun(t, filepath.Dir(bare), "init", "-q", "--bare", bare)
	gitRun(t, src, "push", "-q", bare, "main")

	// 前置条件：裸仓库 HEAD 仍指向 master。
	if got := gitRun(t, bare, "symbolic-ref", "--short", "HEAD"); got != "master" {
		t.Fatalf("前置条件不成立：HEAD = %q", got)
	}

	svc := NewService("git", filepath.Join(t.TempDir(), "git-storage"))
	defer svc.Close()

	chosen, err := svc.EnsureHEAD(context.Background(), bare, "")
	if err != nil {
		t.Fatalf("EnsureHEAD 失败: %v", err)
	}
	if chosen != "main" {
		t.Fatalf("选择的分支 = %q, 期望 main", chosen)
	}
	if got := gitRun(t, bare, "symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Fatalf("HEAD = %q, 期望 main", got)
	}
}

// TestEnsureHEADUsesPreferred 确认 DB 默认分支优先于 main/master 探测。
func TestEnsureHEADUsesPreferred(t *testing.T) {
	if !gitAvailable() {
		t.Skip("环境里没有 git")
	}
	src := t.TempDir()
	gitRun(t, src, "init", "-q", "-b", "develop")
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, src, "add", "README.md")
	gitRun(t, src, "commit", "-q", "-m", "first")

	bare := filepath.Join(t.TempDir(), "repo.git")
	gitRun(t, filepath.Dir(bare), "init", "-q", "--bare", bare)
	gitRun(t, src, "push", "-q", bare, "develop")

	svc := NewService("git", filepath.Join(t.TempDir(), "git-storage"))
	defer svc.Close()
	chosen, err := svc.EnsureHEAD(context.Background(), bare, "develop")
	if err != nil {
		t.Fatalf("EnsureHEAD 失败: %v", err)
	}
	if chosen != "develop" {
		t.Fatalf("选择的分支 = %q, 期望 develop", chosen)
	}
}
