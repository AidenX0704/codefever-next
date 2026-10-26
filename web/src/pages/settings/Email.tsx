import { useEffect, useMemo, useState } from 'react'

import {
  addCommitEmail,
  changeEmail,
  deleteCommitEmail,
  requestEmailCode,
  resendCommitEmailCode,
  validateCommitEmailCode,
} from '@/api/user'
import type { CommitEmail } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { useConfirm } from '@/components/unit/Confirm'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { Tag } from '@/components/unit/Tag'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/** 旧版 UserSettingEmail.js:250 —— 重发验证码的倒计时秒数。 */
const RESEND_COUNTDOWN = 120

/**
 * 邮箱设置（`/settings/email`）。
 *
 * 两块内容，端点逐项对齐后端：
 *   1. 主邮箱更换：`user/getEmailCode`（email + password 发码）→ `user/changeEmail`（email + code + password）；
 *   2. 提交邮箱管理：`user/addCommitEmail`、`user/resentCommitEmailCode`、
 *      `user/validationCommitEmailCode`（id + email + code）、`user/deleteCommitEmail`（id）。
 *
 * 主邮箱在 `user/info` 的 `emails[0]`（`id` 为空、`isCheck=true`），其余为提交邮箱；
 * 列表里主邮箱排在最前面并单独标注（旧版 UserSettingEmail.js:336-361）。
 */
export default function EmailSetting() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { user, refreshUser } = useAppData()

  // 主邮箱（id 为空）也在列表里，旧版就是把它排在第一行并标注"主邮箱"
  const commitEmails = useMemo<CommitEmail[]>(
    () => (user?.emails ?? []).filter((item): item is CommitEmail => typeof item !== 'string'),
    [user],
  )

  const [nextEmail, setNextEmail] = useState('')
  const [code, setCode] = useState('')
  const [password, setPassword] = useState('')
  const [sending, setSending] = useState(false)
  const [savingPrimary, setSavingPrimary] = useState(false)

  const [newCommitEmail, setNewCommitEmail] = useState('')
  const [addingCommit, setAddingCommit] = useState(false)
  const [verifyId, setVerifyId] = useState('')
  const [verifyCode, setVerifyCode] = useState('')

  /** 每个邮箱各自的重发倒计时（旧版 UserSettingEmail.js:246-265）。 */
  const [countdowns, setCountdowns] = useState<Record<string, number>>({})

  // 统一每秒推进所有邮箱的倒计时；某个邮箱归零后单独停住，其它邮箱不受影响
  useEffect(() => {
    const timer = window.setInterval(() => {
      setCountdowns((prev) => {
        const entries = Object.entries(prev)
        if (entries.length === 0) {
          return prev
        }
        let changed = false
        const next: Record<string, number> = {}
        for (const [email, value] of entries) {
          if (value <= 0) {
            next[email] = value
            continue
          }
          changed = true
          next[email] = value - 1
        }
        return changed ? next : prev
      })
    }, 1000)
    return () => window.clearInterval(timer)
  }, [])

  const sendCode = async () => {
    if (!nextEmail.trim() || !password) {
      toast.error(t('label.emailCode'))
      return
    }
    setSending(true)
    try {
      await requestEmailCode(nextEmail.trim(), password)
      toast.success(t('user_form_code_sent'))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSending(false)
    }
  }

  const submitPrimary = async () => {
    if (!nextEmail.trim() || !code.trim() || !password) {
      toast.error(t('label.emailCode'))
      return
    }
    setSavingPrimary(true)
    try {
      await changeEmail(nextEmail.trim(), code.trim(), password)
      toast.success(t('message.updateSuccess'))
      setNextEmail('')
      setCode('')
      setPassword('')
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSavingPrimary(false)
    }
  }

  const addEmail = async () => {
    if (!newCommitEmail.trim()) {
      toast.error(t('label.email'))
      return
    }
    setAddingCommit(true)
    try {
      await addCommitEmail(newCommitEmail.trim())
      toast.success(t('message.addSuccess'))
      setNewCommitEmail('')
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setAddingCommit(false)
    }
  }

  const resend = async (item: CommitEmail) => {
    // 倒计时期间不允许再次重发（旧版 codeDisabled[email]）
    if ((countdowns[item.email] ?? 0) > 0) {
      return
    }
    try {
      await resendCommitEmailCode(item.email)
      toast.success(t('user_form_code_sent'))
      setCountdowns((prev) => ({ ...prev, [item.email]: RESEND_COUNTDOWN }))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const verify = async () => {
    if (!verifyId || !verifyCode.trim()) {
      toast.error(t('label.emailCode'))
      return
    }
    const target = commitEmails.find((item) => item.id === verifyId)
    if (!target) {
      return
    }
    try {
      await validateCommitEmailCode(target.id, target.email, verifyCode.trim())
      toast.success(t('message.updateSuccess'))
      setVerifyId('')
      setVerifyCode('')
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const remove = async (item: CommitEmail) => {
    const ok = await confirm({
      title: t('label.delete'),
      message: item.email,
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    try {
      await deleteCommitEmail(item.id)
      toast.success(t('message.deleteSuccess'))
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.primaryEmail')}</div>
        <div className="row">
          <Tag tone="primary">{t('label.primaryEmail')}</Tag>
          <span>{user?.email}</span>
        </div>
        <Form onSubmit={submitPrimary}>
          <FormGrid>
            <FormItem label={t('label.email')} required>
              <TextField value={nextEmail} onChange={(event) => setNextEmail(event.target.value)} placeholder="name@example.com" />
            </FormItem>
            <FormItem label={t('label.password')} required help={t('message.changeEmailNeedPassword')}>
              <TextField type="password" value={password} onChange={(event) => setPassword(event.target.value)} />
            </FormItem>
            <FormItem label={t('label.emailCode')} required>
              <TextField
                value={code}
                onChange={(event) => setCode(event.target.value)}
                endAdornment={
                  <Button variant="text" size="sm" className="cf-auth-code-btn" loading={sending} onClick={sendCode}>
                    {t('label.getEmailCode')}
                  </Button>
                }
              />
            </FormItem>
          </FormGrid>
          <FormActions>
            <Button variant="contained" type="submit" loading={savingPrimary}>
              {t('label.save')}
            </Button>
          </FormActions>
        </Form>
      </section>

      <section className="section">
        <div className="section__title">{t('label.contactEmail')}</div>
        <div className="list-body">
          {commitEmails.length === 0 ? <div className="muted">{t('message._S_listEmpty', { s: t('label.email') })}</div> : null}
          {commitEmails.map((item) => {
            // 主邮箱（id 为空）只标注，不提供重发 / 验证 / 删除
            const isPrimary = item.id === ''
            const countdown = countdowns[item.email] ?? 0
            return (
              <div className="ref-row" key={item.id || item.email}>
                <span className="ref-row__name">{item.email}</span>
                <span className="ref-row__commit">
                  {isPrimary ? (
                    <Tag tone="primary">{t('label.primaryEmail')}</Tag>
                  ) : (
                    <Tag tone={item.isCheck ? 'success' : 'warning'}>{item.isCheck ? t('label.verified') : t('label.unverified')}</Tag>
                  )}
                </span>
                <span className="row">
                  {!isPrimary && !item.isCheck ? (
                    <>
                      <Button
                        size="sm"
                        variant="text"
                        disabled={countdown > 0}
                        onClick={() => {
                          setVerifyId(item.id)
                          void resend(item)
                        }}
                      >
                        {countdown > 0 ? t('label.retryAfter_N_seconds', { n: countdown }) : t('user_form_get_code_re')}
                      </Button>
                      <Button
                        size="sm"
                        variant="text"
                        onClick={() => {
                          setVerifyId(item.id)
                          setVerifyCode('')
                        }}
                      >
                        {t('label.verify')}
                      </Button>
                    </>
                  ) : null}
                  {!isPrimary ? (
                    <Button size="sm" variant="text" tone="danger" onClick={() => remove(item)}>
                      {t('label.delete')}
                    </Button>
                  ) : null}
                </span>
              </div>
            )
          })}
        </div>

        {verifyId ? (
          <div className="row">
            <TextField value={verifyCode} onChange={(event) => setVerifyCode(event.target.value)} placeholder={t('label.emailCode')} />
            <Button variant="contained" onClick={verify}>{t('label.verify')}</Button>
          </div>
        ) : null}

        <div className="row">
          <TextField value={newCommitEmail} onChange={(event) => setNewCommitEmail(event.target.value)} placeholder="name@example.com" />
          <Button variant="outlined" loading={addingCommit} onClick={addEmail}>
            {t('label.addCommitEmail')}
          </Button>
        </div>
      </section>
    </div>
  )
}
