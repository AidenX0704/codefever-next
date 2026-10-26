import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export type ButtonVariant = 'contained' | 'outlined' | 'text'
export type ButtonTone = 'primary' | 'danger'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** 视觉变体；默认 `text`（与旧 MUI Button 的默认值一致 —— 实心按钮必须显式声明）。 */
  variant?: ButtonVariant
  /** 语义色：危险操作走 `danger`。 */
  tone?: ButtonTone
  size?: 'md' | 'sm' | 'lg'
  block?: boolean
  /** 提交中：自动禁用 + 显示 spinner（旧版各处手写 `disabled` + 文案切换，这里统一）。 */
  loading?: boolean
  startIcon?: ReactNode
}

/**
 * 主按钮组件。
 *
 * 尺寸/圆角/内边距/hover-active 三态取值见 `theme.css` 与 `unit.css` 注释（来源为旧
 * `@pgyer/mui-theme-preset/component/MuiButton.js`），页面不得再写自有按钮样式。
 */
export function Button({
  variant = 'text',
  tone = 'primary',
  size = 'md',
  block = false,
  loading = false,
  startIcon,
  className,
  children,
  disabled,
  type = 'button',
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        'cf-btn',
        `cf-btn--${variant}`,
        tone === 'danger' && 'cf-btn--danger',
        size === 'sm' && 'cf-btn--sm',
        size === 'lg' && 'cf-btn--lg',
        block && 'cf-btn--block',
        className,
      )}
      disabled={disabled ?? loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <span className="cf-spinner cf-btn__spinner" aria-hidden="true" /> : startIcon}
      {children === undefined || children === null ? null : <span>{children}</span>}
    </button>
  )
}
