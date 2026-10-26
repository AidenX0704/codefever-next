import { useCallback, useEffect, useState } from 'react'

import { fetchNotificationRefused, NOTIFICATION_BIT, setGroupOrRepoNotification, toggleNotificationSetting, type NotificationRefuse } from '@/api/user'
import { SettingsNotificationMatrix } from '@/components/view/unit/SettingsNotificationMatrix'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { Switch } from '@/components/unit/Choices'
import { useToast } from '@/components/unit/Toast'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/**
 * 通知设置（`/settings/notification`）。
 *
 * 语义（来自后端 `repo.NotificationStatus*` 与 `userNotificationSetting`）：
 *   - `user.notification` 是位掩码，**位=1 表示关闭**；
 *   - 切换用 `userNotificationSetting`，body `offset` 是位序号：1=合并请求、2=邮件；
 *   - 被拒收的仓库组/仓库由 `user/notificationRefused` 列出（`type` 1=组、2=仓库），
 *     恢复用 `setGroupOrRepoNotification`（`type` + `target` + `open`），级联由后端处理。
 *
 * 组 / 仓库级开关的全量矩阵见 `components/view/unit/SettingsNotificationMatrix`。
 */
export default function NotificationSetting() {
  const t = useT()
  const toast = useToast()
  const { user, groups, repositories, refreshUser } = useAppData()

  const [refused, setRefused] = useState<NotificationRefuse[]>([])
  const [loading, setLoading] = useState(false)
  const [pending, setPending] = useState(false)

  const loadRefused = useCallback(() => {
    setLoading(true)
    return fetchNotificationRefused()
      .then(setRefused)
      .catch(() => setRefused([]))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    loadRefused()
  }, [loadRefused])

  const notification = user?.notification ?? 0
  const mrEnabled = (notification & NOTIFICATION_BIT.mr) === 0
  const emailEnabled = (notification & NOTIFICATION_BIT.email) === 0

  const toggle = async (offset: 1 | 2) => {
    try {
      await toggleNotificationSetting(offset)
      refreshUser()
      toast.success(t('message.updateSuccess'))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  /**
   * 组 / 仓库级开关。
   *
   * 旧版是**双向**开关：点一下取反当前状态再提交（UserSettingNotification.js:288-307），
   * 所以这里必须把"切换之后"的状态传给后端，不能恒传 open=true。
   */
  const toggleTarget = async (type: 1 | 2, target: string, open: boolean) => {
    setPending(true)
    try {
      await setGroupOrRepoNotification(type, target, open)
      toast.success(t('message.updateSuccess'))
      // 后端会级联关闭/恢复组下仓库，重新拉拒收列表才能算出新的开关状态
      await loadRefused()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setPending(false)
    }
  }

  const restore = async (item: NotificationRefuse) => {
    try {
      await setGroupOrRepoNotification(item.type === 1 ? 1 : 2, item.target, true)
      toast.success(t('message.updateSuccess'))
      loadRefused()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const nameOf = (item: NotificationRefuse): string => {
    if (item.type === 1) {
      return groups.find((group) => group.id === item.target)?.displayName ?? item.target
    }
    return repositories.find((repository) => repository.id === item.target)?.displayName ?? item.target
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.notification')}</div>

        <div className="stack">
          <div className="row">
            <Switch checked={mrEnabled} onChange={() => toggle(1)} label={t('message.mergeRequestNotification')} />
            <span className="muted">{t('message.mergeRequestNotificationDesc')}</span>
          </div>
          <div className="row">
            <Switch checked={emailEnabled} onChange={() => toggle(2)} label={t('message.emailNotification')} />
            <span className="muted">{t('message.emailNotificationDesc')}</span>
          </div>
        </div>
      </section>

      <section className="section">
        <div className="section__title">{t('label.repository')}</div>
        <SettingsNotificationMatrix
          groups={groups}
          repositories={repositories}
          refused={refused}
          pending={pending}
          onToggle={(type, target, open) => {
            void toggleTarget(type, target, open)
          }}
        />
      </section>

      <section className="section">
        <div className="section__title">{t('label.notificationReceiveStatus')}</div>
        {refused.length === 0 && !loading ? (
          <EmptyState title={t('message._S_listEmpty', { s: t('label.notification') })} />
        ) : (
          <div className="list-body">
            {refused.map((item) => (
              <div className="ref-row" key={`${item.type}-${item.id}`}>
                <span className="ref-row__name">{nameOf(item)}</span>
                <span className="ref-row__commit">{item.type === 1 ? t('label.group') : t('label.repository')}</span>
                <span className="row">
                  <Button size="sm" variant="text" onClick={() => restore(item)}>
                    {t('label.open')}
                  </Button>
                </span>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
