import { useEffect, useRef, useState } from 'react'

import { cn } from '@/helpers/cn'
import { copyIcon, checkIcon } from '@/config/icons'
import { Icon } from '@/components/unit/Icon'
import { useToast } from '@/components/unit/Toast'


export interface CopyButtonProps {
  /** 要复制的文本。 */
  text: string
  /** 成功提示文案；传空串则不出 Toast（DESIGN.md §9：图标反馈已足够时可省略）。 */
  message?: string
  className?: string
  label?: string
}

/**
 * 复制按钮（DESIGN.md §9）：点击后图标 Copy → Check 保持 1.5s；
 * Toast 默认省略（图标反馈已足够），失败时仍然 Toast。
 * 旧版各页面重复实现的剪贴板降级逻辑统一在这里。
 */
export function CopyButton({ text, message = '', className, label = '复制' }: CopyButtonProps) {
  const toast = useToast()
  const [done, setDone] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => () => clearTimeout(timer.current), [])

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
      setDone(true)
      clearTimeout(timer.current)
      timer.current = setTimeout(() => setDone(false), 1500)
      if (message) {
        toast.success(message)
      }
    } catch {
      toast.error('复制失败，请手动选择文本')
    }
  }

  return (
    <button type="button" className={cn('cf-copy-btn', className)} aria-label={label} title={label} onClick={copy}>
      <Icon icon={done ? checkIcon : copyIcon} size={16} />
    </button>
  )
}
