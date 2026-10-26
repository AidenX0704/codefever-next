import type { ReactNode } from 'react'

import { UI_CONFIG } from '@/config/ui'
import { cn } from '@/helpers/cn'


export interface TooltipProps {
  title: ReactNode
  placement?: 'top' | 'bottom'
  children: ReactNode
  className?: string
}

/**
 * 提示气泡（纯 CSS 实现，无 JS 定位）。
 *
 * 视觉取值：底色 `text`、12px/13px、padding 6px 16px、12×12 旋转 45° 箭头
 * （来源 `@pgyer/mui-theme-preset/component/MuiTooltip.js`）。
 */
export function Tooltip({ title, placement = 'top', children, className }: TooltipProps) {
  // 全局关闭时不渲染气泡（`aria-label` 由各按钮自己提供，无障碍信息不丢）。
  if (!UI_CONFIG.tooltipsEnabled) {
    return <>{children}</>
  }

  return (
    <span className={cn('cf-tooltip-wrap', className)}>
      {children}
      <span role="tooltip" className={cn('cf-tooltip', `cf-tooltip--${placement}`)}>
        {title}
      </span>
    </span>
  )
}
