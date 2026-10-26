import type { InputHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: ReactNode
}

/** 复选框（原生 input + accent-color，保留键盘/读屏能力）。 */
export function Checkbox({ label, className, ...rest }: CheckboxProps) {
  return (
    <label className={cn('cf-check', className)}>
      <input type="checkbox" {...rest} />
      {label ? <span>{label}</span> : null}
    </label>
  )
}

export interface SwitchProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: ReactNode
}

/** 开关（用于通知设置等布尔项）。 */
export function Switch({ label, className, ...rest }: SwitchProps) {
  return (
    <label className={cn('cf-switch', className)}>
      <input type="checkbox" className="cf-switch__input" {...rest} />
      <span className="cf-switch__track" aria-hidden="true" />
      {label ? <span>{label}</span> : null}
    </label>
  )
}
