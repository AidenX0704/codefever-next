import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import { useNavigate } from 'react-router'

import { ApiError } from '@/api/client'
import { updateGroup, uploadGroupAvatar } from '@/api/group'
import { Button } from '@/components/unit/Button'
import { Form, FormActions, FormItem } from '@/components/unit/Form'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { GroupAvatar } from '@/components/view/unit/GroupAvatar'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useGroup } from '@/features/group/GroupProvider'
import { useT } from '@/i18n'


/**
 * 仓库组设置 · 常规（`/groups/{组名}/settings/general`）。
 * 嵌入在 GroupLayout 的右侧主舞台，由左侧空间栏直达。
 */
export default function GroupSettingsGeneral() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { summary, config, refresh } = useGroup()
  const { refresh: refreshAppData } = useAppData()

  const [displayName, setDisplayName] = useState('')
  const [description, setDescription] = useState('')
  const [nameError, setNameError] = useState<string | null>(null)
  const [descError, setDescError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!config) {
      return
    }
    setDisplayName(config.group.displayName)
    setDescription(config.group.description ?? '')
    setNameError(null)
    setDescError(null)
  }, [config])

  const save = async () => {
    if (!summary) {
      return
    }
    setNameError(null)
    setDescError(null)
    const trimmed = displayName.trim()
    if (!trimmed) {
      setNameError(t('message.error._S_empty', { s: t('label.groupName') }))
      return
    }
    if (trimmed.length > 30) {
      setNameError(t('message.error.noMoreThan_N_characters', { n: 30 }))
      return
    }
    if (description.length > 200) {
      setDescError(t('message.error.noMoreThan_N_characters', { n: 200 }))
      return
    }
    setSaving(true)
    try {
      await updateGroup(summary.id, trimmed, description)
      toast.success(t('message.updateSuccess'))
      refresh()
      await refreshAppData()
    } catch (error) {
      const code = error instanceof ApiError ? error.code : 0
      if (code === 0x0404) {
        setNameError(t('group.displayNameDuplicate'))
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
      await refreshAppData()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  return (
    <div className="stage-stack">
      <div className="form-sheet form-sheet--full">
        <section className="section">
          <div className="section__title">{t('group.settingsGeneral')}</div>
          <div className="row">
            <GroupAvatar icon={config?.group.icon} name={config?.group.displayName ?? ''} size="lg" />
            <Button variant="outlined" onClick={() => fileRef.current?.click()}>
              {t('label.uploadAvatar')}
            </Button>
            <input ref={fileRef} type="file" accept="image/*" hidden onChange={onPickAvatar} />
            <span className="repo-row__branch">/{config?.group.name}</span>
          </div>
          <Form onSubmit={save}>
            <FormItem
              label={t('label.groupName')}
              required
              error={nameError}
              help={t('message.error.noMoreThan_N_characters', { n: 30 })}
            >
              <TextField
                value={displayName}
                onChange={(event) => {
                  setDisplayName(event.target.value)
                  setNameError(null)
                }}
              />
            </FormItem>
            <FormItem
              label={t('label.groupDescription')}
              error={descError}
              help={`${description.length}/200`}
            >
              <TextareaField
                value={description}
                onChange={(event) => {
                  setDescription(event.target.value)
                  setDescError(null)
                }}
                rows={4}
              />
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
    </div>
  )
}
