import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { ApiError } from '@/api/client'
import { createMergeRequest, fetchBranchList, fetchCommitList } from '@/api/repository'
import type { Commit as CommitSummary } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { Checkbox } from '@/components/unit/Choices'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { SelectField } from '@/components/unit/SelectField'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { Role } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useOptionalRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'


export interface MergeRequestCreateProps {
  /** 全局新建（`/mergerequests/new`）：需要自己选源/目标仓库。 */
  global?: boolean
}

/**
 * 创建合并请求。
 *
 * `createMergeRequest` 的 body 逐项对齐后端：
 * `sourceRepository` / `targetRepository` / `sourceBranch` / `targetBranch` /
 * `title` / `description` / `reviewers`（逗号分隔的用户主键）。
 * 响应是 `{ id }` —— 业务编号，直接跳详情页。
 *
 * 逐条对齐旧版 `CreateMergeRequest.js` + `MergeRequestDetail.js` 的创建态：
 *   1. 源/目标**仓库**可选（跨仓库与 fork 场景）；
 *   2. 源/目标分支下拉，并显示各分支的**末次提交**；
 *   3. 必须选择不同的分支；
 *   4. 评审人取自**目标**仓库成员且 role >= DEVELOPER。
 */
export default function MergeRequestCreate({ global = false }: MergeRequestCreateProps) {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { groupName, repositoryName } = useParams()
  // 全局新建（/mergerequests/new）在仓库上下文之外，此时拿不到 summary/config，
  // 仓库改由用户在下拉里选，所以这里用"可选"上下文而不是必选的 useRepository()。
  const repository = useOptionalRepository()
  const summary = repository?.summary ?? null
  const config = repository?.config ?? null
  const { repositories } = useAppData()

  const repoOptions = useMemo(
    () => repositories.map((item) => ({ value: item.id, label: `${item.group.displayName}/${item.displayName}` })),
    [repositories],
  )

  const [sourceRepoId, setSourceRepoId] = useState('')
  const [targetRepoId, setTargetRepoId] = useState('')
  const [sourceBranch, setSourceBranch] = useState('')
  const [targetBranch, setTargetBranch] = useState('')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [reviewers, setReviewers] = useState<string[]>([])
  const [saving, setSaving] = useState(false)

  // 非全局模式：仓库来自路由；全局模式：仓库由用户选。
  const effectiveSourceRepoId = global ? sourceRepoId : summary?.id ?? ''
  const effectiveTargetRepoId = global ? targetRepoId : summary?.id ?? ''

  const targetRepository = useMemo(
    () => repositories.find((item) => item.id === effectiveTargetRepoId),
    [repositories, effectiveTargetRepoId],
  )

  const [sourceBranches, setSourceBranches] = useState<string[]>([])
  const [targetBranches, setTargetBranches] = useState<string[]>([])
  const [sourceHead, setSourceHead] = useState<CommitSummary | null>(null)
  const [targetHead, setTargetHead] = useState<CommitSummary | null>(null)

  // 分支列表：路由内的仓库直接用 config，全局模式按仓库主键现拉。
  useEffect(() => {
    if (!global) {
      const list = config?.branches ?? []
      setSourceBranches(list.map((branch) => branch.name))
      setTargetBranches(list.map((branch) => branch.name))
      return
    }
    let cancelled = false
    const load = async (repoId: string, setter: (list: string[]) => void) => {
      if (!repoId) {
        setter([])
        return
      }
      try {
        const list = await fetchBranchList(repoId)
        if (!cancelled) {
          setter(list.map((branch) => branch.name))
        }
      } catch {
        if (!cancelled) {
          setter([])
        }
      }
    }
    void load(effectiveSourceRepoId, setSourceBranches)
    void load(effectiveTargetRepoId, setTargetBranches)
    return () => {
      cancelled = true
    }
  }, [global, config, effectiveSourceRepoId, effectiveTargetRepoId])

  // 末次提交预览（旧版在源/目标分支各显示一条 CommitItem）。
  useEffect(() => {
    const loadHead = async (repoId: string, branch: string, setter: (item: CommitSummary | null) => void) => {
      if (!repoId || !branch) {
        setter(null)
        return
      }
      try {
        const commits = await fetchCommitList({ repositoryKey: repoId, revision: branch, page: 1, pagesize: 1 })
        setter(commits[0] ?? null)
      } catch {
        setter(null)
      }
    }
    void loadHead(effectiveSourceRepoId, sourceBranch, setSourceHead)
    void loadHead(effectiveTargetRepoId, targetBranch, setTargetHead)
  }, [effectiveSourceRepoId, effectiveTargetRepoId, sourceBranch, targetBranch])

  // 目标分支默认预选默认分支。
  useEffect(() => {
    if (targetBranch || targetBranches.length === 0) {
      return
    }
    const preferred =
      targetRepository?.defaultBranch ??
      config?.repository.defaultBranch ??
      targetBranches[0]
    if (preferred) {
      setTargetBranch(preferred)
    }
  }, [targetBranches, targetRepository, config, targetBranch])

  const toggleReviewer = (id: string) => {
    setReviewers((current) => (current.includes(id) ? current.filter((item) => item !== id) : [...current, id]))
  }

  const base = global ? '/mergerequests' : `/${groupName}/${repositoryName}`

  const submit = async () => {
    const source = sourceBranch || sourceBranches[0] || ''
    if (!effectiveSourceRepoId || !effectiveTargetRepoId || !source || !targetBranch) {
      toast.error(t('label.createMergeRequest'))
      return
    }
    if (source === targetBranch && effectiveSourceRepoId === effectiveTargetRepoId) {
      toast.error(t('message.error.sameBranch'))
      return
    }
    if (!title.trim()) {
      toast.error(t('label.title'))
      return
    }
    setSaving(true)
    try {
      const id = await createMergeRequest({
        sourceRepositoryKey: effectiveSourceRepoId,
        targetRepositoryKey: effectiveTargetRepoId,
        sourceBranch: source,
        targetBranch,
        title: title.trim(),
        description,
        reviewers,
      })
      toast.success(t('message.addSuccess'))
      if (id) {
        // 跳到**目标仓库**的 MR 详情：跨仓库 / fork MR 的源仓库与目标仓库可能不同，
        // 全局与组内新建时更不能假设当前路由里的仓库就是目标仓库。
        const target = repositories.find((item) => item.id === effectiveTargetRepoId)
        navigate(target ? `/${target.group.name}/${target.name}/mergerequests/${id}` : `${base}/mergerequests/${id}`)
      } else {
        navigate(`${base}/mergerequests`)
      }
    } catch (error) {
      const code = error instanceof ApiError ? error.code : 0
      if (code === 0x0406) {
        toast.error(t('message.error.mergeRequestExists'))
      } else if (code === 0x040f) {
        toast.error(t('message.error.branchProteced'))
      } else {
        toast.error(error instanceof Error ? error.message : t('base_error_message'))
      }
    } finally {
      setSaving(false)
    }
  }

  // 评审人候选：目标仓库成员，且 role >= DEVELOPER（旧版口径）。
  const reviewerCandidates = useMemo(
    () => (targetRepository?.members ?? []).filter((member) => !member.status && member.role >= Role.DEVELOPER),
    [targetRepository],
  )

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.createMergeRequest')}</div>
        <Form onSubmit={submit}>
          <FormGrid>
            {global ? (
              <FormItem label={t('label.sourceRepository')} required>
                <SelectField
                  options={repoOptions}
                  value={effectiveSourceRepoId}
                  onChange={(event) => setSourceRepoId(event.target.value)}
                />
              </FormItem>
            ) : null}
            {global ? (
              <FormItem label={t('label.targetRepository')} required>
                <SelectField
                  options={repoOptions}
                  value={effectiveTargetRepoId}
                  onChange={(event) => setTargetRepoId(event.target.value)}
                />
              </FormItem>
            ) : null}
            <FormItem label={t('label.sourceBranch')} required help={sourceHead ? sourceHead.commit : undefined}>
              <SelectField
                options={sourceBranches.map((name) => ({ value: name, label: name }))}
                value={sourceBranch || sourceBranches[0] || ''}
                onChange={(event) => setSourceBranch(event.target.value)}
              />
            </FormItem>
            <FormItem label={t('label.targetBranch')} required help={targetHead ? targetHead.commit : undefined}>
              <SelectField
                options={targetBranches.map((name) => ({ value: name, label: name }))}
                value={targetBranch}
                onChange={(event) => setTargetBranch(event.target.value)}
              />
            </FormItem>
          </FormGrid>
          <FormItem label={t('label.title')} required>
            <TextField value={title} onChange={(event) => setTitle(event.target.value)} />
          </FormItem>
          <FormItem label={t('label.description')}>
            <TextareaField value={description} onChange={(event) => setDescription(event.target.value)} rows={5} />
          </FormItem>
          <FormItem label={t('label.reviewReviewer')}>
            <div className="row">
              {reviewerCandidates.map((member) => (
                <Checkbox
                  key={member.id}
                  label={member.name}
                  checked={reviewers.includes(member.id)}
                  onChange={() => toggleReviewer(member.id)}
                />
              ))}
            </div>
          </FormItem>
          {sourceHead || targetHead ? (
            <div className="muted">
              {sourceHead ? (
                <div className="row">
                  <span>{sourceHead.commit}</span>
                  <FormattedTime timestamp={sourceHead.time} absolute />
                </div>
              ) : null}
              {targetHead ? (
                <div className="row">
                  <span>{targetHead.commit}</span>
                  <FormattedTime timestamp={targetHead.time} absolute />
                </div>
              ) : null}
            </div>
          ) : null}
          <FormActions>
            <Button variant="text" onClick={() => navigate(`${base}/mergerequests`)}>
              {t('label.cancel')}
            </Button>
            <Button variant="contained" type="submit" loading={saving}>
              {t('label.createMergeRequest')}
            </Button>
          </FormActions>
        </Form>
      </section>
    </div>
  )
}
