import { useCallback, useEffect, useState } from 'react'

import { createAccessToken, deleteAccessToken, listAccessTokens } from '@/api/user'
import type { AccessToken } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { CodeBlock } from '@/components/unit/CodeBlock'
import { useConfirm } from '@/components/unit/Confirm'
import { Dialog } from '@/components/unit/Dialog'
import { Form, FormActions, FormItem } from '@/components/unit/Form'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { DataTable } from '@/components/unit/Table'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { PUBLIC_API_DOC_PATH } from '@/config/publicApi'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { docUrl } from '@/helpers/docUrl'
import { useI18n, useT } from '@/i18n'


/** 令牌名长度上限，与后端 `handlers/user_token.go` 的 `tokenNameMaxLength` 保持一致。 */
const NAME_MAX_LENGTH = 64

/**
 * 访问令牌设置（`/settings/tokens`）。
 *
 * 这条能力以前是**死的**：公开 API（`/api/v1`）与令牌校验 2026-10-25 就落地了，
 * 但没有任何界面能创建令牌，用户只能拿浏览器 Cookie 调接口。本页把它接通：
 * 列表 / 创建（明文只返回一次）/ 吊销。
 *
 * 安全提示必须写死在页面上，不能只靠文档：
 *   - 明文**只显示一次**，后端只存 SHA-256；
 *   - 令牌 ≡ 用户，拥有我全部权限（scope 尚未实现），泄露等于泄露账号。
 */
export default function TokensSetting() {
  const t = useT()
  const { lang } = useI18n()
  const toast = useToast()
  const confirm = useConfirm()

  const { user } = useAppData()
  const [tokens, setTokens] = useState<AccessToken[]>([])
  const [loading, setLoading] = useState(false)
  const [name, setName] = useState('')
  const [creating, setCreating] = useState(false)
  /** 刚创建的明文：只在这一次弹窗里存在，关闭即丢弃。 */
  const [created, setCreated] = useState<string | null>(null)

  const reload = useCallback(() => {
    setLoading(true)
    listAccessTokens()
      .then(setTokens)
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => setLoading(false))
  }, [t, toast])

  useEffect(() => {
    reload()
  }, [reload])

  const submit = async () => {
    const trimmed = name.trim()
    if (!trimmed) {
      toast.error(t('message.error._S_empty', { s: t('label.name') }))
      return
    }
    if (trimmed.length > NAME_MAX_LENGTH) {
      toast.error(t('message.error.within_N1_to_N2_characters', { n1: 1, n2: NAME_MAX_LENGTH }))
      return
    }
    setCreating(true)
    try {
      const result = await createAccessToken(trimmed)
      setCreated(result.token)
      setName('')
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setCreating(false)
    }
  }

  /** 用法示例：带上真实站点 host，用户复制后只需替换令牌本身。 */
  const curlExample = `curl -H "Authorization: token <token>" ${user?.host ?? ''}/api/v1/user`

  const revoke = async (item: AccessToken) => {
    const ok = await confirm({
      title: t('label.delete'),
      message: item.name,
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    try {
      await deleteAccessToken(item.id)
      toast.success(t('message.deleteSuccess'))
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('tokens.listTitle')}</div>
        <p className="muted">{t('tokens.listNotice')}</p>
        <DataTable
          columns={[
            { key: 'name', title: t('label.name') },
            {
              key: 'lastUsed',
              title: t('tokens.lastUsed'),
              render: (row) =>
                row.lastUsed ? (
                  <FormattedTime timestamp={row.lastUsed} absolute />
                ) : (
                  <span className="muted">{t('tokens.neverUsed')}</span>
                ),
            },
            {
              key: 'created',
              title: t('label.createdAt'),
              render: (row) => <FormattedTime timestamp={row.created} absolute />,
            },
            {
              key: 'ops',
              title: '',
              render: (row) => (
                <Button size="sm" variant="text" tone="danger" onClick={() => revoke(row)}>
                  {t('tokens.revoke')}
                </Button>
              ),
            },
          ]}
          rows={tokens}
          rowKey={(row) => row.id}
          loading={loading}
          emptyTitle={t('message._S_listEmpty', { s: t('tokens.title') })}
          emptyNotice={t('tokens.emptyNotice')}
        />
      </section>

      <section className="section">
        <div className="section__title">{t('tokens.createTitle')}</div>
        <Form onSubmit={submit}>
          <FormItem label={t('label.name')} required help={t('tokens.nameHelp')}>
            <TextField
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="ci-deploy"
              maxLength={NAME_MAX_LENGTH}
            />
          </FormItem>
          <FormActions>
            <Button variant="contained" type="submit" loading={creating}>
              {t('tokens.create')}
            </Button>
          </FormActions>
        </Form>
        <p className="muted">{t('tokens.usageHint')}</p>
        <CodeBlock code={curlExample} language="bash" />
        <p className="muted">
          {t('tokens.docHint')}{' '}
          <a className="cf-link" href={docUrl(lang, PUBLIC_API_DOC_PATH)}>
            {t('tokens.docLink')}
          </a>
        </p>
      </section>

      {/* 明文只出现这一次：弹窗不提供"稍后再看"，关掉就等于丢失。 */}
      <Dialog
        open={created !== null}
        title={t('tokens.createdTitle')}
        onClose={() => setCreated(null)}
        actions={
          <Button variant="contained" onClick={() => setCreated(null)}>
            {t('base_confirm')}
          </Button>
        }
      >
        <p>{t('tokens.createdNotice')}</p>
        <CodeBlock code={created ?? ''} language="text" />
      </Dialog>
    </div>
  )
}
