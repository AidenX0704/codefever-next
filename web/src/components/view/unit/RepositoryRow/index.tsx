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


export interface RepositoryRowProps {
  repository: RepositoryListItem
  /** 组内视图隐藏重复的组名前缀。 */
  hideGroupPrefix?: boolean
  onStarChange?: (repositoryId: string, starred: boolean, starCount: number) => void
}

/**
 * 仓库列表行（全新精炼工作台行）。
 *
 * - 支持一键复制克隆地址、标星切换与 `⋯` 快捷操作菜单；
 * - 组名可点击直达仓库组空间，默认分支/空仓库/派生标签清晰分区。
 */
export function RepositoryRow({ repository, hideGroupPrefix = false, onStarChange }: RepositoryRowProps) {
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

  const urls = cloneUrls(user, group.name, repository.name)
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
      key: 'star',
      label: starred ? t('star.unstar') : t('star.star'),
      icon: <Icon icon={starIcon} size={15} weight={starred ? 'fill' : 'regular'} />,
      onClick: () => void toggleStar(),
    },
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
      className="repo-row"
      onClick={() => navigate(`/${group.name}/${repository.name}`)}
      onKeyDown={activateOnKey(() => navigate(`/${group.name}/${repository.name}`))}
      role="button"
      tabIndex={0}
    >
      <div className="repo-row__main">
        <span className="repo-row__avatar">
          {repository.icon ? (
            <img src={avatarUrl(repository.icon)} alt="" loading="lazy" />
          ) : (
            repository.name.slice(0, 1).toUpperCase()
          )}
        </span>
        <div className="repo-row__info">
          <div className="repo-row__title">
            {!hideGroupPrefix ? (
              <>
                <button
                  type="button"
                  className="repo-row__group-btn"
                  onClick={(event) => {
                    event.stopPropagation()
                    navigate(`/groups/${group.name}`)
                  }}
                >
                  {group.displayName}
                </button>
                <span className="repo-row__sep" aria-hidden="true">/</span>
              </>
            ) : null}
            <span className="repo-row__name">{repository.displayName}</span>
            {repository.defaultBranch ? (
              <span className="repo-row__chip repo-row__chip--mono" title={t('label.branch')}>
                <Icon icon={psBranch} size={12} />
                {repository.defaultBranch}
              </span>
            ) : (
              <span className="repo-row__chip">{t('repo.emptyBranch')}</span>
            )}
            {repository.forkFrom ? (
              <span className="repo-row__chip" title={t('repo.forkBadge')}>
                <Icon icon={forkIcon} size={12} />
                {t('repo.forkBadge')}
              </span>
            ) : null}
            <span className="repo-row__role">
              <Marker
                variant={repository.role === Role.OWNER ? 'containedInfo' : 'info'}
                text={t(`label.roleID_${repository.role}`)}
              />
            </span>
          </div>
          {repository.description ? (
            <div className="repo-row__desc text-overflow">{repository.description}</div>
          ) : (
            <div className="repo-row__desc repo-row__desc--slug text-overflow">
              {group.name}/{repository.name}
            </div>
          )}
        </div>
      </div>

      <div className="repo-row__counts">
        <span className="repo-row__count" title={t('star.star')}>
          <Icon icon={starIcon} size={14} weight={starred ? 'fill' : 'regular'} />
          {starCount}
        </span>
        <span className="repo-row__count" title={t('label.fork')}>
          <Icon icon={forkIcon} size={14} />
          {repository.forkCount}
        </span>
        <span className="repo-row__count" title={t('label.mergeRequest_pl')}>
          <Icon icon={plMerge} size={14} />
          {repository.mergeRequestCount.open}
        </span>
        <span className="repo-row__count" title={t('label.memberCount')}>
          <Icon icon={plMember} size={14} />
          {activeMembers}
        </span>
      </div>

      <div className="repo-row__updated">
        <FormattedTime timestamp={repository.updated} />
      </div>

      <div className="repo-row__action" onClick={(event) => event.stopPropagation()}>
        <button
          type="button"
          className="cf-icon-btn"
          aria-label={t('repo.quickClone')}
          title={t('repo.copyHttps')}
          onClick={(event) => void copyCloneUrl(urls.https, event)}
        >
          <Icon icon={copyIcon} size={16} />
        </button>
        <button
          type="button"
          className={`cf-icon-btn repo-row__star${starred ? ' repo-row__star--on' : ''}`}
          aria-pressed={starred}
          aria-label={starred ? t('star.unstar') : t('star.star')}
          title={starred ? t('star.unstar') : t('star.star')}
          onClick={toggleStar}
        >
          <Icon icon={starIcon} size={17} weight={starred ? 'fill' : 'regular'} />
        </button>
        <Dropdown
          align="right"
          minWidth={196}
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
