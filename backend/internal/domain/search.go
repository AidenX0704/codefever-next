package domain

// 全局搜索（阶段一）的输出结构。
//
// 阶段一的范围刻意收窄：**只搜"元数据"** —— 仓库名/显示名/描述、仓库组名/显示名/描述、
// 合并请求标题。**不搜代码内容**：那需要 `git grep` 或独立索引，成本高一个数量级，
// 而且必须做超时与限流（见 docs/HOME_PAGE_PLAN_2026-10-27.md 的 A2 两阶段划分）。
//
// 权限口径：三类结果都只返回**我可见**的对象 —— SQL 里直接 JOIN 成员表，
// 而不是取出结果再过滤（后者会在分页/截断上出错：过滤掉 5 条就只剩 5 条）。
type SearchResults struct {
	// Keyword 是实际生效的关键词（已 trim；回显给前端做高亮与"你搜的是…"）。
	Keyword string `json:"keyword"`
	// Repositories / Groups / MergeRequests 是三类结果，各自最多 searchBucketLimit 条。
	Repositories  []SearchRepository   `json:"repositories"`
	Groups        []SearchGroup        `json:"groups"`
	MergeRequests []SearchMergeRequest `json:"mergeRequests"`
	// Truncated 标记某一类"还有更多"，前端据此显示"仅显示前 N 条，请细化关键词"。
	Truncated SearchTruncated `json:"truncated"`
}

// SearchTruncated 标记各类结果是否被截断。
type SearchTruncated struct {
	Repositories  bool `json:"repositories"`
	Groups        bool `json:"groups"`
	MergeRequests bool `json:"mergeRequests"`
}

// SearchRepository 是一条仓库搜索结果。
//
// 带 `GroupName` / `GroupDisplayName` 是为了让前端能直接拼出 `/{组名}/{仓库名}`，
// 不必再去翻一遍全量仓库列表。
type SearchRepository struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	DisplayName      string  `json:"displayName"`
	Description      string  `json:"description"`
	Avatar           *string `json:"avatar"`
	GroupName        string  `json:"groupName"`
	GroupDisplayName string  `json:"groupDisplayName"`
	Updated          int64   `json:"updated"`
}

// SearchGroup 是一条仓库组搜索结果。
type SearchGroup struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	DisplayName     string  `json:"displayName"`
	Description     string  `json:"description"`
	Avatar          *string `json:"avatar"`
	RepositoryCount int     `json:"repositoryCount"`
	Updated         int64   `json:"updated"`
}

// SearchMergeRequest 是一条合并请求搜索结果。
//
// **字段刻意与 `RepositoryMergeRequestView`（即前端 `MergeRequestListItem`）同形**：
// 这样搜索结果页可以直接复用 `MergeRequestRow` 的 `compact` 变体，不必为搜索再造一个
// 行组件（AGENTS §4）。
//
// `Reviewers` / `Commit` 恒为空 —— 与首页一致的理由：填它们要逐条 `git log` +
// 额外批量查询，搜索是导航层，不展示这些明细。
type SearchMergeRequest struct {
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
	// Creator 是发起人主键（`mr.u_key`），前端按 AppData 里的成员字典解析成名字。
	Creator string `json:"creator"`
}
