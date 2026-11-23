import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type RefObject } from 'react'
import { createPortal } from 'react-dom'

import { deleteNotification, fetchNotifications, NOTIFICATION_CATEGORY, setNotificationRead, type NotificationItem } from '@/api/user'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Loading } from '@/components/unit/Loading'
import { Tabs } from '@/components/unit/Tabs'
import { useToast } from '@/components/unit/Toast'
import { closeIcon, notificationIcon } from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


type CategoryKey = 'all' | 'unread' | 'system' | 'mention'

const CATEGORY_VALUE: Record<CategoryKey, number> = {
  all: NOTIFICATION_CATEGORY.all,
  unread: NOTIFICATION_CATEGORY.unread,
  system: NOTIFICATION_CATEGORY.system,
  mention: NOTIFICATION_CATEGORY.mention,
}

export interface NotificationDrawerProps {
  open: boolean
  onClose: () => void
  /** 触发按钮容器，用于将弹出卡片右对齐定位到按钮正下方，并在点外部判断时排除触发器本身。 */
  anchorRef?: RefObject<HTMLElement | null>
}

interface PopoverCoords {
  top: number
  right: number
}

const POPOVER_GAP = 8
const VIEWPORT_MARGIN = 16

/**
 * 通知弹出卡片（点页头铃铛展开，挂载在 document.body）。
 *
 * 端点：`user/notifications?category&page`（分类 0..3）+ `setNotificationRead` / `deleteNotification`
 * —— 后两者的 `all` 与 `id` **有且只能有一个**（后端强校验），所以"全部已读/清空"走 `all=1`，
 * 单条走 `id`。`data` 是 JSON 字符串，这里做容错解析后取 `message` 字段展示。
 *
 * 形态：采用锚定在铃铛按钮下方的弹出卡片（无全屏灰底遮罩），高度随消息条数自适应增长，
 * 最多撑满视口剩余高度（超长时列表区独立滚动）。
 */
export function NotificationDrawer({ open, onClose, anchorRef }: NotificationDrawerProps) {
  const t = useT()
  const toast = useToast()
  const { refreshUser } = useAppData()
  const titleId = useId()
  const panelRef = useRef<HTMLElement>(null)
  const restoreRef = useRef<HTMLElement | null>(null)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  const [category, setCategory] = useState<CategoryKey>('all')
  const [items, setItems] = useState<NotificationItem[]>([])
  const [loading, setLoading] = useState(false)
  const [coords, setCoords] = useState<PopoverCoords | null>(null)

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

  useLayoutEffect(() => {
    if (!open) return

    const updatePosition = () => {
      const anchor = anchorRef?.current
      if (!anchor) return
      const rect = anchor.getBoundingClientRect()
      setCoords({
        top: Math.round(rect.bottom + POPOVER_GAP),
        right: Math.max(VIEWPORT_MARGIN, Math.round(window.innerWidth - rect.right)),
      })
    }

    updatePosition()
    window.addEventListener('resize', updatePosition)
    return () => {
      window.removeEventListener('resize', updatePosition)
    }
  }, [open, anchorRef])

  const restoreFocus = useCallback(() => {
    const target = restoreRef.current
    restoreRef.current = null
    if (target?.isConnected) target.focus()
  }, [])

  const closeAndRestore = useCallback(() => {
    onCloseRef.current()
    restoreFocus()
  }, [restoreFocus])

  useEffect(() => {
    if (!open) return

    restoreRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    panelRef.current?.focus()

    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node
      if (panelRef.current?.contains(target)) return
      if (anchorRef?.current?.contains(target)) return
      // 点击外部关闭时不抢回焦点，避免打断用户在页面其它位置（如输入框/按钮）的操作。
      restoreRef.current = null
      onCloseRef.current()
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation()
        closeAndRestore()
      }
    }

    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)

    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open, anchorRef, closeAndRestore])

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

  // 必须通过 Portal 挂到 document.body，避开 .app-header 的 backdrop-filter
  // 包含块与堆叠上下文（否则 fixed 卡片会被困在 56px 顶栏内）。
  return createPortal(
    <aside
      ref={panelRef}
      className="notification-panel"
      role="dialog"
      aria-labelledby={titleId}
      tabIndex={-1}
      style={coords ? { top: coords.top, right: coords.right } : undefined}
    >
      <div className="notification-panel__head">
        <span className="section__title" id={titleId}>
          {t('label.notification')}
        </span>
        <div className="notification-panel__actions">
          <Button size="sm" variant="text" onClick={markAll}>
            {t('label.markAllRead')}
          </Button>
          <Button size="sm" variant="text" tone="danger" onClick={clearAll}>
            {t('label.clearAll')}
          </Button>
          <button type="button" className="notification-panel__close" aria-label={t('label.close')} onClick={closeAndRestore}>
            <Icon icon={closeIcon} size={16} />
          </button>
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
        {loading ? (
          <div className="notification-panel__loading">
            <Loading />
          </div>
        ) : null}
        {!loading && items.length === 0 ? (
          <EmptyState icon={notificationIcon} title={t('message._S_listEmpty', { s: t('label.notification') })} />
        ) : null}
        {!loading
          ? items.map((item) => (
              <div className={item.isRead ? 'notification-item' : 'notification-item notification-item--unread'} key={item.id}>
                {!item.isRead ? <span className="notification-item__dot" aria-hidden="true" /> : null}
                <div className="notification-item__main">
                  <div className="notification-item__text">{messageOf(item)}</div>
                  <div className="notification-item__meta">
                    <FormattedTime timestamp={Number(item.created)} />
                  </div>
                </div>
                {!item.isRead ? (
                  <div className="notification-item__action">
                    <Button size="sm" variant="text" onClick={() => markOne(item)}>
                      {t('label.markRead')}
                    </Button>
                  </div>
                ) : null}
              </div>
            ))
          : null}
      </div>
    </aside>,
    document.body,
  )
}
