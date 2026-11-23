import { useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router'

import { login as loginApi } from '@/api/auth'
import { extraField } from '@/api/client'
import { AppLink } from '@/components/unit/AppLink'
import { Button } from '@/components/unit/Button'
import { Icon } from '@/components/unit/Icon'
import { PasswordField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { emailIcon, lockIcon, warningCircleIcon } from '@/config/icons'
import { useT } from '@/i18n'


const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/**
 * 登录页表单（`/user/login`，由 `AuthEntry` 承载持久化外壳）。
 *
 * 契约（`backend/internal/handlers/user.go` 的 LoginHandler）：
 *   code 0   → 成功，`extra.redirect`（固定 `/repositories`）
 *   code 100 → 需要 2FA，`extra.key` 作为临时键跳到 `/user/verify?key=...`
 *   code 1/2 → 缺邮箱/密码；3 → 账号不存在；4 → 账号或口令错误
 */
export default function Login() {
  const t = useT()
  const navigate = useNavigate()
  const toast = useToast()
  const [params] = useSearchParams()

  const [email, setEmail] = useState(params.get('email') ?? '')
  const [password, setPassword] = useState('')
  const [emailError, setEmailError] = useState('')
  const [passwordError, setPasswordError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState('')

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setMessage('')

    const nextEmailError = !email.trim()
      ? t('user_form_email_tel')
      : EMAIL_RE.test(email.trim())
        ? ''
        : t('user_form_email_error')
    const nextPasswordError = !password
      ? t('user_form_password')
      : password.length < 6
        ? t('user_form_password_min_error')
        : ''
    setEmailError(nextEmailError)
    setPasswordError(nextPasswordError)
    if (nextEmailError || nextPasswordError) {
      return
    }

    setSubmitting(true)
    try {
      const result = await loginApi(email.trim(), password)
      if (result.code === 0) {
        window.location.href = extraField(result, 'redirect') ?? '/repositories'
        return
      }
      if (result.code === 100) {
        const key = extraField(result, 'key')
        if (key) {
          navigate(`/user/verify?key=${encodeURIComponent(key)}`)
          return
        }
      }
      setMessage(result.message ?? '')
      if (result.message) {
        toast.error(result.message)
      }
    } catch (error) {
      const text = error instanceof Error ? error.message : t('base_error_message')
      setMessage(text)
      toast.error(text)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="auth-form" onSubmit={submit} noValidate>
      <TextField
        size="lg"
        label={t('auth.emailLabel')}
        name="email"
        type="email"
        autoComplete="username"
        autoFocus
        placeholder={t('auth.emailPlaceholder')}
        startAdornment={<Icon icon={emailIcon} size={16} />}
        value={email}
        error={emailError}
        onChange={(event) => {
          setEmail(event.target.value)
          setEmailError('')
        }}
      />
      <PasswordField
        size="lg"
        label={t('auth.passwordLabel')}
        labelEnd={
          <AppLink to="/user/resetPassword" className="auth-link">
            {t('user_form_forget_password')}
          </AppLink>
        }
        name="password"
        autoComplete="current-password"
        placeholder={t('auth.passwordPlaceholder')}
        startAdornment={<Icon icon={lockIcon} size={16} />}
        value={password}
        error={passwordError}
        onChange={(event) => {
          setPassword(event.target.value)
          setPasswordError('')
        }}
      />
      {message ? (
        <div className="auth-alert" role="alert">
          <Icon icon={warningCircleIcon} size={16} className="auth-alert__icon" />
          <span>{message}</span>
        </div>
      ) : null}
      <Button variant="contained" className="auth-submit" size="lg" type="submit" block loading={submitting}>
        {submitting ? t('user_form_sending') : t('auth.loginSubmit')}
      </Button>
    </form>
  )
}
