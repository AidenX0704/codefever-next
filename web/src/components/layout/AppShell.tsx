import { Outlet } from 'react-router'

import { AppDataProvider } from '@/features/app-data/AppDataProvider'
import { Header } from './Header'
import { PlatformSidebar } from './PlatformSidebar'


/**
 * 应用外壳（v5 DESIGN.md §4.1：悬浮玻璃侧栏 + 通栏雾面顶栏）。
 *
 * - 平台侧栏是 fixed 悬浮面板，内容区偏移走 `--rail-current` 令牌；
 * - 顶栏通栏雾面，左侧面包屑、右侧全局动作，中部留给命令面板；
 * - 内容区底色是雾白画布（`--bg-canvas`），卡片 / 表格 / 长文本用纸页（`--bg-sheet`）。
 */
export function AppShell() {
  return (
    <AppDataProvider>
      <div className="app-root">
        <Header />
        <PlatformSidebar />
        <div className="app-content">
          <main className="app-main">
            <Outlet />
          </main>
        </div>
      </div>
    </AppDataProvider>
  )
}
