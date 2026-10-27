import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'

import { fetchSearchResults, type SearchGroup, type SearchMergeRequest, type SearchRepository, type SearchResults } from '@/api/search'
import { AppLink } from '@/components/unit/AppLink'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Loading } from '@/components/unit/Loading'
import { SectionCard } from '@/components/unit/SectionCard'
import { TextField } from '@/components/unit/TextField'
import { MergeRequestRow } from '@/components/view/unit/MergeRequestRow'
import { plSearch } from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/**
 * 全局搜索页（`/search?keyword=…`）。
 *
 * 设计要点：
 *   - **关键词放在 URL 里**：结果可分享、可刷新、可后退，符合"搜索是导航"的常识；
 *   - 提交式搜索（回车或点按钮），不做输入即搜 —— 三类结果各 10 条，实时搜会打满
 *     后端 LIKE 查询，而用户的真实动作是"改几次词再回车"；
 *   - 三类结果各一块，被截断时明确提示"只显示前 10 条"，不让人误以为"就这些"；
 *   - 权限过滤在后端完成（SQL 里 JOIN 成员表），前端不再二次过滤。
 */
export default function Search() {
  const t = useT()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const { user, repositories } = useAppData()

  const keyword = (params.get('keyword') ?? '').trim()
  const [input, setInput] = useState(keyword)
  const [results, setResults] = useState<SearchResults | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // URL 变（后退/直接访问带参链接）时把输入框同步过来。
  useEffect(() => {
    setInput(keyword)
  }, [keyword])

  useEffect(() => {
    if (!keyword) {
      setResults(null)
      setError(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchSearchResults(keyword)
      .then((data) => {
        if (!cancelled) {
          setResults(data)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : t('base_error_message'))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [keyword, t])

  const submit = () => {
    const next = input.trim()
    if (!next) {
      setParams({})
      return
    }
    setParams({ keyword: next })
  }

  // 发起人名字：搜索结果是用户主键，按"我可见仓库的成员"解析（与首页同一口径）。
  const memberNames = useMemo(() => {
    const map = new Map<string, string>()
    for (const repository of repositories) {
      for (const member of repository.members) {
        if (!map.has(member.id)) {
          map.set(member.id, member.name)
        }
      }
    }
    if (user) {
      map.set(user.id, user.name)
    }
    return map
  }, [repositories, user])

  const total = results
    ? results.repositories.length + results.groups.length + results.mergeRequests.length
    : 0

  const renderRepository = (item: SearchRepository) => (
    <li key={item.id} className="search-row">
      <Avatar square size="sm" src={item.avatar ? `/file/fetch/avatar/${item.avatar}` : undefined} name={item.name} />
      <span className="search-row__main">
        <AppLink to={`/${item.groupName}/${item.name}`} className="search-row__title">
          {item.groupDisplayName || item.groupName}/{item.displayName || item.name}
        </AppLink>
        <span className="search-row__desc">{item.description || '—'}</span>
      </span>
      <span className="search-row__meta">
        <FormattedTime timestamp={item.updated} />
      </span>
    </li>
  )

  const renderGroup = (item: SearchGroup) => (
    <li key={item.id} className="search-row">
      <Avatar square size="sm" src={item.avatar ? `/file/fetch/avatar/${item.avatar}` : undefined} name={item.name} />
      <span className="search-row__main">
        <AppLink to={`/groups/${item.name}/repositories`} className="search-row__title">
          {item.displayName || item.name}
        </AppLink>
        <span className="search-row__desc">{item.description || '—'}</span>
      </span>
      <span className="search-row__meta">
        {t('search.repositoryCount', { n: item.repositoryCount })}
      </span>
    </li>
  )

  const renderMergeRequest = (item: SearchMergeRequest) => {
    const target = repositories.find((repository) => repository.id === item.targetRepository)
    const href = target ? `/${target.group.name}/${target.name}/mergerequests/${item.number}` : ''
    return (
      <li key={item.id} className="search-row search-row--mr">
        <MergeRequestRow item={item} href={href} showRepository compact />
        <span className="search-row__meta">{memberNames.get(item.creator) ?? item.creator}</span>
      </li>
    )
  }

  return (
    <div className="page">
      <div className="search-page__head">
        <TextField
          value={input}
          onChange={(event) => setInput(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              submit()
            }
          }}
          placeholder={t('search.placeholder')}
          startAdornment={<Icon icon={plSearch} size={16} />}
          aria-label={t('search.title')}
          containerClassName="search-page__field"
        />
        <Button variant="contained" onClick={submit}>
          {t('search.action')}
        </Button>
      </div>

      {!keyword ? (
        <EmptyState title={t('search.emptyTitle')} notice={t('search.emptyNotice')} />
      ) : loading ? (
        <Loading />
      ) : error ? (
        <div className="auth-message">{error}</div>
      ) : total === 0 ? (
        <EmptyState title={t('search.noResult', { s: keyword })} notice={t('search.noResultNotice')} />
      ) : (
        <>
          {results && results.repositories.length > 0 ? (
            <SectionCard
              title={t('search.section.repositories', { n: results.repositories.length })}
            >
              <ul className="search-list">{results.repositories.map(renderRepository)}</ul>
              {results.truncated.repositories ? (
                <p className="muted">{t('search.truncated')}</p>
              ) : null}
            </SectionCard>
          ) : null}

          {results && results.groups.length > 0 ? (
            <SectionCard title={t('search.section.groups', { n: results.groups.length })}>
              <ul className="search-list">{results.groups.map(renderGroup)}</ul>
              {results.truncated.groups ? <p className="muted">{t('search.truncated')}</p> : null}
            </SectionCard>
          ) : null}

          {results && results.mergeRequests.length > 0 ? (
            <SectionCard title={t('search.section.mergeRequests', { n: results.mergeRequests.length })}>
              <ul className="search-list">{results.mergeRequests.map(renderMergeRequest)}</ul>
              {results.truncated.mergeRequests ? <p className="muted">{t('search.truncated')}</p> : null}
            </SectionCard>
          ) : null}

          <p className="muted">
            {t('search.scopeNote')}{' '}
            <Button variant="text" size="sm" onClick={() => navigate(`/repositories`)}>
              {t('search.browseAll')}
            </Button>
          </p>
        </>
      )}
    </div>
  )
}
