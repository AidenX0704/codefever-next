import { useNavigate } from 'react-router'

import { GROUP_TYPE, deleteGroup, updateGroupName } from '@/api/group'
import { AdvancedSettings } from '@/components/view/unit/AdvancedSettings'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useGroup } from '@/features/group/GroupProvider'
import { GROUP_SLUG_RE, isReservedGroupSlug } from '@/helpers/slug'
import { useT } from '@/i18n'


/**
 * 仓库组设置 · 高级（`/groups/{组名}/settings/advanced`）。
 * 嵌入在 GroupLayout 的右侧主舞台，由左侧空间栏直达。
 */
export default function GroupSettingsAdvanced() {
  const t = useT()
  const navigate = useNavigate()
  const { summary, config } = useGroup()
  const { user, refresh } = useAppData()

  const group = config?.group
  const repoCount = config?.count.repository ?? summary?.repositoryCount ?? 0
  const host = (user?.host ?? window.location.origin).replace(/\/+$/, '')

  let deleteBlockedReason: string | undefined
  if (group?.type === GROUP_TYPE.user) {
    deleteBlockedReason = t('group.deleteBlockedPersonal')
  } else if (repoCount > 0) {
    deleteBlockedReason = t('group.deleteBlockedNonEmpty', { n: repoCount })
  }

  return (
    <div className="stage-stack">
      <div className="form-sheet form-sheet--full">
        <AdvancedSettings
          currentName={group?.name ?? ''}
          nameLabel={t('label.groupURL')}
          namePrefix={`${host}/`}
          validateSlug={(value) => {
            if (!GROUP_SLUG_RE.test(value)) {
              return t('group.slugRule')
            }
            if (isReservedGroupSlug(value)) {
              return t('group.slugReserved')
            }
            return null
          }}
          renameConfirm={{
            title: t('label.dangerOperation'),
            message: t('message.nameUsedInURL'),
          }}
          deleteLabel={t('label.deleteGroup')}
          deleteMessage={t('message.deleteGroupWarning')}
          deleteBlockedReason={deleteBlockedReason}
          deleteConfirm={
            group
              ? {
                  title: t('label.deleteGroup'),
                  message: t('message.deleteGroupWarning'),
                  requireInput: group.name,
                  requireInputPrompt: t('group.deleteConfirmInput', { s: group.name }),
                }
              : undefined
          }
          onRename={async (nextName) => {
            if (!summary) {
              return
            }
            await updateGroupName(summary.id, nextName)
            await refresh()
            navigate(`/groups/${nextName}/settings/advanced`, { replace: true })
          }}
          onDelete={async () => {
            if (!summary) {
              return
            }
            await deleteGroup(summary.id)
            await refresh()
            navigate('/groups', { replace: true })
          }}
        />
      </div>
    </div>
  )
}
