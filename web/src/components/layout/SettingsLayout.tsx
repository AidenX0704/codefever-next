import { Outlet } from 'react-router'

import { Sidebar } from './Sidebar'
import { settingsDrawerEntries } from '@/config/drawer'
import { useT } from '@/i18n'


/**
 * 个人设置外壳（`/settings/*`）。
 *
 * 结构沿用旧版：左侧 4 项导航（个人资料 / 邮箱 / SSH Key / 通知）+ 右侧内容区。
 * 旧版 Drawer 只在 `/settings` 下渲染，这里通过独立布局表达同一件事。
 */
export function SettingsLayout() {
  const t = useT()

  return (
    <div className="page page--full">
      <div className="cf-repo-layout">
        <Sidebar entries={settingsDrawerEntries()} />
        <div className="cf-repo-main">

          <div className="repo-head">
            <div className="repo-head__title">{t('menu.setting_pl')}</div>
          </div>
          <Outlet />
        </div>
      </div>
    </div>
  )
}
