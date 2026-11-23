import type { ReactNode } from 'react'

import { closeIcon, plSearch } from '@/config/icons'
import { cn } from '@/helpers/cn'

import { Button } from '@/components/unit/Button'
import { Icon } from '@/components/unit/Icon'
import { SelectField, type SelectOption } from '@/components/unit/SelectField'
import { Tabs } from '@/components/unit/Tabs'
import { TextField } from '@/components/unit/TextField'


export interface FilterBarTab {
  key: string
  label: ReactNode
  count?: number
}

export interface FilterBarSelect {
  key: string
  value: string
  options: ReadonlyArray<SelectOption>
  onChange: (value: string) => void
  ariaLabel?: string
  className?: string
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
  /** 下拉筛选项（例如仓库组筛选、排序字段选择）。 */
  selects?: ReadonlyArray<FilterBarSelect>
  /** 排序切换：true = 倒序（最新在前）。不传 `onSortToggle` 则不渲染排序按钮。 */
  sortDesc?: boolean
  onSortToggle?: () => void
  sortLabel?: ReactNode
  /** 视图切换或额外右侧工具件（Segmented 按钮组等）。 */
  extraTools?: ReactNode
  /** 右侧操作区（新建按钮等）。 */
  actions?: ReactNode
  className?: string
}

/**
 * 筛选工具条：页签 + 关键字搜索 + 下拉筛选 + 排序 + 视图切换 + 右侧操作。
 *
 * 仓库列表 / 仓库组列表 / 分支列表 / 标签列表 / 活动流共用（AGENTS §4）。
 */
export function FilterBar({
  tabs,
  activeTab,
  onTabChange,
  searchValue,
  onSearchChange,
  searchPlaceholder,
  onSearchSubmit,
  selects,
  sortDesc = true,
  onSortToggle,
  sortLabel,
  extraTools,
  actions,
  className,
}: FilterBarProps) {
  return (
    <div className={cn('cf-filter-bar', className)}>
      {tabs && tabs.length > 0 ? (
        <Tabs
          items={tabs.map((tab) => ({ key: tab.key, label: tab.label, count: tab.count }))}
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
          endAdornment={
            searchValue ? (
              <button
                type="button"
                className="cf-filter-bar__clear"
                aria-label="清空"
                onClick={() => {
                  onSearchChange('')
                  onSearchSubmit?.()
                }}
              >
                <Icon icon={closeIcon} size={14} />
              </button>
            ) : undefined
          }
          onChange={(event) => onSearchChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              onSearchSubmit?.()
            }
          }}
        />
        {selects?.map((sel) => (
          <SelectField
            key={sel.key}
            containerClassName={cn('cf-filter-bar__select', sel.className)}
            options={sel.options}
            value={sel.value}
            aria-label={sel.ariaLabel}
            onChange={(event) => sel.onChange(event.target.value)}
          />
        ))}
        {onSortToggle ? (
          <Button variant="outlined" size="sm" onClick={onSortToggle}>
            {sortLabel}
            {sortDesc ? ' ↓' : ' ↑'}
          </Button>
        ) : null}
        {extraTools}
      </div>

      {actions ? <div className="cf-filter-bar__actions">{actions}</div> : null}
    </div>
  )
}
