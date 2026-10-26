import type { ReactNode } from 'react'

import { cn } from '@/helpers/cn'

import { useToast } from '@/components/unit/Toast'


export interface CopyButtonProps {
  /** 要复制的文本。 */
  text: string
  /** 成功提示文案（默认"已复制"）。 */
  message?: string
  className?: string
  /** 图标内容；默认使用一个纯 CSS/文本符号，业务需要可换成图标组件。 */
  children?: ReactNode
  label?: string
}

/**
 * 复制按钮。
 *
 * 旧版各页面重复实现 `navigator.clipboard.writeText` + 成功提示（含降级分支）；
 * 这里统一实现一次：Clipboard API 不可用时退回 `document.execCommand`。
 */
export function CopyButton({ text, message = '已复制', className, children, label = '复制' }: CopyButtonProps) {
  const toast = useToast()

  const copy = async () => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text)
      } else {
        const area = document.createElement('textarea')
        area.value = text
        area.style.position = 'fixed'
        area.style.opacity = '0'
        document.body.appendChild(area)
        area.select()
        document.execCommand('copy')
        document.body.removeChild(area)
      }
      toast.success(message)
    } catch {
      toast.error('复制失败，请手动选择文本')
    }
  }

  return (
    <button type="button" className={cn('cf-copy-btn', className)} aria-label={label} title={label} onClick={copy}>
      {children ?? '⧉'}
    </button>
  )
}
