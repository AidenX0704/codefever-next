import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation, useNavigate, useParams, useSearchParams } from 'react-router'

import { fetchBlameInfo, fetchObject, fetchPathStack } from '@/api/repository'
import { isDirectoryView } from '@/api/types'
import type { BlameBlock, FileObject, ObjectEntry, ObjectView, PathStackEntry } from '@/api/types'
import { Dropdown } from '@/components/unit/Dropdown'
import { EmptyState } from '@/components/unit/EmptyState'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Loading } from '@/components/unit/Loading'
import { Table, TableWrap, TBody, TD, TH, THead, TR } from '@/components/unit/Table'
import { TextField } from '@/components/unit/TextField'
import { CommitRow } from '@/components/view/unit/CommitRow'
import { ObjectViewer } from '@/components/view/unit/ObjectViewer'
import { RepositoryEmpty } from '@/components/view/unit/RepositoryEmpty'
import { psBranch, psCaretDown, psFile, psFolder, psRepository, plSearch } from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { resolveMember } from '@/helpers/members'
import { useT } from '@/i18n'


/**
 * 文件浏览页（`/{组}/{仓库}/files[/{path}]`）。
 *
 * 数据链路（与旧版 `components/view/FileTree.js` 一致）：
 *   1. `repository/config` 已由外壳加载 → 拿默认分支与分支/标签列表；
 *   2. `repository/pathStack?revision&path` → 路径上每一级目录项（**最后一项的 object 就是当前目录/文件的 git 对象**）；
 *   3. `repository/object?parent=<对象>&path=<路径>&revision=<ref>` → "当前这一层"：
 *      目录返回 entries、文件返回 blob；同一次响应还带回**当前路径的最后一次提交**；
 *   4. 面包屑由 pathStack 给出每一级目录项。
 *
 * 相比旧版补齐的能力：**文件名过滤**、**README 自动渲染**、**submodule 条目**、
 * **当前目录最后一条提交行**、**空仓库初始化引导**、**blame 模式**、
 * 以及文件内容区的**复制路径 / 复制代码 / Markdown 渲染切换 / 历史入口**。
 */
export default function FileTree() {
  const t = useT()
  const { groupName, repositoryName } = useParams()
  const { summary, config } = useRepository()
  const { user } = useAppData()
  const navigate = useNavigate()
  const location = useLocation().pathname
  const [searchParams, setSearchParams] = useSearchParams()

  const rev = searchParams.get('rev') ?? ''
  const blameMode = searchParams.get('blame') === '1'
  const defaultBranch = config?.repository.defaultBranch || 'master'
  const effectiveRev = rev || defaultBranch
  const base = `/${groupName}/${repositoryName}`

  // 路由通配段：/files 之后的全部内容就是仓库内的路径，这里从 URL 自行拆解，
  // 避免依赖 react-router 5 的 `:rev:path(/.*)` 复合参数（v8 不支持）。
  const path = useMemo(() => {
    const marker = `/${groupName}/${repositoryName}/files`
    const raw = location.startsWith(marker) ? location.slice(marker.length).replace(/^\//, '') : ''
    try {
      return decodeURIComponent(raw)
    } catch {
      return raw
    }
  }, [location, groupName, repositoryName])

  const [stack, setStack] = useState<PathStackEntry[]>([])
  const [object, setObject] = useState<ObjectView | null>(null)
  const [readme, setReadme] = useState<ObjectView | null>(null)
  const [readmeName, setReadmeName] = useState('')
  const [blame, setBlame] = useState<BlameBlock[]>([])
  const [filter, setFilter] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  /** 竞态守卫：只有最后一次 load 的结果会被采纳。 */
  const loadSeq = useRef(0)

  /** 空仓库：连一个分支都没有（旧版 `FileTree.js` 用 `branches.length === 0` 判定）。 */
  const isEmptyRepo = (config?.branches?.length ?? 0) === 0

  const load = useCallback(async () => {
    if (!summary) {
      return
    }
    const seq = loadSeq.current + 1
    loadSeq.current = seq
    setLoading(true)
    setError(null)
    setStack([])
    setObject(null)
    setReadme(null)
    setReadmeName('')
    setBlame([])
    setFilter('')

    // 空仓库没有可解析的 ref，直接交给"初始化引导"渲染。
    if (isEmptyRepo) {
      return
    }

    const entries = path
      ? await fetchPathStack({ repositoryKey: summary.id, revision: effectiveRev, path })
      : []
    const calculatedPath = entries.map((entry) => entry.name).join('/')
    const parent = entries.length > 0 ? entries[entries.length - 1].object : effectiveRev
    const view = await fetchObject({
      repositoryKey: summary.id,
      parent,
      path: calculatedPath,
      revision: effectiveRev,
    })
    if (seq !== loadSeq.current) {
      return
    }
    setStack(entries)
    setObject(view)

    if (isDirectoryView(view)) {
      // README 自动渲染：当前目录里存在 readme.md 时把它的内容取出来。
      const readmeEntry = view.object.find(
        (entry) => entry.name.toLowerCase() === 'readme.md',
      )
      if (readmeEntry) {
        const readmeView = await fetchObject({
          repositoryKey: summary.id,
          parent: readmeEntry.object,
          path: calculatedPath ? `${calculatedPath}/${readmeEntry.name}` : readmeEntry.name,
          revision: effectiveRev,
        })
        if (seq !== loadSeq.current) {
          return
        }
        setReadme(readmeView)
        setReadmeName(readmeEntry.name)
      }
    }

    if (blameMode && view.type === 'blob') {
      const result = await fetchBlameInfo({
        repositoryKey: summary.id,
        revision: effectiveRev,
        path: calculatedPath,
      })
      if (seq !== loadSeq.current) {
        return
      }
      setBlame(result.blame)
    }
  }, [summary, effectiveRev, path, blameMode, isEmptyRepo])

  useEffect(() => {
    let cancelled = false
    load()
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
  }, [load])

  const refItems = useMemo(() => {
    const branches = (config?.branches ?? []).map((branch) => ({
      key: `b-${branch.id}`,
      label: branch.name,
      onClick: () => navigate(`${base}/files?rev=${encodeURIComponent(branch.name)}`),
    }))
    const tags = (config?.tags ?? []).map((tag) => ({
      key: `t-${tag.id}`,
      label: tag.name,
      onClick: () => navigate(`${base}/files?rev=${encodeURIComponent(tag.name)}`),
    }))
    return [...branches, ...tags]
  }, [config, base, navigate])

  const file = object?.type === 'blob' ? (object.object as FileObject) : null

  /** 目录优先、再子模块、最后普通文件（对齐旧版 `makeFileListData` 的排序）。 */
  const sortedEntries = useMemo(() => {
    const list = isDirectoryView(object) ? object.object : []
    const rank = (entry: ObjectEntry) => (entry.type === 'tree' ? 0 : entry.type === 'commit' ? 1 : 2)
    return [...list].sort((left, right) => rank(left) - rank(right))
  }, [object])

  const visibleEntries = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    if (!needle) {
      return sortedEntries
    }
    return sortedEntries.filter((entry) => entry.name.toLowerCase().includes(needle))
  }, [sortedEntries, filter])

  const crumbs = stack.map((entry) => entry.name)

  const toggleBlame = () => {
    const params = new URLSearchParams(searchParams)
    if (blameMode) {
      params.delete('blame')
    } else {
      params.set('blame', '1')
    }
    setSearchParams(params)
  }

  return (
    <div className="page">
      {/* 空仓库连默认分支都没有，工具条无从选择（与旧版一致：仅在有默认分支时渲染）。 */}
      {!isEmptyRepo ? (
        <div className="file-toolbar">
          <div className="row">
            <Dropdown
              triggerClassName="file-toolbar__rev"
              trigger={
                <>
                  <Icon icon={psBranch} size={16} />
                  {effectiveRev}
                  <Icon icon={psCaretDown} size={12} />
                </>
              }
              items={refItems}
              minWidth={200}
            />
            <nav className="breadcrumb">
              <Link className="breadcrumb__link" to={`${base}/files?rev=${encodeURIComponent(effectiveRev)}`}>
                {repositoryName}
              </Link>
              {crumbs.map((name, index) => {
                const sub = `${base}/files/${crumbs.slice(0, index + 1).join('/')}?rev=${encodeURIComponent(effectiveRev)}`
                const isLast = index === crumbs.length - 1
                return (
                  <span key={sub} className="row">
                    <span className="breadcrumb__sep">/</span>
                    {isLast ? (
                      <span className="breadcrumb__current">{name}</span>
                    ) : (
                      <Link className="breadcrumb__link" to={sub}>
                        {name}
                      </Link>
                    )}
                  </span>
                )
              })}
            </nav>
          </div>

          <div className="row">
            <TextField
              containerClassName="file-toolbar__search"
              placeholder={t('label.search')}
              value={filter}
              startAdornment={<Icon icon={plSearch} size={16} />}
              onChange={(event) => setFilter(event.target.value)}
            />
          </div>
        </div>
      ) : null}

      {error ? <div className="auth-message">{error}</div> : null}
      {loading ? <Loading /> : null}

      {!loading && object?.commit ? (
        <div className="commit-card">
          <CommitRow
            commit={object.commit}
            linkBase={`${base}/commit`}
            author={resolveMember(config?.members, object.commit.email)}
          />
        </div>
      ) : null}

      {!loading && !file && sortedEntries.length > 0 ? (
        <TableWrap>
          <Table className="cf-table--file">
            <THead>
              <TR>
                <TH style={{ width: '30%' }}>{t('label.name')}</TH>
                <TH>{t('label.lastCommit')}</TH>
                <TH style={{ width: 'calc(var(--cf-space) * 26)' }}>{t('label.lastUpdate')}</TH>
              </TR>
            </THead>
            <TBody>
              {visibleEntries.map((entry) => {
                if (entry.type === 'commit') {
                  // 子模块（.gitmodules）：不可进入，只展示指向的提交。
                  return (
                    <TR key={entry.object + entry.name}>
                      <TD className="file-table__name-cell">
                        <span className="file-table__name file-table__name--static" title={entry.url}>
                          <Icon icon={psRepository} size={16} />
                          <span className="file-table__name-label">{entry.name}</span>
                          <span className="file-table__submodule">@{entry.object.slice(0, 8)}</span>
                        </span>
                      </TD>
                      <TD />
                      <TD />
                    </TR>
                  )
                }
                const href = `${base}/files/${path ? `${path}/` : ''}${encodeURIComponent(entry.name)}?rev=${encodeURIComponent(effectiveRev)}`
                return (
                  <TR key={entry.object + entry.name}>
                    <TD className="file-table__name-cell">
                      <Link className="file-table__name" to={href}>
                        <Icon icon={entry.type === 'tree' ? psFolder : psFile} size={16} />
                        <span className="file-table__name-label">{entry.name}</span>
                      </Link>
                    </TD>
                    <TD className="file-table__commit-cell">
                      {entry.commit ? (
                        <Link className="file-table__commit" to={`${base}/commit/${entry.commit.sha.slice(0, 8)}`}>
                          {entry.commit.commit}
                        </Link>
                      ) : null}
                    </TD>
                    <TD>{entry.commit ? <FormattedTime timestamp={entry.commit.time} /> : null}</TD>
                  </TR>
                )
              })}
            </TBody>
          </Table>
        </TableWrap>
      ) : null}

      {!loading && !file && sortedEntries.length === 0 && !isEmptyRepo ? (
        <EmptyState imageName="commits-empty.png" title={t('message._S_listEmpty', { s: t('label.file') })} />
      ) : null}

      {!loading && file ? (
        <ObjectViewer
          file={file}
          path={path}
          base={base}
          rev={effectiveRev}
          blame={blame}
          blameMode={blameMode}
          onToggleBlame={toggleBlame}
        />
      ) : null}

      {!loading && readme?.type === 'blob' ? (
        <ObjectViewer
          file={readme.object as FileObject}
          path={path ? `${path}/${readmeName}` : readmeName}
          base={base}
          rev={effectiveRev}
        />
      ) : null}

      {!loading && isEmptyRepo ? (
        <RepositoryEmpty
          repositoryName={config?.repository.name ?? ''}
          groupName={config?.group.name ?? ''}
          host={user?.host ?? ''}
        />
      ) : null}
    </div>
  )
}
