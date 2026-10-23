//go:build unix

package authkeys

import (
	"os"
	"syscall"
)

// supported 报告当前平台是否支持写 authorized_keys。
//
// 容器内是 Linux（true）；Windows 开发机为 false —— 那里既没有
// /home/<git 用户>/.ssh，也没有 sshd，写了也没有意义。
const supported = true

// lockFile 对 authorized_keys 加排他锁。
//
// PHP 侧靠守护进程串行消费 FIFO 来避免并发写；Go 后端是常驻进程，
// 同一进程内用互斥锁串行化，跨进程（api 与 worker 都可能触发）再用 flock 兜底。
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlockFile 释放排他锁。
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// chown 设置属主/属组。
//
// Go 后端在容器内以 root 运行，因此可以 chown（对应 PHP:22-25）。
func chown(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}
