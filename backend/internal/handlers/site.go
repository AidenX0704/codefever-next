package handlers

import (
	"context"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// SiteHandler 实现 /api/site/* 的公开端点（匿名可访问）。
//
// 站点统计、更新日志与公告是落地页 / 帮助中心需要的公开信息，
// 未登录也必须能读 —— 这是本仓库第一批 AuthPublic 级别的 /api/* 端点
// （PHP 侧所有 /api/* 都要登录，无对应实现可比对）。
//
// 性能约定：全部输出来自 DB count / 单表查询，**不扫 git** ——
// 匿名端点没有登录态保护，任何一行慢查询都会被无限制地刷。
type SiteHandler struct {
	Site *repo.SiteRepo
}

// NewSiteHandler 创建站点公开端点处理器。
func NewSiteHandler(site *repo.SiteRepo) *SiteHandler {
	return &SiteHandler{Site: site}
}

// Register 把站点公开端点注册到服务器。
func (h *SiteHandler) Register(s *httpapi.Server) {
	actions := map[string]map[string]httpapi.ActionHandler{
		"stats":        {"GET": h.stats},
		"changelog":    {"GET": h.changelog},
		"announcement": {"GET": h.announcement},
	}

	s.Register(&httpapi.Controller{
		Name:    "site",
		Auth:    httpapi.AuthPublic,
		Actions: actions,
	})
}

// stats 实现 GET /api/site/stats。
//
// 输出：{repositoryCount, groupCount, memberCount, mergedMergeRequestCount}。
func (h *SiteHandler) stats(ctx context.Context, req *httpapi.Request) (any, error) {
	if h.Site == nil {
		// 未注入仓储时按"动作不存在"处理，与 registerTokenActions 的取舍一致：
		// 比路由存在但必然 500 更容易排查。
		return nil, contract.NewError(contract.CodeEndpointNotFound)
	}
	stats, err := h.Site.Stats(ctx)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// changelog 实现 GET /api/site/changelog。
//
// query：page / pagesize（经 domain.ClampPaging 收敛，上限 100 —— 匿名端点
// 必须防住 `?pagesize=20000` 这类无上限 LIMIT）。
// 输出：{total, list}，list 按 released_at desc, cl_id desc 排序。
func (h *SiteHandler) changelog(ctx context.Context, req *httpapi.Request) (any, error) {
	if h.Site == nil {
		return nil, contract.NewError(contract.CodeEndpointNotFound)
	}
	// query 里的分页值恒为字符串（PHP 的 is_int 判定必然失败），
	// QueryInt 会把 "20" 解析回 20，再统一走 ClampPaging 收敛。
	page, pagesize := domain.ClampPaging(req.Parsed.QueryInt("page"), req.Parsed.QueryInt("pagesize"))

	list, total, err := h.Site.ListChangelogs(ctx, page, pagesize)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ChangelogView, 0, len(list))
	for i := range list {
		out = append(out, domain.NormalizeChangelog(&list[i]))
	}
	return map[string]any{
		"total":    total,
		"page":     page,
		"pagesize": pagesize,
		"list":     out,
	}, nil
}

// announcement 实现 GET /api/site/announcement。
//
// 仅返回启用的公告（an_enabled = 1），按 an_id desc 最多 10 条 ——
// 公告是横幅位内容，多于 10 条前端也展示不下。
func (h *SiteHandler) announcement(ctx context.Context, req *httpapi.Request) (any, error) {
	if h.Site == nil {
		return nil, contract.NewError(contract.CodeEndpointNotFound)
	}
	list, err := h.Site.ListAnnouncements(ctx, siteAnnouncementLimit)
	if err != nil {
		return nil, err
	}
	out := make([]domain.AnnouncementView, 0, len(list))
	for i := range list {
		out = append(out, domain.NormalizeAnnouncement(&list[i]))
	}
	return out, nil
}

// siteAnnouncementLimit 是公开公告端点的最大返回条数。
const siteAnnouncementLimit = 10
