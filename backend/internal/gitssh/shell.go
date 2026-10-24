// Package gitssh 实现 Git over SSH 的 forced command。
//
// sshd 通过 authorized_keys 的 command="..." 调用本程序：
//
//	codefever-ssh-gateway git-receive-pack '<group>/<repo>.git' <u_key>
//
// 流程：校验命令白名单 → 调用后端 /apiv1/internal/authSSH 拿到裸仓库绝对路径
// → 设置 PGYER_UID 后 exec 真正的 git-receive-pack / git-upload-pack。
//
// 与旧实现（ssh-gateway/shell/main.go）的差异：
//   - 不再把日志写到工作目录下的 message.txt（打不开就 panic，曾导致所有
//     SSH push 失败）；现在只写 stderr，出错信息直接回到客户端；
//   - 鉴权失败不再静默 exit 0，而是明确失败（exit 1），避免 git 客户端误判；
//   - 鉴权 HTTP 客户端有超时与连接池；
//   - Linux 上使用 syscall.Exec 直接替换进程，信号（客户端断开）能正确传递
//     给 git，不会留下孤儿进程。
package gitssh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/gateway"
)

// allowedCommandPattern 是允许通过 forced command 执行的 git 命令白名单。
var allowedCommandPattern = regexp.MustCompile(`^git-(receive|upload)-pack$`)

// AuthClient 是 authSSH 的最小接口，便于单元测试注入。
type AuthClient interface {
	AuthSSH(ctx context.Context, req gateway.AuthSSHRequest) (*gateway.AuthSSHResult, error)
}

// Runner 是 forced command 的执行器。
type Runner struct {
	// Auth 调用后端鉴权。
	Auth AuthClient
	// LookPath 解析 git 命令的绝对路径（测试可替换）。
	LookPath func(string) (string, error)
	// RunGit 执行 git 命令（测试可替换；默认实现见 exec_*.go）。
	RunGit func(ctx context.Context, bin string, argv, env []string, stdin io.Reader, stdout, stderr io.Writer) int
	// Logger 记录失败原因（不含凭据）。
	Logger *slog.Logger
}

// NewRunner 从 env.yaml 装配 Runner。
func NewRunner(baseDir string, logger *slog.Logger) (*Runner, error) {
	env, err := config.LoadEnv(baseDir)
	if err != nil {
		return nil, err
	}
	client := gateway.NewClient(env, logger)
	if err := client.ValidateSSH(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{Auth: client, LookPath: exec.LookPath, RunGit: runGit, Logger: logger}, nil
}

// Run 执行一次 forced command，返回进程退出码。
//
// args 是完整的 os.Args（含 argv[0]）。stdin/stdout/stderr 用于非 Linux
// 平台的子进程转发；Linux 下 syscall.Exec 直接接管这些文件描述符。
func (r *Runner) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 4 || !allowedCommandPattern.MatchString(args[1]) {
		// 交互式 shell / 任意命令一律拒绝。
		fmt.Fprintln(stderr, "[CodeFever Next]: Interaction shell is not allowed.")
		return 1
	}

	command, repoRef, userKey := args[1], args[2], args[3]
	if strings.TrimSpace(userKey) == "" || strings.TrimSpace(repoRef) == "" {
		fmt.Fprintln(stderr, "[CodeFever Next]: Missing repository or user key.")
		return 1
	}

	res, err := r.Auth.AuthSSH(ctx, gateway.AuthSSHRequest{
		UserID: userKey,
		Repo:   repoRef,
		Action: command,
	})
	if err != nil {
		if r.Logger != nil {
			r.Logger.Debug("SSH 网关鉴权失败", "repo", repoRef, "action", command, "error", err)
		}
		// 不回显内部细节（数据库地址、业务码等），只给 git 客户端一个明确的失败。
		fmt.Fprintln(stderr, "fatal: Could not read from remote repository.")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "Please make sure you have the correct access rights")
		fmt.Fprintln(stderr, "and the repository exists.")
		return 1
	}
	if res.Path == "" {
		fmt.Fprintln(stderr, "fatal: internal error: empty repository path.")
		return 1
	}

	bin, err := r.LookPath(command)
	if err != nil {
		if r.Logger != nil {
			r.Logger.Error("找不到 git 命令", "command", command, "error", err)
		}
		fmt.Fprintf(stderr, "fatal: git command not found: %s\n", command)
		return 1
	}

	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "PGYER_UID=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "PGYER_UID="+res.UID)
	return r.RunGit(ctx, bin, []string{command, res.Path}, env, stdin, stdout, stderr)
}

// runGit 的 Linux 实现在 exec_linux.go，其它平台在 exec_other.go。
