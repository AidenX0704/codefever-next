import { useCallback, useEffect, useState } from 'react'

import { fetchGroupActivities } from '@/api/group'
import { ACTIVITY_CATEGORY } from '@/api/repository'
import type { ActivityItem } from '@/api/types'
import { EmptyState } from '@/components/unit/EmptyState'
import { FilterBar } from '@/components/unit/FilterBar'
import { ListSkeleton } from '@/components/unit/Skeleton'
import { Pagination } from '@/components/unit/Table'
import { useToast } from '@/components/unit/Toast'
import { ActivityFeed } from '@/components/view/unit/ActivityFeed'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useGroup } from '@/features/group/GroupProvider'
import { useT } from '@/i18n'


const PAGE_SIZE = 20

/**
 * 仓库组动态（`/groups/{组名}/activities`）。
 *
 * 补齐与仓库动态一致的分类页签、关键字搜索与分页（并传入后端必需的 `page`/`pagesize`）。
 */
export default function GroupActivityList() {
  const t = useT()
  const toast = useToast()
  const { summary, config } = useGroup()
  const { groups, repositories } = useAppData()

  const [category, setCategory] = useState<number>(ACTIVITY_CATEGORY.all)
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [items, setItems] = useState<ActivityItem[]>([])
  const [hasNext, setHasNext] = useState(false)
  const [loading, setLoading] = useState(false)

  const load = useCallback(() => {
    if (!summary) {
      return
    }
    setLoading(true)
    fetchGroupActivities({
      groupKey: summary.id,
      category,
      keyword,
      page,
      pagesize: PAGE_SIZE + 1,
    })
      .then((list) => {
        setHasNext(list.length > PAGE_SIZE)
        setItems(list.slice(0, PAGE_SIZE))
      })
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => setLoading(false))
  }, [summary, category, keyword, page, t, toast])

  useEffect(() => {
    load()
  }, [load])

  const memberNameOf = (id: string) => config?.members.find((member) => member.id === id)?.name ?? id

  return (
    <div className="page">
      <FilterBar
        tabs={[
          { key: String(ACTIVITY_CATEGORY.all), label: t('label.all') },
          { key: String(ACTIVITY_CATEGORY.commit), label: t('label.commitActivity') },
          { key: String(ACTIVITY_CATEGORY.mergeRequest), label: t('label.mergeRequestActivity') },
          { key: String(ACTIVITY_CATEGORY.member), label: t('label.memberActivity') },
        ]}
        activeTab={String(category)}
        onTabChange={(key) => {
          setCategory(Number(key))
          setPage(1)
        }}
        searchValue={keywordInput}
        onSearchChange={setKeywordInput}
        onSearchSubmit={() => {
          setKeyword(keywordInput)
          setPage(1)
        }}
        searchPlaceholder={t('label.search')}
      />

      <div className="list-sheet">
        {loading ? (
          <ListSkeleton rows={5} />
        ) : items.length === 0 ? (
          <EmptyState title={t('message._S_listEmpty', { s: t('label.activity_pl') })} />
        ) : (
          <div className="activity-sheet">
            <ActivityFeed
              items={items}
              memberNameOf={memberNameOf}
              memberIconOf={(id) => config?.members.find((member) => member.id === id)?.icon ?? null}
              groupNameOf={(id) => groups.find((group) => group.id === id)?.displayName ?? id ?? ''}
              repositoryNameOf={(id) => repositories.find((repository) => repository.id === id)?.displayName ?? id ?? ''}
            />
          </div>
        )}
      </div>

      {items.length > 0 ? (
        <Pagination page={page} pageCount={page + (hasNext ? 1 : 0)} onChange={setPage} />
      ) : null}
    </div>
  )
}
