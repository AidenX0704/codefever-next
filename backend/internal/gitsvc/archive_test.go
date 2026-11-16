package gitsvc

import (
	"strings"
	"testing"
)

// 本文件覆盖归档下载（/api/repository/archive 的 git 侧）的**纯逻辑**部分。
//
// 关键不变量与 commitListArgs 的断言同源：用户输入（ref）只占一个
// argv 元素且以 `--` 分隔 —— 即使 ref 以 "-" 开头，也只是被
// rejectOptionLike 拒绝，不存在被 shell / git 解释成选项的通道。

// TestArchiveArgs 断言 argv 的形状。
func TestArchiveArgs(t *testing.T) {
	cases := []struct {
		format string
		ref    string
		want   []string
	}{
		{"zip", "master", []string{"archive", "--format=zip", "--", "master"}},
		{"tar", "v1.0", []string{"archive", "--format=tar", "--", "v1.0"}},
		{"zip", "f67cfee73d75172d4e7afd77c3efa310b460fa1b",
			[]string{"archive", "--format=zip", "--", "f67cfee73d75172d4e7afd77c3efa310b460fa1b"}},
		// ref 以 "-" 开头（恶意输入的典型形态）：仍然只是一个 argv 元素，
		// 不会拆成 git 的选项 —— 真正执行前会被 rejectOptionLike 拦截。
		{"zip", "--all", []string{"archive", "--format=zip", "--", "--all"}},
	}
	for _, c := range cases {
		got := archiveArgs(c.format, c.ref)
		if len(got) != len(c.want) {
			t.Fatalf("archiveArgs(%q, %q) = %v, want %v", c.format, c.ref, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("archiveArgs(%q, %q)[%d] = %q, want %q", c.format, c.ref, i, got[i], c.want[i])
			}
		}
	}
}

// TestArchiveFormats 断言格式白名单与 /api/repository/archive 的约定一致。
//
// 白名单的键是 **git --format 值**（"zip" / "tar"）；HTTP 参数 "tar.gz"
// 由 handler 翻译成 "tar" 后传入。"tar.gz" 本身不是合法的 git --format
// （git 会直接报错），必须在拒绝列表里 —— 漏在白名单里会让 tar.gz 请求
// 拼出 `--format=tar.gz` 而非"裸 tar + gzip.Writer"。
func TestArchiveFormats(t *testing.T) {
	if !ArchiveFormats["zip"] || !ArchiveFormats["tar"] {
		t.Error("白名单应包含 zip 与 tar（git --format 值）")
	}
	for _, unsupported := range []string{"", "tar.gz", "rar", "zip bomb", "ZIP"} {
		if ArchiveFormats[unsupported] {
			t.Errorf("格式 %q 不应被白名单接受", unsupported)
		}
	}
}

// TestArchiveInputValidation 断言入参校验在真正拉起 git 之前拦截。
//
// 这些用例不需要真实仓库：合法格式 + 空 ref / 非法格式在参数检查阶段
// 就返回错误，不会走到 exec。
func TestArchiveInputValidation(t *testing.T) {
	s := &Service{}
	if err := s.Archive(nil, "/nonexistent", "zip", "", &strings.Builder{}); err == nil {
		t.Error("空 ref 应当被拒绝")
	}
	if err := s.Archive(nil, "/nonexistent", "7z", "master", &strings.Builder{}); err == nil {
		t.Error("白名单之外的格式应当被拒绝")
	}
	if err := s.Archive(nil, "", "zip", "master", &strings.Builder{}); err == nil {
		t.Error("空仓库路径应当被拒绝")
	}
	// ref 以 "-" 开头：git 会把它解释成选项，必须拒绝。
	if err := s.Archive(nil, "/nonexistent", "zip", "--all", &strings.Builder{}); err == nil {
		t.Error("以 - 开头的 ref 应当被拒绝")
	}
}
