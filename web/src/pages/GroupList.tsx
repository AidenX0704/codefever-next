import { useState } from 'react'
import { useNavigate } from 'react-router'

import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { Loading } from '@/components/unit/Loading'
import { Tabs } from '@/components/unit/Tabs'
import { GroupCard } from '@/components/view/unit/GroupCard'
import { canReadRepository } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


type TabKey = 'all' | 'joined' | 'created'

/**
 * 仓库组列表（`/groups`）。
 *
 * 过滤口径对齐旧版 `components/view/GroupList.js`：先按 REPO_READ 权限过滤，
 * 再按页签（全部 / 我参与的 / 我创建的）过滤；卡片里的仓库数由全局仓库列表
 * 按 `group.id` 统计（旧版同样是前端统计，不是后端字段）。
 */
export default function GroupList() {
  const t = useT()
  const navigate = useNavigate()
  const { user, groups, repositories, loading, error } = useAppData()
  const [tab, setTab] = useState<TabKey>('all')

  const userId = user?.id ?? ''
  const readable = groups.filter((group) => canReadRepository(group.role))
  const filtered = readable.filter((group) => {
    if (tab === 'joined') {
      return group.owner !== userId
    }
    if (tab === 'created') {
      return group.owner === userId
    }
    return true
  })

  const newGroupButton = (block = false) => (
    <Button variant="contained" block={block} onClick={() => navigate('/groups/new')}>
      {t('label.newGroup')}
    </Button>
  )

  return (
    <div className="page">
      <div className="list-header">
        <Tabs<TabKey>
          value={tab}
          onChange={setTab}
          items={[
            { key: 'all', label: t('label.all') },
            { key: 'joined', label: t('label.IJoined') },
            { key: 'created', label: t('label.ICreated') },
          ]}
        />
        {newGroupButton()}
      </div>

      {error ? <div className="auth-message">{error}</div> : null}

      {loading ? (
        <Loading />
      ) : filtered.length > 0 ? (
        <div className="group-grid">
          {filtered.map((group) => (
            <GroupCard
              key={group.id}
              group={group}
              repositoryCount={repositories.filter((repository) => repository.group.id === group.id).length}
            />
          ))}
        </div>
      ) : (
        <EmptyState
          imageName="repositories-empty.png"
          title={t('message._S_listEmpty', { s: t('label.group') })}
          notice={t('message.groupListEmptyNotice')}
          action={newGroupButton(true)}
        />
      )}
    </div>
  )
}
