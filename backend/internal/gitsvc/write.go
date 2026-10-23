// 仓库**写端点**所需的 git 封装。
//
// 与 service.go / commands.go 的分工：
//
//   - service.go / commands.go：只读侧的 git 封装（log / branch / tag / blame / diff …）
//   - write.go（本文件）      ：写侧的 git 封装（init / clone / branch 创建 / tag 创建 / merge …）
//
// 写侧的全部差异都遵循本包的一贯原则：
//
//  1. 命令以 **argv 数组** 交给 exec.CommandContext，**永不经过 shell**；
//  2. 用户输入以 `--` 或独立参数传递，不会被 git 当成选项；
//  3. 退出码用 *exec.ExitError.ExitCode() 精确判定 —— PHP 侧的
//     Command::batch() 丢弃了退出码，"成功但无输出"与"失败"不可区分。
package gitsvc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"codefever-next/backend/internal/domain"
)

// HooksLinkTarget 是裸仓库 hooks 目录的软链接目标。
//
// 逐字对应 PHP 侧 GIT_COMMAND_INIT / GIT_COMMAND_FORK 的
// `ln -s ../../misc/hooks hooks` —— **相对路径不可变**：
// 钩子脚本（misc/hooks/main）依赖这个相对路径定位到 <base>/misc/hooks。
const HooksLinkTarget = "../../misc/hooks"

// MergeResult 是合并操作的结果。
type MergeResult struct {
	// Conflict 为真表示合并产生了冲突（PHP 侧靠输出里的 "conflicts" 判定）。
	Conflict bool
	// Output 是 git 的合并输出。
	Output string
}

// conflictPattern 对应 application/controllers/api/repository.php:1785 的
// `/\bconflicts\b/` —— git 在冲突时输出
// "Automatic merge failed; fix conflicts and then commit the result."。
var conflictPattern = regexp.MustCompile(`\bconflicts\b`)

// ---------------------------------------------------------------------------
// 内部工具
// ---------------------------------------------------------------------------

// runWithEnv 在指定目录里执行一条 git 命令，并追加若干环境变量。
//
// 与 service.go 的 run() 的唯一差别是能追加环境变量（合并提交需要
// GIT_COMMITTER_* / GIT_AUTHOR_*）。其余行为完全一致：argv 直传、
// 退出码精确判定、stderr 一并带回。
func (s *Service) runWithEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error) {
	env, err := s.buildEnv()
	if err != nil {
		return "", err
	}
	env = append(env, extraEnv...)

	cmd := exec.CommandContext(ctx, s.binPath, args...)
	cmd.Dir = dir
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

// chmodRecursive 递归地把目录设成 0777。
//
// 逐字对应 PHP 侧的 `chmod -R 0777 {$repositoryPath}`：裸仓库由 API 进程创建，
// 而推送由 git 用户完成，因此目录必须对所有用户可写。
func chmodRecursive(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o777); err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := chmodRecursive(filepath.Join(path, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 仓库初始化
// ---------------------------------------------------------------------------

// InitBare 初始化一个裸仓库并装好钩子软链接。
//
// 逐字对应 PHP 侧 application/models/repository_model.php:648-657 的
// GIT_COMMAND_INIT：
//
//	mkdir {$repositoryPath}
//	cd {$repositoryPath}
//	git init --bare
//	rm -r hooks
//	ln -s ../../misc/hooks hooks
//	chmod -R 0777 {$repositoryPath}
//
// 与 PHP 的差异：PHP 把这些命令塞进一条 shell 字符串、用 `&&` 串联，
// 任何一步失败都只看整条命令的退出码；这里每一步单独执行并精确定位失败点。
func (s *Service) InitBare(ctx context.Context, repoPath string) error {
	if repoPath == "" {
		return errors.New("仓库路径不能为空")
	}
	if err := os.MkdirAll(repoPath, 0o777); err != nil {
		return err
	}
	if _, err := s.run(ctx, repoPath, "init", "--bare"); err != nil {
		return err
	}
	if err := s.InstallHooks(ctx, repoPath); err != nil {
		return err
	}
	return chmodRecursive(repoPath)
}

// CloneBare 从源裸仓库克隆一个裸仓库到 repoPath。
//
// 逐字对应 PHP 侧 application/models/repository_model.php:658-668 的
// GIT_COMMAND_FORK：
//
//	mkdir {$repositoryPath}
//	cd {$repositoryPath}
//	git clone --bare {$source} .
//	git remote remove origin
//	rm -r hooks
//	ln -s ../../misc/hooks hooks
//	chmod -R 0777 {$repositoryPath}
//
// 与 PHP 的关键差异：PHP 侧 `Repository_model::fork()` **丢弃了 execCommand
// 的返回值**（application/models/repository_model.php:974-976），克隆失败也会
// 返回成功，于是留下"有数据库行、无磁盘仓库"的孤儿。这里把错误如实返回，
// 由调用方回滚事务并清理目录。
func (s *Service) CloneBare(ctx context.Context, repoPath, sourcePath string) error {
	if repoPath == "" || sourcePath == "" {
		return errors.New("克隆参数不完整")
	}
	if err := os.MkdirAll(repoPath, 0o777); err != nil {
		return err
	}
	if _, err := s.run(ctx, repoPath, "clone", "--bare", sourcePath, "."); err != nil {
		return err
	}
	if _, err := s.run(ctx, repoPath, "remote", "remove", "origin"); err != nil {
		return err
	}
	if err := s.InstallHooks(ctx, repoPath); err != nil {
		return err
	}
	return chmodRecursive(repoPath)
}

// InstallHooks 用相对软链接替换 hooks 目录。
//
// 逐字对应 PHP 侧的 `rm -r hooks` + `ln -s ../../misc/hooks hooks`。
func (s *Service) InstallHooks(ctx context.Context, repoPath string) error {
	if repoPath == "" {
		return errors.New("仓库路径不能为空")
	}
	hooks := filepath.Join(repoPath, "hooks")
	if err := os.RemoveAll(hooks); err != nil {
		return err
	}
	return os.Symlink(HooksLinkTarget, hooks)
}

// RemoveRepo 删除磁盘上的裸仓库目录。
//
// 用于建仓/派生失败时的清理 —— PHP 侧没有这个补偿步骤，
// 失败时会留下半成品目录。
func (s *Service) RemoveRepo(ctx context.Context, repoPath string) error {
	if repoPath == "" {
		return errors.New("仓库路径不能为空")
	}
	return os.RemoveAll(repoPath)
}

// ---------------------------------------------------------------------------
// 引用
// ---------------------------------------------------------------------------

// RefExists 判断一个修订号（分支 / 标签 / 提交哈希）是否存在。
//
// 用于区分"源 ref 不存在"（0x040C Old Target Not Exists）与其它失败原因。
// PHP 侧没有这个区分：execCommand 把退出码丢掉，失败一律返回 FALSE，
// 于是"源不存在"与"权限不足"等错误被混为一谈。
func (s *Service) RefExists(ctx context.Context, repoPath, ref string) (bool, error) {
	if ref == "" {
		return false, nil
	}
	if err := rejectOptionLike(ref); err != nil {
		return false, err
	}
	_, err := s.run(ctx, repoPath, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err == nil {
		return true, nil
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		// rev-parse 对不存在的修订号返回 1。
		return false, nil
	}
	return false, err
}

// CreateBranch 在裸仓库里创建一个分支。
//
// 逐字对应 PHP 侧 application/libraries/service/Git/Command.php:52-55 的
// createBranch()：`git branch <branch> <sourceBranch>`。
//
// PHP 侧把分支名与源名拼进同一条 shell 字符串（Command::wrapArgument），
// 这里改为独立 argv 元素 + `--` 分隔符，语义相同但切断了注入路径。
func (s *Service) CreateBranch(ctx context.Context, repoPath, name, origin string) error {
	if name == "" || origin == "" {
		return errors.New("分支参数不完整")
	}
	if err := rejectOptionLike(name); err != nil {
		return err
	}
	if err := rejectOptionLike(origin); err != nil {
		return err
	}
	_, err := s.run(ctx, repoPath, "branch", "--", name, origin)
	return err
}

// DeleteBranch 删除一个分支（强制删除）。
//
// 逐字对应 PHP 侧 Command.php:122-129 的 deleteBranch()：`git branch -D <branch>`。
func (s *Service) DeleteBranch(ctx context.Context, repoPath, name string) error {
	if name == "" {
		return errors.New("分支名不能为空")
	}
	if err := rejectOptionLike(name); err != nil {
		return err
	}
	_, err := s.run(ctx, repoPath, "branch", "-D", "--", name)
	return err
}

// CreateTag 创建一个标签。
//
// 逐字对应 PHP 侧 Command.php:57-73 的 createTag()：
//
//	git tag [-a -m <message>] -- <tag> [<commitSHA>]
//
// PHP 侧用 escapeshellarg 把标签名塞进 shell 字符串；这里直接作为 argv 元素。
//
// 附注标签（-a）需要提交者身份，否则 git 会报 "Please tell me who you are"。
// PHP 侧靠 GIT_COMMAND_QUERY 分支里的
// `export GIT_COMMITTER_NAME=... / GIT_COMMITTER_EMAIL=... / GIT_AUTHOR_NAME=...`
// 提供身份（application/models/repository_model.php:669-676），
// 这里用同样的四个环境变量显式传入。
func (s *Service) CreateTag(ctx context.Context, repoPath, name, description, commitSHA, authorName, authorEmail string) error {
	if name == "" {
		return errors.New("标签名不能为空")
	}
	if err := rejectOptionLike(name); err != nil {
		return err
	}
	args := []string{"tag"}
	if description != "" {
		args = append(args, "-a", "-m", description)
	}
	args = append(args, "--", name)
	if commitSHA != "" {
		if err := rejectOptionLike(commitSHA); err != nil {
			return err
		}
		args = append(args, commitSHA)
	}
	_, err := s.runWithEnv(ctx, repoPath, committerEnv(authorName, authorEmail), args...)
	return err
}

// committerEnv 构造 git 提交所需的身份环境变量。
//
// 逐字对应 PHP 侧 GIT_COMMAND_QUERY 分支导出的三个变量；
// 额外补上 GIT_AUTHOR_EMAIL（PHP 侧未设，git 会回落到 committer 邮箱）。
func committerEnv(authorName, authorEmail string) []string {
	return []string{
		"GIT_COMMITTER_NAME=" + authorName,
		"GIT_COMMITTER_EMAIL=" + authorEmail,
		"GIT_AUTHOR_NAME=" + authorName,
		"GIT_AUTHOR_EMAIL=" + authorEmail,
	}
}

// DeleteTag 删除一个标签。
//
// 逐字对应 PHP 侧 Command.php:80-83 的 deleteTag()：`git tag -d <tagName>`。
func (s *Service) DeleteTag(ctx context.Context, repoPath, name string) error {
	if name == "" {
		return errors.New("标签名不能为空")
	}
	if err := rejectOptionLike(name); err != nil {
		return err
	}
	_, err := s.run(ctx, repoPath, "tag", "-d", "--", name)
	return err
}

// ---------------------------------------------------------------------------
// 合并
// ---------------------------------------------------------------------------

// CanFastForward 判断源提交能否快进到目标提交。
//
// 逐字对应 application/models/repository_model.php:990-1015 的 canBeFastForward()：
// 取目标提交的头（`log --pretty="%H" -n1 <targetHash>`），
// 再看它是否出现在源提交的祖先链里（`log --pretty="%H" <sourceHash>`）。
func (s *Service) CanFastForward(ctx context.Context, sourceRepoPath, sourceSHA, targetRepoPath, targetSHA string) (bool, error) {
	if sourceSHA == "" || targetSHA == "" {
		return false, nil
	}
	head, err := s.run(ctx, targetRepoPath, "log", "--format=%H", "-1", targetSHA)
	if err != nil {
		return false, err
	}
	targetHead := strings.TrimSpace(head)
	if targetHead == "" {
		return false, nil
	}
	logs, err := s.run(ctx, sourceRepoPath, "log", "--format=%H", sourceSHA)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(logs, "\n") {
		if strings.TrimSpace(line) == targetHead {
			return true, nil
		}
	}
	return false, nil
}

// MergeBranch 把源分支合并进目标分支并推送。
//
// 逐字对应 application/models/repository_model.php:1031-1133 的 mergeBranch()：
//
//	克隆目标仓库 → 添加源仓库为 remote → fetch 源分支 → merge → push
//
// 合并方式由 message 与 usingSquash 决定（PHP 侧的注释表格）：
//
//	message=NULL, squash=任意 → Fast-Forward
//	message=String, squash=FALSE → Non-Fast-Forward（--no-ff）
//	message=String, squash=TRUE  → Squash（--squash + commit）
//
// 与 PHP 的差异：PHP 用 `Command::run()` 的输出数组判断冲突（正则 /\bconflicts\b/），
// 这里同样按该正则判定，但**失败原因**通过退出码精确区分。
func (s *Service) MergeBranch(ctx context.Context, sourceRepoPath, sourceBranch, targetRepoPath, targetBranch, message, authorName, authorEmail string, squash bool) (*MergeResult, error) {
	if sourceRepoPath == "" || sourceBranch == "" || targetRepoPath == "" || targetBranch == "" {
		return nil, errors.New("合并参数不完整")
	}
	if err := rejectOptionLike(sourceBranch); err != nil {
		return nil, err
	}
	if err := rejectOptionLike(targetBranch); err != nil {
		return nil, err
	}

	workspace, err := os.MkdirTemp("", "codefever-merge-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspace)

	// 1. 克隆目标仓库并检出目标分支。
	if _, err := s.run(ctx, workspace, "clone", "-b", targetBranch, targetRepoPath, "."); err != nil {
		return nil, err
	}
	// 2. 把源仓库作为 remote 并抓取源分支。
	if _, err := s.run(ctx, workspace, "remote", "add", "cf-source", sourceRepoPath); err != nil {
		return nil, err
	}
	if _, err := s.run(ctx, workspace, "fetch", "-q", "cf-source", sourceBranch); err != nil {
		return nil, err
	}

	env := committerEnv(authorName, authorEmail)

	// 3. 合并。
	switch {
	case message != "" && squash:
		out, err := s.runWithEnv(ctx, workspace, env, "merge", "--squash", "FETCH_HEAD")
		if err != nil {
			return &MergeResult{Conflict: isConflict(out, err), Output: out}, nil
		}
		if _, err := s.runWithEnv(ctx, workspace, env, "commit", "-m", message); err != nil {
			return nil, err
		}
	case message != "":
		out, err := s.runWithEnv(ctx, workspace, env, "merge", "--no-ff", "-m", message, "FETCH_HEAD")
		if err != nil {
			return &MergeResult{Conflict: isConflict(out, err), Output: out}, nil
		}
	default:
		out, err := s.runWithEnv(ctx, workspace, env, "merge", "FETCH_HEAD")
		if err != nil {
			return &MergeResult{Conflict: isConflict(out, err), Output: out}, nil
		}
	}

	// 4. 推送回目标仓库。
	if _, err := s.run(ctx, workspace, "push", "origin", targetBranch); err != nil {
		return nil, err
	}
	return &MergeResult{}, nil
}

// isConflict 判断合并输出里是否出现冲突。
//
// 逐字对应 application/controllers/api/repository.php:1785-1788 的
// `preg_match('/\bconflicts\b/', $res[count($res) - 1])`。
func isConflict(out string, err error) bool {
	text := out
	if err != nil {
		text += "\n" + err.Error()
	}
	return conflictPattern.MatchString(text)
}

// CommitsSinceRemote 列出源仓库中相对目标仓库"多出来"的提交。
//
// 逐字对应 application/models/repository_model.php:1915-1948 的
// getCommitWithRemote()：在源仓库里临时添加目标仓库为 remote、fetch 之后
// 执行 `log --cherry-pick --left-only <source>...<target>`。
//
// 这里在临时工作区里做同样的事，**不污染**真实仓库的 remote 配置。
func (s *Service) CommitsSinceRemote(ctx context.Context, sourceRepoPath, sourceSHA, targetRepoPath, targetSHA string) ([]domain.Commit, error) {
	if sourceSHA == "" || targetSHA == "" {
		return nil, errors.New("提交哈希不完整")
	}
	if err := rejectOptionLike(sourceSHA); err != nil {
		return nil, err
	}
	if err := rejectOptionLike(targetSHA); err != nil {
		return nil, err
	}

	workspace, err := os.MkdirTemp("", "codefever-mr-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspace)

	if _, err := s.run(ctx, workspace, "clone", "--bare", sourceRepoPath, "."); err != nil {
		return nil, err
	}
	if _, err := s.run(ctx, workspace, "remote", "add", "cf-target", targetRepoPath); err != nil {
		return nil, err
	}
	if _, err := s.run(ctx, workspace, "fetch", "-q", "cf-target"); err != nil {
		return nil, err
	}
	out, err := s.run(ctx, workspace, "log", "--cherry-pick", "--left-only",
		"--format="+fmtCommit, sourceSHA+"..."+targetSHA)
	if err != nil {
		return nil, err
	}
	records := parseFormatRecords(out, 5)
	commits := make([]domain.Commit, 0, len(records))
	for _, r := range records {
		commits = append(commits, commitFromFields(r))
	}
	return commits, nil
}

// String 返回便于日志阅读的描述。
func (m *MergeResult) String() string {
	if m == nil {
		return "<nil>"
	}
	return fmt.Sprintf("MergeResult{Conflict:%v Output:%q}", m.Conflict, m.Output)
}
