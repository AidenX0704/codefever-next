package gitssh

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"codefever-next/backend/internal/gateway"
)

// fakeAuth 是 AuthClient 的测试替身。
type fakeAuth struct {
	res *gateway.AuthSSHResult
	err error
	got gateway.AuthSSHRequest
}

func (f *fakeAuth) AuthSSH(_ context.Context, req gateway.AuthSSHRequest) (*gateway.AuthSSHResult, error) {
	f.got = req
	return f.res, f.err
}

// newTestRunner 构造不触达真实 git / 后端的 Runner。
func newTestRunner(auth AuthClient) *Runner {
	return &Runner{
		Auth:     auth,
		LookPath: func(string) (string, error) { return "/usr/bin/git-receive-pack", nil },
		RunGit: func(_ context.Context, _ string, argv, env []string, _ io.Reader, _, _ io.Writer) int {
			// 由用例通过闭包断言；这里返回 0 表示成功。
			return 0
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// TestRunRejectsShell 验证非 git 命令一律拒绝。
func TestRunRejectsShell(t *testing.T) {
	r := newTestRunner(&fakeAuth{})
	var stderr bytes.Buffer
	code := r.Run(context.Background(), []string{"main", "bash", "-i", "u"}, nil, io.Discard, &stderr)
	if code == 0 {
		t.Fatal("交互式 shell 必须被拒绝")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Interaction shell is not allowed")) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// TestRunRejectsEmptyArgs 验证参数缺失时失败。
func TestRunRejectsEmptyArgs(t *testing.T) {
	r := newTestRunner(&fakeAuth{})
	if code := r.Run(context.Background(), []string{"main", "git-receive-pack"}, nil, io.Discard, io.Discard); code == 0 {
		t.Fatal("参数不足必须失败")
	}
}

// TestRunAuthFailure 验证鉴权失败时退出码非 0 且不回显内部细节。
func TestRunAuthFailure(t *testing.T) {
	r := newTestRunner(&fakeAuth{err: errors.New("db down")})
	var stderr bytes.Buffer
	code := r.Run(context.Background(), []string{"main", "git-receive-pack", "/g/r.git", "u"}, nil, io.Discard, &stderr)
	if code == 0 {
		t.Fatal("鉴权失败必须返回非 0")
	}
	if bytes.Contains(stderr.Bytes(), []byte("db down")) {
		t.Fatalf("不应回显内部错误: %q", stderr.String())
	}
}

// TestRunSuccess 验证成功路径会设置 PGYER_UID 并把仓库绝对路径传给 git。
func TestRunSuccess(t *testing.T) {
	auth := &fakeAuth{res: &gateway.AuthSSHResult{UID: "u|r", Path: "/data/git-storage/abc"}}
	r := newTestRunner(auth)

	var gotArgv, gotEnv []string
	r.RunGit = func(_ context.Context, bin string, argv, env []string, _ io.Reader, _, _ io.Writer) int {
		if bin != "/usr/bin/git-receive-pack" {
			t.Fatalf("bin = %q", bin)
		}
		gotArgv = argv
		gotEnv = env
		return 7
	}

	code := r.Run(context.Background(), []string{"main", "git-receive-pack", "/g/r.git", "u"}, nil, io.Discard, io.Discard)
	if code != 7 {
		t.Fatalf("退出码 = %d, 期望透传 7", code)
	}
	if auth.got.UserID != "u" || auth.got.Repo != "/g/r.git" || auth.got.Action != "git-receive-pack" {
		t.Fatalf("鉴权请求 = %+v", auth.got)
	}
	if len(gotArgv) != 2 || gotArgv[0] != "git-receive-pack" || gotArgv[1] != "/data/git-storage/abc" {
		t.Fatalf("argv = %#v", gotArgv)
	}
	found := false
	for _, kv := range gotEnv {
		if kv == "PGYER_UID=u|r" {
			found = true
		}
	}
	if !found {
		t.Fatal("环境变量缺少 PGYER_UID")
	}
}
