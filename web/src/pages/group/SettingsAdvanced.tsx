import { deleteGroup, updateGroupName } from '@/api/group'
import { AdvancedSettings } from '@/components/view/unit/AdvancedSettings'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useGroup } from '@/features/group/GroupProvider'
import { useT } from '@/i18n'

/**
 * 仓库组设置 · 高级（`/groups/{组名}/settings/advanced`）。
 *
 * `group/updateName`（`group` + `name`）与 `group/deleteGroup`（`group`）。
 */
export default function GroupSettingsAdvanced() {
  const t = useT()
  const { summary, config } = useGroup()
  const { refresh } = useAppData()

  return (
    <AdvancedSettings
      currentName={config?.group.name ?? ''}
      deleteLabel={t('label.deleteGroup')}
      deleteMessage={t('message.deleteGroupWarning')}
      onRename={async (name) => {
        if (!summary) {
          return
        }
        await updateGroupName(summary.id, name)
        refresh()
      }}
      onDelete={async () => {
        if (!summary) {
          return
        }
        await deleteGroup(summary.id)
        window.location.href = '/groups'
      }}
    />
  )
}
