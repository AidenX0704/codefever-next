package domain

import "testing"

// TestBranchNameValid 校验分支名校验规则。
//
// 逐字复刻 application/controllers/api/repository.php:1419 的
// preg_match('/^\w[\w\/\.\-]{0,29}$/', $name)。
func TestBranchNameValid(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"master", true},
		{"feature/x", true},
		{"a.b-c_d", true},
		{"a", true},
		// 30 个字符：首字符 + 29 个后续字符。
		{"a12345678901234567890123456789", true},
		// 31 个字符：超长。
		{"a123456789012345678901234567890", false},
		{"", false},
		{"-leading", false},
		{".leading", false},
		{"/leading", false},
		{"has space", false},
		{"中文", false},
		{"with;semicolon", false},
	}
	for _, c := range cases {
		if got := BranchNameValid(c.name); got != c.want {
			t.Errorf("BranchNameValid(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestTagNameValid 校验标签名校验规则。
//
// 逐字复刻 application/controllers/api/repository.php:1933 的
// preg_match('/^\w+(\.\w+)*$/', $name)。
func TestTagNameValid(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"v1.0", true},
		{"v1", true},
		{"v1.0.1", true},
		{"release_2024", true},
		{"", false},
		{"v1.", false},
		{".v1", false},
		{"v1..0", false},
		{"v 1", false},
		{"v1/0", false},
		{"v1-0", false},
	}
	for _, c := range cases {
		if got := TagNameValid(c.name); got != c.want {
			t.Errorf("TagNameValid(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestPHPTrim 校验 trim 的复刻。
//
// PHP 的 trim() 默认字符集是 " \t\n\r\0\x0B"，不含 Unicode 空白。
func TestPHPTrim(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"  master  ", "master"},
		{"\t\nmaster\r\n", "master"},
		{"\x00master\x0B", "master"},
		{"master", "master"},
		{"   ", ""},
		{"\u00a0master\u00a0", "\u00a0master\u00a0"},
	}
	for _, c := range cases {
		if got := PHPTrim(c.in); got != c.want {
			t.Errorf("PHPTrim(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestWebhookEventMatches 校验 webhook 事件匹配。
//
// 逐字对应 application/models/repository_model.php:2233-2236：
// explode(',', $webhook['rw_events']) + in_array($eventType, $eventTypes)。
func TestWebhookEventMatches(t *testing.T) {
	cases := []struct {
		events string
		event  string
		want   bool
	}{
		{"repo:create", "repo:create", true},
		{"repo:create,repo:remove", "repo:remove", true},
		{"repo:create, repo:remove", "repo:remove", false}, // 逗号后有空格，不匹配
		{"repo:create", "repo:remove", false},
		{"repo:crea", "repo:create", false}, // 精确匹配，不做前缀匹配
		{"", "repo:create", false},
	}
	for _, c := range cases {
		if got := WebhookEventMatches(c.events, c.event); got != c.want {
			t.Errorf("WebhookEventMatches(%q, %q) = %v, want %v", c.events, c.event, got, c.want)
		}
	}
}

// TestMergeRequestCanTransition 校验合并请求状态流转。
//
// 只有"待合并"状态的 MR 能被关闭或合并；已合并 / 已关闭的不可再流转。
func TestMergeRequestCanTransition(t *testing.T) {
	cases := []struct {
		from int
		to   int
		want bool
	}{
		{MergeRequestStatusOpen, MergeRequestStatusClosed, true},
		{MergeRequestStatusOpen, MergeRequestStatusMerged, true},
		{MergeRequestStatusOpen, MergeRequestStatusOpen, true},
		{MergeRequestStatusMerged, MergeRequestStatusClosed, false},
		{MergeRequestStatusClosed, MergeRequestStatusMerged, false},
		{MergeRequestStatusMerged, MergeRequestStatusMerged, false},
		{MergeRequestStatusNone, MergeRequestStatusOpen, false},
	}
	for _, c := range cases {
		if got := MergeRequestCanTransition(c.from, c.to); got != c.want {
			t.Errorf("MergeRequestCanTransition(%d, %d) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

// TestProtectedBranchRoleAllowed 校验保护分支的角色取值校验。
//
// 逐字对应 application/controllers/api/repository.php:696-707。
func TestProtectedBranchRoleAllowed(t *testing.T) {
	pushOK := []string{"3", "4", "5", "6"}
	pushBad := []string{"", "0", "1", "2", "7", "abc"}
	for _, raw := range pushOK {
		if !ProtectedBranchPushRoleAllowed(raw, true) {
			t.Errorf("ProtectedBranchPushRoleAllowed(%q) = false, want true", raw)
		}
	}
	for _, raw := range pushBad {
		if ProtectedBranchPushRoleAllowed(raw, true) {
			t.Errorf("ProtectedBranchPushRoleAllowed(%q) = true, want false", raw)
		}
	}
	if ProtectedBranchPushRoleAllowed("3", false) {
		t.Error("ProtectedBranchPushRoleAllowed 缺失参数应为 false")
	}

	mergeOK := []string{"4", "5", "6"}
	mergeBad := []string{"", "0", "1", "2", "3", "7"}
	for _, raw := range mergeOK {
		if !ProtectedBranchMergeRoleAllowed(raw, true) {
			t.Errorf("ProtectedBranchMergeRoleAllowed(%q) = false, want true", raw)
		}
	}
	for _, raw := range mergeBad {
		if ProtectedBranchMergeRoleAllowed(raw, true) {
			t.Errorf("ProtectedBranchMergeRoleAllowed(%q) = true, want false", raw)
		}
	}
}

// TestNormalizeRepositoryWebhook 校验 webhook 的归一化形态。
//
// 逐字对应 application/models/repository_model.php:2124-2142。
func TestNormalizeRepositoryWebhook(t *testing.T) {
	name := "root"
	w := &RepositoryWebhook{
		Key:     "wk",
		URL:     "http://example.com/hook",
		Secret:  "s3cr3t",
		Events:  "repo:create",
		Active:  1,
		Updated: "2024-01-02 03:04:05",
	}
	view := NormalizeRepositoryWebhook(w, &name)
	if view.ID != "wk" || view.URL != "http://example.com/hook" {
		t.Errorf("归一化结果异常: %+v", view)
	}
	if view.Secret != "s3cr3t" {
		t.Errorf("secret 应逐字保留（前端要回显 webhook 配置），实际 %q", view.Secret)
	}
	if view.User == nil || *view.User != "root" {
		t.Errorf("user 应为 root，实际 %v", view.User)
	}
	if view.Updated != 1704164645 {
		t.Errorf("updated = %d, want 1704164645", view.Updated)
	}

	// 单条查询没有 JOIN cc_users → user 为 null。
	view = NormalizeRepositoryWebhook(w, nil)
	if view.User != nil {
		t.Errorf("user 应为 null，实际 %v", *view.User)
	}
}

// TestNormalizeRepositoryWebhookLogs 校验投递日志的归一化形态。
func TestNormalizeRepositoryWebhookLogs(t *testing.T) {
	logs := []RepositoryWebhookLog{
		{ID: "l1", Webhook: "wk", Start: 1.5, End: 2.5, Status: 200, Created: "2024-01-02 03:04:05"},
		{ID: "l2", Webhook: "wk", Start: 1, End: 2, Status: 500, Created: "2024-01-02 03:04:06"},
	}
	views := NormalizeRepositoryWebhookLogs(logs)
	if len(views) != 2 {
		t.Fatalf("日志数量 = %d, want 2", len(views))
	}
	if !views[0].Success {
		t.Error("status 200 的日志 success 应为 true")
	}
	if views[1].Success {
		t.Error("status 500 的日志 success 应为 false")
	}
	if views[0].Created != "2024-01-02 03:04:05" {
		t.Errorf("created 应保留原始字符串，实际 %q", views[0].Created)
	}
}
