import { useCallback, useEffect, useState } from 'react'

import { deleteNotification, fetchNotifications, NOTIFICATION_CATEGORY, setNotificationRead, type NotificationItem } from '@/api/user'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { Loading } from '@/components/unit/Loading'
import { Tabs } from '@/components/unit/Tabs'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { useToast } from '@/components/unit/Toast'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


type CategoryKey = 'all' | 'unread' | 'system' | 'mention'

const CATEGORY_VALUE: Record<CategoryKey, number> = {
  all: NOTIFICATION_CATEGORY.all,
  unread: NOTIFICATION_CATEGORY.unread,
  system: NOTIFICATION_CATEGORY.system,
  mention: NOTIFICATION_CATEGORY.mention,
}

/**
 * 通知抽屉（点页头铃铛展开）。
 *
 * 端点：`user/notifications?category&page`（分类 0..3）+ `setNotificationRead` / `deleteNotification`
 * —— 后两者的 `all` 与 `id` **有且只能有一个**（后端强校验），所以"全部已读/清空"走 `all=1`，
 * 单条走 `id`。`data` 是 JSON 字符串，这里做容错解析后取 `message` 字段展示。
 */
export function NotificationDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const t = useT()
  const toast = useToast()
  const { refreshUser } = useAppData()

  const [category, setCategory] = useState<CategoryKey>('all')
  const [items, setItems] = useState<NotificationItem[]>([])
  const [loading, setLoading] = useState(false)

  const load = useCallback(() => {
    setLoading(true)
    fetchNotifications(CATEGORY_VALUE[category])
      .then(setItems)
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => setLoading(false))
  }, [category, t, toast])

  useEffect(() => {
    if (open) {
      load()
    }
  }, [open, load])

  const messageOf = (item: NotificationItem): string => {
    try {
      const parsed = JSON.parse(item.data) as Record<string, unknown>
      const candidate = parsed.message ?? parsed.content ?? parsed.title
      return typeof candidate === 'string' ? candidate : item.data
    } catch {
      return item.data
    }
  }

  const markAll = async () => {
    try {
      await setNotificationRead({ all: true })
      load()
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const clearAll = async () => {
    try {
      await deleteNotification({ all: true })
      load()
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const markOne = async (item: NotificationItem) => {
    try {
      await setNotificationRead({ id: item.id })
      load()
      refreshUser()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  if (!open) {
    return null
  }

  return (
    <>
      <div className="notification-backdrop" onClick={onClose} />
      <aside className="notification-panel" role="dialog" aria-label={t('label.notification')}>
        <div className="notification-panel__head">
          <span className="section__title">{t('label.notification')}</span>
          <div className="row">
            <Button size="sm" variant="text" onClick={markAll}>
              {t('label.markAllRead')}
            </Button>
            <Button size="sm" variant="text" tone="danger" onClick={clearAll}>
              {t('label.clearAll')}
            </Button>
            <Button size="sm" variant="text" onClick={onClose}>
              {t('base_confirm')}
            </Button>
          </div>
        </div>

        <Tabs<CategoryKey>
          value={category}
          onChange={setCategory}
          items={[
            { key: 'all', label: t('label.all') },
            { key: 'unread', label: t('label.unread') },
            { key: 'system', label: t('label.system') },
            { key: 'mention', label: t('label.mentionMe') },
          ]}
        />

        <div className="notification-panel__body">
          {loading ? <Loading /> : null}
          {!loading && items.length === 0 ? (
            <EmptyState title={t('message._S_listEmpty', { s: t('label.notification') })} />
          ) : null}
          {items.map((item) => (
            <div className={item.isRead ? 'notification-item' : 'notification-item notification-item--unread'} key={item.id}>
              <div className="notification-item__main">
                <div className="notification-item__text">{messageOf(item)}</div>
                <div className="notification-item__meta">
                  <FormattedTime timestamp={Number(item.created)} />
                </div>
              </div>
              <div className="row">
                {!item.isRead ? (
                  <Button size="sm" variant="text" onClick={() => markOne(item)}>
                    {t('label.markRead')}
                  </Button>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      </aside>
    </>
  )
}
