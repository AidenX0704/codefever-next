import { useEffect, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'

import type { CurrentUser, RepositoryConfig, RepositoryListItem } from '@/api/types'
import { starRepository, unstarRepository } from '@/api/repository'
import { AppLink } from '@/components/unit/AppLink'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { CloneBox } from '@/components/unit/CloneBox'
import { Icon } from '@/components/unit/Icon'
import { useToast } from '@/components/unit/Toast'
import { forkIcon, starIcon } from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


export interface RepositoryHeaderProps {
  /** `repository/config` 的完整数据。 */
  config: RepositoryConfig
  /** 当前登录用户（取 host / ssh 拼克隆地址）。 */
  user: CurrentUser | null
  /** fork 来源仓库（全局仓库列表里按 `forkFrom` 找到的那一条）。 */
  forkSource?: RepositoryListItem | null
  /**
   * 页内 Tabs（v5 §7.4：Tabs 行贴在仓库页头纸页底部）。
   * 由 RepositoryLayout 传入，本组件只负责摆位，不感知路由。
   */
  children?: ReactNode
}

/**
 * 仓库页头部（DESIGN.md v5 §7.4 RepoHeader）。
 *
 * 三块内容自上而下：
 *   1. fork 来源行（仅 fork 仓库有）：`forked from 组/仓库`；
 *   2. 标题行：头像 + 可点击的组名 + 仓库名 + 描述（≤2 行）；
 *      右侧动作区：**Star 计数按钮**（fill 表已标星）、**Fork 按钮（含计数）**、
 *      **克隆按钮**（展开 CloneBox，SSH|HTTPS 分段）；
 *   3. Tabs 行：贴纸页底，由调用方以 children 传入。
 *
 * 旧版标题行下方的六格计数条（提交/分支/标签/体积/MR/成员）已移除：
 * 这些数字在各自的 Tab 与设置页都有自然落点，摆在头部只增加视觉噪音。
 */
export function RepositoryHeader({ config, user, forkSource, children }: RepositoryHeaderProps) {
  const t = useT()
  const navigate = useNavigate()
  const toast = useToast()
  const { updateRepositoryStar } = useAppData()
  const [cloneOpen, setCloneOpen] = useState(false)
  // 星标字段在 config **顶层**（与 list 的列表项不同层级，见 types.ts 注记）。
  const [starred, setStarred] = useState(Boolean(config.starred))
  const [starCount, setStarCount] = useState(config.starCount ?? 0)

  useEffect(() => {
    setStarred(Boolean(config.starred))
    setStarCount(config.starCount ?? 0)
  }, [config.repository.id, config.starred, config.starCount])

  const toggleStar = async () => {
    const next = !starred
    const nextCount = Math.max(0, starCount + (next ? 1 : -1))
    setStarred(next)
    setStarCount(nextCount)
    try {
      await (next ? starRepository(config.repository.id) : unstarRepository(config.repository.id))
      updateRepositoryStar(config.repository.id, next, nextCount)
    } catch {
      setStarred(!next)
      setStarCount(starCount)
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
          {forkSource ? (
            <div className="repo-head__fork">
              <Icon icon={forkIcon} size={14} />
              <AppLink to={`/${forkSource.group.name}/${forkSource.name}`}>
                {t('label.forkFrom_S', { s: `${forkSource.group.displayName}/${forkSource.displayName}` })}
              </AppLink>
            </div>
          ) : null}
          <h1 className="repo-head__title">
            <AppLink className="repo-head__group" to={`/groups/${config.group.name}`}>
              {config.group.displayName}
            </AppLink>
            <span className="repo-head__sep" aria-hidden="true">/</span>
            <span className="repo-head__name">{config.repository.displayName}</span>
          </h1>
          {config.repository.description ? <p className="repo-head__desc">{config.repository.description}</p> : null}
        </div>

        <div className="repo-head__actions">
          <button
            type="button"
            className={`repo-head__star${starred ? ' repo-head__star--on' : ''}`}
            aria-pressed={starred}
            title={t('star.star')}
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
            <span className="repo-head__count">{config.repository.forkCount}</span>
          </Button>
          <Button variant="contained" onClick={() => setCloneOpen((value) => !value)}>
            {t('label.clone')}
          </Button>
        </div>
      </div>

      {cloneOpen ? <CloneBox user={user} groupName={config.group.name} repositoryName={config.repository.name} /> : null}

      {children}
    </div>
  )
}
