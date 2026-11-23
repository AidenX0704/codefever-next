import { lazy, Suspense, type ComponentType, type ReactNode } from 'react'
import { createBrowserRouter, Navigate } from 'react-router'

import { AppShell } from '@/components/layout/AppShell'
import { RepositoryLayout } from '@/components/layout/RepositoryLayout'
import { SettingsLayout } from '@/components/layout/SettingsLayout'
import { GroupLayout } from '@/components/layout/GroupLayout'
import { AdminLayout } from '@/components/layout/AdminLayout'
import { PageLoading } from '@/components/unit/Loading'

/**
 * 路由表。
 *
 * 结构约定：
 *   1. 认证页（/user/*）是**独立全屏页**，不套 AppShell；
 *   2. 其余业务页都在 AppShell 里（Header/Drawer 常驻）；
 *   3. 所有页面 `lazy`，保持路由级代码分割（旧版性能整改 E4 的成果不能退化）；
 *   4. react-router 8 不支持路径内嵌正则：旧版 `:groupName([A-Za-z0-9_]{5,})`
 *      这类约束改为在页面内校验后渲染 404（见 FRONTEND_REWRITE_PLAN §8 偏离 #3）。
 *
 * 2026-11-22 减负：/home 展示首页、/:username 个人主页、/help、/search、
 * /__design、admin 公告与设置页、settings 邮箱页、forklist 上下文整批下线，
 * 落地页统一为 /repositories（见 CHANGELOG 同日条目）。
 *
 * 注：状态页（旧 success_message / disable_message / error_message 模板）在 Go 版里
 * 已无调用点（只有 doc 404 与 /lang 参数错误用到），因此不单独建路由，
 * 相关提示由对应页面内联呈现（见计划 §10 批 1 记录）。
 */
/**
 * 旧标签页兜底:`web/build` 重建会清空旧 hash chunk(/static/js 一年 immutable),
 * 发布前打开的标签页内存里仍是旧路由表,访问未加载过的路由时 lazy 拉旧 chunk 得 404,
 * 报 "Failed to fetch dynamically imported module"(2026-11-22 减负发布时真实复现)。
 * 处理:模块加载失败时整页刷新一次 —— 壳 no-cache,刷新即拿到新路由表与新 chunk;
 * sessionStorage 哨兵防止服务端持续不可用时无限刷新;加载成功后清掉哨兵。
 * 正则三段文案分别对应 Chrome / Firefox / Safari 的动态导入失败报错。
 */
const STALE_CHUNK_RELOADED_KEY = 'cf:stale-chunk-reloaded'

function lazyRoute(loader: () => Promise<{ default: ComponentType }>) {
  return lazy(() =>
    loader().then(
      (module) => {
        sessionStorage.removeItem(STALE_CHUNK_RELOADED_KEY)
        return module
      },
      (error: unknown) => {
        const isStaleChunk =
          error instanceof TypeError &&
          /failed to fetch dynamically imported module|error loading dynamically imported module|importing a module script failed/i.test(
            error.message,
          )
        if (isStaleChunk && !sessionStorage.getItem(STALE_CHUNK_RELOADED_KEY)) {
          sessionStorage.setItem(STALE_CHUNK_RELOADED_KEY, '1')
          window.location.reload()
        }
        throw error
      },
    ),
  )
}

const AuthEntry = lazyRoute(() => import('@/pages/auth/AuthEntry'))
const RepositoryList = lazyRoute(() => import('@/pages/RepositoryList'))
const NewRepository = lazyRoute(() => import('@/pages/NewRepository'))
const NewRepositoryFork = lazyRoute(() => import('@/pages/NewRepositoryFork'))
const GroupList = lazyRoute(() => import('@/pages/GroupList'))
const NewGroup = lazyRoute(() => import('@/pages/NewGroup'))
const NewMergeRequest = lazyRoute(() => import('@/pages/NewMergeRequest'))
const FileTree = lazyRoute(() => import('@/pages/repository/FileTree'))
const CreateBranch = lazyRoute(() => import('@/pages/repository/CreateBranch'))
const CreateTag = lazyRoute(() => import('@/pages/repository/CreateTag'))
const SettingsProfile = lazyRoute(() => import('@/pages/settings/Profile'))
const SettingsSSHKey = lazyRoute(() => import('@/pages/settings/SSHKey'))
const SettingsNotification = lazyRoute(() => import('@/pages/settings/Notification'))
const SettingsTokens = lazyRoute(() => import('@/pages/settings/Tokens'))
const SettingsAppearance = lazyRoute(() => import('@/pages/settings/Appearance'))
const RepoSettingsGeneral = lazyRoute(() => import('@/pages/repository/SettingsGeneral'))
const RepoSettingsMembers = lazyRoute(() => import('@/pages/repository/SettingsMembers'))
const RepoSettingsBranch = lazyRoute(() => import('@/pages/repository/SettingsBranch'))
const RepoSettingsWebhook = lazyRoute(() => import('@/pages/repository/SettingsWebhook'))
const RepoSettingsAdvanced = lazyRoute(() => import('@/pages/repository/SettingsAdvanced'))
const MergeRequestList = lazyRoute(() => import('@/pages/repository/MergeRequestList'))
const MergeRequestDetailPage = lazyRoute(() => import('@/pages/repository/MergeRequestDetail'))
const MergeRequestCreate = lazyRoute(() => import('@/pages/repository/MergeRequestCreate'))
const GlobalMergeRequestBoard = lazyRoute(() =>
  import('@/pages/MergeRequestBoard').then((m) => ({ default: m.GlobalMergeRequestBoard })),
)
const GroupMergeRequestBoard = lazyRoute(() =>
  import('@/pages/MergeRequestBoard').then((m) => ({ default: m.GroupMergeRequestBoard })),
)
const RepositoryActivityList = lazyRoute(() => import('@/pages/repository/ActivityList'))
const GroupActivityList = lazyRoute(() => import('@/pages/group/ActivityList'))
const AdminDashboard = lazyRoute(() => import('@/pages/admin/Dashboard'))
const AdminUsers = lazyRoute(() => import('@/pages/admin/Users'))
const AdminGroups = lazyRoute(() => import('@/pages/admin/Groups'))
const AdminRepositories = lazyRoute(() => import('@/pages/admin/Repositories'))
const Doc = lazyRoute(() => import('@/pages/Doc'))
const GroupSettingsGeneral = lazyRoute(() => import('@/pages/group/SettingsGeneral'))
const GroupSettingsMembers = lazyRoute(() => import('@/pages/group/SettingsMembers'))
const GroupSettingsAdvanced = lazyRoute(() => import('@/pages/group/SettingsAdvanced'))
const CommitList = lazyRoute(() => import('@/pages/repository/CommitList'))
const CommitDetail = lazyRoute(() => import('@/pages/repository/CommitDetail'))
const BranchList = lazyRoute(() => import('@/pages/repository/BranchList'))
const TagList = lazyRoute(() => import('@/pages/repository/TagList'))
const NotFound = lazyRoute(() => import('@/pages/NotFound'))

function suspense(node: ReactNode): ReactNode {
  return <Suspense fallback={<PageLoading />}>{node}</Suspense>
}

export const router = createBrowserRouter([
  // 文档中心：独立布局（无 AppShell），URL 与旧版一致（/doc/{cn|en}/{path}）。
  { path: '/doc', element: suspense(<Doc />) },
  { path: '/doc/:lang/*', element: suspense(<Doc />) },
  // 认证页：四个子路由共享持久化 AuthEntry 外壳与同一懒加载分包，切换登录/注册零闪烁。
  {
    element: suspense(<AuthEntry />),
    children: [
      { path: '/user/login', element: null },
      { path: '/user/register', element: null },
      { path: '/user/verify', element: null },
      { path: '/user/resetPassword', element: null },
    ],
  },
  {
    path: '/',
    element: <AppShell />,
    children: [
      // 落地页：仓库列表。
      { index: true, element: <Navigate to="/repositories" replace /> },
      { path: 'repositories', element: suspense(<RepositoryList />) },
      { path: 'repositories/new', element: suspense(<NewRepository />) },
      { path: 'repositories/fork/:repositoryId', element: suspense(<NewRepositoryFork />) },
      { path: 'groups', element: suspense(<GroupList />) },
      { path: 'groups/new', element: suspense(<NewGroup />) },
      {
        // 仓库组上下文：URL 与旧版一致（/groups/{组名}/…）；组名合法性由 GroupProvider 承担。
        path: 'groups/:groupName',
        element: <GroupLayout />,
        children: [
          { index: true, element: <Navigate to="repositories" replace /> },
          { path: 'repositories', element: suspense(<RepositoryList />) },
          { path: 'repositories/new', element: suspense(<NewRepository />) },
          { path: 'members', element: suspense(<GroupSettingsMembers />) },
          { path: 'mergerequests', element: suspense(<GroupMergeRequestBoard />) },
          { path: 'mergerequests/new', element: suspense(<NewMergeRequest />) },
          { path: 'activities', element: suspense(<GroupActivityList />) },
          { path: 'settings', element: suspense(<GroupSettingsGeneral />) },
          { path: 'settings/general', element: suspense(<GroupSettingsGeneral />) },
          { path: 'settings/advanced', element: suspense(<GroupSettingsAdvanced />) },
        ],
      },
      {
        path: 'settings',
        element: <SettingsLayout />,
        children: [
          { index: true, element: <Navigate to="profile" replace /> },
          { path: 'profile', element: suspense(<SettingsProfile />) },
          { path: 'sshkey', element: suspense(<SettingsSSHKey />) },
          { path: 'notification', element: suspense(<SettingsNotification />) },
          { path: 'tokens', element: suspense(<SettingsTokens />) },
          { path: 'appearance', element: suspense(<SettingsAppearance />) },
        ],
      },
      {
        path: 'admin',
        element: <AdminLayout />,
        children: [
          { index: true, element: <Navigate to="dashboard" replace /> },
          { path: 'dashboard', element: suspense(<AdminDashboard />) },
          { path: 'users', element: suspense(<AdminUsers />) },
          { path: 'groups', element: suspense(<AdminGroups />) },
          { path: 'repositories', element: suspense(<AdminRepositories />) },
        ],
      },
      { path: 'mergerequests', element: suspense(<GlobalMergeRequestBoard />) },
      { path: 'mergerequests/new', element: suspense(<NewMergeRequest />) },
      {
        // 仓库上下文：URL 形状与旧版一致（/{组名}/{仓库名}/…）；组名/仓库名合法性
        // 由 RepositoryProvider“列表里找不到即 404”承担（v8 不支持路径正则）。
        path: ':groupName/:repositoryName',
        element: <RepositoryLayout />,
        children: [
          { index: true, element: <Navigate to="files" replace /> },
          { path: 'files/*', element: suspense(<FileTree />) },
          { path: 'commits', element: suspense(<CommitList />) },
          // 子目录提交：旧版是 `/:组/:仓库/commits/:rev/*`（`rev` 是分支/标签/短 SHA，
          // `*` 是仓库内的子目录路径）。v8 不支持路径正则，rev 合法性由后端判定。
          { path: 'commits/:rev', element: suspense(<CommitList />) },
          { path: 'commits/:rev/*', element: suspense(<CommitList />) },
          { path: 'commit/:hash', element: suspense(<CommitDetail />) },
          { path: 'branches', element: suspense(<BranchList />) },
          { path: 'branches/new', element: suspense(<CreateBranch />) },
          { path: 'tags', element: suspense(<TagList />) },
          { path: 'tags/new', element: suspense(<CreateTag />) },
          { path: 'mergerequests', element: suspense(<MergeRequestList />) },
          { path: 'mergerequests/new', element: suspense(<MergeRequestCreate />) },
          { path: 'mergerequests/:mid', element: suspense(<MergeRequestDetailPage />) },
          { path: 'members', element: suspense(<RepoSettingsMembers />) },
          { path: 'activities', element: suspense(<RepositoryActivityList />) },
          { path: 'settings', element: suspense(<RepoSettingsGeneral />) },
          { path: 'settings/general', element: suspense(<RepoSettingsGeneral />) },
          { path: 'settings/branch', element: suspense(<RepoSettingsBranch />) },
          { path: 'settings/webhook', element: suspense(<RepoSettingsWebhook />) },
          { path: 'settings/advanced', element: suspense(<RepoSettingsAdvanced />) },
        ],
      },
      { path: '*', element: suspense(<NotFound />) },
    ],
  },
])
