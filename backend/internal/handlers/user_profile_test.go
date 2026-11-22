package handlers

import (
	"testing"
)

// TestRepositoryListViewEmptyIsNonNil 锁定共享列表视图的空输入边界。
//
// 返回 nil 切片会被 JSON 序列化成 null，前端按数组消费会炸。
func TestRepositoryListViewEmptyIsNonNil(t *testing.T) {
	// 空列表在触碰任何仓储之前就返回，因此无需构造数据库。
	got, err := repositoryListView(nil, nil, nil, "viewer", nil, "", "")
	if err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	if got == nil {
		t.Fatal("空输入返回 nil 切片，会被序列化成 null")
	}
	if len(got) != 0 {
		t.Fatalf("空输入条数 = %d, 期望 0", len(got))
	}
}
