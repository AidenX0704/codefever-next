package authkeys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestWriter 造一个落在临时目录里的 Writer。
//
// 家目录根被换成 t.TempDir()，因此测试**不会**写真实的 /home/<用户>/.ssh。
func newTestWriter(t *testing.T, cliGit string) (*Writer, string) {
	t.Helper()
	home := t.TempDir()
	w := newWriterWithHomeRoot(home, "/data/www/codefever-community", "git", cliGit)
	return w, w.keyFile()
}

// TestFormatLineMatchesPHPByteForByte 锁定写出的行与 PHP 守护进程逐字节一致。
//
// 期望值是直接在容器里跑 PHP 的 addAuthorizedKey（misc/modify_authorized_keys.php:106-123）
// 得到的真实输出，不是手写推测。
func TestFormatLineMatchesPHPByteForByte(t *testing.T) {
	w, _ := newTestWriter(t, "/usr/bin/git")

	const (
		uKey   = "UKEY123"
		keyTyp = "ssh-rsa"
		key    = "AAAAB3NzaC1yc2EAAAADAQABAAABgQC"
		name   = "my key"
	)

	want := `command="PATH=$PATH:/usr/bin && /data/www/codefever-community/ssh-gateway/shell/main $SSH_ORIGINAL_COMMAND UKEY123",` +
		`no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC my key` + "\n"

	got := w.formatLine(uKey, keyTyp, key, name)
	if got != want {
		t.Fatalf("行内容与 PHP 版本不一致\n got: %q\nwant: %q", got, want)
	}
}

// TestFormatLineKeepsLiteralDollarSigns 确认 command="..." 里的美元符号是字面量。
//
// $PATH 与 $SSH_ORIGINAL_COMMAND 必须原样写进文件，由 sshd 在登录时展开；
// 若被 Go 的 fmt 展开或被 shell 提前展开，SSH 登录就会失效。
func TestFormatLineKeepsLiteralDollarSigns(t *testing.T) {
	w, _ := newTestWriter(t, "/usr/bin/git")

	line := w.formatLine("UKEY", "ssh-rsa", "KEYDATA", "name")

	if !strings.Contains(line, "PATH=$PATH:") {
		t.Fatalf("缺少字面量 $PATH: %q", line)
	}
	if !strings.Contains(line, "$SSH_ORIGINAL_COMMAND") {
		t.Fatalf("缺少字面量 $SSH_ORIGINAL_COMMAND: %q", line)
	}
	if strings.Count(line, "$") != 2 {
		t.Fatalf("美元符号数量 = %d, want 2: %q", strings.Count(line, "$"), line)
	}
}

// TestFormatLineFieldSeparators 确认四个字段之间是**单个空格**。
//
// 逐字对应 PHP 的 implode(' ', $authorized)。
func TestFormatLineFieldSeparators(t *testing.T) {
	w, _ := newTestWriter(t, "/usr/bin/git")

	line := w.formatLine("UKEY", "ssh-rsa", "KEYDATA", "name")

	if !strings.HasSuffix(line, " ssh-rsa KEYDATA name\n") {
		t.Fatalf("字段分隔或行尾换行不符合规格: %q", line)
	}
	if strings.Contains(line, "  ") {
		t.Fatalf("出现了连续空格: %q", line)
	}
}

// TestFormatLineNoForwardOptions 确认四个 no-* 选项齐全且顺序固定。
func TestFormatLineNoForwardOptions(t *testing.T) {
	w, _ := newTestWriter(t, "/usr/bin/git")

	line := w.formatLine("UKEY", "ssh-rsa", "KEYDATA", "name")

	want := ",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty "
	if !strings.Contains(line, want) {
		t.Fatalf("no-* 选项段不符合规格: %q", line)
	}
}

// TestCLIGitDirname 取 cli.git 的**目录部分**。
//
// PHP 侧是 dirname($config['cli']['git'])（misc/modify_authorized_keys.php:116），
// 实测 dirname("/usr/bin/git") === "/usr/bin"、dirname("git") === "."。
func TestCLIGitDirname(t *testing.T) {
	cases := []struct {
		cliGit string
		want   string
	}{
		{"/usr/bin/git", "/usr/bin"},
		{"/usr/local/git/bin/git", "/usr/local/git/bin"},
		{"git", "."},
		{"", ""},
		{"/git", "/"},
		{"/usr/bin/git/", "/usr/bin"},
	}
	for _, c := range cases {
		w, _ := newTestWriter(t, c.cliGit)
		if w.gitDir != c.want {
			t.Fatalf("phpDirname(%q) = %q, want %q", c.cliGit, w.gitDir, c.want)
		}
	}
}

// TestAddAppendsLine 确认追加写：文件里原有内容不被覆盖，且新行以 "\n" 结尾。
func TestAddAppendsLine(t *testing.T) {
	w, keyFile := newTestWriter(t, "/usr/bin/git")

	// 先塞一行"历史遗留"内容，模拟既有文件。
	if err := os.MkdirAll(filepath.Dir(keyFile), 0o777); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	const legacy = "ssh-rsa LEGACYKEY legacy\n"
	if err := os.WriteFile(keyFile, []byte(legacy), 0o666); err != nil {
		t.Fatalf("写初始文件失败: %v", err)
	}

	if err := w.Add("UKEY1", "ssh-rsa", "AAAKEY1", "first"); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	want := legacy + `command="PATH=$PATH:/usr/bin && /data/www/codefever-community/ssh-gateway/shell/main $SSH_ORIGINAL_COMMAND UKEY1",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty ssh-rsa AAAKEY1 first` + "\n"
	if string(raw) != want {
		t.Fatalf("追加结果不符\n got: %q\nwant: %q", string(raw), want)
	}
}

// TestAddCreatesFileAndDir 确认首次写入时自动创建 .ssh 目录与 authorized_keys。
func TestAddCreatesFileAndDir(t *testing.T) {
	w, keyFile := newTestWriter(t, "/usr/bin/git")

	if _, err := os.Stat(keyFile); !os.IsNotExist(err) {
		t.Fatalf("前置条件不成立：文件已存在")
	}

	if err := w.Add("UKEY1", "ssh-rsa", "AAAKEY1", "first"); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}

	if _, err := os.Stat(keyFile); err != nil {
		t.Fatalf("authorized_keys 未被创建: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(keyFile)); err != nil {
		t.Fatalf(".ssh 目录未被创建: %v", err)
	}
}

// TestRemoveMatchesBySubstring 确认删除是**子串**匹配。
//
// 一行包含该 key 子串就被删掉，其余行（含不含子串的行）保留。
// 逐字对应 PHP 的 strpos($item, $key) === FALSE 才保留。
func TestRemoveMatchesBySubstring(t *testing.T) {
	w, keyFile := newTestWriter(t, "/usr/bin/git")

	if err := os.MkdirAll(filepath.Dir(keyFile), 0o777); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	const content = "ssh-rsa AAAKEY1 keepme\n" +
		"ssh-rsa OTHERKEY2 deleteme\n" +
		"ssh-rsa THIRDKEY3 keepme-too\n"
	if err := os.WriteFile(keyFile, []byte(content), 0o666); err != nil {
		t.Fatalf("写初始文件失败: %v", err)
	}

	if err := w.Remove("OTHERKEY2"); err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	want := "ssh-rsa AAAKEY1 keepme\n" + "ssh-rsa THIRDKEY3 keepme-too\n"
	if string(raw) != want {
		t.Fatalf("删除结果不符\n got: %q\nwant: %q", string(raw), want)
	}
}

// TestRemoveSubstringAtLineStart 确认子串出现在行首时也算命中。
//
// PHP 的 strpos 命中位置 0 时返回 0，而 0 !== FALSE，所以整行仍被删。
func TestRemoveSubstringAtLineStart(t *testing.T) {
	w, keyFile := newTestWriter(t, "/usr/bin/git")

	if err := os.MkdirAll(filepath.Dir(keyFile), 0o777); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	const content = "AAAKEY1 ssh-rsa keepme\nssh-rsa AAAKEY1 other\n"
	if err := os.WriteFile(keyFile, []byte(content), 0o666); err != nil {
		t.Fatalf("写初始文件失败: %v", err)
	}

	if err := w.Remove("AAAKEY1"); err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	if strings.Contains(string(raw), "AAAKEY1") {
		t.Fatalf("命中子串的行没有被删除: %q", string(raw))
	}
}

// TestRemoveKeepsPHPNoTrailingNewlineQuirk 确认删除后**不补**末尾换行。
//
// 逐字对应 PHP：file_put_contents($file, implode("\n", $final))
// —— 末尾空元素只有在原文件以 "\n" 结尾时才被保留。
func TestRemoveKeepsPHPNoTrailingNewlineQuirk(t *testing.T) {
	w, keyFile := newTestWriter(t, "/usr/bin/git")

	if err := os.MkdirAll(filepath.Dir(keyFile), 0o777); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	// 原文件没有末尾换行：explode 后没有末尾空元素，implode 后也没有。
	const content = "ssh-rsa AAAKEY1 deleteme\nssh-rsa OTHERKEY2 keepme"
	if err := os.WriteFile(keyFile, []byte(content), 0o666); err != nil {
		t.Fatalf("写初始文件失败: %v", err)
	}

	if err := w.Remove("AAAKEY1"); err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	if string(raw) != "ssh-rsa OTHERKEY2 keepme" {
		t.Fatalf("删除结果不符: %q", string(raw))
	}
}

// TestRemoveEmptyFile 确认空文件删除后仍是空文件（不产生多余换行）。
func TestRemoveEmptyFile(t *testing.T) {
	w, keyFile := newTestWriter(t, "/usr/bin/git")

	if err := os.MkdirAll(filepath.Dir(keyFile), 0o777); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(keyFile, nil, 0o666); err != nil {
		t.Fatalf("写初始文件失败: %v", err)
	}

	if err := w.Remove("ANYKEY"); err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("空文件删除后应仍为空, got %q", string(raw))
	}
}

// TestRemoveEmptyKeyRejected 确认空 key 被拒绝（PHP 侧同样直接返回 FALSE）。
func TestRemoveEmptyKeyRejected(t *testing.T) {
	w, _ := newTestWriter(t, "/usr/bin/git")
	if err := w.Remove(""); err == nil {
		t.Fatalf("空 key 应被拒绝")
	}
}

// TestAddThenRemoveRoundTrip 确认增删往返后文件回到原样。
func TestAddThenRemoveRoundTrip(t *testing.T) {
	w, keyFile := newTestWriter(t, "/usr/bin/git")

	const legacy = "ssh-rsa LEGACYKEY legacy\n"
	if err := os.MkdirAll(filepath.Dir(keyFile), 0o777); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(keyFile, []byte(legacy), 0o666); err != nil {
		t.Fatalf("写初始文件失败: %v", err)
	}

	if err := w.Add("UKEY1", "ssh-rsa", "AAAKEY1", "first"); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}
	if err := w.Remove("AAAKEY1"); err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	if string(raw) != legacy {
		t.Fatalf("往返后文件应回到原样\n got: %q\nwant: %q", string(raw), legacy)
	}
}
