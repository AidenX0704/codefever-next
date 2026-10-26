import type { HTMLAttributes, ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  /** 列表卡片：hover 抬升阴影（旧仓库卡片行为）。 */
  hoverable?: boolean
  children: ReactNode
}

export function Card({ hoverable = false, className, children, ...rest }: CardProps) {
  return (
    <div className={cn('cf-card', hoverable && 'cf-card--hoverable', className)} {...rest}>
      {children}
    </div>
  )
}
