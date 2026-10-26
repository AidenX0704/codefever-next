import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { createRepository } from '@/api/repository'
import { Button } from '@/components/unit/Button'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { SelectField } from '@/components/unit/SelectField'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { makeSlug } from '@/helpers/slug'
import { useT } from '@/i18n'


/**
 * 新建仓库（`/repositories/new`、组内 `/groups/{组名}/repositories/new`）。
 *
 * 端点：`repository/create`（`group` + `name`(slug) + `displayName` + `description`）。
 *
 * 校验逐条对齐旧版 `NewRepository.js`：组必须选、显示名 ≤30 字且非空、
 * 标识名只允许字母数字下划线且 ≤30 字、描述 ≤200 字。
 */
export default function NewRepository() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { groupName } = useParams()
  const { groups, refresh } = useAppData()

  const currentGroup = useMemo(() => groups.find((group) => group.name === groupName), [groups, groupName])

  const [displayName, setDisplayName] = useState('')
  const [name, setName] = useState('')
  const [groupKey, setGroupKey] = useState(currentGroup?.id ?? '')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  const submit = async () => {
    if (!groupKey) {
      toast.error(t('label.group'))
      return
    }
    if (!displayName.trim()) {
      toast.error(t('label.repositoryName'))
      return
    }
    if (displayName.trim().length > 30) {
      toast.error(t('message.error.noMoreThan_N_characters', { n: 30 }))
      return
    }
    if (!name.trim()) {
      toast.error(t('label.repositorySlug'))
      return
    }
    if (!/^[0-9a-zA-Z_]+$/.test(name.trim())) {
      toast.error(t('message.error.requireCombinationOfCharactersNumbersAndUnderscore'))
      return
    }
    if (name.trim().length > 30) {
      toast.error(t('message.error.noMoreThan_N_characters', { n: 30 }))
      return
    }
    if (description.length > 200) {
      toast.error(t('message.error.noMoreThan_N_characters', { n: 200 }))
      return
    }
    setSaving(true)
    try {
      const repository = await createRepository({
        groupKey,
        name: name.trim(),
        displayName: displayName.trim(),
        description,
      })
      toast.success(t('message.addSuccess'))
      await refresh()
      navigate(`/${repository.group.name}/${repository.name}`)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.createRepository')}</div>
        <Form onSubmit={submit}>
          <FormGrid>
            <FormItem label={t('label.group')} required>
              <SelectField
                options={groups.map((group) => ({ value: group.id, label: group.displayName }))}
                value={groupKey || currentGroup?.id || ''}
                onChange={(event) => setGroupKey(event.target.value)}
              />
            </FormItem>
            <FormItem label={t('label.repositoryName')} required help={t('message.error.noMoreThan_N_characters', { n: 30 })}>
              <TextField
                value={displayName}
                onChange={(event) => {
                  const value = event.target.value
                  setDisplayName(value)
                  // 旧版 NewRepository.js:271-274：改仓库名时同步生成标识名。
                  setName(makeSlug(value))
                }}
              />
            </FormItem>
            <FormItem label={t('label.repositorySlug')} required help={t('message.error.noMoreThan_N_characters', { n: 30 })}>
              <TextField
                value={name}
                onChange={(event) => setName(makeSlug(event.target.value))}
                placeholder="my-repo"
              />
            </FormItem>
            <FormItem label={t('label.description')} help={`${description.length}/200`}>
              <TextareaField value={description} onChange={(event) => setDescription(event.target.value)} rows={5} />
            </FormItem>
          </FormGrid>
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
