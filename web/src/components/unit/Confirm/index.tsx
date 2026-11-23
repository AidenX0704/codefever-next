import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'

import { Button } from '@/components/unit/Button'
import { Dialog } from '@/components/unit/Dialog'
import { TextField } from '@/components/unit/TextField'

export interface ConfirmOptions {
  title?: ReactNode
  message: ReactNode
  confirmText?: string
  cancelText?: string
  /** 危险操作（删除/改名/转移所有权）用 danger。 */
  tone?: 'primary' | 'danger'
  /**
   * 不可恢复的危险操作需输入目标名称/路径才可点确认（DESIGN.md §1.2 G9）。
   * 不传则为普通确认弹窗。
   */
  requireInput?: string
  requireInputPrompt?: ReactNode
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
 * 收口成 `const ok = await confirm({...})`，支持可选的危险输入确认（`requireInput`）。
 */
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<ConfirmState | null>(null)
  const [typed, setTyped] = useState('')

  const confirm = useCallback<ConfirmFn>(
    (options) =>
      new Promise<boolean>((resolve) => {
        setTyped('')
        setState({ options, resolve })
      }),
    [],
  )

  const settle = useCallback(
    (value: boolean) => {
      setTyped('')
      setState((current) => {
        current?.resolve(value)
        return null
      })
    },
    [],
  )

  const value = useMemo(() => confirm, [confirm])
  const expected = state?.options.requireInput
  const canConfirm = !expected || typed.trim() === expected

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
            <Button tone={state?.options.tone ?? 'primary'} disabled={!canConfirm} onClick={() => settle(true)}>
              {state?.options.confirmText ?? '确定'}
            </Button>
          </>
        }
      >
        <div className="stack">
          <div>{state?.options.message}</div>
          {expected ? (
            <div className="stack">
              {state?.options.requireInputPrompt ? (
                <div className="muted">{state.options.requireInputPrompt}</div>
              ) : null}
              <TextField
                value={typed}
                placeholder={expected}
                autoFocus
                onChange={(event) => setTyped(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && canConfirm) {
                    settle(true)
                  }
                }}
              />
            </div>
          ) : null}
        </div>
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
