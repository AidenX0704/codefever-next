import { useState } from 'react'
import { useNavigate } from 'react-router'

import type { RepositoryListItem } from '@/api/types'
import { starRepository, unstarRepository } from '@/api/repository'
import { avatarUrl } from '@/config/constants'
import { activateOnKey } from '@/helpers/keyboard'
import { forkIcon, moreIcon, plMember, plMerge, starIcon } from '@/config/icons'
import { Role } from '@/config/uac'
import { Icon } from '@/components/unit/Icon'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Marker } from '@/components/unit/Marker'
import { useToast } from '@/components/unit/Toast'
import { useT } from '@/i18n'


export interface RepositoryRowProps {
  repository: RepositoryListItem
}

/**
 * 仓库列表行。
 *
 * v4（DESIGN.md §7.3）：行末是 ⋯ 菜单（星标切换入口），计数列含星标数；
 * 其余口径沿用旧版（行高 72、上 16 下 24 padding、1px 分割线）。
 * 星标失败不改变行内状态（乐观更新 + 失败回滚 + Toast）。
 */
export function RepositoryRow({ repository }: RepositoryRowProps) {
  const t = useT()
  const navigate = useNavigate()
  const toast = useToast()
  const [starred, setStarred] = useState(Boolean(repository.starred))
  const [starCount, setStarCount] = useState(repository.starCount ?? 0)
  const group = repository.group

  const toggleStar = async (event: React.MouseEvent) => {
    event.stopPropagation()
    const next = !starred
    setStarred(next)
    setStarCount((count) => count + (next ? 1 : -1))
    try {
      await (next ? starRepository(repository.id) : unstarRepository(repository.id))
    } catch {
      setStarred(!next)
      setStarCount((count) => count + (next ? -1 : 1))
      toast.error(t('star.failed'))
    }
  }

  return (
    <div
      className="repo-row"
      onClick={() => navigate(`/${group.name}/${repository.name}`)}
      onKeyDown={activateOnKey(() => navigate(`/${group.name}/${repository.name}`))}
      role="button"
      tabIndex={0}
    >
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
            {/* v5 §6 G2：组路径 fg-3 常规字重，仓库名才是主列重点。 */}
            <span className="repo-row__group">{group.displayName}</span>
            {'/'}
            <span className="repo-row__name">{repository.displayName}</span>
            {/* 默认分支：V3 §7.3 要求仓库行能看到默认分支。
                刻意做成等宽的普通文字而不是彩色胶囊 —— §6.3 明确
                「不要把分支名、SHA、角色、流水线状态都做成同款彩色胶囊」，
                否则一行里三个色块，谁是状态谁是标识就分不清了。 */}
            {repository.defaultBranch ? (
              <span className="repo-row__branch" title={t('label.branch')}>
                {repository.defaultBranch}
              </span>
            ) : null}
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
          <Icon icon={starIcon} size={14} weight={starred ? 'fill' : 'regular'} />
          {starCount}
        </span>
        <span className="repo-row__count">
          <Icon icon={forkIcon} size={14} />
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
        <button
          type="button"
          className={`cf-icon-btn repo-row__star${starred ? ' repo-row__star--on' : ''}`}
          aria-pressed={starred}
          aria-label={starred ? t('star.unstar') : t('star.star')}
          title={starred ? t('star.unstar') : t('star.star')}
          onClick={toggleStar}
        >
          <Icon icon={starIcon} size={18} weight={starred ? 'fill' : 'regular'} />
        </button>
        <button
          type="button"
          className="cf-icon-btn"
          aria-label={t('label.moreActions')}
          title={t('label.moreActions')}
          onClick={(event) => {
            event.stopPropagation()
            navigate(`/${group.name}/${repository.name}/settings`)
          }}
        >
          <Icon icon={moreIcon} size={18} />
        </button>
      </div>
    </div>
  )
}
