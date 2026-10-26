import { Fragment, useMemo, useState } from 'react'

import type { Group, RepositoryListItem } from '@/api/types'
import type { NotificationRefuse } from '@/api/user'
import { Switch } from '@/components/unit/Choices'
import { Icon } from '@/components/unit/Icon'
import { IconButton } from '@/components/unit/IconButton'
import { Marker } from '@/components/unit/Marker'
import { Table, TableWrap, TBody, TD, TH, THead, TR } from '@/components/unit/Table'
import { psCaretDown } from '@/config/icons'
import { Role } from '@/config/uac'
import { useT } from '@/i18n'

export interface SettingsNotificationMatrixProps {
  /** 当前用户可见的仓库组。 */
  groups: Group[]
  /** 当前用户可见的仓库。 */
  repositories: RepositoryListItem[]
  /** 被拒收通知的目标（`user/notificationRefused`）：命中即视为该目标已关闭通知。 */
  refused: NotificationRefuse[]
  /** 切换组 / 仓库的通知开关；`open` 是**切换之后**的目标状态。 */
  onToggle: (type: 1 | 2, target: string, open: boolean) => void
  /** 请求进行中：禁用全部开关。 */
  pending?: boolean
}

/**
 * 仓库组 / 仓库的通知开关矩阵。
 *
 * 对应旧版 `UserSettingNotification.js`：
 *   - 组的开关下面是它名下的仓库（可折叠，旧版有展开箭头）；
 *   - 不属于任何可见仓库组的仓库单独一张表（旧版 `otherRepository` 分支）；
 *   - 每行显示当前用户在该组 / 仓库里的角色徽标（OWNER 用 containedInfo，其余用 info）；
 *   - 角色低于 DEVELOPER 的条目是只读的，开关禁用。
 *
 * 状态语义：`user/notificationRefused` 命中的目标即"已关闭"，其余为"已开启"；
 * 组的开关额外沿用旧版口径 —— 组内仓库全部关闭（或当前用户在其仓库内角色不足
 * DEVELOPER）时，组也显示为关闭（旧版 UserSettingNotification.js:206-220）。
 */
export function SettingsNotificationMatrix({ groups, repositories, refused, onToggle, pending = false }: SettingsNotificationMatrixProps) {
  const t = useT()
  const [folded, setFolded] = useState<Record<string, boolean>>({})

  // 旧版 UserSettingNotification.js:233 isTargetOpen —— 命中拒收列表即视为"关闭"
  const refusedTargets = useMemo(() => new Set(refused.map((item) => item.target)), [refused])

  // 组 → 组内仓库（旧版 FilterGenerator.group）
  const reposByGroup = useMemo(() => {
    const map = new Map<string, RepositoryListItem[]>()
    for (const repo of repositories) {
      const key = repo.group?.id ?? ''
      const list = map.get(key)
      if (list) {
        list.push(repo)
      } else {
        map.set(key, [repo])
      }
    }
    return map
  }, [repositories])

  // 不属于任何可见仓库组的仓库（旧版 otherRepository 分支）
  const standaloneRepos = useMemo(() => {
    const groupIds = new Set(groups.map((group) => group.id))
    return repositories.filter((repo) => !groupIds.has(repo.group?.id ?? ''))
  }, [groups, repositories])

  const isOpen = (target: string) => !refusedTargets.has(target)

  // 组开关：组自身被拒收 → 关闭；否则组内仓库"全部关闭或角色不足 DEVELOPER"时也算关闭
  const isGroupOpen = (group: Group) => {
    if (!isOpen(group.id)) {
      return false
    }
    const repos = reposByGroup.get(group.id) ?? []
    if (repos.length === 0) {
      return true
    }
    const blocked = repos.filter((repo) => !isOpen(repo.id) || repo.role < Role.DEVELOPER)
    return repos.length !== blocked.length
  }

  const renderSwitch = (role: number, type: 1 | 2, target: string, open: boolean, title: string) => {
    // 只读成员（role < DEVELOPER）不能改通知开关（旧版 UserSettingNotification.js:319）
    const readOnly = role < Role.DEVELOPER
    return (
      <Switch
        checked={readOnly ? false : open}
        disabled={readOnly || pending}
        aria-label={title}
        onChange={() => onToggle(type, target, !open)}
      />
    )
  }

  const renderGroupRows = (group: Group) => {
    const repos = reposByGroup.get(group.id) ?? []
    const isFolded = folded[group.id] ?? false
    return (
      <>
        <TR>
          <TD>
            <span className="row">
              {repos.length > 0 ? (
                <IconButton
                  label={isFolded ? t('label.expand') : t('label.collapse')}
                  aria-expanded={!isFolded}
                  onClick={() => setFolded((prev) => ({ ...prev, [group.id]: !isFolded }))}
                >
                  <span
                    style={{
                      display: 'inline-flex',
                      transform: isFolded ? 'rotate(180deg)' : undefined,
                      transition: 'transform 0.3s',
                    }}
                  >
                    <Icon icon={psCaretDown} size={12} />
                  </span>
                </IconButton>
              ) : null}
              <span>{group.displayName}</span>
            </span>
          </TD>
          <TD>
            <Marker variant={group.role === Role.OWNER ? 'containedInfo' : 'info'} text={t(`label.roleID_${group.role}`)} />
          </TD>
          <TD>{renderSwitch(group.role, 1, group.id, isGroupOpen(group), group.displayName)}</TD>
        </TR>
        {repos.length > 0 && !isFolded
          ? repos.map((repo) => (
              <TR key={repo.id}>
                <TD>
                  <span>
                    {/* 旧版：组内仓库显示成"组名/仓库名"（UserSettingNotification.js:322） */}
                    <span className="muted" style={{ marginInlineStart: 'var(--cf-space)' }}>
                      {group.displayName}/
                    </span>
                    {repo.displayName}
                  </span>
                </TD>
                <TD>
                  <Marker variant={repo.role === Role.OWNER ? 'containedInfo' : 'info'} text={t(`label.roleID_${repo.role}`)} />
                </TD>
                <TD>{renderSwitch(repo.role, 2, repo.id, isOpen(repo.id), `${group.displayName}/${repo.displayName}`)}</TD>
              </TR>
            ))
          : null}
      </>
    )
  }

  const renderStandaloneRow = (repo: RepositoryListItem) => (
    <TR key={repo.id}>
      <TD>
        <span>{repo.displayName}</span>
      </TD>
      <TD>
        <Marker variant={repo.role === Role.OWNER ? 'containedInfo' : 'info'} text={t(`label.roleID_${repo.role}`)} />
      </TD>
      <TD>{renderSwitch(repo.role, 2, repo.id, isOpen(repo.id), repo.displayName)}</TD>
    </TR>
  )

  if (groups.length === 0 && repositories.length === 0) {
    return null
  }

  return (
    <div className="stack">
      {groups.length > 0 ? (
        <TableWrap>
          <Table>
            <THead>
              <TR>
                <TH>{t('label.group')}</TH>
                <TH>{t('label.role')}</TH>
                <TH>{t('label.notificationReceiveStatus')}</TH>
              </TR>
            </THead>
            <TBody>
              {groups.map((group) => (
                <Fragment key={group.id}>{renderGroupRows(group)}</Fragment>
              ))}
            </TBody>
          </Table>
        </TableWrap>
      ) : null}

      {standaloneRepos.length > 0 ? (
        <TableWrap>
          <Table>
            <THead>
              <TR>
                <TH>{t('label.repository')}</TH>
                <TH>{t('label.role')}</TH>
                <TH>{t('label.notificationReceiveStatus')}</TH>
              </TR>
            </THead>
            <TBody>
              {standaloneRepos.map((repo) => renderStandaloneRow(repo))}
            </TBody>
          </Table>
        </TableWrap>
      ) : null}
    </div>
  )
}
