package handlers

import "testing"

// TestClipSearchBucket 锁定"多取一条 → 裁回上限 + 标记截断"的语义。
//
// 三类结果共用这一个函数，若它的边界写错，前端会同时出现两种错误：
// ①明明还有更多结果却不说；②说"还有更多"但下一页什么都没有。
func TestClipSearchBucket(t *testing.T) {
	cases := []struct {
		name      string
		size      int
		wantLen   int
		truncated bool
	}{
		{name: "空结果", size: 0, wantLen: 0, truncated: false},
		{name: "未达上限", size: searchBucketLimit - 1, wantLen: searchBucketLimit - 1, truncated: false},
		{name: "正好等于上限（不多取到第 11 条）", size: searchBucketLimit, wantLen: searchBucketLimit, truncated: false},
		{name: "多取到一条 → 截断", size: searchBucketLimit + 1, wantLen: searchBucketLimit, truncated: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := make([]int, tc.size)
			for i := range list {
				list[i] = i
			}
			got, truncated := clipSearchBucket(list)
			if len(got) != tc.wantLen {
				t.Errorf("裁剪后条数 = %d, 期望 %d", len(got), tc.wantLen)
			}
			if truncated != tc.truncated {
				t.Errorf("truncated = %v, 期望 %v", truncated, tc.truncated)
			}
		})
	}

	// nil 切片必须变成空切片：否则 JSON 会序列化成 null，前端按数组消费会炸。
	empty, _ := clipSearchBucket[int](nil)
	if empty == nil {
		t.Fatal("nil 输入返回 nil 切片，会被序列化成 null")
	}
}
