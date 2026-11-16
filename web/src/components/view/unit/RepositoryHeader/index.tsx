import { useState } from 'react'
import { useNavigate } from 'react-router'

import type { CurrentUser, RepositoryConfig, RepositoryListItem } from '@/api/types'
import { starRepository, unstarRepository } from '@/api/repository'
import { AppLink } from '@/components/unit/AppLink'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { CloneBox } from '@/components/unit/CloneBox'
import { Icon } from '@/components/unit/Icon'
import { Tag } from '@/components/unit/Tag'
import { useToast } from '@/components/unit/Toast'
import { forkIcon, plMerge, psBranch, psCommit, psFile, psMembers, psTag, starIcon } from '@/config/icons'
import { formatBytes } from '@/helpers/format'
import { useT } from '@/i18n'


export interface RepositoryHeaderProps {
  /** `repository/config` 的完整数据。 */
  config: RepositoryConfig
  /** 当前登录用户（取 host / ssh 拼克隆地址）。 */
  user: CurrentUser | null
  /** fork 来源仓库（全局仓库列表里按 `forkFrom` 找到的那一条）。 */
  forkSource?: RepositoryListItem | null
}

/**
 * 仓库页头部（DESIGN.md §7.5 二级头部）。
 *
 * 三块内容：
 *   1. 身份区：头像 + 组名/显示名 + 描述 + **fork 来源链接**；
 *   2. 操作区：**Star 计数按钮**（fill 表已标星）、**Fork 按钮 + fork 数**、
 *      **克隆按钮**（展开 CloneBox，SSH|HTTPS 分段）；
 *   3. 计数条：提交 / 分支 / 标签 / 仓库体积 / 未合并 MR / 成员数。
 *
 * 数据全部来自 `repository/config`（v4 起 config 含 starCount / starred）。
 */
export function RepositoryHeader({ config, user, forkSource }: RepositoryHeaderProps) {
  const t = useT()
  const navigate = useNavigate()
  const toast = useToast()
  const [cloneOpen, setCloneOpen] = useState(false)
  // 星标字段在 config **顶层**（与 list 的列表项不同层级，见 types.ts 注记）。
  const [starred, setStarred] = useState(Boolean(config.starred))
  const [starCount, setStarCount] = useState(config.starCount ?? 0)

  const toggleStar = async () => {
    const next = !starred
    setStarred(next)
    setStarCount((count) => count + (next ? 1 : -1))
    try {
      await (next ? starRepository(config.repository.id) : unstarRepository(config.repository.id))
    } catch {
      setStarred(!next)
      setStarCount((count) => count + (next ? -1 : 1))
      toast.error(t('star.failed'))
    }
  }

  return (
    <div className="repo-head">
      <div className="repo-head__top">
        <Avatar
          square
          size="lg"
          src={config.repository.icon ? `/file/fetch/avatar/${config.repository.icon}` : undefined}
          name={config.repository.name}
        />
        <div className="repo-head__identity">
          <div className="repo-head__title">
            <span className="repo-head__group">{config.group.displayName}/</span>
            {config.repository.displayName}
            {forkSource ? (
              <span className="repo-head__fork">
                <AppLink to={`/${forkSource.group.name}/${forkSource.name}`}>
                  {t('label.forkFrom_S', { s: `${forkSource.group.displayName}/${forkSource.displayName}` })}
                </AppLink>
              </span>
            ) : null}
          </div>
          {config.repository.description ? <div className="repo-head__desc">{config.repository.description}</div> : null}
        </div>

        <div className="repo-head__actions">
          <button
            type="button"
            className={`repo-head__star${starred ? ' repo-head__star--on' : ''}`}
            aria-pressed={starred}
            onClick={toggleStar}
          >
            <Icon icon={starIcon} size={16} weight={starred ? 'fill' : 'regular'} />
            <span className="repo-head__star-count">{starCount}</span>
          </button>
          <Button
            variant="outlined"
            startIcon={<Icon icon={forkIcon} size={16} />}
            onClick={() => navigate(`/repositories/fork/${config.repository.id}`)}
          >
            {t('label.fork')}
          </Button>
          <Tag tone="primary" title={t('label.fork')}>
            {config.repository.forkCount}
          </Tag>
          <Button variant="contained" onClick={() => setCloneOpen((value) => !value)}>
            {t('label.clone')}
          </Button>
        </div>
      </div>

      {cloneOpen ? <CloneBox user={user} groupName={config.group.name} repositoryName={config.repository.name} /> : null}

      <div className="repo-head__stats">
        <Stat icon={psCommit} label={t('label.commit')} value={config.count.commit} />
        <Stat icon={psBranch} label={t('label.branch')} value={config.count.branch} />
        <Stat icon={psTag} label={t('label.tag')} value={config.count.tag} />
        <Stat icon={psFile} label={t('label.repoSize')} value={formatBytes(config.count.file)} />
        <Stat icon={plMerge} label={t('label.mergeRequestCount')} value={config.repository.mergeRequestCount.open} />
        <Stat icon={psMembers} label={t('label.memberCount')} value={config.members.length} />
      </div>
    </div>
  )
}

function Stat({ icon, label, value }: { icon: Parameters<typeof Icon>[0]['icon']; label: string; value: number | string }) {
  return (
    <span className="repo-stat">
      <Icon icon={icon} size={16} />
      <span className="repo-stat__value">{value}</span>
      <span className="repo-stat__label">{label}</span>
    </span>
  )
}
