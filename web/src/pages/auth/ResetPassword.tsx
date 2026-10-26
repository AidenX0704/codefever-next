import { useEffect, useState, type FormEvent } from 'react'
import { useSearchParams } from 'react-router'

import { applyNewPassword, requestResetCode, requestResetTicket } from '@/api/auth'
import { extraField } from '@/api/client'
import { Button } from '@/components/unit/Button'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { AuthLayout } from '@/components/view/auth/AuthLayout'
import { useT } from '@/i18n'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
/** 重发倒计时：与旧模板一致（120 秒）。 */
const COUNTDOWN_SECONDS = 120

/** 「通过邮箱找回」：邮箱 + 邮箱验证码 → 换取临时 key。 */
function RequestForm() {
  const t = useT()
  const toast = useToast()

  const [email, setEmail] = useState('')
  const [emailCode, setEmailCode] = useState('')
  const [errors, setErrors] = useState<{ email?: string; emailCode?: string }>({})
  const [submitting, setSubmitting] = useState(false)
  const [sending, setSending] = useState(false)
  const [countdown, setCountdown] = useState(0)
  const [message, setMessage] = useState('')

  useEffect(() => {
    if (countdown <= 0) {
      return
    }
    const timer = window.setTimeout(() => setCountdown((value) => value - 1), 1000)
    return () => window.clearTimeout(timer)
  }, [countdown])

  const sendCode = async () => {
    if (!EMAIL_RE.test(email.trim())) {
      setErrors((prev) => ({ ...prev, email: t('user_form_email_error') }))
      return
    }
    setSending(true)
    try {
      const result = await requestResetCode(email.trim())
      if (result.code === 0) {
        setCountdown(COUNTDOWN_SECONDS)
        toast.success(t('user_form_code_sent'))
        return
      }
      if (result.message) {
        toast.error(result.message)
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSending(false)
    }
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setMessage('')

    const next: typeof errors = {}
    if (!email.trim()) {
      next.email = t('user_form_email_tel')
    } else if (!EMAIL_RE.test(email.trim())) {
      next.email = t('user_form_email_error')
    }
    if (!emailCode.trim()) {
      next.emailCode = t('user_form_email_code')
    }
    setErrors(next)
    if (Object.keys(next).length > 0) {
      return
    }

    setSubmitting(true)
    try {
      const result = await requestResetTicket(email.trim(), emailCode.trim())
      const redirect = extraField(result, 'redirect')
      if (result.code === 0 && redirect) {
        // 旧页同样是整页跳转（服务端已把临时 key 写进会话，跳转后进入 mode=reset）。
        window.location.href = redirect
        return
      }
      const text = result.message || t('base_error_message')
      setMessage(text)
      toast.error(text)
    } catch (error) {
      const text = error instanceof Error ? error.message : t('base_error_message')
      setMessage(text)
      toast.error(text)
    } finally {
      setSubmitting(false)
    }
  }

  const codeButtonLabel =
    countdown > 0 ? `${t('user_form_code_sent')} (${countdown}s)` : t('user_form_get_code')

  return (
    <AuthLayout title={t('user_password_retrieval_title')}>
      <form className="auth-form" onSubmit={submit} noValidate>
        <TextField
          variant="underline"
          name="email"
          type="text"
          autoComplete="username"
          placeholder={t('user_form_email_tel')}
          value={email}
          error={errors.email}
          onChange={(event) => {
            setEmail(event.target.value.replace(/[\u4e00-\u9fa5\s]/g, ''))
            setErrors((prev) => ({ ...prev, email: undefined }))
          }}
        />
        <TextField
          variant="underline"
          name="emailCode"
          type="text"
          inputMode="numeric"
          autoComplete="one-time-code"
          maxLength={6}
          placeholder={t('user_form_email_code')}
          value={emailCode}
          error={errors.emailCode}
          endAdornment={
            <button
              type="button"
              className="cf-btn cf-btn--text cf-auth-code-btn"
              disabled={sending || countdown > 0}
              onClick={sendCode}
            >
              {sending ? t('user_form_sending') : codeButtonLabel}
            </button>
          }
          onChange={(event) => {
            setEmailCode(event.target.value.replace(/\D/g, ''))
            setErrors((prev) => ({ ...prev, emailCode: undefined }))
          }}
        />
        {message ? <div className="auth-message">{message}</div> : null}
        <Button variant="contained" className="cf-btn--auth auth-submit" size="lg" type="submit" block loading={submitting}>
          {t('user_form_verfiy')}
        </Button>
        <div className="auth-footnote">
          <a className="auth-link" href="/user/login">
            {t('user_password_retrieval_login_now')}
          </a>
        </div>
      </form>
    </AuthLayout>
  )
}

/** 「设置新口令」：临时 key + 新密码 + 确认密码。 */
function ResetForm({ ticket }: { ticket: string }) {
  const t = useT()
  const toast = useToast()

  const [password, setPassword] = useState('')
  const [rePassword, setRePassword] = useState('')
  const [errors, setErrors] = useState<{ password?: string; rePassword?: string }>({})
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState('')

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setMessage('')

    const next: typeof errors = {}
    if (!password) {
      next.password = t('user_form_password')
    } else if (password.length < 6) {
      next.password = t('user_form_password_min_error')
    }
    if (rePassword !== password) {
      next.rePassword = t('user_form_password_re_error')
    }
    setErrors(next)
    if (Object.keys(next).length > 0) {
      return
    }

    setSubmitting(true)
    try {
      const result = await applyNewPassword(ticket, password)
      const redirect = extraField(result, 'redirect')
      if (result.code === 0) {
        window.location.href = redirect ?? '/user/login'
        return
      }
      if (result.code === 100 && redirect) {
        window.location.href = redirect
        return
      }
      const text = result.message ?? ''
      setMessage(text)
      if (text) {
        toast.error(text)
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
    <AuthLayout title={t('user_password_retrieval')}>
      <form className="auth-form" onSubmit={submit} noValidate>
        <TextField
          variant="underline"
          name="password"
          type="password"
          autoComplete="new-password"
          maxLength={18}
          placeholder={t('user_password_retrieval_enter_password')}
          value={password}
          error={errors.password}
          onChange={(event) => {
            setPassword(event.target.value)
            setErrors((prev) => ({ ...prev, password: undefined }))
          }}
        />
        <TextField
          variant="underline"
          name="re_password"
          type="password"
          autoComplete="new-password"
          placeholder={t('user_password_retrieval_re_enter_password')}
          value={rePassword}
          error={errors.rePassword}
          onChange={(event) => {
            setRePassword(event.target.value.replace(/[\u4e00-\u9fa5\s]/g, ''))
            setErrors((prev) => ({ ...prev, rePassword: undefined }))
          }}
        />
        {message ? <div className="auth-message">{message}</div> : null}
        <Button variant="contained" className="cf-btn--auth auth-submit" size="lg" type="submit" block loading={submitting}>
          {t('user_password_retrieval_reset_password')}
        </Button>
        <div className="auth-footnote">
          <a className="auth-link" href="/user/login">
            {t('user_password_retrieval_login_now')}
          </a>
        </div>
      </form>
    </AuthLayout>
  )
}

/**
 * 找回密码页（`/user/resetPassword`）。
 *
 * 两种模式由查询参数决定，与旧模板 `reset_password.html` 的 `?mode=` 一致：
 *   - 缺省（请求模式）：邮箱 + 邮箱验证码 → `POST /user/resetPassword` 换取临时 key；
 *   - `mode=reset&key=...`：设置新口令 → `POST /user/resetPassword`（`mode=reset`）。
 */
export default function ResetPassword() {
  const [params] = useSearchParams()
  const mode = params.get('mode')
  const ticket = params.get('key') ?? ''

  if (mode === 'reset') {
    return <ResetForm ticket={ticket} />
  }
  return <RequestForm />
}
