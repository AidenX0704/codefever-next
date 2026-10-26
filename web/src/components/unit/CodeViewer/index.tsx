import { useMemo } from 'react'

import { highlightLines } from '@/helpers/prism'


export interface CodeViewerProps {
  /** 文件原文（接口返回的 `object.raw`）。 */
  raw: string
  /** 文件路径，用于判定高亮语言（`null` 时按 markdown）。 */
  path?: string | null
  /** 指定语言（覆盖按扩展名推断，diff 片段会用）。 */
  language?: string
  /** 跳转锚点行号（`#L12` 场景）；为 1 基。 */
  focusLine?: number
}

/**
 * 代码查看器。
 *
 * 视觉口径来自旧版 `components/unit/CodeViewer.js`：
 *   - 表格布局，行号列宽 32px(spacing 4)、左右内边距 8px、右对齐、字号 12px、
 *     文字色 `text-lighter`、底色 `bg-main`；
 *   - 行高 16px(spacing 2)，代码单元允许换行（`overflow-wrap: break-word`）；
 *   - 高亮用 Prism 的 `coy` 主题（第三方固定色值，集中见 `helpers/prism.ts`）。
 *
 * 与旧版差异：blame 侧栏尚未实现（依赖提交详情组件，批 3 后续批次补）。
 */
export function CodeViewer({ raw, path, language, focusLine }: CodeViewerProps) {
  const lines = useMemo(() => highlightLines(raw, path, language), [raw, path, language])
  const plainLines = useMemo(() => raw.split('\n'), [raw])

  return (
    <table className="cf-code">
      <tbody>
        {plainLines.map((line, index) => {
          const isFocus = focusLine !== undefined && index + 1 === focusLine
          return (
            <tr key={index} className={isFocus ? 'cf-code__row--focus' : undefined}>
              <td className="cf-code__lineno">
                <code>{index + 1}</code>
              </td>
              <td className="cf-code__cell">
                {/* Prism 的输出是自带的 HTML（token 化），这里只能走 innerHTML；
                    内容来自用户仓库文件，Prism 会对文本做转义后再包裹 token。 */}
                <code className="cf-code__code" dangerouslySetInnerHTML={{ __html: lines[index] ?? escapeHtml(line) }} />
              </td>
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
