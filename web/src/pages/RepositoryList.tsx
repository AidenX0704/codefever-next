import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { Loading } from '@/components/unit/Loading'
import { Tabs } from '@/components/unit/Tabs'
import { RepositoryRow } from '@/components/view/unit/RepositoryRow'
import { canReadRepository } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


type TabKey = 'all' | 'joined' | 'created'

/**
 * 仓库列表（`/repositories`）。
 *
 * 过滤与排序逐项对齐旧版 `components/view/RepositoryList.js`：
 *   1. 先按 `UAC.PermissionCode.REPO_READ` 过滤掉无读权限的仓库；
 *   2. 再按页签过滤：全部 / 我参与的（owner 不是自己）/ 我创建的（owner 是自己）；
 *   3. 按 `updated` 倒序（旧版在 render 里 sort，这里用不可变复制）。
 *
 * 另外对齐旧版的两处上下文分支：
 *   - 组上下文（`/groups/{组名}/repositories`）：只显示该组仓库，首个页签文案改为"仓库"；
 *   - fork 上下文（`/repositories/forklist/{仓库主键}`）：只显示 `forkFrom` 指向该仓库的
 *     fork 列表，首个页签文案改为"fork 自 组名/仓库名"，且不显示"新建仓库"按钮。
 */
export default function RepositoryList() {
  const t = useT()
  const navigate = useNavigate()
  const { groupName, repositoryId } = useParams()
  const { user, repositories, groups, loading, error } = useAppData()
  const [tab, setTab] = useState<TabKey>('all')

  // 组上下文（/groups/{组名}/repositories）：只显示该组下的仓库，首个页签文案改为"仓库"，
  // 与旧版 RepositoryList 的行为一致（旧版用 currentGroupKey 判断）。
  const currentGroup = groupName ? groups.find((group) => group.name === groupName) : undefined

  // fork 上下文（/repositories/forklist/{仓库主键}）：只显示由该仓库 fork 出来的仓库。
  const forkSource = repositoryId ? repositories.find((repository) => repository.id === repositoryId) : undefined

  const userId = user?.id ?? ''
  const scoped = currentGroup
    ? repositories.filter((repository) => repository.group.id === currentGroup.id)
    : repositoryId
      ? repositories.filter((repository) => repository.forkFrom === repositoryId)
      : repositories
  const readable = scoped.filter((repository) => canReadRepository(repository.role))
  const sorted = [...readable].sort((a, b) => b.updated - a.updated)
  const filtered = sorted.filter((repository) => {
    if (tab === 'joined') {
      return repository.owner !== userId
    }
    if (tab === 'created') {
      return repository.owner === userId
    }
    return true
  })

  const firstTabLabel = currentGroup
    ? t('label.repository')
    : repositoryId
      ? t('label.forkFrom_S', {
          s: forkSource
            ? `${forkSource.group.displayName}/${forkSource.displayName}`
            : t('label.unknownRepository'),
        })
      : t('label.all')

  const newRepositoryButton = (block = false) => (
    <Button variant="contained" block={block} onClick={() => navigate(currentGroup ? `/groups/${currentGroup.name}/repositories/new` : '/repositories/new')}>
      {t('label.newRepository')}
    </Button>
  )

  return (
    <div className="page">
      <div className="list-header">
        <Tabs<TabKey>
          value={tab}
          onChange={setTab}
          items={[
            { key: 'all', label: firstTabLabel },
            { key: 'joined', label: t('label.IJoined') },
            { key: 'created', label: t('label.ICreated') },
          ]}
        />
        {currentGroup || repositoryId ? null : newRepositoryButton()}
      </div>

      {error ? <div className="auth-message">{error}</div> : null}

      <div className="list-body">
        {loading ? (
          <Loading />
        ) : filtered.length > 0 ? (
          filtered.map((repository) => <RepositoryRow key={repository.id} repository={repository} />)
        ) : (
          <EmptyState
            imageName="repositories-empty.png"
            title={t('message._S_listEmpty', { s: t('label.repository') })}
            notice={t('message.repositoryListEmptyNotice')}
            action={currentGroup || repositoryId ? undefined : newRepositoryButton(true)}
          />
        )}
      </div>
    </div>
  )
}
