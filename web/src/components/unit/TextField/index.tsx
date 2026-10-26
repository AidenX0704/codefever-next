import type { InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from 'react'

import { cn } from '@/helpers/cn'


/**
 * 输入框外观变体：
 *   - `outlined`：应用内表单（旧 MuiOutlinedInput，边框盒 + 圆角 4）；
 *   - `underline`：认证页（旧 SSR 页 `.login-wrap input`，只留底边框、无圆角、大号字）。
 * 两套外观都由组件库统一提供，页面不得自绘输入框。
 */
export type FieldVariant = 'outlined' | 'underline'

interface FieldShellProps {
  label?: ReactNode
  required?: boolean
  error?: ReactNode | boolean
  helperText?: ReactNode
  disabled?: boolean
  variant?: FieldVariant
  startAdornment?: ReactNode
  endAdornment?: ReactNode
  containerClassName?: string
}

/** 输入框外壳：标签 + 边框盒 + 帮助/错误文案，Input / Textarea / Select 共用。 */
export function FieldShell({
  label,
  required,
  error,
  helperText,
  disabled,
  variant = 'outlined',
  startAdornment,
  endAdornment,
  containerClassName,
  children,
}: FieldShellProps & { children: ReactNode }) {
  const hasError = Boolean(error)
  return (
    <label
      className={cn(
        'cf-field',
        variant === 'underline' && 'cf-field--underline',
        hasError && 'cf-field--error',
        disabled && 'cf-field--disabled',
        containerClassName,
      )}
    >
      {label ? (
        <span className="cf-field__label">
          {label}
          {required ? <span className="cf-field__required">*</span> : null}
        </span>
      ) : null}
      <span className="cf-field__box">
        {startAdornment ? <span className="cf-field__adornment">{startAdornment}</span> : null}
        {children}
        {endAdornment ? <span className="cf-field__adornment">{endAdornment}</span> : null}
      </span>
      {error || helperText ? <span className="cf-field__help">{error ?? helperText}</span> : null}
    </label>
  )
}

export interface TextFieldProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'>,
    Omit<FieldShellProps, 'children' | 'disabled'> {}

/** 单行输入框。视觉：border #dee2ec、圆角 4、padding 6.5px 16px、行高 19px。 */
export function TextField({
  label,
  required,
  error,
  helperText,
  variant,
  startAdornment,
  endAdornment,
  containerClassName,
  className,
  ...rest
}: TextFieldProps) {
  return (
    <FieldShell
      label={label}
      required={required}
      error={error}
      helperText={helperText}
      disabled={rest.disabled}
      variant={variant}
      startAdornment={startAdornment}
      endAdornment={endAdornment}
      containerClassName={containerClassName}
    >
      <input className={cn('cf-field__input', className)} {...rest} />
    </FieldShell>
  )
}

export interface TextareaFieldProps
  extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'size'>,
    Omit<FieldShellProps, 'children' | 'disabled'> {}

/** 多行输入框（与单行共用外壳，仅控件换成 textarea）。 */
export function TextareaField({
  label,
  required,
  error,
  helperText,
  variant,
  containerClassName,
  className,
  ...rest
}: TextareaFieldProps) {
  return (
    <FieldShell
      label={label}
      required={required}
      error={error}
      helperText={helperText}
      disabled={rest.disabled}
      variant={variant}
      containerClassName={containerClassName}
    >
      <textarea className={cn('cf-field__input', className)} {...rest} />
    </FieldShell>
  )
}
