import type { ReactNode } from 'react'

import { plSearch } from '@/config/icons'
import { cn } from '@/helpers/cn'

import { Button } from '@/components/unit/Button'
import { Icon } from '@/components/unit/Icon'
import { Tabs } from '@/components/unit/Tabs'
import { TextField } from '@/components/unit/TextField'


export interface FilterBarTab {
  key: string
  label: ReactNode
}

export interface FilterBarProps {
  /** 页签（可选）：给了才渲染页签区。 */
  tabs?: ReadonlyArray<FilterBarTab>
  activeTab?: string
  onTabChange?: (key: string) => void
  /** 搜索框当前值。 */
  searchValue: string
  onSearchChange: (value: string) => void
  searchPlaceholder?: string
  /** 回车提交（服务端搜索的场景用；纯前端过滤不需要）。 */
  onSearchSubmit?: () => void
  /** 排序切换：true = 倒序（最新在前）。不传 `onSortToggle` 则不渲染排序按钮。 */
  sortDesc?: boolean
  onSortToggle?: () => void
  sortLabel?: ReactNode
  /** 右侧操作区（新建按钮等）。 */
  actions?: ReactNode
  className?: string
}

/**
 * 筛选工具条：页签 + 关键字搜索 + 排序 + 右侧操作。
 *
 * 旧版 `components/unit/RefFilter.js` 与 `components/view/ActivityList.js`
 * 各写了一套"页签 + 搜索框 + 排序按钮"的栅格；这里收口成一个原子件，
 * 分支列表 / 标签列表 / 活动流三处复用（AGENTS §4）。
 */
export function FilterBar({
  tabs,
  activeTab,
  onTabChange,
  searchValue,
  onSearchChange,
  searchPlaceholder,
  onSearchSubmit,
  sortDesc = true,
  onSortToggle,
  sortLabel,
  actions,
  className,
}: FilterBarProps) {
  return (
    <div className={cn('cf-filter-bar', className)}>
      {tabs && tabs.length > 0 ? (
        <Tabs
          items={tabs.map((tab) => ({ key: tab.key, label: tab.label }))}
          value={activeTab ?? tabs[0].key}
          onChange={(key) => onTabChange?.(key)}
        />
      ) : null}

      <div className="cf-filter-bar__tools">
        <TextField
          containerClassName="cf-filter-bar__search"
          placeholder={searchPlaceholder}
          value={searchValue}
          startAdornment={<Icon icon={plSearch} size={16} />}
          onChange={(event) => onSearchChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              onSearchSubmit?.()
            }
          }}
        />
        {onSortToggle ? (
          <Button variant="outlined" size="sm" onClick={onSortToggle}>
            {sortLabel}
            {sortDesc ? ' ↓' : ' ↑'}
          </Button>
        ) : null}
      </div>

      {actions ? <div className="cf-filter-bar__actions">{actions}</div> : null}
    </div>
  )
}
