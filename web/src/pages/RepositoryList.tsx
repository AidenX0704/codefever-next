import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'

import { fetchRepositoryPage } from '@/api/repository'
import type { RepositoryListItem, RepositoryScopeCounts } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { FilterBar, type FilterBarSelect } from '@/components/unit/FilterBar'
import { Icon } from '@/components/unit/Icon'
import { ListSkeleton } from '@/components/unit/Skeleton'
import { Pagination } from '@/components/unit/Table'
import { RepositoryCard } from '@/components/view/unit/RepositoryCard'
import { RepositoryRow } from '@/components/view/unit/RepositoryRow'
import { gridViewIcon, groupIcon, listViewIcon, plusIcon } from '@/config/icons'
import { PermissionCode, hasPermission } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useDebouncedValue } from '@/helpers/useDebouncedValue'
import { useT } from '@/i18n'


type TabKey = 'all' | 'created' | 'joined' | 'starred'
type SortKey = 'updated' | 'created' | 'name' | 'stars' | 'openMr'
type OrderKey = 'desc' | 'asc'
type ViewKey = 'list' | 'grid'

const PAGE_SIZE = 20

function parseTab(raw: string | null): TabKey {
  if (raw === 'joined' || raw === 'created' || raw === 'starred') {
    return raw
  }
  return 'all'
}

function parseSort(raw: string | null): SortKey {
  if (raw === 'created' || raw === 'name' || raw === 'stars' || raw === 'openMr') {
    return raw
  }
  return 'updated'
}

function parseOrder(raw: string | null): OrderKey {
  return raw === 'asc' ? 'asc' : 'desc'
}

function parseView(raw: string | null): ViewKey {
  return raw === 'grid' ? 'grid' : 'list'
}

function parsePage(raw: string | null): number {
  const n = Number(raw)
  return Number.isInteger(n) && n > 0 ? n : 1
}

/**
 * 仓库工作台（`/repositories` 与组内 `/groups/{组名}/repositories`）。
 *
 * 全新设计：
 *   - 全局页采用「标题 + 副标题 + 快捷入口」页头与 4 格可切换统计卡（Scope Stat Deck），
 *     将概览读数与范围切换融为一体；
 *   - 无底边框悬浮工具条（关键字搜索 + 定制 Select 组过滤/排序 + 列表/卡片双视图）；
 *   - 组内页自动融入仓库组双栏工作台的右侧主舞台，零重复大标题。
 */
export default function RepositoryList() {
  const t = useT()
  const navigate = useNavigate()
  const { groupName } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const { groups, loading: appLoading } = useAppData()

  const currentGroup = useMemo(
    () => (groupName ? groups.find((group) => group.name === groupName) : undefined),
    [groups, groupName],
  )

  const tab = parseTab(searchParams.get('tab'))
  const sortBy = parseSort(searchParams.get('sort'))
  const order = parseOrder(searchParams.get('order'))
  const view = parseView(searchParams.get('view'))
  const page = parsePage(searchParams.get('page'))
  const urlKeyword = searchParams.get('q') ?? ''
  const selectedGroupKey = currentGroup ? currentGroup.id : (searchParams.get('group') ?? '')

  const [searchInput, setSearchInput] = useState(urlKeyword)
  const debouncedKeyword = useDebouncedValue(searchInput, 200)

  const [items, setItems] = useState<RepositoryListItem[]>([])
  const [total, setTotal] = useState(0)
  const [counts, setCounts] = useState<RepositoryScopeCounts>({ all: 0, joined: 0, created: 0, starred: 0 })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const listTopRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setSearchInput(urlKeyword)
  }, [urlKeyword])

  const updateParams = useCallback(
    (patch: Record<string, string | null>, resetPage = true) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          for (const [k, v] of Object.entries(patch)) {
            if (
              !v ||
              (k === 'tab' && v === 'all') ||
              (k === 'sort' && v === 'updated') ||
              (k === 'order' && v === 'desc') ||
              (k === 'view' && v === 'list') ||
              (k === 'page' && v === '1')
            ) {
              next.delete(k)
            } else {
              next.set(k, v)
            }
          }
          if (resetPage && !('page' in patch)) {
            next.delete('page')
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

  useEffect(() => {
    if (groupName && appLoading && !currentGroup) {
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)

    fetchRepositoryPage({
      groupKey: selectedGroupKey || undefined,
      scope: tab,
      keyword: urlKeyword,
      sort: sortBy,
      order,
      page,
      pagesize: PAGE_SIZE,
    })
      .then((res) => {
        if (cancelled) {
          return
        }
        setItems(res.list)
        setTotal(res.total)
        setCounts(res.counts)
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return
        }
        setError(err instanceof Error ? err.message : t('base_error_message'))
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [groupName, appLoading, currentGroup, selectedGroupKey, tab, urlKeyword, sortBy, order, page, t])

  const handleStarChange = useCallback(
    (repositoryId: string, starred: boolean, starCount: number) => {
      setCounts((prev) => ({
        ...prev,
        starred: Math.max(0, prev.starred + (starred ? 1 : -1)),
      }))
      setItems((prev) => {
        if (tab === 'starred' && !starred) {
          return prev.filter((item) => item.id !== repositoryId)
        }
        return prev.map((item) => (item.id === repositoryId ? { ...item, starred, starCount } : item))
      })
      if (tab === 'starred' && !starred) {
        setTotal((prev) => Math.max(0, prev - 1))
      }
    },
    [tab],
  )

  const canCreateInCurrentGroup =
    !currentGroup || hasPermission(currentGroup.role, PermissionCode.GROUP_CREATE_REPO)
  const hasActiveFilters = Boolean(urlKeyword || (!currentGroup && selectedGroupKey) || tab !== 'all')
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const clearFilters = () => {
    setSearchInput('')
    setSearchParams({}, { replace: true })
  }

  const newRepositoryButton = (block = false) => (
    <Button
      variant="contained"
      block={block}
      startIcon={<Icon icon={plusIcon} size={15} />}
      onClick={() => navigate(currentGroup ? `/groups/${currentGroup.name}/repositories/new` : '/repositories/new')}
    >
      {t('label.newRepository')}
    </Button>
  )

  const scopeCards: Array<{ key: TabKey; label: string; count: number }> = [
    { key: 'all', label: currentGroup ? t('label.repository_pl') : t('label.all'), count: counts.all },
    { key: 'created', label: t('label.ICreated'), count: counts.created },
    { key: 'joined', label: t('label.IJoined'), count: counts.joined },
    { key: 'starred', label: t('repo.tabStarred'), count: counts.starred },
  ]

  const filterSelects: FilterBarSelect[] = [
    ...(!currentGroup && groups.length > 0
      ? [
          {
            key: 'group',
            value: selectedGroupKey,
            ariaLabel: t('label.group'),
            options: [
              { value: '', label: t('repo.allGroups') },
              ...groups.map((g) => ({ value: g.id, label: g.displayName, description: `/${g.name}` })),
            ],
            onChange: (value: string) => updateParams({ group: value || null }),
          },
        ]
      : []),
    {
      key: 'sort',
      value: sortBy,
      ariaLabel: t('repo.sortUpdated'),
      options: [
        { value: 'updated', label: t('repo.sortUpdated') },
        { value: 'created', label: t('repo.sortCreated') },
        { value: 'name', label: t('repo.sortName') },
        { value: 'stars', label: t('repo.sortStars') },
        { value: 'openMr', label: t('repo.sortOpenMr') },
      ],
      onChange: (value: string) => updateParams({ sort: value }),
    },
  ]

  const viewToggle = (
    <div className="segmented" role="group" aria-label="View mode">
      <button
        type="button"
        className={`segmented__btn${view === 'list' ? ' segmented__btn--active' : ''}`}
        aria-pressed={view === 'list'}
        title={t('group.viewList')}
        onClick={() => updateParams({ view: 'list' }, false)}
      >
        <Icon icon={listViewIcon} size={16} />
      </button>
      <button
        type="button"
        className={`segmented__btn${view === 'grid' ? ' segmented__btn--active' : ''}`}
        aria-pressed={view === 'grid'}
        title={t('group.viewGrid')}
        onClick={() => updateParams({ view: 'grid' }, false)}
      >
        <Icon icon={gridViewIcon} size={16} />
      </button>
    </div>
  )

  return (
    <div className={currentGroup ? 'stage-stack' : 'page'} ref={listTopRef}>
      {!currentGroup ? (
        <div className="workspace-head">
          <div className="workspace-head__text">
            <h1 className="page__title">{t('label.repository')}</h1>
            <p className="page__subtitle">{t('repo.workspaceSubtitle')}</p>
          </div>
          <div className="workspace-head__actions">
            <Button
              variant="outlined"
              startIcon={<Icon icon={groupIcon} size={15} />}
              onClick={() => navigate('/groups')}
            >
              {t('repo.manageGroups')}
            </Button>
            {newRepositoryButton()}
          </div>
        </div>
      ) : null}

      {error ? <div className="auth-message">{error}</div> : null}

      {/* 四格范围统计切换卡：读数与范围筛选一体，替代传统厚重 Tab 栏 */}
      <div className="scope-deck" role="tablist" aria-label={t('label.repository')}>
        {scopeCards.map((item) => {
          const active = tab === item.key
          return (
            <button
              key={item.key}
              type="button"
              role="tab"
              aria-selected={active}
              className={`scope-tile${active ? ' scope-tile--active' : ''}`}
              onClick={() => updateParams({ tab: item.key })}
            >
              <span className="scope-tile__label">{item.label}</span>
              <span className="scope-tile__count">{item.count}</span>
            </button>
          )
        })}
      </div>

      <FilterBar
        searchValue={searchInput}
        onSearchChange={setSearchInput}
        searchPlaceholder={t('repo.searchPlaceholder')}
        selects={filterSelects}
        sortDesc={order === 'desc'}
        sortLabel={t('label.order')}
        onSortToggle={() => updateParams({ order: order === 'desc' ? 'asc' : 'desc' })}
        extraTools={viewToggle}
      />

      {loading ? (
        <div className="list-sheet">
          <ListSkeleton rows={5} />
        </div>
      ) : items.length > 0 ? (
        view === 'list' ? (
          <div className="list-sheet">
            {items.map((repository) => (
              <RepositoryRow
                key={repository.id}
                repository={repository}
                hideGroupPrefix={Boolean(currentGroup)}
                onStarChange={handleStarChange}
              />
            ))}
            {total > PAGE_SIZE ? (
              <div className="list-sheet__foot">
                <Pagination
                  page={page}
                  pageCount={pageCount}
                  total={total}
                  onChange={(nextPage) => {
                    updateParams({ page: String(nextPage) }, false)
                    listTopRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
                  }}
                />
              </div>
            ) : null}
          </div>
        ) : (
          <>
            <div className="repo-grid">
              {items.map((repository) => (
                <RepositoryCard
                  key={repository.id}
                  repository={repository}
                  onStarChange={handleStarChange}
                />
              ))}
            </div>
            {total > PAGE_SIZE ? (
              <div className="grid-pagination">
                <Pagination
                  page={page}
                  pageCount={pageCount}
                  total={total}
                  onChange={(nextPage) => {
                    updateParams({ page: String(nextPage) }, false)
                    listTopRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
                  }}
                />
              </div>
            ) : null}
          </>
        )
      ) : hasActiveFilters ? (
        <div className="list-sheet">
          <EmptyState
            imageName="repositories-empty.png"
            title={t('repo.noMatchTitle')}
            notice={t('repo.noMatchNotice')}
            action={
              <Button variant="outlined" onClick={clearFilters}>
                {t('repo.clearFilters')}
              </Button>
            }
          />
        </div>
      ) : (
        <div className="list-sheet">
          <EmptyState
            imageName="repositories-empty.png"
            title={t('message._S_listEmpty', { s: t('label.repository') })}
            notice={t('message.repositoryListEmptyNotice')}
            action={canCreateInCurrentGroup ? newRepositoryButton(true) : undefined}
          />
        </div>
      )}
    </div>
  )
}
