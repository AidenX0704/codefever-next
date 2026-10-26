import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  deleteWebhook,
  fetchWebhookLogData,
  fetchWebhookLogs,
  fetchWebhooks,
  saveWebhook,
} from '@/api/repository'
import type { RepositoryWebhook, RepositoryWebhookLog, RepositoryWebhookLogData } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { Checkbox, Switch } from '@/components/unit/Choices'
import { useConfirm } from '@/components/unit/Confirm'
import { CopyButton } from '@/components/unit/CopyButton'
import { DataTable } from '@/components/unit/Table'
import { Dialog } from '@/components/unit/Dialog'
import { Form, FormItem } from '@/components/unit/Form'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Tag } from '@/components/unit/Tag'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { joinEvents, parseEvents, WEBHOOK_EVENTS } from '@/config/webhookEvents'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'


interface EditState {
  id: string
  url: string
  secret: string
  events: string[]
  active: boolean
}

/** 把日志报文渲染成可读文本：后端可能给字符串，也可能给已解码的对象。 */
function dumpPayload(payload: unknown): string {
  if (payload === undefined || payload === null) {
    return ''
  }
  if (typeof payload === 'string') {
    return payload
  }
  try {
    return JSON.stringify(payload, null, 2)
  } catch {
    return String(payload)
  }
}

/** 密钥只显示头尾，避免整串明文铺在列表里。 */
function maskSecret(secret: string | undefined): string {
  if (!secret) {
    return '-'
  }
  if (secret.length <= 8) {
    return '••••••••'
  }
  return `${secret.slice(0, 4)}••••${secret.slice(-4)}`
}

/**
 * Webhook 设置（`/{组}/{仓库}/settings/webhook`）。
 *
 * 六个端点逐项对齐后端：
 *   `webhooks`（列表）/ `getWebhook`（详情，本页未用——列表已含全部字段）/
 *   `editWebhook`（**不带 rwKey 即新建**，带则更新；`active` 必须是 1 或 2）/
 *   `deleteWebhook` / `getRepositoryWebhookLogs`（`webhook` 参数）/ `getRepositoryWebhookLogData`（`id`）。
 * `events` 是逗号分隔的事件名（后端 `WebhookEventMatches` 精确匹配，不做通配）。
 */
export default function SettingsWebhook() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { summary } = useRepository()

  const [webhooks, setWebhooks] = useState<RepositoryWebhook[]>([])
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<EditState | null>(null)
  const [saving, setSaving] = useState(false)
  const [logWebhook, setLogWebhook] = useState<RepositoryWebhook | null>(null)
  const [logs, setLogs] = useState<RepositoryWebhookLog[]>([])
  const [logData, setLogData] = useState<RepositoryWebhookLogData | null>(null)

  const reload = useCallback(() => {
    if (!summary) {
      return
    }
    setLoading(true)
    fetchWebhooks(summary.id)
      .then(setWebhooks)
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => setLoading(false))
  }, [summary, t, toast])

  useEffect(() => {
    reload()
  }, [reload])

  const openCreate = () => {
    const defaults = WEBHOOK_EVENTS.filter((spec) => spec.checked && spec.event).map((spec) => spec.event as string)
    setEditing({ id: '', url: '', secret: '', events: defaults, active: true })
  }

  const openEdit = (item: RepositoryWebhook) => {
    setEditing({ id: item.id, url: item.url, secret: item.secret ?? '', events: parseEvents(item.events), active: item.active === 1 })
  }

  const toggleEvent = (event: string) => {
    setEditing((current) => {
      if (!current) {
        return current
      }
      const has = current.events.includes(event)
      return { ...current, events: has ? current.events.filter((item) => item !== event) : [...current.events, event] }
    })
  }

  const save = async () => {
    if (!summary || !editing) {
      return
    }
    if (!editing.url.trim() || editing.events.length === 0) {
      toast.error(t('label.webhook_pl'))
      return
    }
    setSaving(true)
    try {
      await saveWebhook({
        repositoryKey: summary.id,
        webhookKey: editing.id || undefined,
        url: editing.url.trim(),
        // 后端 `editWebhook` 缺 secret 会直接 500（rw_secret 是 NOT NULL 列），必须传。
        secret: editing.secret,
        events: joinEvents(editing.events),
        active: editing.active ? 1 : 2,
      })
      toast.success(t('message.updateSuccess'))
      setEditing(null)
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSaving(false)
    }
  }

  const remove = async (item: RepositoryWebhook) => {
    if (!summary) {
      return
    }
    const ok = await confirm({ title: t('label.delete'), message: item.url, tone: 'danger', confirmText: t('base_confirm') })
    if (!ok) {
      return
    }
    try {
      await deleteWebhook(summary.id, item.id)
      toast.success(t('message.deleteSuccess'))
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const openLogs = async (item: RepositoryWebhook) => {
    setLogWebhook(item)
    setLogs([])
    try {
      setLogs(await fetchWebhookLogs(item.id))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const eventSummary = useMemo(() => {
    return (events: string) => {
      const list = parseEvents(events)
      const labels = list
        .map((event) => WEBHOOK_EVENTS.find((spec) => spec.event === event)?.labelKey)
        .filter((key): key is string => Boolean(key))
        .map((key) => t(key))
      return labels.length > 2 ? `${labels.slice(0, 2).join(', ')} +${labels.length - 2}` : labels.join(', ')
    }
  }, [t])

  return (
    <div className="page">
      <div className="row">
        <Button variant="contained" onClick={openCreate}>{t('label.addWebhook')}</Button>
      </div>

      <DataTable
        columns={[
          {
            key: 'url',
            title: 'URL',
            render: (row) => (
              <span className="row">
                <span className="text-overflow">{row.url}</span>
                <CopyButton text={row.url} label={t('label.copy')} />
              </span>
            ),
          },
          {
            key: 'secret',
            title: t('label.webhookSecret'),
            render: (row) => (
              <span className="row">
                <span className="text-overflow">{maskSecret(row.secret)}</span>
                <CopyButton text={row.secret} label={t('label.copy')} />
              </span>
            ),
          },
          { key: 'events', title: t('label.webhookEvents'), render: (row) => eventSummary(row.events) },
          {
            key: 'active',
            title: t('label.enable'),
            render: (row) => (
              <Tag tone={row.active === 1 ? 'success' : 'default'}>
                {row.active === 1 ? t('label.enable') : t('label.disable')}
              </Tag>
            ),
          },
          { key: 'user', title: t('label.creator'), render: (row) => row.user ?? '-' },
          {
            key: 'updated',
            title: t('label.updatedAt'),
            render: (row) => <FormattedTime timestamp={row.updated} absolute />,
          },
          {
            key: 'ops',
            title: '',
            render: (row) => (
              <span className="row">
                <Button size="sm" variant="text" onClick={() => openEdit(row)}>
                  {t('label.edit')}
                </Button>
                <Button size="sm" variant="text" onClick={() => openLogs(row)}>
                  {t('label.webhookLog')}
                </Button>
                <Button size="sm" variant="text" tone="danger" onClick={() => remove(row)}>
                  {t('label.delete')}
                </Button>
              </span>
            ),
          },
        ]}
        rows={webhooks}
        rowKey={(row) => row.id}
        loading={loading}
        emptyTitle={t('message._S_listEmpty', { s: t('label.webhook_pl') })}
      />

      <Dialog
        open={editing !== null}
        title={editing?.id ? t('label.edit') : t('label.addWebhook')}
        onClose={() => setEditing(null)}
        actions={
          <>
            <Button variant="text" onClick={() => setEditing(null)}>
              {t('base_confirm')}
            </Button>
            <Button variant="contained" loading={saving} onClick={save}>
              {t('label.save')}
            </Button>
          </>
        }
      >
        <Form>
          <FormItem label="URL" required>
            <TextField
              value={editing?.url ?? ''}
              onChange={(event) => setEditing((current) => (current ? { ...current, url: event.target.value } : current))}
              placeholder="https://example.com/hook"
            />
          </FormItem>
          <FormItem label={t('label.webhookSecret')} required help={t('label.webhookSecretTip')}>
            <TextField
              value={editing?.secret ?? ''}
              onChange={(event) => setEditing((current) => (current ? { ...current, secret: event.target.value } : current))}
              placeholder="******"
            />
          </FormItem>
          <FormItem label={t('label.webhookEvents')} required>
            <div className="stack">
              {WEBHOOK_EVENTS.map((spec) => (
                <div key={spec.event ?? spec.labelKey}>
                  {spec.divider ? <div className="cf-menu__divider" /> : null}
                  {spec.event ? (
                    <Checkbox
                      label={t(spec.labelKey)}
                      checked={editing?.events.includes(spec.event) ?? false}
                      onChange={() => toggleEvent(spec.event as string)}
                    />
                  ) : null}
                </div>
              ))}
            </div>
          </FormItem>
          <Switch
            label={t('label.enable')}
            checked={editing?.active ?? true}
            onChange={(event) => setEditing((current) => (current ? { ...current, active: event.target.checked } : current))}
          />
        </Form>
      </Dialog>

      <Dialog
        open={logWebhook !== null}
        title={t('label.webhookLog')}
        onClose={() => {
          setLogWebhook(null)
          setLogData(null)
        }}
        actions={
          <Button variant="text" onClick={() => setLogWebhook(null)}>
            {t('base_confirm')}
          </Button>
        }
      >
        <div className="list-body">
          {logs.length === 0 ? <div className="muted">{t('message._S_listEmpty', { s: t('label.webhookLog') })}</div> : null}
          {logs.map((log) => (
            <div className="ref-row" key={log.id}>
              <span className="ref-row__name">{log.created}</span>
              <span className="ref-row__commit">
                <Tag tone={log.success ? 'success' : 'danger'}>{log.status || (log.success ? 200 : 500)}</Tag>
              </span>
              <span className="row">
                <Button
                  size="sm"
                  variant="text"
                  onClick={async () => {
                    try {
                      setLogData(await fetchWebhookLogData(log.id))
                    } catch (error) {
                      toast.error(error instanceof Error ? error.message : t('base_error_message'))
                    }
                  }}
                >
                  {t('label.detail')}
                </Button>
              </span>
            </div>
          ))}
        </div>
        {logData ? (
          <div className="stack">
            <div className="muted">{t('label.requestBody')}</div>
            <pre className="webhook-log">{dumpPayload(logData.request)}</pre>
            <div className="muted">{t('label.responseBody')}</div>
            <pre className="webhook-log">{dumpPayload(logData.response)}</pre>
          </div>
        ) : null}
      </Dialog>
    </div>
  )
}
