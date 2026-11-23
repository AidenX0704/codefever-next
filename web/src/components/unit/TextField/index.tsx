import { useId, useState, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from 'react'

import { Icon } from '@/components/unit/Icon'
import { eyeIcon, eyeSlashIcon } from '@/config/icons'
import { cn } from '@/helpers/cn'
import { useT } from '@/i18n'


export type FieldSize = 'md' | 'lg'

interface FieldShellProps {
  label?: ReactNode
  /** 标签行右侧辅助节点（如登录表单密码栏的「忘记密码？」链接）。 */
  labelEnd?: ReactNode
  required?: boolean
  error?: ReactNode | boolean
  helperText?: ReactNode
  disabled?: boolean
  size?: FieldSize
  startAdornment?: ReactNode
  endAdornment?: ReactNode
  containerClassName?: string
}

/** 控件需要补到自身上的无障碍属性（由外壳算好回传）。 */
export interface FieldAriaProps {
  id: string
  'aria-describedby'?: string
  'aria-invalid'?: true
}

/**
 * 输入框外壳：标签 + 边框盒 + 帮助/错误文案，Input / Password / Textarea / Select 共用。
 *
 * 无障碍：
 *   - 控件与 `<label htmlFor={inputId}>` 显式关联（避免 labelEnd 内的链接或密码切换按钮被
 *     外层 `<label>` 劫持点击）；
 *   - 帮助 / 错误文案生成 id，通过 `aria-describedby` 指给控件；
 *   - 有错误时给控件 `aria-invalid`。
 */
export function FieldShell({
  label,
  labelEnd,
  required,
  error,
  helperText,
  disabled,
  size = 'md',
  startAdornment,
  endAdornment,
  containerClassName,
  children,
}: FieldShellProps & { children: (aria: FieldAriaProps) => ReactNode }) {
  const inputId = useId()
  const helpId = useId()
  const hasError = Boolean(error)
  const hasHelp = Boolean(error || helperText)
  const aria: FieldAriaProps = {
    id: inputId,
    'aria-describedby': hasHelp ? helpId : undefined,
    'aria-invalid': hasError ? true : undefined,
  }
  return (
    <div
      className={cn(
        'cf-field',
        size === 'lg' && 'cf-field--lg',
        hasError && 'cf-field--error',
        disabled && 'cf-field--disabled',
        containerClassName,
      )}
    >
      {label || labelEnd ? (
        <div className="cf-field__head">
          {label ? (
            <label className="cf-field__label" htmlFor={inputId}>
              {label}
              {required ? <span className="cf-field__required">*</span> : null}
            </label>
          ) : (
            <span />
          )}
          {labelEnd ? <div className="cf-field__label-end">{labelEnd}</div> : null}
        </div>
      ) : null}
      <div className="cf-field__box">
        {startAdornment ? <span className="cf-field__adornment">{startAdornment}</span> : null}
        {children(aria)}
        {endAdornment ? <span className="cf-field__adornment">{endAdornment}</span> : null}
      </div>
      {hasHelp ? (
        <span className="cf-field__help" id={helpId} role={hasError ? 'alert' : undefined}>
          {error ?? helperText}
        </span>
      ) : null}
    </div>
  )
}

export interface TextFieldProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'>,
    Omit<FieldShellProps, 'children' | 'disabled'> {}

/** 单行输入框。 */
export function TextField({
  label,
  labelEnd,
  required,
  error,
  helperText,
  size,
  startAdornment,
  endAdornment,
  containerClassName,
  className,
  id,
  ...rest
}: TextFieldProps) {
  return (
    <FieldShell
      label={label}
      labelEnd={labelEnd}
      required={required}
      error={error}
      helperText={helperText}
      disabled={rest.disabled}
      size={size}
      startAdornment={startAdornment}
      endAdornment={endAdornment}
      containerClassName={containerClassName}
    >
      {(aria) => (
        <input
          className={cn('cf-field__input', className)}
          {...aria}
          id={id ?? aria.id}
          aria-label={label ? undefined : rest.placeholder}
          {...rest}
        />
      )}
    </FieldShell>
  )
}

export type PasswordFieldProps = Omit<TextFieldProps, 'type' | 'endAdornment'>

/** 带显隐切换的密码输入框（登录 / 注册 / 重置口令共用，AGENTS §4）。 */
export function PasswordField(props: PasswordFieldProps) {
  const t = useT()
  const [visible, setVisible] = useState(false)

  return (
    <TextField
      {...props}
      type={visible ? 'text' : 'password'}
      endAdornment={
        <button
          type="button"
          className="cf-field__toggle"
          aria-label={visible ? t('auth.hidePassword') : t('auth.showPassword')}
          aria-pressed={visible}
          onClick={() => setVisible((prev) => !prev)}
        >
          <Icon icon={visible ? eyeSlashIcon : eyeIcon} size={16} />
        </button>
      }
    />
  )
}

export interface TextareaFieldProps
  extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'size'>,
    Omit<FieldShellProps, 'children' | 'disabled'> {}

/** 多行输入框（与单行共用外壳，仅控件换成 textarea）。 */
export function TextareaField({
  label,
  labelEnd,
  required,
  error,
  helperText,
  size,
  containerClassName,
  className,
  id,
  ...rest
}: TextareaFieldProps) {
  return (
    <FieldShell
      label={label}
      labelEnd={labelEnd}
      required={required}
      error={error}
      helperText={helperText}
      disabled={rest.disabled}
      size={size}
      containerClassName={containerClassName}
    >
      {(aria) => (
        <textarea
          className={cn('cf-field__input', className)}
          {...aria}
          id={id ?? aria.id}
          aria-label={label ? undefined : rest.placeholder}
          {...rest}
        />
      )}
    </FieldShell>
  )
}
