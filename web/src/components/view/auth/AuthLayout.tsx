import type { ReactNode } from 'react'

import { activityHero, activityHeroWebp, headerLogo } from '@/assets/images'


export interface AuthLayoutProps {
  /** 表单头部欢迎语（旧模板 `user_form_page_welcome` / `user_form_register_page_welcome`）。 */
  title: string
  children: ReactNode
}

/**
 * 认证页外壳：左侧品牌图 + 右侧表单列。
 *
 * 对应旧 SSR 页结构（`www/static/css/main.css` 的 .section-body / .activityBox）：
 * 卡片 1184×644、左图 644 宽、表单列 360 宽、≤1184px 隐藏左图。
 * 左图用 `<picture>` 优先 WebP（30.5 KB vs PNG 147.9 KB），
 * 与旧版性能整改 A4 的做法保持一致。
 */
export function AuthLayout({ title, children }: AuthLayoutProps) {
  return (
    <div className="auth-body">
      <div className="auth-card">
        <div className="auth-card__hero">
          <picture>
            <source srcSet={activityHeroWebp} type="image/webp" />
            <img src={activityHero} alt="CodeFever" fetchPriority="high" decoding="async" />
          </picture>
        </div>
        <div className="auth-card__panel">
          <div className="auth-card__inner">
            <div className="auth-head">
              <img className="auth-head__logo" src={headerLogo} alt="CodeFever" />
              <div className="auth-head__text">
                <div className="auth-head__title">{title}</div>
                <div className="auth-head__subtitle">CodeFever</div>
              </div>
            </div>
            {children}
          </div>
        </div>
      </div>
    </div>
  )
}
