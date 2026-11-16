package repo

import (
	"testing"
)

// MR 评论长度的**纯逻辑**用例（不需要数据库）。

// TestMRCommentBodyAllowed 锁定评论正文 1..10000 字的边界。
//
// 上限按字符数而不是字节数 —— 中文评论 10000 字是 30000 字节，
// text 列装得下，应用层的约束是给"一次评论"定合理边界。
func TestMRCommentBodyAllowed(t *testing.T) {
	if MRCommentBodyAllowed("") {
		t.Error("空评论应当被拒绝")
	}
	if !MRCommentBodyAllowed("a") {
		t.Error("1 字评论应当被接受")
	}
	if !MRCommentBodyAllowed(makeString("中", 10000)) {
		t.Error("10000 字评论应当被接受")
	}
	if MRCommentBodyAllowed(makeString("中", 10001)) {
		t.Error("10001 字评论应当被拒绝")
	}
}

// makeString 拼接 n 个 s。
func makeString(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
