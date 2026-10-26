import { useState } from 'react'

import { defaultAvatar } from '@/assets/images'
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
 * 旧版各自 `src || '/static/images/default_avatar.png'`，且失败时不兜底；
 * 这里统一处理 fallback 与 onError，页面不再关心。
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
