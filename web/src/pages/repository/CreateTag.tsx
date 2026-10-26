import { useState } from 'react'
import { useNavigate } from 'react-router'

import { createTag } from '@/api/repository'
import { Button } from '@/components/unit/Button'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { SelectField } from '@/components/unit/SelectField'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'


/**
 * 新建标签（`/{组}/{仓库}/tags/new`）。
 *
 * 端点：`repository/createTag`（`repository` + `name` + `origin` + `description`）。
 */
export default function CreateTag() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { summary, config } = useRepository()

  const [name, setName] = useState('')
  const [origin, setOrigin] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  const origins = [...(config?.branches ?? []).map((branch) => branch.name), ...(config?.tags ?? []).map((tag) => tag.name)]

  const submit = async () => {
    if (!summary) {
      return
    }
    if (!name.trim()) {
      toast.error(t('label.tagName'))
      return
    }
    if (!origin) {
      toast.error(t('label.createOrigin'))
      return
    }
    setSaving(true)
    try {
      await createTag(summary.id, name.trim(), origin, description)
      toast.success(t('message.addSuccess'))
      navigate(`/${config?.group.name}/${config?.repository.name}/tags`)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.newTag')}</div>
        <Form onSubmit={submit}>
          <FormGrid>
            <FormItem label={t('label.tagName')} required>
              <TextField value={name} onChange={(event) => setName(event.target.value)} placeholder="v1.0.0" />
            </FormItem>
            <FormItem label={t('label.createOrigin')} required help={t('label.choseCreateOrigin')}>
              <SelectField
                options={origins.map((item) => ({ value: item, label: item }))}
                value={origin}
                onChange={(event) => setOrigin(event.target.value)}
              />
            </FormItem>
            <FormItem label={t('label.tagDescription')}>
              <TextareaField value={description} onChange={(event) => setDescription(event.target.value)} rows={4} />
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
