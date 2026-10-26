import { useCallback, useEffect, useRef, useState } from 'react'

import { fetchAdminConfig, saveAdminConfig } from '@/api/admin'
import { Button } from '@/components/unit/Button'
import { Switch } from '@/components/unit/Choices'
import { useConfirm } from '@/components/unit/Confirm'
import { FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { Loading } from '@/components/unit/Loading'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useT } from '@/i18n'


/**
 * 站点 URL 前缀：必须以 `http://` 或 `https://` 开头。
 *
 * 旧版 `admin/Settings.js:56` 的正则是 `/^http(s)?:\/\/[^/]+$/`（额外禁止带路径）；
 * 这里按"必须带协议前缀"的底线校验，路径部分交由后端与运维决定。
 */
const HOST_PATTERN = /^https?:\/\/\S+$/

/** SSH 前缀：必须形如 `user@host`（旧版 `admin/Settings.js:64`）。 */
const SSH_PATTERN = /^[^@\s]+@.+$/

/** 发件人地址：非空 + 邮箱格式（旧版 `admin/Settings.js:88`）。 */
const EMAIL_PATTERN = /^[^@\s]+@[^@\s]+\.[^@\s]+$/

interface SettingsForm {
  host: string
  ssh: string
  emailName: string
  emailFrom: string
  allowRegister: boolean
}

/**
 * 管理后台 · 系统设置（`/admin/settings`）。
 *
 * `admin/config` 的读写语义（后端 `configGet` / `configPost`）：
 *   - GET 返回整份 config.yaml 的结构化快照；
 *   - POST 的 body 是 `{ data: "<JSON 字符串>" }`，服务端校验"必须是 JSON 对象" →
 *     与现值**浅合并** → 结构校验 → 备份 + 原子写 → 热更新内存配置。
 *
 * 本页因此只提交旧版那 5 个字段（`host` / `ssh` / `allowRegister` / `email.name` /
 * `email.from`），其余配置保持服务端现值不变。
 *
 * ⚠️ 后端 `domain.ValidateSiteDocument` 对 `ssh` 只查"是不是字符串"、
 * 对 `email.*` 也只查类型，**格式完全不做校验** —— 所以下面这 7 条前端校验
 * 是唯一防线，不能省（后端 `backend/internal/domain/admin.go:765-789`）。
 */
export default function AdminSettings() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()

  const [form, setForm] = useState<SettingsForm>({
    host: '',
    ssh: '',
    emailName: '',
    emailFrom: '',
    allowRegister: false,
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [errors, setErrors] = useState<Partial<Record<keyof SettingsForm, string>>>({})

  const aliveRef = useRef(true)
  useEffect(
    () => () => {
      aliveRef.current = false
    },
    [],
  )

  const fill = useCallback((config: Record<string, unknown>) => {
    const email = (config.email ?? {}) as Record<string, unknown>
    setForm({
      host: typeof config.host === 'string' ? config.host : '',
      ssh: typeof config.ssh === 'string' ? config.ssh : '',
      emailName: typeof email.name === 'string' ? email.name : '',
      emailFrom: typeof email.from === 'string' ? email.from : '',
      allowRegister: config.allowRegister === true,
    })
    setErrors({})
  }, [])

  const load = useCallback(() => {
    setLoading(true)
    fetchAdminConfig()
      .then((config) => {
        if (aliveRef.current) {
          fill(config)
        }
      })
      .catch((error: unknown) => {
        if (aliveRef.current) {
          toast.error(error instanceof Error ? error.message : t('base_error_message'))
        }
      })
      .finally(() => {
        if (aliveRef.current) {
          setLoading(false)
        }
      })
  }, [fill, t, toast])

  useEffect(() => {
    load()
  }, [load])

  /** 7 条校验：host 协议前缀、ssh user@host、发件人名称、发件人地址（空 + 格式）。 */
  const validate = (): boolean => {
    const next: Partial<Record<keyof SettingsForm, string>> = {}

    if (!form.host.trim()) {
      next.host = t('message.error._S_empty', { s: t('label.urlPrefix') })
    } else if (!HOST_PATTERN.test(form.host.trim())) {
      next.host = t('message.error._S_invalid', { s: t('label.urlPrefix') })
    }

    if (!SSH_PATTERN.test(form.ssh.trim())) {
      next.ssh = t('message.error._S_invalid', { s: t('label.sshPrefix') })
    }

    if (!form.emailName.trim()) {
      next.emailName = t('message.error._S_empty', { s: t('label.senderName') })
    }

    if (!form.emailFrom.trim()) {
      next.emailFrom = t('message.error._S_empty', { s: t('label.senderAddress') })
    } else if (!EMAIL_PATTERN.test(form.emailFrom.trim())) {
      next.emailFrom = t('message.error._S_invalid', { s: t('label.senderAddress') })
    }

    setErrors(next)
    return Object.keys(next).length === 0
  }

  const save = async () => {
    if (!validate()) {
      return
    }
    const ok = await confirm({
      title: t('menu.setting_pl'),
      message: t('message.configSaveWarning'),
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    setSaving(true)
    try {
      await saveAdminConfig({
        host: form.host.trim(),
        ssh: form.ssh.trim(),
        allowRegister: form.allowRegister,
        email: { name: form.emailName.trim(), from: form.emailFrom.trim() },
      })
      toast.success(t('message.updateSuccess'))
      load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <Loading />
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.host')}</div>
        <FormGrid>
          <FormItem label={t('label.urlPrefix')} required error={errors.host}>
            <TextField
              value={form.host}
              placeholder={t('label.urlPrefix')}
              onChange={(event) => setForm((prev) => ({ ...prev, host: event.target.value }))}
            />
          </FormItem>
          <FormItem label={t('label.sshPrefix')} required error={errors.ssh}>
            <TextField
              value={form.ssh}
              placeholder={t('label.sshPrefix')}
              onChange={(event) => setForm((prev) => ({ ...prev, ssh: event.target.value }))}
            />
          </FormItem>
        </FormGrid>
      </section>

      <section className="section">
        <div className="section__title">SMTP</div>
        <FormGrid>
          <FormItem label={t('label.senderName')} required error={errors.emailName}>
            <TextField
              value={form.emailName}
              placeholder={t('label.name')}
              onChange={(event) => setForm((prev) => ({ ...prev, emailName: event.target.value }))}
            />
          </FormItem>
          <FormItem label={t('label.senderAddress')} required error={errors.emailFrom}>
            <TextField
              value={form.emailFrom}
              placeholder={t('label.email')}
              onChange={(event) => setForm((prev) => ({ ...prev, emailFrom: event.target.value }))}
            />
          </FormItem>
        </FormGrid>
      </section>

      <section className="section">
        <div className="section__title">{t('label.register')}</div>
        <FormGrid>
          <FormItem label={t('label.allowRegister')}>
            <Switch
              checked={form.allowRegister}
              onChange={(event) => setForm((prev) => ({ ...prev, allowRegister: event.target.checked }))}
            />
          </FormItem>
        </FormGrid>
      </section>

      <FormActions>
        <Button variant="contained" loading={saving} onClick={save}>
          {t('label.save')}
        </Button>
        <Button variant="outlined" onClick={load}>
          {t('label.reset')}
        </Button>
      </FormActions>
    </div>
  )
}
