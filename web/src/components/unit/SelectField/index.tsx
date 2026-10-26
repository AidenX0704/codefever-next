import type { ReactNode, SelectHTMLAttributes } from 'react'

import { cn } from '@/helpers/cn'

import { FieldShell } from '@/components/unit/TextField'


export interface SelectOption {
  value: string
  label: ReactNode
  disabled?: boolean
}

export interface SelectFieldProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'size' | 'children'> {
  label?: ReactNode
  required?: boolean
  error?: ReactNode | boolean
  helperText?: ReactNode
  placeholder?: string
  options: ReadonlyArray<SelectOption>
  containerClassName?: string
}

/** 下拉选择。原生 select + 令牌化外观（保留键盘与无障碍能力，不引第三方组件库）。 */
export function SelectField({
  label,
  required,
  error,
  helperText,
  placeholder,
  options,
  containerClassName,
  className,
  ...rest
}: SelectFieldProps) {
  return (
    <FieldShell
      label={label}
      required={required}
      error={error}
      helperText={helperText}
      disabled={rest.disabled}
      containerClassName={containerClassName}
      endAdornment={<span className="cf-field__caret" />}
    >
      <select className={cn('cf-field__select', className)} {...rest}>
        {placeholder ? <option value="">{placeholder}</option> : null}
        {options.map((option) => (
          <option key={option.value} value={option.value} disabled={option.disabled}>
            {option.label}
          </option>
        ))}
      </select>
    </FieldShell>
  )
}
