import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** 必填：纯图标按钮必须有可读标签（a11y，同时作为 tooltip 文本）。 */
  label: string
  bordered?: boolean
  children: ReactNode
}

export function IconButton({ label, bordered = false, className, children, type = 'button', ...rest }: IconButtonProps) {
  return (
    <button
      type={type}
      aria-label={label}
      className={cn('cf-icon-btn', bordered && 'cf-icon-btn--bordered', className)}
      {...rest}
    >
      {children}
    </button>
  )
}
