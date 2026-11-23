import { useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router'

import { verifyTwoFA } from '@/api/auth'
import { extraField } from '@/api/client'
import { Button } from '@/components/unit/Button'
import { Icon } from '@/components/unit/Icon'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { shieldCheckIcon, warningCircleIcon } from '@/config/icons'
import { useT } from '@/i18n'


/**
 * 二次验证页表单（`/user/verify?key=...`，由 `AuthEntry` 承载持久化外壳）。
 *
 * 契约（`web_user.go` 的 verifyPost）：
 *   code 0   → 成功，`extra.redirect = /repositories`；
 *   code 100 → 临时键失效，`extra.redirect = /user/login`；
 *   code 1   → 验证码错误。
 */
export default function Verify() {
  const t = useT()
  const navigate = useNavigate()
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
          if (redirect.startsWith('/user/')) {
            navigate(redirect)
          } else {
            window.location.href = redirect
          }
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
    <form className="auth-form" onSubmit={submit} noValidate>
      <TextField
        size="lg"
        label={t('auth.twoFACodeLabel')}
        name="code"
        type="text"
        inputMode="numeric"
        autoComplete="one-time-code"
        autoFocus
        maxLength={6}
        placeholder={t('auth.twoFACodePlaceholder')}
        startAdornment={<Icon icon={shieldCheckIcon} size={16} />}
        value={code}
        error={error}
        onChange={(event) => {
          setCode(event.target.value.replace(/\D/g, ''))
          setError('')
        }}
      />
      {message ? (
        <div className="auth-alert" role="alert">
          <Icon icon={warningCircleIcon} size={16} className="auth-alert__icon" />
          <span>{message}</span>
        </div>
      ) : null}
      <Button variant="contained" className="auth-submit" size="lg" type="submit" block loading={submitting}>
        {t('auth.loginSubmit')}
      </Button>
    </form>
  )
}
