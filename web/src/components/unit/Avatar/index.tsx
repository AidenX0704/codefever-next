import { useState } from 'react'

import { defaultAvatar } from '@/assets/images'
import { avatarUrl } from '@/config/constants'
import { cn } from '@/helpers/cn'


export interface AvatarProps {
  /** 头像地址；为空时回退到 default_avatar.png。 */
  src?: string | null
  /** 无图时的首字母（取名称首字）。 */
  name?: string
  size?: 'sm' | 'md' | 'lg'
  square?: boolean
  className?: string
  alt?: string
}

/**
 * 头像。
 *
 * 统一处理 fallback 与 onError，页面不再关心。
 */
export function Avatar({ src, name, size = 'md', square = false, className, alt }: AvatarProps) {
  const [broken, setBroken] = useState(false)
  const resolved = !src || broken ? defaultAvatar : src

  return (
    <span
      className={cn(
        'cf-avatar',
        square && 'cf-avatar--square',
        size === 'sm' && 'cf-avatar--sm',
        size === 'lg' && 'cf-avatar--lg',
        className,
      )}
    >
      {resolved === defaultAvatar && name ? (
        <span aria-hidden="true">{name.trim().slice(0, 1)}</span>
      ) : (
        <img src={resolved} alt={alt ?? name ?? ''} loading="lazy" onError={() => setBroken(true)} />
      )}
    </span>
  )
}

export interface AvatarStackItem {
  id: string
  name: string
  icon?: string | null
}

export interface AvatarStackProps {
  items: ReadonlyArray<AvatarStackItem>
  max?: number
  className?: string
}

/**
 * 头像叠放（DESIGN.md §7.2 AvatarStack）：
 * 重叠 -6px、外缘 2px var(--bg-sheet) 描边，最多展示 max 个 + `+N` 溢出芯片。
 */
export function AvatarStack({ items, max = 5, className }: AvatarStackProps) {
  if (items.length === 0) {
    return null
  }
  const visible = items.slice(0, max)
  const overflow = items.length - visible.length
  const overflowNames = items
    .slice(max)
    .map((m) => m.name)
    .join(', ')

  return (
    <span className={cn('cf-avatar-stack', className)}>
      {visible.map((member) => (
        <span className="cf-avatar-stack__item" key={member.id} title={member.name}>
          {member.icon ? (
            <img src={avatarUrl(member.icon)} alt={member.name} loading="lazy" />
          ) : (
            member.name.trim().slice(0, 1).toUpperCase()
          )}
        </span>
      ))}
      {overflow > 0 ? (
        <span className="cf-avatar-stack__more" title={overflowNames}>
          +{overflow}
        </span>
      ) : null}
    </span>
  )
}
