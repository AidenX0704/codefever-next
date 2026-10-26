import { useMemo, useState } from 'react'
import { useParams } from 'react-router'

import { fetchObject } from '@/api/repository'
import type { DiffFile } from '@/api/types'
import { AppLink } from '@/components/unit/AppLink'
import { CodeViewer } from '@/components/unit/CodeViewer'
import { CopyButton } from '@/components/unit/CopyButton'
import { Icon } from '@/components/unit/Icon'
import { IconButton } from '@/components/unit/IconButton'
import { Loading } from '@/components/unit/Loading'
import { psFile, plAngleDown, plAngleRight, plCopy, plExpand } from '@/config/icons'
import { languageForPath } from '@/config/languages'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { cn } from '@/helpers/cn'
import { highlightLines } from '@/helpers/prism'
import { useT } from '@/i18n'


interface RenderedLine {
  kind: 'hunk' | 'add' | 'del' | 'context'
  oldNo: number | null
  newNo: number | null
  text: string
  html: string
}

/** 双列（side-by-side）模式的一格。 */
interface SplitCell {
  kind: 'add' | 'del' | 'context'
  no: number | null
  html: string
}

/** 双列模式的一行：左侧旧版、右侧新版；hunk 头单独占一行。 */
interface SplitRow {
  hunk?: string
  left?: SplitCell
  right?: SplitCell
}

/** unified diff 的 hunk 头：`@@ -a,b +c,d @@`。 */
const HUNK_PATTERN = /^@@\s-+(\d+)(?:,\d+)?\s+\++(\d+)(?:,\d+)?\s+@@/i

/** 解析 unified diff 的逐行数据。
 *
 * 与旧 `CodeDiffViewer.makeDiffContent()` 的算法一致：遇到 `@@ -a,b +c,d @@`
 * 重置两侧行号，`-` 行只增旧侧、`+` 行只增新侧、其余两侧同增。
 * 逐行高亮复用 `helpers/prism.ts`（同上：单行高亮，diff 场景不会有多行 token 问题）。
 */
function renderLines(file: DiffFile, language: string | null): RenderedLine[] {
  const lines = file.diff?.line ?? []
  const out: RenderedLine[] = []
  let oldNo = 0
  let newNo = 0

  for (const line of lines) {
    const hunk = line.match(HUNK_PATTERN)
    if (hunk) {
      oldNo = Number(hunk[1])
      newNo = Number(hunk[2])
      out.push({ kind: 'hunk', oldNo: null, newNo: null, text: line, html: escapeHtml(line) })
      continue
    }
    const sign = line[0]
    const text = line.slice(1)
    if (sign === '-') {
      out.push({ kind: 'del', oldNo, newNo: null, text, html: highlightLine(text, language) })
      oldNo += 1
    } else if (sign === '+') {
      out.push({ kind: 'add', oldNo: null, newNo, text, html: highlightLine(text, language) })
      newNo += 1
    } else {
      out.push({ kind: 'context', oldNo, newNo, text, html: highlightLine(text, language) })
      oldNo += 1
      newNo += 1
    }
  }
  return out
}

/**
 * 解析成双列（side-by-side）行。
 *
 * git 的 unified diff 在一个变更块里先给全部 `-` 行、再给全部 `+` 行；
 * 这里按顺序两两配对，多出来的行留空另一侧（旧版 `CodeDiffViewer` 的 side-by-side 同口径）。
 */
function renderSplitRows(file: DiffFile, language: string | null): SplitRow[] {
  const lines = file.diff?.line ?? []
  const rows: SplitRow[] = []
  let oldNo = 0
  let newNo = 0
  let removed: SplitCell[] = []
  let added: SplitCell[] = []

  const flush = () => {
    const count = Math.max(removed.length, added.length)
    for (let index = 0; index < count; index += 1) {
      rows.push({ left: removed[index], right: added[index] })
    }
    removed = []
    added = []
  }

  for (const line of lines) {
    const hunk = line.match(HUNK_PATTERN)
    if (hunk) {
      flush()
      oldNo = Number(hunk[1])
      newNo = Number(hunk[2])
      rows.push({ hunk: line })
      continue
    }
    const sign = line[0]
    const text = line.slice(1)
    if (sign === '-') {
      removed.push({ kind: 'del', no: oldNo, html: highlightLine(text, language) })
      oldNo += 1
    } else if (sign === '+') {
      added.push({ kind: 'add', no: newNo, html: highlightLine(text, language) })
      newNo += 1
    } else {
      flush()
      const html = highlightLine(text, language)
      rows.push({ left: { kind: 'context', no: oldNo, html }, right: { kind: 'context', no: newNo, html } })
      oldNo += 1
      newNo += 1
    }
  }
  flush()
  return rows
}

function highlightLine(text: string, language: string | null): string {
  if (!language) {
    return escapeHtml(text)
  }
  return highlightLines(text, null, language)[0] ?? escapeHtml(text)
}

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

/** diff 布局：inline = 单列（旧版 inline），split = 双列（旧版 side-by-side）。 */
export type DiffLayout = 'inline' | 'split'

/** "展开完整文件"取回的内容。 */
type FullContent =
  | { kind: 'text'; raw: string }
  | { kind: 'image'; src: string }
  | { kind: 'message'; text: string }

export interface DiffViewerProps {
  files: DiffFile[]
  /** 布局模式；不传则由组件内部状态控制（默认 inline）。 */
  layout?: DiffLayout
  onLayoutChange?: (layout: DiffLayout) => void
  /** 汇总条（所有文件 N / +add / -del + 布局切换）。默认显示。 */
  showSummary?: boolean
  /** 逐文件折叠面板。默认开启。 */
  collapsible?: boolean
  /**
   * "查看文件"时定位到的 commit-ish。
   *
   * diff 里的 SHA 是**blob** 的 abbreviated SHA（后端 `ParseDiff` 解析 `index a..b` 行），
   * 不能直接当 rev 用，因此缺省取仓库默认分支；调用方（如 MR 详情）知道
   * 源/目标提交时可以显式传进来。
   */
  rev?: string
}

/**
 * 文件级 diff 视图。
 *
 * 视觉口径来自旧 `components/unit/CodeDiffViewer.js`：
 *   - 每个文件一个区块：文件名（改名时显示 `旧名 → 新名`）+ `+x -y` 统计；
 *   - 行级着色取 `--cf-color-diff-addition` / `--cf-color-diff-deletion` /
 *     `--cf-color-diff-label`（旧主题 preset 的 `background.addition` 等）；
 *   - 双列行号（旧 | 新）。
 * `type === 'oversize'` 的文件按旧版语义只显示提示，不渲染正文。
 *
 * 与旧 `components/view/MergeRequestDiff.js` 对齐的补齐项：
 *   - 汇总条（所有文件 N 个 / +add / -del）；
 *   - inline / side-by-side 布局切换；
 *   - 逐文件折叠面板；
 *   - 复制路径 / 查看 @hash / 展开完整文件 / mode 变更提示。
 */
export function DiffViewer({ files, layout, onLayoutChange, showSummary = true, collapsible = true, rev }: DiffViewerProps) {
  const t = useT()
  const { groupName, repositoryName } = useParams()
  const { repositories } = useAppData()

  const [innerLayout, setInnerLayout] = useState<DiffLayout>('inline')
  const currentLayout = layout ?? innerLayout

  const repository = useMemo(
    () => repositories.find((item) => item.group.name === groupName && item.name === repositoryName) ?? null,
    [repositories, groupName, repositoryName],
  )
  const repositoryPath = groupName && repositoryName ? `/${groupName}/${repositoryName}` : null
  const viewRev = rev ?? repository?.defaultBranch ?? ''

  const totals = useMemo(() => {
    let additions = 0
    let deletions = 0
    files.forEach((file) => {
      additions += file.diff?.additions ?? 0
      deletions += Math.abs(file.diff?.deletions ?? 0)
    })
    return { additions, deletions }
  }, [files])

  const switchLayout = () => {
    const next: DiffLayout = currentLayout === 'inline' ? 'split' : 'inline'
    if (layout === undefined) {
      setInnerLayout(next)
    }
    onLayoutChange?.(next)
  }

  return (
    <div className="cf-diff">
      {showSummary ? (
        <div className="cf-diff__summary">
          <span className="cf-diff__summary-files">
            {t('label.allFiles')}: {files.length}
          </span>
          <span className="cf-diff__add">+{totals.additions}</span>
          <span className="cf-diff__del">-{totals.deletions}</span>
          <button type="button" className="cf-diff__layout" onClick={switchLayout}>
            {currentLayout === 'inline' ? 'side-by-side' : 'inline'}
          </button>
        </div>
      ) : null}

      {files.map((file, index) => (
        <DiffFileBlock
          key={`${file.original.name}-${file.modified.name}-${index}`}
          file={file}
          layout={currentLayout}
          collapsible={collapsible}
          oversizeLabel={t('message.fileDiffOversize')}
          repositoryKey={repository?.id ?? null}
          repositoryPath={repositoryPath}
          viewRev={viewRev}
        />
      ))}
    </div>
  )
}

interface DiffFileBlockProps {
  file: DiffFile
  layout: DiffLayout
  collapsible: boolean
  oversizeLabel: string
  /** 当前仓库主键（"展开完整文件"需要）。 */
  repositoryKey: string | null
  /** 当前仓库路径前缀（"查看 @hash"需要）。 */
  repositoryPath: string | null
  /** "查看文件"用的 commit-ish（缺省为仓库默认分支）。 */
  viewRev: string
}

function DiffFileBlock({
  file,
  layout,
  collapsible,
  oversizeLabel,
  repositoryKey,
  repositoryPath,
  viewRev,
}: DiffFileBlockProps) {
  const t = useT()
  const [expanded, setExpanded] = useState(true)
  const [full, setFull] = useState<FullContent | null>(null)
  const [fullLoading, setFullLoading] = useState(false)

  const language = useMemo(() => languageFromName(file.modified.name ?? file.original.name), [file])
  const lines = useMemo(() => (file.type === 'oversize' ? [] : renderLines(file, language)), [file, language])
  const splitRows = useMemo(() => (file.type === 'oversize' ? [] : renderSplitRows(file, language)), [file, language])

  const renamed = file.original.name !== file.modified.name
  // 后端 deletions 沿用 PHP 的递减写法（可能为负），展示时取绝对值。
  const deletions = Math.abs(file.diff?.deletions ?? 0)
  const path = file.modified.name ?? file.original.name ?? ''
  const modeChanged = Boolean(file.original.mode && file.modified.mode && file.original.mode !== file.modified.mode)
  const sha = file.modified.sha ?? ''
  const viewHref = repositoryPath && viewRev && path ? `${repositoryPath}/files/${path}?rev=${encodeURIComponent(viewRev)}` : null

  const loadFullFile = async () => {
    if (!repositoryKey || !sha || !path) {
      return
    }
    setFullLoading(true)
    try {
      // 只取 blob 本身：不传 `revision`（后端会拿它去查"最后提交"，
      // 而 diff 里的 SHA 是 blob 的 abbreviated SHA，不是 commit）。
      const view = await fetchObject({ repositoryKey, parent: sha, path })
      const object = view.object
      if (Array.isArray(object)) {
        setFull({ kind: 'message', text: t('message.fileTypeNotSupport') })
      } else if (object.raw !== undefined) {
        setFull({ kind: 'text', raw: object.raw })
      } else if (object.base64Encoded) {
        setFull({ kind: 'image', src: `data:${object.mime ?? 'image/png'};base64,${object.base64Encoded}` })
      } else if (object.oversize) {
        setFull({ kind: 'message', text: t('message.fileContentOversize') })
      } else {
        setFull({ kind: 'message', text: t('message.fileTypeNotSupport') })
      }
    } catch {
      setFull({ kind: 'message', text: t('base_error_message') })
    } finally {
      setFullLoading(false)
    }
  }

  const toggleFullFile = () => {
    if (full) {
      setFull(null)
      return
    }
    void loadFullFile()
  }

  return (
    <section className="cf-diff__file">
      <header className="cf-diff__head cf-diff__head--panel">
        {collapsible ? (
          <button
            type="button"
            className="cf-diff__toggle"
            onClick={() => setExpanded((previous) => !previous)}
            aria-expanded={expanded}
          >
            <Icon icon={expanded ? plAngleDown : plAngleRight} size={14} />
            <span className="cf-diff__name">
              {renamed ? (
                <>
                  <span className="cf-diff__old">{file.original.name}</span>
                  <span className="cf-diff__arrow">→</span>
                </>
              ) : null}
              {file.modified.name ?? file.original.name}
            </span>
          </button>
        ) : (
          <span className="cf-diff__name">
            {renamed ? (
              <>
                <span className="cf-diff__old">{file.original.name}</span>
                <span className="cf-diff__arrow">→</span>
              </>
            ) : null}
            {file.modified.name ?? file.original.name}
          </span>
        )}

        {modeChanged ? (
          <span className="cf-diff__mode">
            {file.original.mode} → {file.modified.mode}
          </span>
        ) : null}

        <span className="cf-diff__stats">
          <span className="cf-diff__add">+{file.diff?.additions ?? 0}</span>
          <span className="cf-diff__del">-{deletions}</span>
        </span>

        <span className="cf-diff__actions">
          {path ? (
            <CopyButton text={path} label={t('label.copyPath')} message={t('label.copied')}>
              <Icon icon={plCopy} size={14} />
            </CopyButton>
          ) : null}
          {viewHref ? (
            <AppLink to={viewHref} title={`${t('label.browserFile')}@${viewRev}`}>
              <Icon icon={psFile} size={14} />
            </AppLink>
          ) : null}
          {repositoryKey && sha && path ? (
            <IconButton
              label={full ? t('label.showDiffOnly') : t('label.expandFullFile')}
              onClick={toggleFullFile}
              disabled={fullLoading}
            >
              <Icon icon={plExpand} size={14} />
            </IconButton>
          ) : null}
        </span>
      </header>

      {expanded ? (
        file.type === 'oversize' ? (
          <div className="cf-diff__oversize">{oversizeLabel}</div>
        ) : full ? (
          <FullContentView content={full} path={path} />
        ) : fullLoading ? (
          <Loading />
        ) : layout === 'split' ? (
          <SplitDiffTable rows={splitRows} />
        ) : (
          <InlineDiffTable lines={lines} />
        )
      ) : null}
    </section>
  )
}

/** 单列（inline）diff 表。 */
function InlineDiffTable({ lines }: { lines: RenderedLine[] }) {
  return (
    <table className="cf-diff__table">
      <tbody>
        {lines.map((line, index) => (
          <tr key={index} className={`cf-diff__row cf-diff__row--${line.kind}`}>
            <td className="cf-diff__no">{line.oldNo ?? ''}</td>
            <td className="cf-diff__no">{line.newNo ?? ''}</td>
            <td className="cf-diff__code">
              <code dangerouslySetInnerHTML={{ __html: line.html || '&nbsp;' }} />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

/** 双列（side-by-side）diff 表：左旧右新。 */
function SplitDiffTable({ rows }: { rows: SplitRow[] }) {
  return (
    <table className="cf-diff__table cf-diff__table--split">
      <tbody>
        {rows.map((row, index) => {
          if (row.hunk !== undefined) {
            return (
              <tr key={index} className="cf-diff__row cf-diff__row--hunk">
                <td className="cf-diff__hunk" colSpan={4}>
                  {row.hunk}
                </td>
              </tr>
            )
          }
          return (
            <tr key={index} className="cf-diff__row cf-diff__row--split">
              <td className={cn('cf-diff__no', row.left && `cf-diff__no--${row.left.kind}`)}>{row.left?.no ?? ''}</td>
              <td className={cn('cf-diff__code', row.left && `cf-diff__code--${row.left.kind}`)}>
                <code dangerouslySetInnerHTML={{ __html: row.left?.html || '&nbsp;' }} />
              </td>
              <td className={cn('cf-diff__no', row.right && `cf-diff__no--${row.right.kind}`)}>{row.right?.no ?? ''}</td>
              <td className={cn('cf-diff__code', row.right && `cf-diff__code--${row.right.kind}`)}>
                <code dangerouslySetInnerHTML={{ __html: row.right?.html || '&nbsp;' }} />
              </td>
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}

/** "展开完整文件"的内容区。 */
function FullContentView({ content, path }: { content: FullContent; path: string }) {
  if (content.kind === 'text') {
    return (
      <div className="cf-diff__full">
        <CodeViewer raw={content.raw} path={path} />
      </div>
    )
  }
  if (content.kind === 'image') {
    return (
      <div className="cf-diff__full cf-diff__full--image">
        <img src={content.src} alt="" />
      </div>
    )
  }
  return <div className="cf-diff__oversize">{content.text}</div>
}

function languageFromName(name: string | null): string | null {
  if (!name) {
    return null
  }
  // 与 CodeViewer 共用同一张扩展名映射（config/languages.ts），不再本地维护副本。
  const language = languageForPath(name)
  return language === 'markdown' ? null : language
}
