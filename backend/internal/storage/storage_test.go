package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPathByFileName 锁定 PHP 的 getPathByFileName() 目录布局。
//
// 该布局决定了"文件名 → 磁盘路径"的映射，既有头像文件都按它存放，
// 一旦变更会导致全部历史头像失效。
//
// 注意：PHP 的 substr($name, $i, 1) 在超出长度时返回空串，
// 于是短文件名会产生空目录段（如 "abc" → "a/b/c///abc"），
// 落盘时会被文件系统折叠为 "a/b/c/abc" —— Go 侧直接跳过空段，结果一致。
func TestPathByFileName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"d24d2281d6354ec58b27167fe21a6e1e", filepath.Join("d", "2", "4", "d", "2", "d24d2281d6354ec58b27167fe21a6e1e")},
		{"abc", filepath.Join("a", "b", "c", "abc")},
		{"ab", filepath.Join("a", "b", "ab")},
		{"a", filepath.Join("a", "a")},
	}
	for _, c := range cases {
		if got := PathByFileName(c.name); got != c.want {
			t.Errorf("PathByFileName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
	if got := PathByFileName(""); got != "" {
		t.Errorf("PathByFileName(\"\") = %q, want 空串", got)
	}
}

// TestAbsolutePathBlocksEscape 验证路径穿越被拦截。
func TestAbsolutePathBlocksEscape(t *testing.T) {
	root := t.TempDir()
	d := NewDisk(root)

	if _, err := d.AbsolutePath("avatar", filepath.Join("..", "..", "etc", "passwd")); err == nil {
		t.Fatal("路径穿越应当被拒绝")
	}
	if _, err := d.AbsolutePath("avatar", "a/b/c/d/e/ok"); err != nil {
		t.Fatalf("正常路径不应被拒绝: %v", err)
	}
}

// TestSaveAndRead 验证写入与读取的往返。
func TestSaveAndRead(t *testing.T) {
	root := t.TempDir()
	d := NewDisk(root)

	key := "d24d2281d6354ec58b27167fe21a6e1e"
	content := []byte("\x89PNG\r\n\x1a\nfake-image")

	if err := d.Save("avatar", key, content); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	got, err := d.Read("avatar", key)
	if err != nil {
		t.Fatalf("Read 失败: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("读回内容不符: %q", got)
	}

	// 文件确实落在 PHP 约定的五级目录下。
	expected := filepath.Join(root, "avatar", "d", "2", "4", "d", "2", key)
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("文件未落在预期路径 %s: %v", expected, err)
	}

	// 不存在的文件返回 ErrNotFound。
	if _, err := d.Read("avatar", "ffffffffffffffffffffffffffffffff"); err != ErrNotFound {
		t.Fatalf("不存在文件应返回 ErrNotFound，实际 %v", err)
	}
}

// TestRemove 验证删除。
func TestRemove(t *testing.T) {
	root := t.TempDir()
	d := NewDisk(root)

	key := "abcdef1234567890abcdef1234567890"
	if err := d.Save("avatar", key, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := d.Remove("avatar", key); err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}
	if _, err := d.Read("avatar", key); err != ErrNotFound {
		t.Fatalf("删除后不应存在，实际 %v", err)
	}
	// 删除不存在的文件不报错（与 PHP 的 unlink 警告不同，这里选择幂等）。
	if err := d.Remove("avatar", key); err != nil {
		t.Fatalf("重复删除应幂等: %v", err)
	}
}
