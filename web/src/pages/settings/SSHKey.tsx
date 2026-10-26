import { useCallback, useEffect, useState } from 'react'

import { addSSHKey, listSSHKeys, removeSSHKey } from '@/api/user'
import type { SSHKey as SSHKeyType } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { useConfirm } from '@/components/unit/Confirm'
import { DataTable } from '@/components/unit/Table'
import { Form, FormActions, FormItem } from '@/components/unit/Form'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { TextareaField, TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useT } from '@/i18n'


/**
 * 旧版 UserSettingSSHKey.js:69 —— 整行公钥解析：
 * 第 1 组是密钥类型，第 2 组是**公钥本体**（提交给后端的就是它），第 3 组是注释。
 * 后端 `user/addSSHKey` 拒绝含空白的 key（user_profile.go:54），所以剥离必须在前端做。
 */
const SSH_KEY_PATTERN = /^((?:ssh|ecdsa)-[\w\d]+)\s+(\S+)(?:\s+(\S+))?\s*$/

/** 旧版 SSHKeyItem.js:44 —— 每 2 个十六进制字符插一个冒号，并去掉末尾多余的冒号。 */
function formatFingerprint(hash: string): string {
  return hash.replace(/([a-f0-9]{2})/gi, '$1:').slice(0, -1)
}

/**
 * SSH 公钥设置（`/settings/sshkey`）。
 *
 * 端点：`user/listKey`（GET）、`user/addSSHKey`（`name` + `key`）、`user/removeSSHKey`（`id`）。
 * 提交前先剥离整行公钥里的注释，只把公钥本体交给后端；注释在用户没填名称时用作默认名称。
 */
export default function SSHKeySetting() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()

  const [keys, setKeys] = useState<SSHKeyType[]>([])
  const [loading, setLoading] = useState(false)
  const [name, setName] = useState('')
  const [key, setKey] = useState('')
  /** 用户是否手动改过名称：改过之后就不再用注释覆盖（旧的 UserSettingSSHKey.js:198-203 是直接覆盖）。 */
  const [nameTouched, setNameTouched] = useState(false)
  const [saving, setSaving] = useState(false)

  const reload = useCallback(() => {
    setLoading(true)
    listSSHKeys()
      .then(setKeys)
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => setLoading(false))
  }, [t, toast])

  useEffect(() => {
    reload()
  }, [reload])

  const onKeyChange = (value: string) => {
    setKey(value)
    const matched = value.trim().match(SSH_KEY_PATTERN)
    const comment = matched?.[3]
    // 注释存在且用户没填名称时，用注释预填名称
    if (comment && !nameTouched) {
      setName(comment)
    }
  }

  const submit = async () => {
    const matched = key.trim().match(SSH_KEY_PATTERN)
    if (!matched || !matched[2]) {
      toast.error(t('message.error._S_invalid', { s: t('label.SSHKey') }))
      return
    }
    const publicKey = matched[2]
    if (!name.trim()) {
      toast.error(t('message.error._S_empty', { s: t('label.name') }))
      return
    }
    // 旧版 UserSettingSSHKey.js:83-90：名称 5~30 字
    if (name.trim().length < 5 || name.trim().length > 30) {
      toast.error(t('message.error.within_N1_to_N2_characters', { n1: 5, n2: 30 }))
      return
    }
    setSaving(true)
    try {
      // 只提交公钥本体（不含注释），后端对含空白的 key 会直接拒绝
      await addSSHKey(name.trim(), publicKey)
      toast.success(t('message.addSuccess'))
      setName('')
      setKey('')
      setNameTouched(false)
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSaving(false)
    }
  }

  const remove = async (item: SSHKeyType) => {
    const ok = await confirm({
      title: t('label.delete'),
      message: `${item.name}`,
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    try {
      await removeSSHKey(item.id)
      toast.success(t('message.deleteSuccess'))
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.SSHKey')}</div>
        <DataTable
          columns={[
            { key: 'name', title: t('label.name') },
            { key: 'type', title: 'type' },
            // 旧版 SSHKeyItem.js:44 显示公钥指纹
            {
              key: 'hash',
              title: t('label.fingerprint'),
              render: (row) => <span className="muted">{formatFingerprint(row.hash ?? '')}</span>,
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
                <Button size="sm" variant="text" tone="danger" onClick={() => remove(row)}>
                  {t('label.delete')}
                </Button>
              ),
            },
          ]}
          rows={keys}
          rowKey={(row) => row.id}
          loading={loading}
          emptyTitle={t('message._S_listEmpty', { s: t('label.SSHKey') })}
        />
      </section>

      <section className="section">
        <div className="section__title">{t('label.addSSHKey')}</div>
        <Form onSubmit={submit}>
          <FormItem label={t('label.name')} required>
            <TextField
              value={name}
              onChange={(event) => {
                setName(event.target.value)
                setNameTouched(true)
              }}
              placeholder="id_ed25519"
            />
          </FormItem>
          <FormItem label={t('label.SSHKey')} required help={t('message.inputSSHKEYContent')}>
            <TextareaField
              value={key}
              onChange={(event) => onKeyChange(event.target.value)}
              rows={4}
              placeholder="ssh-rsa AAAA... user@host"
            />
          </FormItem>
          <FormActions>
            <Button variant="contained" type="submit" loading={saving}>
              {t('label.addSSHKey')}
            </Button>
          </FormActions>
        </Form>
      </section>
    </div>
  )
}
