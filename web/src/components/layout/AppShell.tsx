import { Outlet } from 'react-router'

import { AppDataProvider } from '@/features/app-data/AppDataProvider'
import { Header } from './Header'


/**
 * 应用外壳（批 2：页头 + 内容容器）。
 *
 * 布局口径来自旧版 `components/App.js` + `components/Main.js`：
 *   - 内容区底色是**白色**（`theme.palette.background.light`），不是 bg-main；
 *   - 固定页头（Toolbar 64px）用一个等高占位块让开，避免内容被遮住；
 *   - 主内容内边距 `spacing(3) spacing(6)` = 24px 48px。
 *
 * 批 3 会在这里补 Drawer（仓库/组/设置/管理四种上下文菜单）。
 */
export function AppShell() {
  return (
    <AppDataProvider>
      <div className="app-root">
        <Header />
        <div className="app-content">
          <main className="app-main">
            <Outlet />
          </main>
        </div>
      </div>
    </AppDataProvider>
  )
}
