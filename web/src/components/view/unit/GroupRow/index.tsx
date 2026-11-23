import { useNavigate } from 'react-router'

import { GROUP_TYPE } from '@/api/group'
import type { Group } from '@/api/types'
import { moreIcon, plMember, plMerge, plRepository, plusIcon, settingsIcon } from '@/config/icons'
import { PermissionCode, Role, hasPermission } from '@/config/uac'
import { AvatarStack } from '@/components/unit/Avatar'
import { Dropdown, type MenuItemSpec } from '@/components/unit/Dropdown'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Marker } from '@/components/unit/Marker'
import { GroupAvatar } from '@/components/view/unit/GroupAvatar'
import { activateOnKey } from '@/helpers/keyboard'
import { useT } from '@/i18n'


export interface GroupRowProps {
  group: Group
  repositoryCount: number
}

/**
 * 仓库组列表行（DESIGN.md v5 §6 G3 纸页行式列表）。
 */
export function GroupRow({ group, repositoryCount }: GroupRowProps) {
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
      className="repo-row"
      onClick={() => navigate(`/groups/${group.name}`)}
      onKeyDown={activateOnKey(() => navigate(`/groups/${group.name}`))}
      role="button"
      tabIndex={0}
    >
      <div className="repo-row__main">
        <GroupAvatar icon={group.icon} name={group.displayName} size="sm" />
        <div className="repo-row__info">
          <div className="repo-row__title">
            <span className="repo-row__name">{group.displayName}</span>
            <span className="repo-row__branch">/{group.name}</span>
            {group.type === GROUP_TYPE.user ? (
              <span className="repo-row__chip">{t('group.personalTag')}</span>
            ) : null}
            <span className="repo-row__role">
              <Marker
                variant={group.role === Role.OWNER ? 'containedInfo' : 'info'}
                text={t(`label.roleID_${group.role}`)}
              />
            </span>
          </div>
          {group.description ? <div className="repo-row__desc text-overflow">{group.description}</div> : null}
        </div>
      </div>

      <div className="repo-row__counts">
        <span className="repo-row__count" title={t('label.repository_pl')}>
          <Icon icon={plRepository} size={14} />
          {repoCount}
        </span>
        <span className="repo-row__count" title={t('label.mergeRequest_pl')}>
          <Icon icon={plMerge} size={14} />
          {openMRs}
        </span>
        <span className="repo-row__count" title={t('label.memberCount')}>
          <Icon icon={plMember} size={14} />
          {memberCount}
        </span>
        <AvatarStack items={aliveMembers} max={4} />
      </div>

      <div className="repo-row__updated">
        <FormattedTime timestamp={group.updated || group.created} />
      </div>

      <div className="repo-row__action" onClick={(event) => event.stopPropagation()}>
        <Dropdown
          align="right"
          minWidth={180}
          items={menuItems}
          trigger={
            <button
              type="button"
              className="cf-icon-btn"
              aria-label={t('label.moreActions')}
              title={t('label.moreActions')}
            >
              <Icon icon={moreIcon} size={18} />
            </button>
          }
        />
      </div>
    </div>
  )
}
