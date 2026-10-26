import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import { useNavigate, useParams } from 'react-router'

import { ApiError } from '@/api/client'
import { updateRepository, uploadRepositoryAvatar } from '@/api/repository'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { Form, FormActions, FormItem } from '@/components/unit/Form'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/** 名称 / 描述的长度上限（旧版 `RepositorySettingGeneral.js` 的校验器）。 */
const NAME_MAX = 30
const DESCRIPTION_MAX = 200

/**
 * 仓库设置 · 基本信息（`/{组}/{仓库}/settings/general`）。
 *
 * 端点：
 *   - `repository/update`：`repository` + `displayName`（必填）+ `description`（**必须传**，
 *     缺参数后端按 PHP 的历史行为返回 500，所以这里始终发送）；
 *   - `repository/uploadAvatar`：multipart，字段名 `avatar`。
 *
 * 与旧版对齐：名称 30 字 / 描述 200 字校验、描述字数计数 n/200、
 * `0x0402`（仓库名重复）的中文提示，以及"取消"回退按钮。
 */
export default function SettingsGeneral() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { groupName, repositoryName } = useParams()
  const { summary, config, refresh } = useRepository()
  const { refresh: refreshAppData } = useAppData()

  const [displayName, setDisplayName] = useState('')
  const [description, setDescription] = useState('')
  const [nameError, setNameError] = useState<string | null>(null)
  const [descriptionError, setDescriptionError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!config) {
      return
    }
    setDisplayName(config.repository.displayName)
    setDescription(config.repository.description ?? '')
  }, [config])

  const save = async () => {
    if (!summary) {
      return
    }
    if (!displayName.trim()) {
      setNameError(t('message.error._S_empty', { s: t('label.repositoryName') }))
      return
    }
    if (displayName.trim().length > NAME_MAX) {
      setNameError(t('message.error.noMoreThan_N_characters', { n: NAME_MAX }))
      return
    }
    if (description.length > DESCRIPTION_MAX) {
      setDescriptionError(t('message.error.noMoreThan_N_characters', { n: DESCRIPTION_MAX }))
      return
    }
    setNameError(null)
    setDescriptionError(null)
    setSaving(true)
    try {
      await updateRepository(summary.id, displayName.trim(), description)
      toast.success(t('message.updateSuccess'))
      refresh()
      refreshAppData()
    } catch (error) {
      // 0x0402：仓库显示名重复。
      if (error instanceof ApiError && error.code === 0x0402) {
        setNameError(t('message.error._S_duplicate', { s: t('label.repositoryName') }))
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
      await uploadRepositoryAvatar(summary.id, file)
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
          <Avatar square size="lg" src={config?.repository.icon ? `/file/fetch/avatar/${config.repository.icon}` : undefined} name={config?.repository.name} />
          <Button variant="outlined" onClick={() => fileRef.current?.click()}>
            {t('label.uploadAvatar')}
          </Button>
          <input ref={fileRef} type="file" accept="image/*" hidden onChange={onPickAvatar} />
          <span className="muted">
            {config?.group.displayName}/{config?.repository.name}
          </span>
        </div>
        <Form onSubmit={save}>
          <FormItem label={t('label.repositoryName')} required error={nameError}>
            <TextField
              value={displayName}
              onChange={(event) => {
                setDisplayName(event.target.value)
                setNameError(null)
              }}
            />
          </FormItem>
          <FormItem
            label={t('label.repositoryDescription')}
            error={descriptionError}
            help={`${description.length}/${DESCRIPTION_MAX}`}
          >
            <TextareaField
              value={description}
              onChange={(event) => {
                setDescription(event.target.value)
                setDescriptionError(null)
              }}
              rows={4}
            />
          </FormItem>
          <FormActions>
            <Button variant="contained" type="submit" loading={saving}>
              {t('label.update')}
            </Button>
            <Button
              variant="outlined"
              type="button"
              onClick={() => navigate(`/${groupName}/${repositoryName}/files`)}
            >
              {t('label.cancel')}
            </Button>
          </FormActions>
        </Form>
      </section>
    </div>
  )
}
