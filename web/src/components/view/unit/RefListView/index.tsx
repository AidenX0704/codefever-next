import { useMemo, useState, type ReactNode } from 'react'
import { Link } from 'react-router'

import type { Commit, RepositoryMember } from '@/api/types'
import { EmptyState } from '@/components/unit/EmptyState'
import { FilterBar } from '@/components/unit/FilterBar'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Loading } from '@/components/unit/Loading'
import { Tag } from '@/components/unit/Tag'
import { Button } from '@/components/unit/Button'
import { psBranch, psTag } from '@/config/icons'
import { resolveMember } from '@/helpers/members'
import { useT } from '@/i18n'


/** 分支 / 标签列表的一行（页面把自己的数据结构映射成这个形状）。 */
export interface RefListItem {
  key: string
  name: string
  /** 分支/标签说明（后端只有标签有）。 */
  detail?: string
  commit?: Commit
  protected?: boolean
  /** 默认分支标记。 */
  isDefault?: boolean
  /** 已合并标记（后端 `isMerge`：与默认分支无差异）。 */
  isMerge?: boolean
}

export interface RefListViewProps {
  kind: 'branch' | 'tag'
  items: RefListItem[]
  loading?: boolean
  /** 仓库成员（用于把提交邮箱解析成"更新者"姓名）。 */
  members?: RepositoryMember[]
  /** 删除回调；缺省则不渲染删除按钮。 */
  onDelete?: (name: string) => void
  /** 逐行判定能否删除（默认分支不能删，对齐旧版 `RefList.js` 的 `item.name !== defaultBranch`）。 */
  canDelete?: (item: RefListItem) => boolean
  /** 新建回调；缺省则不渲染新建按钮。 */
  onCreate?: () => void
  /** 是否提供"活跃 / 非活跃"页签（分支列表有，标签列表没有）。 */
  showActiveTabs?: boolean
  /** 跳转前缀（`/{组}/{仓库}`）。 */
  base: string
  /** 顶部错误提示区。 */
  error?: ReactNode
}

/** 每页条数（旧版 `RefList.js` 的 perpage = 10）。 */
const PER_PAGE = 10

/** 三个月（秒）——旧版 `BranchList.js` 的 threeMonth = 7776000。 */
const THREE_MONTHS = 7776000

/**
 * 分支 / 标签列表（对应旧版 `components/unit/RefFilter.js` + `RefList.js`）。
 *
 * 页面只负责"取数 + 映射成 `RefListItem`"，搜索 / 排序 / 活跃页签 / 分页
 * 这些通用交互全部收口在这里（AGENTS §4、§6）。
 */
export function RefListView({
  kind,
  items,
  loading = false,
  members,
  onDelete,
  canDelete,
  onCreate,
  showActiveTabs = false,
  base,
  error,
}: RefListViewProps) {
  const t = useT()
  const [keyword, setKeyword] = useState('')
  const [tab, setTab] = useState<'all' | 'active' | 'inactive'>('all')
  const [sortDesc, setSortDesc] = useState(true)
  const [page, setPage] = useState(1)

  const filtered = useMemo(() => {
    const needle = keyword.trim().toLowerCase()
    const matched = items.filter((item) => (needle ? item.name.toLowerCase().includes(needle) : true))
    matched.sort((left, right) => {
      const leftTime = Number(left.commit?.time ?? 0)
      const rightTime = Number(right.commit?.time ?? 0)
      return sortDesc ? rightTime - leftTime : leftTime - rightTime
    })
    return matched
  }, [items, keyword, sortDesc])

  const groups = useMemo(() => {
    const now = Math.floor(Date.now() / 1000)
    const active: RefListItem[] = []
    const inactive: RefListItem[] = []
    filtered.forEach((item) => {
      const time = Number(item.commit?.time ?? 0)
      if (now - time <= THREE_MONTHS) {
        active.push(item)
      } else {
        inactive.push(item)
      }
    })
    if (tab === 'active') {
      return [{ title: t('label.active'), items: active }]
    }
    if (tab === 'inactive') {
      return [{ title: t('label.inactive'), items: inactive }]
    }
    return [
      { title: t('label.active'), items: active },
      { title: t('label.inactive'), items: inactive },
    ]
  }, [filtered, tab, t])

  const visibleCount = page * PER_PAGE
  const total = filtered.length
  const hasMore = total > visibleCount

  const tabs = showActiveTabs
    ? [
        { key: 'all', label: t('label.all') },
        { key: 'active', label: t('label.active') },
        { key: 'inactive', label: t('label.inactive') },
      ]
    : undefined

  const resetPage = () => setPage(1)

  return (
    <div className="page">
      {error ? <div className="auth-message">{error}</div> : null}
      <FilterBar
        tabs={tabs}
        activeTab={tab}
        onTabChange={(key) => {
          setTab(key as 'all' | 'active' | 'inactive')
          resetPage()
        }}
        searchValue={keyword}
        onSearchChange={(value) => {
          setKeyword(value)
          resetPage()
        }}
        searchPlaceholder={t('message.input_S_name', { s: t(kind === 'branch' ? 'label.branch' : 'label.tag') })}
        sortDesc={sortDesc}
        onSortToggle={() => setSortDesc((value) => !value)}
        sortLabel={t('label.sort')}
        actions={
          onCreate ? (
            <Button variant="contained" onClick={onCreate}>
              {kind === 'branch' ? t('label.newBranch') : t('label.newTag')}
            </Button>
          ) : null
        }
      />

      {loading ? <Loading /> : null}

      {!loading && total === 0 ? (
        <EmptyState
          imageName={kind === 'branch' ? 'branches-empty.png' : 'tags-empty.png'}
          title={t('message._S_listEmpty', { s: t(kind === 'branch' ? 'label.branch_pl' : 'label.tag_pl') })}
        />
      ) : null}

      {groups.map((group) =>
        group.items.length === 0 ? null : (
          <div key={group.title}>
            <div className="ref-group__title">{group.title}</div>
            <div className="list-body">
              {group.items.slice(0, visibleCount).map((item) => (
                <RefRow
                  key={item.key}
                  item={item}
                  kind={kind}
                  base={base}
                  members={members}
                  onDelete={onDelete}
                  canDelete={canDelete}
                />
              ))}
            </div>
          </div>
        ),
      )}

      {!loading && total > 0 ? (
        <div className="list-more">
          <Button variant={hasMore ? 'contained' : 'outlined'} disabled={!hasMore} onClick={() => setPage((value) => value + 1)}>
            {hasMore ? t('label.more') : t('label.noMore')}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

function RefRow({
  item,
  kind,
  base,
  members,
  onDelete,
  canDelete,
}: {
  item: RefListItem
  kind: 'branch' | 'tag'
  base: string
  members?: RepositoryMember[]
  onDelete?: (name: string) => void
  canDelete?: (item: RefListItem) => boolean
}) {
  const t = useT()
  const commit = item.commit
  const updater = resolveMember(members, commit?.email)
  const filesHref = `${base}/files?rev=${encodeURIComponent(item.name)}`
  const commitHref = commit?.sha ? `${base}/commit/${commit.sha.slice(0, 8)}` : ''

  return (
    <div className="ref-row">
      <span className="ref-row__icon">
        <Icon icon={kind === 'branch' ? psBranch : psTag} size={16} />
      </span>
      <span className="ref-row__name">
        <Link className="cf-link" to={filesHref}>
          {item.name}
        </Link>
      </span>
      <span className="ref-row__badges">
        {item.protected ? <Tag tone="primary">{t('label.protected')}</Tag> : null}
        {item.isDefault ? <Tag tone="success">{t('label.default')}</Tag> : null}
        {item.isMerge && !item.isDefault ? <Tag tone="success">{t('message.merged')}</Tag> : null}
      </span>
      {item.detail ? <span className="ref-row__detail">{item.detail}</span> : null}
      <span className="ref-row__commit text-overflow">
        {commit ? (
          commitHref ? (
            <Link className="cf-link" to={commitHref}>
              {commit.commit}
            </Link>
          ) : (
            commit.commit
          )
        ) : null}
      </span>
      <span className="ref-row__updater">{updater.name}</span>
      <span className="ref-row__time">{commit ? <FormattedTime timestamp={Number(commit.time)} /> : null}</span>
      <span className="ref-row__hash">{commit?.sha ? commit.sha.slice(0, 8) : ''}</span>
      {onDelete && (canDelete ? canDelete(item) : true) ? (
        <span className="ref-row__actions">
          <Button size="sm" variant="text" tone="danger" onClick={() => onDelete(item.name)}>
            {t('label.delete')}
          </Button>
        </span>
      ) : null}
    </div>
  )
}
