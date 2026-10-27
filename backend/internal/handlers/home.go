package handlers

import (
	"context"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// 首页聚合端点的取数上限。
//
// 全部是**编译期常量**，不接受客户端传参 —— 首页不是列表页，没有分页语义，
// 也就不存在 AGENTS §9 那条"pagesize 必须 ClampPaging"的风险面。
const (
	homeReviewRequestLimit  = 5
	homeMyMergeRequestLimit = 5
	homeActivityLimit       = 10
	// 取数时多取一些：越权过滤（角色 ≤ guest）会丢掉部分行，
	// 若只取 limit 条，过滤后常常凑不满一屏。
	homeFetchFactor = 2
)

// HomeAPIHandler 实现 `/api/home/*`。
//
// **与 web 控制器 `home` 不是一回事**：`handlers/web.go` 里的 `HomeHandler` 是
// CodeIgniter 时代的页面控制器（`/` 的 302 落地），本类型是 JSON API 控制器
// （`/api/home/overview`）。两者注册在不同的 map（`s.controllers` vs
// `s.webControllers`），互不影响 —— 后来者请勿把两者合并。
type HomeAPIHandler struct {
	Home *repo.HomeRepo
	// Repos 用于权限判定：仓库可见性含**组继承**，只有 `RolesFor` 能算对，
	// SQL 里判断不了（这也是 publicapi 的 canReadRepo 走同一条路径的原因）。
	Repos *repo.RepositoryRepo
}

// NewHomeAPIHandler 创建首页聚合处理器。
func NewHomeAPIHandler(home *repo.HomeRepo, repos *repo.RepositoryRepo) *HomeAPIHandler {
	return &HomeAPIHandler{Home: home, Repos: repos}
}

// Register 把 `/api/home/*` 注册到服务器。
func (h *HomeAPIHandler) Register(s *httpapi.Server) {
	s.Register(&httpapi.Controller{
		Name: "home",
		Auth: httpapi.AuthWebUser,
		Actions: map[string]map[string]httpapi.ActionHandler{
			"overview": {"GET": h.overview},
		},
	})
}

// overview 实现 GET /api/home/overview。
//
// 首页首屏的**唯一**聚合请求：待我评审 / 我发起的 / 团队动态 / 平台概览。
// 三个列表都做了可见性过滤，越权数据**不返回**（不是前端过滤）。不可见时
// 直接消失比返回空数组更好：后者会让人以为"有 0 条"。
func (h *HomeAPIHandler) overview(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}

	pending, err := h.Home.PendingReviews(ctx, uKey, homeReviewRequestLimit*homeFetchFactor)
	if err != nil {
		return nil, err
	}
	pending = h.filterVisible(ctx, uKey, pending)
	if len(pending) > homeReviewRequestLimit {
		pending = pending[:homeReviewRequestLimit]
	}

	mine, err := h.Home.MyOpenMergeRequests(ctx, uKey, homeMyMergeRequestLimit*homeFetchFactor)
	if err != nil {
		return nil, err
	}
	mine = h.filterVisible(ctx, uKey, mine)
	if len(mine) > homeMyMergeRequestLimit {
		mine = mine[:homeMyMergeRequestLimit]
	}

	reviewCount, myCount, err := h.Home.TodoCounts(ctx, uKey)
	if err != nil {
		return nil, err
	}

	activities, err := h.Home.RecentActivities(ctx, uKey, homeActivityLimit)
	if err != nil {
		return nil, err
	}

	platform, err := h.Home.PlatformSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	activityViews := make([]domain.RepositoryActivityView, 0, len(activities))
	for i := range activities {
		activityViews = append(activityViews, activityView(&activities[i]))
	}

	return domain.HomeOverview{
		Counts: domain.HomeCounts{
			ReviewRequests:      reviewCount,
			MyOpenMergeRequests: myCount,
		},
		ReviewRequests:   homeMergeRequestViews(pending),
		MyMergeRequests:  homeMergeRequestViews(mine),
		RecentActivities: activityViews,
		Platform:         platform,
	}, nil
}

// filterVisible 丢掉我没有读权限的合并请求（仓库角色 ≤ guest）。
//
// 判定看**目标仓库**：评审发生在目标仓库侧，源仓库是只读来源；只要目标仓库可见，
// 该 MR 就该出现在待办里。
func (h *HomeAPIHandler) filterVisible(ctx context.Context, uKey string, list []domain.RepositoryMergeRequest) []domain.RepositoryMergeRequest {
	if len(list) == 0 {
		return list
	}
	rKeys := make([]string, 0, len(list))
	for _, mr := range list {
		rKeys = append(rKeys, mr.TargetRKey)
	}
	roles, err := h.Repos.RolesFor(ctx, rKeys, uKey)
	if err != nil {
		// 权限查询失败时**宁可不展示**：首页多一条待办事小，泄露一个不可见仓库的
		// 标题事大。
		return []domain.RepositoryMergeRequest{}
	}
	return filterMergeRequestsByRole(list, roles)
}

// filterMergeRequestsByRole 是 filterVisible 的纯函数部分（可单测）。
//
// 拆出来的原因：`RolesFor` 要连数据库，而"角色 ≤ guest 就不返回"这条
// **安全语义**必须有无需数据库的回归用例（见 home_test.go）。
func filterMergeRequestsByRole(list []domain.RepositoryMergeRequest, roles map[string]int) []domain.RepositoryMergeRequest {
	out := make([]domain.RepositoryMergeRequest, 0, len(list))
	for _, mr := range list {
		if roles[mr.TargetRKey] > int(domain.RoleGuest) {
			out = append(out, mr)
		}
	}
	return out
}

// homeMergeRequestViews 把合并请求行转成前端行结构。
//
// 与 `repository/mergeRequests` 的差异（有意）：**不填 `commit`、不填 `reviewers`**。
// 填 `commit` 需要逐条 `git log`（列表里 5 条就是 5 次子进程），填 `reviewers`
// 需要额外一次批量查询 —— 首页是导航层，行内只需要"哪个仓库、谁提的、什么分支、
// 什么时候更新"，这些字段 mr 行本身就有。前端用 `MergeRequestRow` 的 compact
// 变体渲染，正是为了不展示这两块。
func homeMergeRequestViews(list []domain.RepositoryMergeRequest) []domain.RepositoryMergeRequestView {
	out := make([]domain.RepositoryMergeRequestView, 0, len(list))
	for _, mr := range list {
		out = append(out, domain.RepositoryMergeRequestView{
			ID:               mr.Key,
			SourceRepository: mr.SourceRKey,
			SourceBranch:     mr.SourceBranch,
			TargetRepository: mr.TargetRKey,
			TargetBranch:     mr.TargetBranch,
			Number:           mr.ID,
			Title:            mr.Title,
			Status:           mr.ProceedStatus,
			Create:           phpTimeOf(mr.Created),
			Update:           phpTimeOf(mr.Updated),
			Reviewers:        []any{},
		})
	}
	return out
}
