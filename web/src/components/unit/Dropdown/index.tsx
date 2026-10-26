import { useEffect, useRef, useState, type ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface MenuItemSpec {
  key: string
  label: ReactNode
  icon?: ReactNode
  /** 行尾附加内容（如"当前所在"的勾选标记）。 */
  trailing?: ReactNode
  onClick?: () => void
  disabled?: boolean
  /** 在该项之后画一条分隔线（对应旧版菜单里的 <Divider />）。 */
  dividerAfter?: boolean
}

export interface DropdownProps {
  /** 触发元素（按钮/头像等）。点击它展开菜单。 */
  trigger: ReactNode
  items: ReadonlyArray<MenuItemSpec>
  /**
   * 面板顶部的小标题（如"仓库"）。
   *
   * 对齐官方原版 `GroupRepositoryMenu` 的菜单首行（caption 字号的分类名），
   * 让"这是一份什么列表"在面板内部也说清楚，而不只靠触发器文字。
   */
  title?: ReactNode
  /**
   * `items` 为空时的占位文案（如"仓库列表为空"）。
   *
   * 不传则空列表渲染成一个没有内容的空面板 —— 那在视觉上与"面板没打开"
   * 无法区分（页头的"仓库 / 仓库组"菜单踩过这个坑）。由调用方传入本地化文案。
   */
  emptyLabel?: ReactNode
  /** 面板对齐方式：右对齐用于页头右侧的菜单。 */
  align?: 'left' | 'right'
  /** 面板宽度（默认自适应内容）。 */
  minWidth?: number
  /**
   * 鼠标移入即展开、移出即收起（默认 `false` = 点击展开）。
   *
   * 官方原版页头 `GroupRepositoryMenu` 就是这个交互：悬停出菜单做快速跳转，
   * 点击触发器本身则**跳转到完整列表页**。配 `onTriggerClick` 一起用。
   * 触摸设备没有 hover，退化成"点击 → 跳转"，不会卡住。
   */
  openOnHover?: boolean
  /**
   * 触发器被点击时的动作（通常是跳到列表页）。提供后点击**不再**切换展开态 ——
   * 否则"点一下既跳转又弹菜单"，两个动作会互相打架。
   */
  onTriggerClick?: () => void
  triggerClassName?: string
  className?: string
}

/**
 * 下拉菜单（页头"仓库/仓库组/新建/帮助/语言/用户"共用）。
 *
 * 旧版用 MUI `Menu`（Popover + 定位）；这里用「相对定位容器 + 绝对定位面板」实现，
 * 行为对齐：点击触发开合、点击外部关闭、Esc 关闭、`transitionDuration: 0` 的即时显隐。
 * `openOnHover` 另提供官方原版页头那种"悬停展开"模式。
 */

/**
 * 悬停模式下离开触发器/面板后延迟收起。
 *
 * 面板与触发器之间隔着 `--cf-space-half` 的视觉间隙，指针从按钮移向面板会先触发
 * `mouseleave`；不延迟就会"一移过去就消失"（旧版 `GroupRepositoryMenu` 用的也是
 * 100ms 延迟）。这个值只影响关闭的灵敏度，不参与任何视觉尺寸。
 */
const HOVER_CLOSE_DELAY = 120

export function Dropdown({
  trigger,
  items,
  title,
  emptyLabel,
  align = 'left',
  minWidth,
  openOnHover = false,
  onTriggerClick,
  triggerClassName,
  className,
}: DropdownProps) {
  const [open, setOpen] = useState(false)
  const wrapRef = useRef<HTMLDivElement>(null)
  const closeTimerRef = useRef<number | null>(null)

  const cancelScheduledClose = () => {
    if (closeTimerRef.current !== null) {
      window.clearTimeout(closeTimerRef.current)
      closeTimerRef.current = null
    }
  }

  const scheduleClose = () => {
    cancelScheduledClose()
    closeTimerRef.current = window.setTimeout(() => {
      closeTimerRef.current = null
      setOpen(false)
    }, HOVER_CLOSE_DELAY)
  }

  // 卸载时清掉待执行的收起定时器，避免对已卸载组件 setState。
  useEffect(
    () => () => {
      if (closeTimerRef.current !== null) {
        window.clearTimeout(closeTimerRef.current)
      }
    },
    [],
  )

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
    <div
      className={cn('cf-dropdown', className)}
      ref={wrapRef}
      onMouseEnter={
        openOnHover
          ? () => {
              cancelScheduledClose()
              setOpen(true)
            }
          : undefined
      }
      onMouseLeave={openOnHover ? scheduleClose : undefined}
    >
      <div
        className={cn('cf-dropdown__trigger', triggerClassName)}
        onClick={() => {
          // 有 onTriggerClick 时点击是"跳转"，不是"开合"（官方原版的口径）。
          if (onTriggerClick) {
            cancelScheduledClose()
            setOpen(false)
            onTriggerClick()
            return
          }
          setOpen((value) => !value)
        }}
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
          {title !== undefined ? <div className="cf-menu__title">{title}</div> : null}
          {items.length === 0 && emptyLabel !== undefined ? <div className="cf-menu__empty">{emptyLabel}</div> : null}
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
                {item.trailing ? <span className="cf-menu__trailing">{item.trailing}</span> : null}
              </button>
              {item.dividerAfter ? <div className="cf-menu__divider" /> : null}
            </div>
          ))}
        </div>
      ) : null}
    </div>
  )
}
