import { useCallback, useEffect, useState } from 'react'

import { fetchGroupActivities } from '@/api/group'
import type { ActivityItem } from '@/api/types'
import { EmptyState } from '@/components/unit/EmptyState'
import { Loading } from '@/components/unit/Loading'
import { useToast } from '@/components/unit/Toast'
import { ActivityFeed } from '@/components/view/unit/ActivityFeed'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useGroup } from '@/features/group/GroupProvider'
import { useT } from '@/i18n'

/**
 * 仓库组动态（`/groups/{组名}/activities`）。
 *
 * `group/activities?group&category`，渲染复用 `ActivityFeed`（与仓库动态同一套 41 码映射）。
 */
export default function GroupActivityList() {
  const t = useT()
  const toast = useToast()
  const { summary, config } = useGroup()
  const { groups, repositories } = useAppData()

  const [items, setItems] = useState<ActivityItem[]>([])
  const [loading, setLoading] = useState(false)

  const load = useCallback(() => {
    if (!summary) {
      return
    }
    setLoading(true)
    fetchGroupActivities({ groupKey: summary.id })
      .then(setItems)
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => setLoading(false))
  }, [summary, t, toast])

  useEffect(() => {
    load()
  }, [load])

  const memberNameOf = (id: string) => config?.members.find((member) => member.id === id)?.name ?? id

  return (
    <div className="page">
      {loading ? <Loading /> : null}
      {!loading && items.length === 0 ? (
        <EmptyState title={t('message._S_listEmpty', { s: t('label.activity_pl') })} />
      ) : null}
      <ActivityFeed
        items={items}
        memberNameOf={memberNameOf}
        memberIconOf={(id) => config?.members.find((member) => member.id === id)?.icon ?? null}
        groupNameOf={(id) => groups.find((group) => group.id === id)?.displayName ?? id ?? ''}
        repositoryNameOf={(id) => repositories.find((repository) => repository.id === id)?.displayName ?? id ?? ''}
      />
    </div>
  )
}
