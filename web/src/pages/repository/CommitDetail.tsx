import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { fetchCommitDetail, fetchFileChanges } from '@/api/repository'
import type { CommitDetail as CommitDetailType, DiffFile, FileChangesLarge, FileChanges } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { CopyButton } from '@/components/unit/CopyButton'
import { DiffViewer } from '@/components/unit/DiffViewer'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Loading } from '@/components/unit/Loading'
import { CommitRefs } from '@/components/view/unit/CommitRefs'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { resolveMember } from '@/helpers/members'
import { useT } from '@/i18n'


/**
 * 提交详情（`/{组}/{仓库}/commit/{8 位短 SHA}`）。
 *
 * 两个请求：
 *   1. `repository/commitDetail?repository&commitSHA` → 提交元信息（sha/commit/parent/email/time）；
 *   2. `repository/fileChanges?repository&original={parent}&modified={sha}` → 文件级 diff（**后端已解析成结构**，
 *      前端不再解析 unified diff 文本，见 `DiffViewer`）。
 * 父提交为空（首个提交）时 `original` 留空，后端会补成全 0 的 40 位 SHA（等价空树）。
 *
 * 另有两处与旧版对齐：**父提交展开列表**（`CommitRefs`）与 **"浏览文件"跳转**。
 */
export default function CommitDetail() {
  const t = useT()
  const navigate = useNavigate()
  const { groupName, repositoryName, hash } = useParams()
  const { summary, config } = useRepository()

  const [detail, setDetail] = useState<CommitDetailType | null>(null)
  const [files, setFiles] = useState<DiffFile[]>([])
  const [large, setLarge] = useState<FileChangesLarge | null>(null)
  const [showRefs, setShowRefs] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!summary || !hash) {
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)

    fetchCommitDetail(summary.id, hash)
      .then(async (info) => {
        if (cancelled) {
          return
        }
        setDetail(info)
        if (!info.sha) {
          return
        }
        const changes = await fetchFileChanges({
          repositoryKey: summary.id,
          modified: info.sha,
          original: info.parent ?? '',
        })
        if (cancelled) {
          return
        }
        if ('large' in changes) {
          setLarge(changes as FileChangesLarge)
          setFiles([])
        } else {
          setLarge(null)
          setFiles((changes as FileChanges).detail ?? [])
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : '加载失败')
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [summary, hash])

  const author = resolveMember(config?.members, detail?.email ?? undefined)
  const base = `/${groupName}/${repositoryName}`
  const parentHashes = detail?.parent ? detail.parent.split(' ').filter(Boolean) : []

  return (
    <div className="page">
      <div className="repo-head">
        <div className="repo-head__title">
          {t('label.commit')} <span className="repo-head__group">{repositoryName}</span>
        </div>
      </div>

      {error ? <div className="auth-message">{error}</div> : null}
      {loading ? <Loading /> : null}

      {detail?.sha ? (
        <div className="section">
          <div className="stack">
            <div className="cf-diff__name" style={{ fontSize: 'var(--cf-font-size-h4)', fontWeight: 'var(--cf-font-weight-bold)' }}>
              {detail.commit}
            </div>
            <div className="row">
              <Avatar size="sm" src={author.icon ?? undefined} name={author.name} />
              <span className="muted">{author.name}</span>
              <span className="muted">
                {t('label.editIn')}: <FormattedTime timestamp={Number(detail.time)} absolute />
              </span>
              <span className="commit-row__hash">{detail.sha}</span>
              <CopyButton text={detail.sha ?? ''} />
              <Button variant="outlined" size="sm" onClick={() => setShowRefs((value) => !value)}>
                {t('label.detail')}
              </Button>
              <Button
                variant="contained"
                size="sm"
                onClick={() => navigate(`${base}/files?rev=${encodeURIComponent(detail.sha ?? '')}`)}
              >
                {t('label.browserFile')}
              </Button>
            </div>
            {showRefs && summary ? (
              <CommitRefs
                repositoryKey={summary.id}
                parentHashes={parentHashes}
                currentHash={detail.sha}
                base={base}
              />
            ) : null}
          </div>
        </div>
      ) : null}

      {large ? (
        <div className="auth-message">
          {t('message.diffFileToMany')}
        </div>
      ) : null}

      {files.length > 0 ? <DiffViewer files={files} /> : null}

      {!loading && detail?.sha && files.length === 0 && !large ? (
        <div className="auth-message">{t('message.noFileChanged')}</div>
      ) : null}
    </div>
  )
}
