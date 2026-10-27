package domain

// HomeOverview 是 `GET /api/home/overview` 的输出（首页工作台的一次性聚合）。
//
// 为什么单独做一个聚合端点：首页要同时展示"待我评审""我发起的""团队动态"与
// "平台概览"四类数据，它们跨 user / repository / group 三个域。若各自复用现有端点：
//   - 待我评审：现有 `repository/relatedMergeRequests` 只能按仓库范围取全部 MR，
//     表达不了"指派给我且我还没评审"；
//   - 团队动态：`repository/activities` 是**单仓库**维度，首页要跨仓库 + 跨组；
//   - 平台概览：管理端才有统计，普通员工没有入口。
//
// 凑出首屏需要 4~5 个请求，且其中一个还要拉全量列表再前端过滤。一个聚合端点用
// 3~4 条带索引的 SQL 就够了（见 docs/HOME_PAGE_PLAN_2026-10-27.md §2.6）。
type HomeOverview struct {
	// Counts 是"我"的待办计数，用于 KPI 条。
	Counts HomeCounts `json:"counts"`
	// ReviewRequests 是待我评审的合并请求（≤5 条，按更新时间倒序）。
	ReviewRequests []RepositoryMergeRequestView `json:"reviewRequests"`
	// MyMergeRequests 是我发起且未合并的合并请求（≤5 条）。
	MyMergeRequests []RepositoryMergeRequestView `json:"myMergeRequests"`
	// RecentActivities 是我可见范围内的团队动态（≤10 条）。
	RecentActivities []RepositoryActivityView `json:"recentActivities"`
	// Platform 是平台概览（全站口径，供门户区展示）。
	Platform HomePlatform `json:"platform"`
}

// HomeCounts 是首页 KPI 条里的待办计数。
//
// 注意：`我的仓库` / `我参与的组` 两个数字**不在这里** —— 前端 `useAppData()` 已经
// 持有仓库与组清单，再算一遍纯属重复查询。
type HomeCounts struct {
	ReviewRequests      int `json:"reviewRequests"`
	MyOpenMergeRequests int `json:"myOpenMergeRequests"`
}

// HomePlatform 是平台概览（全站计数，一条 SQL 取回）。
type HomePlatform struct {
	Repositories      int `json:"repositories"`
	Groups            int `json:"groups"`
	Members           int `json:"members"`
	OpenMergeRequests int `json:"openMergeRequests"`
}
