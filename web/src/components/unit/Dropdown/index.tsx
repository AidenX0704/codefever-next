import { useEffect, useRef, useState, type ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface MenuItemSpec {
  key: string
  label: ReactNode
  icon?: ReactNode
  onClick?: () => void
  disabled?: boolean
  /** 在该项之后画一条分隔线（对应旧版菜单里的 <Divider />）。 */
  dividerAfter?: boolean
}

export interface DropdownProps {
  /** 触发元素（按钮/头像等）。点击它展开菜单。 */
  trigger: ReactNode
  items: ReadonlyArray<MenuItemSpec>
  /** 面板对齐方式：右对齐用于页头右侧的菜单。 */
  align?: 'left' | 'right'
  /** 面板宽度（默认自适应内容）。 */
  minWidth?: number
  triggerClassName?: string
  className?: string
}

/**
 * 下拉菜单（页头"新建/帮助/语言/用户"四处共用）。
 *
 * 旧版用 MUI `Menu`（Popover + 定位）；这里用「相对定位容器 + 绝对定位面板」实现，
 * 行为对齐：点击触发开合、点击外部关闭、Esc 关闭、`transitionDuration: 0` 的即时显隐。
 */
export function Dropdown({ trigger, items, align = 'left', minWidth, triggerClassName, className }: DropdownProps) {
  const [open, setOpen] = useState(false)
  const wrapRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) {
      return
    }
    const onPointerDown = (event: MouseEvent) => {
      if (!wrapRef.current?.contains(event.target as Node)) {
        setOpen(false)
      }
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  return (
    <div className={cn('cf-dropdown', className)} ref={wrapRef}>
      <div
        className={cn('cf-dropdown__trigger', triggerClassName)}
        onClick={() => setOpen((value) => !value)}
        aria-haspopup="menu"
        aria-expanded={open}
      >
        {trigger}
      </div>
      {open ? (
        <div
          className={cn('cf-menu', align === 'right' && 'cf-menu--right')}
          role="menu"
          style={minWidth ? { minWidth } : undefined}
        >
          {items.map((item) => (
            <div key={item.key}>
              <button
                type="button"
                role="menuitem"
                className="cf-menu__item"
                disabled={item.disabled}
                onClick={() => {
                  if (item.disabled) {
                    return
                  }
                  setOpen(false)
                  item.onClick?.()
                }}
              >
                {item.icon ? <span className="cf-menu__icon">{item.icon}</span> : null}
                <span className="cf-menu__label">{item.label}</span>
              </button>
              {item.dividerAfter ? <div className="cf-menu__divider" /> : null}
            </div>
          ))}
        </div>
      ) : null}
    </div>
  )
}
