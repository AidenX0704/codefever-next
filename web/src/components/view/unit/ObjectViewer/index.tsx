import { lazy, Suspense, useState } from 'react'
import { useNavigate } from 'react-router'

import type { BlameBlock, FileObject } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { CodeViewer } from '@/components/unit/CodeViewer'
import { CopyButton } from '@/components/unit/CopyButton'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { psFile } from '@/config/icons'
import { formatBytes } from '@/helpers/format'
import { useT } from '@/i18n'


/** Markdown 渲染器按需加载（见 `MarkdownView.tsx` 的说明）。 */
const MarkdownView = lazy(() => import('@/components/view/unit/MarkdownView'))

type DisplayMode = 'oversize' | 'empty' | 'markdown' | 'text' | 'image' | 'video' | 'unknown'

/** 旧版 `ObjectViewer.getDisplayMode()` 的判定（逐字对齐）。 */
function displayModeOf(file: FileObject, path: string): DisplayMode {
  if (file.oversize) {
    return 'oversize'
  }
  if ((file.size ?? 0) <= 0) {
    return 'empty'
  }
  const mime = file.mime ?? ''
  if (mime.startsWith('text/') || mime === 'application/xml' || mime === 'text/xml') {
    return /\.md$/i.test(path) ? 'markdown' : 'text'
  }
  if (mime.startsWith('image/')) {
    return 'image'
  }
  if (mime === 'video/mp4') {
    return 'video'
  }
  return 'unknown'
}

export interface ObjectViewerProps {
  /** 文件内容（`repository/object` 返回的 blob）。 */
  file: FileObject
  /** 完整路径（展示 + 复制用）。 */
  path: string
  /** 跳转前缀（`/{组}/{仓库}`）。 */
  base: string
  /** 当前 ref（历史 / blame 跳转用）。 */
  rev: string
  /** blame 归属区间（可选）。 */
  blame?: BlameBlock[]
  /** 是否处于 blame 模式。 */
  blameMode?: boolean
  /** 切换 blame 模式。 */
  onToggleBlame?: () => void
}

/**
 * 文件内容查看器（对应旧版 `components/unit/ObjectViewer.js`）。
 *
 * 头部：文件路径 + 大小 + **复制路径 / 复制源码** + **Markdown 渲染切换** + **历史入口** + **Blame 切换**。
 * 主体：文本走 `CodeViewer`（Prism）、Markdown 走渲染器、图片 / 视频直接展示、
 * 二进制与超限给出提示。
 */
export function ObjectViewer({
  file,
  path,
  base,
  rev,
  blame,
  blameMode = false,
  onToggleBlame,
}: ObjectViewerProps) {
  const t = useT()
  const navigate = useNavigate()
  const [showSource, setShowSource] = useState(false)

  const mode = displayModeOf(file, path)
  const isText = mode === 'text' || mode === 'markdown'
  const raw = file.raw ?? ''

  return (
    <div className="file-view">
      <div className="file-view__head">
        <div className="file-view__meta">
          <Icon icon={psFile} size={16} />
          <span className="file-view__path">{path}</span>
          <span className="file-view__size">{formatBytes(file.size ?? 0)}</span>
          <CopyButton text={path} label={t('label.copyPath')} message={t('label.copyPath')} />
          {isText ? (
            <CopyButton text={raw} label={t('label.copyCode')} message={t('label.copyCode')} />
          ) : null}
        </div>

        <div className="file-view__actions">
          {mode === 'markdown' ? (
            <Button variant="text" size="sm" onClick={() => setShowSource((value) => !value)}>
              {showSource ? t('message.displayRendereFile') : t('message.displaySource')}
            </Button>
          ) : null}
          <Button
            variant="outlined"
            size="sm"
            onClick={() => navigate(`${base}/commits?rev=${encodeURIComponent(rev)}`)}
          >
            {t('label.history')}
          </Button>
          {isText && onToggleBlame ? (
            <Button variant="outlined" size="sm" onClick={onToggleBlame}>
              {blameMode ? t('label.normal') : t('label.blame')}
            </Button>
          ) : null}
        </div>
      </div>

      {blameMode && blame && blame.length > 0 ? (
        <div className="blame-list">
          {blame.map((block) => (
            <div className="blame-row" key={`${block.start}-${block.commit.sha}`}>
              <span className="blame-row__lines">
                {block.start}-{block.start + block.length - 1}
              </span>
              <span>{block.commit.commit}</span>
              <span className="blame-row__lines">{block.commit.sha.slice(0, 8)}</span>
              <FormattedTime timestamp={Number(block.commit.time)} />
            </div>
          ))}
        </div>
      ) : null}

      <div className="file-view__body">
        {mode === 'oversize' ? (
          <div className="file-view__placeholder">{t('message.fileContentOversize')}</div>
        ) : null}
        {mode === 'empty' ? (
          <div className="file-view__placeholder">{t('message.fileContentEmpty')}</div>
        ) : null}
        {mode === 'unknown' || file.unsupport ? (
          <div className="file-view__placeholder">{t('message.fileTypeNotSupport')}</div>
        ) : null}
        {mode === 'image' && file.base64Encoded ? (
          <img
            className="file-view__media"
            src={`data:${file.mime};base64,${file.base64Encoded}`}
            alt={path}
          />
        ) : null}
        {mode === 'video' && file.base64Encoded ? (
          <video className="file-view__media" controls>
            <source src={`data:${file.mime};base64,${file.base64Encoded}`} type={file.mime} />
          </video>
        ) : null}
        {mode === 'text' ? <CodeViewer raw={raw} path={path} /> : null}
        {mode === 'markdown' ? (
          <>
            <div hidden={!showSource}>
              <CodeViewer raw={raw} path={path} />
            </div>
            <div hidden={showSource} className="file-view__markdown">
              <Suspense fallback={<div className="file-view__placeholder">{t('label.loading')}</div>}>
                <MarkdownView raw={raw} />
              </Suspense>
            </div>
          </>
        ) : null}
      </div>
    </div>
  )
}
