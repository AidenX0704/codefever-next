import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { fetchMergeRequests } from '@/api/repository'
import type { MergeRequestListItem } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { useToast } from '@/components/unit/Toast'
import { MergeRequestListBody } from '@/components/view/unit/MergeRequestListBody'
import {
  MERGE_REQUEST_STATUS_BY_TAB,
  MergeRequestToolbar,
} from '@/components/view/unit/MergeRequestToolbar'
import type { MergeRequestCounts, MergeRequestTabKey } from '@/components/view/unit/MergeRequestToolbar'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'


/** 旧版每页 10 条（`MergeRequest.js` 的 `pagesize: 10`）。 */
const PAGE_SIZE = 10

/**
 * 合并请求列表（`/{组}/{仓库}/mergerequests`）。
 *
 * `repository/mergeRequests?repository&status&keyword&sort&page&pagesize`：
 * `status` 0=全部、1=待合并、2=已合并、3=已关闭（后端 `MergeRequestStatus*`）。
 *
 * 逐条对齐旧版 `components/view/MergeRequest.js`：
 *   - 默认页签是**全部**（旧版 `tabValue: 0`）；
 *   - 页签带数量角标（仓库内取 `summary.mergeRequestCount`；有关键字时不显示）；
 *   - 关键字搜索**回车或点搜索按钮才触发**（旧版 `searchMergeRequests()`）；
 *   - 排序可切换 asc/desc（旧版 `sortDesc` + 排序图标，默认 desc）；
 *   - 每页 10 条 + "查看更多"追加下一页（本次返回 < pagesize 即没有更多）。
 */
export default function MergeRequestList() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { groupName, repositoryName } = useParams()
  const { summary } = useRepository()

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

  const base = `/${groupName}/${repositoryName}`

  useEffect(() => {
    if (!summary) {
      return
    }
    let cancelled = false
    setLoading(true)
    fetchMergeRequests({
      repositoryKey: summary.id,
      status: MERGE_REQUEST_STATUS_BY_TAB[tab],
      keyword,
      sort: sortDesc ? 'desc' : 'asc',
      page,
      pagesize: PAGE_SIZE,
    })
      .then((list) => {
        if (cancelled) {
          return
        }
        setItems((previous) => (page === 1 ? list : [...previous, ...list]))
        setNoMore(list.length < PAGE_SIZE)
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
  }, [summary, tab, keyword, sortDesc, page, t, toast])

  /** 角标：仓库内直接用 `mergeRequestCount`；有关键字搜索时旧版不显示角标。 */
  const counts = useMemo<MergeRequestCounts | null>(() => {
    if (keyword || !summary) {
      return null
    }
    const count = summary.mergeRequestCount
    return {
      all: count.open + count.merged + count.closed,
      open: count.open,
      merged: count.merged,
      closed: count.closed,
    }
  }, [summary, keyword])

  /** 重新取数：回到第一页并清空累积列表（先置 loading，避免闪一下空态）。 */
  const reset = () => {
    setPage(1)
    setItems([])
    setLoading(true)
  }

  const changeTab = (next: MergeRequestTabKey) => {
    if (next === tab) {
      return
    }
    setTab(next)
    reset()
  }

  const toggleSort = () => {
    setSortDesc((previous) => !previous)
    reset()
  }

  const search = () => {
    if (keywordInput === keyword) {
      return
    }
    setKeyword(keywordInput)
    reset()
  }

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
          <Button variant="contained" onClick={() => navigate(`${base}/mergerequests/new`)}>
            {t('label.createMergeRequest')}
          </Button>
        }
      />

      <MergeRequestListBody
        items={items}
        loading={loading}
        noMore={noMore}
        loadingMore={loading && page > 1}
        onLoadMore={() => setPage((previous) => previous + 1)}
        hrefOf={(item) => `${base}/mergerequests/${item.number}`}
      />
    </div>
  )
}
