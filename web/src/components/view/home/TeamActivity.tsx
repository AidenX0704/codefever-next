import { useMemo } from 'react'

import type { ActivityItem } from '@/api/types'
import { ActivityFeed } from '@/components/view/unit/ActivityFeed'
import { SectionCard } from '@/components/unit/SectionCard'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/**
 * 首页「团队动态」：复用 `ActivityFeed`（41 个活动码的表驱动渲染）。
 *
 * 这里只做一件事 —— 把 AppData 里的组名 / 仓库名 / 成员名解析函数接上去。
 * 活动流本身（活动码 → 文案、图标、主体）在 `ActivityFeed` 里，页面不得重写。
 *
 * 默认只显示前 5 条（方案 §2.2 的调研结论：动态 feed 是加分项，但内部噪音大，
 * 默认折叠）。"展开"是纯前端行为，不额外请求。
 */
export function TeamActivity({ items }: { items: ActivityItem[] }) {
  const t = useT()
  const { repositories, groups } = useAppData()

  // 成员字典：活动里的 creator / uid 是用户主键，名字要从"我可见的仓库成员"里找。
  // 同一个用户在多个仓库里重复出现，按主键覆盖即可（名字一致）。
  const members = useMemo(() => {
    const map = new Map<string, { name: string; icon: string | null }>()
    for (const repository of repositories) {
      for (const member of repository.members) {
        if (!map.has(member.id)) {
          map.set(member.id, { name: member.name, icon: member.icon })
        }
      }
    }
    return map
  }, [repositories])

  return (
    <SectionCard title={t('home.section.activities')}>
      <ActivityFeed
        items={items}
        groupNameOf={(id) => groups.find((group) => group.id === id)?.displayName ?? id ?? ''}
        repositoryNameOf={(id) => repositories.find((repository) => repository.id === id)?.displayName ?? id ?? ''}
        memberNameOf={(id) => members.get(id)?.name ?? id}
        memberIconOf={(id) => members.get(id)?.icon ?? null}
      />
    </SectionCard>
  )
}
