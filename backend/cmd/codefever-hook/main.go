// Command codefever-hook 是 CodeFever Next 的 git 钩子可执行体。
//
// 它替换 misc/hooks/main 与 git-storage/hooks/main 里的 PHP CLI 脚本，
// 是"彻底移除 PHP 运行时"的第一块拼图 —— PHP 一删，钩子就无法执行，
// 所有 git push 都会被拒。
//
// 行为契约逐字复刻 misc/hooks/main（58 行 PHP），详见 internal/hook 包注释。
//
// 用法（git 会自动调用，不需要手工执行）：
//
//	misc/hooks/{pre-receive,post-receive,update} -> main -> <BASE>/bin/codefever-hook
package main

import (
	"context"
	"os"

	"codefever-next/backend/internal/hook"
)

func main() {
	// argv 完整透传（含 argv[0] 自身路径）—— 后端靠 basename 判定钩子名。
	// stdin 由 Execute 读到 EOF。
	os.Exit(hook.Execute(
		context.Background(),
		hook.ResolveBaseDir(),
		os.Args,
		os.Stdin,
		os.Stdout,
	))
}
