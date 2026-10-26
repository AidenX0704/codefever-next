import { cn } from '@/helpers/cn'

import { CopyButton } from '@/components/unit/CopyButton'


export interface CodeBlockProps {
  /** 片段原文（按等宽字体原样展示，不做语法高亮）。 */
  code: string
  /** 语言标签（仅展示用，例如 `bash` / `json`）。 */
  language?: string
  /** 复制成功提示文案（传 i18n 文案，缺省用组件的通用文案）。 */
  copyMessage?: string
  className?: string
}

/**
 * 文档片段代码块（等宽 + 左上语言标签 + 右上复制）。
 *
 * 与 `CodeViewer` 的分工：`CodeViewer` 面向**仓库文件**（行号、Prism 高亮、可跳行），
 * 这里面向**文档示例**（curl 命令、JSON 响应）—— 不需要行号，但需要一键复制。
 * 帮助中心「API 接口文档」区块的两段示例就靠它，避免页面里手写 `pre` + 复制逻辑。
 */
export function CodeBlock({ code, language, copyMessage, className }: CodeBlockProps) {
  return (
    <div className={cn('cf-code-block', className)}>
      <div className="cf-code-block__bar">
        <span className="cf-code-block__lang">{language}</span>
        <CopyButton text={code} message={copyMessage} />
      </div>
      <pre className="cf-code-block__pre">
        <code>{code}</code>
      </pre>
    </div>
  )
}
