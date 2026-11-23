import { useState, type FormEvent } from 'react'

import { register as registerApi } from '@/api/auth'
import { extraField } from '@/api/client'
import { Button } from '@/components/unit/Button'
import { Icon } from '@/components/unit/Icon'
import { PasswordField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { emailIcon, lockIcon, warningCircleIcon } from '@/config/icons'
import { useT } from '@/i18n'


const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const ILLEGAL_CHARS = /[\u4e00-\u9fa5\s; &|]/

/**
 * 注册页表单（`/user/register`，由 `AuthEntry` 承载持久化外壳）。
 *
 * 校验规则：
 *   - 邮箱：必填 + 格式；输入时剔除中文/空白/`;`/`&`/`|`；
 *   - 密码：6~18 位且不允许空格；
 *   - 确认密码：需与密码一致。
 */
export default function Register() {
  const t = useT()
  const toast = useToast()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [rePassword, setRePassword] = useState('')
  const [errors, setErrors] = useState<{ email?: string; password?: string; rePassword?: string }>({})
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState('')

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setMessage('')

    const next: typeof errors = {}
    if (!email.trim()) {
      next.email = t('user_form_register_email')
    } else if (!EMAIL_RE.test(email.trim())) {
      next.email = t('user_form_email_error')
    }
    if (ILLEGAL_CHARS.test(password)) {
      next.password = t('user_form_password_space_error')
    } else if (password.length < 6 || password.length > 18) {
      next.password = t('user_register_password_regex')
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
      const result = await registerApi(email.trim(), password, rePassword)
      if (result.code === 0) {
        window.location.href = extraField(result, 'redirect') ?? '/repositories'
        return
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
        error={errors.email}
        onChange={(event) => {
          setEmail(event.target.value.replace(ILLEGAL_CHARS, ''))
          setErrors((prev) => ({ ...prev, email: undefined }))
        }}
      />
      <PasswordField
        size="lg"
        label={t('auth.passwordLabel')}
        name="password"
        autoComplete="new-password"
        maxLength={18}
        placeholder={t('auth.registerPasswordPlaceholder')}
        startAdornment={<Icon icon={lockIcon} size={16} />}
        value={password}
        error={errors.password}
        onChange={(event) => {
          setPassword(event.target.value)
          setErrors((prev) => ({ ...prev, password: undefined }))
        }}
      />
      <PasswordField
        size="lg"
        label={t('auth.confirmPasswordLabel')}
        name="rePassword"
        autoComplete="new-password"
        maxLength={18}
        placeholder={t('auth.confirmPasswordPlaceholder')}
        startAdornment={<Icon icon={lockIcon} size={16} />}
        value={rePassword}
        error={errors.rePassword}
        onChange={(event) => {
          setRePassword(event.target.value)
          setErrors((prev) => ({ ...prev, rePassword: undefined }))
        }}
      />
      {message ? (
        <div className="auth-alert" role="alert">
          <Icon icon={warningCircleIcon} size={16} className="auth-alert__icon" />
          <span>{message}</span>
        </div>
      ) : null}
      <Button variant="contained" className="auth-submit" size="lg" type="submit" block loading={submitting}>
        {submitting ? t('user_bind_registering') : t('auth.registerSubmit')}
      </Button>
    </form>
  )
}
