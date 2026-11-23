import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router'

import { ApiError } from '@/api/client'
import { GROUP_TYPE, checkGroupName, createGroup } from '@/api/group'
import { Button } from '@/components/unit/Button'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { Icon } from '@/components/unit/Icon'
import { Marker } from '@/components/unit/Marker'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { GroupAvatar } from '@/components/view/unit/GroupAvatar'
import { arrowLeftIcon } from '@/config/icons'
import { Role } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { GROUP_SLUG_RE, isReservedGroupSlug, makeSlug, suggestSlug } from '@/helpers/slug'
import { useT } from '@/i18n'


/**
 * 新建仓库组（`/groups/new`）—— 双栏创作台设计（左表单 + 右实时门户卡预览与说明）。
 *
 * 修复：使用 `prefilledRef` 确保首次个人组邮箱前缀预填仅执行一次，
 * 用户清空输入框后绝不会再自动重新填回 `root`。
 */
export default function NewGroup() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { user, groups, loading, refresh } = useAppData()

  const hasPersonalGroup = useMemo(() => groups.some((group) => group.type === GROUP_TYPE.user), [groups])
  const type = hasPersonalGroup ? GROUP_TYPE.normal : GROUP_TYPE.user

  const [displayName, setDisplayName] = useState('')
  const [name, setName] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  const [displayNameError, setDisplayNameError] = useState<string | null>(null)
  const [nameError, setNameError] = useState<string | null>(null)
  const [descError, setDescError] = useState<string | null>(null)
  const prefilledRef = useRef(false)

  // 仅在首次加载完毕且用户尚未建过个人组时预填一次，删除内容后不再回填。
  useEffect(() => {
    if (prefilledRef.current || loading || hasPersonalGroup || !user?.email) {
      return
    }
    prefilledRef.current = true
    const prefix = user.email.split('@')[0]
    setDisplayName(prefix)
    setName(suggestSlug(prefix))
  }, [loading, hasPersonalGroup, user?.email])

  const host = (user?.host ?? window.location.origin).replace(/\/+$/, '')
  const previewSlug = name.trim() || 'platform-team'
  const previewDisplayName = displayName.trim() || t('label.groupName')

  const verifySlugRemote = async (slug: string) => {
    const trimmed = slug.trim()
    if (!trimmed) {
      return
    }
    if (!GROUP_SLUG_RE.test(trimmed)) {
      setNameError(t('group.slugRule'))
      return
    }
    if (isReservedGroupSlug(trimmed)) {
      setNameError(t('group.slugReserved'))
      return
    }
    try {
      const res = await checkGroupName({ name: trimmed })
      if (!res.available) {
        if (res.reason === 'preserved') {
          setNameError(t('group.slugReserved'))
        } else if (res.reason === 'duplicate_slug') {
          setNameError(t('group.slugDuplicate'))
        } else {
          setNameError(t('group.slugRule'))
        }
      }
    } catch {
      /* 失焦查重失败不阻断提交 */
    }
  }

  const verifyDisplayNameRemote = async (title: string) => {
    const trimmed = title.trim()
    if (!trimmed || trimmed.length > 30) {
      return
    }
    try {
      const res = await checkGroupName({ displayName: trimmed })
      if (!res.available && res.reason === 'duplicate_display_name') {
        setDisplayNameError(t('group.displayNameDuplicate'))
      }
    } catch {
      /* 静默降级 */
    }
  }

  const submit = async () => {
    setDisplayNameError(null)
    setNameError(null)
    setDescError(null)

    const trimmedDisplay = displayName.trim()
    const trimmedName = name.trim()

    let invalid = false
    if (!trimmedDisplay) {
      setDisplayNameError(t('message.error._S_empty', { s: t('label.groupName') }))
      invalid = true
    } else if (trimmedDisplay.length > 30) {
      setDisplayNameError(t('message.error.noMoreThan_N_characters', { n: 30 }))
      invalid = true
    }
    if (!trimmedName) {
      setNameError(t('message.error._S_empty', { s: t('label.groupURL') }))
      invalid = true
    } else if (!GROUP_SLUG_RE.test(trimmedName)) {
      setNameError(t('group.slugRule'))
      invalid = true
    } else if (isReservedGroupSlug(trimmedName)) {
      setNameError(t('group.slugReserved'))
      invalid = true
    }
    if (description.length > 200) {
      setDescError(t('message.error.noMoreThan_N_characters', { n: 200 }))
      invalid = true
    }
    if (invalid) {
      return
    }

    setSaving(true)
    try {
      const group = await createGroup({
        name: trimmedName,
        displayName: trimmedDisplay,
        description,
        type,
      })
      toast.success(t('message.addSuccess'))
      await refresh()
      navigate(`/groups/${group.name}`)
    } catch (error) {
      if (error instanceof ApiError) {
        if (error.code === 0x0403) {
          setNameError(t('group.slugDuplicate'))
          return
        }
        if (error.code === 0x0404) {
          setDisplayNameError(t('group.displayNameDuplicate'))
          return
        }
      }
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="page">
      <div className="studio-header">
        <button type="button" className="group-rail__back" onClick={() => navigate('/groups')}>
          <Icon icon={arrowLeftIcon} size={14} />
          <span>{t('label.group')}</span>
        </button>
        <h1 className="page__title">{t('label.newGroup')}</h1>
        <p className="page__subtitle">{t('group.createSubtitle')}</p>
      </div>

      <div className="create-studio">
        <div className="form-sheet form-sheet--full">
          {type === GROUP_TYPE.user ? <div className="form-banner">{t('group.personalNoticeDetail')}</div> : null}

          <Form onSubmit={submit}>
            <FormGrid>
              <FormItem
                label={t('label.groupName')}
                required
                error={displayNameError}
                help={t('message.error.noMoreThan_N_characters', { n: 30 })}
              >
                <TextField
                  value={displayName}
                  placeholder="例如：平台研发组 / Platform Team"
                  onChange={(event) => {
                    const value = event.target.value
                    prefilledRef.current = true
                    setDisplayName(value)
                    setDisplayNameError(null)
                    if (!slugTouched) {
                      setName(suggestSlug(value))
                      setNameError(null)
                    }
                  }}
                  onBlur={() => void verifyDisplayNameRemote(displayName)}
                />
              </FormItem>

              <FormItem
                label={t('label.groupURL')}
                required
                error={nameError}
                help={t('group.slugRule')}
              >
                <TextField
                  value={name}
                  placeholder="platform-team"
                  startAdornment={<span className="muted">{host}/</span>}
                  onChange={(event) => {
                    prefilledRef.current = true
                    setSlugTouched(true)
                    setName(makeSlug(event.target.value))
                    setNameError(null)
                  }}
                  onBlur={() => void verifySlugRemote(name)}
                />
              </FormItem>

              <FormItem
                label={t('label.groupDescription')}
                error={descError}
                help={`${description.length}/200`}
              >
                <TextareaField
                  value={description}
                  placeholder="简要说明该仓库组负责的业务域或团队职责（可选）"
                  onChange={(event) => {
                    setDescription(event.target.value)
                    setDescError(null)
                  }}
                  rows={4}
                />
              </FormItem>
            </FormGrid>

            <FormActions>
              <Button variant="contained" type="submit" loading={saving}>
                {t('label.newGroup')}
              </Button>
              <Button variant="outlined" type="button" onClick={() => navigate('/groups')}>
                {t('label.cancel')}
              </Button>
            </FormActions>
          </Form>
        </div>

        <aside className="create-studio__aside">
          <div className="preview-panel">
            <div className="preview-panel__title">{t('group.previewTitle')}</div>
            <div className="group-portal group-portal--static">
              <div className="group-portal__head">
                <div className="group-portal__identity">
                  <GroupAvatar name={previewDisplayName} size="md" />
                  <div className="group-portal__meta">
                    <div className="group-portal__title-row">
                      <span className="group-portal__title">{previewDisplayName}</span>
                      <span className="repo-row__chip">
                        {type === GROUP_TYPE.user ? t('group.personalTag') : t('group.teamTag')}
                      </span>
                    </div>
                    <div className="group-portal__sub">
                      <span className="group-card__slug">/{previewSlug}</span>
                      <Marker variant="containedInfo" text={t(`label.roleID_${Role.OWNER}`)} />
                    </div>
                  </div>
                </div>
              </div>
              <div className="group-portal__desc">{description.trim() || '—'}</div>
              <div className="group-portal__metrics">
                <div className="group-portal__metric">
                  <span className="group-portal__metric-val">0</span>
                  <span className="group-portal__metric-lbl">{t('label.repository_pl')}</span>
                </div>
                <div className="group-portal__metric">
                  <span className="group-portal__metric-val">0</span>
                  <span className="group-portal__metric-lbl">{t('label.mrStatusPending')}</span>
                </div>
                <div className="group-portal__metric">
                  <span className="group-portal__metric-val">1</span>
                  <span className="group-portal__metric-lbl">{t('label.memberCount')}</span>
                </div>
              </div>
            </div>

            <div className="url-preview">
              <div className="url-preview__label">{t('group.urlPreview')}</div>
              <div className="url-preview__line">{`${host}/groups/${previewSlug}`}</div>
              <div className="url-preview__line">{`${host}/${previewSlug}/<repo>.git`}</div>
            </div>

            <div className="preview-tips">
              <div className="preview-tips__title">{t('group.tipTitle')}</div>
              <ul className="preview-tips__list">
                <li>{t('group.tipRule1')}</li>
                <li>{t('group.tipRule2')}</li>
              </ul>
            </div>
          </div>
        </aside>
      </div>
    </div>
  )
}
