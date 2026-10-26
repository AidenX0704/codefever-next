import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'

import { ApiError } from '@/api/client'
import {
  assignReviewer,
  checkMergeType,
  closeMergeRequest,
  deleteReviewer,
  fetchMergeRequestChanges,
  fetchMergeRequestCommits,
  fetchMergeRequestDetail,
  fetchMergeRequestVersionList,
  fetchRepositoryActivities,
  mergeMergeRequest,
  reviewMergeRequest,
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
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { Checkbox } from '@/components/unit/Choices'
import { useConfirm } from '@/components/unit/Confirm'
import { DiffViewer } from '@/components/unit/DiffViewer'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Loading } from '@/components/unit/Loading'
import { SelectField } from '@/components/unit/SelectField'
import { Tabs } from '@/components/unit/Tabs'
import { Tag } from '@/components/unit/Tag'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { Role } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'


type DetailTab = 'diff' | 'commits' | 'activities'

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

        const [versionList, commitList, activityList] = await Promise.all([
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
        ])
        if (cancelled) {
          return
        }
        setVersions(versionList)
        setStartHash(versionList?.mergeVersionList?.[0]?.sha ?? '')
        setEndHash(versionList?.baseVersion ?? '')
        setCommits(commitList)
        setActivities(activityList)
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
    const result = await mergeMergeRequest(detail.id, message, squash)
    if (result.conflict) {
      // 后端冲突时返回 {"conflict":true} 且 code=0，不是错误码，必须自己判断。
      setConflict(true)
      setShowCommitInput(false)
      return false
    }
    return true
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
        <div className="cf-mr-layout">
          <div className="cf-mr-layout__main">
            <section className="section">
              <div className="stack">
                <div className="row">
                  <span className="repo-head__title">
                    #{detail.number} {detail.title}
                  </span>
                  <Tag tone={detail.status === 2 ? 'success' : detail.status === 3 ? 'danger' : 'primary'}>{statusLabel}</Tag>
                </div>
                <div className="row">
                  <Avatar size="sm" src={submitter?.icon ? `/file/fetch/avatar/${submitter.icon}` : undefined} name={submitter?.name} />
                  <span className="muted">{submitter?.name}</span>
                  <span className="muted">
                    {t('label.createdAt')}: <FormattedTime timestamp={Number(detail.create)} absolute />
                  </span>
                </div>
                <div className="row">
                  <Tag tone="primary">{detail.sourceBranch}</Tag>
                  <span className="muted">→</span>
                  <Tag>{detail.targetBranch}</Tag>
                  <Link className="cf-link" to={`${base}/files`}>
                    {t('label.openFileTree')}
                  </Link>
                </div>
                {detail.description ? <div className="markdown-body">{detail.description}</div> : null}
                {detail.status === 1 ? (
                  <div className="row">
                    <Button variant="outlined" loading={busy} onClick={review}>
                      {t('label.review')}
                    </Button>
                    <Button variant="contained" onClick={merge} loading={busy}>
                      {t('label.merge')}
                    </Button>
                    <Button variant="outlined" tone="danger" onClick={close}>
                      {t('label.closeMergeRequest')}
                    </Button>
                  </div>
                ) : null}
                {showCommitInput && detail.status === 1 ? (
                  <div className="row">
                    <TextField
                      value={mergeMessage}
                      onChange={(event) => setMergeMessage(event.target.value)}
                      placeholder={t('message.inputMergeCommit')}
                    />
                    <Checkbox
                      label={t('label.usingSquash')}
                      checked={usingSquash}
                      onChange={(event) => setUsingSquash(event.target.checked)}
                    />
                  </div>
                ) : null}
                {conflict ? (
                  <div className="stack">
                    <div className="section__title">{t('message.mergeConflictTitle')}</div>
                    <div className="muted">{t('message.mergeConflictTip1')}</div>
                    <pre className="webhook-log">git fetch origin{`\n`}git checkout -b {detail.sourceBranch} origin/{detail.sourceBranch}</pre>
                    <div className="muted">{t('message.mergeConflictTip2')}</div>
                    <div className="muted">{t('message.mergeConflictTip3')}</div>
                    <pre className="webhook-log">
                      {`git fetch origin\ngit checkout origin/${detail.targetBranch}\ngit merge origin/${detail.sourceBranch}`}
                    </pre>
                  </div>
                ) : null}
              </div>
            </section>

            <Tabs<DetailTab>
              value={tab}
              onChange={setTab}
              items={[
                { key: 'diff', label: `${t('label.codeDiff')}(${files.length})` },
                { key: 'commits', label: `${t('label.commitActivity')}(${commits.length})` },
                { key: 'activities', label: `${t('label.mergeRequestActivity')}(${activities.length})` },
              ]}
            />

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

          <aside className="cf-mr-layout__side">
            <section className="section">
              <div className="section__title">{t('label.reviewer')}</div>
              <div className="stack">
                {detail.reviewers.length === 0 ? <span className="muted">{t('message.notSelectReviewers')}</span> : null}
                {detail.reviewers.map((reviewer) => {
                  const member = memberOf(reviewer.user)
                  return (
                    <div className="ref-row" key={reviewer.id}>
                      <Avatar size="sm" src={member?.icon ? `/file/fetch/avatar/${member.icon}` : undefined} name={member?.name} />
                      <span className="ref-row__name">{member?.name ?? reviewer.user}</span>
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
      ) : null}
    </div>
  )
}
