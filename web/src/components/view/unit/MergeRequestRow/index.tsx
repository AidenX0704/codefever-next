import { Link } from 'react-router'

import type { MergeRequestListItem } from '@/api/types'
import { AppLink } from '@/components/unit/AppLink'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Tag, type TagTone } from '@/components/unit/Tag'
import { plMerge } from '@/config/icons'
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

/** 状态徽标配色：旧版 open=success、merged=info、closed=warning。 */
function statusTone(status: number): TagTone {
  if (status === 2) {
    return 'info'
  }
  if (status === 3) {
    return 'warning'
  }
  return 'success'
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
      <div className="cf-mr-row__main">
        <div className="cf-mr-row__title">
          <Link className="cf-link" to={href}>
            #{item.number} {item.title}
          </Link>
          <Tag tone={statusTone(item.status)}>{statusLabel}</Tag>
        </div>
        <div className="cf-mr-row__meta">
          {showRepository ? <span>{targetName}</span> : null}
          <span>!{item.number}</span>
          <span aria-hidden="true">·</span>
          <span className="cf-mr-row__merge">
            <Icon icon={plMerge} size={14} />
            {sourceHref ? <AppLink to={sourceHref}>{item.sourceBranch}</AppLink> : <span>{item.sourceBranch}</span>}
            <span aria-hidden="true">→</span>
            {targetHref ? <AppLink to={targetHref}>{item.targetBranch}</AppLink> : <span>{item.targetBranch}</span>}
          </span>
          <span aria-hidden="true">·</span>
          {compact ? (
            // 紧凑变体：首页工作台拿不到 commit（后端刻意不跑 git），
            // 因此这里显示 MR 自身的更新时间，而不是"最后提交人 + 编辑于"。
            <span>
              {t('label.updateTime')}: <FormattedTime timestamp={Number(item.update)} />
            </span>
          ) : (
            <span>
              {updater} {t('label.editIn')}: <FormattedTime timestamp={Number(item.commit?.time ?? 0)} />
            </span>
          )}
        </div>
      </div>
      {compact ? null : (
        <div className="cf-mr-row__side">
          <MergeRequestReviewers reviewers={item.reviewers ?? []} members={target?.members ?? []} />
          <span>
            {t('label.updateTime')}: <FormattedTime timestamp={Number(item.update)} />
          </span>
        </div>
      )}
    </div>
  )
}
