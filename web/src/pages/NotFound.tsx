import { useNavigate } from 'react-router'

import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'

/**
 * 404 页。
 * 旧版由 nopage/index.php 渲染 Bootstrap 静态页，这里用组件库重做，文案保持一致。
 * 注：刻意用 `navigate` 而不是 `<a>` 包裹按钮 —— 交互元素不能嵌套（a > button 是非法结构）。
 */
export default function NotFound() {
  const navigate = useNavigate()

  return (
    <EmptyState
      title="The page you’re looking for is not here."
      notice="抱歉，页面不见了~"
      action={
        <Button variant="outlined" onClick={() => navigate('/repositories')}>
          返回首页
        </Button>
      }
    />
  )
}
