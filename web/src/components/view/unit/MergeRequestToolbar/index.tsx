import type { ReactNode } from 'react'

import { Icon } from '@/components/unit/Icon'
import { IconButton } from '@/components/unit/IconButton'
import { SquareIconButton } from '@/components/unit/SquareIconButton'
import { Tabs } from '@/components/unit/Tabs'
import { TextField } from '@/components/unit/TextField'
import { plSearch, plSortDesc } from '@/config/icons'
import { useT } from '@/i18n'


/** 合并请求状态页签（0=全部）。 */
export type MergeRequestTabKey = 'all' | 'open' | 'merged' | 'closed'

/** 后端 `domain.MergeRequestStatus*`：0=全部、1=待合并、2=已合并、3=已关闭。 */
export const MERGE_REQUEST_STATUS_BY_TAB: Record<MergeRequestTabKey, number> = {
  all: 0,
  open: 1,
  merged: 2,
  closed: 3,
}

/** 各状态的合并请求数量（页签角标）。 */
export interface MergeRequestCounts {
  all: number
  open: number
  merged: number
  closed: number
}

export interface MergeRequestToolbarProps {
  tab: MergeRequestTabKey
  onTabChange: (tab: MergeRequestTabKey) => void
  /** 各状态角标；`null` = 不显示（旧版：有关键字搜索时不显示角标）。 */
  counts?: MergeRequestCounts | null
  /** 输入框里的关键字（未提交的也算）。 */
  keyword: string
  onKeywordChange: (keyword: string) => void
  /** 触发搜索：回车或点搜索按钮（旧版只在回车时触发）。 */
  onSearch: () => void
  /** true = 倒序（旧版默认 `sortDesc: true`）。 */
  sortDesc: boolean
  onSortToggle: () => void
  /** 右侧主操作（创建合并请求）。 */
  action?: ReactNode
  /** 请求进行中：禁用排序按钮（旧版 `!pending` 才允许切换排序）。 */
  sortDisabled?: boolean
}

/** 角标文案：`标签(数量)`；旧版是 `tabs[i] += '(' + count + ')'`。 */
function withCount(label: string, count: number | undefined): string {
  return count === undefined ? label : `${label}(${count})`
}

/**
 * 合并请求列表 / 看板的工具条：状态页签（带角标）+ 关键字搜索 + 排序 + 主操作。
 *
 * 旧版这段 UI 在 `components/view/MergeRequest.js` 的 `render()` 里，
 * 仓库内列表、全局看板、组内看板三处共用同一份结构（AGENTS §4：第二处即抽取）。
 */
export function MergeRequestToolbar({
  tab,
  onTabChange,
  counts,
  keyword,
  onKeywordChange,
  onSearch,
  sortDesc,
  onSortToggle,
  action,
  sortDisabled = false,
}: MergeRequestToolbarProps) {
  const t = useT()

  const items = [
    { key: 'all' as const, label: withCount(t('label.all'), counts?.all) },
    { key: 'open' as const, label: withCount(t('label.mrStatusPending'), counts?.open) },
    { key: 'merged' as const, label: withCount(t('label.mrStatusMerged'), counts?.merged) },
    { key: 'closed' as const, label: withCount(t('label.mrStatusClosed'), counts?.closed) },
  ]

  return (
    <div className="list-header">
      <Tabs<MergeRequestTabKey> value={tab} onChange={onTabChange} items={items} />
      <div className="row">
        <TextField
          placeholder={t('label.search')}
          value={keyword}
          onChange={(event) => onKeywordChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              onSearch()
            }
          }}
        />
        <IconButton label={t('label.search')} onClick={onSearch}>
          <Icon icon={plSearch} size={16} />
        </IconButton>
        <SquareIconButton
          icon={plSortDesc}
          label={t('label.sort')}
          onClick={onSortToggle}
          disabled={sortDisabled}
          className={sortDesc ? undefined : 'cf-mr-sort--asc'}
        />
        {action}
      </div>
    </div>
  )
}
