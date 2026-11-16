import type { ReactNode } from 'react'

import { emptyImages, type EmptyImageName } from '@/assets/images'
import { cn } from '@/helpers/cn'
import { Icon } from '@/components/unit/Icon'
import type { IconComponent } from '@/config/icons'


export interface EmptyStateProps {
  /** duotone 图标（DESIGN.md §6.10：64px，优先于旧插图）。 */
  icon?: IconComponent
  /** 旧版插图名（v3 兼容；与新 `icon` 同时给时以 `icon` 为准）。 */
  imageName?: EmptyImageName
  title?: ReactNode
  notice?: ReactNode
  /** 空态只放一个按钮（DESIGN.md §6.10）——调用方负责只传一个动作。 */
  action?: ReactNode
  className?: string
}

/**
 * 空态组件。
 *
 * DESIGN.md §6.10：居中，64px duotone 图标 + H3 标题 + Small 描述 + 一个按钮；
 * 不展示无意义的跳转链接。旧版"全部 →"描边按钮类空态在页面改造时移除。
 */
export function EmptyState({ icon, imageName, title, notice, action, className }: EmptyStateProps) {
  return (
    <div className={cn('cf-empty', className)}>
      {icon ? (
        <span className="cf-empty__icon">
          <Icon icon={icon} size={64} weight="duotone" />
        </span>
      ) : imageName ? (
        <img className="cf-empty__img" src={emptyImages[imageName]} alt="" loading="lazy" />
      ) : null}
      {title ? <div className="cf-empty__title">{title}</div> : null}
      {notice ? <div className="cf-empty__notice">{notice}</div> : null}
      {action}
    </div>
  )
}
