import { cn } from '@/helpers/cn'


/** 单色旋转指示器。旧版有 38 个文件各自 import MUI 的 CircularProgress，本组件是唯一替代品。 */
export function Spinner({ className }: { className?: string }) {
  return <span className={cn('cf-spinner', className)} aria-hidden="true" />
}

export interface LoadingProps {
  text?: string
  className?: string
}

/** 行内加载态（表格/面板内部使用）。 */
export function Loading({ text = '加载中…', className }: LoadingProps) {
  return (
    <div className={cn('cf-loading', className)} role="status">
      <Spinner />
      <span>{text}</span>
    </div>
  )
}

/** 路由/页面级加载占位：给足高度，避免 chunk 到达后插入内容造成布局跳动（CLS）。 */
export function PageLoading({ text = '加载中…' }: { text?: string }) {
  return (
    <div className="cf-page-loading" role="status">
      {text}
    </div>
  )
}
