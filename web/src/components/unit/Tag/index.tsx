import type { ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export type TagTone = 'default' | 'primary' | 'success' | 'danger' | 'warning' | 'info'

export interface TagProps {
  tone?: TagTone
  children: ReactNode
  className?: string
  title?: string
}

/** 轻量标签（分支名、提交哈希、状态、计数等）。 */
export function Tag({ tone = 'default', children, className, title }: TagProps) {
  return (
    <span className={cn('cf-tag', tone !== 'default' && `cf-tag--${tone}`, className)} title={title}>
      {children}
    </span>
  )
}
