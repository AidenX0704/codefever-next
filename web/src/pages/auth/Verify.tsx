import { useState, type FormEvent } from 'react'
import { useSearchParams } from 'react-router'

import { verifyTwoFA } from '@/api/auth'
import { extraField } from '@/api/client'
import { Button } from '@/components/unit/Button'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { AuthLayout } from '@/components/view/auth/AuthLayout'
import { useT } from '@/i18n'


/**
 * 二次验证页（`/user/verify?key=...`）。
 *
 * 契约（`web_user.go` 的 verifyPost）：
 *   code 0   → 成功，`extra.redirect = /repositories`；
 *   code 100 → 临时键失效，`extra.redirect = /user/login`；
 *   code 1   → 验证码错误。
 * 按钮文案沿用旧模板（`user_login_btn_login` = "登 录"）。
 */
export default function Verify() {
  const t = useT()
  const toast = useToast()
  const [params] = useSearchParams()

  const [code, setCode] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState('')

  const key = params.get('key') ?? ''

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setMessage('')

    if (!code.trim()) {
      setError(t('user_form_code'))
      return
    }
    setError('')

    setSubmitting(true)
    try {
      const result = await verifyTwoFA(key, code.trim())
      const redirect = extraField(result, 'redirect')
      if (result.code === 0) {
        window.location.href = redirect ?? '/repositories'
        return
      }
      const text = result.message || t('user_js_2fa_verify_fail')
      setMessage(text)
      toast.error(text)
      if (result.code === 100 && redirect) {
        window.setTimeout(() => {
          window.location.href = redirect
        }, 1500)
      }
    } catch (err) {
      const text = err instanceof Error ? err.message : t('user_js_2fa_verify_fail')
      setMessage(text)
      toast.error(text)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout title={t('user_form_page_welcome')}>
      <form className="auth-form" onSubmit={submit} noValidate>
        <TextField
          variant="underline"
          name="code"
          type="text"
          inputMode="numeric"
          autoComplete="one-time-code"
          placeholder={t('user_form_code')}
          value={code}
          error={error}
          onChange={(event) => {
            setCode(event.target.value.replace(/\D/g, ''))
            setError('')
          }}
        />
        {message ? <div className="auth-message">{message}</div> : null}
        <Button variant="contained" className="cf-btn--auth auth-submit" size="lg" type="submit" block loading={submitting}>
          {t('user_login_btn_login')}
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
