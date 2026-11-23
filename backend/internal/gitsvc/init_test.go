package gitsvc

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitBareWithFiles(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("本机未安装 git，跳过测试")
	}

	root := t.TempDir()
	svc := NewService(gitBin, root)
	defer svc.Close()

	repoPath := filepath.Join(root, "demo-init")
	err = svc.InitBareWithFiles(context.Background(), repoPath, InitOptions{
		DefaultBranch:   "main",
		InitReadme:      true,
		RepoDisplayName: "Demo Service",
		RepoDescription: "Internal microservice",
		Gitignore:       "go",
		AuthorName:      "alice",
		AuthorEmail:     "alice@example.com",
	})
	if err != nil {
		t.Fatalf("InitBareWithFiles 失败: %v", err)
	}

	branches, err := svc.Branches(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("Branches 失败: %v", err)
	}
	if len(branches) != 1 || branches[0] != "main" {
		t.Fatalf("分支列表 = %v, want [main]", branches)
	}

	commit, err := svc.LastCommit(context.Background(), repoPath, "main", "")
	if err != nil || commit == nil {
		t.Fatalf("LastCommit 失败: %v", err)
	}
	if commit.Commit != "Initial commit" {
		t.Errorf("提交说明 = %q, want %q", commit.Commit, "Initial commit")
	}
	if commit.Email != "alice@example.com" {
		t.Errorf("提交邮箱 = %q, want %q", commit.Email, "alice@example.com")
	}

	out, err := svc.run(context.Background(), repoPath, "ls-tree", "--name-only", "main")
	if err != nil {
		t.Fatalf("ls-tree 失败: %v", err)
	}
	if !strings.Contains(out, "README.md") || !strings.Contains(out, ".gitignore") {
		t.Errorf("初始树缺少文件: %q", out)
	}
}

func TestInitBareWithFilesEmptyDefaultBranch(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("本机未安装 git，跳过测试")
	}

	root := t.TempDir()
	svc := NewService(gitBin, root)
	defer svc.Close()

	repoPath := filepath.Join(root, "demo-empty")
	err = svc.InitBareWithFiles(context.Background(), repoPath, InitOptions{
		DefaultBranch: "develop",
	})
	if err != nil {
		t.Fatalf("InitBareWithFiles 空仓失败: %v", err)
	}

	head, err := svc.run(context.Background(), repoPath, "symbolic-ref", "HEAD")
	if err != nil {
		t.Fatalf("symbolic-ref HEAD 失败: %v", err)
	}
	if strings.TrimSpace(head) != "refs/heads/develop" {
		t.Errorf("HEAD = %q, want refs/heads/develop", strings.TrimSpace(head))
	}
}
