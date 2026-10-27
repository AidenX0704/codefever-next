import { useEffect } from 'react'
import { Outlet, useParams } from 'react-router'

import { PageLoading } from '@/components/unit/Loading'
import { repositoryDrawerEntries } from '@/config/drawer'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { recordRecentRepository } from '@/helpers/recentRepos'
import { RepositoryProvider, useRepository } from '@/features/repository/RepositoryProvider'
import { RepositoryHeader } from '@/components/view/unit/RepositoryHeader'
import NotFound from '@/pages/NotFound'
import { Sidebar } from './Sidebar'


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
    <div className="page page--full">

      <div className="cf-repo-layout">
        <Sidebar entries={repositoryDrawerEntries(config)} />
        <div className="cf-repo-main">

          {config ? <RepositoryHeader config={config} user={user} forkSource={forkSource} /> : null}
          <Outlet />
        </div>
      </div>
    </div>
  )
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
