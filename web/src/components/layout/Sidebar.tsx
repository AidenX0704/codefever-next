import { useEffect, useState } from 'react'
import { NavLink, useLocation } from 'react-router'

import type { DrawerEntry } from '@/config/drawer'
import { plMenuCollapse } from '@/config/icons'
import { Icon } from '@/components/unit/Icon'
import { useT } from '@/i18n'


const COLLAPSE_STORAGE_KEY = 'codefever.drawer.collapsed'

/**
 * 左侧导航。
 *
 * 视觉/行为口径来自旧版 `components/Drawer.js` + `components/DrawerList.js`：
 *   - 永久展开（`variant='permanent'`），展开宽 320px(spacing 40)、收起宽 88px(spacing 11)；
 *   - 右边缘中部有一个圆形折叠按钮（距顶 96px、跨出 12px）；
 *   - 组底色 `bg-main`，与内容区白底形成分界，中间有 1px 边框。
 *
 * 收起状态记在 localStorage：旧版是 Redux `DrawerStates`（刷新即丢），
 * 这里顺手做成持久化，行为更符合直觉（记录在计划 §10 批 3 的差异里）。
 */
export function Sidebar({ entries }: { entries: DrawerEntry[] }) {
  const t = useT()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState<boolean>(() => {
    try {
      return window.localStorage.getItem(COLLAPSE_STORAGE_KEY) === '1'
    } catch {
      return false
    }
  })

  useEffect(() => {
    try {
      window.localStorage.setItem(COLLAPSE_STORAGE_KEY, collapsed ? '1' : '0')
    } catch {
      /* localStorage 不可用时忽略：只影响下次是否记住折叠状态 */
    }
  }, [collapsed])

  return (
    <aside className={collapsed ? 'cf-sidebar cf-sidebar--collapsed' : 'cf-sidebar'}>
      <button
        type="button"
        className="cf-sidebar__collapse"
        aria-label={collapsed ? 'expand' : 'collapse'}
        onClick={() => setCollapsed((value) => !value)}
      >
        <Icon icon={plMenuCollapse} size={14} className={collapsed ? 'cf-sidebar__collapse-icon--flip' : undefined} />
      </button>

      <nav className="cf-sidebar__nav">
        {entries.map((entry) => {
          const active = entry.active(location.pathname)
          return (
            <div key={entry.path}>
              <NavLink to={entry.path} className={active ? 'cf-sidebar__item cf-sidebar__item--active' : 'cf-sidebar__item'}>
                <Icon icon={entry.icon} size={18} />
                <span className="cf-sidebar__label">{t(entry.labelKey)}</span>
                {entry.count !== undefined ? <span className="cf-sidebar__count">{entry.count}</span> : null}
              </NavLink>
              {active && entry.children?.length ? (
                <div className="cf-sidebar__children">
                  {entry.children.map((child) => (
                    <NavLink
                      key={child.path}
                      to={child.path}
                      className={child.active(location.pathname) ? 'cf-sidebar__child cf-sidebar__child--active' : 'cf-sidebar__child'}
                    >
                      {t(child.labelKey)}
                    </NavLink>
                  ))}
                </div>
              ) : null}
            </div>
          )
        })}
      </nav>
    </aside>
  )
}
