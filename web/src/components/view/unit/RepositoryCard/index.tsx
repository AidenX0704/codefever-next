import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'

import type { RepositoryListItem } from '@/api/types'
import { starRepository, unstarRepository } from '@/api/repository'
import { avatarUrl } from '@/config/constants'
import { copyIcon, forkIcon, moreIcon, plMember, plMerge, psBranch, settingsIcon, starIcon } from '@/config/icons'
import { PermissionCode, Role, hasPermission } from '@/config/uac'
import { Dropdown, type MenuItemSpec } from '@/components/unit/Dropdown'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Marker } from '@/components/unit/Marker'
import { useToast } from '@/components/unit/Toast'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { cloneUrls } from '@/helpers/cloneUrl'
import { activateOnKey } from '@/helpers/keyboard'
import { useT } from '@/i18n'


export interface RepositoryCardProps {
  repository: RepositoryListItem
  onStarChange?: (repositoryId: string, starred: boolean, starCount: number) => void
}

/**
 * 仓库卡片视图单元（与 `RepositoryRow` 共享全部交互能力：一键复制克隆、标星、⋯ 快捷菜单）。
 */
export function RepositoryCard({ repository, onStarChange }: RepositoryCardProps) {
  const t = useT()
  const navigate = useNavigate()
  const toast = useToast()
  const { user, updateRepositoryStar } = useAppData()
  const [starred, setStarred] = useState(Boolean(repository.starred))
  const [starCount, setStarCount] = useState(repository.starCount ?? 0)
  const group = repository.group

  useEffect(() => {
    setStarred(Boolean(repository.starred))
    setStarCount(repository.starCount ?? 0)
  }, [repository.starred, repository.starCount])

  const activeMembers =
    repository.memberCount ?? repository.members.filter((member) => !member.deleteFlag).length
  const urls = cloneUrls(user, group.name, repository.name)

  const toggleStar = async (event?: React.MouseEvent) => {
    event?.stopPropagation()
    const next = !starred
    const nextCount = Math.max(0, starCount + (next ? 1 : -1))
    setStarred(next)
    setStarCount(nextCount)
    try {
      await (next ? starRepository(repository.id) : unstarRepository(repository.id))
      updateRepositoryStar(repository.id, next, nextCount)
      onStarChange?.(repository.id, next, nextCount)
    } catch {
      setStarred(!next)
      setStarCount(starCount)
      toast.error(t('star.failed'))
    }
  }

  const copyCloneUrl = async (url: string, event?: React.MouseEvent) => {
    event?.stopPropagation()
    try {
      await navigator.clipboard.writeText(url)
      toast.success(t('repo.copiedUrl'))
    } catch {
      toast.error(t('base_error_message'))
    }
  }

  const canEditInfo =
    hasPermission(repository.role, PermissionCode.REPO_CHANGE_INFO) ||
    hasPermission(repository.role, PermissionCode.REPO_CHANGE_MEMBER)
  const canRemove = hasPermission(repository.role, PermissionCode.REPO_REMOVE)

  const menuItems: MenuItemSpec[] = [
    {
      key: 'copy-https',
      label: t('repo.copyHttps'),
      icon: <Icon icon={copyIcon} size={15} />,
      onClick: () => void copyCloneUrl(urls.https),
    },
    ...(urls.ssh
      ? [
          {
            key: 'copy-ssh',
            label: t('repo.copySsh'),
            icon: <Icon icon={copyIcon} size={15} />,
            onClick: () => void copyCloneUrl(urls.ssh),
            dividerAfter: true,
          } as MenuItemSpec,
        ]
      : []),
    {
      key: 'fork',
      label: t('label.fork'),
      icon: <Icon icon={forkIcon} size={15} />,
      onClick: () => navigate(`/repositories/fork/${repository.id}`),
      dividerAfter: canEditInfo || canRemove,
    },
    ...(canEditInfo
      ? [
          {
            key: 'settings',
            label: t('menu.setting_pl'),
            icon: <Icon icon={settingsIcon} size={15} />,
            onClick: () => navigate(`/${group.name}/${repository.name}/settings`),
          } as MenuItemSpec,
        ]
      : []),
    ...(canRemove
      ? [
          {
            key: 'advanced',
            label: t('label.advanced'),
            icon: <Icon icon={settingsIcon} size={15} />,
            onClick: () => navigate(`/${group.name}/${repository.name}/settings/advanced`),
          } as MenuItemSpec,
        ]
      : []),
  ]

  return (
    <div
      className="repo-card"
      onClick={() => navigate(`/${group.name}/${repository.name}`)}
      onKeyDown={activateOnKey(() => navigate(`/${group.name}/${repository.name}`))}
      role="button"
      tabIndex={0}
    >
      <div className="repo-card__head">
        <div className="repo-card__identity">
          <span className="repo-row__avatar">
            {repository.icon ? (
              <img src={avatarUrl(repository.icon)} alt="" loading="lazy" />
            ) : (
              repository.name.slice(0, 1).toUpperCase()
            )}
          </span>
          <div className="repo-card__titles">
            <button
              type="button"
              className="repo-card__group-link"
              onClick={(event) => {
                event.stopPropagation()
                navigate(`/groups/${group.name}`)
              }}
            >
              {group.displayName}
            </button>
            <div className="repo-card__name" title={repository.displayName}>
              {repository.displayName}
            </div>
          </div>
        </div>
        <div className="repo-card__actions" onClick={(event) => event.stopPropagation()}>
          <button
            type="button"
            className={`cf-icon-btn repo-row__star${starred ? ' repo-row__star--on' : ''}`}
            aria-pressed={starred}
            aria-label={starred ? t('star.unstar') : t('star.star')}
            title={starred ? t('star.unstar') : t('star.star')}
            onClick={toggleStar}
          >
            <Icon icon={starIcon} size={16} weight={starred ? 'fill' : 'regular'} />
          </button>
          <Dropdown
            align="right"
            minWidth={192}
            items={menuItems}
            trigger={
              <button type="button" className="cf-icon-btn" aria-label={t('label.moreActions')}>
                <Icon icon={moreIcon} size={16} />
              </button>
            }
          />
        </div>
      </div>

      <div className="repo-card__desc">{repository.description || '—'}</div>

      <div className="repo-card__tags">
        {repository.defaultBranch ? (
          <span className="repo-row__chip repo-row__chip--mono">
            <Icon icon={psBranch} size={12} />
            {repository.defaultBranch}
          </span>
        ) : (
          <span className="repo-row__chip">{t('repo.emptyBranch')}</span>
        )}
        {repository.forkFrom ? (
          <span className="repo-row__chip">
            <Icon icon={forkIcon} size={12} />
            {t('repo.forkBadge')}
          </span>
        ) : null}
        <Marker
          variant={repository.role === Role.OWNER ? 'containedInfo' : 'info'}
          text={t(`label.roleID_${repository.role}`)}
        />
      </div>

      <div className="repo-card__foot">
        <div className="repo-card__metrics">
          <span className="repo-row__count" title={t('star.star')}>
            <Icon icon={starIcon} size={13} weight={starred ? 'fill' : 'regular'} />
            {starCount}
          </span>
          <span className="repo-row__count" title={t('label.fork')}>
            <Icon icon={forkIcon} size={13} />
            {repository.forkCount}
          </span>
          <span className="repo-row__count" title={t('label.mergeRequest_pl')}>
            <Icon icon={plMerge} size={13} />
            {repository.mergeRequestCount.open}
          </span>
          <span className="repo-row__count" title={t('label.memberCount')}>
            <Icon icon={plMember} size={13} />
            {activeMembers}
          </span>
        </div>
        <span className="repo-card__time">
          <FormattedTime timestamp={repository.updated} />
        </span>
      </div>
    </div>
  )
}
