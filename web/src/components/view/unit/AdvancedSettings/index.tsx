import { useEffect, useState, type ReactNode } from 'react'

import { ApiError } from '@/api/client'
import { Button } from '@/components/unit/Button'
import { useConfirm } from '@/components/unit/Confirm'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { SelectField } from '@/components/unit/SelectField'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { REPO_SLUG_RE, makeSlug } from '@/helpers/slug'
import { useT } from '@/i18n'


export interface AdvancedSettingsProps {
  /** 当前标识名（用于"改名"表单的初始值）。 */
  currentName: string
  /** 改名的回调（由页面注入具体端点）。 */
  onRename: (name: string) => Promise<void>
  /** 删除的回调。 */
  onDelete: () => Promise<void>
  /** 删除确认文案（仓库/组各自的措辞）。 */
  deleteMessage: string
  /** 删除按钮文案。 */
  deleteLabel: string
  /** 禁止删除时的说明（存在时删除按钮禁用，如个人组或非空仓库组）。 */
  deleteBlockedReason?: string
  /** 名称字段的标签（仓库 slug / 组 slug），用于校验错误文案。 */
  nameLabel?: string
  /** 名称输入框的前缀提示（如 `https://host/group/`）。 */
  namePrefix?: string
  /** 自定义 slug 校验器（返回错误文案或 null）。 */
  validateSlug?: (value: string) => string | null
  /** 改名前的确认（给了就弹；取消则不提交）。 */
  renameConfirm?: { title: string; message: string }
  /** 删除前的第一级确认（给了就弹第一级）。 */
  deleteConfirm?: { title: string; message: string; requireInput?: string; requireInputPrompt?: ReactNode }
  /** 删除前的第二级确认（给了就弹第二级）。 */
  deleteSecondConfirm?: { title: string; message: string; requireInput?: string; requireInputPrompt?: ReactNode }
  /** 创建者下拉（仓库有，组没有）。`onChange` 返回 false 表示用户取消。 */
  owner?: {
    label: string
    currentId: string
    options: Array<{ value: string; label: ReactNode }>
    onChange: (userID: string) => Promise<boolean>
  }
}

/**
 * 高级设置（改标识名 + 创建者 + 危险操作区）。
 *
 * 仓库与仓库组的高级页结构一致（AGENTS §4），抽成共用组件；
 * 支持异步同步 `currentName`、中划线 slug 校验、禁止删除提示与输入路径确认（DESIGN.md G9）。
 */
export function AdvancedSettings({
  currentName,
  onRename,
  onDelete,
  deleteMessage,
  deleteLabel,
  deleteBlockedReason,
  nameLabel,
  namePrefix,
  validateSlug,
  renameConfirm,
  deleteConfirm,
  deleteSecondConfirm,
  owner,
}: AdvancedSettingsProps) {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()

  const [name, setName] = useState(currentName)
  const [nameError, setNameError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setName(currentName)
    setNameError(null)
  }, [currentName])

  const rename = async () => {
    const label = nameLabel ?? t('label.name')
    const trimmed = name.trim()
    if (!trimmed) {
      setNameError(t('message.error._S_empty', { s: label }))
      return
    }
    if (validateSlug) {
      const err = validateSlug(trimmed)
      if (err) {
        setNameError(err)
        return
      }
    } else if (!REPO_SLUG_RE.test(trimmed)) {
      setNameError(t('repo.slugRule'))
      return
    }
    if (trimmed === currentName) {
      return
    }
    if (renameConfirm) {
      const ok = await confirm({
        title: renameConfirm.title,
        message: renameConfirm.message,
        tone: 'danger',
        confirmText: t('base_confirm'),
      })
      if (!ok) {
        return
      }
    }
    setNameError(null)
    setSaving(true)
    try {
      await onRename(trimmed)
      toast.success(t('message.updateSuccess'))
    } catch (error) {
      if (error instanceof ApiError && (error.code === 0x0401 || error.code === 0x0403)) {
        setNameError(error.code === 0x0403 ? t('group.slugDuplicate') : t('repo.slugDuplicate'))
      } else {
        toast.error(error instanceof Error ? error.message : t('base_error_message'))
      }
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    if (deleteBlockedReason) {
      return
    }
    if (deleteConfirm) {
      const first = await confirm({
        title: deleteConfirm.title,
        message: deleteConfirm.message,
        tone: 'danger',
        confirmText: t('base_confirm'),
        requireInput: deleteConfirm.requireInput,
        requireInputPrompt: deleteConfirm.requireInputPrompt,
      })
      if (!first) {
        return
      }
    }
    if (deleteSecondConfirm) {
      const second = await confirm({
        title: deleteSecondConfirm.title,
        message: deleteSecondConfirm.message,
        tone: 'danger',
        confirmText: t('base_confirm'),
        requireInput: deleteSecondConfirm.requireInput,
        requireInputPrompt: deleteSecondConfirm.requireInputPrompt,
      })
      if (!second) {
        return
      }
    }
    try {
      await onDelete()
      toast.success(t('message.deleteSuccess'))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const changeOwner = async (userID: string) => {
    if (!owner || userID === owner.currentId) {
      return
    }
    try {
      const changed = await owner.onChange(userID)
      if (changed) {
        toast.success(t('message.updateSuccess'))
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  return (
    <div className="page">
      {owner ? (
        <section className="section">
          <div className="section__title">{owner.label}</div>
          <FormGrid>
            <SelectField
              options={owner.options}
              value={owner.currentId}
              onChange={(event) => void changeOwner(event.target.value)}
            />
          </FormGrid>
        </section>
      ) : null}

      <section className="section">
        <div className="section__title">{t('label.updateName')}</div>
        <Form onSubmit={rename}>
          <FormItem label={t('label.name')} error={nameError} help={t('message.nameUsedInURL')}>
            <TextField
              value={name}
              startAdornment={namePrefix ? <span className="muted">{namePrefix}</span> : undefined}
              onChange={(event) => {
                setName(makeSlug(event.target.value))
                setNameError(null)
              }}
            />
          </FormItem>
          <FormActions>
            <Button type="submit" loading={saving} disabled={name.trim() === currentName}>
              {t('label.save')}
            </Button>
          </FormActions>
        </Form>
      </section>

      <section className="section danger-zone">
        <div className="section__title danger-zone__title">{t('label.dangerOperation')}</div>
        <div className="danger-zone__row">
          <div className="danger-zone__text">
            <div>{deleteMessage}</div>
            {deleteBlockedReason ? <div className="danger-zone__blocked">{deleteBlockedReason}</div> : null}
          </div>
          <Button tone="danger" variant="outlined" disabled={Boolean(deleteBlockedReason)} onClick={remove}>
            {deleteLabel}
          </Button>
        </div>
      </section>
    </div>
  )
}
