import type { ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface TabItem<K extends string = string> {
  key: K
  label: ReactNode
  /** 计数徽标（DESIGN.md §6.6：Tab 右侧 --bg-muted 胶囊）。undefined 不显示。 */
  count?: number
}

export interface TabsProps<K extends string = string> {
  items: ReadonlyArray<TabItem<K>>
  value: K
  onChange: (key: K) => void
  className?: string
}

/** 页签：下划线式（DESIGN.md §6.6）—— 选中 --text-primary + 底部 2px --brand，
 *  Tab 栏底部 1px --line，右侧可带计数徽标。 */
export function Tabs<K extends string = string>({ items, value, onChange, className }: TabsProps<K>) {
  return (
    <div className={cn('cf-tabs', className)} role="tablist">
      {items.map((item) => (
        <button
          key={item.key}
          type="button"
          role="tab"
          aria-selected={item.key === value}
          className={cn('cf-tab', item.key === value && 'cf-tab--active')}
          onClick={() => onChange(item.key)}
        >
          {item.label}
          {item.count !== undefined ? <span className="cf-tab__count">{item.count}</span> : null}
        </button>
      ))}
    </div>
  )
}
