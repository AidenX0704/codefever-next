import { useEffect, useState } from 'react'

import { fetchChangelogs, type ChangelogItem } from '@/api/site'
import { SectionCard } from '@/components/unit/SectionCard'
import { Timeline } from '@/components/unit/Timeline'
import { Tag } from '@/components/unit/Tag'
import { useT } from '@/i18n'


/**
 * 「更新日志」（DESIGN.md §7.2 右栏）：Timeline 形态，取最新 5 条。
 *
 * 数据源：`GET /api/site/changelog`（匿名端点）。管理后台可维护；
 * 空列表显示空态，不占位假数据。
 */
export function ChangelogPanel() {
  const t = useT()
  const [items, setItems] = useState<ChangelogItem[] | undefined>(undefined)

  useEffect(() => {
    let cancelled = false
    fetchChangelogs({ page: 1, pagesize: 5 })
      .then((data) => {
        if (!cancelled) {
          setItems(data?.list ?? [])
        }
      })
      .catch(() => {
        if (!cancelled) {
          setItems([])
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <SectionCard title={t('home.section.changelog')}>
      {items === undefined ? null : items.length === 0 ? (
        <div className="home-changelog__empty">{t('changelog.empty')}</div>
      ) : (
        <Timeline
          items={items.map((item, index) => ({
            key: item.id,
            date: item.releasedAt ?? '',
            title: item.title,
            content: item.body,
            tag: item.version ? <Tag tone="primary">{item.version}</Tag> : undefined,
            latest: index === 0,
          }))}
        />
      )}
    </SectionCard>
  )
}
