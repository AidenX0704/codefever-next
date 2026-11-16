import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export type ButtonVariant = 'contained' | 'outlined' | 'text' | 'link'
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
  /**
   * 尾部图标（最典型的是下拉箭头 ▾）。
   *
   * 与 `startIcon` 对称：与文字之间的间距由 `.cf-btn` 的 `gap` 统一给，
   * 调用方不要自己再包 `<span>` 补 margin。作为下拉触发器时箭头会在菜单展开后
   * 旋转 180°，该规则写在 `components.css`（选择器为
   * `.cf-dropdown__trigger[aria-expanded='true'] .cf-btn__end-icon`），
   * 因此这里必须包一层 `.cf-btn__end-icon` 容器。
   */
  endIcon?: ReactNode
}

/**
 * 主按钮组件。
 *
 * 变体对应 DESIGN.md §6.1：contained=Primary、outlined=Secondary（--bg-subtle 底）、
 * text=Ghost、link=Link；危险操作用 tone="danger"（soft 变体 hover 变实心）。
 * 一个区域只允许一个 Primary（contained）。
 */
export function Button({
  variant = 'text',
  tone = 'primary',
  size = 'md',
  block = false,
  loading = false,
  startIcon,
  endIcon,
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
      {endIcon ? <span className="cf-btn__end-icon">{endIcon}</span> : null}
    </button>
  )
}
