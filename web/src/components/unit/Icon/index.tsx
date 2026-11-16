import type { IconProps as PhosphorIconProps } from '@phosphor-icons/react'
import { cn } from '@/helpers/cn'

export type { IconProps as PhosphorIconProps }
export type { IconComponent } from '@/config/icons'

export interface IconProps {
  /** Phosphor 图标组件（经 `@/config/icons` 白名单导入）。 */
  icon: React.ComponentType<PhosphorIconProps>
  /** 视觉尺寸（px）；默认 18px（页头/行内口径）。 */
  size?: number
  /** 字重：regular（默认）/ duotone（空态、功能介绍）/ fill（选中态）。 */
  weight?: PhosphorIconProps['weight']
  className?: string
  /** 传入即视为有语义（role="img" + aria-label），否则对读屏隐藏。 */
  title?: string
}

/**
 * 图标渲染器（Phosphor 包装层）。
 *
 * 颜色用 currentColor，继承文字颜色（hover 变色不需要额外交互逻辑）。
 * 注意：图标一律从 `@/config/icons` 导入 —— 那是白名单，直接引
 * `@phosphor-icons/react` 会绕过使用面管理（AGENTS §9 同款约束）。
 */
export function Icon({ icon: Glyph, size = 18, weight, className, title }: IconProps) {
  return (
    <Glyph
      className={cn('cf-icon', className)}
      size={size}
      weight={weight}
      role={title ? 'img' : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
    />
  )
}
