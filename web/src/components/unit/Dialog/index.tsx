import { useEffect, type ReactNode } from 'react'
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

/**
 * 模态框。
 *
 * 统一处理：Esc 关闭、遮罩点击关闭、body 滚动锁定、portal 挂载到 body。
 * 旧版依赖 MUI Dialog（每处各自 `setState` 控制），此组件保留同样的视觉：
 * 标题居中 20px 24px、阴影 elevation24、圆角 4。
 */
export function Dialog({ open, title, children, actions, onClose, closeOnBackdrop = true, className }: DialogProps) {
  useEffect(() => {
    if (!open) {
      return
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose?.()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = previousOverflow
    }
  }, [open, onClose])

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
      <div className={cn('cf-dialog', className)} role="dialog" aria-modal="true">
        {title ? <div className="cf-dialog__title">{title}</div> : null}
        <div className="cf-dialog__content">{children}</div>
        {actions ? <div className="cf-dialog__actions">{actions}</div> : null}
      </div>
    </div>,
    document.body,
  )
}
