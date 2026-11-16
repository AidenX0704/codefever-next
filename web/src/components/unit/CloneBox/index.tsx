import { useState } from 'react'

import { cn } from '@/helpers/cn'
import { cloneUrls } from '@/helpers/cloneUrl'
import type { CurrentUser } from '@/api/types'
import { CopyButton } from '@/components/unit/CopyButton'


export interface CloneBoxProps {
  user: Pick<CurrentUser, 'host' | 'ssh'> | null | undefined
  groupName: string
  repositoryName: string
  className?: string
}

/**
 * 克隆地址组件（DESIGN.md §6.12）。
 *
 * Segmented 切换 `SSH | HTTPS`，下方等宽只读输入 + 复制按钮；
 * 无边框，--bg-subtle 底。SSH/HTTPS 拼装统一走 `helpers/cloneUrl`。
 */
export function CloneBox({ user, groupName, repositoryName, className }: CloneBoxProps) {
  const [protocol, setProtocol] = useState<'ssh' | 'https'>('ssh')
  const urls = cloneUrls(user, groupName, repositoryName)
  const value = protocol === 'ssh' ? urls.ssh : urls.https

  return (
    <div className={cn('cf-clone-box', className)}>
      <div className="cf-clone-box__segment" role="tablist" aria-label="克隆协议">
        {(['ssh', 'https'] as const).map((item) => (
          <button
            key={item}
            type="button"
            role="tab"
            aria-selected={protocol === item}
            className={cn('cf-clone-box__option', protocol === item && 'cf-clone-box__option--active')}
            onClick={() => setProtocol(item)}
          >
            {item.toUpperCase()}
          </button>
        ))}
      </div>
      <div className="cf-clone-box__row">
        <input className="cf-clone-box__input" readOnly value={value} aria-label="克隆地址" onFocus={(e) => e.target.select()} />
        <CopyButton text={value} className="cf-clone-box__copy" />
      </div>
    </div>
  )
}
