// 仓库域**写端点**的业务规则与归一化形态。
//
// 本文件只放"纯规则"（名称校验、状态机、事件匹配、输出结构），
// 不碰数据库与 git —— 那些分别在 repo 与 gitsvc 包里。
//
// 每个函数都标注了对应的 PHP 源码位置，便于逐条比对契约。
package domain

import (
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// 状态常量
// ---------------------------------------------------------------------------

// CommonStatusDelete 对应 PHP 常量 COMMON_STATUS_DELETE。
//
// 软删除的行把 *_status 置为 4（不是 0），因此"未删除"必须用
// CommonStatusNormal 判定，不能用 `status != 0`。
const CommonStatusDelete = 4

// GlobalTrue / GlobalFalse 对应 PHP 常量 GLOBAL_TRUE / GLOBAL_FALSE。
//
// 注意这不是布尔：cc_merge_request_reviewers.mrr_is_review 用 1 表示
// "已评审"、2 表示"未评审"（见 application/config/constants.php:74-75）。
const (
	GlobalTrue  = 1
	GlobalFalse = 2
)

// ---------------------------------------------------------------------------
// 名称校验
// ---------------------------------------------------------------------------

// branchNameRE 对应 application/controllers/api/repository.php:1419 的
// preg_match('/^\w[\w\/\.\-]{0,29}$/', $name)。
//
// 语义：以 \w 开头，后接至多 29 个 [\w/.-] 字符，总长 1~30。
// PHP 的 \w 在无 /u 修饰符时等价于 [A-Za-z0-9_]，与 Go 的 \w 一致。
var branchNameRE = regexp.MustCompile(`^\w[\w/\.\-]{0,29}$`)

// tagNameRE 对应 application/controllers/api/repository.php:1933 的
// preg_match('/^\w+(\.\w+)*$/', $name)。
//
// 语义：点号分隔的若干段，每段由 \w+ 组成（v1.0 合法，v1. 非法，.v1 非法）。
var tagNameRE = regexp.MustCompile(`^\w+(\.\w+)*$`)

// BranchNameValid 判断分支名是否合法。
//
// 不合法时 PHP 侧返回 0x0202（Missing Amount Or Amount Out Of Range ——
// 错误码表的文案与实际用途对不上，但数值必须保持一致）。
func BranchNameValid(name string) bool { return branchNameRE.MatchString(name) }

// TagNameValid 判断标签名是否合法（不合法时同样返回 0x0202）。
func TagNameValid(name string) bool { return tagNameRE.MatchString(name) }

// PHPTrim 复刻 PHP 的 trim() 默认字符集：空格、制表符、换行、回车、NUL、垂直制表符。
//
// 仓库域大量使用 trim($data['xxx'])，Go 的 strings.TrimSpace 会额外去掉
// 各种 Unicode 空白（如 U+00A0），与 PHP 不等价，因此这里单列一个函数。
func PHPTrim(s string) string { return strings.Trim(s, " \t\n\r\x00\x0B") }

// ---------------------------------------------------------------------------
// 保护分支规则
// ---------------------------------------------------------------------------

// ProtectedBranchRuleView 是保护分支规则的对外形态。
//
// 逐字对应 application/models/repository_model.php:98-111 的
// normalizeProtectedBranchRules()。
type ProtectedBranchRuleView struct {
	ID        string `json:"id"`
	Rule      string `json:"rule"`
	RolePush  int    `json:"rolePush"`
	RoleMerge int    `json:"roleMerge"`
}

// NormalizeProtectedBranchRules 把保护分支规则行转换为对外形态。
func NormalizeProtectedBranchRules(list []ProtectedBranchRule) []ProtectedBranchRuleView {
	out := make([]ProtectedBranchRuleView, 0, len(list))
	for i := range list {
		out = append(out, ProtectedBranchRuleView{
			ID:        list[i].Key,
			Rule:      list[i].Rule,
			RolePush:  list[i].PushRole,
			RoleMerge: list[i].MergeRole,
		})
	}
	return out
}

// ProtectedBranchPushRoleAllowed 判断 push 最低角色参数是否被 PHP 接受。
//
// 逐字对应 application/controllers/api/repository.php:696-701 的
// in_array($rolePush, [ROLE_NO_BODY, ROLE_OWNER, ROLE_MAINTAINER, ROLE_DEVELOPER])。
func ProtectedBranchPushRoleAllowed(raw string, present bool) bool {
	if !present {
		return false
	}
	for _, r := range []int{
		int(RoleNoBody), int(RoleOwner), int(RoleMaintainer), int(RoleDeveloper),
	} {
		if PHPLooseEqualsInt(raw, r) {
			return true
		}
	}
	return false
}

// ProtectedBranchMergeRoleAllowed 判断 merge 最低角色参数是否被 PHP 接受。
//
// 逐字对应 application/controllers/api/repository.php:701-705 的
// in_array($roleMerge, [ROLE_NO_BODY, ROLE_OWNER, ROLE_MAINTAINER])。
func ProtectedBranchMergeRoleAllowed(raw string, present bool) bool {
	if !present {
		return false
	}
	for _, r := range []int{int(RoleNoBody), int(RoleOwner), int(RoleMaintainer)} {
		if PHPLooseEqualsInt(raw, r) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 合并请求状态机
// ---------------------------------------------------------------------------

// MergeRequestCanTransition 判断合并请求能否从状态 from 流转到状态 to。
//
// 逐字对应 PHP 侧的隐含状态机：
//
//	mergeRequestClose()：只有 OPEN 的 MR 能被关闭
//	mergeBranch()       ：只有 OPEN 的 MR 能被合并
//	mergeRequestReview()：任何状态的 MR 都能被评审
//
// 因此唯一合法的起点是 OPEN，终点可以是 OPEN（评审）/ CLOSED / MERGED。
func MergeRequestCanTransition(from, to int) bool {
	if from != MergeRequestStatusOpen {
		return false
	}
	switch to {
	case MergeRequestStatusOpen, MergeRequestStatusClosed, MergeRequestStatusMerged:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Webhook
// ---------------------------------------------------------------------------

// RepositoryWebhook 是 cc_repository_webhooks 表的一行。
//
// 注意 rw_secret 是**明文**存储 —— 这是 PHP 侧的既有行为，
// 前端需要回显 webhook 配置（application/models/repository_model.php:2124-2142
// 直接把 rw_secret 放进响应）。此处保持兼容，但不要新增其它明文密钥。
type RepositoryWebhook struct {
	Key     string
	RKey    string
	UKey    string
	URL     string
	Secret  string
	Events  string
	Active  int
	Updated string
}

// RepositoryWebhookView 是 webhook 的对外形态。
//
// 逐字对应 application/models/repository_model.php:2124-2142 的
// normalizeWebhooks()。
//
// user 取自 LEFT JOIN cc_users 的 u_name；单条查询（getWebhook）没有 JOIN，
// 该键不存在 → PHP 输出 null，因此这里是 *string。
type RepositoryWebhookView struct {
	ID      string  `json:"id"`
	User    *string `json:"user"`
	URL     string  `json:"url"`
	Secret  string  `json:"secret"`
	Events  string  `json:"events"`
	Active  int     `json:"active"`
	Updated int64   `json:"updated"`
}

// NormalizeRepositoryWebhook 把 webhook 行转换为对外形态。
func NormalizeRepositoryWebhook(w *RepositoryWebhook, userName *string) RepositoryWebhookView {
	if w == nil {
		return RepositoryWebhookView{}
	}
	return RepositoryWebhookView{
		ID:      w.Key,
		User:    userName,
		URL:     w.URL,
		Secret:  w.Secret,
		Events:  w.Events,
		Active:  w.Active,
		Updated: PhpTime(w.Updated),
	}
}

// WebhookEventMatches 判断 webhook 订阅的事件列表是否包含给定事件类型。
//
// 逐字对应 application/models/repository_model.php:2233-2236 的
// explode(',', $webhook['rw_events']) + in_array($eventType, $eventTypes)：
// 逗号分隔的**精确**匹配，不做通配。
func WebhookEventMatches(events, eventType string) bool {
	if events == "" {
		return false
	}
	for _, item := range strings.Split(events, ",") {
		if item == eventType {
			return true
		}
	}
	return false
}

// RepositoryWebhookLog 是 cc_repository_webhook_logs 表的一行。
type RepositoryWebhookLog struct {
	ID       string
	Webhook  string
	Request  string
	Response string
	Start    float64
	End      float64
	Status   int
	Created  string
}

// RepositoryWebhookLogView 是 webhook 投递日志的对外形态。
//
// 逐字对应 application/models/repository_model.php:2319-2335 的
// normalizeRepositoryWebhookLogs()。
//
// created 是**原始字符串**（PHP 侧没有对它做 strtotime）。
type RepositoryWebhookLogView struct {
	ID      string  `json:"id"`
	Webhook string  `json:"webhook"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Status  int     `json:"status"`
	Success bool    `json:"success"`
	Created string  `json:"created"`
}

// NormalizeRepositoryWebhookLogs 把投递日志行转换为对外形态。
func NormalizeRepositoryWebhookLogs(list []RepositoryWebhookLog) []RepositoryWebhookLogView {
	out := make([]RepositoryWebhookLogView, 0, len(list))
	for i := range list {
		out = append(out, RepositoryWebhookLogView{
			ID:      list[i].ID,
			Webhook: list[i].Webhook,
			Start:   list[i].Start,
			End:     list[i].End,
			Status:  list[i].Status,
			Success: list[i].Status == 200,
			Created: list[i].Created,
		})
	}
	return out
}

// RepositoryWebhookLogData 是单条投递日志的请求/响应内容。
//
// 逐字对应 application/models/repository_model.php:2298-2304 的
// normalizeRepositoryWebhookLogData()：对 rwl_request / rwl_response
// 做 json_decode（非关联数组 → 对象）。
type RepositoryWebhookLogData struct {
	Request  any `json:"request"`
	Response any `json:"response"`
}
