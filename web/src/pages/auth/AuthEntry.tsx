import { useLocation, useSearchParams } from 'react-router'

import { AuthLayout, type AuthMode } from '@/components/view/auth/AuthLayout'
import { keyIcon, shieldCheckIcon, type IconComponent } from '@/config/icons'
import { useT } from '@/i18n'

import Login from './Login'
import Register from './Register'
import ResetPassword from './ResetPassword'
import Verify from './Verify'


/**
 * 认证页统一懒加载入口。
 *
 * 将登录 / 注册 / 找回密码 / 二次验证合入同一个懒加载分包，并由单一持久化的
 * `<AuthLayout>` 包裹：
 *   1. 主应用首屏依然不包含认证页代码（保持路由级分包）；
 *   2. 在四个认证子页之间切换时，不触发 `<Suspense>` 回退，不卸载外壳卡片，
 *      彻底消除切换注册与登录时的闪烁。
 */
export default function AuthEntry() {
  const t = useT()
  const { pathname } = useLocation()
  const [params] = useSearchParams()

  const normalized = pathname.toLowerCase()
  const bind = params.get('bind') ?? ''
  const registerTo = bind ? `/user/register?bind=${encodeURIComponent(bind)}` : '/user/register'

  let mode: AuthMode = 'login'
  let title = t('auth.loginTitle')
  let subtitle = t('auth.loginSubtitle')
  let headerIcon: IconComponent | undefined
  let paneKey = 'login'
  let content = <Login />

  if (normalized === '/user/register') {
    mode = 'register'
    title = t('auth.registerTitle')
    subtitle = t('auth.registerSubtitle')
    paneKey = 'register'
    content = <Register />
  } else if (normalized === '/user/resetpassword') {
    const isResetStep = params.get('mode') === 'reset'
    mode = 'reset'
    title = isResetStep ? t('user_password_retrieval') : t('auth.resetTitle')
    subtitle = isResetStep ? t('auth.resetNewSubtitle') : t('auth.resetSubtitle')
    headerIcon = keyIcon
    paneKey = isResetStep ? 'reset-apply' : 'reset-request'
    content = <ResetPassword />
  } else if (normalized === '/user/verify') {
    mode = 'verify'
    title = t('auth.verifyTitle')
    subtitle = t('auth.verifySubtitle')
    headerIcon = shieldCheckIcon
    paneKey = 'verify'
    content = <Verify />
  }

  return (
    <AuthLayout
      mode={mode}
      title={title}
      subtitle={subtitle}
      registerTo={registerTo}
      headerIcon={headerIcon}
      paneKey={paneKey}
    >
      {content}
    </AuthLayout>
  )
}
