import { useNavigate } from 'react-router'

import { changeRepositoryOwner, deleteRepository, updateRepositoryName } from '@/api/repository'
import { useConfirm } from '@/components/unit/Confirm'
import { useToast } from '@/components/unit/Toast'
import { AdvancedSettings } from '@/components/view/unit/AdvancedSettings'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'

/**
 * 仓库设置 · 高级（`/{组}/{仓库}/settings/advanced`）。
 *
 * `repository/updateName`（`repository` + `name`）、`repository/changeOwner`（`repository` + `userID`）、
 * `repository/deleteRepository`（`repository`）。
 * 结构与仓库组高级页共用 `AdvancedSettings`。
 *
 * 与旧版 `RepositorySettingAdvanced.js` 对齐：**更新创建者下拉**、**slug 校验 + 0x0401 映射**、
 * **删除仓库两级确认**。
 */
export default function RepositorySettingsAdvanced() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const navigate = useNavigate()
  const { summary, config, refresh } = useRepository()
  const { refresh: refreshAppData, user } = useAppData()

  return (
    <AdvancedSettings
      currentName={config?.repository.name ?? ''}
      nameLabel={t('label.repositorySlug')}
      namePrefix={`${user?.host ?? ''}/${config?.group.name ?? ''}/`}
      renameConfirm={{
        title: t('label.dangerOperation'),
        message: t('message.repositoryChangeURLNotice'),
      }}
      deleteLabel={t('label.deleteRepository')}
      deleteMessage={t('message.deleteRepositoryWarning')}
      deleteConfirm={{
        title: t('label.dangerOperation'),
        message: t('message.repositoryDeleteRepositoryNotice'),
      }}
      deleteSecondConfirm={{
        title: t('label.dangerOperationConfirm'),
        message: t('message.repositoryDeleteRepositoryConfirmNotice'),
      }}
      owner={{
        label: t('label.updateCreator'),
        currentId: config?.repository.owner ?? '',
        options: (config?.members ?? []).map((member) => ({ value: member.id, label: member.name })),
        onChange: async (userID) => {
          if (!summary) {
            return false
          }
          const ok = await confirm({
            title: t('label.dangerOperation'),
            message: t('message.repositoryChangeCreatorNotice'),
            tone: 'danger',
            confirmText: t('base_confirm'),
          })
          if (!ok) {
            return false
          }
          try {
            await changeRepositoryOwner(summary.id, userID)
            toast.success(t('message.updateSuccess'))
            refresh()
            refreshAppData()
            return true
          } catch (error) {
            toast.error(error instanceof Error ? error.message : t('base_error_message'))
            return false
          }
        },
      }}
      onRename={async (name) => {
        if (!summary) {
          return
        }
        await updateRepositoryName(summary.id, name)
        refresh()
        refreshAppData()
      }}
      onDelete={async () => {
        if (!summary) {
          return
        }
        await deleteRepository(summary.id)
        navigate('/repositories', { replace: true })
      }}
    />
  )
}
