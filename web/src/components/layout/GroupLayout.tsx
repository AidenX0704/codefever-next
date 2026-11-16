import { NavLink, Outlet, useLocation, useParams } from 'react-router'

import { avatarUrl } from '@/config/constants'
import { settingsIcon } from '@/config/icons'
import { PageLoading } from '@/components/unit/Loading'
import { Icon } from '@/components/unit/Icon'
import { GroupProvider, useGroup } from '@/features/group/GroupProvider'
import NotFound from '@/pages/NotFound'
import { useT } from '@/i18n'


/**
 * 仓库组外壳（v5 §6 G3：GroupHeader 纸页 + 页内下划线 Tabs）。
 *
 * 左侧上下文导航随 v5 退役；Tabs = 仓库｜合并请求｜成员｜动态｜设置
 * （原型的"概览"即仓库列表首屏；"子组"后端为扁平结构，不单列——数据边界注记）。
 */
function GroupFrame() {
  const { config, loading, notFound } = useGroup()
  const t = useT()

  if (notFound) {
    return <NotFound />
  }
  if (loading && !config) {
    return <PageLoading />
  }

  const group = config?.group
  const memberCount = config?.count.member ?? 0

  return (
    <div className="repo-page">
      {group ? (
        <header className="repo-head">
          <div className="repo-head__top">
            <GroupAvatar icon={group.icon} name={group.name} />
            <div className="repo-head__identity">
              <div className="repo-head__title">{group.displayName}</div>
              <div className="repo-head__desc">
                {group.description ? `${group.description} · ` : ''}
                {t('label.memberCount')} {memberCount}
              </div>
            </div>
            <div className="repo-head__actions">
              <NavLink className="cf-btn cf-btn--outlined" to={`/groups/${group.name}/repositories/new`}>
                {t('label.newRepository')}
              </NavLink>
              <NavLink
                className="cf-icon-btn"
                to={`/groups/${group.name}/settings`}
                aria-label={t('menu.setting_pl')}
                title={t('menu.setting_pl')}
              >
                <Icon icon={settingsIcon} size={16} />
              </NavLink>
            </div>
          </div>
          <GroupTabs groupName={group.name} />
        </header>
      ) : null}
      <Outlet />
    </div>
  )
}

/** 组头像 48：有图标用图，无图标用 2×2 方格标（v5 §7.2 组头像 = 中央 2×2 小方格图案）。 */
function GroupAvatar({ icon, name }: { icon: string | null; name: string }) {
  if (icon) {
    return <img className="repo-head__avatar" src={avatarUrl(icon)} alt={name} />
  }
  return (
    <span className="group-mark" aria-hidden="true">
      <i />
      <i />
      <i />
      <i />
    </span>
  )
}

/** v5 §6 G3：组内 Tabs，active 按路径前缀判定（index 重定向到 repositories）。 */
function GroupTabs({ groupName }: { groupName: string }) {
  const t = useT()
  const { pathname } = useLocation()
  const base = `/groups/${groupName}`

  const tabs = [
    { key: 'repositories', label: t('label.repository_pl'), to: base },
    { key: 'mergerequests', label: t('label.mergeRequest_pl'), to: `${base}/mergerequests` },
    { key: 'members', label: t('menu.member_pl'), to: `${base}/members` },
    { key: 'activities', label: t('menu.activity_pl'), to: `${base}/activities` },
    { key: 'settings', label: t('menu.setting_pl'), to: `${base}/settings` },
  ]

  return (
    <nav className="cf-tabs repo-tabs">
      {tabs.map((tab) => {
        const active = pathname === tab.to || pathname.startsWith(`${tab.to}/`)
        return (
          <NavLink
            key={tab.key}
            to={tab.to}
            className={active ? 'cf-tab cf-tab--active' : 'cf-tab'}
            aria-current={active ? 'page' : undefined}
          >
            {tab.label}
          </NavLink>
        )
      })}
    </nav>
  )
}

/**
 * 仓库组外壳（`/groups/{组名}/*`）。
 *
 * URL 与旧版一致：`/groups/{组名}/repositories`、`/members`、`/mergerequests`、
 * `/activities`、`/settings`；组名合法性由"全局列表里找不到即 404"承担。
 */
export function GroupLayout() {
  const { groupName } = useParams()

  return (
    <GroupProvider groupName={groupName}>
      <GroupFrame />
    </GroupProvider>
  )
}
