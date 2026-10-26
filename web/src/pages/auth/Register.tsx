import { useState, type FormEvent } from 'react'

import { register as registerApi } from '@/api/auth'
import { extraField } from '@/api/client'
import { Button } from '@/components/unit/Button'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { AuthLayout } from '@/components/view/auth/AuthLayout'
import { useT } from '@/i18n'


const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const ILLEGAL_CHARS = /[\u4e00-\u9fa5\s; &|]/

/**
 * 注册页（`/user/register`）。
 *
 * 校验规则逐条对齐旧模板 `register.html`：
 *   - 邮箱：必填 + 格式；输入时剔除中文/空白/`;`/`&`/`|`（旧页 onkeyup 的替换）；
 *   - 密码：6~18 位（旧页 maxlength=18 + `user_register_password_regex` 提示）；
 *   - 确认密码：需一致（`user_form_password_re_error`）；
 *   - 不允许空格（`user_form_password_space_error`）。
 * 提交失败时后端返回业务码 1/2/3…，文案由后端给（`writeAjax` 的 message）。
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
      const text = error instanceof Error ? error.message : '注册失败'
      setMessage(text)
      toast.error(text)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout title={t('user_form_register_page_welcome')}>
      <form className="auth-form" onSubmit={submit} noValidate>
        <TextField
          variant="underline"
          name="email"
          type="text"
          autoComplete="username"
          placeholder={t('user_form_register_email')}
          value={email}
          error={errors.email}
          onChange={(event) => {
            setEmail(event.target.value.replace(ILLEGAL_CHARS, ''))
            setErrors((prev) => ({ ...prev, email: undefined }))
          }}
        />
        <TextField
          variant="underline"
          name="password"
          type="password"
          autoComplete="new-password"
          maxLength={18}
          placeholder={t('user_register_password')}
          value={password}
          error={errors.password}
          onChange={(event) => {
            setPassword(event.target.value)
            setErrors((prev) => ({ ...prev, password: undefined }))
          }}
        />
        <TextField
          variant="underline"
          name="rePassword"
          type="password"
          autoComplete="new-password"
          maxLength={18}
          placeholder={t('user_register_re_password')}
          value={rePassword}
          error={errors.rePassword}
          onChange={(event) => {
            setRePassword(event.target.value)
            setErrors((prev) => ({ ...prev, rePassword: undefined }))
          }}
        />
        {message ? <div className="auth-message">{message}</div> : null}
        <Button variant="contained" className="cf-btn--auth auth-submit" size="lg" type="submit" block loading={submitting}>
          {submitting ? t('user_bind_registering') : t('user_register_free')}
        </Button>
        <div className="auth-footnote">
          {t('user_register_have_account')}
          <a className="auth-link" href="/user/login">
            {t('user_form_login_now')}
          </a>
        </div>
      </form>
    </AuthLayout>
  )
}
