package events

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/mailer"
	"codefever-next/backend/internal/repo"
)

// messageDelimiter 是 PHP 侧 Helper::getDelimiter() 的返回值。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:25-29：
// json_decode('{"encode":"\ue000\ue001\ue000"}', TRUE)['encode']。
// 它用来拼接 / 拆分模板的各个片段，站内信内容也用它分隔键与值。
const messageDelimiter = "\ue000\ue001\ue000"

// notificationTemplate 是一个事件对应的一套文案。
type notificationTemplate struct {
	mailTitle string
	mailBody  string
}

// notificationField 是站内信内容里的一个字段。
//
// value 是模板表达式（如 "{{$user@users:u_name}}"），
// 顺序与 PHP 侧 $messageFieldData 的数组顺序一致（站内信内容按此顺序落库）。
type notificationField struct {
	key   string
	value string
}

// notificationTemplates 是全部事件的中英文案。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:22-67 的
// $messageTemplate —— 中文（LANG_CN）与英文（LANG_EN）各一套。
var notificationTemplates = map[string]map[string]notificationTemplate{
	domain.LanguageCN: {
		string(domain.EventTypeMERGE_REQUEST_CREATE): {
			mailTitle: "【CodeFever Next】{{$user@users:u_name}} 打开了合并请求",
			mailBody:  "{{$user@users:u_name}} 打开了和并请求: {{$data->sourceGKey@groups:g_name}}/{{$data->sourceRKey@repositories:r_name}}/{{$data->sourceBranch}} -> {{$data->gKey@groups:g_name}}/{{$data->rKey@repositories:r_name}}/{{$data->targetBranch}}",
		},
		string(domain.EventTypeMERGE_REQUEST_CLOSE): {
			mailTitle: "【CodeFever Next】{{$user@users:u_name}} 关闭了合并请求",
			mailBody:  "{{$user@users:u_name}} 关闭了和并请求: {{$data->sourceGKey@groups:g_name}}/{{$data->sourceRKey@repositories:r_name}}/{{$data->sourceBranch}} -> {{$data->gKey@groups:g_name}}/{{$data->rKey@repositories:r_name}}/{{$data->targetBranch}}",
		},
		string(domain.EventTypeMERGE_REQUEST_MERGE): {
			mailTitle: "【CodeFever Next】{{$user@users:u_name}} 合并了合并请求",
			mailBody:  "{{$user@users:u_name}} 合并了和并请求: {{$data->sourceGKey@groups:g_name}}/{{$data->sourceRKey@repositories:r_name}}/{{$data->sourceBranch}} -> {{$data->gKey@groups:g_name}}/{{$data->rKey@repositories:r_name}}/{{$data->targetBranch}}",
		},
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE): {
			mailTitle: "【CodeFever Next】合并请求评审",
			mailBody:  "{{$user@users:u_name}} 指定你为和并请求 {{$data->rKey@repositories:r_name}}!{{$data->id}} 的评审员",
		},
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW): {
			mailTitle: "【CodeFever Next】合并请求评审",
			mailBody:  "{{$user@users:u_name}} 通过了合并请求 {{$data->rKey@repositories:r_name}}!{{$data->id}} 的评审",
		},
	},
	domain.LanguageEN: {
		string(domain.EventTypeMERGE_REQUEST_CREATE): {
			mailTitle: "【CodeFever Next】{{$user@users:u_name}} Open Merge Request",
			mailBody:  "{{$user@users:u_name}} Open Merge Request: {{$data->sourceGKey@groups:g_name}}/{{$data->sourceRKey@repositories:r_name}}/{{$data->sourceBranch}} -> {{$data->gKey@groups:g_name}}/{{$data->rKey@repositories:r_name}}/{{$data->targetBranch}}",
		},
		string(domain.EventTypeMERGE_REQUEST_CLOSE): {
			mailTitle: "【CodeFever Next】{{$user@users:u_name}} Close Merge Request",
			mailBody:  "{{$user@users:u_name}} Close Merge Request: {{$data->sourceGKey@groups:g_name}}/{{$data->sourceRKey@repositories:r_name}}/{{$data->sourceBranch}} -> {{$data->gKey@groups:g_name}}/{{$data->rKey@repositories:r_name}}/{{$data->targetBranch}}",
		},
		string(domain.EventTypeMERGE_REQUEST_MERGE): {
			mailTitle: "【CodeFever Next】{{$user@users:u_name}} Merge The Merge Request",
			mailBody:  "{{$user@users:u_name}} Merge The Merge Request: {{$data->sourceGKey@groups:g_name}}/{{$data->sourceRKey@repositories:r_name}}/{{$data->sourceBranch}} -> {{$data->gKey@groups:g_name}}/{{$data->rKey@repositories:r_name}}/{{$data->targetBranch}}",
		},
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE): {
			mailTitle: "【CodeFever Next】Merge Request Review",
			mailBody:  "{{$user@users:u_name}} Designates You As A Reviewer For Merge Request {{$data->rKey@repositories:r_name}}!{{$data->id}}",
		},
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW): {
			mailTitle: "【CodeFever Next】Merge Request Review",
			mailBody:  "{{$user@users:u_name}} Passed Review Of Merge Request {{$data->rKey@repositories:r_name}}!{{$data->id}}",
		},
	},
}

// notificationFieldData 是站内信的内容字段表。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:69-138 的
// $messageFieldData（键名与顺序都必须一致 —— 前端按字段名渲染）。
var notificationFieldData = map[string][]notificationField{
	string(domain.EventTypeMERGE_REQUEST_CREATE): {
		{"user", "{{$user@users:u_name}}"},
		{"group", "{{$data->gKey@groups:g_name}}"},
		{"repository", "{{$data->rKey@repositories:r_name}}"},
		{"sourceRepository", "{{$data->sourceRKey@repositories:r_name}}"},
		{"sourceBranch", "{{$data->sourceBranch}}"},
		{"targetBranch", "{{$data->targetBranch}}"},
		{"number", "{{$data->id}}"},
		{"title", "{{$data->mrKey@merge_requests:mr_title}}"},
		{"description", "{{$data->mrKey@merge_requests:mr_description}}"},
		{"created", "{{$data->mrKey@merge_requests:mr_created}}"},
	},
	string(domain.EventTypeMERGE_REQUEST_CLOSE): {
		{"user", "{{$user@users:u_name}}"},
		{"group", "{{$data->gKey@groups:g_name}}"},
		{"repository", "{{$data->rKey@repositories:r_name}}"},
		{"sourceRepository", "{{$data->sourceRKey@repositories:r_name}}"},
		{"sourceBranch", "{{$data->sourceBranch}}"},
		{"targetBranch", "{{$data->targetBranch}}"},
		{"number", "{{$data->id}}"},
		{"title", "{{$data->mrKey@merge_requests:mr_title}}"},
		{"description", "{{$data->mrKey@merge_requests:mr_description}}"},
		{"closed", "{{$data->mrKey@merge_requests:mr_close_time}}"},
	},
	string(domain.EventTypeMERGE_REQUEST_MERGE): {
		{"user", "{{$user@users:u_name}}"},
		{"group", "{{$data->gKey@groups:g_name}}"},
		{"repository", "{{$data->rKey@repositories:r_name}}"},
		{"sourceRepository", "{{$data->sourceRKey@repositories:r_name}}"},
		{"sourceBranch", "{{$data->sourceBranch}}"},
		{"targetBranch", "{{$data->targetBranch}}"},
		{"number", "{{$data->id}}"},
		{"title", "{{$data->mrKey@merge_requests:mr_title}}"},
		{"description", "{{$data->mrKey@merge_requests:mr_description}}"},
		{"merged", "{{$data->mrKey@merge_requests:mr_merge_time}}"},
	},
	string(domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE): {
		{"user", "{{$user@users:u_name}}"},
		{"reviewer", "{{$data->reviewer@users:u_name}}"},
		{"group", "{{$data->gKey@groups:g_name}}"},
		{"repository", "{{$data->rKey@repositories:r_name}}"},
		{"number", "{{$data->id}}"},
		{"title", "{{$data->mrKey@merge_requests:mr_title}}"},
		{"created", "{{$data->mrrKey@merge_request_reviewers:mrr_created}}"},
	},
	string(domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW): {
		{"user", "{{$user@users:u_name}}"},
		{"reviewer", "{{$data->reviewer@users:u_name}}"},
		{"group", "{{$data->gKey@groups:g_name}}"},
		{"repository", "{{$data->rKey@repositories:r_name}}"},
		{"number", "{{$data->id}}"},
		{"title", "{{$data->mrKey@merge_requests:mr_title}}"},
	},
}

// notificationCueOptions 是每个事件的"提示对象"选项。
//
// 逐字对应 UserNotificationHandler.php:69-138 里各事件的 'cueOption'：
//
//	members → 仓库组成员 + 仓库组成员（组）
//	reviewer → 事件里的评审人
var notificationCueOptions = map[string][]string{
	string(domain.EventTypeMERGE_REQUEST_CREATE):          {"members"},
	string(domain.EventTypeMERGE_REQUEST_CLOSE):           {"members"},
	string(domain.EventTypeMERGE_REQUEST_MERGE):           {"members"},
	string(domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE): {"reviewer"},
	string(domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW): {"members"},
}

// notificationEventTypes 是 UserNotificationHandler 关心的全部事件。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:14-20 的
// $ListenedEventList（共 5 个，全部是合并请求相关事件）。
func notificationEventTypes() []domain.EventType {
	return []domain.EventType{
		domain.EventTypeMERGE_REQUEST_CREATE,
		domain.EventTypeMERGE_REQUEST_CLOSE,
		domain.EventTypeMERGE_REQUEST_MERGE,
		domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE,
		domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW,
	}
}

// refuseChecker 判断用户是否拒收了某个对象（仓库组 / 仓库）的通知。
//
// repo.NotificationRepo.HasRefused / RefusedFor 满足该接口。
//
// RefusedFor 是性能整改 C3 新增的批量入口：一次查询取回整批用户的拒收状态。
// 保留 HasRefused 是因为它仍是"单用户单目标"最直观的表达，接口变宽不影响实现。
type refuseChecker interface {
	HasRefused(ctx context.Context, uKey string, typ int, target string) (bool, error)
	RefusedFor(ctx context.Context, uKeys []string, gKey, rKey string) (map[string]repo.RefuseFlags, error)
}

// notifyUser 是一个"受影响用户"。
//
// 对应 PHP 侧 users 表的一行加上一个 role 字段（组成员取 gm_role，
// 仓库成员取 rm_role，仓库 / 组的创建者固定是 ROLE_OWNER）。
type notifyUser struct {
	key                string
	name               string
	email              string
	notificationStatus int
	role               int
}

// notificationHandler 发送站内信与邮件。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php。
type notificationHandler struct {
	db     *sql.DB
	cfg    *config.Config
	log    *slog.Logger
	mailer mailer.Sender
	refuse refuseChecker
}

// NewNotificationHandler 创建通知处理器。
func NewNotificationHandler(db *sql.DB, cfg *config.Config, logger *slog.Logger) Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &notificationHandler{
		db:     db,
		cfg:    cfg,
		log:    logger,
		mailer: mailer.FromEnv(cfg.SiteConfig().Email.From),
		refuse: repo.NewNotificationRepo(db),
	}
}

// Types 实现 Handler。
func (h *notificationHandler) Types() []domain.EventType { return notificationEventTypes() }

// Capture 实现 Handler。
func (h *notificationHandler) Capture(ctx context.Context, ev domain.EventPayload) error {
	tpl, ok := notificationTemplates[languageOf(ev)][ev.Type]
	if !ok {
		// 不是本 handler 关心的事件（或该语言没有文案）→ 什么都不做。
		return nil
	}
	fields, ok := notificationFieldData[ev.Type]
	if !ok {
		return nil
	}
	cueOption := notificationCueOptions[ev.Type]

	// 受众计算（PHP 侧 getAffectedUsers）。
	affected, err := h.getAffectedUsers(ctx, ev, cueOption)
	if err != nil {
		return err
	}
	if len(affected.internal) == 0 && len(affected.email) == 0 {
		// 没有任何人需要被通知。
		return nil
	}

	data, err := h.composeMessageData(ctx, ev, tpl, fields, cueOption)
	if err != nil {
		return err
	}

	// 邮件：发送失败只记日志，不让事件处理失败
	//（PHP 侧的 mail 驱动是无条件返回 true 的，新实现返回真实错误）。
	h.sendMail(ctx, ev, data, affected.email)

	// 站内信：写入失败必须让事件处理失败，否则通知会丢。
	return h.addWebSiteMessage(ctx, ev, data, affected.internal)
}

// affectedUsers 是受众计算的结果。
type affectedUsers struct {
	// internal 是要收到站内信的用户。
	internal []notifyUser
	// email 是要收到邮件的邮箱地址。
	email []string
}

// getAffectedUsers 计算事件的受影响用户。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:160-172 的
// getAffectedUsers()：先取候选名单，再依次过三道过滤。
func (h *notificationHandler) getAffectedUsers(ctx context.Context, ev domain.EventPayload, cueOption []string) (affectedUsers, error) {
	if len(cueOption) == 0 {
		return affectedUsers{}, nil
	}

	candidates, err := h.fetchUserList(ctx, ev, cueOption)
	if err != nil {
		return affectedUsers{}, err
	}

	candidates = filterOtherRule(candidates, ev)

	candidates, err = h.filterProjectNotificationSetting(ctx, candidates, ev)
	if err != nil {
		return affectedUsers{}, err
	}

	internal, emails := filterUserNotificationSetting(candidates, ev)
	return affectedUsers{internal: internal, email: emails}, nil
}

// fetchUserList 取候选名单。
//
// 逐字对应 application/libraries/service/Utility/MessageGenerator.php:21-83 的
// fetchUserList()：
//
//	members → 仓库成员 + 仓库创建者 + 组成员 + 组创建者（按此顺序）
//	reviewer → 事件里的评审人
//
// 随后按 PHP 的规则去重：主键必须是 32 位 \w、不能是事件触发者、保留首次出现。
func (h *notificationHandler) fetchUserList(ctx context.Context, ev domain.EventPayload, cueOption []string) ([]notifyUser, error) {
	var merged []notifyUser

	for _, option := range cueOption {
		switch option {
		case "members":
			repoMembers, err := h.repositoryMembers(ctx, ev)
			if err != nil {
				return nil, err
			}
			merged = append(merged, repoMembers...)

			repoCreator, err := h.repositoryCreator(ctx, ev)
			if err != nil {
				return nil, err
			}
			if repoCreator != nil {
				merged = append(merged, *repoCreator)
			}

			groupMembers, err := h.groupMembers(ctx, ev)
			if err != nil {
				return nil, err
			}
			merged = append(merged, groupMembers...)

			groupCreator, err := h.groupCreator(ctx, ev)
			if err != nil {
				return nil, err
			}
			if groupCreator != nil {
				merged = append(merged, *groupCreator)
			}

		case "reviewer":
			reviewer, err := h.reviewerUser(ctx, ev)
			if err != nil {
				return nil, err
			}
			if reviewer != nil {
				merged = append(merged, *reviewer)
			}
		}
	}

	return dedupeNotifyUsers(merged, ev.GeneratedBy), nil
}

// userKeyPattern 是 PHP 侧 preg_match('/^\w{32}$/', $item['u_key']) 的等价规则。
var userKeyPattern = regexp.MustCompile(`^\w{32}$`)

// dedupeNotifyUsers 按 PHP 规则去重候选名单。
//
// 规则：主键匹配 32 位 \w、不等于事件触发者、同一个主键只保留第一次出现。
func dedupeNotifyUsers(users []notifyUser, generatedBy string) []notifyUser {
	seen := map[string]bool{}
	out := []notifyUser{}
	for _, u := range users {
		if !userKeyPattern.MatchString(u.key) {
			continue
		}
		if u.key == generatedBy {
			continue
		}
		if seen[u.key] {
			continue
		}
		seen[u.key] = true
		out = append(out, u)
	}
	return out
}

// repositoryMembers 列出仓库成员。
//
// 逐字对应 MessageGenerator.php:48-54：
// SELECT u.*, rm.rm_role AS role FROM repository_members rm
//
//	 LEFT JOIN users u ON rm.u_key = u.u_key
//	WHERE r_key = ? AND rm_status = COMMON_STATUS_NORMAL
func (h *notificationHandler) repositoryMembers(ctx context.Context, ev domain.EventPayload) ([]notifyUser, error) {
	rKey := eventDataString(ev.Data, "rKey")
	if rKey == "" {
		return nil, nil
	}
	rows, err := h.db.QueryContext(ctx, `
		SELECT u.u_key, u.u_name, u.u_email, u.u_notification_status, rm.rm_role
		FROM cc_repository_members AS rm
		LEFT JOIN cc_users AS u ON rm.u_key = u.u_key
		WHERE rm.r_key = ? AND rm.rm_status = ?`,
		rKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, fmt.Errorf("查询仓库成员失败: %w", err)
	}
	return scanNotifyUsers(rows)
}

// groupMembers 列出仓库组成员。
//
// 逐字对应 MessageGenerator.php:34-40。
func (h *notificationHandler) groupMembers(ctx context.Context, ev domain.EventPayload) ([]notifyUser, error) {
	gKey := eventDataString(ev.Data, "gKey")
	if gKey == "" {
		return nil, nil
	}
	rows, err := h.db.QueryContext(ctx, `
		SELECT u.u_key, u.u_name, u.u_email, u.u_notification_status, gm.gm_role
		FROM cc_group_members AS gm
		LEFT JOIN cc_users AS u ON gm.u_key = u.u_key
		WHERE gm.g_key = ? AND gm.gm_status = ?`,
		gKey, domain.CommonStatusNormal)
	if err != nil {
		return nil, fmt.Errorf("查询组成员失败: %w", err)
	}
	return scanNotifyUsers(rows)
}

// repositoryCreator 取仓库创建者（角色固定为 ROLE_OWNER）。
//
// 逐字对应 MessageGenerator.php:56-59。
func (h *notificationHandler) repositoryCreator(ctx context.Context, ev domain.EventPayload) (*notifyUser, error) {
	rKey := eventDataString(ev.Data, "rKey")
	if rKey == "" {
		return nil, nil
	}
	var ownerKey string
	err := h.db.QueryRowContext(ctx,
		"SELECT u_key FROM cc_repositories WHERE r_key = ? LIMIT 1", rKey).Scan(&ownerKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询仓库创建者失败: %w", err)
	}
	user, err := h.userByKey(ctx, ownerKey)
	if err != nil || user == nil {
		return nil, err
	}
	user.role = int(domain.RoleOwner)
	return user, nil
}

// groupCreator 取仓库组创建者（角色固定为 ROLE_OWNER）。
//
// 逐字对应 MessageGenerator.php:42-45。
func (h *notificationHandler) groupCreator(ctx context.Context, ev domain.EventPayload) (*notifyUser, error) {
	gKey := eventDataString(ev.Data, "gKey")
	if gKey == "" {
		return nil, nil
	}
	var ownerKey string
	err := h.db.QueryRowContext(ctx,
		"SELECT u_key FROM cc_groups WHERE g_key = ? LIMIT 1", gKey).Scan(&ownerKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询组创建者失败: %w", err)
	}
	user, err := h.userByKey(ctx, ownerKey)
	if err != nil || user == nil {
		return nil, err
	}
	user.role = int(domain.RoleOwner)
	return user, nil
}

// reviewerUser 取事件里的评审人。
//
// 逐字对应 MessageGenerator.php:62-68（评审人没有 role，PHP 侧也没用到）。
func (h *notificationHandler) reviewerUser(ctx context.Context, ev domain.EventPayload) (*notifyUser, error) {
	reviewer := eventDataString(ev.Data, "reviewer")
	if reviewer == "" {
		return nil, nil
	}
	return h.userByKey(ctx, reviewer)
}

// userByKey 按主键查用户。
func (h *notificationHandler) userByKey(ctx context.Context, uKey string) (*notifyUser, error) {
	if uKey == "" {
		return nil, nil
	}
	var u notifyUser
	var key, name, email sql.NullString
	var status sql.NullInt64
	err := h.db.QueryRowContext(ctx, `
		SELECT u_key, u_name, u_email, u_notification_status
		FROM cc_users WHERE u_key = ? LIMIT 1`, uKey).
		Scan(&key, &name, &email, &status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	u.key = key.String
	u.name = name.String
	u.email = email.String
	u.notificationStatus = int(status.Int64)
	return &u, nil
}

// scanNotifyUsers 扫描成员查询结果。
//
// LEFT JOIN 命中不到用户时 u.* 全是 NULL —— PHP 侧 preg_match 会把这些行
// 过滤掉，这里保留空主键，由 dedupeNotifyUsers 统一丢弃。
func scanNotifyUsers(rows *sql.Rows) ([]notifyUser, error) {
	defer rows.Close()
	out := []notifyUser{}
	for rows.Next() {
		var u notifyUser
		var key, name, email sql.NullString
		var status sql.NullInt64
		var role sql.NullInt64
		if err := rows.Scan(&key, &name, &email, &status, &role); err != nil {
			return nil, fmt.Errorf("扫描成员失败: %w", err)
		}
		u.key = key.String
		u.name = name.String
		u.email = email.String
		u.notificationStatus = int(status.Int64)
		u.role = int(role.Int64)
		out = append(out, u)
	}
	return out, rows.Err()
}

// filterOtherRule 按事件类型过滤受影响用户。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:202-231 的
// filterOtherRule()：
//
//   - 评审通过事件只通知 role > ROLE_DEVELOPER；
//   - 合并请求创建 / 关闭 / 合并只通知 role > ROLE_REPORTER；
//   - 合并请求创建时若已指定评审人，则**清空**广播名单（改为定向通知）；
//   - 指派评审人事件不做角色过滤。
func filterOtherRule(affected []notifyUser, ev domain.EventPayload) []notifyUser {
	result := []notifyUser{}

	switch ev.Type {
	case string(domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW):
		for _, u := range affected {
			if u.role > int(domain.RoleDeveloper) {
				result = append(result, u)
			}
		}

	case string(domain.EventTypeMERGE_REQUEST_CREATE),
		string(domain.EventTypeMERGE_REQUEST_CLOSE),
		string(domain.EventTypeMERGE_REQUEST_MERGE):
		users := affected
		if ev.Type == string(domain.EventTypeMERGE_REQUEST_CREATE) && hasReviewers(ev.Data) {
			users = nil
		}
		for _, u := range users {
			if u.role > int(domain.RoleReporter) {
				result = append(result, u)
			}
		}

	case string(domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE):
		result = affected
	}

	return result
}

// hasReviewers 判断合并请求创建事件里是否已经指定了评审人。
//
// 逐字对应 UserNotificationHandler.php:217 的 `$event->data->reviewers` 真值判断：
// 空字符串与 "0" 都是假值。
func hasReviewers(data map[string]any) bool {
	raw, ok := data["reviewers"]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case string:
		return v != "" && v != "0"
	case []any:
		return len(v) > 0
	case []string:
		return len(v) > 0
	case bool:
		return v
	default:
		return true
	}
}

// filterProjectNotificationSetting 按用户对组 / 仓库的通知设置过滤。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:175-200 的
// filterProjectNotificationSetting()：命中组级或仓库级拒收记录即被排除。
//
// 级联语义：PHP 侧写入拒收配置时（notification_model.php:180-206）会把组下
// 所有仓库一并写入拒收记录，因此"拒收某个组"等价于"该组下所有仓库都拒收"。
// 本实现同时检查组与仓库两条记录，语义等价且不依赖写入端的级联是否执行过。
//
// 性能整改 C3（2026-10）：整改前这里对每个收件人各查两次（组级 + 仓库级
// COUNT(*)），200 人的仓库事件就是 400 条查询。改为一次批量查询
// （repo.NotificationRepo.RefusedFor），查询数固定为 1，与人数无关。
func (h *notificationHandler) filterProjectNotificationSetting(ctx context.Context, affected []notifyUser, ev domain.EventPayload) ([]notifyUser, error) {
	if len(affected) == 0 {
		return []notifyUser{}, nil
	}
	gKey := eventDataString(ev.Data, "gKey")
	rKey := eventDataString(ev.Data, "rKey")

	uKeys := make([]string, 0, len(affected))
	for _, u := range affected {
		uKeys = append(uKeys, u.key)
	}
	refused, err := h.refuse.RefusedFor(ctx, uKeys, gKey, rKey)
	if err != nil {
		return nil, fmt.Errorf("查询通知拒收配置失败: %w", err)
	}

	result := []notifyUser{}
	for _, u := range affected {
		// map 里没有该用户 = 组级与仓库级都未拒收。
		if flags, ok := refused[u.key]; ok && (flags.Group || flags.Repository) {
			continue
		}
		result = append(result, u)
	}
	return result, nil
}

// filterUserNotificationSetting 按 u_notification_status 位掩码拆分站内信 / 邮件名单。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:234-266 的
// filterUserNotificationSetting()：
//
//	NOTIFICATION_STATUS_MR    = 1 → 第 1 位，**位 = 1 表示关闭合并请求通知**
//	NOTIFICATION_STATUS_EMAIL = 2 → 第 2 位，**位 = 1 表示关闭邮件通知**
//
// 合并请求类事件里，关闭 MR 通知的用户两个通道都不收；
// 只关闭邮件的用户仍收站内信。
func filterUserNotificationSetting(affected []notifyUser, ev domain.EventPayload) ([]notifyUser, []string) {
	internal := []notifyUser{}
	emails := []string{}

	for _, u := range affected {
		if isMergeRequestEvent(ev.Type) {
			if (u.notificationStatus >> (repo.NotificationStatusMR - 1) & 1) == 1 {
				continue
			}
		}

		internal = append(internal, u)

		if (u.notificationStatus >> (repo.NotificationStatusEmail - 1) & 1) == 0 {
			emails = append(emails, u.email)
		}
	}
	return internal, emails
}

// isMergeRequestEvent 判断事件是否属于合并请求类。
func isMergeRequestEvent(evType string) bool {
	switch evType {
	case string(domain.EventTypeMERGE_REQUEST_CREATE),
		string(domain.EventTypeMERGE_REQUEST_CLOSE),
		string(domain.EventTypeMERGE_REQUEST_MERGE),
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE),
		string(domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW):
		return true
	}
	return false
}

// messageData 是 composeMessageData 的产物。
type messageData struct {
	mailTitle string
	mailBody  string
	// internal 是站内信内容（有序 JSON 对象）。
	internal *jsonObject
}

// value 读取站内信内容里的一个字段。
func (m *messageData) value(key string) string {
	for i, k := range m.internal.keys {
		if k == key {
			if s, ok := m.internal.values[i].(string); ok {
				return s
			}
			return ""
		}
	}
	return ""
}

// composeMessageData 组装消息内容。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:268-326 的
// composeMessageData() / toInternalData() / parseInternalData()：
//
//  1. 站内信字段先按 "键 + 分隔符 + 模板" 拼成多行文本；
//  2. 标题、正文、cueOption、站内信文本用**双分隔符**连成一个模板；
//  3. 一次性解析全部占位符，再按双分隔符切回四段；
//  4. 站内信文本按行拆回键值对。
func (h *notificationHandler) composeMessageData(ctx context.Context, ev domain.EventPayload, tpl notificationTemplate, fields []notificationField, cueOption []string) (*messageData, error) {
	return composeMessageData(ctx, &templateResolver{db: h.db, ev: ev}, tpl, fields, cueOption)
}

// composeMessageData 组装消息内容。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:268-326 的
// composeMessageData() / toInternalData() / parseInternalData()：
//
//  1. 站内信字段先按 "键 + 分隔符 + 模板" 拼成多行文本（行间用 \n 分隔）；
//  2. 标题、正文、cueOption、站内信文本用**双分隔符**连成一个模板；
//  3. 一次性解析全部占位符，再按双分隔符切回四段；
//  4. 站内信文本按行拆回键值对。
func composeMessageData(ctx context.Context, resolver *templateResolver, tpl notificationTemplate, fields []notificationField, cueOption []string) (*messageData, error) {
	// 站内信字段先按 "键 + 分隔符 + 模板" 拼成多行文本（行间用 \n 分隔）。
	//
	// 逐字对应 UserNotificationHandler.php:291-306 的 toInternalData()：
	// $result[] = $key . $delimiter . $val; implode("\n", $result);
	lines := make([]string, 0, len(fields))
	for _, f := range fields {
		lines = append(lines, f.key+messageDelimiter+f.value)
	}
	internalText := strings.Join(lines, "\n")

	parts := []string{
		tpl.mailTitle,
		tpl.mailBody,
		strings.Join(cueOption, messageDelimiter),
		internalText,
	}
	template := strings.Join(parts, messageDelimiter+messageDelimiter)

	resolved, err := resolver.resolve(ctx, template)
	if err != nil {
		return nil, err
	}

	sections := strings.Split(resolved, messageDelimiter+messageDelimiter)
	data := &messageData{internal: newActivityData()}
	if len(sections) > 0 {
		data.mailTitle = sections[0]
	}
	if len(sections) > 1 {
		data.mailBody = sections[1]
	}
	if len(sections) > 3 {
		data.internal = parseInternalData(sections[3])
	}
	return data, nil
}

// parseInternalData 把站内信文本拆回键值对。
//
// 逐字对应 UserNotificationHandler.php:308-326 的 parseInternalData()：
// 按行拆开，每行再按分隔符拆成键与值，只有正好两段才算数。
func parseInternalData(message string) *jsonObject {
	out := newActivityData()
	for _, line := range strings.Split(message, "\n") {
		parts := strings.Split(line, messageDelimiter)
		if len(parts) == 2 {
			out.set(parts[0], parts[1])
		}
	}
	return out
}

// addWebSiteMessage 写站内信。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:328-342 的
// addWebSiteMessage() 与 application/models/notification_model.php:88-120 的
// addNotification()：先写 cc_notification_content，再写 cc_notification_users。
func (h *notificationHandler) addWebSiteMessage(ctx context.Context, ev domain.EventPayload, data *messageData, users []notifyUser) error {
	if len(users) == 0 {
		return nil
	}

	payload, err := data.internal.marshalHexTag()
	if err != nil {
		return fmt.Errorf("序列化站内信内容失败: %w", err)
	}

	gKey := eventDataString(ev.Data, "gKey")
	rKey := eventDataString(ev.Data, "rKey")
	now := nowString()

	// 性能整改 B16（2026-10）：多行 INSERT 批量化。
	//
	// 整改前是"每个收件人 2 条独立 INSERT"（内容 + 用户行），各自 autocommit：
	// 200 人的仓库通知 = 400 条语句，每条都是一次独立往返 + 一次隐式事务提交。
	//
	// 现在按批拼成多行 VALUES：仍然保持"每人一行内容 + 一人一行用户关系"的
	// 数据形态（不改数据模型，因此所有读取路径无需改动），只是把 N 次发送
	// 压缩成 ceil(N/批大小) 次。
	//
	// 每批都是一条语句，因此天然原子：要么这批全写入，要么全不写。
	// 中途失败时整批回滚由数据库保证，已写入的批次保留 —— 这与原实现在
	// "部分成功后失败" 时的行为一致（原实现同样是前面的人已写入）。
	const batchSize = 100

	for start := 0; start < len(users); start += batchSize {
		if handlerContextIsDone(ctx) {
			return ctx.Err()
		}
		end := start + batchSize
		if end > len(users) {
			end = len(users)
		}
		batch := users[start:end]

		contentSQL := "INSERT INTO cc_notification_content " +
			"(nc_key, nc_type, g_key, r_key, nc_data, nc_scope, nc_created, nc_updated, nc_deleted, nc_status) VALUES "
		userSQL := "INSERT INTO cc_notification_users " +
			"(nu_key, u_key, nc_key, nu_is_read, nu_created, nu_updated, nu_deleted, nu_status) VALUES "

		contentArgs := make([]any, 0, len(batch)*10)
		userArgs := make([]any, 0, len(batch)*8)
		for i, u := range batch {
			if i > 0 {
				contentSQL += ", "
				userSQL += ", "
			}
			ncKey := contract.NewKey()
			contentSQL += "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
			contentArgs = append(contentArgs,
				ncKey, ev.Type, gKey, rKey, payload, repo.NotifyForUser,
				now, zeroDateTime, zeroDateTime, domain.CommonStatusNormal)

			userSQL += "(?, ?, ?, ?, ?, ?, ?, ?)"
			userArgs = append(userArgs,
				contract.NewKey(), u.key, ncKey, repo.NotifyUnread,
				now, zeroDateTime, zeroDateTime, domain.CommonStatusNormal)
		}

		if _, err := h.db.ExecContext(ctx, contentSQL, contentArgs...); err != nil {
			return fmt.Errorf("写入通知内容失败: %w", err)
		}
		if _, err := h.db.ExecContext(ctx, userSQL, userArgs...); err != nil {
			return fmt.Errorf("写入用户通知失败: %w", err)
		}
	}
	return nil
}

// sendMail 发送邮件。
//
// 逐字对应 application/event_handlers/UserNotificationHandler.php:344-372 的
// sendMail()：正文用 EmailTemplate::notify() 拼装，逐个邮箱发送。
//
// 与 PHP 的差异：PHP 的 mail 驱动**无条件返回 true**，发送失败上层感知不到；
// 新实现返回真实错误，但**只记日志**——邮件发不出去不能让整个事件处理失败。
func (h *notificationHandler) sendMail(ctx context.Context, ev domain.EventPayload, data *messageData, emails []string) {
	if len(emails) == 0 {
		return
	}
	if h.mailer == nil {
		h.log.Error("没有可用的邮件发送器", "event", ev.Type)
		return
	}

	url := h.mergeRequestURL(data)
	body := emailNotifyTemplate(data.mailBody, url)
	from := h.fromAddress()

	for _, email := range emails {
		if handlerContextIsDone(ctx) {
			return
		}
		err := h.mailer.Send(ctx, mailer.Message{
			To:      email,
			Subject: data.mailTitle,
			Body:    body,
			From:    from,
		})
		if err != nil {
			h.log.Error("发送邮件失败", "event", ev.Type, "to", email, "error", err)
		}
	}
}

// mergeRequestURL 拼出合并请求的页面地址。
//
// 逐字对应 UserNotificationHandler.php:350 与 360：
// $host = YAML_HOST . '/'; $url = $host . group . '/' . repository . '/mergerequests/' . number;
func (h *notificationHandler) mergeRequestURL(data *messageData) string {
	host := ""
	if h.cfg != nil {
		host = h.cfg.SiteConfig().Host
	}
	if host != "" {
		host = strings.TrimRight(host, "/") + "/"
	}
	return host + data.value("group") + "/" + data.value("repository") + "/mergerequests/" + data.value("number")
}

// fromAddress 拼出发件人地址。
//
// 逐字对应 application/libraries/service/MessageService/Email/Driver/Mail.php:15：
// $from = YAML_EMAIL_NAME . '<' . YAML_EMAIL_FROM . '>';
func (h *notificationHandler) fromAddress() string {
	if h.cfg == nil {
		return ""
	}
	return h.cfg.SiteConfig().Email.Name + "<" + h.cfg.SiteConfig().Email.From + ">"
}

// emailNotifyTemplate 是邮件正文模板。
//
// 逐字对应 application/libraries/service/MessageService/Email/EmailTemplate.php:13-16：
// sprintf('【CodeFever Next】%s <br/> <a href="%s">%s</a>', mailBody, url, url)
func emailNotifyTemplate(mailBody, url string) string {
	return fmt.Sprintf("【CodeFever Next】%s <br/> <a href=\"%s\">%s</a>", mailBody, url, url)
}

// languageOf 返回事件语言对应的模板语言。
//
// PHP 侧事件语言是 LANG_CN(1) / LANG_EN(2)，这里对应 Go 的 zh-cn / en-us。
// 未识别一律按中文处理（PHP 侧缺省也是 LANG_CN）。
func languageOf(ev domain.EventPayload) string {
	if ev.Language == domain.LanguageEN {
		return domain.LanguageEN
	}
	return domain.LanguageCN
}

// ---------------------------------------------------------------------------
// 模板占位符解析
// ---------------------------------------------------------------------------

// placeholderPattern 是消息模板里的占位符。
//
// 逐字对应 application/libraries/service/Utility/MessageGenerator.php:87 的
// preg_match_all('/{{((?:(?!}}[^}]).)*)}}/', ...)。
var placeholderPattern = regexp.MustCompile(`\{\{(.+?)\}\}`)

// 模板表达式的三种形态（逐字对应 MessageGenerator.php:146-205）：
var (
	// directVarPattern 匹配 {{$user}} 与 {{$data->rKey}}。
	directVarPattern = regexp.MustCompile(`^\$([\w"'\[\]\->]+)$`)
	// indirectVarPattern 匹配 {{$user@users:u_name}}。
	indirectVarPattern = regexp.MustCompile(`^\$([\w"'\[\]\->]+)@(\w+):(\w+)$`)
	// tableVarPattern 匹配 {{users:u_name}}（{table}:{pk} 规则）。
	tableVarPattern = regexp.MustCompile(`^(\w+):(\w+)$`)
)

// templateTableSpec 描述一个可被模板引用的表。
type templateTableSpec struct {
	// table 是物理表名（含 cc_ 前缀）。
	//
	// PHP 侧的 CodeIgniter 会自动给表名加 dbprefix（cc_），
	// 而模板里写的是不带前缀的逻辑表名（users / groups / ...），
	// 因此这里把两者分开记录。
	table string
	// primaryKey 是该表的主键列。
	primaryKey string
	// columns 是模板允许引用的列（白名单）。
	columns map[string]bool
}

// templateTables 是模板允许引用的表与列。
//
// 逐字对应 application/libraries/service/Utility/MessageGenerator.php:13-19 的
// $databaseConfig（表 → 主键）；列名在此额外做白名单校验，
// 避免模板拼出非预期 SQL。
var templateTables = map[string]templateTableSpec{
	"users":                   {table: "cc_users", primaryKey: "u_key", columns: map[string]bool{"u_name": true, "u_email": true}},
	"groups":                  {table: "cc_groups", primaryKey: "g_key", columns: map[string]bool{"g_name": true, "g_display_name": true}},
	"repositories":            {table: "cc_repositories", primaryKey: "r_key", columns: map[string]bool{"r_name": true, "r_display_name": true}},
	"merge_requests":          {table: "cc_merge_requests", primaryKey: "mr_key", columns: map[string]bool{"mr_title": true, "mr_description": true, "mr_created": true, "mr_close_time": true, "mr_merge_time": true}},
	"merge_request_reviewers": {table: "cc_merge_request_reviewers", primaryKey: "mrr_key", columns: map[string]bool{"mrr_created": true}},
}

// templateResolver 解析消息模板里的占位符。
//
// 逐字对应 application/libraries/service/Utility/MessageGenerator.php 的
// getMessageData() / _messageQueryBuilder()：
//
//	{{$user}}                        → 事件触发者
//	{{$data->rKey}}                  → 事件数据里的字段
//	{{$user@users:u_name}}           → 用触发者主键查 users.u_name
//	{{$data->rKey@repositories:r_name}} → 用 rKey 查 repositories.r_name
//	{{users:u_name}}                 → 用事件触发者主键查 users.u_name
type templateResolver struct {
	db *sql.DB
	ev domain.EventPayload
	// lookupFn 覆盖查表逻辑（测试用）；为 nil 时走数据库。
	lookupFn func(ctx context.Context, table, column, key string) (string, error)
}

// resolve 解析模板里的全部占位符。
func (r *templateResolver) resolve(ctx context.Context, template string) (string, error) {
	matches := placeholderPattern.FindAllStringSubmatchIndex(template, -1)
	if len(matches) == 0 {
		return template, nil
	}

	var sb strings.Builder
	prev := 0
	for _, m := range matches {
		sb.WriteString(template[prev:m[0]])
		value, err := r.resolveOne(ctx, template[m[2]:m[3]])
		if err != nil {
			return "", err
		}
		sb.WriteString(value)
		prev = m[1]
	}
	sb.WriteString(template[prev:])
	return sb.String(), nil
}

// resolveOne 解析单个占位符表达式。
func (r *templateResolver) resolveOne(ctx context.Context, expr string) (string, error) {
	// {{$var@table:column}} —— 用事件里的主键查表。
	if m := indirectVarPattern.FindStringSubmatch(expr); m != nil {
		key, ok := r.property(m[1])
		if !ok {
			return "", nil
		}
		return r.lookup(ctx, m[2], m[3], key)
	}

	// {{$var}} —— 事件里的字段。
	if m := directVarPattern.FindStringSubmatch(expr); m != nil {
		value, ok := r.property(m[1])
		if !ok {
			return "", nil
		}
		return value, nil
	}

	// {{table:column}} —— 用事件触发者主键查表（{table}:{pk} 规则）。
	if m := tableVarPattern.FindStringSubmatch(expr); m != nil {
		return r.lookup(ctx, m[1], m[2], r.ev.GeneratedBy)
	}

	return "", nil
}

// property 读取事件里的字段。
//
// 逐字对应 application/libraries/service/Utility/MessageGenerator.php:267-280 的
// _objectPropertyGetter()：$user 是事件触发者，$data->xxx 是事件数据里的字段。
func (r *templateResolver) property(path string) (string, bool) {
	parts := strings.Split(path, "->")

	var current any
	switch parts[0] {
	case "user":
		current = r.ev.GeneratedBy
	case "data":
		current = r.ev.Data
	case "id":
		current = r.ev.ID
	case "type":
		current = r.ev.Type
	case "timestamp":
		current = r.ev.Timestamp
	case "language":
		current = r.ev.Language
	default:
		return "", false
	}

	for _, p := range parts[1:] {
		m, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		v, present := m[p]
		if !present {
			return "", false
		}
		current = v
	}
	return scalarString(current), true
}

// lookup 按主键查表取一列的值。
//
// 逐字对应 application/libraries/service/Utility/MessageGenerator.php:174-185 的
// databaseVarible 规则：
//
//	SELECT {table}.{column} FROM {table} WHERE {table}.{pk} = {value}
//
// 查不到行时 PHP 得到 NULL，最终渲染成空字符串。
func (r *templateResolver) lookup(ctx context.Context, table, column, key string) (string, error) {
	if r.lookupFn != nil {
		return r.lookupFn(ctx, table, column, key)
	}
	spec, ok := templateTables[table]
	if !ok || !spec.columns[column] {
		return "", fmt.Errorf("模板引用了不允许的表或列: %s.%s", table, column)
	}
	if key == "" {
		return "", nil
	}
	query := fmt.Sprintf(
		"SELECT CAST(`%s` AS CHAR) FROM `%s` WHERE `%s` = ? LIMIT 1",
		column, spec.table, spec.primaryKey)
	var out sql.NullString
	if err := r.db.QueryRowContext(ctx, query, key).Scan(&out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("查询模板字段失败(%s.%s): %w", table, column, err)
	}
	return out.String, nil
}
