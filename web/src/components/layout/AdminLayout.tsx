import { Outlet } from 'react-router'

/**
 * 管理后台外壳（`/admin/*`）。
 *
 * v5：管理子导航（用户 / 仓库 / 仓库组 / 公告 / 设置）进**平台侧栏「管理」分组**
 * （PlatformSidebar 按 user.admin 门控），本层不再渲染二级侧栏；
 * 各管理页自带 v5 页头（标题 + 计数）。
 */
export function AdminLayout() {
  return (
    <div className="page">
      <Outlet />
    </div>
  )
}
