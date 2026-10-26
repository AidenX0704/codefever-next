import { useCallback, useEffect, useState } from 'react'

import { ACTIVITY_CATEGORY, fetchRepositoryActivities } from '@/api/repository'
import type { ActivityItem } from '@/api/types'
import { EmptyState } from '@/components/unit/EmptyState'
import { FilterBar } from '@/components/unit/FilterBar'
import { Loading } from '@/components/unit/Loading'
import { Pagination } from '@/components/unit/Table'
import { useToast } from '@/components/unit/Toast'
import { ActivityFeed } from '@/components/view/unit/ActivityFeed'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'

/** 每页条数（旧版 `ActivityList.js` 的 pagesize = 20）。 */
const PAGE_SIZE = 20

/**
 * 仓库动态（`/{组}/{仓库}/activities`）。
 *
 * `repository/activities?repository&category&keyword&page&pagesize`（category 由后端 `parseCategory` 校验）。
 * 渲染交给 `ActivityFeed`（41 个活动码的表驱动实现，与旧 `config/Activity.js` 文案一一对应）。
 *
 * 与旧版对齐的三件事：**分类页签**（全部/提交/MR/成员）、**关键字搜索**（回车提交）、
 * **翻页**（每页 20 条，页码分页）。
 */
export default function RepositoryActivityList() {
  const t = useT()
  const toast = useToast()
  const { summary, config } = useRepository()
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
    // 多取一条用来精确判断"还有没有下一页"（返回条数 < pagesize 的老做法不精确）。
    fetchRepositoryActivities({
      repositoryKey: summary.id,
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
      {items.length > 0 ? (
        <Pagination page={page} pageCount={page + (hasNext ? 1 : 0)} onChange={setPage} />
      ) : null}
    </div>
  )
}
