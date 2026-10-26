import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'


export type ToastKind = 'success' | 'danger' | 'warning' | 'info'

interface ToastItem {
  id: number
  kind: ToastKind
  message: ReactNode
}

export interface ToastApi {
  show: (kind: ToastKind, message: ReactNode) => void
  error: (message: ReactNode) => void
  success: (message: ReactNode) => void
  warning: (message: ReactNode) => void
  info: (message: ReactNode) => void
}

const ToastContext = createContext<ToastApi | null>(null)

/** 自动消失时长：与旧版 toastr 的默认体感一致。 */
const TOAST_DURATION = 3000

/**
 * 全局消息提示。
 *
 * 旧版是 jQuery 的 `toastr.error(...)`，散布在 4 个服务端模板里；
 * 现在统一为 `useToast()`，页面不再自己实现提示。
 */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const seq = useRef(0)

  const remove = useCallback((id: number) => {
    setItems((prev) => prev.filter((item) => item.id !== id))
  }, [])

  const show = useCallback(
    (kind: ToastKind, message: ReactNode) => {
      seq.current += 1
      const id = seq.current
      setItems((prev) => [...prev, { id, kind, message }])
      window.setTimeout(() => remove(id), TOAST_DURATION)
    },
    [remove],
  )

  const api = useMemo<ToastApi>(
    () => ({
      show,
      error: (message) => show('danger', message),
      success: (message) => show('success', message),
      warning: (message) => show('warning', message),
      info: (message) => show('info', message),
    }),
    [show],
  )

  return (
    <ToastContext.Provider value={api}>
      {children}
      {createPortal(
        <div className="cf-toast-stack">
          {items.map((item) => (
            <div key={item.id} className={`cf-toast cf-toast--${item.kind}`} role="status">
              <span>{item.message}</span>
              <button type="button" className="cf-toast__close" aria-label="关闭" onClick={() => remove(item.id)}>
                ×
              </button>
            </div>
          ))}
        </div>,
        document.body,
      )}
    </ToastContext.Provider>
  )
}

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    throw new Error('useToast 必须在 <ToastProvider> 内使用')
  }
  return ctx
}
