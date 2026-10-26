import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'

import { fetchCommitList } from '@/api/repository'
import type { Commit } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { Dropdown } from '@/components/unit/Dropdown'
import { EmptyState } from '@/components/unit/EmptyState'
import { Icon } from '@/components/unit/Icon'
import { Loading } from '@/components/unit/Loading'
import { TextField } from '@/components/unit/TextField'
import { CommitRow } from '@/components/view/unit/CommitRow'
import { plSearch, psBranch, psCaretDown } from '@/config/icons'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { dayKey } from '@/helpers/format'
import { resolveMember } from '@/helpers/members'
import { useT } from '@/i18n'


/** 每页条数（旧版 `CommitList.js` 的 pageSize = 20）。 */
const PAGE_SIZE = 20

/**
 * 提交列表（`/{组}/{仓库}/commits[/{rev}[/*]]`）。
 *
 * 数据链路与视觉对齐旧 `components/view/CommitList.js`：
 *   - `repository/commitList?repository&revision&keyword&path&page&pagesize`；
 *   - 关键字搜索走服务端（`keyword`），**回车提交**（旧版 onKeyDown Enter）；
 *   - 「查看更多」**追加**下一页（旧版 label.more），跨页同日期合并进同一组；
 *   - 每个日期组 = 1px 边框盒 + 灰底标题条（旧版 `TitleList`），标题 `yyyy-MM-dd [N 提交]`；
 *   - 子目录提交列表点提交 → 该提交的目录视图（旧版 linkPathBase='files' + path），
 *     根目录提交列表 → 提交详情。
 */
export default function CommitList() {
  const t = useT()
  const navigate = useNavigate()
  const { groupName, repositoryName, rev: routeRev, '*': routePath } = useParams()
  const [params] = useSearchParams()
  const { summary, config } = useRepository()

  const defaultBranch = config?.repository.defaultBranch || 'master'
  const [searchInput, setSearchInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [commits, setCommits] = useState<Commit[]>([])
  const [hasNext, setHasNext] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const effectiveRef = routeRev || params.get('rev') || defaultBranch
  // 子目录路径：`/{组}/{仓库}/commits/{rev}/*` 的 `*` 段（仓库内相对路径）。
  const subPath = routePath ? routePath.replace(/^\/+/, '') : ''
  const base = `/${groupName}/${repositoryName}`

  /** 竞态守卫：只有最后一次请求的结果会被采纳。 */
  const loadSeq = useRef(0)

  // 换 ref / 子目录时回到第一页并清空搜索（旧版 shouldComponentUpdate 的重置语义）。
  useEffect(() => {
    setPage(1)
    setKeyword('')
    setSearchInput('')
  }, [effectiveRef, subPath])

  useEffect(() => {
    if (!summary) {
      return
    }
    const seq = loadSeq.current + 1
    loadSeq.current = seq
    let cancelled = false
    setLoading(true)
    setError(null)
    // 每页固定 20 条（旧版口径）：返回满一页即认为还有下一页，
    // 末页恰好整页时会多一次空查询，旧版同样如此。
    fetchCommitList({
      repositoryKey: summary.id,
      revision: effectiveRef,
      keyword,
      path: subPath,
      page,
      pagesize: PAGE_SIZE,
    })
      .then((list) => {
        if (cancelled || seq !== loadSeq.current) {
          return
        }
        setHasNext(list.length >= PAGE_SIZE)
        setCommits((prev) => (page === 1 ? list : [...prev, ...list]))
      })
      .catch((err: unknown) => {
        if (!cancelled && seq === loadSeq.current) {
          setError(err instanceof Error ? err.message : '加载失败')
        }
      })
      .finally(() => {
        if (!cancelled && seq === loadSeq.current) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [summary, effectiveRef, keyword, page, subPath])

  const refItems = useMemo(() => {
    // 旧版选择分支/标签后 push `/commits/{rev}`，URL 是唯一事实源。
    const branches = (config?.branches ?? []).map((branch) => ({
      key: `b-${branch.id}`,
      label: branch.name,
      onClick: () => navigate(`${base}/commits/${encodeURIComponent(branch.name)}`),
    }))
    const tags = (config?.tags ?? []).map((tag) => ({
      key: `t-${tag.id}`,
      label: tag.name,
      onClick: () => navigate(`${base}/commits/${encodeURIComponent(tag.name)}`),
    }))
    return [...branches, ...tags]
  }, [config, base, navigate])

  /** 按 `yyyy-MM-dd` 分组（组内保持后端返回的时间倒序；跨页同日期合并）。 */
  const groups = useMemo(() => {
    const buckets = new Map<string, Commit[]>()
    commits.forEach((commit) => {
      const key = dayKey(Number(commit.time))
      const bucket = buckets.get(key)
      if (bucket) {
        bucket.push(commit)
      } else {
        buckets.set(key, [commit])
      }
    })
    return [...buckets.entries()].map(([date, list]) => ({ date, list }))
  }, [commits])

  return (
    <div className="page">
      {/* 空仓库连默认分支都没有，工具条无从选择（与旧版一致）。 */}
      {(config?.branches?.length ?? 0) > 0 ? (
        <div className="file-toolbar">
          <div className="row">
            <Dropdown
              triggerClassName="file-toolbar__rev"
              trigger={
                <>
                  <Icon icon={psBranch} size={16} />
                  {effectiveRef}
                  <Icon icon={psCaretDown} size={12} />
                </>
              }
              items={refItems}
              minWidth={200}
            />
            {subPath ? <span className="muted">{subPath}</span> : null}
          </div>
          <div className="row">
            <TextField
              containerClassName="file-toolbar__search"
              placeholder={t('label.search')}
              value={searchInput}
              disabled={loading}
              startAdornment={<Icon icon={plSearch} size={16} />}
              onChange={(event) => setSearchInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  setKeyword(searchInput.trim())
                  setPage(1)
                }
              }}
            />
          </div>
        </div>
      ) : null}

      {error ? <div className="auth-message">{error}</div> : null}
      {loading ? <Loading /> : null}

      {!loading && commits.length === 0 ? (
        <EmptyState imageName="commits-empty.png" title={t('message.noCommitRecord')} />
      ) : null}

      {groups.map((group) => (
        <div className="commit-group" key={group.date}>
          <div className="commit-group__title">
            {group.date} [{t('label._N_commit', { n: group.list.length })}]
          </div>
          {group.list.map((commit) => (
            <CommitRow
              key={commit.sha}
              commit={commit}
              linkBase={`${base}/commit`}
              author={resolveMember(config?.members, commit.email)}
              makeHref={
                subPath
                  ? (sha8) => `${base}/files/${subPath}?rev=${encodeURIComponent(sha8)}`
                  : undefined
              }
            />
          ))}
        </div>
      ))}

      {commits.length > 0 ? (
        <div className="list-more">
          <Button
            variant={hasNext ? 'contained' : 'outlined'}
            disabled={!hasNext || loading}
            onClick={() => setPage((value) => value + 1)}
          >
            {loading ? t('label.processing') : hasNext ? t('label.more') : t('label.noMore')}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
