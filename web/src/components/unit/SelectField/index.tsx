import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'

import { Icon } from '@/components/unit/Icon'
import { FieldShell } from '@/components/unit/TextField'
import { caretDownIcon, checkIcon, plSearch } from '@/config/icons'
import { cn } from '@/helpers/cn'


export interface SelectOption {
  value: string
  label: ReactNode
  /** 可选辅助说明（下拉面板内第二行展示）。 */
  description?: ReactNode
  /** 可选前置图标。 */
  icon?: ReactNode
  disabled?: boolean
}

export interface SelectFieldProps {
  label?: ReactNode
  required?: boolean
  error?: ReactNode | boolean
  helperText?: ReactNode
  placeholder?: string
  options: ReadonlyArray<SelectOption>
  value?: string | number | readonly string[]
  defaultValue?: string | number | readonly string[]
  disabled?: boolean
  name?: string
  id?: string
  'aria-label'?: string
  /** 是否显示下拉内搜索框（不传时，选项数 >= 8 自动开启）。 */
  searchable?: boolean
  searchPlaceholder?: string
  emptyText?: ReactNode
  onChange?: (event: ChangeEvent<HTMLSelectElement>) => void
  onBlur?: () => void
  containerClassName?: string
  className?: string
}

interface MenuCoords {
  top: number
  left: number
  width: number
  openUp: boolean
}

function optionText(label: ReactNode): string {
  if (typeof label === 'string' || typeof label === 'number') {
    return String(label)
  }
  return ''
}

/**
 * 定制下拉选择器（替代浏览器原生 `<select>`）。
 *
 * - Portal 挂载到 `document.body`，避免被纸页/弹窗的 overflow 裁切；
 * - 自动检测视口边界向上/向下翻转，随滚动与窗口变化同步定位；
 * - 完整键盘可达性（↑ / ↓ / Home / End / Enter / Space / Esc）；
 * - 选项较多时自动支持关键字筛选；
 * - 保持与原生 `<select>` 一致的 `onChange(e)` 签名，全站调用方零改动升级。
 */
export function SelectField({
  label,
  required,
  error,
  helperText,
  placeholder,
  options,
  value,
  defaultValue,
  disabled = false,
  name,
  id,
  'aria-label': ariaLabel,
  searchable,
  searchPlaceholder = '搜索选项…',
  emptyText = '无匹配选项',
  onChange,
  onBlur,
  containerClassName,
  className,
}: SelectFieldProps) {
  const isControlled = value !== undefined
  const [innerValue, setInnerValue] = useState<string>(
    defaultValue !== undefined ? String(defaultValue) : '',
  )
  const currentValue = isControlled ? String(value ?? '') : innerValue

  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [activeIndex, setActiveIndex] = useState(-1)
  const [coords, setCoords] = useState<MenuCoords | null>(null)

  const triggerRef = useRef<HTMLButtonElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const listboxId = useId()

  const allOptions = useMemo<ReadonlyArray<SelectOption>>(() => {
    if (!placeholder) {
      return options
    }
    if (options.some((opt) => opt.value === '')) {
      return options
    }
    return [{ value: '', label: placeholder }, ...options]
  }, [options, placeholder])

  const showSearch = searchable ?? allOptions.length >= 8

  const filteredOptions = useMemo(() => {
    const kw = query.trim().toLowerCase()
    if (!kw) {
      return allOptions
    }
    return allOptions.filter((opt) => {
      const text = `${optionText(opt.label)} ${optionText(opt.description)} ${opt.value}`.toLowerCase()
      return text.includes(kw)
    })
  }, [allOptions, query])

  const selectedOption = useMemo(
    () => allOptions.find((opt) => opt.value === currentValue),
    [allOptions, currentValue],
  )

  const updatePosition = useCallback(() => {
    const trigger = triggerRef.current
    if (!trigger) {
      return
    }
    const rect = trigger.getBoundingClientRect()
    const gap = 6
    const estimatedHeight = Math.min(allOptions.length * 38 + (showSearch ? 48 : 16), 312)
    const spaceBelow = window.innerHeight - rect.bottom
    const spaceAbove = rect.top
    const openUp = spaceBelow < estimatedHeight && spaceAbove > spaceBelow
    const width = Math.max(rect.width, 168)
    const maxLeft = Math.max(8, window.innerWidth - width - 8)
    const left = Math.min(Math.max(8, rect.left), maxLeft)
    setCoords({
      top: openUp ? rect.top - gap : rect.bottom + gap,
      left,
      width,
      openUp,
    })
  }, [allOptions.length, showSearch])

  const closeMenu = useCallback(
    (restoreFocus = false) => {
      setOpen(false)
      setQuery('')
      setActiveIndex(-1)
      if (restoreFocus) {
        triggerRef.current?.focus()
      }
      onBlur?.()
    },
    [onBlur],
  )

  const selectOption = useCallback(
    (opt: SelectOption) => {
      if (opt.disabled) {
        return
      }
      if (!isControlled) {
        setInnerValue(opt.value)
      }
      if (onChange) {
        const synthetic = {
          target: { value: opt.value, name: name ?? '' },
          currentTarget: { value: opt.value, name: name ?? '' },
        } as ChangeEvent<HTMLSelectElement>
        onChange(synthetic)
      }
      closeMenu(true)
    },
    [isControlled, name, onChange, closeMenu],
  )

  useLayoutEffect(() => {
    if (!open) {
      return
    }
    updatePosition()
    const onWindowChange = () => updatePosition()
    window.addEventListener('resize', onWindowChange)
    window.addEventListener('scroll', onWindowChange, true)
    return () => {
      window.removeEventListener('resize', onWindowChange)
      window.removeEventListener('scroll', onWindowChange, true)
    }
  }, [open, updatePosition])

  useEffect(() => {
    if (!open) {
      return
    }
    const idx = filteredOptions.findIndex((opt) => opt.value === currentValue && !opt.disabled)
    setActiveIndex(idx >= 0 ? idx : filteredOptions.findIndex((opt) => !opt.disabled))
    if (showSearch) {
      window.requestAnimationFrame(() => searchRef.current?.focus())
    }
  }, [open, showSearch, filteredOptions, currentValue])

  useEffect(() => {
    if (!open) {
      return
    }
    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node
      if (triggerRef.current?.contains(target) || menuRef.current?.contains(target)) {
        return
      }
      closeMenu(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [open, closeMenu])

  const moveActive = (delta: number) => {
    if (filteredOptions.length === 0) {
      return
    }
    let next = activeIndex
    for (let step = 0; step < filteredOptions.length; step++) {
      next = (next + delta + filteredOptions.length) % filteredOptions.length
      if (!filteredOptions[next].disabled) {
        setActiveIndex(next)
        return
      }
    }
  }

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (disabled) {
      return
    }
    if (!open) {
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Enter' || event.key === ' ') {
        event.preventDefault()
        setOpen(true)
      }
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      closeMenu(true)
    } else if (event.key === 'ArrowDown') {
      event.preventDefault()
      moveActive(1)
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      moveActive(-1)
    } else if (event.key === 'Home') {
      event.preventDefault()
      const first = filteredOptions.findIndex((opt) => !opt.disabled)
      if (first >= 0) setActiveIndex(first)
    } else if (event.key === 'End') {
      event.preventDefault()
      for (let i = filteredOptions.length - 1; i >= 0; i--) {
        if (!filteredOptions[i].disabled) {
          setActiveIndex(i)
          break
        }
      }
    } else if (event.key === 'Enter') {
      event.preventDefault()
      const target = filteredOptions[activeIndex]
      if (target && !target.disabled) {
        selectOption(target)
      }
    } else if (event.key === 'Tab') {
      closeMenu(false)
    }
  }

  const isPlaceholderShown = !selectedOption || (selectedOption.value === '' && Boolean(placeholder))

  const menuNode =
    open && coords
      ? createPortal(
          <div
            ref={menuRef}
            id={listboxId}
            role="listbox"
            className={cn('cf-select-menu', coords.openUp && 'cf-select-menu--up')}
            style={{
              top: coords.top,
              left: coords.left,
              width: coords.width,
            }}
            onKeyDown={handleKeyDown}
          >
            {showSearch ? (
              <div className="cf-select-menu__search">
                <Icon icon={plSearch} size={14} />
                <input
                  ref={searchRef}
                  type="text"
                  className="cf-select-menu__search-input"
                  value={query}
                  placeholder={searchPlaceholder}
                  onChange={(event) => {
                    setQuery(event.target.value)
                    setActiveIndex(0)
                  }}
                />
              </div>
            ) : null}
            <div className="cf-select-menu__list">
              {filteredOptions.length === 0 ? (
                <div className="cf-select-menu__empty">{emptyText}</div>
              ) : (
                filteredOptions.map((option, index) => {
                  const isSelected = option.value === currentValue
                  const isActive = index === activeIndex
                  return (
                    <button
                      key={`${option.value}-${index}`}
                      type="button"
                      role="option"
                      aria-selected={isSelected}
                      disabled={option.disabled}
                      className={cn(
                        'cf-select-menu__option',
                        isSelected && 'cf-select-menu__option--selected',
                        isActive && 'cf-select-menu__option--active',
                      )}
                      onMouseEnter={() => {
                        if (!option.disabled) {
                          setActiveIndex(index)
                        }
                      }}
                      onClick={() => selectOption(option)}
                    >
                      {option.icon ? <span className="cf-select-menu__icon">{option.icon}</span> : null}
                      <span className="cf-select-menu__text">
                        <span className="cf-select-menu__label">{option.label}</span>
                        {option.description ? (
                          <span className="cf-select-menu__desc">{option.description}</span>
                        ) : null}
                      </span>
                      {isSelected ? (
                        <span className="cf-select-menu__check">
                          <Icon icon={checkIcon} size={14} />
                        </span>
                      ) : null}
                    </button>
                  )
                })
              )}
            </div>
          </div>,
          document.body,
        )
      : null

  return (
    <FieldShell
      label={label}
      required={required}
      error={error}
      helperText={helperText}
      disabled={disabled}
      containerClassName={cn('cf-select', open && 'cf-select--open', containerClassName)}
    >
      {(aria) => (
        <>
          {name ? <input type="hidden" name={name} value={currentValue} /> : null}
          <button
            ref={triggerRef}
            type="button"
            className={cn('cf-select__trigger', isPlaceholderShown && 'cf-select__trigger--placeholder', className)}
            id={id ?? aria.id}
            disabled={disabled}
            role="combobox"
            aria-controls={open ? listboxId : undefined}
            aria-expanded={open}
            aria-haspopup="listbox"
            aria-label={ariaLabel ?? (label ? undefined : placeholder)}
            aria-describedby={aria['aria-describedby']}
            aria-invalid={aria['aria-invalid']}
            onClick={() => {
              if (!disabled) {
                setOpen((prev) => !prev)
              }
            }}
            onKeyDown={handleKeyDown}
          >
            {selectedOption?.icon ? <span className="cf-select__icon">{selectedOption.icon}</span> : null}
            <span className="cf-select__value">
              {selectedOption ? selectedOption.label : placeholder ?? ''}
            </span>
            <span className="cf-select__caret" aria-hidden="true">
              <Icon icon={caretDownIcon} size={13} />
            </span>
          </button>
          {menuNode}
        </>
      )}
    </FieldShell>
  )
}
