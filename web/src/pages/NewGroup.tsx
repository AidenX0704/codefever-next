import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'

import { createGroup, GROUP_TYPE } from '@/api/group'
import { Button } from '@/components/unit/Button'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/** 把任意文本转成可用的标识名（字母数字下划线）。 */
function makeSlug(value: string): string {
  return value.replace(/[^0-9a-zA-Z_]/g, '_')
}

/**
 * 新建仓库组（`/groups/new`）。
 *
 * 端点：`group/create`（`displayName` + `name`(slug) + `type` 三者缺一即 0x0201）。
 *
 * 组类型沿用旧版 `NewGroup.js` 的判定：**用户还没有个人分组时强制建个人组**
 * （`type=2`，组名取邮箱前缀），否则建普通组（`type=1`）。
 */
export default function NewGroup() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { user, groups, refresh } = useAppData()

  const hasPersonalGroup = useMemo(() => groups.some((group) => group.type === GROUP_TYPE.user), [groups])
  const type = hasPersonalGroup ? GROUP_TYPE.normal : GROUP_TYPE.user

  const [displayName, setDisplayName] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!hasPersonalGroup && user?.email) {
      const prefix = user.email.split('@')[0]
      setDisplayName(prefix)
      setName(makeSlug(prefix))
    }
  }, [hasPersonalGroup, user?.email])

  const submit = async () => {
    if (!displayName.trim()) {
      toast.error(t('label.groupName'))
      return
    }
    if (displayName.trim().length > 30) {
      toast.error(t('message.error.noMoreThan_N_characters', { n: 30 }))
      return
    }
    if (!name.trim()) {
      toast.error(t('label.groupSlug'))
      return
    }
    if (!/^[0-9a-zA-Z_]+$/.test(name.trim())) {
      toast.error(t('message.error.requireCombinationOfCharactersNumbersAndUnderscore'))
      return
    }
    if (name.trim().length < 5 || name.trim().length > 30) {
      toast.error(t('message.error.within_N1_to_N2_characters', { n1: 5, n2: 30 }))
      return
    }
    if (description.length > 200) {
      toast.error(t('message.error.noMoreThan_N_characters', { n: 200 }))
      return
    }
    setSaving(true)
    try {
      const group = await createGroup({
        name: name.trim(),
        displayName: displayName.trim(),
        description,
        type,
      })
      toast.success(t('message.addSuccess'))
      await refresh()
      navigate(`/groups/${group.name}`)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.newGroup')}</div>
        {type === GROUP_TYPE.user ? <div className="muted">{t('message.createPersonalGroupNotice')}</div> : null}
        <Form onSubmit={submit}>
          <FormGrid>
            <FormItem label={t('label.groupName')} required help={t('message.error.noMoreThan_N_characters', { n: 30 })}>
              <TextField value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
            </FormItem>
            <FormItem label={t('label.groupURL')} required help={t('message.error.within_N1_to_N2_characters', { n1: 5, n2: 30 })}>
              <TextField
                value={name}
                onChange={(event) => setName(makeSlug(event.target.value))}
                placeholder="my-group"
              />
            </FormItem>
            <FormItem label={t('label.groupDescription')} help={`${description.length}/200`}>
              <TextareaField value={description} onChange={(event) => setDescription(event.target.value)} rows={5} />
            </FormItem>
          </FormGrid>
          <FormActions>
            <Button variant="contained" type="submit" loading={saving}>
              {t('label.ok')}
            </Button>
            <Button variant="outlined" type="button" onClick={() => navigate('/groups')}>
              {t('label.cancel')}
            </Button>
          </FormActions>
        </Form>
      </section>
    </div>
  )
}
