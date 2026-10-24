// Package appdir 负责定位应用根目录（env.yaml 所在目录）。
//
// 三个以不同方式启动的进程都需要它：
//
//	codefever-api        —— 通过 -base flag / CODEFEVER_BASE_DIR
//	codefever-http-gateway —— 通过 -base flag / CODEFEVER_BASE_DIR
//	codefever-ssh-gateway  —— **没有参数通道**（authorized_keys 的 forced command
//	                          只传 git 命令、仓库名与 u_key），只能从可执行文件
//	                          位置反推，因此这里提供统一的向上查找逻辑。
//
// 容器内的默认根目录保持 /data/www/codefever-community 不变，以兼容既有镜像、
// 数据卷与 env.yaml 中的相对路径。
package appdir

import (
	"os"
	"path/filepath"
	"strings"
)

// Default 是容器内的默认应用根目录。
//
// 该值与 Dockerfile 的 WORKDIR、supervisor 的 -base 参数保持一致。
const Default = "/data/www/codefever-community"

// Resolve 返回应用根目录。
//
// 优先级：
//  1. 环境变量 CODEFEVER_BASE_DIR（显式覆盖，测试与本地开发用）；
//  2. 从可执行文件真实路径向上逐级查找 env.yaml；
//  3. 容器默认根目录（若其中存在 env.yaml）；
//  4. 可执行文件所在目录的上一级（本地 `go run` / `go build` 的兜底）。
func Resolve() string {
	if v := strings.TrimSpace(os.Getenv("CODEFEVER_BASE_DIR")); v != "" {
		return v
	}

	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		// Linux 下 /proc/self/exe 已经是真实路径；其它平台用 EvalSymlinks 兜底。
		// 这样即使 ssh 的 forced command 走的是兼容软链
		// <base>/ssh-gateway/shell/main，也能解析到 <base>/bin/codefever-ssh-gateway
		// 并正确反推出 <base>。
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		exeDir = filepath.Dir(exe)
		if dir, ok := FindUpwards(exeDir); ok {
			return dir
		}
	}

	if fileExists(filepath.Join(Default, "env.yaml")) {
		return Default
	}
	if exeDir != "" {
		return filepath.Dir(exeDir)
	}
	return Default
}

// FindUpwards 从 dir 开始逐级向上查找 env.yaml，返回其所在目录。
func FindUpwards(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	for {
		if fileExists(filepath.Join(dir, "env.yaml")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// fileExists 判断路径是否存在且是普通文件。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
