package domain

import "testing"

// TestClampPaging 覆盖性能整改 C1 引入的分页收敛口径。
//
// 回归背景：整改前全站 pagesize 只有下限、没有上限，`?pagesize=20000`
// 会原样进入 LIMIT，而合并请求列表对每行都要跑一次 `git log -1` 子进程。
func TestClampPaging(t *testing.T) {
	cases := []struct {
		name                   string
		page, pagesize         int
		wantPage, wantPageSize int
	}{
		{"缺省值归一到 1/20", 0, 0, 1, PageSizeDefault},
		{"负值归一到 1/20", -5, -1, 1, PageSizeDefault},
		{"正常值原样保留", 3, 50, 3, 50},
		{"上限边界保留", 1, PageSizeMax, 1, PageSizeMax},
		{"超过上限被截断", 1, PageSizeMax + 1, 1, PageSizeMax},
		{"极端值被截断", 1, 1000000, 1, PageSizeMax},
		{"page 超过下限不影响 pagesize", 999, 20, 999, 20},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotPage, gotSize := ClampPaging(tc.page, tc.pagesize)
			if gotPage != tc.wantPage || gotSize != tc.wantPageSize {
				t.Errorf("ClampPaging(%d, %d) = (%d, %d), 期望 (%d, %d)",
					tc.page, tc.pagesize, gotPage, gotSize, tc.wantPage, tc.wantPageSize)
			}
		})
	}
}

// TestAdminPagingMatchesClamp 断言 admin 域的口径与全站一致。
func TestAdminPagingMatchesClamp(t *testing.T) {
	page, pagesize := AdminPaging(0, 1000000)
	if page != 1 || pagesize != PageSizeMax {
		t.Errorf("AdminPaging(0, 1000000) = (%d, %d), 期望 (1, %d)", page, pagesize, PageSizeMax)
	}
	// 缺省仍是 20（PHP 语义），不能因为加了上限而改变默认值。
	page, pagesize = AdminPaging(0, 0)
	if page != 1 || pagesize != AdminPageSizeDefault {
		t.Errorf("AdminPaging(0, 0) = (%d, %d), 期望 (1, %d)", page, pagesize, AdminPageSizeDefault)
	}
}

// TestPageSizeMaxMatchesAdmin 断言两个上限常量没有分叉。
func TestPageSizeMaxMatchesAdmin(t *testing.T) {
	if PageSizeMax != AdminPageSizeMax {
		t.Errorf("PageSizeMax(%d) 与 AdminPageSizeMax(%d) 不一致，分页口径必须唯一",
			PageSizeMax, AdminPageSizeMax)
	}
}
