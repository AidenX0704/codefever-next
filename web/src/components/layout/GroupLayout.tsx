import { NavLink, Outlet, useLocation, useNavigate, useParams } from 'react-router'

import { GROUP_TYPE } from '@/api/group'
import { AvatarStack } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { Icon } from '@/components/unit/Icon'
import { PageLoading } from '@/components/unit/Loading'
import { Marker } from '@/components/unit/Marker'
import { GroupAvatar } from '@/components/view/unit/GroupAvatar'
import {
  arrowLeftIcon,
  plMember,
  plMerge,
  plRepository,
  plusIcon,
  psClock,
  settingsIcon,
  shieldCheckIcon,
} from '@/config/icons'
import { PermissionCode, Role, hasPermission } from '@/config/uac'
import { GroupProvider, useGroup } from '@/features/group/GroupProvider'
import { useT } from '@/i18n'
import NotFound from '@/pages/NotFound'


/**
 * 仓库组双栏空间布局（全新重设计）。
 *
 * 抛弃旧版"顶部通栏横幅 + 堆叠 Tabs"的挤压结构，改为：
 *   - 左侧 280px 粘性空间侧栏（`.group-rail`）：组身份、3 项核心读数、新建仓库主操作、
 *     纵向空间导航（直接平铺仓库/MR/成员/动态 + 常规设置/高级设置）与核心成员头像叠放；
 *   - 右侧独立宽敞主舞台（`.group-stage`）：承载子路由内容，无双重页头冲突；
 *   - 窄屏（<1024px）自动平滑切换为单列顶栏 + 主舞台。
 */
function GroupFrame() {
  const { summary, config, loading, notFound } = useGroup()
  const t = useT()
  const navigate = useNavigate()
  const { pathname } = useLocation()

  if (notFound) {
    return <NotFound />
  }
  if (loading && !config) {
    return <PageLoading />
  }

  const group = config?.group
  if (!group) {
    return <Outlet />
  }

  const role = summary?.role ?? Role.NO_PERMISSION
  const canCreateRepo = hasPermission(role, PermissionCode.GROUP_CREATE_REPO)
  const canManage = hasPermission(role, PermissionCode.GROUP_CHANGE_MEMBER) || role === Role.OWNER

  const aliveMembers = (config?.members ?? []).filter((m) => !m.deleteFlag)
  const memberCount = config?.count.member ?? summary?.memberCount ?? aliveMembers.length
  const repoCount = config?.count.repository ?? summary?.repositoryCount ?? 0
  const openMRCount = config?.count.openMergeRequest ?? summary?.openMergeRequestCount ?? 0

  const base = `/groups/${group.name}`
  const isRepos =
    pathname === base || pathname === `${base}/repositories` || pathname.startsWith(`${base}/repositories/`)
  const isMRs = pathname === `${base}/mergerequests` || pathname.startsWith(`${base}/mergerequests/`)
  const isMembers = pathname === `${base}/members` || pathname.startsWith(`${base}/members/`)
  const isActivities = pathname === `${base}/activities` || pathname.startsWith(`${base}/activities/`)
  const isAdvanced = pathname === `${base}/settings/advanced`
  const isGeneral =
    pathname === `${base}/settings` || pathname === `${base}/settings/general`

  const primaryNav = [
    { key: 'repos', label: t('label.repository_pl'), to: `${base}/repositories`, icon: plRepository, count: repoCount, active: isRepos },
    { key: 'mrs', label: t('label.mergeRequest_pl'), to: `${base}/mergerequests`, icon: plMerge, count: openMRCount, active: isMRs },
    { key: 'members', label: t('menu.member_pl'), to: `${base}/members`, icon: plMember, count: memberCount, active: isMembers },
    { key: 'activities', label: t('menu.activity_pl'), to: `${base}/activities`, icon: psClock, active: isActivities },
  ]

  return (
    <div className="page">
      <div className="group-workspace">
        <aside className="group-rail">
          <button
            type="button"
            className="group-rail__back"
            onClick={() => navigate('/groups')}
          >
            <Icon icon={arrowLeftIcon} size={14} />
            <span>{t('label.group')}</span>
          </button>

          <div className="group-rail__profile">
            <GroupAvatar icon={group.icon} name={group.displayName} size="lg" />
            <div className="group-rail__name-block">
              <h1 className="group-rail__title" title={group.displayName}>
                {group.displayName}
              </h1>
              <div className="group-rail__slug">/{group.name}</div>
            </div>
          </div>

          <div className="group-rail__badges">
            <span className="repo-row__chip">
              {group.type === GROUP_TYPE.user ? t('group.personalTag') : t('group.teamTag')}
            </span>
            <Marker variant={role === Role.OWNER ? 'containedInfo' : 'info'} text={t(`label.roleID_${role}`)} />
          </div>

          {group.description ? <p className="group-rail__desc">{group.description}</p> : null}

          <div className="group-portal__metrics">
            <div className="group-portal__metric">
              <span className="group-portal__metric-val">{repoCount}</span>
              <span className="group-portal__metric-lbl">{t('label.repository_pl')}</span>
            </div>
            <div className="group-portal__metric">
              <span className="group-portal__metric-val">{openMRCount}</span>
              <span className="group-portal__metric-lbl">{t('label.mrStatusPending')}</span>
            </div>
            <div className="group-portal__metric">
              <span className="group-portal__metric-val">{memberCount}</span>
              <span className="group-portal__metric-lbl">{t('label.memberCount')}</span>
            </div>
          </div>

          {canCreateRepo ? (
            <Button
              variant="contained"
              block
              startIcon={<Icon icon={plusIcon} size={15} />}
              onClick={() => navigate(`${base}/repositories/new`)}
            >
              {t('label.newRepository')}
            </Button>
          ) : null}

          <nav className="group-rail__nav" aria-label={t('group.spaceNavigation')}>
            <div className="group-rail__section-lbl">{t('group.spaceNavigation')}</div>
            {primaryNav.map((item) => (
              <NavLink
                key={item.key}
                to={item.to}
                className={item.active ? 'group-rail__link group-rail__link--active' : 'group-rail__link'}
                aria-current={item.active ? 'page' : undefined}
              >
                <Icon icon={item.icon} size={16} />
                <span className="group-rail__link-label">{item.label}</span>
                {item.count !== undefined ? <span className="group-rail__link-count">{item.count}</span> : null}
              </NavLink>
            ))}

            {canManage ? (
              <>
                <div className="group-rail__divider" />
                <div className="group-rail__section-lbl">{t('group.spaceSettings')}</div>
                <NavLink
                  to={`${base}/settings/general`}
                  className={isGeneral ? 'group-rail__link group-rail__link--active' : 'group-rail__link'}
                  aria-current={isGeneral ? 'page' : undefined}
                >
                  <Icon icon={settingsIcon} size={16} />
                  <span className="group-rail__link-label">{t('group.settingsGeneral')}</span>
                </NavLink>
                <NavLink
                  to={`${base}/settings/advanced`}
                  className={isAdvanced ? 'group-rail__link group-rail__link--active' : 'group-rail__link'}
                  aria-current={isAdvanced ? 'page' : undefined}
                >
                  <Icon icon={shieldCheckIcon} size={16} />
                  <span className="group-rail__link-label">{t('group.settingsAdvanced')}</span>
                </NavLink>
              </>
            ) : null}
          </nav>

          {aliveMembers.length > 0 ? (
            <div className="group-rail__members">
              <div className="group-rail__members-head">
                <span>{t('group.membersPreview')}</span>
                <NavLink className="group-rail__members-all" to={`${base}/members`}>
                  {t('group.viewAllMembers')}
                </NavLink>
              </div>
              <AvatarStack items={aliveMembers} max={6} />
            </div>
          ) : null}
        </aside>

        <section className="group-stage">
          <Outlet />
        </section>
      </div>
    </div>
  )
}

export function GroupLayout() {
  const { groupName } = useParams()

  return (
    <GroupProvider groupName={groupName}>
      <GroupFrame />
    </GroupProvider>
  )
}
