import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { fetchGroupMergeRequests } from '@/api/group'
import { fetchRelatedMergeRequests } from '@/api/repository'
import type { MergeRequestListItem } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { useToast } from '@/components/unit/Toast'
import { MergeRequestListBody } from '@/components/view/unit/MergeRequestListBody'
import {
  MERGE_REQUEST_STATUS_BY_TAB,
  MergeRequestToolbar,
} from '@/components/view/unit/MergeRequestToolbar'
import type { MergeRequestCounts, MergeRequestTabKey } from '@/components/view/unit/MergeRequestToolbar'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useGroup } from '@/features/group/GroupProvider'
import { useT } from '@/i18n'


/** 旧版每页 10 条（`MergeRequest.js` 的 `pagesize: 10`）。 */
const PAGE_SIZE = 10

interface BoardQuery {
  status: number
  keyword: string
  sort: string
  page: number
  pagesize: number
}

interface BoardResult {
  mergeRequests: MergeRequestListItem[]
  /** 全局看板：后端一并返回的"相关仓库"主键（用于算角标）。 */
  relatedRepository?: string[]
}

interface BoardState {
  tab: MergeRequestTabKey
  changeTab: (tab: MergeRequestTabKey) => void
  /** 输入框里的关键字（未回车 / 未点搜索前不生效）。 */
  keywordInput: string
  setKeywordInput: (value: string) => void
  /** 已提交的关键字（旧版 `observed.keyword`）：有关键字时不显示页签角标。 */
  keyword: string
  search: () => void
  sortDesc: boolean
  toggleSort: () => void
  items: MergeRequestListItem[]
  loading: boolean
  noMore: boolean
  page: number
  loadMore: () => void
  relatedRepository: string[]
}

/**
 * 全局 / 组内看板共用的列表状态机。
 *
 * 旧版 `components/view/MergeRequest.js` 用一个组件同时承载三种作用域
 * （仓库内 / 组内 / 全局），状态字段完全一致：页签、关键字、排序、页码、
 * 累积列表、"没有更多"。这里把它收敛成一个 hook，三处页面复用（AGENTS §4）。
 */
function useMergeRequestBoard(fetcher: (query: BoardQuery) => Promise<BoardResult>): BoardState {
  const t = useT()
  const toast = useToast()

  const [tab, setTab] = useState<MergeRequestTabKey>('all')
  /** 输入框里的关键字（未回车 / 未点搜索前不生效）。 */
  const [keywordInput, setKeywordInput] = useState('')
  /** 已提交的关键字（旧版 `observed.keyword`）。 */
  const [keyword, setKeyword] = useState('')
  const [sortDesc, setSortDesc] = useState(true)
  const [page, setPage] = useState(1)
  const [items, setItems] = useState<MergeRequestListItem[]>([])
  const [noMore, setNoMore] = useState(false)
  const [loading, setLoading] = useState(false)
  const [relatedRepository, setRelatedRepository] = useState<string[]>([])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    fetcher({
      status: MERGE_REQUEST_STATUS_BY_TAB[tab],
      keyword,
      sort: sortDesc ? 'desc' : 'asc',
      page,
      pagesize: PAGE_SIZE,
    })
      .then((result) => {
        if (cancelled) {
          return
        }
        const list = result.mergeRequests
        setItems((previous) => (page === 1 ? list : [...previous, ...list]))
        setNoMore(list.length < PAGE_SIZE)
        if (result.relatedRepository) {
          setRelatedRepository(result.relatedRepository)
        }
      })
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [fetcher, tab, keyword, sortDesc, page, t, toast])

  /** 重新取数：回到第一页并清空累积列表（先置 loading，避免闪一下空态）。 */
  const reset = () => {
    setPage(1)
    setItems([])
    setLoading(true)
  }

  return {
    tab,
    changeTab: (next) => {
      if (next === tab) {
        return
      }
      setTab(next)
      reset()
    },
    keywordInput,
    setKeywordInput,
    keyword,
    search: () => {
      if (keywordInput === keyword) {
        return
      }
      setKeyword(keywordInput)
      reset()
    },
    sortDesc,
    toggleSort: () => {
      setSortDesc((previous) => !previous)
      reset()
    },
    items,
    loading,
    noMore,
    page,
    loadMore: () => setPage((previous) => previous + 1),
    relatedRepository,
  }
}

/** 合并请求详情链接：按目标仓库拼 `/{组}/{仓库}/mergerequests/{编号}`。 */
function useMergeRequestHref(): (item: MergeRequestListItem) => string {
  const { repositories } = useAppData()
  return useCallback(
    (item: MergeRequestListItem): string => {
      const repository = repositories.find((candidate) => candidate.id === item.targetRepository)
      if (!repository) {
        return '/repositories'
      }
      return `/${repository.group.name}/${repository.name}/mergerequests/${item.number}`
    },
    [repositories],
  )
}

/** 全局合并请求（`/mergerequests`）：跨当前用户所有仓库，`repository/relatedMergeRequests`。 */
export function GlobalMergeRequestBoard() {
  const t = useT()
  const navigate = useNavigate()
  const { repositories } = useAppData()

  const fetchBoard = useCallback(
    (query: BoardQuery): Promise<BoardResult> => fetchRelatedMergeRequests(query),
    [],
  )
  const board = useMergeRequestBoard(fetchBoard)
  const { tab, changeTab, keywordInput, setKeywordInput, keyword, search, sortDesc, toggleSort, items, loading, noMore, page, loadMore, relatedRepository } = board
  const hrefOf = useMergeRequestHref()

  /**
   * 角标（旧版 `MergeRequest.js:198-208`）：把 `repositoryList` 里
   * `id` 命中 `relatedRepository` 的仓库的 `mergeRequestCount` 累加。
   */
  const counts = useMemo<MergeRequestCounts | null>(() => {
    if (keyword) {
      return null
    }
    const totals = { all: 0, open: 0, merged: 0, closed: 0 }
    repositories.forEach((repository) => {
      if (!relatedRepository.includes(repository.id)) {
        return
      }
      totals.open += repository.mergeRequestCount.open
      totals.merged += repository.mergeRequestCount.merged
      totals.closed += repository.mergeRequestCount.closed
    })
    totals.all = totals.open + totals.merged + totals.closed
    return totals
  }, [repositories, relatedRepository, keyword])

  return (
    <div className="page">
      <MergeRequestToolbar
        tab={tab}
        onTabChange={changeTab}
        counts={counts}
        keyword={keywordInput}
        onKeywordChange={setKeywordInput}
        onSearch={search}
        sortDesc={sortDesc}
        onSortToggle={toggleSort}
        sortDisabled={loading}
        action={
          <Button variant="contained" onClick={() => navigate('/mergerequests/new')}>
            {t('label.createMergeRequest')}
          </Button>
        }
      />

      <MergeRequestListBody
        items={items}
        loading={loading}
        noMore={noMore}
        loadingMore={loading && page > 1}
        onLoadMore={loadMore}
        hrefOf={hrefOf}
        showRepository
      />
    </div>
  )
}

/** 组内合并请求（`/groups/{组名}/mergerequests`）：`group/mergeRequests`。 */
export function GroupMergeRequestBoard() {
  const t = useT()
  const navigate = useNavigate()
  const { groupName } = useParams()
  const { summary } = useGroup()
  const { repositories } = useAppData()

  const fetchBoard = useCallback(
    async (query: BoardQuery): Promise<BoardResult> => {
      if (!summary) {
        return { mergeRequests: [] }
      }
      const mergeRequests = await fetchGroupMergeRequests({ groupKey: summary.id, ...query })
      return { mergeRequests }
    },
    [summary],
  )
  const board = useMergeRequestBoard(fetchBoard)
  const { tab, changeTab, keywordInput, setKeywordInput, keyword, search, sortDesc, toggleSort, items, loading, noMore, page, loadMore } = board
  const hrefOf = useMergeRequestHref()

  /**
   * 角标（旧版 `MergeRequest.js:210-217`）：累加**同组**仓库的 `mergeRequestCount`。
   */
  const counts = useMemo<MergeRequestCounts | null>(() => {
    if (!summary || keyword) {
      return null
    }
    const totals = { all: 0, open: 0, merged: 0, closed: 0 }
    repositories.forEach((repository) => {
      if (repository.group.id !== summary.id) {
        return
      }
      totals.open += repository.mergeRequestCount.open
      totals.merged += repository.mergeRequestCount.merged
      totals.closed += repository.mergeRequestCount.closed
    })
    totals.all = totals.open + totals.merged + totals.closed
    return totals
  }, [repositories, summary, keyword])

  return (
    <div className="page">
      <MergeRequestToolbar
        tab={tab}
        onTabChange={changeTab}
        counts={counts}
        keyword={keywordInput}
        onKeywordChange={setKeywordInput}
        onSearch={search}
        sortDesc={sortDesc}
        onSortToggle={toggleSort}
        sortDisabled={loading}
        action={
          <Button variant="contained" onClick={() => navigate(`/groups/${groupName}/mergerequests/new`)}>
            {t('label.createMergeRequest')}
          </Button>
        }
      />

      <MergeRequestListBody
        items={items}
        loading={loading}
        noMore={noMore}
        loadingMore={loading && page > 1}
        onLoadMore={loadMore}
        hrefOf={hrefOf}
        showRepository
      />
    </div>
  )
}
