import type { ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface TimelineItem {
  key: string | number
  /** 日期（Caption，一般传 `yyyy-MM-dd`）。 */
  date: ReactNode
  /** 标题（H3）。 */
  title: ReactNode
  /** 一行摘要（Small）。 */
  content?: ReactNode
  /** 版本号等 Tag 形态的附加信息。 */
  tag?: ReactNode
  /** 最新一条：节点用 --brand，其余 --text-tertiary。 */
  latest?: boolean
}

export interface TimelineProps {
  items: ReadonlyArray<TimelineItem>
  className?: string
}

/**
 * 时间轴（DESIGN.md §6.15）：左侧 1px --line 竖线，节点 8px 实心圆，
 * 日期 Caption、标题 H3、内容 Small；版本号用 Tag 展示。更新日志 / 活动时间线复用。
 */
export function Timeline({ items, className }: TimelineProps) {
  return (
    <ol className={cn('cf-timeline', className)}>
      {items.map((item) => (
        <li key={item.key} className="cf-timeline__item">
          <span className={cn('cf-timeline__dot', item.latest && 'cf-timeline__dot--latest')} aria-hidden="true" />
          <div className="cf-timeline__body">
            <div className="cf-timeline__meta">
              <span className="cf-timeline__date">{item.date}</span>
              {item.tag ? <span className="cf-timeline__tag">{item.tag}</span> : null}
            </div>
            <div className="cf-timeline__title">{item.title}</div>
            {item.content ? <div className="cf-timeline__content">{item.content}</div> : null}
          </div>
        </li>
      ))}
    </ol>
  )
}
