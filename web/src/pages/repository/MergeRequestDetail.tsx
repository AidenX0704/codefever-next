import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { ApiError } from '@/api/client'
import {
  addMergeRequestComment,
  assignReviewer,
  checkMergeType,
  closeMergeRequest,
  deleteReviewer,
  fetchMergeRequestChanges,
  fetchMergeRequestComments,
  fetchMergeRequestCommits,
  fetchMergeRequestDetail,
  fetchMergeRequestVersionList,
  fetchRepositoryActivities,
  mergeMergeRequest,
  reviewMergeRequest,
  type MRComment,
} from '@/api/repository'
import type {
  ActivityItem,
  Commit as CommitSummary,
  DiffFile,
  FileChanges,
  FileChangesLarge,
  MergeRequestDetail as MRDetail,
  MergeRequestVersionList,
  RepositoryMember,
} from '@/api/types'
import { AppLink } from '@/components/unit/AppLink'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { Checkbox } from '@/components/unit/Choices'
import { useConfirm } from '@/components/unit/Confirm'
import { DiffViewer } from '@/components/unit/DiffViewer'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Loading } from '@/components/unit/Loading'
import { SelectField } from '@/components/unit/SelectField'
import { Tabs } from '@/components/unit/Tabs'
import { Tag, type TagTone } from '@/components/unit/Tag'
import { TextField, TextareaField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { arrowRightIcon, caretDownIcon, checkCircleIcon, closeIcon, lockIcon, warningCircleIcon } from '@/config/icons'
import { Role } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'


type DetailTab = 'diff' | 'commits' | 'discussion' | 'activities'

/**
 * 合并请求详情（`/{组}/{仓库}/mergerequests/{编号}`）。
 *
 * 数据源（与旧版 `MergeRequestDetail.js` 一一对应）：
 *   - `mergeRequestDetail` → 详情（含 `reviewers`、`sourceRepoInfo`）；
 *   - `fileChanges` → 代码差异（`original=targetObject`、`modified=sourceObject`）；
 *   - `mergeRequestVersionList` → 版本对比下拉（源侧 / 基线侧各一个下拉）；
 *   - `mergeRequestCommits` → 「提交动态」页签；
 *   - `repository/activities?mergeRequest=` → 「合并请求动态」页签。
 *
 * 动作：`mergeRequestReview` / `mergeRequestClose` / `assignReviewer` / `deleteReviewer`
 *      / `checkMergeType` + `mergeBranch`。
 */
export default function MergeRequestDetail() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const navigate = useNavigate()
  const { groupName, repositoryName, mid } = useParams()
  const { summary } = useRepository()
  const { repositories } = useAppData()

  const [detail, setDetail] = useState<MRDetail | null>(null)
  const [files, setFiles] = useState<DiffFile[]>([])
  const [large, setLarge] = useState<FileChangesLarge | null>(null)
  const [commits, setCommits] = useState<CommitSummary[]>([])
  const [activities, setActivities] = useState<ActivityItem[]>([])
  const [versions, setVersions] = useState<MergeRequestVersionList | null>(null)
  const [tab, setTab] = useState<DetailTab>('diff')
  const [startHash, setStartHash] = useState('')
  const [endHash, setEndHash] = useState('')
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [mergeMessage, setMergeMessage] = useState('')
  const [usingSquash, setUsingSquash] = useState(false)
  const [showCommitInput, setShowCommitInput] = useState(false)
  const [conflict, setConflict] = useState(false)
  const [reviewerToAdd, setReviewerToAdd] = useState('')
  const [comments, setComments] = useState<MRComment[]>([])
  const [commentDraft, setCommentDraft] = useState('')
  const [commentBusy, setCommentBusy] = useState(false)
  /** 合并后同时删除源分支（v4：deleteSourceBranch 参数）。 */
  const [deleteSourceBranch, setDeleteSourceBranch] = useState(false)

  const base = `/${groupName}/${repositoryName}`
  const number = Number(mid)

  /** 目标仓库：后端详情只返回源仓库信息，目标仓库要从全局列表里找。 */
  const targetRepository = useMemo(
    () => repositories.find((item) => item.id === detail?.targetRepository),
    [repositories, detail?.targetRepository],
  )

  /** 评审人候选：旧版取**目标**仓库成员，且 role >= DEVELOPER、未被移除。 */
  const reviewerCandidates = useMemo<RepositoryMember[]>(() => {
    const members = targetRepository?.members ?? detail?.sourceRepoInfo?.members ?? []
    return members.filter((member) => !member.status && member.role >= Role.DEVELOPER)
  }, [targetRepository, detail?.sourceRepoInfo?.members])

  const memberOf = useCallback(
    (userKey: string): RepositoryMember | undefined =>
      [...(targetRepository?.members ?? []), ...(detail?.sourceRepoInfo?.members ?? [])].find(
        (member) => member.id === userKey,
      ),
    [targetRepository, detail?.sourceRepoInfo?.members],
  )

  const loadDiff = useCallback(
    async (info: MRDetail, from?: string, to?: string) => {
      const changes = await fetchMergeRequestChanges({
        repositoryKey: info.sourceRepository,
        mergeRequestKey: info.id,
        original: from ?? info.targetObject,
        modified: to ?? info.sourceObject,
        targetRepositoryKey: info.targetRepository,
      })
      if ('large' in changes) {
        setLarge(changes as FileChangesLarge)
        setFiles([])
      } else {
        setLarge(null)
        setFiles((changes as FileChanges).detail ?? [])
      }
    },
    [],
  )

  useEffect(() => {
    if (!summary || !Number.isFinite(number)) {
      return
    }
    let cancelled = false
    setLoading(true)

    fetchMergeRequestDetail(summary.id, number)
      .then(async (info) => {
        if (cancelled || !info) {
          return
        }
        setDetail(info)
        setMergeMessage(info.title)
        await loadDiff(info)

        const [versionList, commitList, activityList, commentList] = await Promise.all([
          fetchMergeRequestVersionList(info.id).catch(() => null),
          fetchMergeRequestCommits({
            mergeRequestKey: info.id,
            sourceRepositoryKey: info.sourceRepository,
            sourceBranch: info.sourceBranch,
            targetRepositoryKey: info.targetRepository,
            targetBranch: info.targetBranch,
          }).catch(() => [] as CommitSummary[]),
          fetchRepositoryActivities({
            repositoryKey: info.targetRepository,
            mergeRequestKey: info.id,
          }).catch(() => [] as ActivityItem[]),
          fetchMergeRequestComments(info.id).catch(() => [] as MRComment[]),
        ])
        if (cancelled) {
          return
        }
        setVersions(versionList)
        setStartHash(versionList?.mergeVersionList?.[0]?.sha ?? '')
        setEndHash(versionList?.baseVersion ?? '')
        setCommits(commitList)
        setActivities(activityList)
        setComments(commentList)
      })
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [summary, number, t, toast, loadDiff])

  const statusLabel = useMemo(() => {
    if (!detail) {
      return ''
    }
    return detail.status === 2 ? t('label.mrStatusMerged') : detail.status === 3 ? t('label.mrStatusClosed') : t('label.mrStatusPending')
  }, [detail, t])

  /** v5 §11.2 词表：开放=info、已合并=success、已关闭=default。 */
  const statusTone = (status: number): TagTone => {
    if (status === 2) return 'success'
    if (status === 3) return 'default'
    return 'info'
  }

  /** 分支的提交列表链接：`/{组}/{仓库}/commits?rev={分支}`（同 MergeRequestRow）。 */
  const commitsHref = (repo: { group: { name: string }; name: string } | null | undefined, branch: string) =>
    repo ? `/${repo.group.name}/${repo.name}/commits?rev=${encodeURIComponent(branch)}` : ''
  const sourceHref = detail ? commitsHref(detail.sourceRepoInfo, detail.sourceBranch) : ''
  const targetHref = detail ? commitsHref(targetRepository, detail.targetBranch) : ''
  const reviewersApproved = detail ? detail.reviewers.filter((reviewer) => reviewer.isReview).length : 0
  const minReviewersRequired = targetRepository?.minReviewers ?? 0

  const reload = () => {
    if (!summary || !Number.isFinite(number)) {
      return
    }
    fetchMergeRequestDetail(summary.id, number)
      .then((info) => {
        if (info) {
          setDetail(info)
        }
      })
      .catch(() => {
        /* 刷新失败保留当前视图 */
      })
  }

  const addReviewer = async () => {
    if (!detail || !reviewerToAdd) {
      return
    }
    try {
      await assignReviewer(detail.id, reviewerToAdd)
      toast.success(t('message.updateSuccess'))
      setReviewerToAdd('')
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const removeReviewer = async (reviewerUserKey: string, isReviewed: boolean) => {
    if (!detail) {
      return
    }
    // 后端拒绝删除"已评审"的评审人（repo.DeleteReviewer），前端先拦一道。
    if (isReviewed) {
      toast.error(t('message.error.reviewedCanNotDelete'))
      return
    }
    const ok = await confirm({ title: t('label.delete'), message: t('message.deleteReviewer'), tone: 'danger', confirmText: t('base_confirm') })
    if (!ok) {
      return
    }
    try {
      // 必须传**用户主键**（后端按 u_key 匹配），传评审记录主键会静默失败。
      await deleteReviewer(detail.id, reviewerUserKey)
      toast.success(t('message.deleteSuccess'))
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const review = async () => {
    if (!detail) {
      return
    }
    setBusy(true)
    try {
      await reviewMergeRequest(detail.id)
      toast.success(t('message.approved'))
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setBusy(false)
    }
  }

  const close = async () => {
    if (!detail) {
      return
    }
    const ok = await confirm({ title: t('label.closeMergeRequest'), message: detail.title, tone: 'danger', confirmText: t('base_confirm') })
    if (!ok) {
      return
    }
    setBusy(true)
    try {
      await closeMergeRequest(detail.id)
      toast.success(t('message.closed'))
      navigate(`${base}/mergerequests`)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setBusy(false)
    }
  }

  /** 真正执行合并。返回 true 表示已合并（调用方负责跳转）。 */
  const doMerge = async (message: string, squash: boolean): Promise<boolean> => {
    if (!detail) {
      return false
    }
    const result = await mergeMergeRequest(detail.id, message, squash, deleteSourceBranch)
    if (result.conflict) {
      // 后端冲突时返回 {"conflict":true} 且 code=0，不是错误码，必须自己判断。
      setConflict(true)
      setShowCommitInput(false)
      return false
    }
    return true
  }

  /** 发表讨论（成功后本地追加，避免整页刷新）。 */
  const submitComment = async () => {
    const body = commentDraft.trim()
    if (!detail || !body) {
      return
    }
    setCommentBusy(true)
    try {
      await addMergeRequestComment(detail.id, body)
      const list = await fetchMergeRequestComments(detail.id)
      setComments(list)
      setCommentDraft('')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setCommentBusy(false)
    }
  }

  const merge = async () => {
    if (!detail) {
      return
    }
    setBusy(true)
    setConflict(false)
    try {
      const type = await checkMergeType(detail.id)
      if (type.canBeFastForward) {
        // 能快进：不需要合并信息，直接合。
        const ok = await confirm({ title: t('label.merge'), message: t('message.mergeConfirm'), confirmText: t('base_confirm') })
        if (!ok) {
          return
        }
        if (await doMerge('', false)) {
          toast.success(t('message.updateSuccess'))
          navigate(`${base}/mergerequests`)
        }
        return
      }
      // 不能快进：要求填写合并信息，可选 Squash（与旧版 checkMergeType 的分支一致）。
      if (!showCommitInput) {
        setShowCommitInput(true)
        return
      }
      if (!mergeMessage.trim()) {
        toast.error(t('label.mergeMessage'))
        return
      }
      const ok = await confirm({ title: t('label.merge'), message: t('message.mergeConfirm'), confirmText: t('base_confirm') })
      if (!ok) {
        return
      }
      if (await doMerge(mergeMessage.trim(), usingSquash)) {
        toast.success(t('message.updateSuccess'))
        navigate(`${base}/mergerequests`)
      }
    } catch (error) {
      const code = error instanceof ApiError ? error.code : 0
      if (code === 0x040f) {
        toast.error(t('message.error.branchProteced'))
      } else if (code === 0x0405) {
        toast.error(t('message.error.userNotReview'))
      } else {
        toast.error(error instanceof Error ? error.message : t('base_error_message'))
      }
    } finally {
      setBusy(false)
    }
  }

  const applyVersionDiff = async (from: string, to: string) => {
    if (!detail) {
      return
    }
    try {
      await loadDiff(detail, from, to)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const startOptions = (versions?.mergeVersionList ?? []).map((item, index) => ({
    value: item.sha,
    label: `${index === 0 ? t('label.lastModified') : `${t('label.lastModified')} ${versions!.mergeVersionList.length - index}`} - ${item.sha.slice(0, 8)}`,
  }))

  const endOptions = (versions?.baseVersionList ?? []).map((item, index) => ({
    value: item.sha,
    label: `${index === 0 ? t('label.baseVersion') : `${t('label.lastModified')} ${versions!.baseVersionList.length - index}`} - ${item.sha.slice(0, 8)}`,
  }))

  const submitter = detail?.submitter
  const handler = detail?.handler ? memberOf(detail.handler) : undefined

  return (
    <div className="page">
      {loading ? <Loading /> : null}

      {detail ? (
        <div className="cf-mr-detail">
          {/* v5 §6 G9 头部纸页：标题 20/600 + #编号 + 状态胶囊 + 编辑/关闭；说明行 = 作者头像 + 分支芯片 + 仓库标签。 */}
          <header className="cf-mr-detail__head">
            <div className="cf-mr-detail__titlerow">
              <h1 className="cf-mr-detail__title">{detail.title}</h1>
              <span className="cf-mr-detail__number">#{detail.number}</span>
              <Tag tone={statusTone(detail.status)} className="cf-mr-detail__status">{statusLabel}</Tag>
              <span className="cf-mr-detail__spacer" />
              {detail.status === 1 ? (
                <>
                  <Button variant="text" onClick={review} loading={busy}>{t('label.review')}</Button>
                  <Button variant="text" tone="danger" onClick={close} loading={busy}>{t('label.closeMergeRequest')}</Button>
                </>
              ) : null}
            </div>
            <div className="cf-mr-detail__metarow">
              <Avatar size="sm" src={submitter?.icon ? `/file/fetch/avatar/${submitter.icon}` : undefined} name={submitter?.name} />
              <span className="cf-mr-detail__meta-text">{submitter?.name}</span>
              <span className="cf-mr-detail__meta-text">{t('label.createdAt')}: <FormattedTime timestamp={Number(detail.create)} absolute /></span>
              <span className="cf-mr-row__chip">
                {sourceHref ? <AppLink to={sourceHref} title={detail.sourceBranch}>{detail.sourceBranch}</AppLink> : detail.sourceBranch}
              </span>
              <Icon icon={arrowRightIcon} size={12} className="cf-mr-row__arrow" />
              <span className="cf-mr-row__chip">
                {targetHref ? <AppLink to={targetHref} title={detail.targetBranch}>{detail.targetBranch}</AppLink> : detail.targetBranch}
              </span>
            </div>
            {detail.description ? (
              <div className="cf-mr-detail__desc markdown-body">{detail.description}</div>
            ) : null}
            <Tabs<DetailTab>
              value={tab}
              onChange={setTab}
              items={[
                { key: 'diff', label: `${t('label.codeDiff')}(${files.length})` },
                { key: 'commits', label: `${t('label.commitActivity')}(${commits.length})` },
                { key: 'discussion', label: `${t('label.discussion')}(${comments.length})`, count: comments.length },
                { key: 'activities', label: `${t('label.mergeRequestActivity')}(${activities.length})` },
              ]}
            />
          </header>

          {/* v5 §6 G9：MergeBox 在时间线之前（可合并态给分体按钮，阻塞态给原因清单）。 */}
          {detail.status === 1 ? (
            <section className="cf-mergebox">
              {conflict ? (
                <>
                  <div className="cf-mergebox__headline cf-mergebox__headline--bad">
                    <Icon icon={closeIcon} size={16} />
                    {t('message.mergeConflictTitle')}
                  </div>
                  <div className="cf-mergebox__hint">
                    <div className="muted">{t('message.mergeConflictTip1')}</div>
                    <pre className="webhook-log">git fetch origin{`\n`}git checkout -b {detail.sourceBranch} origin/{detail.sourceBranch}</pre>
                    <div className="muted">{t('message.mergeConflictTip2')}</div>
                    <div className="muted">{t('message.mergeConflictTip3')}</div>
                    <pre className="webhook-log">
                      {`git fetch origin\ngit checkout origin/${detail.targetBranch}\ngit merge origin/${detail.sourceBranch}`}
                    </pre>
                  </div>
                </>
              ) : minReviewersRequired > 0 && reviewersApproved < minReviewersRequired ? (
                <div className="cf-mergebox__headline cf-mergebox__headline--warn">
                  <Icon icon={warningCircleIcon} size={16} />
                  {t('message.mergeBlocked', { n: minReviewersRequired - reviewersApproved })}
                </div>
              ) : (
                <div className="cf-mergebox__headline cf-mergebox__headline--ok">
                  <Icon icon={checkCircleIcon} size={16} />
                  {t('message.mergeReady')}
                </div>
              )}
              <div className="cf-mergebox__options">
                <Checkbox
                  label={t('label.usingSquash')}
                  checked={usingSquash}
                  onChange={(event) => setUsingSquash(event.target.checked)}
                />
                <Checkbox
                  label={t('label.deleteSourceBranch')}
                  checked={deleteSourceBranch}
                  onChange={(event) => setDeleteSourceBranch(event.target.checked)}
                />
                <span className="cf-mergebox__spacer" />
                {conflict || (minReviewersRequired > 0 && reviewersApproved < minReviewersRequired) ? (
                  <Button variant="outlined" disabled>
                    <Icon icon={lockIcon} size={14} />
                    {t('label.merge')}
                    <Icon icon={caretDownIcon} size={14} />
                  </Button>
                ) : showCommitInput ? (
                  <>
                    <TextField
                      value={mergeMessage}
                      onChange={(event) => setMergeMessage(event.target.value)}
                      placeholder={t('message.inputMergeCommit')}
                    />
                    <Button variant="contained" onClick={merge} loading={busy}>{t('label.merge')}</Button>
                  </>
                ) : (
                  <Button variant="contained" onClick={merge} loading={busy}>
                    {t('label.merge')}
                  </Button>
                )}
              </div>
            </section>
          ) : null}

          <div className="cf-mr-detail__body">
            <div className="cf-mr-detail__main">
            {tab === 'diff' ? (
              <section className="section">
                {startOptions.length > 0 && endOptions.length > 0 ? (
                  <div className="row">
                    <SelectField
                      options={startOptions}
                      value={startHash}
                      onChange={(event) => {
                        const next = event.target.value
                        setStartHash(next)
                        void applyVersionDiff(next, endHash)
                      }}
                    />
                    <span className="muted">{t('label.and')}</span>
                    <SelectField
                      options={endOptions}
                      value={endHash}
                      onChange={(event) => {
                        const next = event.target.value
                        setEndHash(next)
                        void applyVersionDiff(startHash, next)
                      }}
                    />
                  </div>
                ) : null}
                {large ? <div className="auth-message">{t('message.diffFileToMany')}</div> : null}
                {files.length > 0 ? <DiffViewer files={files} rev={detail.sourceObject} /> : null}
              </section>
            ) : null}

            {tab === 'commits' ? (
              <section className="section">
                <div className="list-body">
                  {commits.map((item) => (
                    <div className="ref-row" key={item.sha}>
                      <span className="ref-row__name">{item.commit}</span>
                      <span className="ref-row__commit">{item.email}</span>
                      <span className="ref-row__time">
                        <FormattedTime timestamp={item.time} absolute />
                      </span>
                    </div>
                  ))}
                </div>
              </section>
            ) : null}

            {tab === 'discussion' ? (
              <section className="section">
                <div className="stack">
                  {comments.length === 0 ? <div className="muted">{t('discussion.empty')}</div> : null}
                  {comments.map((comment) => {
                    const author = memberOf(comment.author.id)
                    return (
                      <div className="mr-comment" key={comment.id}>
                        <Avatar
                          size="sm"
                          src={(author?.icon ?? comment.author.icon) ? `/file/fetch/avatar/${author?.icon ?? comment.author.icon}` : undefined}
                          name={author?.name ?? comment.author.name}
                        />
                        <div className="mr-comment__main">
                          <div className="mr-comment__meta">
                            <span className="mr-comment__name">{author?.name ?? comment.author.name}</span>
                            <FormattedTime timestamp={comment.createdAt} />
                          </div>
                          <div className="mr-comment__body">{comment.body}</div>
                        </div>
                      </div>
                    )
                  })}
                  <div className="row">
                    <TextareaField
                      containerClassName="mr-comment__input"
                      value={commentDraft}
                      onChange={(event) => setCommentDraft(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
                          event.preventDefault()
                          void submitComment()
                        }
                      }}
                      placeholder={t('discussion.placeholder')}
                    />
                    <Button variant="contained" loading={commentBusy} disabled={!commentDraft.trim()} onClick={submitComment}>
                      {t('discussion.submit')}
                    </Button>
                  </div>
                  <div className="muted">{t('discussion.ctrlEnter')}</div>
                </div>
              </section>
            ) : null}

            {tab === 'activities' ? (
              <section className="section">
                <div className="list-body">
                  {activities.map((item) => (
                    <div className="ref-row" key={item.id}>
                      <span className="ref-row__name">{item.creator}</span>
                      <span className="ref-row__time">
                        <FormattedTime timestamp={item.time} absolute />
                      </span>
                    </div>
                  ))}
                </div>
              </section>
            ) : null}
          </div>

          <aside className="cf-mr-detail__side">
            <section className="section">
              <div className="section__title">{t('label.reviewer')}</div>
              <div className="stack">
                {detail.reviewers.length === 0 ? <span className="muted">{t('message.notSelectReviewers')}</span> : null}
                {detail.reviewers.map((reviewer) => {
                  const member = memberOf(reviewer.user)
                  return (
                    <div className="cf-mr-detail__reviewer" key={reviewer.id}>
                      <Avatar size="sm" src={member?.icon ? `/file/fetch/avatar/${member.icon}` : undefined} name={member?.name} />
                      <span className="cf-mr-detail__reviewer-name">{member?.name ?? reviewer.user}</span>
                      <Tag tone={reviewer.isReview ? 'success' : 'danger'}>
                        {reviewer.isReview ? t('message.approved') : t('message.notApprove')}
                      </Tag>
                      {detail.status === 1 ? (
                        <Button size="sm" variant="text" tone="danger" onClick={() => removeReviewer(reviewer.user, reviewer.isReview)}>
                          ×
                        </Button>
                      ) : null}
                    </div>
                  )
                })}
                {detail.status === 1 ? (
                  <div className="row">
                    <SelectField
                      containerClassName="member-role-select"
                      placeholder={t('message.selectReviewer')}
                      options={reviewerCandidates
                        .filter((member) => !detail.reviewers.some((reviewer) => reviewer.user === member.id))
                        .map((member) => ({ value: member.id, label: member.name }))}
                      value={reviewerToAdd}
                      onChange={(event) => setReviewerToAdd(event.target.value)}
                    />
                    <Button size="sm" variant="outlined" onClick={addReviewer}>
                      {t('label.addMember')}
                    </Button>
                  </div>
                ) : null}
              </div>
            </section>

            <section className="section">
              <div className="section__title">{t('label.activity')}</div>
              <div className="stack">
                <div className="muted">
                  {t('label.requestMerge')}: {detail.sourceRepoInfo?.group.displayName}/{detail.sourceRepoInfo?.displayName}/
                  {detail.sourceBranch} → {targetRepository?.group.displayName}/{targetRepository?.displayName}/{detail.targetBranch}
                </div>
                <div className="row">
                  <Avatar size="sm" src={submitter?.icon ? `/file/fetch/avatar/${submitter.icon}` : undefined} name={submitter?.name} />
                  <span className="muted">
                    {submitter?.name} {t('label.requestCreated')}:{' '}
                    <FormattedTime timestamp={Number(detail.create)} absolute />
                  </span>
                </div>
                {detail.status === 2 ? (
                  <div className="row">
                    <Avatar size="sm" src={handler?.icon ? `/file/fetch/avatar/${handler.icon}` : undefined} name={handler?.name} />
                    <span className="muted">
                      {handler?.name ?? detail.handler} {t('label.requestMerged')}:{' '}
                      <FormattedTime timestamp={Number(detail.merge)} absolute />
                    </span>
                  </div>
                ) : null}
                {detail.status === 3 ? (
                  <div className="row">
                    <Avatar size="sm" src={handler?.icon ? `/file/fetch/avatar/${handler.icon}` : undefined} name={handler?.name} />
                    <span className="muted">
                      {handler?.name ?? detail.handler} {t('label.requestClosed')}:{' '}
                      <FormattedTime timestamp={Number(detail.close)} absolute />
                    </span>
                  </div>
                ) : null}
              </div>
            </section>
          </aside>
          </div>
        </div>
      ) : null}
    </div>
  )
}
