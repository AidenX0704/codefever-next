import { useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router'

import { login as loginApi } from '@/api/auth'
import { extraField } from '@/api/client'
import { Button } from '@/components/unit/Button'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { AuthLayout } from '@/components/view/auth/AuthLayout'
import { useT } from '@/i18n'


const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/**
 * 登录页（`/user/login`）。
 *
 * 契约（`backend/internal/handlers/user.go` 的 LoginHandler）：
 *   code 0   → 成功，`extra.redirect`（固定 `/repositories`）
 *   code 100 → 需要 2FA，`extra.key` 作为临时键跳到 `/user/verify?key=...`
 *   code 1/2 → 缺邮箱/密码；3 → 账号不存在；4 → 账号或口令错误
 * 三种成功/中间态的处理顺序与旧模板 `login.html` 完全一致。
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

    // 客户端校验规则对齐旧 jQuery.validate：email required+format、password required+min 6。
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
      const text = error instanceof Error ? error.message : '登录失败'
      setMessage(text)
      toast.error(text)
    } finally {
      setSubmitting(false)
    }
  }

  const bind = params.get('bind') ?? ''

  return (
    <AuthLayout title={t('user_form_page_welcome')}>
      <form className="auth-form" onSubmit={submit} noValidate>
        <TextField
          variant="underline"
          name="email"
          type="text"
          autoComplete="username"
          placeholder={t('user_form_email_tel')}
          value={email}
          error={emailError}
          onChange={(event) => {
            setEmail(event.target.value)
            setEmailError('')
          }}
        />
        <TextField
          variant="underline"
          name="password"
          type="password"
          autoComplete="current-password"
          placeholder={t('user_form_password')}
          value={password}
          error={passwordError}
          onChange={(event) => {
            setPassword(event.target.value)
            setPasswordError('')
          }}
        />
        <div className="auth-form__links">
          <a className="auth-link" href="/user/resetPassword">
            {t('user_form_forget_password')}
          </a>
        </div>
        {message ? <div className="auth-message">{message}</div> : null}
        <Button variant="contained" className="cf-btn--auth auth-submit" size="lg" type="submit" block loading={submitting}>
          {submitting ? t('user_form_sending') : t('user_login_btn_login')}
        </Button>
        <div className="auth-footnote">
          {t('user_login_no_account')}
          <a className="auth-link" href={`/user/register?bind=${encodeURIComponent(bind)}`}>
            {t('user_login_register_new')}
          </a>
        </div>
      </form>
    </AuthLayout>
  )
}
