import { NavLink, Outlet, useLocation } from 'react-router'

import { useT } from '@/i18n'


type SettingsTabKey = 'profile' | 'sshkey' | 'notification' | 'tokens' | 'appearance'

/**
 * 个人设置外壳（v5 §6 G11：左锚点导航 200px + 分区表单）。
 *
 * v4 的左侧 Drawer 改为**锚点导航面板**（纸页，激活 = `--bg-selected` 底 +
 * `--brand-text`，无竖条）；访问令牌 / 外观（主题与语言）沿用 v4 已有路由。
 * 2026-11-22 减负：邮箱管理页下线（改口令 / 头像仍在「个人信息」页）。
 */
const TABS: Array<{ key: SettingsTabKey; labelKey: string; to: string }> = [
  { key: 'profile', labelKey: 'menu.profile', to: '/settings/profile' },
  { key: 'sshkey', labelKey: 'menu.SSHKey_pl', to: '/settings/sshkey' },
  { key: 'notification', labelKey: 'menu.notification', to: '/settings/notification' },
  { key: 'tokens', labelKey: 'tokens.title', to: '/settings/tokens' },
  { key: 'appearance', labelKey: 'appearance.title', to: '/settings/appearance' },
]

function activeKey(pathname: string): SettingsTabKey {
  if (/^\/settings\/sshkey/i.test(pathname)) return 'sshkey'
  if (/^\/settings\/notification/i.test(pathname)) return 'notification'
  if (/^\/settings\/tokens/i.test(pathname)) return 'tokens'
  if (/^\/settings\/appearance/i.test(pathname)) return 'appearance'
  return 'profile'
}

export function SettingsLayout() {
  const t = useT()
  const { pathname } = useLocation()
  const active = activeKey(pathname)

  return (
    <div className="page settings-page">
      <aside className="settings-nav">
        <nav className="settings-nav__list">
          {TABS.map((tab) => (
            <NavLink
              key={tab.key}
              to={tab.to}
              className={tab.key === active ? 'settings-nav__item settings-nav__item--active' : 'settings-nav__item'}
              aria-current={tab.key === active ? 'page' : undefined}
            >
              {t(tab.labelKey)}
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className="settings-page__main">
        <Outlet />
      </div>
    </div>
  )
}
