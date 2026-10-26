import { useEffect, useRef, useState, type ChangeEvent } from 'react'

import { getMFAData, revokeMFAData, updateBasicInfo, updateMFAData, updatePassword, uploadAvatar } from '@/api/user'
import type { MFAData } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { useConfirm } from '@/components/unit/Confirm'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { Tag } from '@/components/unit/Tag'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/** 旧版 UserSettingGeneral.js:126 —— 主邮箱格式校验。 */
const EMAIL_PATTERN = /^\w+([.-]?\w+)*@\w+([.-]?\w+)*(\.\w{2,3})+$/

/** 旧版 UserSettingGeneral.js:172-187 —— MFA 验证码必须是 6 位数字。 */
const MFA_CODE_PATTERN = /^\d{6}$/

interface BasicErrors {
  name?: string
  email?: string
  team?: string
  role?: string
}

/**
 * 个人资料设置（`/settings/profile`）。
 *
 * 三段表单，端点与字段逐项对齐后端：
 *   1. 基本信息 → `user/updateBasicInfo`（`name` / `email` / `team` / `role`）；
 *   2. 头像 → `user/uploadAvatar`（multipart，字段名 `avatar`）；
 *   3. 修改口令 → `user/updatePassword`（`current` / `new`）；
 *   4. MFA → `user/{getMFAData,updateMFAData,revokeMFAData}`。
 */
export default function Profile() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { user, refreshUser } = useAppData()

  const [name, setName] = useState('')
  const [team, setTeam] = useState('')
  const [role, setRole] = useState('')
  const [email, setEmail] = useState('')
  const [errors, setErrors] = useState<BasicErrors>({})
  const [savingBasic, setSavingBasic] = useState(false)

  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirmText, setConfirmText] = useState('')
  const [savingPassword, setSavingPassword] = useState(false)

  const [mfa, setMfa] = useState<MFAData | null>(null)
  /** 0 = 只显示操作按钮，1 = 显示二维码 + 验证码（旧版 mfaMode）。 */
  const [mfaMode, setMfaMode] = useState<'options' | 'setup'>('options')
  const [mfaSecret, setMfaSecret] = useState('')
  const [mfaCode1, setMfaCode1] = useState('')
  const [mfaCode2, setMfaCode2] = useState('')
  const [savingMFA, setSavingMFA] = useState(false)

  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!user) {
      return
    }
    setName(user.name)
    setTeam(user.team ?? '')
    setRole(user.role ?? '')
    setEmail(user.email)
  }, [user])

  const mfaEnabled = Boolean(user?.mfaEnabled)
  const userLoaded = Boolean(user)

  // MFA 未开启时自动进入设置流程并拉取二维码（旧版 UserSettingGeneral.js:281-283）。
  // 已开启时不拉取 —— 只有点"更换设备"才需要重新生成 secret。
  useEffect(() => {
    if (!userLoaded || mfaEnabled) {
      return
    }
    let cancelled = false
    getMFAData()
      .then((data) => {
        if (cancelled) {
          return
        }
        setMfa(data)
        setMfaSecret(String(data.secret ?? ''))
        setMfaMode('setup')
      })
      .catch(() => {
        if (!cancelled) {
          setMfa(null)
        }
      })
    return () => {
      cancelled = true
    }
  }, [userLoaded, mfaEnabled])

  /** 旧版 UserSettingGeneral.js:99-148 的基本信息校验口径。 */
  const validateBasic = (): boolean => {
    const nextErrors: BasicErrors = {}
    if (!/^\S+$/.test(name)) {
      nextErrors.name = t('message.error._S_empty', { s: t('label.userName') })
    } else if (!/^.{2,50}$/.test(name)) {
      nextErrors.name = t('message.error.within_N1_to_N2_characters', { n1: 2, n2: 50 })
    }
    if (!email.trim()) {
      nextErrors.email = t('message.error._S_empty', { s: t('label.email') })
    } else if (!EMAIL_PATTERN.test(email.trim())) {
      nextErrors.email = t('message.error._S_invalid', { s: t('label.email') })
    }
    if (team.length > 50) {
      nextErrors.team = t('message.error.noMoreThan_N_characters', { n: 50 })
    }
    if (role.length > 50) {
      nextErrors.role = t('message.error.noMoreThan_N_characters', { n: 50 })
    }
    setErrors(nextErrors)
    return Object.keys(nextErrors).length === 0
  }

  const saveBasic = async () => {
    if (!user) {
      return
    }
    if (!validateBasic()) {
      return
    }
    setSavingBasic(true)
    try {
      // 后端 `user/updateBasicInfo` 支持 `role` 字段（user_profile.go:167），
      // 旧版资料页也是随基本信息一起提交 role。
      const payload: Parameters<typeof updateBasicInfo>[0] = {
        name: name.trim(),
        email: email.trim(),
        team: team.trim(),
        role: role.trim(),
      }
      await updateBasicInfo(payload)
      toast.success(t('message.updateSuccess'))
      setErrors({})
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSavingBasic(false)
    }
  }

  const savePassword = async () => {
    if (!current || !next) {
      toast.error(t('label.password'))
      return
    }
    if (next !== confirmText) {
      toast.error(t('user_form_password_re_error'))
      return
    }
    setSavingPassword(true)
    try {
      await updatePassword(current, next)
      toast.success(t('message.updateSuccess'))
      setCurrent('')
      setNext('')
      setConfirmText('')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSavingPassword(false)
    }
  }

  const onPickAvatar = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) {
      return
    }
    try {
      await uploadAvatar(file)
      toast.success(t('message.uploadSuccess'))
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  /** 旧版 UserSettingGeneral.js:340-356 —— 重新生成 secret + 二维码（"设置/更换设备"共用）。 */
  const startMFA = async () => {
    try {
      const data = await getMFAData()
      setMfa(data)
      setMfaSecret(String(data.secret ?? ''))
      setMfaCode1('')
      setMfaCode2('')
      setMfaMode('setup')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const enableMFA = async () => {
    // 旧版 UserSettingGeneral.js:171-207 + 376-384：两组验证码都必须是 6 位数字且互不相同
    if (!MFA_CODE_PATTERN.test(mfaCode1) || !MFA_CODE_PATTERN.test(mfaCode2)) {
      toast.error(t('message.error._S_invalid', { s: t('label.mfaCode') }))
      return
    }
    if (mfaCode1 === mfaCode2) {
      toast.error(t('message.error.inputSame'))
      return
    }
    setSavingMFA(true)
    try {
      await updateMFAData(mfaSecret, mfaCode1, mfaCode2)
      toast.success(t('message.updateSuccess'))
      setMfaMode('options')
      setMfaSecret('')
      setMfaCode1('')
      setMfaCode2('')
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSavingMFA(false)
    }
  }

  const disableMFA = async () => {
    const ok = await confirm({
      title: t('label.removeMFADevice'),
      message: t('message.confirmRemove'),
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    try {
      await revokeMFAData()
      toast.success(t('message.updateSuccess'))
      setMfaMode('options')
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.basicInfo')}</div>
        <div className="row">
          <Avatar size="lg" src={user?.icon ? `/file/fetch/avatar/${user.icon}` : undefined} name={user?.name} />
          <Button variant="outlined" onClick={() => fileRef.current?.click()}>
            {t('label.uploadAvatar')}
          </Button>
          <input ref={fileRef} type="file" accept="image/*" hidden onChange={onPickAvatar} />
        </div>
        <FormGrid>
          <FormItem label={t('label.userName')} required error={errors.name}>
            <TextField
              value={name}
              onChange={(event) => {
                setName(event.target.value)
                setErrors((prev) => ({ ...prev, name: undefined }))
              }}
            />
          </FormItem>
          <FormItem label={t('label.email')} required error={errors.email}>
            <TextField
              value={email}
              onChange={(event) => {
                setEmail(event.target.value)
                setErrors((prev) => ({ ...prev, email: undefined }))
              }}
            />
          </FormItem>
          <FormItem label={t('label.team')} error={errors.team}>
            <TextField
              value={team}
              onChange={(event) => {
                setTeam(event.target.value)
                setErrors((prev) => ({ ...prev, team: undefined }))
              }}
            />
          </FormItem>
          {/* 旧版：role 是可编辑输入框，并随 updateBasicInfo 一起提交 */}
          <FormItem label={t('label.role')} error={errors.role}>
            <TextField
              value={role}
              onChange={(event) => {
                setRole(event.target.value)
                setErrors((prev) => ({ ...prev, role: undefined }))
              }}
            />
          </FormItem>
        </FormGrid>
        <FormActions>
          <Button variant="contained" loading={savingBasic} onClick={saveBasic}>
            {t('label.save')}
          </Button>
        </FormActions>
      </section>

      <section className="section">
        <div className="section__title">{t('label.password')}</div>
        <Form onSubmit={savePassword}>
          <FormGrid>
            <FormItem label={t('label.oldPassword')} required>
              <TextField type="password" value={current} onChange={(event) => setCurrent(event.target.value)} />
            </FormItem>
            <FormItem label={t('label.newPassword')} required>
              <TextField type="password" value={next} onChange={(event) => setNext(event.target.value)} />
            </FormItem>
            <FormItem label={t('label.newPasswordConfirm')} required>
              <TextField type="password" value={confirmText} onChange={(event) => setConfirmText(event.target.value)} />
            </FormItem>
          </FormGrid>
          <FormActions>
            <Button variant="contained" type="submit" loading={savingPassword}>
              {t('label.save')}
            </Button>
          </FormActions>
        </Form>
      </section>

      <section className="section">
        <div className="section__title">{t('label.security')}</div>
        <div className="row">
          <Tag tone={mfaEnabled ? 'success' : 'default'}>{mfaEnabled ? t('label.enabledMFA') : t('label.disabledMFA')}</Tag>
          {mfaEnabled && mfaMode === 'options' ? (
            <>
              {/* 旧版 UserSettingGeneral.js:592-594：已开启时同时提供"更换设备"与"移除设备" */}
              <Button variant="outlined" onClick={startMFA}>
                {t('label.changeMFADevice')}
              </Button>
              <Button variant="outlined" tone="danger" onClick={disableMFA}>
                {t('label.removeMFADevice')}
              </Button>
            </>
          ) : null}
          {!mfaEnabled && mfaMode === 'options' ? (
            <Button variant="outlined" onClick={startMFA}>
              {t('label.setupNewMFADevice')}
            </Button>
          ) : null}
        </div>
        {mfaMode === 'setup' ? (
          <Form onSubmit={enableMFA}>
            <div className="stack">
              <span className="muted">{t('message.mfaGuide')}</span>
              {/* 旧版 UserSettingGeneral.js:603 —— 后端返回 SVG data URI，直接当图片渲染 */}
              {mfa?.image ? <img width={150} height={150} src={mfa.image} alt={t('label.twoFactorAuthentication')} /> : null}
              <FormItem label="secret" help={String(mfa?.secret ?? '')}>
                <TextField value={mfaSecret} onChange={(event) => setMfaSecret(event.target.value)} />
              </FormItem>
              <FormGrid>
                <FormItem label={t('label.mfaCode1')} required>
                  <TextField value={mfaCode1} onChange={(event) => setMfaCode1(event.target.value)} />
                </FormItem>
                <FormItem label={t('label.mfaCode2')} required>
                  <TextField value={mfaCode2} onChange={(event) => setMfaCode2(event.target.value)} />
                </FormItem>
              </FormGrid>
              <FormActions>
                <Button variant="contained" type="submit" loading={savingMFA}>
                  {t('label.save')}
                </Button>
              </FormActions>
            </div>
          </Form>
        ) : null}
      </section>
    </div>
  )
}
