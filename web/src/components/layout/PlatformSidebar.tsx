import { useEffect, useState } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router'

import { plMenuCollapse } from '@/config/icons'
import { platformNavGroups } from '@/config/platformNav'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'
import { Icon } from '@/components/unit/Icon'


const RAIL_STORAGE_KEY = 'codefever.rail.collapsed'

/**
 * 平台级侧栏（v5 DESIGN.md §4.1：悬浮玻璃面板）。
 *
 * - 悬浮：四周 12px 内距、16px 圆角、毛玻璃材质，压在顶栏之上；
 * - 导航激活 = 主题色浅底滑块 + 主题色图标文字（无边、无影、无竖条）；
 * - 折叠按钮在侧栏底部（v5 把折叠从右缘中部移到这里），收起后只留图标；
 * - 内容区偏移通过 `<html data-rail>` 联动 `--rail-current` 令牌。
 */
export function PlatformSidebar() {
  const t = useT()
  const navigate = useNavigate()
  const location = useLocation()
  const { user } = useAppData()
  const [collapsed, setCollapsed] = useState<boolean>(() => {
    try {
      return window.localStorage.getItem(RAIL_STORAGE_KEY) === '1'
    } catch {
      return false
    }
  })

  useEffect(() => {
    try {
      window.localStorage.setItem(RAIL_STORAGE_KEY, collapsed ? '1' : '0')
    } catch {
      /* localStorage 不可用时忽略：只影响下次是否记住折叠状态 */
    }
    document.documentElement.dataset.rail = collapsed ? 'collapsed' : 'expanded'
    return () => {
      delete document.documentElement.dataset.rail
    }
  }, [collapsed])

  const groups = platformNavGroups().map((group) => ({
    ...group,
    // 「管理」按账号权限门控（CurrentUser.admin 以服务端返回为准）。
    entries: group.titleKey === 'nav.admin' && !user?.admin ? [] : group.entries,
  }))

  return (
    <aside className={collapsed ? 'cf-rail cf-rail--collapsed' : 'cf-rail'}>
      <button type="button" className="cf-rail__brand" onClick={() => navigate('/home')}>
        <span className="cf-rail__mark" aria-hidden="true">
          <i /><i /><i /><i />
        </span>
        <span className="cf-rail__brand-name">CodeFever</span>
      </button>

      <nav className="cf-rail__nav">
        {groups.map((group) => (
          group.entries.length ? (
            <div key={group.titleKey} className="cf-rail__group">
              <div className="cf-rail__group-title">{t(group.titleKey)}</div>
              {group.entries.map((entry) => {
                const active = entry.active(location.pathname)
                return (
                  <NavLink
                    key={entry.to}
                    to={entry.to}
                    className={active ? 'cf-rail__item cf-rail__item--active' : 'cf-rail__item'}
                    aria-current={active ? 'page' : undefined}
                  >
                    <Icon icon={entry.icon} size={18} />
                    <span className="cf-rail__label">{t(entry.labelKey)}</span>
                  </NavLink>
                )
              })}
            </div>
          ) : null
        ))}
      </nav>

      <button
        type="button"
        className="cf-rail__collapse"
        aria-label={collapsed ? t('nav.expandRail') : t('nav.collapseRail')}
        onClick={() => setCollapsed((value) => !value)}
      >
        <Icon icon={plMenuCollapse} size={16} className={collapsed ? 'cf-rail__collapse-icon--flip' : undefined} />
        <span className="cf-rail__label">{t('nav.collapseRail')}</span>
      </button>
    </aside>
  )
}
