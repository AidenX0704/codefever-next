package events

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
)

// 活动类型 —— 逐字对应 application/libraries/service/Constant/ActivityType.php。
//
// 区间编码（前端 www/view/src/config/Activity.js 按 type 查表渲染）：
//
//	0x02xx 仓库组、0x03xx 仓库、0x04xx 分支、0x05xx 标签、
//	0x07xx 合并请求、0x09xx webhook
//
// 分类区间（见 application/models/repository_model.php:789-801 与
// application/models/group_model.php:228-240）：
//
//	"提交"     → 0x0401 / 0x0402 / 0x0501 / 0x0502
//	"合并请求" → 0x0700 < a_type < 0x0800
//	"成员"     → a_type < 0x0600 且排除上面四种
const (
	// ActivityTypeGroupCreate 仓库组创建。
	ActivityTypeGroupCreate = 0x0201
	// ActivityTypeGroupUpdateAvatar 仓库组头像更新。
	ActivityTypeGroupUpdateAvatar = 0x0202
	// ActivityTypeGroupUpdateName 仓库组名称更新。
	ActivityTypeGroupUpdateName = 0x0203
	// ActivityTypeGroupUpdateDescription 仓库组描述更新。
	ActivityTypeGroupUpdateDescription = 0x0204
	// ActivityTypeGroupAddMember 仓库组新增成员。
	ActivityTypeGroupAddMember = 0x0205
	// ActivityTypeGroupChangeMemberRole 仓库组成员角色变更。
	ActivityTypeGroupChangeMemberRole = 0x0206
	// ActivityTypeGroupRemoveMember 仓库组移除成员。
	ActivityTypeGroupRemoveMember = 0x0207
	// ActivityTypeGroupChangeOwner 仓库组更换拥有者。
	ActivityTypeGroupChangeOwner = 0x0208
	// ActivityTypeGroupChangeURL 仓库组路径变更。
	ActivityTypeGroupChangeURL = 0x0209
	// ActivityTypeGroupRemove 仓库组删除。
	ActivityTypeGroupRemove = 0x020A

	// ActivityTypeRepoCreate 仓库创建。
	ActivityTypeRepoCreate = 0x0301
	// ActivityTypeRepoFork 仓库 fork。
	ActivityTypeRepoFork = 0x0302
	// ActivityTypeRepoUpdateAvatar 仓库头像更新。
	ActivityTypeRepoUpdateAvatar = 0x0303
	// ActivityTypeRepoUpdateName 仓库名称更新。
	ActivityTypeRepoUpdateName = 0x0304
	// ActivityTypeRepoUpdateDescription 仓库描述更新。
	ActivityTypeRepoUpdateDescription = 0x0305
	// ActivityTypeRepoAddMember 仓库新增成员。
	ActivityTypeRepoAddMember = 0x0306
	// ActivityTypeRepoChangeMemberRole 仓库成员角色变更。
	ActivityTypeRepoChangeMemberRole = 0x0307
	// ActivityTypeRepoRemoveMember 仓库移除成员。
	ActivityTypeRepoRemoveMember = 0x0308
	// ActivityTypeRepoChangeOwner 仓库更换拥有者。
	ActivityTypeRepoChangeOwner = 0x0309
	// ActivityTypeRepoChangeURL 仓库路径变更。
	ActivityTypeRepoChangeURL = 0x030A
	// ActivityTypeRepoRemove 仓库删除。
	ActivityTypeRepoRemove = 0x030B

	// ActivityTypeBranchPush 分支 push。
	ActivityTypeBranchPush = 0x0401
	// ActivityTypeBranchPushNew 新分支首次 push。
	ActivityTypeBranchPushNew = 0x0402
	// ActivityTypeBranchCreate 分支创建。
	ActivityTypeBranchCreate = 0x0403
	// ActivityTypeBranchRemove 分支删除。
	ActivityTypeBranchRemove = 0x0404
	// ActivityTypeDefaultBranchChange 默认分支变更。
	ActivityTypeDefaultBranchChange = 0x0405
	// ActivityTypeProtectedBranchRuleCreate 保护分支规则创建。
	ActivityTypeProtectedBranchRuleCreate = 0x0406
	// ActivityTypeProtectedBranchRuleChange 保护分支规则变更。
	ActivityTypeProtectedBranchRuleChange = 0x0407
	// ActivityTypeProtectedBranchRuleRemove 保护分支规则删除。
	ActivityTypeProtectedBranchRuleRemove = 0x0408

	// ActivityTypeTagPush 标签 push。
	ActivityTypeTagPush = 0x0501
	// ActivityTypeTagPushNew 新标签首次 push。
	ActivityTypeTagPushNew = 0x0502
	// ActivityTypeTagCreate 标签创建。
	ActivityTypeTagCreate = 0x0503
	// ActivityTypeTagRemove 标签删除。
	ActivityTypeTagRemove = 0x0504

	// ActivityTypeMergeRequestCreate 合并请求创建。
	ActivityTypeMergeRequestCreate = 0x0701
	// ActivityTypeMergeRequestClose 合并请求关闭。
	ActivityTypeMergeRequestClose = 0x0702
	// ActivityTypeMergeRequestMerge 合并请求合并。
	ActivityTypeMergeRequestMerge = 0x0703
	// ActivityTypeMergeRequestReviewerCreate 合并请求指派评审员。
	ActivityTypeMergeRequestReviewerCreate = 0x0704
	// ActivityTypeMergeRequestReviewerDelete 合并请求移除评审员。
	ActivityTypeMergeRequestReviewerDelete = 0x0705
	// ActivityTypeMergeRequestReviewerReview 合并请求评审通过。
	ActivityTypeMergeRequestReviewerReview = 0x0706

	// ActivityTypeWebhookCreate webhook 创建。
	ActivityTypeWebhookCreate = 0x0901
	// ActivityTypeWebhookUpdate webhook 更新。
	ActivityTypeWebhookUpdate = 0x0902
	// ActivityTypeWebhookDelete webhook 删除。
	ActivityTypeWebhookDelete = 0x0903
)

// pushRefPattern 是 push 事件的 ref 解析规则。
//
// 逐字对应 application/event_handlers/ActivityHandler.php:144 的
// preg_match('/refs\/(heads|tags)\/(.*)/', ...)。
var pushRefPattern = regexp.MustCompile(`refs/(heads|tags)/(.*)`)

// zeroCommitHash 是 git 的空提交哈希（新建分支 / 新建标签的首个 push）。
//
// 逐字对应 ActivityHandler.php:157 与 162 的比较串。
const zeroCommitHash = "0000000000000000000000000000000000000000"

// activityHandler 把领域事件翻译成活动流记录（cc_activities）。
//
// 逐字对应 application/event_handlers/ActivityHandler.php：
// 它监听 39 个事件，按事件族分派，把结构化的事件数据转成
// **前端可直接渲染** 的 a_data JSON。
type activityHandler struct {
	db  *sql.DB
	log *slog.Logger
}

// NewActivityHandler 创建活动流处理器。
func NewActivityHandler(db *sql.DB, logger *slog.Logger) Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &activityHandler{db: db, log: logger}
}

// activityEventTypes 是 ActivityHandler 关心的全部事件。
//
// 逐字对应 application/event_handlers/ActivityHandler.php:13-59 的
// $ListenedEventList（共 39 个）。
func activityEventTypes() []domain.EventType {
	return []domain.EventType{
		// 仓库组事件（10 个）
		domain.EventTypeGROUP_CREATE,
		domain.EventTypeGROUP_UPDATE_AVATAR,
		domain.EventTypeGROUP_UPDATE_NAME,
		domain.EventTypeGROUP_UPDATE_DESCRIPTION,
		domain.EventTypeGROUP_ADD_MEMBER,
		domain.EventTypeGROUP_CHANGE_MEMBER_ROLE,
		domain.EventTypeGROUP_REMOVE_MEMBER,
		domain.EventTypeGROUP_CHANGE_OWNER,
		domain.EventTypeGROUP_CHANGE_URL,
		domain.EventTypeGROUP_REMOVE,

		// 仓库事件（11 个）
		domain.EventTypeREPO_CREATE,
		domain.EventTypeREPO_FORK,
		domain.EventTypeREPO_UPDATE_AVATAR,
		domain.EventTypeREPO_UPDATE_NAME,
		domain.EventTypeREPO_UPDATE_DESCRIPTION,
		domain.EventTypeREPO_ADD_MEMBER,
		domain.EventTypeREPO_CHANGE_MEMBER_ROLE,
		domain.EventTypeREPO_REMOVE_MEMBER,
		domain.EventTypeREPO_CHANGE_OWNER,
		domain.EventTypeREPO_CHANGE_URL,
		domain.EventTypeREPO_REMOVE,

		// 分支事件（6 个）
		domain.EventTypeBRANCH_CREATE,
		domain.EventTypeBRANCH_REMOVE,
		domain.EventTypeDEFAULT_BRANCH_CHANGE,
		domain.EventTypePROTECTED_BRANCH_RULE_CREATE,
		domain.EventTypePROTECTED_BRANCH_RULE_CHANGE,
		domain.EventTypePROTECTED_BRANCH_RULE_REMOVE,

		// 标签事件（2 个）
		domain.EventTypeTAG_CREATE,
		domain.EventTypeTAG_REMOVE,

		// 合并请求事件（6 个）
		domain.EventTypeMERGE_REQUEST_CREATE,
		domain.EventTypeMERGE_REQUEST_CLOSE,
		domain.EventTypeMERGE_REQUEST_MERGE,
		domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE,
		domain.EventTypeMERGE_REQUEST_REVIEWER_DELETE,
		domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW,

		// webhook 事件（3 个）
		domain.EventTypeWEBHOOK_CREATE,
		domain.EventTypeWEBHOOK_UPDATE,
		domain.EventTypeWEBHOOK_DELETE,

		// 仓库 push 事件（1 个）
		domain.EventTypeHOOK_POST_RECEIVE,
	}
}

// Types 实现 Handler。
func (h *activityHandler) Types() []domain.EventType { return activityEventTypes() }

// activityRow 是一条待写入 cc_activities 的活动记录。
//
// 字段与 cc_activities 的列一一对应：
//
//	aType  → a_type
//	uKey   → u_key
//	gKey   → a_relative_g_key（nil 表示 NULL）
//	rKey   → a_relative_r_key（nil 表示 NULL）
//	mrKey  → a_relative_mr_key（nil 表示 NULL）
//	data   → a_data（JSON 文本）
type activityRow struct {
	aType int
	uKey  string
	gKey  *string
	rKey  *string
	mrKey *string
	data  string
}

// Capture 实现 Handler。
func (h *activityHandler) Capture(ctx context.Context, ev domain.EventPayload) error {
	row, ok, err := h.buildActivity(ctx, ev)
	if err != nil {
		return err
	}
	if !ok {
		// PHP 侧对不匹配的事件 / 无法解析的 ref 直接不落库。
		return nil
	}
	return h.insertActivity(ctx, row)
}

// buildActivity 按事件族把事件翻译成活动记录。
//
// 逐字对应 ActivityHandler.php:73-139 的 _processEvnet() 分派结构。
func (h *activityHandler) buildActivity(ctx context.Context, ev domain.EventPayload) (activityRow, bool, error) {
	switch ev.Type {
	case string(domain.EventTypeHOOK_POST_RECEIVE):
		return h.buildPushActivity(ctx, ev)

	case string(domain.EventTypeGROUP_CREATE),
		string(domain.EventTypeGROUP_UPDATE_AVATAR),
		string(domain.EventTypeGROUP_UPDATE_NAME),
		string(domain.EventTypeGROUP_UPDATE_DESCRIPTION),
		string(domain.EventTypeGROUP_ADD_MEMBER),
		string(domain.EventTypeGROUP_CHANGE_MEMBER_ROLE),
		string(domain.EventTypeGROUP_REMOVE_MEMBER),
		string(domain.EventTypeGROUP_CHANGE_OWNER),
		string(domain.EventTypeGROUP_CHANGE_URL),
		string(domain.EventTypeGROUP_REMOVE):
		return buildGroupActivity(ev), true, nil

	case string(domain.EventTypeREPO_CREATE),
		string(domain.EventTypeREPO_FORK),
		string(domain.EventTypeREPO_UPDATE_AVATAR),
		string(domain.EventTypeREPO_UPDATE_NAME),
		string(domain.EventTypeREPO_UPDATE_DESCRIPTION),
		string(domain.EventTypeREPO_ADD_MEMBER),
		string(domain.EventTypeREPO_CHANGE_MEMBER_ROLE),
		string(domain.EventTypeREPO_REMOVE_MEMBER),
		string(domain.EventTypeREPO_CHANGE_OWNER),
		string(domain.EventTypeREPO_CHANGE_URL),
		string(domain.EventTypeREPO_REMOVE):
		row := buildRepoActivity(ev)
		if err := h.touchRepository(ctx, eventDataString(ev.Data, "rKey")); err != nil {
			return activityRow{}, false, err
		}
		return row, true, nil

	case string(domain.EventTypeBRANCH_CREATE),
		string(domain.EventTypeBRANCH_REMOVE),
		string(domain.EventTypeDEFAULT_BRANCH_CHANGE),
		string(domain.EventTypePROTECTED_BRANCH_RULE_CREATE),
		string(domain.EventTypePROTECTED_BRANCH_RULE_CHANGE),
		string(domain.EventTypePROTECTED_BRANCH_RULE_REMOVE):
		return buildBranchActivity(ev), true, nil

	case string(domain.EventTypeTAG_CREATE),
		string(domain.EventTypeTAG_REMOVE):
		return buildTagActivity(ev), true, nil

	case string(domain.EventTypeMERGE_REQUEST_CREATE),
		string(domain.EventTypeMERGE_REQUEST_CLOSE),
		string(domain.EventTypeMERGE_REQUEST_MERGE),
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE),
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_DELETE),
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW):
		return buildMergeRequestActivity(ev), true, nil

	case string(domain.EventTypeWEBHOOK_CREATE),
		string(domain.EventTypeWEBHOOK_UPDATE),
		string(domain.EventTypeWEBHOOK_DELETE):
		return buildWebhookActivity(ev), true, nil
	}
	return activityRow{}, false, nil
}

// buildGroupActivity 处理仓库组事件。
//
// 逐字对应 ActivityHandler.php:180-252 的 _handleGroupEvent()。
func buildGroupActivity(ev domain.EventPayload) activityRow {
	data := ev.Data
	row := activityRow{
		uKey: ev.GeneratedBy,
		// PHP 侧组事件的 a_relative_r_key 固定写入空字符串（不是 NULL）。
		rKey: strPtr(""),
		gKey: strPtr(eventDataString(data, "gKey")),
	}

	switch ev.Type {
	case string(domain.EventTypeGROUP_CREATE), string(domain.EventTypeGROUP_REMOVE):
		row.aType = groupActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().set("name", eventDataString(data, "name")))

	case string(domain.EventTypeGROUP_UPDATE_AVATAR),
		string(domain.EventTypeGROUP_UPDATE_NAME),
		string(domain.EventTypeGROUP_CHANGE_URL):
		row.aType = groupActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("from", eventDataString(data, "from")).
			set("to", eventDataString(data, "to")))

	case string(domain.EventTypeGROUP_UPDATE_DESCRIPTION):
		row.aType = groupActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData())

	case string(domain.EventTypeGROUP_ADD_MEMBER),
		string(domain.EventTypeGROUP_REMOVE_MEMBER),
		string(domain.EventTypeGROUP_CHANGE_OWNER):
		row.aType = groupActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("uid", eventDataString(data, "uid")).
			set("name", eventDataString(data, "name")).
			set("email", eventDataString(data, "email")))

	case string(domain.EventTypeGROUP_CHANGE_MEMBER_ROLE):
		row.aType = groupActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("uid", eventDataString(data, "uid")).
			set("name", eventDataString(data, "name")).
			set("email", eventDataString(data, "email")).
			set("to", eventDataInt(data, "to")))
	}
	return row
}

// buildRepoActivity 处理仓库事件。
//
// 逐字对应 ActivityHandler.php:254-327 的 _handleRepoEvent()。
func buildRepoActivity(ev domain.EventPayload) activityRow {
	data := ev.Data
	row := activityRow{
		uKey: ev.GeneratedBy,
		rKey: strPtr(eventDataString(data, "rKey")),
		gKey: strPtr(eventDataString(data, "gKey")),
	}

	switch ev.Type {
	case string(domain.EventTypeREPO_CREATE), string(domain.EventTypeREPO_REMOVE):
		row.aType = repoActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().set("name", eventDataString(data, "name")))

	case string(domain.EventTypeREPO_FORK):
		row.aType = repoActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("name", eventDataString(data, "name")).
			set("sourceName", eventDataString(data, "sourceName")))

	case string(domain.EventTypeREPO_UPDATE_AVATAR),
		string(domain.EventTypeREPO_UPDATE_NAME),
		string(domain.EventTypeREPO_CHANGE_URL):
		row.aType = repoActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("from", eventDataString(data, "from")).
			set("to", eventDataString(data, "to")))

	case string(domain.EventTypeREPO_UPDATE_DESCRIPTION):
		row.aType = repoActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData())

	case string(domain.EventTypeREPO_ADD_MEMBER),
		string(domain.EventTypeREPO_REMOVE_MEMBER),
		string(domain.EventTypeREPO_CHANGE_OWNER):
		row.aType = repoActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("uid", eventDataString(data, "uid")).
			set("name", eventDataString(data, "name")).
			set("email", eventDataString(data, "email")))

	case string(domain.EventTypeREPO_CHANGE_MEMBER_ROLE):
		row.aType = repoActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("uid", eventDataString(data, "uid")).
			set("name", eventDataString(data, "name")).
			set("email", eventDataString(data, "email")).
			set("to", eventDataInt(data, "to")))
	}
	return row
}

// buildBranchActivity 处理分支事件。
//
// 逐字对应 ActivityHandler.php:329-369 的 _handleBranchEvent()。
func buildBranchActivity(ev domain.EventPayload) activityRow {
	data := ev.Data
	row := activityRow{
		uKey: ev.GeneratedBy,
		rKey: strPtr(eventDataString(data, "rKey")),
		gKey: strPtr(eventDataString(data, "gKey")),
	}

	switch ev.Type {
	case string(domain.EventTypeBRANCH_CREATE),
		string(domain.EventTypeBRANCH_REMOVE),
		string(domain.EventTypePROTECTED_BRANCH_RULE_CREATE),
		string(domain.EventTypePROTECTED_BRANCH_RULE_CHANGE),
		string(domain.EventTypePROTECTED_BRANCH_RULE_REMOVE):
		row.aType = branchActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().set("name", eventDataString(data, "name")))

	case string(domain.EventTypeDEFAULT_BRANCH_CHANGE):
		row.aType = branchActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("from", eventDataString(data, "from")).
			set("to", eventDataString(data, "to")))
	}
	return row
}

// buildTagActivity 处理标签事件。
//
// 逐字对应 ActivityHandler.php:371-396 的 _handleTagEvent()。
func buildTagActivity(ev domain.EventPayload) activityRow {
	data := ev.Data
	row := activityRow{
		uKey:  ev.GeneratedBy,
		rKey:  strPtr(eventDataString(data, "rKey")),
		gKey:  strPtr(eventDataString(data, "gKey")),
		aType: ActivityTypeTagCreate,
	}
	if ev.Type == string(domain.EventTypeTAG_REMOVE) {
		row.aType = ActivityTypeTagRemove
	}
	row.data = activityDataJSON(newActivityData().set("name", eventDataString(data, "name")))
	return row
}

// buildMergeRequestActivity 处理合并请求事件。
//
// 逐字对应 ActivityHandler.php:398-449 的 _handleMergeRequestEvent()。
func buildMergeRequestActivity(ev domain.EventPayload) activityRow {
	data := ev.Data
	row := activityRow{
		uKey:  ev.GeneratedBy,
		rKey:  strPtr(eventDataString(data, "rKey")),
		gKey:  strPtr(eventDataString(data, "gKey")),
		mrKey: strPtr(eventDataString(data, "mrKey")),
	}

	switch ev.Type {
	case string(domain.EventTypeMERGE_REQUEST_CREATE),
		string(domain.EventTypeMERGE_REQUEST_CLOSE),
		string(domain.EventTypeMERGE_REQUEST_MERGE):
		row.aType = mergeRequestActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("id", eventDataInt(data, "id")).
			set("sourceBranch", eventDataString(data, "sourceBranch")).
			set("targetBranch", eventDataString(data, "targetBranch")))

	case string(domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE):
		row.aType = ActivityTypeMergeRequestReviewerCreate
		row.data = activityDataJSON(newActivityData().
			set("id", eventDataInt(data, "id")).
			set("reviewer", eventDataString(data, "reviewer")).
			set("mrrKey", eventDataString(data, "mrrKey")))

	case string(domain.EventTypeMERGE_REQUEST_REVIEWER_DELETE),
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW):
		row.aType = mergeRequestActivityType(ev.Type)
		row.data = activityDataJSON(newActivityData().
			set("id", eventDataInt(data, "id")).
			set("reviewer", eventDataString(data, "reviewer")))
	}
	return row
}

// buildWebhookActivity 处理 webhook 事件。
//
// 逐字对应 ActivityHandler.php:451-478 的 _handleWebhookEvent()。
func buildWebhookActivity(ev domain.EventPayload) activityRow {
	data := ev.Data
	row := activityRow{
		uKey:  ev.GeneratedBy,
		rKey:  strPtr(eventDataString(data, "rKey")),
		gKey:  strPtr(eventDataString(data, "gKey")),
		aType: ActivityTypeWebhookCreate,
	}
	switch ev.Type {
	case string(domain.EventTypeWEBHOOK_UPDATE):
		row.aType = ActivityTypeWebhookUpdate
	case string(domain.EventTypeWEBHOOK_DELETE):
		row.aType = ActivityTypeWebhookDelete
	}
	row.data = activityDataJSON(newActivityData().set("rwKey", eventDataString(data, "rwKey")))
	return row
}

// buildPushActivity 处理仓库 push 事件。
//
// 逐字对应 ActivityHandler.php:141-178 的 _handleRepositoryPushEvent()：
//   - ref 形如 refs/heads/master 或 refs/tags/v1.0；
//   - from 为全 0 哈希时是"新建分支 / 新建标签"；
//   - 仓库组主键来自 cc_repositories（事件数据里没有 gKey）。
func (h *activityHandler) buildPushActivity(ctx context.Context, ev domain.EventPayload) (activityRow, bool, error) {
	data := ev.Data
	name := eventDataString(data, "name")
	if name == "" {
		return activityRow{}, false, nil
	}
	match := pushRefPattern.FindStringSubmatch(name)
	if match == nil {
		return activityRow{}, false, nil
	}

	kind, ref := match[1], match[2]
	from := eventDataString(data, "from")

	row := activityRow{
		uKey: ev.GeneratedBy,
		rKey: strPtr(eventDataString(data, "rKey")),
	}

	switch kind {
	case "heads":
		row.aType = ActivityTypeBranchPush
		if from == zeroCommitHash {
			row.aType = ActivityTypeBranchPushNew
		}
	case "tags":
		row.aType = ActivityTypeTagPush
		if from == zeroCommitHash {
			row.aType = ActivityTypeTagPushNew
		}
	default:
		return activityRow{}, false, nil
	}

	row.data = activityDataJSON(newActivityData().
		set("name", ref).
		set("from", from).
		set("to", eventDataString(data, "to")))

	// 仓库组主键需要回查 cc_repositories（事件数据里没有 gKey）。
	gKey, err := h.repositoryGroupKey(ctx, eventDataString(data, "rKey"))
	if err != nil {
		return activityRow{}, false, err
	}
	row.gKey = strPtr(gKey)

	if err := h.touchRepository(ctx, eventDataString(data, "rKey")); err != nil {
		return activityRow{}, false, err
	}
	return row, true, nil
}

// insertActivity 把活动记录写入 cc_activities。
//
// 逐字对应 PHP 的 `$this->CI->db->insert('activities', $insertData)`：
// 只写显式给出的列，其余列走数据库默认值。
func (h *activityHandler) insertActivity(ctx context.Context, row activityRow) error {
	if handlerContextIsDone(ctx) {
		return ctx.Err()
	}
	_, err := h.db.ExecContext(ctx, `
		INSERT INTO cc_activities
			(a_key, a_type, u_key, a_relative_g_key, a_relative_r_key, a_relative_mr_key, a_data)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		contract.NewKey(), row.aType, row.uKey, row.gKey, row.rKey, row.mrKey, row.data)
	if err != nil {
		return fmt.Errorf("写入活动流失败: %w", err)
	}
	return nil
}

// touchRepository 刷新仓库的更新时间。
//
// 逐字对应 ActivityHandler.php:174 与 324 的
// `$this->CI->repositoryModel->update($rKey, ['r_updated' => date('Y-m-d H:i:s')])`。
func (h *activityHandler) touchRepository(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	if _, err := h.db.ExecContext(ctx,
		"UPDATE cc_repositories SET r_updated = NOW() WHERE r_key = ?", key); err != nil {
		return fmt.Errorf("更新仓库时间戳失败: %w", err)
	}
	return nil
}

// repositoryGroupKey 查仓库所属的仓库组主键。
func (h *activityHandler) repositoryGroupKey(ctx context.Context, rKey string) (string, error) {
	if rKey == "" {
		return "", nil
	}
	var gKey sql.NullString
	err := h.db.QueryRowContext(ctx,
		"SELECT g_key FROM cc_repositories WHERE r_key = ? LIMIT 1", rKey).Scan(&gKey)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("查询仓库所属组失败: %w", err)
	}
	return gKey.String, nil
}

// newActivityData 构造一个空的活动内容对象。
func newActivityData() *jsonObject { return &jsonObject{} }

// activityDataJSON 编码活动内容（编码失败时退化为空数组，保证落库不失败）。
func activityDataJSON(o *jsonObject) string {
	raw, err := o.marshal()
	if err != nil {
		return "[]"
	}
	return raw
}

// eventDataInt 读取事件数据里的整数字段。
//
// PHP 侧的 a_data 里 id / to 都是**数字**（json_encode 保留 int），
// 因此这里保持数字形态，前端才能做数值比较。
func eventDataInt(data map[string]any, key string) any {
	raw, ok := data[key]
	if !ok || raw == nil {
		return 0
	}
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return v
	case float64:
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	case string:
		return v
	default:
		return 0
	}
}

// groupActivityType 返回仓库组事件对应的活动类型。
func groupActivityType(evType string) int {
	switch evType {
	case string(domain.EventTypeGROUP_CREATE):
		return ActivityTypeGroupCreate
	case string(domain.EventTypeGROUP_UPDATE_AVATAR):
		return ActivityTypeGroupUpdateAvatar
	case string(domain.EventTypeGROUP_UPDATE_NAME):
		return ActivityTypeGroupUpdateName
	case string(domain.EventTypeGROUP_UPDATE_DESCRIPTION):
		return ActivityTypeGroupUpdateDescription
	case string(domain.EventTypeGROUP_ADD_MEMBER):
		return ActivityTypeGroupAddMember
	case string(domain.EventTypeGROUP_CHANGE_MEMBER_ROLE):
		return ActivityTypeGroupChangeMemberRole
	case string(domain.EventTypeGROUP_REMOVE_MEMBER):
		return ActivityTypeGroupRemoveMember
	case string(domain.EventTypeGROUP_CHANGE_OWNER):
		return ActivityTypeGroupChangeOwner
	case string(domain.EventTypeGROUP_CHANGE_URL):
		return ActivityTypeGroupChangeURL
	case string(domain.EventTypeGROUP_REMOVE):
		return ActivityTypeGroupRemove
	}
	return 0
}

// repoActivityType 返回仓库事件对应的活动类型。
func repoActivityType(evType string) int {
	switch evType {
	case string(domain.EventTypeREPO_CREATE):
		return ActivityTypeRepoCreate
	case string(domain.EventTypeREPO_FORK):
		return ActivityTypeRepoFork
	case string(domain.EventTypeREPO_UPDATE_AVATAR):
		return ActivityTypeRepoUpdateAvatar
	case string(domain.EventTypeREPO_UPDATE_NAME):
		return ActivityTypeRepoUpdateName
	case string(domain.EventTypeREPO_UPDATE_DESCRIPTION):
		return ActivityTypeRepoUpdateDescription
	case string(domain.EventTypeREPO_ADD_MEMBER):
		return ActivityTypeRepoAddMember
	case string(domain.EventTypeREPO_CHANGE_MEMBER_ROLE):
		return ActivityTypeRepoChangeMemberRole
	case string(domain.EventTypeREPO_REMOVE_MEMBER):
		return ActivityTypeRepoRemoveMember
	case string(domain.EventTypeREPO_CHANGE_OWNER):
		return ActivityTypeRepoChangeOwner
	case string(domain.EventTypeREPO_CHANGE_URL):
		return ActivityTypeRepoChangeURL
	case string(domain.EventTypeREPO_REMOVE):
		return ActivityTypeRepoRemove
	}
	return 0
}

// branchActivityType 返回分支事件对应的活动类型。
func branchActivityType(evType string) int {
	switch evType {
	case string(domain.EventTypeBRANCH_CREATE):
		return ActivityTypeBranchCreate
	case string(domain.EventTypeBRANCH_REMOVE):
		return ActivityTypeBranchRemove
	case string(domain.EventTypeDEFAULT_BRANCH_CHANGE):
		return ActivityTypeDefaultBranchChange
	case string(domain.EventTypePROTECTED_BRANCH_RULE_CREATE):
		return ActivityTypeProtectedBranchRuleCreate
	case string(domain.EventTypePROTECTED_BRANCH_RULE_CHANGE):
		return ActivityTypeProtectedBranchRuleChange
	case string(domain.EventTypePROTECTED_BRANCH_RULE_REMOVE):
		return ActivityTypeProtectedBranchRuleRemove
	}
	return 0
}

// mergeRequestActivityType 返回合并请求事件对应的活动类型。
func mergeRequestActivityType(evType string) int {
	switch evType {
	case string(domain.EventTypeMERGE_REQUEST_CREATE):
		return ActivityTypeMergeRequestCreate
	case string(domain.EventTypeMERGE_REQUEST_CLOSE):
		return ActivityTypeMergeRequestClose
	case string(domain.EventTypeMERGE_REQUEST_MERGE):
		return ActivityTypeMergeRequestMerge
	case string(domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE):
		return ActivityTypeMergeRequestReviewerCreate
	case string(domain.EventTypeMERGE_REQUEST_REVIEWER_DELETE):
		return ActivityTypeMergeRequestReviewerDelete
	case string(domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW):
		return ActivityTypeMergeRequestReviewerReview
	}
	return 0
}
