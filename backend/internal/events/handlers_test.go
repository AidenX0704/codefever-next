package events

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/repo"
)

// ---------------------------------------------------------------------------
// 测试替身
// ---------------------------------------------------------------------------

// fakeRefuse 模拟 cc_notification_refuse 表。
//
// 它同时实现 PHP 侧的**级联语义**：拒收一个仓库组等价于拒收该组下的所有仓库
// （application/models/notification_model.php:180-206 在写入时级联，
// 这里在读取时等价实现，便于验证过滤行为）。
type fakeRefuse struct {
	// groupRefused[uKey] 是被拒收的仓库组集合。
	groupRefused map[string]map[string]bool
	// repoRefused[uKey] 是被拒收的仓库集合。
	repoRefused map[string]map[string]bool
	// repoGroup[rKey] 是仓库所属的仓库组。
	repoGroup map[string]string
}

// HasRefused 实现 refuseChecker。
func (f *fakeRefuse) HasRefused(_ context.Context, uKey string, typ int, target string) (bool, error) {
	if uKey == "" || target == "" {
		return false, nil
	}
	switch typ {
	case repo.NotificationRefuseGroup:
		return f.groupRefused[uKey][target], nil
	case repo.NotificationRefuseRepository:
		if f.repoRefused[uKey][target] {
			return true, nil
		}
		// 组级拒收 → 该组下的仓库一并拒收。
		gKey := f.repoGroup[target]
		if gKey != "" && f.groupRefused[uKey][gKey] {
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

// RefusedFor 实现 refuseChecker 的批量入口（性能整改 C3）。
//
// 必须与 HasRefused 的判定**逐条等价**：这个测试替身存在的意义，正是让
// 批量改造后的过滤结果与原逐条查询版完全一致。
func (f *fakeRefuse) RefusedFor(ctx context.Context, uKeys []string, gKey, rKey string) (map[string]repo.RefuseFlags, error) {
	out := make(map[string]repo.RefuseFlags, len(uKeys))
	for _, uKey := range uKeys {
		g, err := f.HasRefused(ctx, uKey, repo.NotificationRefuseGroup, gKey)
		if err != nil {
			return nil, err
		}
		r, err := f.HasRefused(ctx, uKey, repo.NotificationRefuseRepository, rKey)
		if err != nil {
			return nil, err
		}
		if g || r {
			out[uKey] = repo.RefuseFlags{Group: g, Repository: r}
		}
	}
	return out, nil
}

// refuseHandler 造一个只带拒收过滤器的通知处理器。
func refuseHandler(refuse refuseChecker) *notificationHandler {
	return &notificationHandler{refuse: refuse, log: discardLogger()}
}

// discardLogger 返回一个丢弃全部日志的 logger。
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// 受众过滤
// ---------------------------------------------------------------------------

// TestFilterOtherRuleRoleThreshold 校验角色阈值。
//
// 逐字对应 UserNotificationHandler.php:202-231：
//   - 合并请求创建 / 关闭 / 合并只通知 role > ROLE_REPORTER(2)；
//   - 评审通过只通知 role > ROLE_DEVELOPER(3)；
//   - 指派评审人不做角色过滤。
func TestFilterOtherRuleRoleThreshold(t *testing.T) {
	users := []notifyUser{
		{key: "u1", role: int(domain.RoleGuest)},
		{key: "u2", role: int(domain.RoleReporter)},
		{key: "u3", role: int(domain.RoleDeveloper)},
		{key: "u4", role: int(domain.RoleMaintainer)},
		{key: "u5", role: int(domain.RoleOwner)},
	}

	cases := []struct {
		name   string
		evType domain.EventType
		want   []string
	}{
		{
			name:   "合并请求创建只通知 DEVELOPER 及以上",
			evType: domain.EventTypeMERGE_REQUEST_CREATE,
			want:   []string{"u3", "u4", "u5"},
		},
		{
			name:   "合并请求关闭只通知 DEVELOPER 及以上",
			evType: domain.EventTypeMERGE_REQUEST_CLOSE,
			want:   []string{"u3", "u4", "u5"},
		},
		{
			name:   "合并请求合并只通知 DEVELOPER 及以上",
			evType: domain.EventTypeMERGE_REQUEST_MERGE,
			want:   []string{"u3", "u4", "u5"},
		},
		{
			name:   "评审通过只通知 MAINTAINER 及以上",
			evType: domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW,
			want:   []string{"u4", "u5"},
		},
		{
			name:   "指派评审人不过滤角色",
			evType: domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE,
			want:   []string{"u1", "u2", "u3", "u4", "u5"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := domain.EventPayload{Type: string(c.evType)}
			got := filterOtherRule(users, ev)
			if len(got) != len(c.want) {
				t.Fatalf("人数 = %d（%v），期望 %d（%v）", len(got), keysOf(got), len(c.want), c.want)
			}
			for i, u := range got {
				if u.key != c.want[i] {
					t.Errorf("第 %d 个用户 = %q，期望 %q", i, u.key, c.want[i])
				}
			}
		})
	}
}

// TestFilterOtherRuleReviewersClearBroadcast 校验"已指定评审人时清空广播"。
//
// 逐字对应 UserNotificationHandler.php:217-219：
// 合并请求**创建**事件里带了 reviewers 就把广播名单清空（改为定向通知）。
func TestFilterOtherRuleReviewersClearBroadcast(t *testing.T) {
	users := []notifyUser{
		{key: "u3", role: int(domain.RoleDeveloper)},
		{key: "u4", role: int(domain.RoleMaintainer)},
	}

	withReviewers := domain.EventPayload{
		Type: string(domain.EventTypeMERGE_REQUEST_CREATE),
		Data: map[string]any{"reviewers": "aaaa00000000000000000000000001"},
	}
	if got := filterOtherRule(users, withReviewers); len(got) != 0 {
		t.Errorf("指定评审人后广播名单应被清空，实际保留 %v", keysOf(got))
	}

	withoutReviewers := domain.EventPayload{
		Type: string(domain.EventTypeMERGE_REQUEST_CREATE),
		Data: map[string]any{},
	}
	if got := filterOtherRule(users, withoutReviewers); len(got) != 2 {
		t.Errorf("未指定评审人时应保留 %d 人，实际 %v", 2, keysOf(got))
	}

	// 只有"创建"事件会清空，关闭 / 合并事件即使带 reviewers 也照常广播。
	closeEvent := domain.EventPayload{
		Type: string(domain.EventTypeMERGE_REQUEST_CLOSE),
		Data: map[string]any{"reviewers": "aaaa00000000000000000000000001"},
	}
	if got := filterOtherRule(users, closeEvent); len(got) != 2 {
		t.Errorf("合并请求关闭不应清空广播，实际 %v", keysOf(got))
	}
}

// TestHasReviewersTruthy 校验 reviewers 字段的真值判断。
func TestHasReviewersTruthy(t *testing.T) {
	cases := []struct {
		name string
		data map[string]any
		want bool
	}{
		{"字段缺失", map[string]any{}, false},
		{"空字符串", map[string]any{"reviewers": ""}, false},
		{"字符串 0 是假值", map[string]any{"reviewers": "0"}, false},
		{"非空字符串", map[string]any{"reviewers": "u1,u2"}, true},
		{"数组", map[string]any{"reviewers": []any{"u1"}}, true},
		{"空数组", map[string]any{"reviewers": []any{}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasReviewers(c.data); got != c.want {
				t.Errorf("hasReviewers = %v，期望 %v", got, c.want)
			}
		})
	}
}

// TestFilterUserNotificationSettingBitmask 校验位掩码语义。
//
// 逐字对应 UserNotificationHandler.php:234-266：
// **位 = 1 表示关闭**（NOTIFICATION_STATUS_MR = 1，NOTIFICATION_STATUS_EMAIL = 2）。
func TestFilterUserNotificationSettingBitmask(t *testing.T) {
	ev := domain.EventPayload{Type: string(domain.EventTypeMERGE_REQUEST_CREATE)}

	cases := []struct {
		name       string
		status     int
		wantIn     bool
		wantEmails []string
	}{
		{"默认全开", 0, true, []string{"a@x.cn"}},
		{"关闭合并请求通知", repo.NotificationStatusMR, false, nil},
		{"关闭邮件通知", repo.NotificationStatusEmail, true, nil},
		{"两个都关闭", repo.NotificationStatusMR | repo.NotificationStatusEmail, false, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			users := []notifyUser{{key: "u1", email: "a@x.cn", notificationStatus: c.status}}
			internal, emails := filterUserNotificationSetting(users, ev)
			if (len(internal) > 0) != c.wantIn {
				t.Errorf("站内信 = %v，期望收到 = %v", keysOf(internal), c.wantIn)
			}
			if len(emails) != len(c.wantEmails) {
				t.Fatalf("邮件 = %v，期望 %v", emails, c.wantEmails)
			}
			for i := range emails {
				if emails[i] != c.wantEmails[i] {
					t.Errorf("邮件[%d] = %q，期望 %q", i, emails[i], c.wantEmails[i])
				}
			}
		})
	}
}

// TestFilterProjectNotificationSettingCascade 校验拒收级联。
//
// 逐字对应 UserNotificationHandler.php:175-200：
// 命中组级或仓库级拒收记录即被排除；而 PHP 在写入拒收配置时会把组下所有仓库
// 一并写入拒收记录，因此"拒收一个组"等价于"该组下所有仓库都拒收"。
func TestFilterProjectNotificationSettingCascade(t *testing.T) {
	refuse := &fakeRefuse{
		groupRefused: map[string]map[string]bool{
			// uGroup 拒收了组 g1。
			"uGroup": {"g1": true},
			// uRepo 只拒收了仓库 r1。
			"uRepo": {},
		},
		repoRefused: map[string]map[string]bool{
			"uRepo": {"r1": true},
		},
		repoGroup: map[string]string{
			"r1": "g1",
			"r2": "g1",
			"r3": "g2",
		},
	}
	h := refuseHandler(refuse)

	users := []notifyUser{
		{key: "uGroup", role: int(domain.RoleOwner)},
		{key: "uRepo", role: int(domain.RoleOwner)},
		{key: "uNone", role: int(domain.RoleOwner)},
	}

	// 事件发生在组 g1 的仓库 r1 上：uGroup 拒收了组 g1，uRepo 拒收了仓库 r1。
	ev := domain.EventPayload{
		Type: string(domain.EventTypeMERGE_REQUEST_CREATE),
		Data: map[string]any{"gKey": "g1", "rKey": "r1"},
	}
	got, err := h.filterProjectNotificationSetting(context.Background(), users, ev)
	if err != nil {
		t.Fatalf("过滤失败: %v", err)
	}
	if len(got) != 1 || got[0].key != "uNone" {
		t.Errorf("过滤结果 = %v，期望只保留 uNone", keysOf(got))
	}

	// 组级拒收 → 该组下的**其他**仓库一并拒收（r2 属于 g1）。
	ev2 := domain.EventPayload{
		Type: string(domain.EventTypeMERGE_REQUEST_CREATE),
		Data: map[string]any{"gKey": "g1", "rKey": "r2"},
	}
	got2, err := h.filterProjectNotificationSetting(context.Background(), users, ev2)
	if err != nil {
		t.Fatalf("过滤失败: %v", err)
	}
	if len(got2) != 2 || got2[0].key != "uRepo" || got2[1].key != "uNone" {
		t.Errorf("过滤结果 = %v，期望排除 uGroup 后保留 uRepo 与 uNone", keysOf(got2))
	}

	// 事件发生在组 g2 的仓库 r3 上：组 g1 的拒收不影响它。
	ev3 := domain.EventPayload{
		Type: string(domain.EventTypeMERGE_REQUEST_CREATE),
		Data: map[string]any{"gKey": "g2", "rKey": "r3"},
	}
	got3, err := h.filterProjectNotificationSetting(context.Background(), users, ev3)
	if err != nil {
		t.Fatalf("过滤失败: %v", err)
	}
	if len(got3) != 3 {
		t.Errorf("过滤结果 = %v，期望三个人都保留", keysOf(got3))
	}
}

// TestDedupeNotifyUsers 校验候选名单的去重规则。
//
// 逐字对应 MessageGenerator.php:71-82：主键必须匹配 32 位 \w、
// 不能是事件触发者、同一个主键只保留第一次出现。
func TestDedupeNotifyUsers(t *testing.T) {
	users := []notifyUser{
		{key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", role: 5},
		{key: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", role: 5},
		{key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", role: 3},
		{key: "short", role: 5},
		{key: "cccccccccccccccccccccccccccccccc", role: 5},
	}
	got := dedupeNotifyUsers(users, "cccccccccccccccccccccccccccccccc")
	if len(got) != 2 {
		t.Fatalf("去重结果 = %v，期望 2 人", keysOf(got))
	}
	if got[0].key != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || got[1].key != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Errorf("去重结果 = %v，期望保留首次出现的两个人", keysOf(got))
	}
	if got[0].role != 5 {
		t.Errorf("首次出现的角色应保留 = %d，期望 5", got[0].role)
	}
}

// ---------------------------------------------------------------------------
// 活动类型编码
// ---------------------------------------------------------------------------

// TestActivityTypeMapping 校验"事件 → 活动类型"的十六进制区间编码。
//
// 逐字对应 application/libraries/service/Constant/ActivityType.php，
// 以及前端 www/view/src/config/Activity.js 的渲染分支。
func TestActivityTypeMapping(t *testing.T) {
	cases := []struct {
		name   string
		evType domain.EventType
		want   int
	}{
		{"仓库组创建", domain.EventTypeGROUP_CREATE, 0x0201},
		{"仓库组更名", domain.EventTypeGROUP_UPDATE_NAME, 0x0203},
		{"仓库组加成员", domain.EventTypeGROUP_ADD_MEMBER, 0x0205},
		{"仓库组删除", domain.EventTypeGROUP_REMOVE, 0x020A},
		{"仓库创建", domain.EventTypeREPO_CREATE, 0x0301},
		{"仓库 fork", domain.EventTypeREPO_FORK, 0x0302},
		{"仓库删除", domain.EventTypeREPO_REMOVE, 0x030B},
		{"分支创建", domain.EventTypeBRANCH_CREATE, 0x0403},
		{"默认分支变更", domain.EventTypeDEFAULT_BRANCH_CHANGE, 0x0405},
		{"保护分支规则创建", domain.EventTypePROTECTED_BRANCH_RULE_CREATE, 0x0406},
		{"标签创建", domain.EventTypeTAG_CREATE, 0x0503},
		{"标签删除", domain.EventTypeTAG_REMOVE, 0x0504},
		{"合并请求创建", domain.EventTypeMERGE_REQUEST_CREATE, 0x0701},
		{"合并请求评审", domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW, 0x0706},
		{"webhook 创建", domain.EventTypeWEBHOOK_CREATE, 0x0901},
		{"webhook 删除", domain.EventTypeWEBHOOK_DELETE, 0x0903},
	}

	// 组 / 仓库 / 分支 / 标签 / 合并请求 / webhook 六个事件族。
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := domain.EventPayload{
				Type:        string(c.evType),
				GeneratedBy: "u1",
				Data: map[string]any{
					"gKey": "g1", "rKey": "r1", "mrKey": "m1",
					"name": "n", "from": "a", "to": "b", "id": 1,
				},
			}

			var got int
			switch c.evType {
			case domain.EventTypeGROUP_CREATE, domain.EventTypeGROUP_UPDATE_NAME,
				domain.EventTypeGROUP_ADD_MEMBER, domain.EventTypeGROUP_REMOVE:
				got = buildGroupActivity(ev).aType
			case domain.EventTypeREPO_CREATE, domain.EventTypeREPO_FORK, domain.EventTypeREPO_REMOVE:
				got = buildRepoActivity(ev).aType
			case domain.EventTypeBRANCH_CREATE, domain.EventTypeDEFAULT_BRANCH_CHANGE,
				domain.EventTypePROTECTED_BRANCH_RULE_CREATE:
				got = buildBranchActivity(ev).aType
			case domain.EventTypeTAG_CREATE, domain.EventTypeTAG_REMOVE:
				got = buildTagActivity(ev).aType
			case domain.EventTypeMERGE_REQUEST_CREATE, domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW:
				got = buildMergeRequestActivity(ev).aType
			case domain.EventTypeWEBHOOK_CREATE, domain.EventTypeWEBHOOK_DELETE:
				got = buildWebhookActivity(ev).aType
			}

			if got != c.want {
				t.Errorf("活动类型 = %#x，期望 %#x", got, c.want)
			}
		})
	}
}

// TestActivityTypeRanges 校验活动类型的区间归属。
//
// 前端与后端都用区间判断活动分类：
//
//	"提交"     → 0x0401 / 0x0402 / 0x0501 / 0x0502
//	"合并请求" → 0x0700 < a_type < 0x0800
//	"成员"     → a_type < 0x0600 且排除上面四种
func TestActivityTypeRanges(t *testing.T) {
	commit := []int{ActivityTypeBranchPush, ActivityTypeBranchPushNew, ActivityTypeTagPush, ActivityTypeTagPushNew}
	for _, a := range commit {
		if a != 0x0401 && a != 0x0402 && a != 0x0501 && a != 0x0502 {
			t.Errorf("提交类活动类型 = %#x，不在约定集合内", a)
		}
	}
	if ActivityTypeBranchPush>>8 != 0x04 || ActivityTypeTagPush>>8 != 0x05 {
		t.Errorf("push 活动类型应落在 0x04xx / 0x05xx 区间")
	}
	if ActivityTypeMergeRequestCreate>>8 != 0x07 {
		t.Errorf("合并请求活动类型应落在 0x07xx 区间")
	}
	if ActivityTypeWebhookCreate>>8 != 0x09 {
		t.Errorf("webhook 活动类型应落在 0x09xx 区间")
	}
	// 成员分类：小于 0x0600 且排除提交类。
	member := []int{ActivityTypeGroupAddMember, ActivityTypeRepoChangeMemberRole}
	for _, a := range member {
		if a >= 0x0600 {
			t.Errorf("成员类活动类型 = %#x，应小于 0x0600", a)
		}
	}
}

// TestActivityDataShape 校验 a_data 的字段名与顺序。
//
// 前端 www/view/src/config/Activity.js 按 a_type 查表并读取 content 里的字段，
// 字段名变了前端就会白屏，因此这里逐字锁定。
func TestActivityDataShape(t *testing.T) {
	ev := domain.EventPayload{
		Type:        string(domain.EventTypeMERGE_REQUEST_CREATE),
		GeneratedBy: "u1",
		Data: map[string]any{
			"gKey": "g1", "rKey": "r1", "mrKey": "m1",
			"id": 7, "sourceBranch": "feature", "targetBranch": "master",
		},
	}
	row := buildMergeRequestActivity(ev)
	if row.data != `{"id":7,"sourceBranch":"feature","targetBranch":"master"}` {
		t.Errorf("合并请求活动的 a_data = %s", row.data)
	}
	if row.mrKey == nil || *row.mrKey != "m1" {
		t.Errorf("合并请求活动应带上 a_relative_mr_key")
	}

	// 角色变更：uid / name / email / to。
	roleEvent := domain.EventPayload{
		Type:        string(domain.EventTypeREPO_CHANGE_MEMBER_ROLE),
		GeneratedBy: "u1",
		Data: map[string]any{
			"gKey": "g1", "rKey": "r1", "uid": "u2", "name": "Bob", "email": "b@x.cn", "to": 3,
		},
	}
	roleRow := buildRepoActivity(roleEvent)
	if roleRow.data != `{"uid":"u2","name":"Bob","email":"b@x.cn","to":3}` {
		t.Errorf("角色变更活动的 a_data = %s", roleRow.data)
	}

	// "更新描述"用的是空数组（PHP 的 json_encode([])）。
	descEvent := domain.EventPayload{
		Type:        string(domain.EventTypeREPO_UPDATE_DESCRIPTION),
		GeneratedBy: "u1",
		Data:        map[string]any{"gKey": "g1", "rKey": "r1"},
	}
	if got := buildRepoActivity(descEvent).data; got != "[]" {
		t.Errorf("更新描述活动的 a_data = %s，期望 []", got)
	}

	// 组事件的 a_relative_r_key 是空字符串（不是 NULL）。
	groupEvent := domain.EventPayload{
		Type:        string(domain.EventTypeGROUP_CREATE),
		GeneratedBy: "u1",
		Data:        map[string]any{"gKey": "g1", "name": "团队"},
	}
	groupRow := buildGroupActivity(groupEvent)
	if groupRow.rKey == nil || *groupRow.rKey != "" {
		t.Errorf("组活动的 a_relative_r_key 应为空字符串，实际 %v", groupRow.rKey)
	}
	if groupRow.mrKey != nil {
		t.Errorf("组活动不应带 a_relative_mr_key")
	}
}

// TestPushActivityType 校验 push 事件的活动类型与 ref 解析。
//
// 逐字对应 ActivityHandler.php:141-178。
func TestPushActivityType(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		from string
		want int
	}{
		{"分支 push", "refs/heads/master", "aaaa", ActivityTypeBranchPush},
		{"新分支首次 push", "refs/heads/feature/x", zeroCommitHash, ActivityTypeBranchPushNew},
		{"标签 push", "refs/tags/v1.0", "aaaa", ActivityTypeTagPush},
		{"新标签首次 push", "refs/tags/v1.0", zeroCommitHash, ActivityTypeTagPushNew},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := pushRefPattern.FindStringSubmatch(c.ref)
			if m == nil {
				t.Fatalf("ref %q 解析失败", c.ref)
			}
			got := ActivityTypeBranchPush
			switch m[1] {
			case "heads":
				got = ActivityTypeBranchPush
				if c.from == zeroCommitHash {
					got = ActivityTypeBranchPushNew
				}
			case "tags":
				got = ActivityTypeTagPush
				if c.from == zeroCommitHash {
					got = ActivityTypeTagPushNew
				}
			}
			if got != c.want {
				t.Errorf("活动类型 = %#x，期望 %#x", got, c.want)
			}
			if m[2] != strings.TrimPrefix(strings.TrimPrefix(c.ref, "refs/heads/"), "refs/tags/") {
				t.Errorf("ref 名 = %q", m[2])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 模板占位符
// ---------------------------------------------------------------------------

// TestTemplatePlaceholders 校验消息模板的占位符替换。
//
// 逐字对应 application/libraries/service/Utility/MessageGenerator.php：
//
//	{{$user@users:u_name}}                → 用触发者主键查 users.u_name
//	{{$data->rKey@repositories:r_name}}    → 用事件数据里的 rKey 查 repositories.r_name
//	{{users:u_name}}                      → {table}:{pk} 规则，用触发者主键查表
//	{{$data->sourceBranch}}               → 事件数据里的字段
func TestTemplatePlaceholders(t *testing.T) {
	ev := domain.EventPayload{
		GeneratedBy: "user00000000000000000000000000ab",
		Data: map[string]any{
			"rKey":         "repo00000000000000000000000001",
			"sourceBranch": "feature/x",
			"id":           7,
		},
	}

	var lookups []string
	resolver := &templateResolver{
		ev: ev,
		lookupFn: func(_ context.Context, table, column, key string) (string, error) {
			lookups = append(lookups, table+"."+column+"="+key)
			switch table + "." + column {
			case "users.u_name":
				return "张三", nil
			case "repositories.r_name":
				return "myrepo", nil
			case "groups.g_name":
				return "mygroup", nil
			}
			return "", nil
		},
	}

	got, err := resolver.resolve(context.Background(),
		"{{$user@users:u_name}}|{{$data->rKey@repositories:r_name}}|{{$data->sourceBranch}}|{{users:u_name}}|{{$data->id}}")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	want := "张三|myrepo|feature/x|张三|7"
	if got != want {
		t.Errorf("解析结果 = %q，期望 %q", got, want)
	}

	// {table}:{pk} 规则必须用**事件触发者**的主键去查表。
	found := false
	for _, l := range lookups {
		if l == "users.u_name=user00000000000000000000000000ab" {
			found = true
		}
	}
	if !found {
		t.Errorf("查表记录 = %v，缺少按触发者主键的 {table}:{pk} 查询", lookups)
	}
}

// TestTemplateTablesWhitelist 校验模板只能引用白名单里的表与列。
func TestTemplateTablesWhitelist(t *testing.T) {
	resolver := &templateResolver{ev: domain.EventPayload{}}
	if _, err := resolver.resolveOne(context.Background(), "$user@users:u_password"); err == nil {
		t.Error("引用非白名单列应当报错")
	}
	if _, err := resolver.resolveOne(context.Background(), "$user@secrets:s_value"); err == nil {
		t.Error("引用非白名单表应当报错")
	}
}

// TestNotificationTemplatesLiteral 校验中英文案逐字一致。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:22-67。
func TestNotificationTemplatesLiteral(t *testing.T) {
	cn, ok := notificationTemplates[domain.LanguageCN][string(domain.EventTypeMERGE_REQUEST_CREATE)]
	if !ok {
		t.Fatal("缺少中文的合并请求创建文案")
	}
	if cn.mailTitle != "【CodeFever Next】{{$user@users:u_name}} 打开了合并请求" {
		t.Errorf("中文标题 = %q", cn.mailTitle)
	}
	if !strings.HasPrefix(cn.mailBody, "{{$user@users:u_name}} 打开了和并请求: ") {
		t.Errorf("中文正文 = %q", cn.mailBody)
	}

	en, ok := notificationTemplates[domain.LanguageEN][string(domain.EventTypeMERGE_REQUEST_CREATE)]
	if !ok {
		t.Fatal("缺少英文的合并请求创建文案")
	}
	if en.mailTitle != "【CodeFever Next】{{$user@users:u_name}} Open Merge Request" {
		t.Errorf("英文标题 = %q", en.mailTitle)
	}

	// 站内信字段名与顺序。
	fields := notificationFieldData[string(domain.EventTypeMERGE_REQUEST_CREATE)]
	wantKeys := []string{"user", "group", "repository", "sourceRepository", "sourceBranch",
		"targetBranch", "number", "title", "description", "created"}
	if len(fields) != len(wantKeys) {
		t.Fatalf("站内信字段数 = %d，期望 %d", len(fields), len(wantKeys))
	}
	for i, f := range fields {
		if f.key != wantKeys[i] {
			t.Errorf("字段[%d] = %q，期望 %q", i, f.key, wantKeys[i])
		}
	}
}

// TestEmailNotifyTemplate 校验邮件正文结构。
//
// 逐字对应 application/libraries/service/MessageService/Email/EmailTemplate.php:13-16。
func TestEmailNotifyTemplate(t *testing.T) {
	got := emailNotifyTemplate("正文", "http://host/g/r/mergerequests/1")
	want := `【CodeFever Next】正文 <br/> <a href="http://host/g/r/mergerequests/1">http://host/g/r/mergerequests/1</a>`
	if got != want {
		t.Errorf("邮件正文 = %q，期望 %q", got, want)
	}
}

// TestPHPJSONEncode 校验 JSON 落库内容与 PHP json_encode() 逐字节一致。
//
// 库里的 PHP 历史数据是 {"name":"Root Team\/demo repo"} —— 正斜杠被转义；
// 非 ASCII 也被转义成 \uXXXX；站内信的 nc_data 还带 JSON_HEX_TAG。
func TestPHPJSONEncode(t *testing.T) {
	cases := []struct {
		name   string
		object *jsonObject
		hexTag bool
		want   string
	}{
		{
			name:   "正斜杠被转义",
			object: newActivityData().set("name", "Root Team/demo repo"),
			want:   `{"name":"Root Team\/demo repo"}`,
		},
		{
			name:   "非 ASCII 转成 \\uXXXX（小写十六进制）",
			object: newActivityData().set("name", "中文"),
			want:   `{"name":"\u4e2d\u6587"}`,
		},
		{
			name:   "空对象编码为数组",
			object: newActivityData(),
			want:   `[]`,
		},
		{
			name:   "站内信带 JSON_HEX_TAG（只转义 < >，不转义 &）",
			object: newActivityData().set("title", "<b>x</b> & y"),
			hexTag: true,
			want:   `{"title":"\u003Cb\u003Ex\u003C\/b\u003E & y"}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var (
				got string
				err error
			)
			if c.hexTag {
				got, err = c.object.marshalHexTag()
			} else {
				got, err = c.object.marshal()
			}
			if err != nil {
				t.Fatalf("编码失败: %v", err)
			}
			if got != c.want {
				t.Errorf("编码结果 = %s，期望 %s", got, c.want)
			}
		})
	}
}

// TestComposeMessageData 校验消息组装（标题 / 正文 / 站内信内容）。
//
// 逐字对应 UserNotificationHandler.php:268-326 的 composeMessageData()。
func TestComposeMessageData(t *testing.T) {
	ev := domain.EventPayload{
		GeneratedBy: "user00000000000000000000000000ab",
		Language:    domain.LanguageCN,
		Data: map[string]any{
			"gKey":         "group00000000000000000000000001",
			"rKey":         "repo000000000000000000000000001",
			"sourceRKey":   "repo000000000000000000000000001",
			"sourceGKey":   "group00000000000000000000000001",
			"sourceBranch": "feature/x",
			"targetBranch": "master",
			"id":           99,
			"mrKey":        "mr00000000000000000000000000001",
		},
	}

	values := map[string]string{
		"users.u_name":                  "张三",
		"groups.g_name":                 "mygroup",
		"repositories.r_name":           "myrepo",
		"merge_requests.mr_title":       "标题",
		"merge_requests.mr_description": "描述",
	}
	resolver := &templateResolver{
		ev: ev,
		lookupFn: func(_ context.Context, table, column, _ string) (string, error) {
			return values[table+"."+column], nil
		},
	}

	tpl := notificationTemplates[domain.LanguageCN][string(domain.EventTypeMERGE_REQUEST_CREATE)]
	fields := notificationFieldData[string(domain.EventTypeMERGE_REQUEST_CREATE)]
	cue := notificationCueOptions[string(domain.EventTypeMERGE_REQUEST_CREATE)]

	data, err := composeMessageData(context.Background(), resolver, tpl, fields, cue)
	if err != nil {
		t.Fatalf("组装消息失败: %v", err)
	}

	wantTitle := "【CodeFever Next】张三 打开了合并请求"
	if data.mailTitle != wantTitle {
		t.Errorf("标题 = %q，期望 %q", data.mailTitle, wantTitle)
	}
	wantBody := "张三 打开了和并请求: mygroup/myrepo/feature/x -> mygroup/myrepo/master"
	if data.mailBody != wantBody {
		t.Errorf("正文 = %q，期望 %q", data.mailBody, wantBody)
	}

	// 站内信内容：字段名、顺序与取值都要对。
	wantInternal := map[string]string{
		"user":             "张三",
		"group":            "mygroup",
		"repository":       "myrepo",
		"sourceRepository": "myrepo",
		"sourceBranch":     "feature/x",
		"targetBranch":     "master",
		"number":           "99",
		"title":            "标题",
		"description":      "描述",
		"created":          "",
	}
	if data.internal.len() != len(wantInternal) {
		t.Fatalf("站内信字段数 = %d（%v），期望 %d", data.internal.len(), data.internal.keys, len(wantInternal))
	}
	for key, want := range wantInternal {
		if got := data.value(key); got != want {
			t.Errorf("站内信字段 %q = %q，期望 %q", key, got, want)
		}
	}

	// 站内信字段顺序必须与 PHP 的 $messageFieldData 一致。
	wantOrder := []string{"user", "group", "repository", "sourceRepository", "sourceBranch",
		"targetBranch", "number", "title", "description", "created"}
	for i, key := range wantOrder {
		if data.internal.keys[i] != key {
			t.Errorf("字段[%d] = %q，期望 %q", i, data.internal.keys[i], key)
		}
	}

	// 落库内容必须是 JSON 对象（不是空数组）。
	raw, err := data.internal.marshalHexTag()
	if err != nil {
		t.Fatalf("序列化站内信内容失败: %v", err)
	}
	if raw == "[]" {
		t.Errorf("站内信内容不应为空数组")
	}
}

// ---------------------------------------------------------------------------
// Webhook
// ---------------------------------------------------------------------------

// TestWebhookSubscribes 校验 rw_events 的逗号分隔匹配。
//
// 逐字对应 application/models/repository_model.php:2233-2237。
func TestWebhookSubscribes(t *testing.T) {
	events := "repo:create,mergeRequest:merge,tag:create"
	for _, evType := range []string{"repo:create", "mergeRequest:merge", "tag:create"} {
		if !webhookSubscribes(events, evType) {
			t.Errorf("事件 %q 应当被订阅", evType)
		}
	}
	for _, evType := range []string{"repo:fork", "mergeRequest:close", ""} {
		if webhookSubscribes(events, evType) {
			t.Errorf("事件 %q 不应被订阅", evType)
		}
	}
}

// TestHandlerEventTypes 校验三个 handler 订阅的事件集合。
func TestHandlerEventTypes(t *testing.T) {
	if got := len(activityEventTypes()); got != 39 {
		t.Errorf("活动流订阅事件数 = %d，期望 39", got)
	}
	if got := len(notificationEventTypes()); got != 5 {
		t.Errorf("通知订阅事件数 = %d，期望 5", got)
	}
	if got := len(webhookEventTypes()); got != 26 {
		t.Errorf("webhook 订阅事件数 = %d，期望 26", got)
	}
}

// keysOf 取出用户列表的主键（便于断言输出）。
func keysOf(users []notifyUser) []string {
	out := make([]string, 0, len(users))
	for _, u := range users {
		out = append(out, u.key)
	}
	return out
}
