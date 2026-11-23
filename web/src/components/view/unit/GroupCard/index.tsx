import { useNavigate } from 'react-router'

import { GROUP_TYPE } from '@/api/group'
import type { Group, RepositoryListItem } from '@/api/types'
import { arrowRightIcon, moreIcon, plMember, plRepository, plusIcon, settingsIcon } from '@/config/icons'
import { PermissionCode, Role, hasPermission } from '@/config/uac'
import { AvatarStack } from '@/components/unit/Avatar'
import { Dropdown, type MenuItemSpec } from '@/components/unit/Dropdown'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Marker } from '@/components/unit/Marker'
import { GroupAvatar } from '@/components/view/unit/GroupAvatar'
import { activateOnKey } from '@/helpers/keyboard'
import { useT } from '@/i18n'


export interface GroupCardProps {
  group: Group
  /** 该组下的仓库数（优先使用后端 `group.repositoryCount`）。 */
  repositoryCount: number
  /** 该组下近期活跃仓库预览（最多展示 3 个可直达芯片）。 */
  recentRepositories?: ReadonlyArray<RepositoryListItem>
}

/**
 * 仓库组空间门户卡片（全新重设计）。
 *
 * 将每个仓库组视为一个团队/业务空间：
 *   - 顶栏：2×2 方格标 + 组名 + `/slug` + 个人/团队标签 + 角色 + `⋯` 动作菜单；
 *   - 3 列指标内嵌条：仓库数 / 开放 MR / 成员数；
 *   - 近期仓库直达芯片：点击芯片直达代码页，点击卡片进入仓库组空间；
 *   - 底栏：有效成员头像叠放 + 更新时间 + 「进入空间 →」。
 */
export function GroupCard({ group, repositoryCount, recentRepositories = [] }: GroupCardProps) {
  const t = useT()
  const navigate = useNavigate()

  const aliveMembers = group.members.filter((member) => !member.deleteFlag)
  const memberCount = group.memberCount ?? aliveMembers.length
  const repoCount = group.repositoryCount ?? repositoryCount
  const openMRs = group.openMergeRequestCount ?? 0
  const canCreateRepo = hasPermission(group.role, PermissionCode.GROUP_CREATE_REPO)
  const canManage =
    hasPermission(group.role, PermissionCode.GROUP_CHANGE_MEMBER) || group.role === Role.OWNER

  const menuItems: MenuItemSpec[] = [
    ...(canCreateRepo
      ? [
          {
            key: 'new-repo',
            label: t('label.newRepository'),
            icon: <Icon icon={plusIcon} size={15} />,
            onClick: () => navigate(`/groups/${group.name}/repositories/new`),
          } as MenuItemSpec,
        ]
      : []),
    {
      key: 'members',
      label: t('menu.member_pl'),
      icon: <Icon icon={plMember} size={15} />,
      onClick: () => navigate(`/groups/${group.name}/members`),
      dividerAfter: canManage,
    },
    ...(canManage
      ? [
          {
            key: 'settings',
            label: t('group.settingsGeneral'),
            icon: <Icon icon={settingsIcon} size={15} />,
            onClick: () => navigate(`/groups/${group.name}/settings`),
          } as MenuItemSpec,
          {
            key: 'advanced',
            label: t('group.settingsAdvanced'),
            icon: <Icon icon={settingsIcon} size={15} />,
            onClick: () => navigate(`/groups/${group.name}/settings/advanced`),
          } as MenuItemSpec,
        ]
      : []),
  ]

  return (
    <div
      className="group-portal"
      onClick={() => navigate(`/groups/${group.name}`)}
      onKeyDown={activateOnKey(() => navigate(`/groups/${group.name}`))}
      role="button"
      tabIndex={0}
    >
      <div className="group-portal__head">
        <div className="group-portal__identity">
          <GroupAvatar icon={group.icon} name={group.displayName} size="md" />
          <div className="group-portal__meta">
            <div className="group-portal__title-row">
              <span className="group-portal__title" title={group.displayName}>
                {group.displayName}
              </span>
              <span className="repo-row__chip">
                {group.type === GROUP_TYPE.user ? t('group.personalTag') : t('group.teamTag')}
              </span>
            </div>
            <div className="group-portal__sub">
              <span className="group-card__slug">/{group.name}</span>
              <Marker
                variant={group.role === Role.OWNER ? 'containedInfo' : 'info'}
                text={t(`label.roleID_${group.role}`)}
              />
            </div>
          </div>
        </div>
        <div onClick={(event) => event.stopPropagation()}>
          <Dropdown
            align="right"
            minWidth={176}
            items={menuItems}
            trigger={
              <button type="button" className="cf-icon-btn" aria-label={t('label.moreActions')}>
                <Icon icon={moreIcon} size={17} />
              </button>
            }
          />
        </div>
      </div>

      <div className="group-portal__desc">{group.description || '—'}</div>

      <div className="group-portal__metrics">
        <div className="group-portal__metric">
          <span className="group-portal__metric-val">{repoCount}</span>
          <span className="group-portal__metric-lbl">{t('label.repository_pl')}</span>
        </div>
        <div className="group-portal__metric">
          <span className="group-portal__metric-val">{openMRs}</span>
          <span className="group-portal__metric-lbl">{t('label.mrStatusPending')}</span>
        </div>
        <div className="group-portal__metric">
          <span className="group-portal__metric-val">{memberCount}</span>
          <span className="group-portal__metric-lbl">{t('label.memberCount')}</span>
        </div>
      </div>

      <div className="group-portal__repos">
        <span className="group-portal__repos-lbl">{t('group.recentRepos')}</span>
        {recentRepositories.length > 0 ? (
          <div className="group-portal__repo-chips">
            {recentRepositories.slice(0, 3).map((repo) => (
              <button
                key={repo.id}
                type="button"
                className="group-portal__repo-chip"
                onClick={(event) => {
                  event.stopPropagation()
                  navigate(`/${group.name}/${repo.name}`)
                }}
              >
                <Icon icon={plRepository} size={12} />
                <span>{repo.displayName}</span>
              </button>
            ))}
          </div>
        ) : (
          <span className="group-portal__repos-empty">{t('group.noReposYet')}</span>
        )}
      </div>

      <div className="group-portal__foot">
        <div className="group-portal__foot-left">
          <AvatarStack items={aliveMembers} max={4} />
          <span className="group-card__updated">
            <FormattedTime timestamp={group.updated || group.created} />
          </span>
        </div>
        <span className="group-portal__enter">
          {t('group.enterSpace')}
          <Icon icon={arrowRightIcon} size={13} />
        </span>
      </div>
    </div>
  )
}
