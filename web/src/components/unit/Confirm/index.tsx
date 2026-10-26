import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'

import { Button } from '@/components/unit/Button'
import { Dialog } from '@/components/unit/Dialog'

export interface ConfirmOptions {
  title?: ReactNode
  message: ReactNode
  confirmText?: string
  cancelText?: string
  /** 危险操作（删除/改名/转移所有权）用 danger。 */
  tone?: 'primary' | 'danger'
}

export type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>

const ConfirmContext = createContext<ConfirmFn | null>(null)

interface ConfirmState {
  options: ConfirmOptions
  resolve: (value: boolean) => void
}

/**
 * 二次确认。
 *
 * 旧版每个删除/危险操作各自写一份 Dialog + state，这里收口成
 * `const ok = await confirm({...})`，页面不再持有确认框状态。
 */
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<ConfirmState | null>(null)

  const confirm = useCallback<ConfirmFn>(
    (options) =>
      new Promise<boolean>((resolve) => {
        setState({ options, resolve })
      }),
    [],
  )

  const settle = useCallback(
    (value: boolean) => {
      setState((current) => {
        current?.resolve(value)
        return null
      })
    },
    [],
  )

  const value = useMemo(() => confirm, [confirm])

  return (
    <ConfirmContext.Provider value={value}>
      {children}
      <Dialog
        open={state !== null}
        title={state?.options.title ?? '请确认'}
        onClose={() => settle(false)}
        actions={
          <>
            <Button variant="text" onClick={() => settle(false)}>
              {state?.options.cancelText ?? '取消'}
            </Button>
            <Button tone={state?.options.tone ?? 'primary'} onClick={() => settle(true)}>
              {state?.options.confirmText ?? '确定'}
            </Button>
          </>
        }
      >
        {state?.options.message}
      </Dialog>
    </ConfirmContext.Provider>
  )
}

export function useConfirm(): ConfirmFn {
  const ctx = useContext(ConfirmContext)
  if (!ctx) {
    throw new Error('useConfirm 必须在 <ConfirmProvider> 内使用')
  }
  return ctx
}
