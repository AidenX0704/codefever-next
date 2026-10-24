// Package authkeys 负责把 SSH 公钥同步到 /home/<git 用户>/.ssh/authorized_keys。
//
// # 历史实现
//
// 原来由 PHP 守护进程（misc/modify_authorized_keys.php）承担：
//
//  1. PHP 侧（application/models/user_sshkey_model.php:100-140）把公钥变更
//     写进 FIFO（misc/modify_authorized_keys.child.pipe），并给守护进程发 SIGUSR1；
//  2. 守护进程读到 {"user":...} 就**追加一行**，读到 {"key":...} 就**按子串删一行**
//     （misc/modify_authorized_keys.php:97-101）。
//
// 守护进程随 PHP 运行时一起下线，因此本包改为由 Go 后端**直接写文件**，
// 语义与守护进程逐字一致：
//
//   - 追加一行：行格式固定（见 Writer.formatLine），字段间单个空格，行尾补 "\n"；
//   - 删除一行：按**公钥正文子串**匹配，拆行过滤后再用 "\n" 拼回；
//   - **不做全量重写** —— 文件里可能存在历史遗留行，全量重写会把它们抹掉，
//     与 PHP 守护进程的行为不符。
//
// # 规格来源
//
//	misc/modify_authorized_keys.php:12-26   路径、目录/文件创建与归属
//	misc/modify_authorized_keys.php:106-123 追加一行
//	misc/modify_authorized_keys.php:125-140 按子串删除一行
package authkeys

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	// defaultHomeRoot 是容器内的家目录根。
	defaultHomeRoot = "/home"

	// sshDirName 是 SSH 配置目录名（PHP:12）。
	sshDirName = ".ssh"

	// authorizedKeysName 是公钥文件名（PHP:13）。
	authorizedKeysName = "authorized_keys"

	// sshGatewayRel 是 ssh-gateway 可执行体相对应用根目录的路径。
	//
	// 历史上是 ssh-gateway/shell/main（PHP:114）。源码合并进 backend 后，
	// 规范位置改为 <base>/bin/codefever-ssh-gateway；镜像里同时保留
	// <base>/ssh-gateway/shell/main 软链，保证旧部署已写入的 authorized_keys
	// 行继续可用。
	sshGatewayRel = "bin/codefever-ssh-gateway"

	// sshDirMode 与 keyFileMode 是新建 .ssh / authorized_keys 的权限。
	//
	// PHP 时代用 0777 / 0666；这里收紧为 0700 / 0600（sshd StrictModes
	// 接受的标准权限）。既有文件不会被 chmod，保持原样。
	sshDirMode  = 0o700
	keyFileMode = 0o600
)

// ErrUnsupported 表示当前平台不支持 authorized_keys 同步协议。
//
// Deprecated: 保留仅为兼容旧 API；新代码请用 Writer。
var ErrUnsupported = errors.New("当前平台不支持 authorized_keys 同步协议")

// Payload 是公钥变更的内容。
//
// 字段与 PHP 侧 json_encode 的数组键逐字一致：
// user / type / key / name。
type Payload struct {
	User string `json:"user"`
	Type string `json:"type"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Writer 负责写 authorized_keys。
//
// 一个 Writer 对应一个部署实例：公钥文件是 <users.git 的家目录>/.ssh/authorized_keys。
// 家目录优先从系统用户数据库读取；读不到时回退 /home/<users.git>。
//
// 写操作由进程内互斥锁串行化；跨进程再用 flock 兜底（见 platform_unix.go）。
type Writer struct {
	homeDir string
	baseDir string
	gitUser string
	gitDir  string

	mu sync.Mutex
}

// NewWriter 创建 Writer。
//
//   - baseDir 是 env.yaml 所在目录；
//     ssh-gateway 可执行体位于 <baseDir>/bin/codefever-ssh-gateway。
//   - gitUser 是 env.yaml 的 users.git，即 authorized_keys 家目录的属主。
//   - cliGit 是 env.yaml 的 cli.git，取其**目录部分**拼进 PATH= 前缀（PHP:116）。
func NewWriter(baseDir, gitUser, cliGit string) *Writer {
	homeDir := filepath.Join(defaultHomeRoot, gitUser)
	if supported {
		if u, err := user.Lookup(gitUser); err == nil && strings.TrimSpace(u.HomeDir) != "" {
			homeDir = u.HomeDir
		}
	}
	return &Writer{
		homeDir: homeDir,
		baseDir: baseDir,
		gitUser: gitUser,
		gitDir:  phpDirname(cliGit),
	}
}

// newWriterWithHomeRoot 指定家目录根创建 Writer。
//
// 仅供单元测试使用：测试不能写真实的 /home/<用户>/.ssh。
func newWriterWithHomeRoot(homeRoot, baseDir, gitUser, cliGit string) *Writer {
	return &Writer{
		homeDir: filepath.Join(homeRoot, gitUser),
		baseDir: baseDir,
		gitUser: gitUser,
		gitDir:  phpDirname(cliGit),
	}
}

// Add 追加一条公钥。
//
// 逐字对应 misc/modify_authorized_keys.php:106-123。写出的行是：
//
//	command="PATH=$PATH:<cli.git 的目录> && <BASE>/bin/codefever-ssh-gateway $SSH_ORIGINAL_COMMAND <u_key>",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <type> <key> <name>
//
// 四个字段以**单个空格**连接，行尾补 "\n"，以追加方式写入。
//
// 安全约束：key / keyType / name 都会原样写进 authorized_keys。若其中含
// 换行，攻击者可以追加一条**不带 command= 选项**的公钥行，sshd 会为该键
// 提供 git 用户的交互式 shell（绕过 forced command）。因此这里在任何写入
// 之前做单行化校验；只拒绝异常输入，正常取值的字节序列保持不变。
func (w *Writer) Add(uKey, keyType, key, name string) error {
	if err := validateKeyFields(keyType, key, name); err != nil {
		return err
	}
	if err := w.ensure(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(w.keyFile(), os.O_WRONLY|os.O_APPEND|os.O_CREATE, keyFileMode)
	if err != nil {
		return fmt.Errorf("打开 authorized_keys 失败: %w", err)
	}
	defer f.Close()

	if err := lockFile(f); err != nil {
		return fmt.Errorf("锁定 authorized_keys 失败: %w", err)
	}
	defer unlockFile(f)

	if _, err := f.WriteString(w.formatLine(uKey, keyType, key, name)); err != nil {
		return fmt.Errorf("写入 authorized_keys 失败: %w", err)
	}
	return nil
}

// Remove 按**公钥正文子串**删除匹配的整行。
//
// 逐字对应 misc/modify_authorized_keys.php:125-140：
// 把文件按 "\n" 拆行，丢掉包含该 key 子串的行，再以 "\n" 拼接写回。
//
// 注意 PHP 这里**没有**在末尾补 "\n"（写回时用 implode 拼接，末尾的空元素
// 只在原文件以 "\n" 结尾时才保留）。这是既有文件的既有形态，必须逐字保持：
// sshd 两种形态都能接受，但回滚到 PHP 时 PHP 会再按同样的规则处理一次。
func (w *Writer) Remove(key string) error {
	if key == "" {
		return errors.New("待删除的公钥正文为空")
	}
	if strings.ContainsAny(key, "\r\n\x00") {
		return errors.New("公钥正文不能包含换行或 NUL")
	}
	if err := w.ensure(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 跨进程锁挂在独立的 .lock 文件上：写回使用 rename 原子替换，
	// 若锁挂在 authorized_keys 本身，rename 之后锁就失效了。
	lock, err := os.OpenFile(w.keyFile()+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("打开 authorized_keys 锁失败: %w", err)
	}
	defer lock.Close()
	if err := lockFile(lock); err != nil {
		return fmt.Errorf("锁定 authorized_keys 失败: %w", err)
	}
	defer unlockFile(lock)

	raw, err := os.ReadFile(w.keyFile())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 authorized_keys 失败: %w", err)
	}

	// PHP: explode("\n", 文件内容) —— 空文件得到 [""]，不是空数组。
	lines := strings.Split(string(raw), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		// PHP: strpos($item, $key) === FALSE 才保留，即**子串**命中即删除。
		if strings.Contains(line, key) {
			continue
		}
		kept = append(kept, line)
	}

	// PHP: file_put_contents(文件, implode("\n", 保留的行)) —— 整体覆盖写。
	//
	// 这里改为"临时文件 + rename"：旧实现先 Truncate(0) 再写回，两步之间
	// 进程崩溃会让 authorized_keys 变空，所有用户的 SSH 公钥一次丢失。
	out := strings.Join(kept, "\n")
	tmp, err := os.CreateTemp(filepath.Dir(w.keyFile()), ".authorized_keys.tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("设置临时文件权限失败: %w", err)
	}
	if _, err := tmp.WriteString(out); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("同步临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tmpName, w.keyFile()); err != nil {
		return fmt.Errorf("替换 authorized_keys 失败: %w", err)
	}
	// rename 后的新文件属主是当前进程，重新归给 git 用户（sshd StrictModes 要求）。
	w.ensureOwnership()
	return nil
}

// validateKeyFields 校验写入 authorized_keys 的字段不含行分隔符。
//
// authorized_keys 是**一行一个键**的格式；任何字段里出现换行都等于注入
// 一条新记录。keyType 与 key 还必须是单 token（不能含空白），否则会改变
// 行内字段边界。
func validateKeyFields(keyType, key, name string) error {
	if strings.TrimSpace(keyType) == "" || strings.ContainsAny(keyType, " \t\r\n\x00") {
		return errors.New("公钥类型非法")
	}
	if strings.TrimSpace(key) == "" || strings.ContainsAny(key, " \t\r\n\x00") {
		return errors.New("公钥正文非法：必须是不含空白的单个 token")
	}
	if strings.ContainsAny(name, "\r\n\x00") {
		return errors.New("公钥名称不能包含换行或 NUL")
	}
	return nil
}

// formatLine 生成 authorized_keys 的一行。
//
// 逐字对应 misc/modify_authorized_keys.php:114-122。
//
// 注意 command="..." 里的 $PATH 与 $SSH_ORIGINAL_COMMAND 是**字面量美元符号**
// （PHP 侧用 \$ 转义），必须原样写进文件，由 sshd 在登录时展开。
func (w *Writer) formatLine(uKey, keyType, key, name string) string {
	return fmt.Sprintf(
		"command=\"PATH=$PATH:%s && %s $SSH_ORIGINAL_COMMAND %s\","+
			"no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty %s %s %s\n",
		w.gitDir, w.sshGateway(), uKey, keyType, key, name,
	)
}

// ensure 确保 .ssh 目录与 authorized_keys 文件存在，且归属 git 用户。
//
// 逐字对应 misc/modify_authorized_keys.php:18-26：
// 文件不存在时创建目录与文件（mkdir 默认 0777、touch 默认 0666，实际生效值由 umask 决定）。
//
// 归属设置是**尽力而为**：PHP 侧 chown/chgrp 的返回值被忽略（PHP:22-25），
// 失败不影响守护进程继续写文件。这里保持同样的取舍 —— 归属不对只会让
// sshd 的 StrictModes 拒绝该文件，而让"加公钥"接口整体失败对用户更糟。
func (w *Writer) ensure() error {
	if err := w.ensureExists(); err != nil {
		return err
	}
	w.ensureOwnership()
	return nil
}

// ensureExists 确保 .ssh 目录与 authorized_keys 文件存在。
func (w *Writer) ensureExists() error {
	if _, err := os.Stat(w.keyFile()); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查 authorized_keys 失败: %w", err)
	}

	if err := os.MkdirAll(w.sshDir(), sshDirMode); err != nil {
		return fmt.Errorf("创建 SSH 目录失败: %w", err)
	}
	f, err := os.OpenFile(w.keyFile(), os.O_CREATE|os.O_WRONLY, keyFileMode)
	if err != nil {
		return fmt.Errorf("创建 authorized_keys 失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("关闭 authorized_keys 失败: %w", err)
	}
	return nil
}

// ensureOwnership 尽力把 .ssh 目录与 authorized_keys 的属主/属组改成 users.git。
//
// 对应 PHP:22-25 的 chown / chgrp。失败只记告警，不阻断写入。
func (w *Writer) ensureOwnership() {
	if !supported {
		// 非 POSIX 平台没有属主概念，跳过（开发机降级路径）。
		return
	}
	if err := w.chownToGitUser(); err != nil {
		slog.Warn("设置 authorized_keys 归属失败", "user", w.gitUser, "error", err)
	}
}

// chownToGitUser 把 .ssh 目录与 authorized_keys 的属主/属组改成 users.git。
//
// 对应 PHP:22-25 的 chown / chgrp。
func (w *Writer) chownToGitUser() error {
	if w.gitUser == "" {
		return errors.New("未配置 users.git，无法设置 authorized_keys 归属")
	}
	u, err := user.Lookup(w.gitUser)
	if err != nil {
		return fmt.Errorf("查找用户 %s 失败: %w", w.gitUser, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("用户 %s 的 uid %q 不是数字: %w", w.gitUser, u.Uid, err)
	}
	// PHP 的 chgrp($path, GIT_USER) 用的是**同名用户组**；
	// 组不存在时退回用户的主组（容器里 git 用户的附加组通常就是它自己）。
	gidStr := u.Gid
	if g, err := user.LookupGroup(w.gitUser); err == nil {
		gidStr = g.Gid
	}
	gid, err := strconv.Atoi(gidStr)
	if err != nil {
		return fmt.Errorf("用户 %s 的 gid %q 不是数字: %w", w.gitUser, gidStr, err)
	}
	if err := chown(w.sshDir(), uid, gid); err != nil {
		return fmt.Errorf("设置 SSH 目录归属失败: %w", err)
	}
	if err := chown(w.keyFile(), uid, gid); err != nil {
		return fmt.Errorf("设置 authorized_keys 归属失败: %w", err)
	}
	return nil
}

// sshDir 返回 <家目录>/.ssh。
func (w *Writer) sshDir() string {
	return filepath.Join(w.homeDir, sshDirName)
}

// keyFile 返回 authorized_keys 的完整路径。
func (w *Writer) keyFile() string {
	return filepath.Join(w.homeDir, sshDirName, authorizedKeysName)
}

// sshGateway 返回 ssh-gateway 可执行体的绝对路径（PHP:114）。
//
// 这里刻意用 "/" 拼接而不是 filepath.Join —— 目标文件在 Linux 容器里，
// 路径分隔符必须永远是 "/"，否则 Windows 开发机上写出的内容与生产不一致。
func (w *Writer) sshGateway() string {
	return strings.TrimRight(w.baseDir, "/") + "/" + sshGatewayRel
}

// phpDirname 复刻 PHP 的 dirname()。
//
// PHP 的 dirname 只按 "/" 切分，与运行平台无关：
// dirname("/usr/bin/git") === "/usr/bin"；dirname("git") === "."；dirname("") === ""。
//
// 这里不能用 filepath.Dir —— 它按运行平台的分隔符切分，
// 在 Windows 上会把 "/usr/bin/git" 变成 "\usr\bin"，
// 导致写出的 PATH= 前缀与 PHP 版本不一致。
func phpDirname(p string) string {
	if p == "" {
		return ""
	}
	trimmed := strings.TrimRight(p, "/")
	if trimmed == "" {
		// 原路径全是 "/"（例如 "/"）。
		return "/"
	}
	i := strings.LastIndex(trimmed, "/")
	if i < 0 {
		return "."
	}
	if i == 0 {
		return "/"
	}
	return trimmed[:i]
}
