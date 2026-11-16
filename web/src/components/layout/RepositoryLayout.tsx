import { useEffect } from 'react'
import { NavLink, Outlet, useLocation, useParams } from 'react-router'

import { PageLoading } from '@/components/unit/Loading'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { recordRecentRepository } from '@/helpers/recentRepos'
import { RepositoryProvider, useRepository } from '@/features/repository/RepositoryProvider'
import { RepositoryHeader } from '@/components/view/unit/RepositoryHeader'
import NotFound from '@/pages/NotFound'
import { useT } from '@/i18n'


/** 仓库页内一级 Tabs（v5 §7.4 RepoHeader Tabs 行）：代码｜提交｜分支｜标签｜合并请求｜设置。 */
type RepoTabKey = 'files' | 'commits' | 'branches' | 'tags' | 'mergerequests' | 'settings'

function RepositoryFrame() {
  const { config, loading, notFound } = useRepository()
  const { user, repositories } = useAppData()

  // 记录"最近访问"（首页侧栏用）。放在外壳里而不是文件页：进仓库的任意子页
  // （文件 / 提交 / 分支 / 合并请求 / 设置）都算"访问过这个仓库"。
  const visitedGroup = config?.group.name
  const visitedRepository = config?.repository.name
  useEffect(() => {
    if (visitedGroup && visitedRepository) {
      recordRecentRepository(visitedGroup, visitedRepository)
    }
  }, [visitedGroup, visitedRepository])

  if (notFound) {
    return <NotFound />
  }
  if (loading && !config) {
    return <PageLoading />
  }

  // fork 来源：config.repository.forkFrom 是源仓库主键，从全局列表里反查。
  const forkSource = repositories.find((item) => item.id === config?.repository.forkFrom) ?? null

  return (
    <div className="repo-page">
      {config ? (
        <RepositoryHeader config={config} user={user} forkSource={forkSource}>
          {/* v5 §7.4：Tabs 行由页头承载（贴纸页底），不再是页头外面的独立导航条。 */}
          <RepoTabs config={config} />
        </RepositoryHeader>
      ) : null}
      <Outlet />
    </div>
  )
}

/** v5 §7.4：仓库内二级导航 = RepoHeader 底部的下划线 Tabs（左侧上下文导航随 v5 退役）。 */
function RepoTabs({ config }: { config: NonNullable<ReturnType<typeof useRepository>['config']> }) {
  const t = useT()
  const { pathname } = useLocation()
  const repositoryPath = `/${config.group.name}/${config.repository.name}`
  const active = useRepoTab(repositoryPath, pathname)

  const tabs: Array<{ key: RepoTabKey; label: string; to: string; count?: number }> = [
    { key: 'files', label: t('menu.file_pl'), to: repositoryPath },
    { key: 'commits', label: t('menu.commit_pl'), to: `${repositoryPath}/commits` },
    { key: 'branches', label: t('menu.branch_pl'), to: `${repositoryPath}/branches` },
    { key: 'tags', label: t('menu.tag_pl'), to: `${repositoryPath}/tags` },
    // 未合并 MR 数挂在 Tab 上（GitHub 把 PR 数放在同位）；其余 Tab 不放计数。
    {
      key: 'mergerequests',
      label: t('menu.mergeRequest_pl'),
      to: `${repositoryPath}/mergerequests`,
      count: config.repository.mergeRequestCount.open,
    },
    { key: 'settings', label: t('menu.setting_pl'), to: `${repositoryPath}/settings` },
  ]

  return (
    <nav className="cf-tabs repo-tabs">
      {tabs.map((tab) => (
        <NavLink
          key={tab.key}
          to={tab.to}
          className={tab.key === active ? 'cf-tab cf-tab--active' : 'cf-tab'}
          aria-current={tab.key === active ? 'page' : undefined}
        >
          {tab.label}
          {tab.count !== undefined ? <span className="cf-tab__count">{tab.count}</span> : null}
        </NavLink>
      ))}
    </nav>
  )
}

/** files Tab 兼容仓库根路径与 blame 子路径（与旧 drawer 的 active 语义一致）。 */
function useRepoTab(repositoryPath: string, pathname: string): RepoTabKey {
  if (new RegExp(`^${repositoryPath}/commits`, 'i').test(pathname)) return 'commits'
  if (new RegExp(`^${repositoryPath}/branches`, 'i').test(pathname)) return 'branches'
  if (new RegExp(`^${repositoryPath}/tags`, 'i').test(pathname)) return 'tags'
  if (new RegExp(`^${repositoryPath}/mergerequests`, 'i').test(pathname)) return 'mergerequests'
  if (new RegExp(`^${repositoryPath}/settings`, 'i').test(pathname)) return 'settings'
  return 'files'
}

/**
 * 仓库页外壳：解析 `:groupName` / `:repositoryName` → 加载仓库 config → 渲染左侧导航 + 子页面。
 *
 * URL 形状与旧版保持一致：`/{组名}/{仓库名}/files`、`/commits`、`/branches`、`/tags`、`/settings/...`。
 * 组名/仓库名的合法性由 `RepositoryProvider` 的"列表里找不到即 404"承担
 * （react-router 8 不支持路径内嵌正则，见计划 §8 偏离 #3）。
 */
export function RepositoryLayout() {
  const { groupName, repositoryName } = useParams()

  return (
    <RepositoryProvider groupName={groupName} repositoryName={repositoryName}>
      <RepositoryFrame />
    </RepositoryProvider>
  )
}
