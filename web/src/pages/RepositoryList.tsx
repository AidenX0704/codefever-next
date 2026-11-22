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
 * 另外对齐旧版的组上下文分支（`/groups/{组名}/repositories`）：只显示该组仓库，
 * 首个页签文案改为"仓库"。fork 上下文（旧 /repositories/forklist/:id）已于
 * 2026-11-22 减负下线。
 */
export default function RepositoryList() {
  const t = useT()
  const navigate = useNavigate()
  const { groupName } = useParams()
  const { user, repositories, groups, loading, error } = useAppData()
  const [tab, setTab] = useState<TabKey>('all')

  // 组上下文（/groups/{组名}/repositories）：只显示该组下的仓库，首个页签文案改为"仓库"，
  // 与旧版 RepositoryList 的行为一致（旧版用 currentGroupKey 判断）。
  const currentGroup = groupName ? groups.find((group) => group.name === groupName) : undefined

  const userId = user?.id ?? ''
  const scoped = currentGroup
    ? repositories.filter((repository) => repository.group.id === currentGroup.id)
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

  const firstTabLabel = currentGroup ? t('label.repository') : t('label.all')

  const newRepositoryButton = (block = false) => (
    <Button variant="contained" block={block} onClick={() => navigate(currentGroup ? `/groups/${currentGroup.name}/repositories/new` : '/repositories/new')}>
      {t('label.newRepository')}
    </Button>
  )

  return (
    <div className="page">
      {/* v5 §6 G2 页头：标题 24/600 + 计数 + 右侧唯一主按钮。 */}
      <div className="page-head">
        <div className="page-head__text">
          <h1 className="page__title">{currentGroup ? currentGroup.displayName : t('label.repository')}</h1>
          <span className="page-head__count">{t('label._N_repository', { n: filtered.length })}</span>
        </div>
        {currentGroup ? null : newRepositoryButton()}
      </div>

      {error ? <div className="auth-message">{error}</div> : null}

      <Tabs<TabKey>
        value={tab}
        onChange={setTab}
        items={[
          { key: 'all', label: firstTabLabel },
          { key: 'joined', label: t('label.IJoined') },
          { key: 'created', label: t('label.ICreated') },
        ]}
      />

      <div className="list-sheet">
        {loading ? (
          <Loading />
        ) : filtered.length > 0 ? (
          filtered.map((repository) => <RepositoryRow key={repository.id} repository={repository} />)
        ) : (
          <EmptyState
            imageName="repositories-empty.png"
            title={t('message._S_listEmpty', { s: t('label.repository') })}
            notice={t('message.repositoryListEmptyNotice')}
            action={currentGroup ? undefined : newRepositoryButton(true)}
          />
        )}
      </div>
    </div>
  )
}
