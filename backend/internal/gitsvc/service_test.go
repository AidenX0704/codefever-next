package gitsvc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 辅助：构造一个真实的临时 git 仓库
// ---------------------------------------------------------------------------

// gitAvailable 判断当前环境是否有 git。
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// newTestRepo 创建一个带两次提交的临时仓库，返回仓库路径。
//
// 提交内容固定，便于断言：
//
//	commit 1: "first commit"  → 写入 README.md
//	commit 2: "second commit; rm -rf / #" → 追加一行
func newTestRepo(t *testing.T) string {
	t.Helper()
	if !gitAvailable() {
		t.Skip("环境里没有 git，跳过真实 git 调用测试")
	}
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=codefever", "GIT_AUTHOR_EMAIL=root@codefever.cn",
			"GIT_COMMITTER_NAME=codefever", "GIT_COMMITTER_EMAIL=root@codefever.cn",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v 失败: %v\n%s", args, err, out)
		}
	}

	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}

	run("init", "-q", "-b", "master")
	write("hello codefever local build\n")
	run("add", "README.md")
	run("commit", "-q", "-m", "first commit")

	write("hello codefever local build\nsecond line\n")
	run("add", "README.md")
	run("commit", "-q", "-m", "second commit")

	return dir
}

// ---------------------------------------------------------------------------
// 路径解析
// ---------------------------------------------------------------------------

func TestRepoPath(t *testing.T) {
	svc := NewService("git", filepath.Join(t.TempDir(), "git-storage"))

	got, err := svc.RepoPath("/de7347a84f6236f06b0356ce6668c27d")
	if err != nil {
		t.Fatalf("RepoPath 失败: %v", err)
	}
	want := filepath.Join(svc.StorageRoot(), "de7347a84f6236f06b0356ce6668c27d")
	if got != want {
		t.Fatalf("RepoPath = %q, 期望 %q", got, want)
	}

	for _, bad := range []string{"", "   ", "/", "../etc/passwd", "/a/../../b", "/a\\b"} {
		if _, err := svc.RepoPath(bad); err == nil {
			t.Fatalf("RepoPath(%q) 应当被拒绝", bad)
		}
	}
}

// ---------------------------------------------------------------------------
// argv 构造：证明不经过 shell
// ---------------------------------------------------------------------------

// TestCommitListArgsAreArgv 断言搜索关键词是**一个** argv 元素，
// 而不是被拼进 shell 字符串。
func TestCommitListArgsAreArgv(t *testing.T) {
	keyword := "; rm -rf / #"
	args := commitListArgs("master", "", keyword, 1, 20)

	joined := strings.Join(args, " ")
	if strings.Contains(joined, "sh -c") || strings.Contains(joined, "bash -c") {
		t.Fatalf("argv 里出现了 shell: %v", args)
	}

	found := false
	for _, a := range args {
		if a == "--grep="+keyword {
			found = true
		}
		if a == keyword {
			t.Fatalf("关键词被拆成了独立参数: %v", args)
		}
	}
	if !found {
		t.Fatalf("argv 里找不到完整的 --grep 参数: %v", args)
	}
}

// TestCommitListArgsPathSeparator 断言路径类参数前置 "--"。
func TestCommitListArgsPathSeparator(t *testing.T) {
	args := commitListArgs("master", "--upload-pack=evil", "", 1, 20)
	idx := -1
	for i, a := range args {
		if a == "--" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("argv 里缺少路径分隔符: %v", args)
	}
	if args[idx+1] != "--upload-pack=evil" {
		t.Fatalf("路径参数位置不对: %v", args)
	}
}

// ---------------------------------------------------------------------------
// 注入测试：真实执行，验证注入被阻断
// ---------------------------------------------------------------------------

// TestKeywordInjectionIsBlocked 用真实的 git 调用验证命令注入被阻断。
//
// 关键词 "; rm -rf / #" 在 PHP 侧会被原样拼进 shell 字符串
// （application/libraries/service/Git/Command.php:201-203），
// 从而执行任意命令。这里用 argv 调用，注入内容只会被 git 当成正则字面量。
func TestKeywordInjectionIsBlocked(t *testing.T) {
	dir := newTestRepo(t)
	svc := NewService("git", filepath.Dir(dir))

	// 注入目标：仓库旁边的哨兵文件。若发生 shell 注入，它会被删除。
	marker := filepath.Join(filepath.Dir(dir), "pwned.txt")
	if err := os.WriteFile(marker, []byte("safe"), 0o644); err != nil {
		t.Fatalf("写哨兵文件失败: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(marker) })

	commits, err := svc.Commits(context.Background(), dir, "master", "", "; rm -rf / #", 1, 20)
	if err != nil {
		t.Fatalf("Commits 失败: %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("哨兵文件消失，说明命令注入生效了: %v", err)
	}
	// 该关键词在 git 里是字面正则，不会匹配任何提交。
	if len(commits) != 0 {
		t.Fatalf("注入关键词不应匹配到提交，实际返回 %d 条", len(commits))
	}
}

// TestOptionInjectionIsBlocked 断言以 "-" 开头的修订号被拒绝。
func TestOptionInjectionIsBlocked(t *testing.T) {
	dir := newTestRepo(t)
	svc := NewService("git", filepath.Dir(dir))

	if _, err := svc.Commits(context.Background(), dir, "--all", "", "", 1, 20); err == nil {
		t.Fatal("以 - 开头的修订号应当被拒绝")
	}
	if _, err := svc.CommitDetail(context.Background(), dir, "--all"); err == nil {
		t.Fatal("以 - 开头的提交号应当被拒绝")
	}
}

// ---------------------------------------------------------------------------
// 真实 git 调用
// ---------------------------------------------------------------------------

// TestRealGitCalls 覆盖一次真实仓库上的主要 git 操作。
func TestRealGitCalls(t *testing.T) {
	dir := newTestRepo(t)
	svc := NewService("git", filepath.Dir(dir))
	ctx := context.Background()

	commits, err := svc.Commits(ctx, dir, "master", "", "", 1, 20)
	if err != nil {
		t.Fatalf("Commits 失败: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("Commits 返回 %d 条, 期望 2 条", len(commits))
	}
	// git log 默认按时间倒序：最新的一条在前。
	if commits[0].Commit != "second commit" {
		t.Fatalf("提交标题不对: %q（全部: %+v）", commits[0].Commit, commits)
	}
	if commits[0].SHA == "" || commits[0].Email != "root@codefever.cn" {
		t.Fatalf("提交元信息不对: %+v", commits[0])
	}
	if commits[0].Time <= 0 {
		t.Fatalf("提交时间不对: %+v", commits[0])
	}

	detail, err := svc.CommitDetail(ctx, dir, commits[0].SHA)
	if err != nil {
		t.Fatalf("CommitDetail 失败: %v", err)
	}
	if detail.SHA != commits[0].SHA {
		t.Fatalf("CommitDetail.SHA = %q, 期望 %q", detail.SHA, commits[0].SHA)
	}
	if detail.Parent == "" {
		t.Fatal("CommitDetail.Parent 不应为空")
	}

	last, err := svc.LastCommit(ctx, dir, "master", "README.md", "")
	if err != nil {
		t.Fatalf("LastCommit 失败: %v", err)
	}
	if last.SHA != commits[0].SHA {
		t.Fatalf("LastCommit.SHA = %q, 期望 %q", last.SHA, commits[0].SHA)
	}

	branches, err := svc.Branches(ctx, dir)
	if err != nil {
		t.Fatalf("Branches 失败: %v", err)
	}
	if len(branches) != 1 || branches[0] != "master" {
		t.Fatalf("Branches = %v, 期望 [master]", branches)
	}

	tags, err := svc.Tags(ctx, dir)
	if err != nil {
		t.Fatalf("Tags 失败: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("Tags = %v, 期望空", tags)
	}

	count, err := svc.CommitCount(ctx, dir, "master")
	if err != nil {
		t.Fatalf("CommitCount 失败: %v", err)
	}
	if count != 2 {
		t.Fatalf("CommitCount = %d, 期望 2", count)
	}

	obj, err := svc.Object(ctx, dir, "master")
	if err != nil {
		t.Fatalf("Object 失败: %v", err)
	}
	if obj.Type != "commit" || len(obj.Entries) != 1 {
		t.Fatalf("Object = %+v, 期望 commit 且 1 个条目", obj)
	}
	if obj.Entries[0].Name != "README.md" || obj.Entries[0].Type != "blob" {
		t.Fatalf("Object 条目不对: %+v", obj.Entries[0])
	}

	blame, err := svc.Blame(ctx, dir, "master", "README.md")
	if err != nil {
		t.Fatalf("Blame 失败: %v", err)
	}
	if len(blame.Lines) != 2 {
		t.Fatalf("Blame 行数 = %d, 期望 2", len(blame.Lines))
	}
	if len(blame.Blame) != 2 {
		t.Fatalf("Blame 段数 = %d, 期望 2", len(blame.Blame))
	}

	containing, err := svc.BranchesContainingSHA(ctx, dir, commits[0].SHA)
	if err != nil {
		t.Fatalf("BranchesContainingSHA 失败: %v", err)
	}
	if len(containing) != 1 || containing[0] != "master" {
		t.Fatalf("BranchesContainingSHA = %v, 期望 [master]", containing)
	}

	diff, err := svc.Diff(ctx, dir, commits[1].SHA, commits[0].SHA)
	if err != nil {
		t.Fatalf("Diff 失败: %v", err)
	}
	if !strings.Contains(diff, "+second line") {
		t.Fatalf("Diff 内容不对:\n%s", diff)
	}

	files, err := svc.DiffFileCount(ctx, dir, commits[1].SHA, commits[0].SHA)
	if err != nil {
		t.Fatalf("DiffFileCount 失败: %v", err)
	}
	if files != 1 {
		t.Fatalf("DiffFileCount = %d, 期望 1", files)
	}

	objects, err := svc.ObjectsCount(ctx, dir)
	if err != nil {
		t.Fatalf("ObjectsCount 失败: %v", err)
	}
	if objects.Count <= 0 {
		t.Fatalf("ObjectsCount = %+v, 期望 count > 0", objects)
	}
}

// ---------------------------------------------------------------------------
// 输出协议
// ---------------------------------------------------------------------------

func TestPHPValue(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"a\nb", `a\nb`},
		{"a\rb", `a\rb`},
		{"a\tb", `a\tb`},
		{"a\x01b", "a b"},
		{`back\slash`, `back\slash`},
		{`quote"`, `quote"`},
	}
	for _, c := range cases {
		if got := phpValue(c.in); got != c.want {
			t.Fatalf("phpValue(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

func TestParseDiff(t *testing.T) {
	diff := `diff --git a/SSH.md b/SSH.md
new file mode 100644
index 0000000..437facb
--- /dev/null
+++ b/SSH.md
@@ -0,0 +1 @@
+ssh push test
`
	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("ParseDiff 返回 %d 个文件, 期望 1 个", len(files))
	}
	f := files[0]
	if f.Type != "text" {
		t.Fatalf("type = %q, 期望 text", f.Type)
	}
	if f.Original.Name != nil {
		t.Fatalf("original.name = %v, 期望 nil", *f.Original.Name)
	}
	if f.Modified.Name == nil || *f.Modified.Name != "SSH.md" {
		t.Fatalf("modified.name 不对")
	}
	if f.Original.SHA == nil || *f.Original.SHA != "0000000" {
		t.Fatalf("original.sha 不对")
	}
	if f.Diff.Additions != 1 || f.Diff.Deletions != 0 {
		t.Fatalf("增删行数不对: %+v", f.Diff)
	}
	if f.Diff.Raw == nil || *f.Diff.Raw != "@@ -0,0 +1 @@\n+ssh push test" {
		t.Fatalf("diff.raw 不对: %v", f.Diff.Raw)
	}
}

// TestServiceRunsWithoutShell 断言 git 进程的 argv 里没有 shell。
//
// 这里通过"git 收到的是哪个参数"来反推：把一个带元字符的参数传进去，
// 若经过 shell，它会被拆开或被执行。
func TestServiceRunsWithoutShell(t *testing.T) {
	dir := newTestRepo(t)
	svc := NewService("git", filepath.Dir(dir))

	// 路径参数含 shell 元字符：在 argv 语义下它是**一个**路径。
	out, err := svc.run(context.Background(), dir, "log", "-1", "--format=%s", "--", "; touch pwned.txt")
	if err != nil {
		// 路径不存在时 git 会报错，这本身就是"没有经过 shell"的证据：
		// 若经过 shell，"; touch pwned.txt" 会被执行成两条命令。
		t.Logf("git 退出码 %d（预期：路径不存在）", ExitCodeOf(err))
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned.txt")); err == nil {
		t.Fatal("出现了 pwned.txt，说明命令被 shell 执行了")
	}
	if strings.TrimSpace(out) != "" {
		t.Logf("git 输出: %q", out)
	}
}

// TestExitCode 断言退出码被精确判定。
func TestExitCode(t *testing.T) {
	dir := newTestRepo(t)
	svc := NewService("git", filepath.Dir(dir))

	_, err := svc.run(context.Background(), dir, "cat-file", "-t", "0000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("应当返回错误")
	}
	if ExitCodeOf(err) == 0 {
		t.Fatalf("退出码应当非 0，实际 %d", ExitCodeOf(err))
	}
}

// TestWindowsRepoPathSeparator 在 Windows 上也要能拼出正确的路径。
func TestWindowsRepoPathSeparator(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 相关")
	}
	svc := NewService("git", `C:\data\git-storage`)
	got, err := svc.RepoPath("/de7347a84f6236f06b0356ce6668c27d")
	if err != nil {
		t.Fatalf("RepoPath 失败: %v", err)
	}
	if !strings.Contains(got, `de7347a84f6236f06b0356ce6668c27d`) {
		t.Fatalf("路径不对: %q", got)
	}
	if strings.Contains(got, "/") {
		t.Fatalf("Windows 路径里不应出现 /: %q", got)
	}
}
