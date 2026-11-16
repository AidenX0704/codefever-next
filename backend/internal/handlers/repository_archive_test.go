package handlers

import "testing"

// 归档下载的**纯逻辑**用例。

// TestSanitizeArchiveFilename 锁定下载文件名的字符白名单。
//
// Content-Disposition 是引号包裹的响应头：引号/反斜杠/换行会破坏
// 头结构，"/" 会把浏览器导向子目录 —— 这四类必须被替换掉；
// 中文等合法字符必须原样保留。
func TestSanitizeArchiveFilename(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"demorepo", "demorepo"},
		{"my-repo", "my-repo"},
		{"仓库名", "仓库名"},
		{`ha"ck`, "ha-ck"},
		{"a/b\\c", "a-b-c"},
		{"line\nbreak", "line-break"},
	}
	for _, c := range cases {
		if got := sanitizeArchiveFilename(c.in); got != c.want {
			t.Errorf("sanitizeArchiveFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// 全部字符都被替换掉时不能给空文件名。
	if got := sanitizeArchiveFilename("///"); got != "archive" {
		t.Errorf("全非法输入应回退为 archive, got %q", got)
	}
}

// TestArchiveFormatsTable 锁定 format 参数与响应头的对应关系。
func TestArchiveFormatsTable(t *testing.T) {
	zip, ok := archiveFormats["zip"]
	if !ok {
		t.Fatal("zip 应在白名单里")
	}
	if zip.gitFormat != "zip" || zip.mime != "application/zip" || zip.ext != ".zip" {
		t.Errorf("zip 的映射不正确: %+v", zip)
	}
	tgz, ok := archiveFormats["tar.gz"]
	if !ok {
		t.Fatal("tar.gz 应在白名单里")
	}
	// tar.gz 的 git 格式是 tar，gzip 由响应侧包 —— 与 gitsvc.Archive 的约定一致。
	if tgz.gitFormat != "tar" || tgz.mime != "application/gzip" || tgz.ext != ".tar.gz" {
		t.Errorf("tar.gz 的映射不正确: %+v", tgz)
	}
	if _, ok := archiveFormats["tar"]; ok {
		t.Error("裸 tar 不应作为端点格式暴露（用 tar.gz）")
	}
}
