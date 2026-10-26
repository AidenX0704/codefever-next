import type { ReactNode } from 'react'

import { emptyImages, type EmptyImageName } from '@/assets/images'
import { cn } from '@/helpers/cn'


export interface EmptyStateProps {
  /** 插图名，与旧版 `EmptyListNotice` 的 `imageName` 完全一致。 */
  imageName?: EmptyImageName
  title?: ReactNode
  notice?: ReactNode
  action?: ReactNode
  className?: string
}

/**
 * 空态组件。
 *
 * 旧版在 7 个页面里各写一遍"插图 + 标题 + 说明"，且图名以字符串 URL 拼接
 * （`/static/images/${imageName}`）；这里统一为组件 + 编译期可校验的图名枚举。
 */
export function EmptyState({ imageName, title, notice, action, className }: EmptyStateProps) {
  return (
    <div className={cn('cf-empty', className)}>
      {imageName ? <img className="cf-empty__img" src={emptyImages[imageName]} alt="" loading="lazy" /> : null}
      {title ? <div className="cf-empty__title">{title}</div> : null}
      {notice ? <div className="cf-empty__notice">{notice}</div> : null}
      {action}
    </div>
  )
}
