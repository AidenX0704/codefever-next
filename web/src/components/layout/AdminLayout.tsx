import { Outlet } from 'react-router'

import { adminDrawerEntries } from '@/config/drawer'
import { useT } from '@/i18n'
import { Sidebar } from './Sidebar'


/** 管理后台外壳（`/admin/*`）：左侧 5 项导航 + 右侧内容区，与旧版 admin Drawer 一致。 */
export function AdminLayout() {
  const t = useT()

  return (
    <div className="page page--full">
      <div className="cf-repo-layout">
        <Sidebar entries={adminDrawerEntries()} />
        <div className="cf-repo-main">

          <div className="repo-head">
            <div className="repo-head__title">{t('label.adminArea')}</div>
          </div>
          <Outlet />
        </div>
      </div>
    </div>
  )
}
