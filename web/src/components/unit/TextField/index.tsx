import { useId, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from 'react'

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

/** 控件需要补到自身上的无障碍属性（由外壳算好回传）。 */
export interface FieldAriaProps {
  'aria-describedby'?: string
  'aria-invalid'?: true
}

/**
 * 输入框外壳：标签 + 边框盒 + 帮助/错误文案，Input / Textarea / Select 共用。
 *
 * 无障碍（V3 §10）：
 *   - 外壳本身是 `<label>`，控件放在里面，标签天然关联；
 *   - 帮助 / 错误文案生成 id，通过 `aria-describedby` 指给控件 —— 读屏聚焦时会
 *     读出"请输入至少 6 位密码"，而不是只看到一个红框；
 *   - 有错误时给控件 `aria-invalid`。
 * `children` 因此是渲染函数：外壳算好属性再交给控件，避免每个调用方各写一遍。
 */
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
}: FieldShellProps & { children: (aria: FieldAriaProps) => ReactNode }) {
  const helpId = useId()
  const hasError = Boolean(error)
  const hasHelp = Boolean(error || helperText)
  const aria: FieldAriaProps = {
    'aria-describedby': hasHelp ? helpId : undefined,
    'aria-invalid': hasError ? true : undefined,
  }
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
        {children(aria)}
        {endAdornment ? <span className="cf-field__adornment">{endAdornment}</span> : null}
      </span>
      {hasHelp ? (
        <span className="cf-field__help" id={helpId}>
          {error ?? helperText}
        </span>
      ) : null}
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
      {(aria) => (
        <input
          className={cn('cf-field__input', className)}
          {...aria}
          aria-label={label ? undefined : rest.placeholder}
          {...rest}
        />
      )}
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
      {(aria) => (
        <textarea
          className={cn('cf-field__input', className)}
          {...aria}
          aria-label={label ? undefined : rest.placeholder}
          {...rest}
        />
      )}
    </FieldShell>
  )
}
