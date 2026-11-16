package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"codefever-next/backend/internal/domain"
)

// TestNormalizeUserProfileOmitsPrivateFields 锁定个人主页"只出公开资料"的口径。
//
// 背景：个人主页是任何登录用户可看的页面。若有人图省事直接复用
// NormalizeUser（带 email / phone / MFA / 未读数），"能看主页"就升级成了
// "能看联系方式" —— 这是隐私泄露，不是显示问题。该用例对输出做 JSON
// 级别的键集合断言：多一个键都会失败。
func TestNormalizeUserProfileOmitsPrivateFields(t *testing.T) {
	team := "平台组"
	role := "后端"
	location := "远程"
	u := &domain.User{
		Key:      "u-1",
		Email:    "secret@codefever.cn",
		Name:     "alice",
		Team:     &team,
		Role:     &role,
		Location: &location,
		Admin:    true,
		Created:  time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC),
	}

	raw, err := json.Marshal(domain.NormalizeUserProfile(u))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	want := map[string]bool{
		"name": false, "icon": false, "bio": false, "team": false,
		"role": false, "location": false, "created": false, "admin": false,
	}
	for key := range out {
		if _, ok := want[key]; !ok {
			t.Fatalf("公开资料出现未知键 %q（原始输出: %s）", key, raw)
		}
		delete(want, key)
	}
	for key := range want {
		t.Fatalf("公开资料缺少键 %q（原始输出: %s）", key, raw)
	}

	// 私密字段即便将来有人加回来，也不该带着值混出去。
	for _, secret := range []string{"email", "phoneCode", "phoneNumber", "mfaEnabled", "unReadNotification", "emails"} {
		if _, ok := out[secret]; ok {
			t.Fatalf("公开资料出现私密字段 %q", secret)
		}
	}

	if out["name"] != "alice" {
		t.Fatalf("name = %v, 期望 alice", out["name"])
	}
}

// TestRepositoryListViewEmptyIsNonNil 锁定共享列表视图的空输入边界。
//
// repository/list 与个人主页共用这段逻辑；返回 nil 切片会被 JSON 序列化成
// null，前端按数组消费会炸（与 filterMergeRequestsByRole 的断言同一理由）。
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
