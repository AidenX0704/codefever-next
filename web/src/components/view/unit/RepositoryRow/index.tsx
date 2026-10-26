import { useNavigate } from 'react-router'

import type { RepositoryListItem } from '@/api/types'
import { avatarUrl } from '@/config/constants'
import { plFork, plMember, plMerge, plSetting } from '@/config/icons'
import { Role } from '@/config/uac'
import { Icon } from '@/components/unit/Icon'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Marker } from '@/components/unit/Marker'
import { SquareIconButton } from '@/components/unit/SquareIconButton'
import { useT } from '@/i18n'


export interface RepositoryRowProps {
  repository: RepositoryListItem
}

/**
 * 仓库列表行。
 *
 * 视觉口径逐项来自旧版 `components/unit/RepositoryListItem.js`：
 * 行高 `spacing(9)`=72px、上下 padding 16px、左右 24px、上下 1px 边框、
 * 图标 40px、hover 底色 `bg-dark`、右侧三列计数（fork / 未合并 MR / 成员）、
 * 更新时间列在 md 以上显示、末尾设置按钮。
 */
export function RepositoryRow({ repository }: RepositoryRowProps) {
  const t = useT()
  const navigate = useNavigate()
  const group = repository.group

  return (
    <div className="repo-row" onClick={() => navigate(`/${group.name}/${repository.name}`)} role="button" tabIndex={0}>
      <div className="repo-row__main">
        <span className="repo-row__avatar">
          {repository.icon ? (
            <img src={avatarUrl(repository.icon)} alt="" />
          ) : (
            repository.name.slice(0, 1).toUpperCase()
          )}
        </span>
        <div className="repo-row__info">
          <div className="repo-row__title">
            {group.displayName}
            {'/'}
            <span className="repo-row__name">{repository.displayName}</span>
            <span className="repo-row__role">
              <Marker
                variant={repository.role === Role.OWNER ? 'containedInfo' : 'info'}
                text={t(`label.roleID_${repository.role}`)}
              />
            </span>
          </div>
          {repository.description ? <div className="repo-row__desc text-overflow">{repository.description}</div> : null}
        </div>
      </div>

      <div className="repo-row__counts">
        <span className="repo-row__count">
          <Icon icon={plFork} size={14} />
          {repository.forkCount}
        </span>
        <span className="repo-row__count">
          <Icon icon={plMerge} size={14} />
          {repository.mergeRequestCount.open}
        </span>
        <span className="repo-row__count">
          <Icon icon={plMember} size={14} />
          {repository.members.length}
        </span>
      </div>

      <div className="repo-row__updated">
        {t('label.updatedIn')}: <FormattedTime timestamp={repository.updated} />
      </div>

      <div className="repo-row__action">
        <SquareIconButton
          icon={plSetting}
          label={t('label.setting')}
          onClick={(event) => {
            event.stopPropagation()
            navigate(`/${group.name}/${repository.name}/settings`)
          }}
        />
      </div>
    </div>
  )
}
