import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'


/** remark 插件数组放模块级：旧版同样把它提到组件外（避免每次渲染新建数组）。 */
const REMARK_PLUGINS = [remarkGfm]

/**
 * Markdown 渲染（README / `.md` 文件）。
 *
 * 单独成文件是为了让调用方能用 `React.lazy` 按需加载：
 * `react-markdown` + `remark-gfm` 及其依赖约 172 KB，
 * 只有真正查看 Markdown 文件时才需要（对齐旧版性能整改 E3）。
 */
export default function MarkdownView({ raw }: { raw: string }) {
  return (
    <div className="markdown-body">
      <ReactMarkdown remarkPlugins={REMARK_PLUGINS}>{raw}</ReactMarkdown>
    </div>
  )
}
