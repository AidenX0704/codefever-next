import type { ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export type MarkerVariant = 'info' | 'containedInfo'

export interface MarkerProps {
  text: ReactNode
  variant?: MarkerVariant
  className?: string
}

/**
 * 行内标记（角色徽标等）。
 *
 * 对应旧版 `InlineMarker`：padding 4px、字号 12px、行高 1.5、圆角 4px；
 * `info` = 次色文字 + 次色浅底，`containedInfo` = 白字 + 次色实底
 * （旧版由 `theme.palette.secondary` 与 `primary.contrastText` 组合而来）。
 */
export function Marker({ text, variant = 'info', className }: MarkerProps) {
  return <span className={cn('cf-marker', `cf-marker--${variant}`, className)}>{text}</span>
}
