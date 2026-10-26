import { useState, type ReactNode } from 'react'

import { ApiError } from '@/api/client'
import { Button } from '@/components/unit/Button'
import { useConfirm } from '@/components/unit/Confirm'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { SelectField } from '@/components/unit/SelectField'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useT } from '@/i18n'


/** 标识名（slug）长度上限（旧版 `RepositorySettingAdvanced.js` 的校验器）。 */
const NAME_MAX = 30

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
  /** 名称字段的标签（仓库 slug / 组 slug），用于校验错误文案。 */
  nameLabel?: string
  /** 名称输入框的前缀提示（如 `https://host/group/`）。 */
  namePrefix?: string
  /** 改名前的确认（给了就弹；取消则不提交）。 */
  renameConfirm?: { title: string; message: string }
  /** 删除前的第一级确认（给了就弹第一级）。 */
  deleteConfirm?: { title: string; message: string }
  /** 删除前的第二级确认（给了就弹第二级 —— 旧版仓库删除是两级确认）。 */
  deleteSecondConfirm?: { title: string; message: string }
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
 * 仓库与仓库组的高级页结构一致（AGENTS §4 的第二处重复），抽成组件；
 * 页面只注入端点与文案。删除按传入的确认文案走**一级或两级**确认。
 */
export function AdvancedSettings({
  currentName,
  onRename,
  onDelete,
  deleteMessage,
  deleteLabel,
  nameLabel,
  namePrefix,
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

  const rename = async () => {
    const label = nameLabel ?? t('label.name')
    if (!name.trim()) {
      setNameError(t('message.error._S_empty', { s: label }))
      return
    }
    if (!/^[0-9a-zA-Z_]+$/.test(name.trim())) {
      setNameError(t('message.error.requireCombinationOfCharactersNumbersAndUnderscore'))
      return
    }
    if (name.trim().length > NAME_MAX) {
      setNameError(t('message.error.noMoreThan_N_characters', { n: NAME_MAX }))
      return
    }
    if (name.trim() === currentName) {
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
      await onRename(name.trim())
      toast.success(t('message.updateSuccess'))
    } catch (error) {
      // 0x0401：仓库标识串重复。
      if (error instanceof ApiError && error.code === 0x0401) {
        setNameError(t('message.error._S_duplicate', { s: label }))
      } else {
        toast.error(error instanceof Error ? error.message : t('base_error_message'))
      }
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    if (deleteConfirm) {
      const first = await confirm({
        title: deleteConfirm.title,
        message: deleteConfirm.message,
        tone: 'danger',
        confirmText: t('base_confirm'),
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
              onChange={(event) => changeOwner(event.target.value)}
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
                setName(event.target.value)
                setNameError(null)
              }}
            />
          </FormItem>
          <FormActions>
            <Button type="submit" loading={saving}>
              {t('label.save')}
            </Button>
          </FormActions>
        </Form>
      </section>

      <section className="section">
        <div className="section__title">{t('label.advanced')}</div>
        <div className="row">
          <span className="muted">{deleteMessage}</span>
          <Button tone="danger" variant="outlined" onClick={remove}>
            {deleteLabel}
          </Button>
        </div>
      </section>
    </div>
  )
}
