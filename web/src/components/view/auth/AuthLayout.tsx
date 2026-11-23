import { useState, type ReactNode } from 'react'

import { AppLink } from '@/components/unit/AppLink'
import { Icon } from '@/components/unit/Icon'
import { arrowLeftIcon, globeIcon, moonIcon, sunIcon, type IconComponent } from '@/config/icons'
import { docUrl } from '@/helpers/docUrl'
import { applyTheme, getTheme, type Theme } from '@/helpers/theme'
import { useI18n, useT } from '@/i18n'


export type AuthMode = 'login' | 'register' | 'reset' | 'verify'

export interface AuthLayoutProps {
  mode: AuthMode
  title: string
  subtitle: string
  /** 登录跳注册时保留 bind 参数。 */
  registerTo?: string
  /** 找回密码 / 二次验证顶部的上下文图标瓷砖。 */
  headerIcon?: IconComponent
  /** 用于触发子表单轻量入场动画的路由标识。 */
  paneKey: string
  children: ReactNode
}

/**
 * 认证页持久化外壳（v5 Cell Code 格·代码）。
 *
 * 由 `AuthEntry` 在路由父层持续挂载：在 `/user/login`、`/user/register`、
 * `/user/resetPassword`、`/user/verify` 之间切换时外壳不卸载，消除整页刷新与切换闪烁。
 */
export function AuthLayout({
  mode,
  title,
  subtitle,
  registerTo = '/user/register',
  headerIcon,
  paneKey,
  children,
}: AuthLayoutProps) {
  const t = useT()
  const { lang, setLang } = useI18n()
  const [theme, setTheme] = useState<Theme>(() => getTheme())

  const toggleTheme = () => {
    const next: Theme = theme === 'dark' ? 'light' : 'dark'
    applyTheme(next)
    setTheme(next)
  }

  const toggleLang = () => {
    setLang(lang === 'zh-cn' ? 'en-us' : 'zh-cn')
  }

  const showBackLink = mode === 'reset' || mode === 'verify'

  return (
    <div className="auth-shell">
      <div className="auth-shell__grid" aria-hidden="true" />
      <div className="auth-shell__halo" aria-hidden="true" />

      <header className="auth-topbar">
        <AppLink to="/user/login" className="auth-brand">
          <span className="auth-brand__mark" aria-hidden="true">
            <i /><i /><i /><i />
          </span>
          <span className="auth-brand__name">CodeFever</span>
        </AppLink>

        <div className="auth-topbar__actions">
          <button type="button" className="auth-util-btn" onClick={toggleLang}>
            <Icon icon={globeIcon} size={14} />
            <span>{lang === 'zh-cn' ? 'EN' : '中文'}</span>
          </button>
          <button
            type="button"
            className="auth-util-btn auth-util-btn--icon"
            aria-label={theme === 'dark' ? t('auth.switchToLight') : t('auth.switchToDark')}
            title={theme === 'dark' ? t('auth.switchToLight') : t('auth.switchToDark')}
            onClick={toggleTheme}
          >
            <Icon icon={theme === 'dark' ? sunIcon : moonIcon} size={15} />
          </button>
        </div>
      </header>

      <main className="auth-stage">
        <section className="auth-card" aria-labelledby="auth-card-title">
          {showBackLink ? (
            <AppLink to="/user/login" className="auth-card__back">
              <Icon icon={arrowLeftIcon} size={14} />
              <span>{t('user_password_retrieval_login_now')}</span>
            </AppLink>
          ) : null}

          <div className="auth-card__head">
            {headerIcon ? (
              <span className="auth-icon-tile" aria-hidden="true">
                <Icon icon={headerIcon} size={20} />
              </span>
            ) : null}
            <h1 id="auth-card-title" className="auth-card__title">
              {title}
            </h1>
            <p className="auth-card__subtitle">{subtitle}</p>
          </div>

          <div key={paneKey} className="auth-pane">
            {children}
          </div>

          {mode === 'login' ? (
            <div className="auth-switch">
              <span>{t('auth.noAccount')}</span>
              <AppLink to={registerTo} className="auth-link">
                {t('auth.tabRegister')}
              </AppLink>
            </div>
          ) : null}

          {mode === 'register' ? (
            <div className="auth-switch">
              <span>{t('auth.hasAccount')}</span>
              <AppLink to="/user/login" className="auth-link">
                {t('auth.tabLogin')}
              </AppLink>
            </div>
          ) : null}
        </section>

        <footer className="auth-footer">
          <span>CodeFever Next</span>
          <span className="auth-footer__sep" aria-hidden="true">·</span>
          <AppLink to={docUrl(lang)} className="auth-footer__link">
            {t('auth.docs')}
          </AppLink>
        </footer>
      </main>
    </div>
  )
}
