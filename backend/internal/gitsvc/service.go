// Package gitsvc 是 Git 服务层。
//
// 它取代 PHP 侧的两个文件：
//
//   - application/libraries/service/Utility/Command.php —— 把命令拼成 shell 字符串交给 exec()；
//   - application/libraries/service/Git/Command.php —— 把用户输入原样拼进同一条 shell 字符串。
//
// 与 PHP 的根本差异：
//
//  1. 全部命令以 **argv 数组** 交给 exec.CommandContext 执行，**永不经过 shell**。
//     因此 `;`、`|`、`>`、`$(...)` 等元字符在参数里只是普通字符，不具备命令注入能力。
//  2. 路径类参数统一前置 `--`，避免以 `-` 开头的值被 git 当成选项。
//  3. 退出码用 *exec.ExitError.ExitCode() 精确判定，把"成功但无输出"与"失败"区分开；
//     PHP 侧的 Command::batch() 丢弃了退出码，二者不可区分。
//
// 输出协议：PHP 用 --pretty="<分隔符>{JSON}<分隔符>" 把 git 输出编码成 JSON，
// 再用 Helper::parseJSON() 解回来。本包改用 git 原生的 --format 显式字段
// （记录分隔符 RS + 字段分隔符 NUL），语义等价但不必再手工拼 JSON ——
// 对外可见的 JSON 字段名保持不变（见 domain 包）。
package gitsvc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// defaultCommandTimeout 是单条 git 命令的兜底超时。
//
// HTTP 层没有给请求 ctx 设置 deadline；损坏仓库上的 log/blame/rev-list 或
// 临时 clone 可能挂住几十分钟，占满 goroutine 与 DB 连接。
//
// 性能整改 C5（2026-10）把这里从 10 分钟收紧到 60 秒：
//
//   - 上游 HTTP 服务器是 ReadTimeout 60s / WriteTimeout 120s
//     （cmd/codefever-api/main.go）。原先 10 分钟的 git 兜底超时**远长于**
//     服务端写超时：客户端早已被断开，服务端还在等一条 git 命令，
//     期间 goroutine 与（若在事务内）池连接都被白占。
//     60 秒留出了向客户端返回错误的时间窗，也让异常命令快速失败。
//   - 绝大多数命令是本地只读短操作（log / rev-parse / for-each-ref /
//     cat-file / count-objects），正常在毫秒级；60 秒只是异常兜底。
//
// 真正可能耗时的重命令（裸仓库克隆）用 longCommandTimeout 单独放宽，
// 见 runWithTimeout 与 CloneBare。
const defaultCommandTimeout = 60 * time.Second

// longCommandTimeout 是"可能真的要跑很久"的 git 命令的超时。
//
// 目前只有裸仓库克隆使用它（git clone --bare 是磁盘级全量复制，
// 多 GB 仓库可达分钟级）。放宽的前提是：自性能整改 C5 起，clone 不再在
// 数据库事务内执行，因此长时间运行不会再占着连接池
// （见 handlers/repository_write.go 的 fork）。
const longCommandTimeout = 10 * time.Minute

// commandContext 在调用方没有设置 deadline 时补一个兜底超时。
func commandContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return commandContextWith(ctx, defaultCommandTimeout)
}

// commandContextWith 在调用方没有设置 deadline 时补上指定兜底超时。
func commandContextWith(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

// ExitError 表示 git 以非零退出码结束。
//
// PHP 侧 Command::run() 只返回 TRUE/FALSE，stderr 被丢弃；
// 这里把退出码与 stderr 一并带回，便于定位失败原因。
type ExitError struct {
	Code   int
	Stderr string
}

// Error 实现 error 接口。
func (e *ExitError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		return fmt.Sprintf("git 退出码 %d", e.Code)
	}
	return fmt.Sprintf("git 退出码 %d: %s", e.Code, msg)
}

// ExitCodeOf 取出 git 的退出码；不是 ExitError 时返回 -1。
func ExitCodeOf(err error) int {
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return -1
}

// Service 是 Git 命令执行器。
type Service struct {
	// binPath 是 git 可执行文件路径（来自 env.yaml 的 cli.git）。
	binPath string
	// storageRoot 是裸仓库根目录（来自 env.yaml 的 path.git）。
	storageRoot string

	homeOnce sync.Once
	homeDir  string
	homeErr  error
}

// NewService 创建 Git 服务。
//
// binPath 通常取 env.yaml 的 cli.git（容器内是 /usr/bin/git）；
// storageRoot 通常取 env.yaml 的 path.git 解析后的绝对路径。
// 两者都不做硬编码，由调用方注入。
func NewService(binPath, storageRoot string) *Service {
	if binPath == "" {
		binPath = "git"
	}
	return &Service{binPath: binPath, storageRoot: storageRoot}
}

// BinPath 返回 git 可执行文件路径。
func (s *Service) BinPath() string { return s.binPath }

// StorageRoot 返回裸仓库根目录。
func (s *Service) StorageRoot() string { return s.storageRoot }

// Close 清理服务创建的临时 HOME 目录。
func (s *Service) Close() error {
	if s.homeDir == "" {
		return nil
	}
	err := os.RemoveAll(s.homeDir)
	s.homeDir = ""
	return err
}

// RepoPath 由仓库的 r_path 推导出裸仓库的绝对路径。
//
// r_path 形如 "/de7347a84f6236f06b0356ce6668c27d"，对应
// <storageRoot>/<32 位 hex>。这里同时做**路径穿越校验**：
// 解析后的路径必须落在 storageRoot 之内，否则拒绝 ——
// 因为 safe.directory 是按目录放行的，路径必须可信。
func (s *Service) RepoPath(rPath string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(rPath), "/")
	if trimmed == "" {
		return "", errors.New("仓库路径为空")
	}
	// 拒绝 Windows 与 POSIX 两种分隔符混用的穿越写法。
	if strings.ContainsAny(trimmed, `\`) {
		return "", fmt.Errorf("仓库路径非法: %q", rPath)
	}
	for _, seg := range strings.Split(trimmed, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", fmt.Errorf("仓库路径非法: %q", rPath)
		}
	}

	root, err := filepath.Abs(s.storageRoot)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(trimmed))
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("仓库路径越界: %q", rPath)
	}
	return target, nil
}

// run 在裸仓库目录里执行一条 git 命令，返回标准输出。
//
// 关键点：argv 数组直传，**不经过 shell**；用户输入即使含有
// `; rm -rf / #` 也只是一个普通参数。
func (s *Service) run(ctx context.Context, repoPath string, args ...string) (string, error) {
	return s.runWithTimeout(ctx, defaultCommandTimeout, repoPath, args...)
}

// runWithTimeout 与 run 相同，但使用调用方指定的兜底超时。
//
// 仅用于确实需要放宽的命令（当前只有裸仓库克隆）；其余命令一律走 run，
// 以保证默认 60 秒的异常兜底不被绕过。
func (s *Service) runWithTimeout(ctx context.Context, timeout time.Duration, repoPath string, args ...string) (string, error) {
	ctx, cancel := commandContextWith(ctx, timeout)
	defer cancel()

	env, err := s.buildEnv()
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, s.binPath, args...)
	cmd.Dir = repoPath
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	out := stdout.String()
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			return out, &ExitError{Code: ee.ExitCode(), Stderr: stderr.String()}
		}
		return out, runErr
	}
	return out, nil
}

// buildEnv 构造 git 子进程的环境变量。
//
// LANG=en_US.UTF-8 复刻 PHP 侧 Command::run() 的 `export LANG=en_US.UTF-8 &&`。
//
// HOME 指向服务自建的受管目录：git ≥ 2.35.2（以及 Debian 对 2.20.1 的安全回补）
// 在仓库属主与当前进程用户不一致时会拒绝操作仓库，报
// "detected dubious ownership"。PHP 侧靠 php-fpm 以 git 用户运行来规避，
// Go 后端不一定以该用户运行，因此这里显式声明 safe.directory。
// 该放宽以"仓库路径必须落在 storageRoot 之内"为前提（见 RepoPath）。
func (s *Service) buildEnv() ([]string, error) {
	home, err := s.gitHome()
	if err != nil {
		return nil, err
	}

	env := os.Environ()
	out := make([]string, 0, len(env)+2)
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "HOME="),
			strings.HasPrefix(kv, "LANG="),
			strings.HasPrefix(kv, "LC_ALL="),
			strings.HasPrefix(kv, "GIT_DIR="),
			strings.HasPrefix(kv, "GIT_WORK_TREE="),
			strings.HasPrefix(kv, "GIT_INDEX_FILE="):
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "HOME="+home, "LANG=en_US.UTF-8")
	return out, nil
}

// gitHome 惰性创建受管的 HOME 目录，并在其中写入 safe.directory 声明。
func (s *Service) gitHome() (string, error) {
	s.homeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "codefever-gitsvc-home-")
		if err != nil {
			s.homeErr = err
			return
		}
		const gitConfig = "[safe]\n\tdirectory = *\n"
		if err := os.WriteFile(filepath.Join(dir, ".gitconfig"), []byte(gitConfig), 0o600); err != nil {
			_ = os.RemoveAll(dir)
			s.homeErr = err
			return
		}
		s.homeDir = dir
	})
	return s.homeDir, s.homeErr
}
