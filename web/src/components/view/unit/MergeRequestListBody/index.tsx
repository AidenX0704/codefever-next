import type { MergeRequestListItem } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { Loading } from '@/components/unit/Loading'
import { MergeRequestRow } from '@/components/view/unit/MergeRequestRow'
import { useT } from '@/i18n'


export interface MergeRequestListBodyProps {
  items: MergeRequestListItem[]
  /** 正在请求（首屏或翻页）。 */
  loading: boolean
  /** 没有更多了（旧版判据：本次返回条数 < pagesize）。 */
  noMore: boolean
  /** 正在加载下一页。 */
  loadingMore?: boolean
  onLoadMore?: () => void
  /** 详情页链接（全局/组内看板按目标仓库拼）。 */
  hrefOf: (item: MergeRequestListItem) => string
  /** 是否显示目标仓库前缀（仓库内列表不需要）。 */
  showRepository?: boolean
}

/**
 * 合并请求列表主体：行列表 + 空态 + "查看更多"。
 *
 * 逐字对齐旧版 `components/view/MergeRequest.js` 的列表区：
 *   - 空列表 → `EmptyListNotice`（插图 `mergerequests-empty.png`）；
 *   - 列表尾部 → "查看更多"按钮；没有更多时按钮置灰显示"没有更多"；
 *   - 翻页中显示加载指示（不再显示按钮）。
 */
export function MergeRequestListBody({
  items,
  loading,
  noMore,
  loadingMore = false,
  onLoadMore,
  hrefOf,
  showRepository = false,
}: MergeRequestListBodyProps) {
  const t = useT()

  if (loading && items.length === 0) {
    return <Loading />
  }

  if (!loading && items.length === 0) {
    return <EmptyState imageName="mergerequests-empty.png" title={t('message._S_listEmpty', { s: t('label.mergeRequest_pl') })} />
  }

  return (
    <>
      <div className="list-body">
        {items.map((item) => (
          <MergeRequestRow key={item.id} item={item} href={hrefOf(item)} showRepository={showRepository} />
        ))}
      </div>
      <div className="cf-mr-more">
        {noMore ? (
          <Button size="sm" disabled>
            {t('label.noMore')}
          </Button>
        ) : loadingMore ? (
          <Loading />
        ) : (
          <Button size="sm" variant="contained" onClick={onLoadMore}>
            {t('label.more')}
          </Button>
        )}
      </div>
    </>
  )
}
