// Command codefever-ssh-gateway 是 Git over SSH 的 forced command。
//
// sshd 依据 authorized_keys 里的 command="..." 调用本程序，参数形态固定：
//
//	codefever-ssh-gateway git-receive-pack '<group>/<repo>.git' <u_key>
//
// 它不常驻、不接受任意参数，因此没有 flag。应用根目录通过可执行文件位置
// 向上查找 env.yaml（见 internal/appdir），也支持 CODEFEVER_BASE_DIR 覆盖。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"codefever-next/backend/internal/appdir"
	"codefever-next/backend/internal/gitssh"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	slog.SetDefault(logger)

	runner, err := gitssh.NewRunner(appdir.Resolve(), logger)
	if err != nil {
		// 配置错误发生在 SSH 会话里，直接告诉客户端；不要吞掉后 exit 0。
		fmt.Fprintf(os.Stderr, "fatal: CodeFever SSH gateway configuration error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(runner.Run(context.Background(), os.Args, os.Stdin, os.Stdout, os.Stderr))
}
