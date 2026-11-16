import { useEffect, useState } from 'react'

import { fetchAnnouncements, type AnnouncementItem } from '@/api/site'
import { SectionCard } from '@/components/unit/SectionCard'
import { Tag } from '@/components/unit/Tag'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { useT } from '@/i18n'


/**
 * 「平台公告」（DESIGN.md §7.2 右栏）。
 *
 * 数据源：`GET /api/site/announcement`（匿名端点，管理员在后台维护）。
 * 公告为空时回落到三条内置使用须知 —— 管理员没写公告不等于这里开天窗，
 * 须知（安全规范 / 保密提醒 / 支持渠道）本身是长期有效的内容。
 */
export function AnnouncementCard() {
  const t = useT()
  const [items, setItems] = useState<AnnouncementItem[] | undefined>(undefined)

  useEffect(() => {
    let cancelled = false
    fetchAnnouncements()
      .then((data) => {
        if (!cancelled) {
          setItems(data ?? [])
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

  const remote = (items ?? []).filter((item) => item.body.trim() !== '')

  return (
    <SectionCard title={t('home.section.announcement')}>
      {remote.length > 0 ? (
        <ul className="home-announcement">
          {remote.map((item) => (
            <li key={item.id}>
              {item.type === 'security' ? <Tag tone="danger">{t('announcement.type.security')}</Tag> : null}
              {item.type === 'privacy' ? <Tag tone="warning">{t('announcement.type.privacy')}</Tag> : null}
              {item.type === 'support' ? <Tag tone="primary">{t('announcement.type.support')}</Tag> : null}
              <span className="home-announcement__body">{item.body}</span>
              <span className="home-announcement__time">
                <FormattedTime timestamp={item.createdAt} />
              </span>
            </li>
          ))}
        </ul>
      ) : items === undefined ? null : (
        <ul className="home-announcement">
          <li>{t('home.announcement.security')}</li>
          <li>{t('home.announcement.confidential')}</li>
          <li>{t('home.announcement.support')}</li>
        </ul>
      )}
    </SectionCard>
  )
}
