import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'

import { ApiError } from '@/api/client'
import { GROUP_TYPE } from '@/api/group'
import { checkRepositoryName } from '@/api/repository'
import type { RepositoryListItem } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { Icon } from '@/components/unit/Icon'
import { Marker } from '@/components/unit/Marker'
import { SelectField } from '@/components/unit/SelectField'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { forkIcon, psBranch } from '@/config/icons'
import { PermissionCode, Role, hasPermission } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { cloneUrls } from '@/helpers/cloneUrl'
import { BRANCH_NAME_RE, REPO_SLUG_RE, makeSlug, suggestSlug } from '@/helpers/slug'
import { useT } from '@/i18n'


export interface RepositoryFormValues {
  groupKey: string
  displayName: string
  name: string
  description: string
  initReadme: boolean
  defaultBranch: string
  gitignore: string
}

export interface RepositoryFormProps {
  mode: 'create' | 'fork'
  initialGroupName?: string
  sourceRepository?: RepositoryListItem
  onSubmit: (values: RepositoryFormValues) => Promise<void>
}

const GITIGNORE_OPTIONS = [
  { value: '', labelKey: 'repo.gitignoreNone' },
  { value: 'go', label: 'Go' },
  { value: 'node', label: 'Node.js / TypeScript' },
  { value: 'python', label: 'Python' },
  { value: 'java', label: 'Java' },
  { value: 'rust', label: 'Rust' },
  { value: 'cpp', label: 'C / C++' },
] as const

/**
 * 新建仓库 / Fork 仓库双栏创作台（左表单 + 右实时预览卡与 Git 地址预览）。
 */
export function RepositoryForm({ mode, initialGroupName, sourceRepository, onSubmit }: RepositoryFormProps) {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { user, groups, loading } = useAppData()

  const writableGroups = useMemo(
    () => groups.filter((group) => hasPermission(group.role, PermissionCode.GROUP_CREATE_REPO)),
    [groups],
  )

  const [groupKey, setGroupKey] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [name, setName] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [description, setDescription] = useState('')
  const [initReadme, setInitReadme] = useState(false)
  const [defaultBranch, setDefaultBranch] = useState('main')
  const [gitignore, setGitignore] = useState('')
  const [saving, setSaving] = useState(false)

  const [groupError, setGroupError] = useState<string | null>(null)
  const [displayNameError, setDisplayNameError] = useState<string | null>(null)
  const [nameError, setNameError] = useState<string | null>(null)
  const [descError, setDescError] = useState<string | null>(null)
  const [branchError, setBranchError] = useState<string | null>(null)

  useEffect(() => {
    if (groupKey && writableGroups.some((g) => g.id === groupKey)) {
      return
    }
    if (mode === 'fork' && sourceRepository) {
      const preferred =
        writableGroups.find((g) => g.type === GROUP_TYPE.user && g.id !== sourceRepository.group.id) ??
        writableGroups.find((g) => g.id === sourceRepository.group.id) ??
        writableGroups[0]
      if (preferred) {
        setGroupKey(preferred.id)
      }
      return
    }
    const routeGroup = initialGroupName
      ? writableGroups.find((g) => g.name === initialGroupName)
      : undefined
    const fallback =
      routeGroup ?? writableGroups.find((g) => g.type === GROUP_TYPE.user) ?? writableGroups[0]
    if (fallback) {
      setGroupKey(fallback.id)
    }
  }, [writableGroups, initialGroupName, groupKey, mode, sourceRepository])

  useEffect(() => {
    if (mode !== 'fork' || !sourceRepository) {
      return
    }
    setDisplayName(sourceRepository.displayName)
    setName(sourceRepository.name)
    setSlugTouched(true)
    setDescription(sourceRepository.description ?? '')
  }, [mode, sourceRepository])

  const selectedGroup = useMemo(
    () => writableGroups.find((group) => group.id === groupKey),
    [writableGroups, groupKey],
  )

  const previewUrls = useMemo(() => {
    const gSlug = selectedGroup?.name || 'group'
    const rSlug = name.trim() || 'my-service'
    return cloneUrls(user, gSlug, rSlug)
  }, [user, selectedGroup?.name, name])

  const verifySlugRemote = async (targetGroupKey: string, slug: string) => {
    const trimmed = slug.trim()
    if (!targetGroupKey || !trimmed || !REPO_SLUG_RE.test(trimmed)) {
      return
    }
    try {
      const res = await checkRepositoryName({ groupKey: targetGroupKey, name: trimmed })
      if (!res.available) {
        setNameError(
          res.reason === 'duplicate_slug' ? t('repo.slugDuplicate') : t('repo.slugRule'),
        )
      }
    } catch {
      /* 失焦查重失败不阻断提交 */
    }
  }

  const verifyDisplayNameRemote = async (targetGroupKey: string, title: string) => {
    const trimmed = title.trim()
    if (!targetGroupKey || !trimmed || trimmed.length > 30) {
      return
    }
    try {
      const res = await checkRepositoryName({ groupKey: targetGroupKey, displayName: trimmed })
      if (!res.available && res.reason === 'duplicate_display_name') {
        setDisplayNameError(t('repo.displayNameDuplicate'))
      }
    } catch {
      /* 静默降级 */
    }
  }

  const handleSubmit = async () => {
    setGroupError(null)
    setDisplayNameError(null)
    setNameError(null)
    setDescError(null)
    setBranchError(null)

    const trimmedDisplay = displayName.trim()
    const trimmedName = name.trim()
    const trimmedBranch = defaultBranch.trim()

    let invalid = false
    if (!groupKey) {
      setGroupError(t('message.error._S_empty', { s: t('label.group') }))
      invalid = true
    }
    if (!trimmedDisplay) {
      setDisplayNameError(t('message.error._S_empty', { s: t('label.repositoryName') }))
      invalid = true
    } else if (trimmedDisplay.length > 30) {
      setDisplayNameError(t('message.error.noMoreThan_N_characters', { n: 30 }))
      invalid = true
    }
    if (!trimmedName) {
      setNameError(t('message.error._S_empty', { s: t('label.repositorySlug') }))
      invalid = true
    } else if (!REPO_SLUG_RE.test(trimmedName)) {
      setNameError(t('repo.slugRule'))
      invalid = true
    }
    if (description.length > 200) {
      setDescError(t('message.error.noMoreThan_N_characters', { n: 200 }))
      invalid = true
    }
    if (mode === 'create' && trimmedBranch && !BRANCH_NAME_RE.test(trimmedBranch)) {
      setBranchError(t('message.error.noMoreThan_N_characters', { n: 30 }))
      invalid = true
    }
    if (invalid) {
      return
    }

    setSaving(true)
    try {
      await onSubmit({
        groupKey,
        displayName: trimmedDisplay,
        name: trimmedName,
        description,
        initReadme: mode === 'create' ? initReadme : false,
        defaultBranch: mode === 'create' ? trimmedBranch || 'main' : '',
        gitignore: mode === 'create' ? gitignore : '',
      })
    } catch (error) {
      if (error instanceof ApiError) {
        if (error.code === 0x0401) {
          setNameError(t('repo.slugDuplicate'))
          return
        }
        if (error.code === 0x0402) {
          setDisplayNameError(t('repo.displayNameDuplicate'))
          return
        }
      }
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSaving(false)
    }
  }

  if (!loading && writableGroups.length === 0) {
    return (
      <div className="form-sheet">
        <EmptyState
          title={t('repo.noWritableGroupTitle')}
          notice={t('repo.noWritableGroupNotice')}
          action={
            <Button variant="contained" onClick={() => navigate('/groups/new')}>
              {t('repo.goCreateGroup')}
            </Button>
          }
        />
      </div>
    )
  }

  const willInitialize = mode === 'create' && (initReadme || Boolean(gitignore))
  const previewTitle = displayName.trim() || t('label.repositoryName')
  const previewSlug = name.trim() || 'my-service'

  return (
    <div className="create-studio">
      <div className="form-sheet form-sheet--full">
        {mode === 'fork' && sourceRepository ? (
          <div className="form-banner">
            <Icon icon={forkIcon} size={16} />
            <span>
              {t('repo.sourceRepo')}：
              <strong>
                {sourceRepository.group.displayName} / {sourceRepository.displayName}
              </strong>
            </span>
          </div>
        ) : null}

        <Form onSubmit={handleSubmit}>
          <div className="form-section-head">{t('repo.sectionBasic')}</div>
          <FormGrid>
            <FormItem
              label={t('label.repositoryName')}
              required
              error={displayNameError}
              help={t('message.error.noMoreThan_N_characters', { n: 30 })}
            >
              <TextField
                value={displayName}
                placeholder={mode === 'create' ? '例如：订单中心 / Order Service' : undefined}
                onChange={(event) => {
                  const value = event.target.value
                  setDisplayName(value)
                  setDisplayNameError(null)
                  if (!slugTouched) {
                    setName(suggestSlug(value))
                    setNameError(null)
                  }
                }}
                onBlur={() => void verifyDisplayNameRemote(groupKey, displayName)}
              />
            </FormItem>

            <div className="form-section-head">{t('repo.sectionPath')}</div>
            <div className="form-path-row">
              <FormItem label={t('label.group')} required error={groupError}>
                <SelectField
                  options={writableGroups.map((group) => ({
                    value: group.id,
                    label: group.displayName,
                    description:
                      group.type === GROUP_TYPE.user
                        ? `/${group.name} · ${t('group.personalTag')}`
                        : `/${group.name}`,
                  }))}
                  value={groupKey}
                  onChange={(event) => {
                    const nextKey = event.target.value
                    setGroupKey(nextKey)
                    setGroupError(null)
                    setNameError(null)
                    setDisplayNameError(null)
                    void verifySlugRemote(nextKey, name)
                  }}
                />
              </FormItem>
              <span className="form-path-row__sep" aria-hidden="true">/</span>
              <FormItem
                label={t('label.repositorySlug')}
                required
                error={nameError}
                help={t('repo.slugRule')}
              >
                <TextField
                  value={name}
                  placeholder="my-service"
                  onChange={(event) => {
                    setSlugTouched(true)
                    setName(makeSlug(event.target.value))
                    setNameError(null)
                  }}
                  onBlur={() => void verifySlugRemote(groupKey, name)}
                />
              </FormItem>
            </div>

            <FormItem
              label={t('label.description')}
              error={descError}
              help={`${description.length}/200`}
            >
              <TextareaField
                value={description}
                placeholder="简要说明仓库用途与技术栈（可选）"
                onChange={(event) => {
                  setDescription(event.target.value)
                  setDescError(null)
                }}
                rows={4}
              />
            </FormItem>

            {mode === 'create' ? (
              <div className="form-init-box">
                <div className="form-init-box__title">{t('repo.initSection')}</div>
                <label className="form-checkbox">
                  <input
                    type="checkbox"
                    checked={initReadme}
                    onChange={(event) => setInitReadme(event.target.checked)}
                  />
                  <span className="form-checkbox__text">
                    <span className="form-checkbox__label">{t('repo.initReadme')}</span>
                    <span className="form-checkbox__help">{t('repo.initReadmeHelp')}</span>
                  </span>
                </label>

                <div className="form-init-box__grid">
                  <FormItem label={t('repo.defaultBranch')} error={branchError}>
                    <TextField
                      value={defaultBranch}
                      placeholder="main"
                      onChange={(event) => {
                        setDefaultBranch(event.target.value)
                        setBranchError(null)
                      }}
                    />
                  </FormItem>
                  <FormItem label={t('repo.gitignore')}>
                    <SelectField
                      value={gitignore}
                      options={GITIGNORE_OPTIONS.map((opt) => ({
                        value: opt.value,
                        label: 'labelKey' in opt ? t(opt.labelKey) : opt.label,
                      }))}
                      onChange={(event) => setGitignore(event.target.value)}
                    />
                  </FormItem>
                </div>
              </div>
            ) : null}
          </FormGrid>

          <FormActions>
            <Button variant="contained" type="submit" loading={saving}>
              {mode === 'fork' ? t('label.fork') : t('label.createRepository')}
            </Button>
            <Button variant="outlined" type="button" onClick={() => navigate(-1)}>
              {t('label.cancel')}
            </Button>
          </FormActions>
        </Form>
      </div>

      <aside className="create-studio__aside">
        <div className="preview-panel">
          <div className="preview-panel__title">{t('repo.previewTitle')}</div>
          <div className="repo-card repo-card--static">
            <div className="repo-card__head">
              <div className="repo-card__identity">
                <span className="repo-row__avatar">{previewSlug.slice(0, 1).toUpperCase()}</span>
                <div className="repo-card__titles">
                  <span className="repo-card__group-link">
                    {selectedGroup?.displayName ?? t('label.group')}
                  </span>
                  <div className="repo-card__name">{previewTitle}</div>
                </div>
              </div>
            </div>
            <div className="repo-card__desc">{description.trim() || '—'}</div>
            <div className="repo-card__tags">
              {willInitialize ? (
                <span className="repo-row__chip repo-row__chip--mono">
                  <Icon icon={psBranch} size={12} />
                  {defaultBranch.trim() || 'main'}
                </span>
              ) : (
                <span className="repo-row__chip">{t('repo.emptyBranch')}</span>
              )}
              {mode === 'fork' ? (
                <span className="repo-row__chip">
                  <Icon icon={forkIcon} size={12} />
                  {t('repo.forkBadge')}
                </span>
              ) : null}
              <Marker variant="containedInfo" text={t(`label.roleID_${Role.OWNER}`)} />
            </div>
          </div>

          <div className="url-preview">
            <div className="url-preview__label">{t('repo.clonePreview')}</div>
            <div className="url-preview__line">{previewUrls.https}</div>
            {previewUrls.ssh ? <div className="url-preview__line">{previewUrls.ssh}</div> : null}
          </div>

          {mode === 'create' ? (
            <div className="preview-tips">
              <div className="preview-tips__title">{t('repo.stepTipTitle')}</div>
              <p className="preview-tips__body">
                {willInitialize ? t('repo.stepTipInit') : t('repo.stepTipEmpty')}
              </p>
            </div>
          ) : null}
        </div>
      </aside>
    </div>
  )
}
