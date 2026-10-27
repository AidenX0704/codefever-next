import { lazy, Suspense, type ReactNode } from 'react'
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
 * 注：状态页（旧 success_message / disable_message / error_message 模板）在 Go 版里
 * 已无调用点（只有 doc 404 与 /lang 参数错误用到），因此不单独建路由，
 * 相关提示由对应页面内联呈现（见计划 §10 批 1 记录）。
 */
const Login = lazy(() => import('@/pages/auth/Login'))
const Register = lazy(() => import('@/pages/auth/Register'))
const Verify = lazy(() => import('@/pages/auth/Verify'))
const ResetPassword = lazy(() => import('@/pages/auth/ResetPassword'))
const RepositoryList = lazy(() => import('@/pages/RepositoryList'))
const NewRepository = lazy(() => import('@/pages/NewRepository'))
const NewRepositoryFork = lazy(() => import('@/pages/NewRepositoryFork'))
const GroupList = lazy(() => import('@/pages/GroupList'))
const NewGroup = lazy(() => import('@/pages/NewGroup'))
const NewMergeRequest = lazy(() => import('@/pages/NewMergeRequest'))
const FileTree = lazy(() => import('@/pages/repository/FileTree'))
const CreateBranch = lazy(() => import('@/pages/repository/CreateBranch'))
const CreateTag = lazy(() => import('@/pages/repository/CreateTag'))
const SettingsProfile = lazy(() => import('@/pages/settings/Profile'))
const SettingsEmail = lazy(() => import('@/pages/settings/Email'))
const SettingsSSHKey = lazy(() => import('@/pages/settings/SSHKey'))
const SettingsNotification = lazy(() => import('@/pages/settings/Notification'))
const SettingsTokens = lazy(() => import('@/pages/settings/Tokens'))
const RepoSettingsGeneral = lazy(() => import('@/pages/repository/SettingsGeneral'))
const RepoSettingsMembers = lazy(() => import('@/pages/repository/SettingsMembers'))
const RepoSettingsBranch = lazy(() => import('@/pages/repository/SettingsBranch'))
const RepoSettingsWebhook = lazy(() => import('@/pages/repository/SettingsWebhook'))
const RepoSettingsAdvanced = lazy(() => import('@/pages/repository/SettingsAdvanced'))
const MergeRequestList = lazy(() => import('@/pages/repository/MergeRequestList'))
const MergeRequestDetailPage = lazy(() => import('@/pages/repository/MergeRequestDetail'))
const MergeRequestCreate = lazy(() => import('@/pages/repository/MergeRequestCreate'))
const GlobalMergeRequestBoard = lazy(() =>
  import('@/pages/MergeRequestBoard').then((m) => ({ default: m.GlobalMergeRequestBoard })),
)
const GroupMergeRequestBoard = lazy(() =>
  import('@/pages/MergeRequestBoard').then((m) => ({ default: m.GroupMergeRequestBoard })),
)
const RepositoryActivityList = lazy(() => import('@/pages/repository/ActivityList'))
const GroupActivityList = lazy(() => import('@/pages/group/ActivityList'))
const AdminDashboard = lazy(() => import('@/pages/admin/Dashboard'))
const AdminUsers = lazy(() => import('@/pages/admin/Users'))
const AdminGroups = lazy(() => import('@/pages/admin/Groups'))
const AdminRepositories = lazy(() => import('@/pages/admin/Repositories'))
const AdminSettings = lazy(() => import('@/pages/admin/Settings'))
const Doc = lazy(() => import('@/pages/Doc'))
const Help = lazy(() => import('@/pages/Help'))
const Home = lazy(() => import('@/pages/Home'))
const Search = lazy(() => import('@/pages/Search'))
const GroupSettingsGeneral = lazy(() => import('@/pages/group/SettingsGeneral'))
const GroupSettingsMembers = lazy(() => import('@/pages/group/SettingsMembers'))
const GroupSettingsAdvanced = lazy(() => import('@/pages/group/SettingsAdvanced'))
const CommitList = lazy(() => import('@/pages/repository/CommitList'))
const CommitDetail = lazy(() => import('@/pages/repository/CommitDetail'))
const BranchList = lazy(() => import('@/pages/repository/BranchList'))
const TagList = lazy(() => import('@/pages/repository/TagList'))
const DesignSystem = lazy(() => import('@/pages/DesignSystem'))
const NotFound = lazy(() => import('@/pages/NotFound'))

function suspense(node: ReactNode): ReactNode {
  return <Suspense fallback={<PageLoading />}>{node}</Suspense>
}

export const router = createBrowserRouter([
  // 文档中心：独立布局（无 AppShell），URL 与旧版一致（/doc/{cn|en}/{path}）。
  { path: '/doc', element: suspense(<Doc />) },
  { path: '/doc/:lang/*', element: suspense(<Doc />) },
  { path: '/user/login', element: suspense(<Login />) },
  { path: '/user/register', element: suspense(<Register />) },
  { path: '/user/verify', element: suspense(<Verify />) },
  { path: '/user/resetPassword', element: suspense(<ResetPassword />) },
  {
    path: '/',
    element: <AppShell />,
    children: [
      // 落地页：员工首页（`/home`）。2026-10-27 起由 `/repositories` 改为 `/home` ——
      // 登录后第一屏要回答"我该做什么"，而不是先给一张仓库列表。
      // 后端 `home/index` 的 302 目标已同步改为 `/home`（handlers/web.go），两处必须一起改。
      { index: true, element: <Navigate to="/home" replace /> },
      { path: 'home', element: suspense(<Home />) },
      // 全局搜索（阶段一：仓库 / 仓库组 / 合并请求标题）。关键词走 query，可分享。
      { path: 'search', element: suspense(<Search />) },
      { path: '__design', element: suspense(<DesignSystem />) },
      // 帮助中心：站内帮助落点（技术支持 / 反馈 / 贡献 / 关于 + 公开 API 说明）。
      // 与文档中心（/doc/*）的分工：那边是"产品怎么用"，这里回答"有问题去哪儿"。
      { path: 'help', element: suspense(<Help />) },
      // 批 2 已落地；其余为最终路由结构的占位，逐批替换（见计划 §6）。
      { path: 'repositories', element: suspense(<RepositoryList />) },
      { path: 'repositories/new', element: suspense(<NewRepository />) },
      { path: 'repositories/fork/:repositoryId', element: suspense(<NewRepositoryFork />) },
      // fork 列表上下文：只显示 forkFrom 指向该仓库的仓库（旧版 /repositories/forklist/:id）。
      { path: 'repositories/forklist/:repositoryId', element: suspense(<RepositoryList />) },
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
          { path: 'email', element: suspense(<SettingsEmail />) },
          { path: 'sshkey', element: suspense(<SettingsSSHKey />) },
          { path: 'notification', element: suspense(<SettingsNotification />) },
          { path: 'tokens', element: suspense(<SettingsTokens />) },
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
          { path: 'settings', element: suspense(<AdminSettings />) },
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
