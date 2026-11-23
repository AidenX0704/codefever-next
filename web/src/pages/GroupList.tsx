import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'

import type { RepositoryListItem } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { FilterBar, type FilterBarSelect } from '@/components/unit/FilterBar'
import { Icon } from '@/components/unit/Icon'
import { ListSkeleton } from '@/components/unit/Skeleton'
import { GroupCard } from '@/components/view/unit/GroupCard'
import { GroupRow } from '@/components/view/unit/GroupRow'
import { gridViewIcon, listViewIcon, plusIcon } from '@/config/icons'
import { canReadRepository } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useDebouncedValue } from '@/helpers/useDebouncedValue'
import { useT } from '@/i18n'


type TabKey = 'all' | 'created' | 'joined'
type SortKey = 'updated' | 'repositories' | 'members' | 'name' | 'created'
type ViewKey = 'grid' | 'list'

function parseTab(raw: string | null): TabKey {
  if (raw === 'joined' || raw === 'created') {
    return raw
  }
  return 'all'
}

function parseSort(raw: string | null): SortKey {
  if (raw === 'repositories' || raw === 'members' || raw === 'name' || raw === 'created') {
    return raw
  }
  return 'updated'
}

function parseView(raw: string | null): ViewKey {
  return raw === 'list' ? 'list' : 'grid'
}

/**
 * 仓库组空间工作台（`/groups`）。
 *
 * 全新重设计：
 *   - 默认采用「空间门户卡片网格」：每张卡展示组身份、3 项核心读数、近期活跃仓库直达芯片、核心成员叠放；
 *   - 顶部采用胶囊分段筛选 + 搜索 + 定制 Select 排序 + 卡片/列表一键切换，零冗余底线；
 *   - 全部状态同步 URL 查询参数。
 */
export default function GroupList() {
  const t = useT()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { user, groups, repositories, loading, error } = useAppData()

  const tab = parseTab(searchParams.get('tab'))
  const sortBy = parseSort(searchParams.get('sort'))
  const order = searchParams.get('order') === 'asc' ? 'asc' : 'desc'
  const view = parseView(searchParams.get('view'))
  const urlKeyword = searchParams.get('q') ?? ''

  const [searchInput, setSearchInput] = useState(urlKeyword)
  const debouncedKeyword = useDebouncedValue(searchInput, 200)

  useEffect(() => {
    setSearchInput(urlKeyword)
  }, [urlKeyword])

  const updateParams = useCallback(
    (patch: Record<string, string | null>) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          for (const [k, v] of Object.entries(patch)) {
            if (
              !v ||
              (k === 'tab' && v === 'all') ||
              (k === 'sort' && v === 'updated') ||
              (k === 'order' && v === 'desc') ||
              (k === 'view' && v === 'grid')
            ) {
              next.delete(k)
            } else {
              next.set(k, v)
            }
          }
          return next
        },
        { replace: true },
      )
    },
    [setSearchParams],
  )

  useEffect(() => {
    const trimmed = debouncedKeyword.trim()
    if (trimmed !== urlKeyword) {
      updateParams({ q: trimmed || null })
    }
  }, [debouncedKeyword, urlKeyword, updateParams])

  const userId = user?.id ?? ''

  // 按仓库组归拢可读仓库并按更新时间倒序，用于门户卡的「近期仓库」直达芯片。
  const reposByGroup = useMemo(() => {
    const map = new Map<string, RepositoryListItem[]>()
    const readableRepos = repositories
      .filter((r) => canReadRepository(r.role))
      .sort((a, b) => b.updated - a.updated)
    for (const repo of readableRepos) {
      const list = map.get(repo.group.id)
      if (list) {
        list.push(repo)
      } else {
        map.set(repo.group.id, [repo])
      }
    }
    return map
  }, [repositories])

  const readable = useMemo(() => groups.filter((group) => canReadRepository(group.role)), [groups])

  const tabCounts = useMemo(() => {
    let created = 0
    let joined = 0
    for (const g of readable) {
      if (g.owner === userId) {
        created++
      } else {
        joined++
      }
    }
    return { all: readable.length, created, joined }
  }, [readable, userId])

  const filtered = useMemo(() => {
    const kw = urlKeyword.trim().toLowerCase()
    const scoped = readable.filter((group) => {
      if (tab === 'joined' && group.owner === userId) {
        return false
      }
      if (tab === 'created' && group.owner !== userId) {
        return false
      }
      if (kw) {
        return (
          group.displayName.toLowerCase().includes(kw) ||
          group.name.toLowerCase().includes(kw) ||
          (group.description ?? '').toLowerCase().includes(kw)
        )
      }
      return true
    })

    const asc = order === 'asc'
    return [...scoped].sort((a, b) => {
      let cmp: number
      switch (sortBy) {
        case 'name':
          cmp = a.displayName.localeCompare(b.displayName)
          break
        case 'created':
          cmp = a.created - b.created
          break
        case 'repositories': {
          const ra = a.repositoryCount ?? (reposByGroup.get(a.id)?.length ?? 0)
          const rb = b.repositoryCount ?? (reposByGroup.get(b.id)?.length ?? 0)
          cmp = ra - rb
          break
        }
        case 'members': {
          const ma = a.memberCount ?? a.members.filter((m) => !m.deleteFlag).length
          const mb = b.memberCount ?? b.members.filter((m) => !m.deleteFlag).length
          cmp = ma - mb
          break
        }
        default:
          cmp = (a.updated || a.created) - (b.updated || b.created)
          break
      }
      return asc ? cmp : -cmp
    })
  }, [readable, tab, userId, urlKeyword, sortBy, order, reposByGroup])

  const hasActiveFilters = Boolean(urlKeyword || tab !== 'all')
  const newGroupButton = (block = false) => (
    <Button
      variant="contained"
      block={block}
      startIcon={<Icon icon={plusIcon} size={15} />}
      onClick={() => navigate('/groups/new')}
    >
      {t('label.newGroup')}
    </Button>
  )

  const scopePills: Array<{ key: TabKey; label: string; count: number }> = [
    { key: 'all', label: t('label.all'), count: tabCounts.all },
    { key: 'created', label: t('label.ICreated'), count: tabCounts.created },
    { key: 'joined', label: t('label.IJoined'), count: tabCounts.joined },
  ]

  const sortSelects: FilterBarSelect[] = [
    {
      key: 'sort',
      value: sortBy,
      ariaLabel: t('repo.sortUpdated'),
      options: [
        { value: 'updated', label: t('repo.sortUpdated') },
        { value: 'repositories', label: t('group.sortRepos') },
        { value: 'members', label: t('group.sortMembers') },
        { value: 'name', label: t('repo.sortName') },
        { value: 'created', label: t('repo.sortCreated') },
      ],
      onChange: (value) => updateParams({ sort: value }),
    },
  ]

  const viewToggle = (
    <div className="segmented" role="group" aria-label="View mode">
      <button
        type="button"
        className={`segmented__btn${view === 'grid' ? ' segmented__btn--active' : ''}`}
        aria-pressed={view === 'grid'}
        title={t('group.viewGrid')}
        onClick={() => updateParams({ view: 'grid' })}
      >
        <Icon icon={gridViewIcon} size={16} />
      </button>
      <button
        type="button"
        className={`segmented__btn${view === 'list' ? ' segmented__btn--active' : ''}`}
        aria-pressed={view === 'list'}
        title={t('group.viewList')}
        onClick={() => updateParams({ view: 'list' })}
      >
        <Icon icon={listViewIcon} size={16} />
      </button>
    </div>
  )

  return (
    <div className="page">
      <div className="workspace-head">
        <div className="workspace-head__text">
          <div className="workspace-head__title-row">
            <h1 className="page__title">{t('label.group')}</h1>
            <span className="page-head__count">{t('group.countLabel', { n: filtered.length })}</span>
          </div>
          <p className="page__subtitle">{t('group.workspaceSubtitle')}</p>
        </div>
        <div className="workspace-head__actions">{newGroupButton()}</div>
      </div>

      {error ? <div className="auth-message">{error}</div> : null}

      <div className="group-toolbar-row">
        <div className="pill-switch" role="tablist" aria-label={t('label.group')}>
          {scopePills.map((item) => (
            <button
              key={item.key}
              type="button"
              role="tab"
              aria-selected={tab === item.key}
              className={`pill-switch__item${tab === item.key ? ' pill-switch__item--active' : ''}`}
              onClick={() => updateParams({ tab: item.key })}
            >
              <span>{item.label}</span>
              <span className="pill-switch__count">{item.count}</span>
            </button>
          ))}
        </div>

        <FilterBar
          className="group-toolbar-row__filter"
          searchValue={searchInput}
          onSearchChange={setSearchInput}
          searchPlaceholder={t('group.searchPlaceholder')}
          selects={sortSelects}
          sortDesc={order === 'desc'}
          sortLabel={t('label.order')}
          onSortToggle={() => updateParams({ order: order === 'desc' ? 'asc' : 'desc' })}
          extraTools={viewToggle}
        />
      </div>

      {loading ? (
        <div className="list-sheet">
          <ListSkeleton rows={5} />
        </div>
      ) : filtered.length > 0 ? (
        view === 'grid' ? (
          <div className="group-portal-grid">
            {filtered.map((group) => {
              const groupRepos = reposByGroup.get(group.id) ?? []
              return (
                <GroupCard
                  key={group.id}
                  group={group}
                  repositoryCount={groupRepos.length}
                  recentRepositories={groupRepos}
                />
              )
            })}
          </div>
        ) : (
          <div className="list-sheet">
            {filtered.map((group) => (
              <GroupRow
                key={group.id}
                group={group}
                repositoryCount={reposByGroup.get(group.id)?.length ?? 0}
              />
            ))}
          </div>
        )
      ) : hasActiveFilters ? (
        <div className="list-sheet">
          <EmptyState
            imageName="repositories-empty.png"
            title={t('group.noMatchTitle')}
            notice={t('group.noMatchNotice')}
            action={
              <Button
                variant="outlined"
                onClick={() => {
                  setSearchInput('')
                  setSearchParams({}, { replace: true })
                }}
              >
                {t('repo.clearFilters')}
              </Button>
            }
          />
        </div>
      ) : (
        <div className="list-sheet">
          <EmptyState
            imageName="repositories-empty.png"
            title={t('message._S_listEmpty', { s: t('label.group') })}
            notice={t('message.groupListEmptyNotice')}
            action={newGroupButton(true)}
          />
        </div>
      )}
    </div>
  )
}
