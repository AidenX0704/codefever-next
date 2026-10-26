import type { ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface TabItem<K extends string = string> {
  key: K
  label: ReactNode
}

export interface TabsProps<K extends string = string> {
  items: ReadonlyArray<TabItem<K>>
  value: K
  onChange: (key: K) => void
  className?: string
}

/** 页签：高 40px、padding 0 24px、选中加粗 + 2px 主色指示条（见 unit.css 注释）。 */
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
        </button>
      ))}
    </div>
  )
}
