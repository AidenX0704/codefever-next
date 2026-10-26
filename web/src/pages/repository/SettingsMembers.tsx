import { ApiError } from '@/api/client'
import { addRepositoryMember, changeRepositoryMemberRole, changeRepositoryOwner, removeRepositoryMember } from '@/api/repository'
import { useConfirm } from '@/components/unit/Confirm'
import { useToast } from '@/components/unit/Toast'
import { MembersManager } from '@/components/view/unit/MembersManager'
import { PermissionCode, Role, hasPermission } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'

/**
 * 仓库成员（`/{组}/{仓库}/members`）。
 *
 * 列表与交互由 `MembersManager` 承担（与仓库组成员页共用），这里只注入
 * 仓库域的四个端点与"是否可管理"的判定。
 *
 * 与旧版 `RepositorySettingMembers.js` 对齐的三点：
 *   1. **仓库成员 / 仓库组成员分两表**（后端 `groupMember` 字段）；
 *   2. **过滤已删除成员**（`deleteFlag`，在 `MembersManager` 里做）；
 *   3. **自身权限粗粒度判定**：旧版 `MemberList.js:96` 是 `role < MAINTAINER` 整体禁用，
 *      这里保留这道粗粒度闸门（与 `REPO_CHANGE_MEMBER` 权限取交集，语义更严）。
 */
export default function RepositoryMembers() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { summary, config, refresh } = useRepository()
  const { refresh: refreshAppData } = useAppData()

  const canManage =
    summary
      ? hasPermission(summary.role, PermissionCode.REPO_CHANGE_MEMBER) && summary.role >= Role.MAINTAINER
      : false
  const afterChange = () => {
    refresh()
    refreshAppData()
  }

  return (
    <MembersManager
      members={(config?.members ?? [])}
      canManage={canManage}
      splitGroupMembers
      onAdd={async (email) => {
        if (!summary) {
          return
        }
        try {
          await addRepositoryMember(summary.id, email)
          toast.success(t('message.addSuccess'))
          afterChange()
        } catch (error) {
          // 0x0407：按邮箱找不到用户；0x0408：不能把创建者加成普通成员。
          if (error instanceof ApiError && error.code === 0x0407) {
            toast.error(t('message.error._S_notFound', { s: t('label.email') }))
          } else if (error instanceof ApiError && error.code === 0x0408) {
            toast.error(t('message.error.canNotAddCreatorAsMember'))
          } else {
            toast.error(error instanceof Error ? error.message : t('base_error_message'))
          }
        }
      }}
      onChangeRole={async (userID, roleID) => {
        if (!summary) {
          return
        }
        try {
          await changeRepositoryMemberRole(summary.id, userID, roleID)
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
          await removeRepositoryMember(summary.id, userID)
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
          await changeRepositoryOwner(summary.id, userID)
          toast.success(t('message.updateSuccess'))
          afterChange()
        } catch (error) {
          toast.error(error instanceof Error ? error.message : t('base_error_message'))
        }
      }}
    />
  )
}
