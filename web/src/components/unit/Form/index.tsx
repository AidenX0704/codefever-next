import type { FormEvent, FormHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface FormProps extends Omit<FormHTMLAttributes<HTMLFormElement>, 'onSubmit'> {
  onSubmit?: (event: FormEvent<HTMLFormElement>) => void
}

/**
 * 表单外壳：统一 `preventDefault` + 纵向间距节奏。
 *
 * 旧版每个页面各写一套表单栅格与间距，这里固定为"表单项间距 16px、
 * 双列栅格 16px、操作区右对齐 8px"（与 preset 的 spacing 节奏一致）。
 */
export function Form({ onSubmit, className, children, ...rest }: FormProps) {
  return (
    <form
      className={cn('cf-form', className)}
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit?.(event)
      }}
      {...rest}
    >
      {children}
    </form>
  )
}

export interface FormItemProps {
  label?: ReactNode
  required?: boolean
  error?: ReactNode
  help?: ReactNode
  children: ReactNode
  className?: string
}

/** 表单行：标签 + 控件 + 错误/帮助文案（错误优先）。 */
export function FormItem({ label, required, error, help, children, className }: FormItemProps) {
  return (
    <div className={cn('cf-field', error ? 'cf-field--error' : null, className)}>
      {label ? (
        <span className="cf-field__label">
          {label}
          {required ? <span className="cf-field__required">*</span> : null}
        </span>
      ) : null}
      {children}
      {error || help ? <span className="cf-field__help">{error ?? help}</span> : null}
    </div>
  )
}

/** 双列表单栅格（窄屏由容器自行降级，见使用者页面）。 */
export function FormGrid({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('cf-form__grid', className)}>{children}</div>
}

/** 操作区：右对齐按钮组。 */
export function FormActions({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('cf-form__actions', className)}>{children}</div>
}
