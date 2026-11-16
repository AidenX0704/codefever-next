import { Link } from 'react-router'

import type { MergeRequestListItem } from '@/api/types'
import { AppLink } from '@/components/unit/AppLink'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Tag, type TagTone } from '@/components/unit/Tag'
import { arrowRightIcon, plMerge } from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { resolveMember } from '@/helpers/members'
import { useT } from '@/i18n'

import { MergeRequestReviewers } from '@/components/view/unit/MergeRequestReviewers'


export interface MergeRequestRowProps {
  item: MergeRequestListItem
  /** 详情页链接（`/{组}/{仓库}/mergerequests/{编号}`）。 */
  href: string
  /** 是否展示目标仓库前缀（全局/组内看板需要，仓库内列表不需要）。 */
  showRepository?: boolean
  /**
   * 紧凑变体：去掉"最后提交人 / 评审人头像组"，只留标题、状态、仓库、分支与更新时间。
   *
   * 用途是**首页工作台**（`view/home/MergeRequestPanel`）。那里的数据来自
   * `GET /api/home/overview`，刻意不带 `commit` 与 `reviewers`（否则 5 条待办要跑
   * 5 次 `git log`，见 docs/HOME_PAGE_PLAN_2026-10-27.md 的 D2）。
   * 不传 `commit` 时旧渲染会显示"更新于 1970-01-01"，所以必须有这个变体。
   */
  compact?: boolean
}

/** 状态徽标（v5 §11.2 词表）：开放=info、已合并=success、已关闭=default（idle）。 */
function statusTone(status: number): TagTone {
  if (status === 2) {
    return 'success'
  }
  if (status === 3) {
    return 'default'
  }
  return 'info'
}

/** 分支的提交列表链接：`/{组}/{仓库}/commits?rev={分支}`。 */
function commitsLink(groupName: string, repositoryName: string, branch: string): string {
  return `/${groupName}/${repositoryName}/commits?rev=${encodeURIComponent(branch)}`
}

/**
 * 合并请求行（仓库内列表 / 组内看板 / 全局看板共用）。
 *
 * 逐条对齐旧版 `components/unit/MergeRequestItem.js`：
 *   - 标题 + 状态徽标；
 *   - 目标仓库名（旧版显示真实名字；新版原实现显示主键前 8 位，已修正）；
 *   - `!编号`、源分支 → 目标分支（**可点**，跳该仓库该分支的提交列表）；
 *   - 最后提交人（源仓库成员 → 目标仓库成员 → 邮箱本身）与"编辑于"时间；
 *   - 评审人头像组（已/未评审角标）与"更新时间"。
 */
export function MergeRequestRow({ item, href, showRepository = false, compact = false }: MergeRequestRowProps) {
  const t = useT()
  const { repositories } = useAppData()

  const target = repositories.find((repo) => repo.id === item.targetRepository) ?? null
  const source = repositories.find((repo) => repo.id === item.sourceRepository) ?? null

  const statusLabel =
    item.status === 2 ? t('label.mrStatusMerged') : item.status === 3 ? t('label.mrStatusClosed') : t('label.mrStatusPending')

  // 目标仓库名：匹配不到（无权访问 / 已删除）时降级显示主键前 8 位。
  const targetName = target ? target.displayName || target.name : item.targetRepository.slice(0, 8)

  // 最后提交人：源仓库成员 → 目标仓库成员 → 提交邮箱本身（旧版 getUserInfo 的三级回退）。
  // compact 变体不展示这一段（且数据里没有 commit），直接跳过解析。
  const fromSource = compact ? null : resolveMember(source?.members, item.commit?.email)
  const updater = !fromSource
    ? ''
    : !fromSource.temporary
      ? fromSource.name
      : (() => {
          const fromTarget = resolveMember(target?.members, item.commit?.email)
          return fromTarget.temporary ? (item.commit?.email ?? '') : fromTarget.name
        })()

  const sourceHref = source ? commitsLink(source.group.name, source.name, item.sourceBranch) : null
  const targetHref = target ? commitsLink(target.group.name, target.name, item.targetBranch) : null

  return (
    <div className="cf-mr-row">
      {/* v5 §6 G8：行首 MR 方格图标（fg-3，不染状态色）。 */}
      <Icon icon={plMerge} size={16} className="cf-mr-row__icon" />

      <div className="cf-mr-row__main">
        <div className="cf-mr-row__title">
          <Link className="cf-mr-row__title-link" to={href}>
            {item.title}
          </Link>
        </div>
        <div className="cf-mr-row__meta">
          {showRepository ? <span className="cf-mr-row__path">{targetName}</span> : null}
          <span className="cf-mr-row__number">#{item.number}</span>
          {!compact && updater ? (
            <>
              <span className="cf-mr-row__author-avatar" aria-hidden="true">
                {updater.slice(0, 1).toUpperCase()}
              </span>
              <span className="cf-mr-row__author">{updater}</span>
            </>
          ) : null}
          <span className="cf-mr-row__chip">
            {sourceHref ? (
              <AppLink to={sourceHref} title={item.sourceBranch}>
                {item.sourceBranch}
              </AppLink>
            ) : (
              item.sourceBranch
            )}
          </span>
          <Icon icon={arrowRightIcon} size={12} className="cf-mr-row__arrow" />
          <span className="cf-mr-row__chip">
            {targetHref ? (
              <AppLink to={targetHref} title={item.targetBranch}>
                {item.targetBranch}
              </AppLink>
            ) : (
              item.targetBranch
            )}
          </span>
        </div>
      </div>

      {compact ? (
        <span className="cf-mr-row__time">
          <FormattedTime timestamp={Number(item.update)} />
        </span>
      ) : (
        <div className="cf-mr-row__side">
          <MergeRequestReviewers reviewers={item.reviewers ?? []} members={target?.members ?? []} />
          <Tag tone={statusTone(item.status)}>{statusLabel}</Tag>
          <span className="cf-mr-row__time">
            <FormattedTime timestamp={Number(item.update)} />
          </span>
        </div>
      )}
    </div>
  )
}
