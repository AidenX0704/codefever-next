package handlers

import (
	"context"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// 仓库标星。
//
// 读侧（repository/list / config 的 starCount / starred / mode=starred）
// 挂在 RepositoryHandler；写侧（star / unstar 两个 POST 端点）挂在
// RepositoryWriteHandler —— 与"只读端点在 handlers/repository.go、
// 写端点在 handlers/repository_write.go"的既有分工一致，两者最终
// 注册进同一个 "repository" 控制器。
//
// 权限约定：**能读到该仓库即可标星**（UACRepoRead，与 config 端点同款），
// 取消标星用同一权限 —— 已标星是前提，而标星本身就要求读过。

// WithRepositoryStars 给只读侧注入标星仓储。
//
// 未注入时 starCount 恒为 0、starred 恒为 false，mode=starred 返回空列表
// （测试与旧调用点不必凭空构造数据库）。
func WithRepositoryStars(stars *repo.StarRepo) RepositoryOption {
	return func(h *RepositoryHandler) { h.Stars = stars }
}

// WithRepositoryWriteStars 给写侧注入标星仓储。
func WithRepositoryWriteStars(stars *repo.StarRepo) RepositoryWriteOption {
	return func(h *RepositoryWriteHandler) { h.Stars = stars }
}

// registerStarActions 把标星端点挂到 "repository" 控制器。
//
// Stars 未注入时**不注册**（请求落 0x0105），与 registerTokenActions 一致。
func (h *RepositoryWriteHandler) registerStarActions(actions map[string]map[string]httpapi.ActionHandler) {
	if h.Stars == nil {
		return
	}
	actions["star"] = map[string]httpapi.ActionHandler{"POST": h.star}
	actions["unstar"] = map[string]httpapi.ActionHandler{"POST": h.unstar}
}

// star 实现 POST /api/repository/star。
//
// 入参：{repositoryId}。幂等：重复标星返回成功。
func (h *RepositoryWriteHandler) star(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	rKey, _ := phpBodyArg(req.Parsed, "repositoryId")
	if !domain.PHPTruthy(rKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	// 能读到才允许标星 —— 复用 config 端点的守卫语义。
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}
	if err := h.Stars.Star(ctx, uKey, rKey); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// unstar 实现 POST /api/repository/unstar。
//
// 入参：{repositoryId}。幂等：没标过星同样返回成功。
func (h *RepositoryWriteHandler) unstar(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	rKey, _ := phpBodyArg(req.Parsed, "repositoryId")
	if !domain.PHPTruthy(rKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}
	if err := h.Stars.Unstar(ctx, uKey, rKey); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// starView 是 list / config 的标星字段聚合结果。
type starView struct {
	count   map[string]int
	starred map[string]bool
}

// newStarView 一次性取回一批仓库的标星数与当前用户的标星状态。
//
// 两条 GROUP BY / IN 查询覆盖整批仓库，避免逐仓库 N+1；
// Stars 未注入时返回零值视图（list / config 的字段照常输出）。
func (h *RepositoryHandler) newStarView(ctx context.Context, uKey string, rKeys []string) starView {
	return starViewOf(ctx, h.Stars, uKey, rKeys)
}

// listStarredFilter 实现 list 的 mode=starred 筛选。
//
// 返回用户标星且当前可见的仓库子集，保持 ListJoined 的既有顺序 ——
// 与 PHP 侧"列表顺序 = 可见仓库的稳定顺序"的口径一致。
// Stars 未注入时该筛选退化为空列表。
func (h *RepositoryHandler) listStarredFilter(ctx context.Context, uKey string, repos []domain.Repository) []domain.Repository {
	if h.Stars == nil {
		return []domain.Repository{}
	}
	starredKeys, err := h.Stars.StarredRepositories(ctx, uKey)
	if err != nil {
		// 筛选条件读不出来时宁可返回空列表也不返回全量：
		// mode=starred 的语义是"只要我标过星的"。
		return []domain.Repository{}
	}
	set := make(map[string]bool, len(starredKeys))
	for _, key := range starredKeys {
		set[key] = true
	}
	out := make([]domain.Repository, 0, len(set))
	for _, r := range repos {
		if set[r.Key] {
			out = append(out, r)
		}
	}
	return out
}
