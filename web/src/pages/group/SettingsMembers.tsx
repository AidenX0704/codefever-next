import { addGroupMember, changeGroupMemberRole, changeGroupOwner, removeGroupMember } from '@/api/group'
import { useConfirm } from '@/components/unit/Confirm'
import { useToast } from '@/components/unit/Toast'
import { MembersManager } from '@/components/view/unit/MembersManager'
import { PermissionCode, hasPermission } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useGroup } from '@/features/group/GroupProvider'
import { useT } from '@/i18n'

/**
 * 仓库组成员（`/groups/{组名}/members`）。
 *
 * 与仓库成员页共用 `MembersManager`，只替换端点（`group/*`）与权限码
 * （`GROUP_CHANGE_MEMBER`）。
 */
export default function GroupSettingsMembers() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { summary, config, refresh } = useGroup()
  const { refresh: refreshAppData } = useAppData()

  const canManage = summary ? hasPermission(summary.role, PermissionCode.GROUP_CHANGE_MEMBER) : false
  const afterChange = () => {
    refresh()
    refreshAppData()
  }

  return (
    <MembersManager
      members={config?.members ?? []}
      canManage={canManage}
      onAdd={async (email) => {
        if (!summary) {
          return
        }
        try {
          await addGroupMember(summary.id, email)
          toast.success(t('message.addSuccess'))
          afterChange()
        } catch (error) {
          toast.error(error instanceof Error ? error.message : t('base_error_message'))
        }
      }}
      onChangeRole={async (userID, roleID) => {
        if (!summary) {
          return
        }
        try {
          await changeGroupMemberRole(summary.id, userID, roleID)
          toast.success(t('message.updateSuccess'))
          afterChange()
        } catch (error) {
          toast.error(error instanceof Error ? error.message : t('base_error_message'))
        }
      }}
      onRemove={async (userID, name) => {
        if (!summary) {
          return
        }
        const ok = await confirm({ title: t('label.delete'), message: name, tone: 'danger', confirmText: t('base_confirm') })
        if (!ok) {
          return
        }
        try {
          await removeGroupMember(summary.id, userID)
          toast.success(t('message.deleteSuccess'))
          afterChange()
        } catch (error) {
          toast.error(error instanceof Error ? error.message : t('base_error_message'))
        }
      }}
      onTransfer={async (userID, name) => {
        if (!summary) {
          return
        }
        const ok = await confirm({ title: t('label.changeOwner'), message: name, tone: 'danger', confirmText: t('base_confirm') })
        if (!ok) {
          return
        }
        try {
          await changeGroupOwner(summary.id, userID)
          toast.success(t('message.updateSuccess'))
          afterChange()
        } catch (error) {
          toast.error(error instanceof Error ? error.message : t('base_error_message'))
        }
      }}
    />
  )
}
