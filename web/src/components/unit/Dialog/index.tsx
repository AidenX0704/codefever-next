import { useCallback, useEffect, useId, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

import { cn } from '@/helpers/cn'


export interface DialogProps {
  open: boolean
  title?: ReactNode
  children?: ReactNode
  /** 底部操作区（一般是 <Button> 组合）。 */
  actions?: ReactNode
  onClose?: () => void
  closeOnBackdrop?: boolean
  className?: string
}

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

/**
 * 模态框。
 *
 * 统一处理（V3 §6.5）：
 *   - Esc 关闭、遮罩点击关闭、body 滚动锁定、portal 挂载到 body；
 *   - **打开时焦点进入弹窗**，关闭后**焦点回到触发元素** —— 否则键盘用户
 *     关掉弹窗后焦点会掉到页面顶部，得重新 Tab 一遍才能回到原处；
 *   - **Tab 在弹窗内循环**，不会跑到背后的页面上；
 *   - 标题通过 `aria-labelledby` 关联，读屏能报出"某某对话框"。
 *
 * 视觉：标题 20px 24px、圆角 12、阴影 shadow-4（令牌见 components.css）。
 */
export function Dialog({ open, title, children, actions, onClose, closeOnBackdrop = true, className }: DialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null)
  const restoreRef = useRef<HTMLElement | null>(null)
  const titleId = useId()

  // onClose 通常是内联箭头（`() => setOpen(false)`），每次父组件重渲染都是新引用。
  // 若把它放进 effect 依赖，父组件一重渲染 effect 就会重跑 —— 而重跑会把焦点
  // 重新抢到第一个可聚焦元素上，键盘用户正在操作的位置就丢了。
  // 因此用 ref 持有它，effect 只依赖 `open`。
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  const focusables = useCallback((): HTMLElement[] => {
    const root = dialogRef.current
    if (!root) return []
    return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
      (el) => el.offsetWidth > 0 || el.offsetHeight > 0 || el === document.activeElement,
    )
  }, [])

  useEffect(() => {
    if (!open) return

    // 记住触发元素，关闭后还给它
    restoreRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null

    // 初始焦点：第一个可聚焦元素；没有则落在弹窗容器本身
    const initial = focusables()[0]
    if (initial) initial.focus()
    else dialogRef.current?.focus()

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation()
        onCloseRef.current?.()
        return
      }
      if (event.key !== 'Tab') return
      // 焦点循环：从最后一个 Tab 回到第一个，反之亦然
      const items = focusables()
      if (items.length === 0) {
        event.preventDefault()
        dialogRef.current?.focus()
        return
      }
      const first = items[0]
      const last = items[items.length - 1]
      const active = document.activeElement
      if (event.shiftKey && (active === first || active === dialogRef.current)) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && (active === last || !dialogRef.current?.contains(active))) {
        event.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', onKeyDown)
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = previousOverflow
      // 焦点还给触发元素（它可能已被卸载，所以要判 connected）
      const target = restoreRef.current
      restoreRef.current = null
      if (target?.isConnected) target.focus()
    }
  }, [open, focusables])

  if (!open) {
    return null
  }

  return createPortal(
    <div
      className="cf-dialog-backdrop"
      onMouseDown={(event) => {
        if (closeOnBackdrop && event.target === event.currentTarget) {
          onClose?.()
        }
      }}
    >
      <div
        ref={dialogRef}
        className={cn('cf-dialog', className)}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? titleId : undefined}
        tabIndex={-1}
      >
        {title ? (
          <div className="cf-dialog__title" id={titleId}>
            {title}
          </div>
        ) : null}
        <div className="cf-dialog__content">{children}</div>
        {actions ? <div className="cf-dialog__actions">{actions}</div> : null}
      </div>
    </div>,
    document.body,
  )
}
