import { Outlet, useParams } from 'react-router'

import { PageLoading } from '@/components/unit/Loading'
import { groupDrawerEntries } from '@/config/drawer'
import { GroupProvider, useGroup } from '@/features/group/GroupProvider'
import NotFound from '@/pages/NotFound'
import { Sidebar } from './Sidebar'


function GroupFrame() {
  const { config, loading, notFound } = useGroup()

  if (notFound) {
    return <NotFound />
  }
  if (loading && !config) {
    return <PageLoading />
  }

  return (
    <div className="page page--full">
      <div className="cf-repo-layout">
        <Sidebar entries={groupDrawerEntries(config?.group ?? null)} />
        <div className="cf-repo-main">

          <div className="repo-head">
            <div className="repo-head__title">{config?.group.displayName}</div>
            {config?.group.description ? <div className="repo-head__desc">{config.group.description}</div> : null}
          </div>
          <Outlet />
        </div>
      </div>
    </div>
  )
}

/**
 * 仓库组外壳（`/groups/{组名}/*`）。
 *
 * 与旧版一致：左侧 5 项导航（仓库 / 合并请求 / 成员 / 动态 / 设置）；
 * URL 使用组标识名（`group.name`），组名合法性由"全局列表里找不到即 404"承担。
 */
export function GroupLayout() {
  const { groupName } = useParams()

  return (
    <GroupProvider groupName={groupName}>
      <GroupFrame />
    </GroupProvider>
  )
}
