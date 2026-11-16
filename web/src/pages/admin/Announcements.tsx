import { useCallback, useEffect, useRef, useState } from 'react'

import { deleteAdminAnnouncement, deleteAdminChangelog, saveAdminAnnouncement, saveAdminChangelog } from '@/api/admin'
import { fetchChangelogs, fetchAnnouncements, type AnnouncementItem, type ChangelogItem } from '@/api/site'
import { Button } from '@/components/unit/Button'
import { Switch } from '@/components/unit/Choices'
import { useConfirm } from '@/components/unit/Confirm'
import { Dialog } from '@/components/unit/Dialog'
import { FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { Loading } from '@/components/unit/Loading'
import { SelectField } from '@/components/unit/SelectField'
import { TextField, TextareaField } from '@/components/unit/TextField'
import { Tag } from '@/components/unit/Tag'
import { useToast } from '@/components/unit/Toast'
import { useT } from '@/i18n'


/**
 * 管理后台 · 公告与更新日志（`/admin/announcements`）。
 *
 * 维护两个匿名端点的数据源：
 *   - `cc_announcements`（/api/site/announcement）：≤10 条启用公告；
 *   - `cc_changelogs`（/api/site/changelog）：分页更新日志。
 * 保存 / 删除走 `admin/changelogSave|Delete` 与 `admin/announcementSave|Delete`。
 */
export default function AdminAnnouncements() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()

  const [loading, setLoading] = useState(true)
  const [changelogs, setChangelogs] = useState<ChangelogItem[]>([])
  const [announcements, setAnnouncements] = useState<AnnouncementItem[]>([])
  const aliveRef = useRef(true)
  useEffect(
    () => () => {
      aliveRef.current = false
    },
    [],
  )

  const load = useCallback(() => {
    setLoading(true)
    Promise.all([fetchChangelogs({ page: 1, pagesize: 20 }), fetchAnnouncements()])
      .then(([logPage, notices]) => {
        if (!aliveRef.current) {
          return
        }
        setChangelogs(logPage?.list ?? [])
        setAnnouncements(notices ?? [])
      })
      .catch((error: unknown) => {
        if (aliveRef.current) {
          toast.error(error instanceof Error ? error.message : t('base_error_message'))
        }
      })
      .finally(() => {
        if (aliveRef.current) {
          setLoading(false)
        }
      })
  }, [t, toast])

  useEffect(() => {
    load()
  }, [load])

  /* ------------------------------ 更新日志编辑 ------------------------------ */
  interface ChangelogForm {
    id?: number
    version: string
    title: string
    body: string
    releasedAt: string
  }
  const emptyChangelog: ChangelogForm = { version: '', title: '', body: '', releasedAt: '' }
  const [logForm, setLogForm] = useState<ChangelogForm | null>(null)

  const saveChangelog = async () => {
    if (!logForm || !logForm.title.trim()) {
      return
    }
    try {
      await saveAdminChangelog({
        id: logForm.id,
        version: logForm.version.trim(),
        title: logForm.title.trim(),
        body: logForm.body.trim(),
        releasedAt: logForm.releasedAt.trim() || undefined,
      })
      toast.success(t('message.updateSuccess'))
      setLogForm(null)
      load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const removeChangelog = async (item: ChangelogItem) => {
    const ok = await confirm({ title: t('changelog.delete'), message: item.title, tone: 'danger', confirmText: t('label.delete') })
    if (!ok) {
      return
    }
    try {
      await deleteAdminChangelog(item.id)
      load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  /* ------------------------------ 公告编辑 ------------------------------ */
  interface NoticeForm {
    id?: number
    type: AnnouncementItem['type']
    body: string
    enabled: boolean
  }
  const emptyNotice: NoticeForm = { type: 'support', body: '', enabled: true }
  const [noticeForm, setNoticeForm] = useState<NoticeForm | null>(null)

  const saveNotice = async () => {
    if (!noticeForm || !noticeForm.body.trim()) {
      return
    }
    try {
      await saveAdminAnnouncement({
        id: noticeForm.id,
        type: noticeForm.type,
        body: noticeForm.body.trim(),
        enabled: noticeForm.enabled,
      })
      toast.success(t('message.updateSuccess'))
      setNoticeForm(null)
      load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const removeNotice = async (item: AnnouncementItem) => {
    const ok = await confirm({ title: t('announcement.delete'), message: item.body, tone: 'danger', confirmText: t('label.delete') })
    if (!ok) {
      return
    }
    try {
      await deleteAdminAnnouncement(item.id)
      load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  if (loading) {
    return <Loading />
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__head">
          <div className="section__title">{t('changelog.title')}</div>
          <Button variant="contained" onClick={() => setLogForm({ ...emptyChangelog })}>
            {t('changelog.create')}
          </Button>
        </div>
        <div className="stack">
          {changelogs.length === 0 ? <div className="muted">{t('changelog.empty')}</div> : null}
          {changelogs.map((item) => (
            <div className="ref-row" key={item.id}>
              <span className="ref-row__name">
                {item.releasedAt ? <span className="muted">{item.releasedAt} · </span> : null}
                {item.title}
                {item.version ? <Tag tone="primary">{item.version}</Tag> : null}
              </span>
              <span className="ref-row__actions">
                <Button
                  size="sm"
                  variant="text"
                  onClick={() =>
                    setLogForm({
                      id: item.id,
                      version: item.version,
                      title: item.title,
                      body: item.body,
                      releasedAt: item.releasedAt ?? '',
                    })
                  }
                >
                  {t('label.edit')}
                </Button>
                <Button size="sm" variant="text" tone="danger" onClick={() => removeChangelog(item)}>
                  {t('label.delete')}
                </Button>
              </span>
            </div>
          ))}
        </div>
      </section>

      <section className="section">
        <div className="section__head">
          <div className="section__title">{t('announcement.title')}</div>
          <Button variant="contained" onClick={() => setNoticeForm({ ...emptyNotice })}>
            {t('announcement.create')}
          </Button>
        </div>
        <div className="stack">
          {announcements.length === 0 ? <div className="muted">{t('announcement.empty')}</div> : null}
          {announcements.map((item) => (
            <div className="ref-row" key={item.id}>
              <span className="ref-row__name">
                <Tag tone={item.type === 'security' ? 'danger' : item.type === 'privacy' ? 'warning' : 'primary'}>
                  {t(`announcement.type.${item.type}`)}
                </Tag>
                {item.body}
              </span>
              <span className="ref-row__actions">
                <Switch checked={item.enabled} onChange={() => setNoticeForm({ id: item.id, type: item.type, body: item.body, enabled: !item.enabled })} />
                <Button size="sm" variant="text" onClick={() => setNoticeForm({ id: item.id, type: item.type, body: item.body, enabled: item.enabled })}>
                  {t('label.edit')}
                </Button>
                <Button size="sm" variant="text" tone="danger" onClick={() => removeNotice(item)}>
                  {t('label.delete')}
                </Button>
              </span>
            </div>
          ))}
        </div>
      </section>

      <Dialog open={logForm !== null} title={logForm?.id ? t('changelog.edit') : t('changelog.create')} onClose={() => setLogForm(null)}>
        <FormGrid>
          <FormItem label={t('changelog.version')}>
            <TextField value={logForm?.version ?? ''} onChange={(event) => setLogForm((prev) => (prev ? { ...prev, version: event.target.value } : prev))} />
          </FormItem>
          <FormItem label={t('changelog.releasedAt')} help="YYYY-MM-DD">
            <TextField
              value={logForm?.releasedAt ?? ''}
              placeholder="2026-01-01"
              onChange={(event) => setLogForm((prev) => (prev ? { ...prev, releasedAt: event.target.value } : prev))}
            />
          </FormItem>
        </FormGrid>
        <FormItem label={t('changelog.formTitle')} required>
          <TextField value={logForm?.title ?? ''} onChange={(event) => setLogForm((prev) => (prev ? { ...prev, title: event.target.value } : prev))} />
        </FormItem>
        <FormItem label={t('changelog.formBody')}>
          <TextareaField value={logForm?.body ?? ''} onChange={(event) => setLogForm((prev) => (prev ? { ...prev, body: event.target.value } : prev))} />
        </FormItem>
        <FormActions>
          <Button variant="text" onClick={() => setLogForm(null)}>
            {t('label.cancel')}
          </Button>
          <Button variant="contained" onClick={saveChangelog}>
            {t('label.save')}
          </Button>
        </FormActions>
      </Dialog>

      <Dialog open={noticeForm !== null} title={noticeForm?.id ? t('announcement.edit') : t('announcement.create')} onClose={() => setNoticeForm(null)}>
        <FormItem label={t('announcement.formType')}>
          <SelectField
            value={noticeForm?.type ?? 'support'}
            onChange={(event) => setNoticeForm((prev) => (prev ? { ...prev, type: event.target.value as AnnouncementItem['type'] } : prev))}
            options={[
              { value: 'support', label: t('announcement.type.support') },
              { value: 'security', label: t('announcement.type.security') },
              { value: 'privacy', label: t('announcement.type.privacy') },
            ]}
          />
        </FormItem>
        <FormItem label={t('announcement.formBody')} required>
          <TextareaField
            value={noticeForm?.body ?? ''}
            onChange={(event) => setNoticeForm((prev) => (prev ? { ...prev, body: event.target.value } : prev))}
          />
        </FormItem>
        <FormItem label={t('announcement.enabled')}>
          <Switch
            checked={noticeForm?.enabled ?? true}
            onChange={(event) => setNoticeForm((prev) => (prev ? { ...prev, enabled: event.target.checked } : prev))}
          />
        </FormItem>
        <FormActions>
          <Button variant="text" onClick={() => setNoticeForm(null)}>
            {t('label.cancel')}
          </Button>
          <Button variant="contained" onClick={saveNotice}>
            {t('label.save')}
          </Button>
        </FormActions>
      </Dialog>
    </div>
  )
}
