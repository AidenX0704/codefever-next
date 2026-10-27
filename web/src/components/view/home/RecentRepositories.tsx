import { useMemo } from 'react'

import { AppLink } from '@/components/unit/AppLink'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { SectionCard } from '@/components/unit/SectionCard'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { readRecentRepositories } from '@/helpers/recentRepos'
import { useT } from '@/i18n'


/**
 * 「最近访问」：localStorage 记录（`helpers/recentRepos`），渲染前与当前可见仓库求交集。
 *
 * 求交集的原因：仓库被删除、或我的权限被收回之后，历史记录还在，
 * 直接渲染会得到一个"点进去 404"的死链。宁可少一行。
 */
export function RecentRepositories({ onBrowse }: { onBrowse: () => void }) {
  const t = useT()
  const { repositories } = useAppData()

  const items = useMemo(() => {
    const visible = new Map(repositories.map((repository) => [`${repository.group.name}/${repository.name}`, repository]))
    return readRecentRepositories()
      .map((entry) => visible.get(`${entry.groupName}/${entry.repositoryName}`) ?? null)
      .filter((repository): repository is NonNullable<typeof repository> => repository !== null)
  }, [repositories])

  return (
    <SectionCard title={t('home.section.recent')}>
      {items.length === 0 ? (
        <EmptyState
          title={t('home.empty.recent')}
          action={
            <Button variant="outlined" onClick={onBrowse}>
              {t('home.action.browse')}
            </Button>
          }
        />
      ) : (
        <ul className="home-recent">
          {items.map((repository) => (
            <li key={repository.id}>
              <AppLink to={`/${repository.group.name}/${repository.name}`} className="home-recent__link">
                <Avatar square size="sm" src={repository.icon ? `/file/fetch/avatar/${repository.icon}` : undefined} name={repository.name} />
                <span className="home-recent__name">
                  {repository.group.displayName}/{repository.displayName}
                </span>
              </AppLink>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  )
}
