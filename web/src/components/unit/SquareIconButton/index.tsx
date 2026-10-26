import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/helpers/cn'
import { Icon, type IconData } from '@/components/unit/Icon'
import { Tooltip } from '@/components/unit/Tooltip'


export interface SquareIconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  icon: IconData
  /** 工具提示文本（同时也是 aria-label）。 */
  label: string
  /** 未读角标数字（0 不显示；>99 显示为 …，与旧版一致）。 */
  badge?: number
}

/**
 * 页头方形图标按钮。
 *
 * 对应旧版 `SquareIconButton`：`minWidth: spacing(3)`、`padding: 7px 8px`、
 * 图标 18px、Tooltip 顶部、未读角标 `#FF3F35` 且 >99 时不显示数字。
 */
export function SquareIconButton({ icon, label, badge = 0, className, type = 'button', ...rest }: SquareIconButtonProps) {
  const button: ReactNode = (
    <button type={type} className={cn('cf-square-btn', className)} aria-label={label} {...rest}>
      <Icon icon={icon} size={18} />
      {badge > 0 ? <span className="cf-square-btn__badge">{badge > 99 ? '' : badge}</span> : null}
    </button>
  )

  return (
    <Tooltip title={label} placement="top">
      {button}
    </Tooltip>
  )
}
