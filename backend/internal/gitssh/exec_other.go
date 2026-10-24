//go:build !linux

package gitssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// runGit 是非 Linux 平台的兜底实现（本地开发 / 单元测试用）。
//
// 生产环境是 Linux 容器；Windows/macOS 上没有 syscall.Exec 的等价语义，
// 这里用普通子进程并透传退出码。
func runGit(ctx context.Context, bin string, argv, env []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(argv) == 0 {
		fmt.Fprintln(stderr, "fatal: empty git command")
		return 1
	}
	cmd := exec.CommandContext(ctx, bin, argv[1:]...)
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "fatal: run %s failed: %v\n", argv[0], err)
		return 1
	}
	return 0
}
