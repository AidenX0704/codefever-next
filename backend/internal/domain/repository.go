package domain

import (
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// 表行
//
// 时间列一律保留 MySQL 返回的原始字符串，而不是 time.Time。
// 原因：PHP 用 strtotime() 解析这些值，"0000-00-00 00:00:00" 会得到
// -62169984000，而 Go 的 time.Time 零值（0001-01-01）会得到 -62135596800。
// 契约要求逐字一致，因此必须保留原始字符串再自行转换。
// ---------------------------------------------------------------------------

// Repository 是 cc_repositories 表的一行。
//
// 可空列用 *string 表示 —— PHP 的 row_array() 对 NULL 返回 NULL，
// JSON 编码后是 null 而不是空字符串（契约要求逐字一致）。
type Repository struct {
	Key           string
	OwnerKey      string
	GroupKey      string
	ForkFromKey   *string
	DisplayName   string
	Name          string
	Path          string
	Description   string
	Avatar        *string
	DefaultBranch *string
	Created       string
	Updated       string
	Status        int
}

// RepositoryGroup 是 cc_groups 表的一行（仓库视角）。
//
// 注意：domain 包里的 group.go 属于仓库组域，那边已有 Group / GroupMember /
// NormalizedGroup 等类型；本文件是仓库域，类型一律加 Repository 前缀以免冲突。
type RepositoryGroup struct {
	Key         string
	OwnerKey    string
	Type        int
	DisplayName string
	Name        string
	Description string
	Avatar      *string
	Created     string
	Updated     string
	Status      int
}

// RepositoryMember 是 cc_repository_members 表的一行。
type RepositoryMember struct {
	Key     string
	RKey    string
	UKey    string
	Role    int
	Created string
	Status  int
}

// RepositoryGroupMember 是 cc_group_members 表的一行。
type RepositoryGroupMember struct {
	Key     string
	GKey    string
	UKey    string
	Role    int
	Created string
	Status  int
}

// RepositoryActivity 是 cc_activities 表的一行。
type RepositoryActivity struct {
	Key        string
	Type       int
	UKey       string
	GroupKey   *string
	Repository *string
	MergeReq   *string
	Data       *string
	Created    string
	Status     int
}

// RepositoryMergeRequest 是 cc_merge_requests 表的一行。
type RepositoryMergeRequest struct {
	Key           string
	UKey          string
	MergeUKey     *string
	SourceHash    *string
	SourceRKey    string
	SourceBranch  string
	TargetHash    *string
	TargetRKey    string
	TargetBranch  string
	StartHash     string
	ID            int
	Title         string
	Description   *string
	ProceedStatus int
	Created       string
	MergeTime     string
	CloseTime     string
	Updated       string
	Status        int
}

// RepositoryMergeRequestReviewer 是 cc_merge_request_reviewers 表的一行。
type RepositoryMergeRequestReviewer struct {
	Key      string
	MRKey    string
	UKey     string
	IsReview bool
}

// ProtectedBranchRule 是 cc_proteced_branch_rules 表的一行。
type ProtectedBranchRule struct {
	Key       string
	RKey      string
	Rule      string
	PushRole  int
	MergeRole int
	Updated   string
	Status    int
}

// phpUnixTime 把 MySQL 的时间字符串转换为 PHP strtotime() 的结果。
//
// PHP 侧的 date_default_timezone 是 UTC，MySQL 服务器的 system time_zone
// 也是 UTC，因此这里按 UTC 解析；零值日期单独返回 PHP 的取值。
//
// 与 group.go 里的 PHPUnixTime 是同一套语义，但这里保留一份**未导出**的副本，
// 使仓库域不依赖仓库组域的实现细节（两者由不同任务并行实现）。
func phpUnixTime(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "0000-00-00") {
		return -62169984000
	}
	// MySQL 的零值日期在驱动里会变成 Go 的零值时间（0001-01-01）。
	if strings.HasPrefix(raw, "0001-01-01") {
		return -62169984000
	}
	// MySQL 驱动可能带上小数秒（".000000"），这里截掉。
	if i := strings.IndexByte(raw, '.'); i >= 0 {
		raw = raw[:i]
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.UTC)
	if err != nil {
		return -62169984000
	}
	return t.Unix()
}

// ---------------------------------------------------------------------------
// 归一化输出
// ---------------------------------------------------------------------------

// NormalizedRepositoryGroup 是仓库组在仓库域里的归一化输出。
//
// 逐字对应 application/models/group_model.php:18-36 的 normalize()。
type NormalizedRepositoryGroup struct {
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

// NormalizeRepositoryGroup 把仓库组行转换为对外形态。
func NormalizeRepositoryGroup(g *RepositoryGroup) NormalizedRepositoryGroup {
	if g == nil {
		return NormalizedRepositoryGroup{}
	}
	return NormalizedRepositoryGroup{
		ID:          g.Key,
		Icon:        g.Avatar,
		DisplayName: g.DisplayName,
		Name:        g.Name,
		Description: g.Description,
		Owner:       g.OwnerKey,
		Type:        g.Type,
		Created:     phpUnixTime(g.Created),
		Updated:     phpUnixTime(g.Updated),
		Status:      g.Status,
	}
}

// RepositoryMergeRequestCount 是合并请求的计数视图。
//
// 逐字对应 application/models/repository_model.php:586-617 的 countMergeRequests()。
type RepositoryMergeRequestCount struct {
	Open   int `json:"open"`
	Merged int `json:"merged"`
	Closed int `json:"closed"`
	All    int `json:"all"`
}

// NormalizedRepository 是仓库的归一化输出。
//
// 逐字对应 application/models/repository_model.php:24-49 的 normalize()。
// 注意这里**不含** role / members —— PHP 侧只有 repository/list 会在
// normalize() 之后再补这两个字段（见 RepositoryListItem）。
type NormalizedRepository struct {
	ID                string                      `json:"id"`
	Icon              *string                     `json:"icon"`
	DisplayName       string                      `json:"displayName"`
	Name              string                      `json:"name"`
	Description       string                      `json:"description"`
	Owner             string                      `json:"owner"`
	ForkFrom          *string                     `json:"forkFrom"`
	Created           int64                       `json:"created"`
	Updated           int64                       `json:"updated"`
	Group             NormalizedRepositoryGroup   `json:"group"`
	ForkCount         int                         `json:"forkCount"`
	MergeRequestCount RepositoryMergeRequestCount `json:"mergeRequestCount"`
	DefaultBranch     *string                     `json:"defaultBranch"`
	Status            int                         `json:"status"`
}

// NormalizeRepository 把仓库行转换为对外形态。
//
// group 为该仓库所属的仓库组（PHP 侧是 groupModel->get() 查出来的行）；
// forkCount / mrCount 由调用方批量统计后传入。
func NormalizeRepository(r *Repository, group *RepositoryGroup, forkCount int, mrCount RepositoryMergeRequestCount) NormalizedRepository {
	return NormalizedRepository{
		ID:                r.Key,
		Icon:              r.Avatar,
		DisplayName:       r.DisplayName,
		Name:              r.Name,
		Description:       r.Description,
		Owner:             r.OwnerKey,
		ForkFrom:          r.ForkFromKey,
		Created:           phpUnixTime(r.Created),
		Updated:           phpUnixTime(r.Updated),
		Group:             NormalizeRepositoryGroup(group),
		ForkCount:         forkCount,
		MergeRequestCount: mrCount,
		DefaultBranch:     r.DefaultBranch,
		Status:            r.Status,
	}
}

// RepositoryListItem 是 repository/list 的元素。
//
// PHP 侧在 normalize() 之后追加 role 与 members 两个字段
// （application/controllers/api/repository.php:155-158）。
type RepositoryListItem struct {
	NormalizedRepository
	Role    int                          `json:"role"`
	Members []NormalizedRepositoryMember `json:"members"`
}

// NormalizedRepositoryMember 是仓库成员的归一化输出。
//
// 逐字对应 application/models/user_model.php:18-43 的 normalize()
// 加上 repository_model.php:51-83 的 normalizeMembers()/normalizeCreator() 追加的字段。
//
// 注意 role 在成员语境下被 rm_role / ROLE_OWNER **覆盖**成整数
// （不再是用户的 u_role 字符串），因此这里的类型是 int 而非 *string。
type NormalizedRepositoryMember struct {
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

// PhpTime 把 MySQL 的时间字符串转换为 PHP strtotime() 的结果。
//
// 供其它包（handlers）使用；语义与 phpUnixTime 一致。
func PhpTime(raw string) int64 { return phpUnixTime(raw) }

// repositoryMemberEmails 把 NormalizeUser 的提交邮箱形态转成字符串数组。
//
// NormalizeUser 的 Emails 是 []any（isCheck=FALSE 的对象形态），
// 而成员语境下 PHP 用的是 isCheck=TRUE 的字符串数组形态
// （application/models/user_model.php:35）。
func repositoryMemberEmails(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// NormalizeRepositoryMember 把仓库成员行转换为对外形态。
//
// 逐字对应 application/models/repository_model.php:51-67 的 normalizeMembers()：
// 先取用户归一化形态，再覆盖 role（仓库内角色）与 joined（加入时间），
// 并补上 groupMember / creatorFlag / deleteFlag 三个成员标记。
//
// user 的提交邮箱以"已验证邮箱字符串数组"形态给出 —— PHP 侧 normalize()
// 的 $extra 默认为 FALSE，因此 getCommitEmails() 的 $isCheck 为 TRUE
// （application/models/user_model.php:35）。
func NormalizeRepositoryMember(m *RepositoryMember, u *User, emails []CommitEmail, host, ssh string) NormalizedRepositoryMember {
	v := NormalizeUser(u, emails, false, 0, host, ssh)
	return NormalizedRepositoryMember{
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
		Emails:             repositoryMemberEmails(v.Emails),
		UnReadNotification: v.UnReadNotification,
		Status:             v.Status,
		Host:               v.Host,
		SSH:                v.SSH,
		Joined:             phpUnixTime(m.Created),
		GroupMember:        false,
		CreatorFlag:        false,
		DeleteFlag:         m.Status != CommonStatusNormal,
	}
}

// NormalizeRepositoryCreator 把仓库创建者转换为成员形态。
//
// 逐字对应 application/models/repository_model.php:69-83 的 normalizeCreator()：
// 角色恒为 ROLE_OWNER，joined 取仓库的创建时间，creatorFlag 为真。
func NormalizeRepositoryCreator(r *Repository, u *User, emails []CommitEmail, host, ssh string) NormalizedRepositoryMember {
	v := NormalizeUser(u, emails, false, 0, host, ssh)
	return NormalizedRepositoryMember{
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
		Emails:             repositoryMemberEmails(v.Emails),
		UnReadNotification: v.UnReadNotification,
		Status:             v.Status,
		Host:               v.Host,
		SSH:                v.SSH,
		Joined:             phpUnixTime(r.Created),
		GroupMember:        false,
		CreatorFlag:        true,
		DeleteFlag:         false,
	}
}

// NormalizeRepositoryGroupMember 把仓库组成员转换为成员形态。
//
// 逐字对应 application/models/group_model.php:38-54 的 normalizeMembers()：
// 与仓库成员的区别是 groupMember 恒为 TRUE。
func NormalizeRepositoryGroupMember(m *RepositoryGroupMember, u *User, emails []CommitEmail, host, ssh string) NormalizedRepositoryMember {
	v := NormalizeUser(u, emails, false, 0, host, ssh)
	return NormalizedRepositoryMember{
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
		Emails:             repositoryMemberEmails(v.Emails),
		UnReadNotification: v.UnReadNotification,
		Status:             v.Status,
		Host:               v.Host,
		SSH:                v.SSH,
		Joined:             phpUnixTime(m.Created),
		GroupMember:        true,
		CreatorFlag:        false,
		DeleteFlag:         m.Status != CommonStatusNormal,
	}
}

// NormalizeRepositoryGroupCreator 把仓库组创建者转换为成员形态。
//
// 逐字对应 application/models/group_model.php:56-70 的 normalizeCreator()。
func NormalizeRepositoryGroupCreator(g *RepositoryGroup, u *User, emails []CommitEmail, host, ssh string) NormalizedRepositoryMember {
	v := NormalizeUser(u, emails, false, 0, host, ssh)
	return NormalizedRepositoryMember{
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
		Emails:             repositoryMemberEmails(v.Emails),
		UnReadNotification: v.UnReadNotification,
		Status:             v.Status,
		Host:               v.Host,
		SSH:                v.SSH,
		Joined:             phpUnixTime(g.Created),
		GroupMember:        true,
		CreatorFlag:        true,
		DeleteFlag:         false,
	}
}

// NormalizedBranchRef 是分支/标签的引用形态（只有 id 与 name）。
//
// 逐字对应 repository_model.php:85-96 的 normalizeBranches()
// 与 repository_model.php:113-124 的 normalizeTags()。
type NormalizedBranchRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Commit 是一次提交的摘要信息。
//
// 逐字对应 application/models/repository_model.php:145-159 的 normalizeCommits()。
type Commit struct {
	SHA    string `json:"sha"`
	Commit string `json:"commit"`
	Detail string `json:"detail"`
	Email  string `json:"email"`
	Time   int64  `json:"time"`
}

// CommitDetail 是一次提交的详情。
//
// 逐字对应 application/models/repository_model.php:161-171 的 normalizeCommitDetail()。
// parent 是空格分隔的多个父提交 SHA。
type CommitDetail struct {
	SHA    string `json:"sha"`
	Commit string `json:"commit"`
	Parent string `json:"parent"`
	Email  string `json:"email"`
	Time   int64  `json:"time"`
}

// Branch 是 branchList 的元素。
//
// PHP 侧 normalizeBranches() 先给出 id/name，
// 再由 getBranchListInfo() 补 commit 与 isMerge、isProtected() 补 protected。
type Branch struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Commit    Commit `json:"commit"`
	IsMerge   bool   `json:"isMerge"`
	Protected bool   `json:"protected"`
}

// Tag 是 tagList 的元素。
//
// detail 与 commit 只有在 git show 能解析出内容时才存在
// （repository_model.php:1790-1821 的 getTagDetail()），
// 因此用指针 + omitempty 复刻 PHP 的"键可能不存在"语义。
type Tag struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Detail *string `json:"detail,omitempty"`
	Commit *Commit `json:"commit,omitempty"`
}

// ObjectEntry 是 tree 对象列表中的一项。
//
// 逐字对应 repository_model.php:1403-1407 的 Helper::parseTable()
// 解析出的 mode/type/object/name 四元组，
// 再由 addLastCommitToObjectList() 追加 commit、由 object_get() 追加 url。
type ObjectEntry struct {
	Mode   string  `json:"mode"`
	Type   string  `json:"type"`
	Object string  `json:"object"`
	Name   string  `json:"name"`
	Commit *Commit `json:"commit,omitempty"`
	URL    string  `json:"url"`
}

// PathStackEntry 是 pathStack 的元素（只有 tree 解析出的四个字段）。
type PathStackEntry struct {
	Mode   string `json:"mode"`
	Type   string `json:"type"`
	Object string `json:"object"`
	Name   string `json:"name"`
}

// ObjectView 是 object 端点的输出。
//
// PHP 侧 $output['type'] / ['object'] 恒存在，
// ['commit'] 与 ['path'] 只在给了 revision 时才追加
// （application/controllers/api/repository.php:1084-1094）。
type ObjectView struct {
	Type   string  `json:"type"`
	Object any     `json:"object"`
	Commit *Commit `json:"commit,omitempty"`
	Path   *string `json:"path,omitempty"`
}

// BlameBlock 是 blame 的一段归属区间。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:225-274
// 的 parseBlameData()。
type BlameBlock struct {
	Start  int    `json:"start"`
	Length int    `json:"length"`
	Commit Commit `json:"commit"`
}

// BlameResult 是 blameInfo 的输出。
type BlameResult struct {
	Blame []BlameBlock `json:"blame"`
	Lines []string     `json:"lines"`
}

// DiffSide 是 diff 中的一侧文件信息。
type DiffSide struct {
	Name *string `json:"name"`
	Mode *string `json:"mode"`
	SHA  *string `json:"sha"`
}

// DiffBody 是 diff 的正文统计。
//
// 注意 additions/deletions 复刻了 PHP 侧 `deletions--` 的写法 ——
// 删除行会让 deletions 变成**负数**，这是原实现的行为。
type DiffBody struct {
	Raw       *string  `json:"raw"`
	Line      []string `json:"line"`
	Additions int      `json:"additions"`
	Deletions int      `json:"deletions"`
}

// DiffFile 是 fileChanges 的一个文件条目。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:137-223
// 的 parseDiffString()。
type DiffFile struct {
	Original DiffSide `json:"original"`
	Modified DiffSide `json:"modified"`
	Diff     DiffBody `json:"diff"`
	Type     string   `json:"type"`
}

// FileChanges 是 fileChanges 的输出。
type FileChanges struct {
	Count  int        `json:"count"`
	Detail []DiffFile `json:"detail"`
}

// FileChangesLarge 是 fileChanges 命中超大阈值时的输出。
//
// application/controllers/api/repository.php:926-932。
type FileChangesLarge struct {
	Large  bool   `json:"large"`
	Count  int    `json:"count"`
	Detail string `json:"detail"`
}

// RepositoryActivityView 是 activities 的元素。
//
// 逐字对应 application/models/repository_model.php:126-143 的 normalizeActivities()。
type RepositoryActivityView struct {
	ID         string  `json:"id"`
	Type       int     `json:"type"`
	Creator    string  `json:"creator"`
	Group      *string `json:"group"`
	Repository *string `json:"repository"`
	Content    any     `json:"content"`
	Time       int64   `json:"time"`
}

// RepositoryMergeRequestView 是 mergeRequests 的元素。
//
// 逐字对应 application/models/repository_model.php:173-194 的 normalizeMergeRequests()。
type RepositoryMergeRequestView struct {
	ID               string `json:"id"`
	SourceRepository string `json:"sourceRepository"`
	SourceBranch     string `json:"sourceBranch"`
	TargetRepository string `json:"targetRepository"`
	TargetBranch     string `json:"targetBranch"`
	Number           int    `json:"number"`
	Title            string `json:"title"`
	Status           int    `json:"status"`
	Create           int64  `json:"create"`
	Update           int64  `json:"update"`
	Reviewers        []any  `json:"reviewers"`
	Commit           Commit `json:"commit"`
}

// RepositorySourceInfo 是合并请求详情里的源仓库信息。
//
// PHP 侧 normalizeMergeRequest() 在 normalize() 的结果上再补 members
// （application/models/repository_model.php:202）。
type RepositorySourceInfo struct {
	NormalizedRepository
	Members []NormalizedRepositoryMember `json:"members"`
}

// RepositoryMergeRequestDetail 是 mergeRequestDetail 的输出。
//
// 逐字对应 application/models/repository_model.php:196-227 的 normalizeMergeRequest()
// 再由控制器追加 email 字段。
type RepositoryMergeRequestDetail struct {
	ID               string               `json:"id"`
	User             string               `json:"user"`
	SourceRepository string               `json:"sourceRepository"`
	SourceBranch     string               `json:"sourceBranch"`
	SourceObject     *string              `json:"sourceObject"`
	TargetRepository string               `json:"targetRepository"`
	TargetBranch     string               `json:"targetBranch"`
	TargetObject     *string              `json:"targetObject"`
	Submitter        NormalizedUser       `json:"submitter"`
	SourceRepoInfo   RepositorySourceInfo `json:"sourceRepoInfo"`
	Title            string               `json:"title"`
	Description      *string              `json:"description"`
	Number           int                  `json:"number"`
	Handler          *string              `json:"handler"`
	Status           int                  `json:"status"`
	Create           int64                `json:"create"`
	Update           int64                `json:"update"`
	Close            int64                `json:"close"`
	Merge            int64                `json:"merge"`
	Reviewers        []any                `json:"reviewers"`
	Email            *string              `json:"email,omitempty"`
}

// RelatedMergeRequests 是 relatedMergeRequests 的输出。
type RelatedMergeRequests struct {
	RelatedRepository []string                     `json:"relatedRepository"`
	MergeRequests     []RepositoryMergeRequestView `json:"mergeRequests"`
}

// RepositoryMergeRequestVersionList 是 mergeRequestVersionList 的输出。
type RepositoryMergeRequestVersionList struct {
	MergeVersionList []Commit `json:"mergeVersionList"`
	BaseVersion      string   `json:"baseVersion"`
	BaseVersionList  []Commit `json:"baseVersionList"`
}

// TargetRepository 是 targetRepository 的元素。
type TargetRepository struct {
	RKey     string `json:"rKey"`
	Name     string `json:"name"`
	ForkFlag bool   `json:"forkFlag"`
}

// RefList 是 refListContainSHA 的输出。
type RefList struct {
	Branch []string `json:"branch"`
	Tag    []string `json:"tag"`
}

// RepositoryConfig 是 config 端点的输出。
type RepositoryConfig struct {
	Repository NormalizedRepository         `json:"repository"`
	Group      NormalizedRepositoryGroup    `json:"group"`
	Members    []NormalizedRepositoryMember `json:"members"`
	Branches   []NormalizedBranchRef        `json:"branches"`
	Tags       []NormalizedBranchRef        `json:"tags"`
	Count      RepositoryConfigCount        `json:"count"`
}

// RepositoryConfigCount 是 config.count 的内容。
type RepositoryConfigCount struct {
	Commit       int                         `json:"commit"`
	Branch       int                         `json:"branch"`
	Tag          int                         `json:"tag"`
	File         int                         `json:"file"`
	MergeRequest RepositoryMergeRequestCount `json:"mergeRequest"`
}
