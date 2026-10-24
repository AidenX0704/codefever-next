package domain

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// 表行
//
// 仓库域的行模型统一带 Repository 前缀（见 domain/repository.go），
// 本文件是仓库组域，因此不加前缀。
// ---------------------------------------------------------------------------

// Group 是 cc_groups 表的一行。
type Group struct {
	Key         string
	OwnerKey    string
	Type        int
	DisplayName string
	Name        string
	Description string
	Avatar      *string
	// Created / Updated 是 MySQL 返回的时间。
	//
	// 零值日期 '0000-00-00 00:00:00' 在 PHP 里是 -62169984000，
	// 而 Go 的 time.Time 零值（0001-01-01）是 -62135596800 ——
	// 因此仓储层统一用 domain.PHPUnixTime 换算后再存入。
	Created time.Time
	Updated time.Time
	Status  int
}

// GroupMember 是 cc_group_members 表的一行。
type GroupMember struct {
	Key     string
	GKey    string
	UKey    string
	Role    int
	Created time.Time
	Status  int
}

// Activity 是 cc_activities 表的一行。
type Activity struct {
	Key        string
	Type       int
	UKey       string
	GroupKey   *string
	Repository *string
	MergeReq   *string
	Data       *string
	Created    time.Time
	Status     int
}

// MergeRequest 是 cc_merge_requests 表的一行。
type MergeRequest struct {
	Key           string
	UKey          string
	SourceRKey    string
	SourceBranch  string
	TargetRKey    string
	TargetBranch  string
	ID            int
	Title         string
	Description   *string
	ProceedStatus int
	Created       time.Time
	Updated       time.Time
	Status        int
}

// MergeRequestReviewer 是 cc_merge_request_reviewers 表的一行。
type MergeRequestReviewer struct {
	Key      string
	MRKey    string
	UKey     string
	IsReview bool
}

// ---------------------------------------------------------------------------
// 常量
// ---------------------------------------------------------------------------

// GroupType* 对应 PHP 常量 GROUP_TYPE_*（application/config/constants.php）。
const (
	// GroupTypeNormal 是普通组。
	GroupTypeNormal = 1
	// GroupTypeUser 是用户个人默认分组。
	GroupTypeUser = 2
)

// activityCategory* 对应 PHP 常量 ACTIVITY_CATEGORY_*。
const (
	// ActivityCategoryAll 是全部动态。
	ActivityCategoryAll = 0
	// ActivityCategoryCommit 是提交类动态。
	ActivityCategoryCommit = 1
	// ActivityCategoryMergeRequest 是合并请求类动态。
	ActivityCategoryMergeRequest = 2
	// ActivityCategoryMember 是成员类动态。
	ActivityCategoryMember = 3
)

// MergeRequestStatus* 对应 PHP 的 service\Constant\MergeRequestStatus。
const (
	// MergeRequestStatusNone 表示不过滤。
	MergeRequestStatusNone = 0
	// MergeRequestStatusOpen 是待合并。
	MergeRequestStatusOpen = 1
	// MergeRequestStatusMerged 是已合并。
	MergeRequestStatusMerged = 2
	// MergeRequestStatusClosed 是已关闭。
	MergeRequestStatusClosed = 3
)

// ---------------------------------------------------------------------------
// 归一化
// ---------------------------------------------------------------------------

// NormalizedGroup 是仓库组的归一化输出。
//
// 逐字对应 application/models/group_model.php:18-36 的 normalize()。
type NormalizedGroup struct {
	ID          string  `json:"id"`
	Icon        *string `json:"icon"`
	DisplayName string  `json:"displayName"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Owner       string  `json:"owner"`
	Type        int     `json:"type"`
	Created     int64   `json:"created"`
	Updated     int64   `json:"updated"`
	Status      int     `json:"status"`
}

// NormalizeGroup 把仓库组行转换为对外形态。
func NormalizeGroup(g *Group) NormalizedGroup {
	if g == nil {
		return NormalizedGroup{}
	}
	return NormalizedGroup{
		ID:          g.Key,
		Icon:        g.Avatar,
		DisplayName: g.DisplayName,
		Name:        g.Name,
		Description: g.Description,
		Owner:       g.OwnerKey,
		Type:        g.Type,
		Created:     g.Created.Unix(),
		Updated:     g.Updated.Unix(),
		Status:      g.Status,
	}
}

// NormalizedGroupMember 是仓库组成员的归一化输出。
//
// 逐字对应 application/models/user_model.php:18-43 的 normalize()
// 加上 group_model.php:38-54 的 normalizeMembers() 追加的字段。
//
// 注意 role 在成员语境下被 gm_role / ROLE_OWNER **覆盖**成整数
// （不再是用户的 u_role 字符串），因此这里的类型是 int 而非 *string。
type NormalizedGroupMember struct {
	ID                 string   `json:"id"`
	Icon               *string  `json:"icon"`
	Name               string   `json:"name"`
	Email              string   `json:"email"`
	PhoneCode          *string  `json:"phoneCode"`
	PhoneNumber        *string  `json:"phoneNumber"`
	Team               *string  `json:"team"`
	Role               int      `json:"role"`
	Notification       int      `json:"notification"`
	MFAEnabled         bool     `json:"mfaEnabled"`
	Admin              bool     `json:"admin"`
	Emails             []string `json:"emails"`
	UnReadNotification int      `json:"unReadNotification"`
	Status             bool     `json:"status"`
	Host               string   `json:"host"`
	SSH                string   `json:"ssh"`
	Joined             int64    `json:"joined"`
	GroupMember        bool     `json:"groupMember"`
	CreatorFlag        bool     `json:"creatorFlag"`
	DeleteFlag         bool     `json:"deleteFlag"`
}

// NormalizeGroupMember 把成员行与对应用户转换为对外形态。
//
// 逐字对应 application/models/group_model.php:38-54 的 normalizeMembers()：
// 先取用户归一化形态，再覆盖 role（组内角色）与 joined（入组时间），
// 并补上 groupMember / creatorFlag / deleteFlag 三个成员标记。
//
// user 的提交邮箱以"已验证邮箱字符串数组"形态给出 —— PHP 侧
// normalize() 的 $extra 默认为 FALSE，因此 getCommitEmails() 的
// $isCheck 为 TRUE（application/models/user_model.php:35）。
func NormalizeGroupMember(m *GroupMember, u *User, emails []CommitEmail, host, ssh string) NormalizedGroupMember {
	v := NormalizeUser(u, emails, false, 0, host, ssh)
	return NormalizedGroupMember{
		ID:                 v.ID,
		Icon:               v.Icon,
		Name:               v.Name,
		Email:              v.Email,
		PhoneCode:          v.PhoneCode,
		PhoneNumber:        v.PhoneNumber,
		Team:               v.Team,
		Role:               m.Role,
		Notification:       v.Notification,
		MFAEnabled:         v.MFAEnabled,
		Admin:              v.Admin,
		Emails:             memberEmails(v.Emails),
		UnReadNotification: v.UnReadNotification,
		Status:             v.Status,
		Host:               v.Host,
		SSH:                v.SSH,
		Joined:             m.Created.Unix(),
		GroupMember:        true,
		CreatorFlag:        false,
		DeleteFlag:         m.Status != CommonStatusNormal,
	}
}

// NormalizeGroupCreator 把组创建者转换为成员形态。
//
// 逐字对应 application/models/group_model.php:56-70 的 normalizeCreator()：
// 角色恒为 ROLE_OWNER，joined 取组的创建时间，creatorFlag 为真。
func NormalizeGroupCreator(g *Group, u *User, emails []CommitEmail, host, ssh string) NormalizedGroupMember {
	v := NormalizeUser(u, emails, false, 0, host, ssh)
	return NormalizedGroupMember{
		ID:                 v.ID,
		Icon:               v.Icon,
		Name:               v.Name,
		Email:              v.Email,
		PhoneCode:          v.PhoneCode,
		PhoneNumber:        v.PhoneNumber,
		Team:               v.Team,
		Role:               int(RoleOwner),
		Notification:       v.Notification,
		MFAEnabled:         v.MFAEnabled,
		Admin:              v.Admin,
		Emails:             memberEmails(v.Emails),
		UnReadNotification: v.UnReadNotification,
		Status:             v.Status,
		Host:               v.Host,
		SSH:                v.SSH,
		Joined:             g.Created.Unix(),
		GroupMember:        true,
		CreatorFlag:        true,
		DeleteFlag:         false,
	}
}

// memberEmails 把归一化用户里的提交邮箱转成字符串数组。
func memberEmails(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// NormalizedActivity 是仓库组动态的归一化输出。
//
// 逐字对应 application/models/group_model.php:72-89 的 normalizeActivities()。
type NormalizedActivity struct {
	ID         string  `json:"id"`
	Type       int     `json:"type"`
	Creator    string  `json:"creator"`
	Group      *string `json:"group"`
	Repository *string `json:"repository"`
	Content    any     `json:"content"`
	Time       int64   `json:"time"`
}

// NormalizeActivity 把动态行转换为对外形态。
//
// content 来自 json_decode($item['a_data'], TRUE)：a_data 为 NULL 或非法 JSON
// 时 PHP 得到 null，这里同样给出 null。
func NormalizeActivity(a *Activity) NormalizedActivity {
	var content any
	if a.Data != nil {
		var decoded any
		if err := json.Unmarshal([]byte(*a.Data), &decoded); err == nil {
			content = decoded
		}
	}
	return NormalizedActivity{
		ID:         a.Key,
		Type:       a.Type,
		Creator:    a.UKey,
		Group:      a.GroupKey,
		Repository: a.Repository,
		Content:    content,
		Time:       a.Created.Unix(),
	}
}

// NormalizedReviewer 是合并请求评审人的归一化输出。
//
// 逐字对应 application/models/repository_model.php:229-242 的 normalizeReviewers()。
type NormalizedReviewer struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	IsReview bool   `json:"isReview"`
}

// NormalizedMergeRequest 是合并请求的归一化输出。
//
// 逐字对应 application/models/repository_model.php:173-194 的 normalizeMergeRequests()。
type NormalizedMergeRequest struct {
	ID               string               `json:"id"`
	SourceRepository string               `json:"sourceRepository"`
	SourceBranch     string               `json:"sourceBranch"`
	TargetRepository string               `json:"targetRepository"`
	TargetBranch     string               `json:"targetBranch"`
	Number           int                  `json:"number"`
	Title            string               `json:"title"`
	Status           int                  `json:"status"`
	Create           int64                `json:"create"`
	Update           int64                `json:"update"`
	Reviewers        []NormalizedReviewer `json:"reviewers"`
	// Commit 对应 PHP 的 getLastCommit()。
	//
	// 由 GroupHandler 在注入 git 服务后按源仓库 + 源分支查询填充；
	// 仓库不可用时保持零值（与 repository.mergeRequests 的降级一致）。
	// 见 application/models/repository_model.php:189。
	Commit Commit `json:"commit"`
}

// NormalizeMergeRequest 把合并请求行转换为对外形态。
//
// reviewers 由调用方批量查询后传入（PHP 侧是逐条查询，这里只是挪动位置）。
func NormalizeMergeRequest(m *MergeRequest, reviewers []NormalizedReviewer) NormalizedMergeRequest {
	if reviewers == nil {
		reviewers = []NormalizedReviewer{}
	}
	return NormalizedMergeRequest{
		ID:               m.Key,
		SourceRepository: m.SourceRKey,
		SourceBranch:     m.SourceBranch,
		TargetRepository: m.TargetRKey,
		TargetBranch:     m.TargetBranch,
		Number:           m.ID,
		Title:            m.Title,
		Status:           m.ProceedStatus,
		Create:           m.Created.Unix(),
		Update:           m.Updated.Unix(),
		Reviewers:        reviewers,
		Commit:           Commit{},
	}
}

// ---------------------------------------------------------------------------
// 时间
// ---------------------------------------------------------------------------

// PHPZeroDateTimestamp 对应 PHP 的 strtotime('0000-00-00 00:00:00')。
//
// PHP 把 MySQL 的零值日期解析成公元 0 年，返回 -62169984000；
// Go 的 time.Time 零值（0001-01-01）会得到 -62135596800，因此必须显式处理。
const PHPZeroDateTimestamp int64 = -62169984000

// PHPTimestamp 把 MySQL 驱动解析出的时间转换为 PHP strtotime 的结果。
//
// 两处差异需要抹平：
//   - 驱动按 DSN 的 loc（本地时区）解析时间列，而 PHP 侧是按 UTC
//     解释 MySQL 返回的原始时间串（date_default_timezone = UTC），
//     因此这里取"墙上时钟"并按 UTC 重新解释；
//   - MySQL 的零值日期 '0000-00-00 00:00:00' 在 PHP 里是 -62169984000，
//     而驱动的零值 time.Time 会得到 -62135596800。
func PHPTimestamp(t time.Time) int64 {
	if t.IsZero() {
		return PHPZeroDateTimestamp
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC).Unix()
}

// ---------------------------------------------------------------------------
// PHP 弱类型语义
// ---------------------------------------------------------------------------

// phpNumericString 匹配 PHP 的"数值串"格式（首尾允许空白）。
//
// PHP 8 的定义：可选正负号 + 数字（可带小数点与十进制指数）。
// 空串、"abc"、"1abc" 都不是数值串。
var phpNumericString = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)

// PHPNumericValue 把 PHP 数值串转成数值。
//
// 非数值串返回 ok=false —— PHP 8 里它与整数做弱类型比较时不会发生数值转换。
func PHPNumericValue(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(raw)
	if !phpNumericString.MatchString(trimmed) {
		return 0, false
	}
	v, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// PHPLooseEqualsInt 判断 PHP 8 弱类型比较下 $string == $int 是否成立。
//
// PHP 8 的规则：
//   - 字符串是数值串 → 按数值比较（"1.5" == 1 为 false）
//   - 字符串不是数值串 → 把整数转成字符串再比较（"1abc" == 1 为 false）
func PHPLooseEqualsInt(raw string, n int) bool {
	if v, ok := PHPNumericValue(raw); ok {
		return v == float64(n)
	}
	return raw == strconv.Itoa(n)
}

// PHPIntArg 模拟把一个请求参数传给 PHP 的 int 形参。
//
// PHP 8 的规则：
//   - 键不存在（值为 null）→ TypeError
//   - 数值串 → 转换为整数（向零截断）
//   - 非数值串（含空串）→ TypeError
//
// ok 为 false 表示 PHP 会抛 TypeError —— 那是一个致命错误，
// 表现为 HTTP 500 + 空响应体。
func PHPIntArg(raw string, present bool) (int, bool) {
	if !present {
		return 0, false
	}
	v, ok := PHPNumericValue(raw)
	if !ok {
		return 0, false
	}
	return int(v), true
}

// PHPStringArg 模拟把一个请求参数传给 PHP 的 string 形参。
//
// PHP 8 里 null 不能传给非空的 string 形参（TypeError），
// 其余标量都可以转换。
func PHPStringArg(raw string, present bool) (string, bool) {
	if !present {
		return "", false
	}
	return raw, true
}

// GroupTypeAllowed 判断组类型参数是否被 PHP 接受。
//
// 逐字对应 application/controllers/api/group.php:48 的
// `in_array($data['type'], [GROUP_TYPE_NORMAL, GROUP_TYPE_USER])`
// （in_array 默认是宽松比较）。
func GroupTypeAllowed(raw string, present bool) bool {
	if !present {
		return false
	}
	return PHPLooseEqualsInt(raw, GroupTypeNormal) || PHPLooseEqualsInt(raw, GroupTypeUser)
}

// GroupActivityCategoryAllowed 判断动态分类参数是否被 PHP 接受。
//
// 逐字对应 application/controllers/api/group.php:174 的
// `($category && !in_array($category, [ACTIVITY_CATEGORY_COMMIT,
//
//	ACTIVITY_CATEGORY_MERGE_REQUEST, ACTIVITY_CATEGORY_MEMBER]))`：
//	 - 空串与 "0" 在 PHP 里是假值 → 不触发 0x0201
//	 - in_array 是宽松比较：数值串 "1" 与整数 1 相等
//	 - 非数值串与整数永不相等（PHP 8 语义）
func GroupActivityCategoryAllowed(raw string, present bool) bool {
	if !present || !PHPTruthy(raw) {
		return true
	}
	return PHPLooseEqualsInt(raw, ActivityCategoryCommit) ||
		PHPLooseEqualsInt(raw, ActivityCategoryMergeRequest) ||
		PHPLooseEqualsInt(raw, ActivityCategoryMember)
}

// GroupMemberRoleAllowed 判断成员角色参数是否被 PHP 接受。
//
// 逐字对应 application/controllers/api/group.php:324-330 的
// `in_array($data['roleID'], [ROLE_GUEST, ROLE_REPORTER, ROLE_DEVELOPER,
//
//	ROLE_MAINTAINER, ROLE_OWNER])`（in_array 默认是宽松比较）。
func GroupMemberRoleAllowed(raw string, present bool) bool {
	if !present {
		return false
	}
	for _, r := range []int{
		int(RoleGuest), int(RoleReporter), int(RoleDeveloper),
		int(RoleMaintainer), int(RoleOwner),
	} {
		if PHPLooseEqualsInt(raw, r) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 保留 URI
// ---------------------------------------------------------------------------

// preserveURIPatterns 逐字搬运 application/config/constants.php 的 PRESERVE_URI。
//
// PHP 侧用 preg_match 且带 i 修饰符（大小写不敏感）。
var preserveURIPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(feature|doc|community|lang|service|about|pricing|user|users|file|avatar|boss|captcha)$`),
	regexp.MustCompile(`(?i)^(static|static-\d+)$`),
	regexp.MustCompile(`(?i)^(api|apiv\d+|apis)$`),
	regexp.MustCompile(`(?i)^(userInfo|userInfos)$`),
	regexp.MustCompile(`(?i)^(pay|payment|transaction|order|coupon|invoice)$`),
	regexp.MustCompile(`(?i)^(group|groups|repository|repositories|setting|settings|mergerequest|mergerequests|admin)$`),
}

// GroupNameIsPreserved 判断名称是否命中保留 URI 段。
//
// 逐字对应 application/models/group_model.php:334-342 的 checkPreserve()。
func GroupNameIsPreserved(input string) bool {
	for _, re := range preserveURIPatterns {
		if re.MatchString(input) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 查询条件与分页
// ---------------------------------------------------------------------------

// GroupActivityCategoryWhere 返回动态分类对应的 SQL 条件。
//
// 逐字对应 application/models/group_model.php:226-241 的 switch：
//
//	COMMIT        → a_type IN (0x0401, 0x0402, 0x0501, 0x0502)
//	MERGE_REQUEST → a_type > 0x0700 AND a_type < 0x0800
//	MEMBER        → a_type < 0x0600 且排除上面四种类型
//
// 分类为 0（全部）时不加条件。
func GroupActivityCategoryWhere(category int) (string, []any) {
	switch category {
	case ActivityCategoryCommit:
		return "a_type IN (?, ?, ?, ?)", []any{0x0401, 0x0402, 0x0501, 0x0502}
	case ActivityCategoryMergeRequest:
		return "(a_type > ? AND a_type < ?)", []any{0x0700, 0x0800}
	case ActivityCategoryMember:
		return "(a_type < ? AND a_type NOT IN (?, ?, ?, ?))", []any{0x0600, 0x0401, 0x0402, 0x0501, 0x0502}
	default:
		return "", nil
	}
}

// GroupPagingOffset 计算 SQL LIMIT 的偏移量。
//
// 逐字对应 PHP 侧的 ($page - 1) * $pagesize。
//
// 注意：contract 包把 query 里的 page/pagesize 归一化成 1/20
// （复刻 PHP 的 is_int 判定失效），因此实际偏移量恒为 0。
func GroupPagingOffset(page, pagesize int) int {
	return (page - 1) * pagesize
}
