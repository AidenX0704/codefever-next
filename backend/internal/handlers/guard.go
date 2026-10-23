package handlers

import (
	"context"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/repo"
)

// Guard 统一封装资源级权限校验。
//
// PHP 侧的做法是每个控制器里手写：
//
//	if (!$this->service->requestRepositoryPermission($rKey, $uKey, UserAccessController::UAC_REPO_READ)) {
//	    Response::reject(0x0106);
//	}
//
// 新后端把它收敛到一处，既避免遗漏（原实现有 11 处越权读取缺口），
// 也让"资源不存在"与"权限不足"返回同一个 0x0106 —— 与 PHP 实测一致。
type Guard struct {
	access *repo.AccessRepo
}

// NewGuard 创建权限守卫。
func NewGuard(access *repo.AccessRepo) *Guard { return &Guard{access: access} }

// RequireRepository 校验用户对仓库的权限。
//
// 失败一律返回 0x0106（Action Not Allowed / 403），
// 与 PHP 实测一致：不存在的仓库同样返回 0x0106，而不是 0x0302。
func (g *Guard) RequireRepository(ctx context.Context, rKey, uKey string, perm domain.Permission) error {
	if g == nil || g.access == nil {
		return contract.NewError(contract.CodeActionNotAllowed)
	}
	role, err := g.access.RepositoryRole(ctx, rKey, uKey)
	if err != nil {
		return err
	}
	if !domain.CheckPermission(perm, role) {
		return contract.NewError(contract.CodeActionNotAllowed)
	}
	return nil
}

// RequireGroup 校验用户对仓库组的权限。
//
// 失败一律返回 0x0106（403），与 PHP 实测一致。
func (g *Guard) RequireGroup(ctx context.Context, gKey, uKey string, perm domain.Permission) error {
	if g == nil || g.access == nil {
		return contract.NewError(contract.CodeActionNotAllowed)
	}
	role, err := g.access.GroupRole(ctx, gKey, uKey)
	if err != nil {
		return err
	}
	if !domain.CheckPermission(perm, role) {
		return contract.NewError(contract.CodeActionNotAllowed)
	}
	return nil
}

// RepositoryRole 暴露角色解析，供需要区分"无权限"与"无资源"的端点使用。
func (g *Guard) RepositoryRole(ctx context.Context, rKey, uKey string) (domain.Role, error) {
	return g.access.RepositoryRole(ctx, rKey, uKey)
}

// GroupRole 暴露角色解析。
func (g *Guard) GroupRole(ctx context.Context, gKey, uKey string) (domain.Role, error) {
	return g.access.GroupRole(ctx, gKey, uKey)
}
