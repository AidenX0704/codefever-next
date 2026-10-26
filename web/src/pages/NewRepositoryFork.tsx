import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { forkRepository } from '@/api/repository'
import { Button } from '@/components/unit/Button'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { SelectField } from '@/components/unit/SelectField'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { makeSlug } from '@/helpers/slug'
import { useT } from '@/i18n'


/**
 * Fork 仓库（`/repositories/fork/{源仓库主键}`）。
 *
 * 端点：`repository/fork`（比 `create` 多一个 `forkID` = 源仓库主键）。
 * 名称/组/标识名/描述都预填自源仓库（逐字对齐旧版 `NewRepositoryFork.js`）。
 */
export default function NewRepositoryFork() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { repositoryId } = useParams()
  const { repositories, groups, refresh } = useAppData()

  const source = useMemo(() => repositories.find((item) => item.id === repositoryId), [repositories, repositoryId])

  const [displayName, setDisplayName] = useState('')
  const [name, setName] = useState('')
  const [groupKey, setGroupKey] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!source) {
      return
    }
    setDisplayName(source.displayName)
    setName(source.name)
    setGroupKey(source.group.id)
    setDescription(source.description ?? '')
  }, [source])

  const submit = async () => {
    if (!source) {
      return
    }
    if (!groupKey) {
      toast.error(t('label.group'))
      return
    }
    if (!displayName.trim()) {
      toast.error(t('label.repositoryName'))
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
    setSaving(true)
    try {
      const repository = await forkRepository({
        groupKey,
        name: name.trim(),
        displayName: displayName.trim(),
        description,
        forkID: source.id,
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
        <div className="section__title">
          {t('label.forkFrom_S', { s: source ? `${source.group.displayName}/${source.displayName}` : '' })}
        </div>
        <Form onSubmit={submit}>
          <FormGrid>
            <FormItem label={t('label.group')} required>
              <SelectField
                options={groups.map((group) => ({ value: group.id, label: group.displayName }))}
                value={groupKey}
                onChange={(event) => setGroupKey(event.target.value)}
              />
            </FormItem>
            <FormItem label={t('label.repositoryName')} required help={t('message.error.noMoreThan_N_characters', { n: 30 })}>
              <TextField
                value={displayName}
                onChange={(event) => {
                  const value = event.target.value
                  setDisplayName(value)
                  // 旧版 NewRepositoryFork.js:276-279：改仓库名时同步生成标识名。
                  setName(makeSlug(value))
                }}
              />
            </FormItem>
            <FormItem label={t('label.repositorySlug')} required help={t('message.error.noMoreThan_N_characters', { n: 30 })}>
              <TextField
                value={name}
                onChange={(event) => setName(makeSlug(event.target.value))}
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
