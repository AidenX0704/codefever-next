import { useState } from 'react'

import { avatarUrl } from '@/config/constants'
import { cn } from '@/helpers/cn'

export interface GroupAvatarProps {
  icon?: string | null
  name: string
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

/**
 * 仓库组头像（DESIGN.md v5 §7.2）：
 * 有图标显示圆角方图；无图标统一渲染中央 2×2 小方格标，一眼区分「组」与「仓库」。
 */
export function GroupAvatar({ icon, name, size = 'md', className }: GroupAvatarProps) {
  const [broken, setBroken] = useState(false)

  if (icon && !broken) {
    return (
      <span className={cn('group-avatar', `group-avatar--${size}`, className)}>
        <img src={avatarUrl(icon)} alt={name} loading="lazy" onError={() => setBroken(true)} />
      </span>
    )
  }

  return (
    <span className={cn('group-avatar', 'group-avatar--mark', `group-avatar--${size}`, className)} aria-hidden="true">
      <i />
      <i />
      <i />
      <i />
    </span>
  )
}
