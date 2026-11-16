package events

import (
	"context"
	"testing"

	"codefever-next/backend/internal/domain"
)

// 合并请求评论事件的**纯逻辑**用例。
//
// 该事件是本仓库新增（PHP 侧没有评论能力），不占 PHP 契约基线；
// 这里的用例锁定它与既有通知流水线的接缝：模板、字段、受众与过滤。

// commentEvent 构造一个评论事件载荷。
func commentEvent() domain.EventPayload {
	return domain.EventPayload{
		ID:          "ev00000000000000000000000000001",
		Type:        string(domain.EventTypeMERGE_REQUEST_COMMENT),
		GeneratedBy: "user00000000000000000000000000ab",
		Language:    domain.LanguageCN,
		Data: map[string]any{
			"gKey":  "group00000000000000000000000001",
			"rKey":  "repo000000000000000000000000001",
			"id":    7,
			"mrKey": "mr00000000000000000000000000001",
			"body":  "这里少了一个分号",
		},
	}
}

// TestCommentEventRegistered 锁定评论事件在通知流水线里的登记。
func TestCommentEventRegistered(t *testing.T) {
	evType := string(domain.EventTypeMERGE_REQUEST_COMMENT)

	// 通知 handler 必须订阅它。
	subscribed := false
	for _, t2 := range notificationEventTypes() {
		if string(t2) == evType {
			subscribed = true
		}
	}
	if !subscribed {
		t.Error("notificationEventTypes 未订阅 mergeRequest:comment")
	}

	// 中英文模板都要有（评论是用户可见内容，缺一套文案就是缺一半用户的通知）。
	for _, lang := range []string{domain.LanguageCN, domain.LanguageEN} {
		tpl, ok := notificationTemplates[lang][evType]
		if !ok {
			t.Errorf("语言 %s 缺少评论事件模板", lang)
			continue
		}
		if tpl.mailTitle == "" || tpl.mailBody == "" {
			t.Errorf("语言 %s 的评论模板不完整: %+v", lang, tpl)
		}
	}

	// 字段表与受众选项。
	if len(notificationFieldData[evType]) == 0 {
		t.Error("notificationFieldData 缺少评论事件的字段表")
	}
	cue := notificationCueOptions[evType]
	if len(cue) != 1 || cue[0] != "mrParticipants" {
		t.Errorf("评论事件的受众选项 = %v, want [mrParticipants]", cue)
	}

	// 事件数据描述必须能通过校验（描述错了事件会被静默丢弃）。
	payload := commentEvent()
	if err := domain.ValidateEventData(
		domain.EventDataDescMERGE_REQUEST_COMMENT_D, domain.NormalizeEventData(payload.Data)); err != nil {
		t.Errorf("评论事件数据未通过描述校验: %v", err)
	}
}

// TestCommentEventComposition 校验评论通知的标题 / 正文 / 站内信内容。
func TestCommentEventComposition(t *testing.T) {
	ev := commentEvent()

	values := map[string]string{
		"users.u_name":            "张三",
		"groups.g_name":           "mygroup",
		"repositories.r_name":     "myrepo",
		"merge_requests.mr_title": "标题",
	}
	resolver := &templateResolver{
		ev: ev,
		lookupFn: func(_ context.Context, table, column, _ string) (string, error) {
			return values[table+"."+column], nil
		},
	}

	evType := string(domain.EventTypeMERGE_REQUEST_COMMENT)
	tpl := notificationTemplates[domain.LanguageCN][evType]
	fields := notificationFieldData[evType]
	cue := notificationCueOptions[evType]

	data, err := composeMessageData(context.Background(), resolver, tpl, fields, cue)
	if err != nil {
		t.Fatalf("组装消息失败: %v", err)
	}

	wantTitle := "【CodeFever Next】张三 评论了合并请求"
	if data.mailTitle != wantTitle {
		t.Errorf("标题 = %q, want %q", data.mailTitle, wantTitle)
	}
	// 正文必须带上评论原文 —— 没有原文的通知等于没说。
	wantBody := "张三 评论了合并请求 myrepo!7: 这里少了一个分号"
	if data.mailBody != wantBody {
		t.Errorf("正文 = %q, want %q", data.mailBody, wantBody)
	}
	if got := data.value("body"); got != "这里少了一个分号" {
		t.Errorf("站内信 body = %q, want 评论原文", got)
	}
}

// TestCommentEventFilterOtherRule 校验评论事件**不做**角色过滤。
//
// 受众已经由 mrParticipants 收敛为「创建者 + 评审人」，
// 再叠加角色过滤会把 Reporter 级的评审人漏掉。
func TestCommentEventFilterOtherRule(t *testing.T) {
	ev := commentEvent()
	affected := []notifyUser{
		{key: "u1", role: int(domain.RoleGuest)},
		{key: "u2", role: int(domain.RoleOwner)},
	}
	got := filterOtherRule(affected, ev)
	if len(got) != len(affected) {
		t.Errorf("评论事件不应做角色过滤: got %d, want %d", len(got), len(affected))
	}
}

// TestCommentEventIsMergeRequestEvent 锁定"关掉 MR 通知也不收评论"的语义。
func TestCommentEventIsMergeRequestEvent(t *testing.T) {
	if !isMergeRequestEvent(string(domain.EventTypeMERGE_REQUEST_COMMENT)) {
		t.Error("评论事件应按合并请求类事件处理（尊重用户的 MR 通知开关）")
	}
}
