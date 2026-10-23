//go:build !unix

package authkeys

import "os"

// supported 报告当前平台是否支持写 authorized_keys。
//
// 非 POSIX 平台（Windows 开发机）为 false —— 那里既没有 /home/<git 用户>/.ssh，
// 也没有 sshd，调用方应当跳过同步而不是报错。
const supported = false

// lockFile 在不支持 flock 的平台上退化为空操作。
//
// 写操作由进程内互斥锁串行化；跨进程并发在开发机上不会发生。
func lockFile(f *os.File) error { return nil }

// unlockFile 同 lockFile，空操作。
func unlockFile(f *os.File) error { return nil }

// chown 在非 POSIX 平台上没有属主概念，直接跳过。
//
// 归属问题只在容器里重要（那里是 root + POSIX），开发机上不影响功能。
func chown(path string, uid, gid int) error { return nil }
