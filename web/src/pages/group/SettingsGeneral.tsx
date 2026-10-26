import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import { useNavigate } from 'react-router'

import { ApiError } from '@/api/client'
import { updateGroup, uploadGroupAvatar } from '@/api/group'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { Form, FormActions, FormItem } from '@/components/unit/Form'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useGroup } from '@/features/group/GroupProvider'
import { useT } from '@/i18n'


/**
 * 仓库组设置 · 常规（`/groups/{组名}/settings/general`）。
 *
 * 端点：`group/update`（`group` + `displayName` 必填 + `description` 必须传）
 * 与 `group/uploadAvatar`（multipart，字段 `avatar`）。
 *
 * 校验逐条对齐旧版 `GroupSettingGeneral.js` 的 `ValidatorGenerator.stateValidator`：
 * 组名非空、5–30 字、只允许「字母/数字/下划线/中划线/点」；描述 ≤200 字。
 * 错误码 `0x0404` = 组名重复。
 */
export default function GroupSettingsGeneral() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { summary, config, refresh } = useGroup()
  const { refresh: refreshAppData } = useAppData()

  const [displayName, setDisplayName] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!config) {
      return
    }
    setDisplayName(config.group.displayName)
    setDescription(config.group.description ?? '')
  }, [config])

  const save = async () => {
    if (!summary) {
      return
    }
    const name = displayName.trim()
    if (!name) {
      toast.error(t('message.error._S_empty', { s: t('label.groupName') }))
      return
    }
    if (name.length < 5 || name.length > 30) {
      toast.error(t('message.error.within_N1_to_N2_characters', { n1: 5, n2: 30 }))
      return
    }
    if (!/^[0-9a-zA-Z._-]+$/.test(name)) {
      toast.error(t('message.error._S_has_illegal_character', { s: t('label.groupName') }))
      return
    }
    if (description.length > 200) {
      toast.error(t('message.error.noMoreThan_N_characters', { n: 200 }))
      return
    }
    setSaving(true)
    try {
      await updateGroup(summary.id, name, description)
      toast.success(t('message.updateSuccess'))
      refresh()
      refreshAppData()
    } catch (error) {
      const code = error instanceof ApiError ? error.code : 0
      if (code === 0x0404) {
        toast.error(t('message.error._S_duplicate', { s: t('label.groupName') }))
      } else {
        toast.error(error instanceof Error ? error.message : t('base_error_message'))
      }
    } finally {
      setSaving(false)
    }
  }

  const onPickAvatar = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file || !summary) {
      return
    }
    try {
      await uploadGroupAvatar(summary.id, file)
      toast.success(t('message.uploadSuccess'))
      refresh()
      refreshAppData()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.basicInfo')}</div>
        <div className="row">
          <Avatar square size="lg" src={config?.group.icon ? `/file/fetch/avatar/${config.group.icon}` : undefined} name={config?.group.name} />
          <Button variant="outlined" onClick={() => fileRef.current?.click()}>
            {t('label.uploadAvatar')}
          </Button>
          <input ref={fileRef} type="file" accept="image/*" hidden onChange={onPickAvatar} />
          <span className="muted">{config?.group.name}</span>
        </div>
        <Form onSubmit={save}>
          <FormItem label={t('label.groupName')} required help={t('message.error.within_N1_to_N2_characters', { n1: 5, n2: 30 })}>
            <TextField value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
          </FormItem>
          <FormItem label={t('label.groupDescription')} help={`${description.length}/200`}>
            <TextareaField value={description} onChange={(event) => setDescription(event.target.value)} rows={4} />
          </FormItem>
          <FormActions>
            <Button variant="contained" type="submit" loading={saving}>
              {t('label.save')}
            </Button>
            <Button variant="outlined" type="button" onClick={() => navigate(-1)}>
              {t('label.cancel')}
            </Button>
          </FormActions>
        </Form>
      </section>
    </div>
  )
}
