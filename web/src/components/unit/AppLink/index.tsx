import type { AnchorHTMLAttributes, ReactNode } from 'react'
import { Link as RouterLink } from 'react-router'

import { cn } from '@/helpers/cn'


export interface AppLinkProps extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> {
  /** 站内路径（走前端路由）或站外绝对地址（配合 external 使用）。 */
  to: string
  external?: boolean
  children: ReactNode
}

/**
 * 统一链接组件：站内走前端路由，站外自动补 `rel`。
 * 旧版大量裸 `<a href="/user/login">` 会触发整页刷新，这里统一收口。
 */
export function AppLink({ to, external = false, className, children, ...rest }: AppLinkProps) {
  if (external) {
    return (
      <a href={to} rel="noreferrer noopener" target="_blank" className={cn('cf-link', className)} {...rest}>
        {children}
      </a>
    )
  }

  return (
    <RouterLink to={to} className={cn('cf-link', className)} {...rest}>
      {children}
    </RouterLink>
  )
}
