import { cn } from '@/helpers/cn'

export interface ListSkeletonProps {
  rows?: number
  className?: string
}

/**
 * 行式列表骨架屏（DESIGN.md §6 G2：5 行骨架，避免白屏跳动）。
 * 所有配色来自 --bg-well / --line 语义令牌。
 */
export function ListSkeleton({ rows = 5, className }: ListSkeletonProps) {
  return (
    <div className={cn('cf-skeleton-list', className)} aria-busy="true" aria-live="polite">
      {Array.from({ length: rows }, (_, idx) => (
        <div className="cf-skeleton-row" key={idx}>
          <span className="cf-skeleton__box cf-skeleton__avatar" />
          <div className="cf-skeleton__body">
            <span className="cf-skeleton__box cf-skeleton__title" />
            <span className="cf-skeleton__box cf-skeleton__desc" />
          </div>
          <span className="cf-skeleton__box cf-skeleton__meta" />
        </div>
      ))}
    </div>
  )
}
