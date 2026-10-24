//go:build linux

package gitssh

import (
	"context"
	"fmt"
	"io"
	"syscall"
)

// runGit 在 Linux 上直接 exec git 命令。
//
// 直接替换进程比 fork+wait 更好：sshd 关闭连接时发送的信号会直接落到 git
// 进程上，不会出现"网关退出、git 仍在后台跑"的孤儿进程；退出码也由内核
// 原样传给 ssh 客户端。
func runGit(_ context.Context, bin string, argv, env []string, _ io.Reader, _, stderr io.Writer) int {
	if err := syscall.Exec(bin, argv, env); err != nil {
		fmt.Fprintf(stderr, "fatal: exec %s failed: %v\n", argv[0], err)
		return 1
	}
	return 0 // 不可达
}
