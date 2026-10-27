package repo

import (
	"strings"
	"testing"
)

// TestVisibleRepositoryPredicatePlaceholders 守护"占位符数量 == 参数数量"。
//
// 这段 SQL 是手工拼接的（要用同一条可见性判定喂给三个查询），最典型的错误是
// 少写或多写一个 `?`：编译期不报错、单测不报错，**只在真实数据库上运行时报错**
// （这次就是这么撞到 `DISTINCT + ORDER BY` 那个问题的）。这条用例把数量钉死。
func TestVisibleRepositoryPredicatePlaceholders(t *testing.T) {
	fragment, args := visibleRepositoryPredicate("r", "u-key")
	if got, want := strings.Count(fragment, "?"), len(args); got != want {
		t.Fatalf("占位符 %d 个，参数 %d 个，不匹配：%s", got, want, fragment)
	}
	// 别名必须真的被用上：否则子查询会与外表同表冲突。
	if !strings.Contains(fragment, "r.u_key") || !strings.Contains(fragment, "r.r_key") {
		t.Fatalf("别名未生效：%s", fragment)
	}
	// 三种可见来源都必须出现在判定里（所有者 / 直接成员 / 组成员）。
	for _, needle := range []string{"r.u_key = ?", "cc_repository_members", "cc_group_members"} {
		if !strings.Contains(fragment, needle) {
			t.Fatalf("可见性判定缺少 %q：%s", needle, fragment)
		}
	}
}

// TestTrimSearchKeyword 锁定关键词规范化的两条规则。
//
// 为什么不截断：截断会在用户不知情的情况下改变搜索语义（"搜了但没搜全"比"拒绝"更难排查）。
// 超长关键词由 handler 按 errorcode 拒绝，这里只负责把空白压平。
func TestTrimSearchKeyword(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  order-svc  ", "order-svc"},
		{"order   svc", "order svc"},
		{"\tfoo\nbar ", "foo bar"},
		{"中文　全角空格", "中文 全角空格"}, // U+3000 也算 unicode 空白，会被压成半角空格
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		if got := TrimSearchKeyword(tc.in); got != tc.want {
			t.Errorf("TrimSearchKeyword(%q) = %q, 期望 %q", tc.in, got, tc.want)
		}
	}
}
