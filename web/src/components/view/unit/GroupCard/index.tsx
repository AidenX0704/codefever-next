import { useNavigate } from 'react-router'

import type { Group } from '@/api/types'
import { avatarUrl } from '@/config/constants'
import { plMember, plRepository, plSetting } from '@/config/icons'
import { Role } from '@/config/uac'
import { Icon } from '@/components/unit/Icon'
import { Marker } from '@/components/unit/Marker'
import { SquareIconButton } from '@/components/unit/SquareIconButton'
import { useT } from '@/i18n'


export interface GroupCardProps {
  group: Group
  /** 该组下的仓库数（旧版由全局 repositoryList 过滤统计得出）。 */
  repositoryCount: number
}

/**
 * 仓库组卡片。
 *
 * 视觉口径来自旧版 `components/unit/GroupCard.js`：图标 48px、标题 14px 粗体
 * （最大宽 128px 省略）、角色标记、描述行、仓库/成员计数、分割线、成员头像行。
 */
export function GroupCard({ group, repositoryCount }: GroupCardProps) {
  const t = useT()
  const navigate = useNavigate()

  return (
    <div className="group-card" onClick={() => navigate(`/groups/${group.name}`)} role="button" tabIndex={0}>
      <div className="group-card__head">
        <div className="group-card__identity">
          <span className="group-card__avatar">
            {group.icon ? <img src={avatarUrl(group.icon)} alt="" /> : group.name.slice(0, 1).toUpperCase()}
          </span>
          <div>
            <div className="group-card__title">{group.displayName}</div>
            <Marker variant={group.role === Role.OWNER ? 'containedInfo' : 'info'} text={t(`label.roleID_${group.role}`)} />
          </div>
        </div>
        <SquareIconButton
          icon={plSetting}
          label={t('label.setting')}
          onClick={(event) => {
            event.stopPropagation()
            navigate(`/groups/${group.name}/settings`)
          }}
        />
      </div>

      <div className="group-card__desc">{group.description || '...'}</div>

      <div className="group-card__stats">
        <Icon icon={plRepository} size={14} />
        {repositoryCount}
        <span style={{ display: 'inline-block', width: 'var(--cf-space)' }} />
        <Icon icon={plMember} size={14} />
        {group.members.length}
      </div>

      <div className="group-card__divider" />

      <div className="group-card__members">
        {group.members.slice(0, 8).map((member) => (
          <span className="group-card__member" key={member.id} title={member.name}>
            {member.icon ? <img src={avatarUrl(member.icon)} alt="" /> : member.name.slice(0, 1)}
          </span>
        ))}
      </div>
    </div>
  )
}
