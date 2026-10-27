import { useMemo, useState } from 'react'

import type { RepositoryListItem } from '@/api/types'
import { AppLink } from '@/components/unit/AppLink'
import { Avatar } from '@/components/unit/Avatar'
import { CopyButton } from '@/components/unit/CopyButton'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { DataTable, type Column } from '@/components/unit/Table'
import { Tabs } from '@/components/unit/Tabs'
import { SectionCard } from '@/components/unit/SectionCard'
import { Button } from '@/components/unit/Button'
import { canReadRepository } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { cloneUrls } from '@/helpers/cloneUrl'
import { useT } from '@/i18n'


type TabKey = 'updated' | 'created' | 'joined'

/** 首页只展示这么多行，其余交给 `/repositories`（首页不做分页）。 */
const HOME_REPOSITORY_LIMIT = 8

export interface MyRepositoriesTableProps {
  onViewAll: () => void
}

/**
 * 「我的仓库」区块（**表格，不是卡片网格**）。
 *
 * 形态依据：`docs/HOME_PAGE_PLAN_2026-10-27.md` §2.2 的行业结论 —— 对内平台普遍
 * "列表优先于卡片"，云效 Codeup 首页直接用表格，卡片只出现在项目管理语境。
 * 另：行高取 `--cf-table-row-height`（40px），与全站表格密度一致。
 *
 * 与云效的**有意差异**：不显示提交数 / 分支数 / 语言分布 —— 这三个都要逐库调用
 * `git`，8 个仓库就是 8 次子进程，首屏会掉到秒级（见方案 D2）。
 *
 * 数据来源是 `useAppData()` 里已经拉过的仓库清单，**不新增请求**。
 */
export function MyRepositoriesTable({ onViewAll }: MyRepositoriesTableProps) {
  const t = useT()
  const { user, repositories, loading } = useAppData()
  const [tab, setTab] = useState<TabKey>('updated')

  const userId = user?.id ?? ''

  const rows = useMemo(() => {
    const readable = repositories.filter((repository) => canReadRepository(repository.role))
    const filtered = readable.filter((repository) => {
      if (tab === 'created') {
        return repository.owner === userId
      }
      if (tab === 'joined') {
        return repository.owner !== userId
      }
      return true
    })
    return [...filtered].sort((a, b) => b.updated - a.updated).slice(0, HOME_REPOSITORY_LIMIT)
  }, [repositories, tab, userId])

  const columns: ReadonlyArray<Column<RepositoryListItem>> = [
    {
      key: 'repository',
      title: t('label.repository'),
      render: (row) => (
        <span className="home-repo-cell">
          <Avatar
            square
            size="sm"
            src={row.icon ? `/file/fetch/avatar/${row.icon}` : undefined}
            name={row.name}
          />
          <span className="home-repo-cell__names">
            <AppLink to={`/${row.group.name}/${row.name}`} className="home-repo-cell__link">
              {row.group.displayName}/{row.displayName}
            </AppLink>
            <span className="home-repo-cell__slug">{row.name}</span>
          </span>
        </span>
      ),
    },
    {
      key: 'description',
      title: t('label.description'),
      render: (row) => <span className="home-repo-cell__desc">{row.description || '—'}</span>,
    },
    {
      key: 'openMergeRequest',
      title: t('label.mrStatusPending'),
      width: 'var(--cf-home-col-narrow)',
      render: (row) => (row.mergeRequestCount.open > 0 ? row.mergeRequestCount.open : '—'),
    },
    {
      key: 'members',
      title: t('label.member_pl'),
      width: 'var(--cf-home-col-narrow)',
      render: (row) => row.members.length,
    },
    {
      key: 'updated',
      title: t('label.updatedIn'),
      width: 'var(--cf-home-col-time)',
      render: (row) => <FormattedTime timestamp={row.updated} />,
    },
    {
      key: 'actions',
      title: '',
      width: 'var(--cf-home-col-action)',
      render: (row) => {
        const urls = cloneUrls(user, row.group.name, row.name)
        return <CopyButton text={urls.ssh || urls.https} label={t('home.copyClone')} />
      },
    },
  ]

  return (
    <SectionCard
      title={t('home.section.mine')}
      actions={
        <Button variant="text" onClick={onViewAll}>
          {t('home.action.viewAll')}
        </Button>
      }
      toolbar={
        <Tabs<TabKey>
          value={tab}
          onChange={setTab}
          items={[
            { key: 'updated', label: t('home.tab.recentUpdated') },
            { key: 'created', label: t('label.ICreated') },
            { key: 'joined', label: t('label.IJoined') },
          ]}
        />
      }
    >
      <DataTable<RepositoryListItem>
        columns={columns}
        rows={rows}
        rowKey={(row) => row.id}
        loading={loading}
        emptyTitle={t('message._S_listEmpty', { s: t('label.repository') })}
        emptyNotice={t('home.empty.repositories')}
      />
    </SectionCard>
  )
}
