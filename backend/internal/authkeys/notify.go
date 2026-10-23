package authkeys

import (
	"errors"

	"codefever-next/backend/internal/config"
)

// Notify 是旧 API 的兼容入口。
//
// Deprecated: PHP 守护进程已下线，请直接使用 Writer.Add / Writer.Remove。
//
// 保留它是为了不破坏 authkeys 包的公开 API。语义与 PHP 侧 FIFO 的分发规则
// 完全一致（misc/modify_authorized_keys.php:97-101）：
//
//   - 载荷带 user 字段 → 追加一行（PHP:106-123）；
//   - 否则载荷带 key 字段 → 按子串删除一行（PHP:125-140）。
//
// pipeName / pidFile 两个参数是旧 FIFO 协议的产物（FIFO 路径与守护进程 PID 文件），
// 现在**被忽略**；旧调用点已全部迁移到 Writer。
func Notify(baseDir, pipeName, pidFile string, p Payload) error {
	env, err := config.LoadEnv(baseDir)
	if err != nil {
		return err
	}

	w := NewWriter(baseDir, env.Users.Git, env.CLI.Git)
	switch {
	case p.User != "":
		return w.Add(p.User, p.Type, p.Key, p.Name)
	case p.Key != "":
		return w.Remove(p.Key)
	default:
		return errors.New("通知内容既没有 user 也没有 key")
	}
}
