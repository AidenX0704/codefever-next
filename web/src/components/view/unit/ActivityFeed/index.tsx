import type { ActivityItem } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { useT } from '@/i18n'


/** 活动码 → 文案键与"主体"来源。逐字对齐旧 `config/Activity.js` 的 41 个分支。 */
type SubjectSource = 'group' | 'repository' | 'repoPath' | 'contentName' | 'none'

interface ActivitySpec {
  key: string
  subject: SubjectSource
  detail?: 'fromTo' | 'uid' | 'reviewer' | 'name' | 'mergeRequest'
}

const ACTIVITY_SPECS: Record<number, ActivitySpec> = {
  0x0201: { key: 'message.activity.createdGroup', subject: 'contentName' },
  0x0202: { key: 'message.activity.updateGroup_S_Avator', subject: 'group' },
  0x0203: { key: 'message.activity.updateGroup_S_Name', subject: 'group', detail: 'fromTo' },
  0x0204: { key: 'message.activity.updateGroup_S_Description', subject: 'group' },
  0x0205: { key: 'message.activity.addGroup_S_Memeber', subject: 'group', detail: 'uid' },
  0x0206: { key: 'message.activity.changeGroup_S_MemebrRole', subject: 'group', detail: 'uid' },
  0x0207: { key: 'message.activity.removeGroup_S_Memebr', subject: 'group', detail: 'uid' },
  0x0208: { key: 'message.activity.setGroup_S_Creator', subject: 'group', detail: 'uid' },
  0x0209: { key: 'message.activity.setGroup_S_URL', subject: 'group', detail: 'fromTo' },
  0x0301: { key: 'message.activity.createdRepository', subject: 'contentName' },
  0x0302: { key: 'message.activity.forkRepository', subject: 'contentName' },
  0x0303: { key: 'message.activity.updateRepository_S_Avator', subject: 'repository' },
  0x0304: { key: 'message.activity.updateRepository_S_Name', subject: 'repository', detail: 'fromTo' },
  0x0305: { key: 'message.activity.updateRepository_S_Description', subject: 'repository' },
  0x0306: { key: 'message.activity.addRepository_S_Memeber', subject: 'repository', detail: 'uid' },
  0x0307: { key: 'message.activity.changeRepository_S_MemebrRole', subject: 'repository', detail: 'uid' },
  0x0308: { key: 'message.activity.removeRepository_S_Memebr', subject: 'repository', detail: 'uid' },
  0x0309: { key: 'message.activity.setRepository_S_Creator', subject: 'repository', detail: 'uid' },
  0x030a: { key: 'message.activity.setRepository_S_URL', subject: 'repository', detail: 'fromTo' },
  0x030b: { key: 'message.activity.deleteRepository', subject: 'contentName' },
  0x0401: { key: 'message.activity.pushTo_S_Branch', subject: 'repoPath', detail: 'name' },
  0x0402: { key: 'message.activity.pushTo_S_NewBranch', subject: 'repoPath', detail: 'name' },
  0x0403: { key: 'message.activity.created_S_NewBranch', subject: 'repoPath', detail: 'name' },
  0x0404: { key: 'message.activity.delete_S_Branch', subject: 'repoPath', detail: 'name' },
  0x0405: { key: 'message.activity.update_S_DefaultBranch', subject: 'repoPath', detail: 'name' },
  0x0406: { key: 'message.activity.created_S_ProtectedBranchRule', subject: 'repoPath', detail: 'name' },
  0x0407: { key: 'message.activity.update_S_ProtectedBranchRule', subject: 'repoPath', detail: 'name' },
  0x0408: { key: 'message.activity.delete_S_ProtectedBranchRule', subject: 'repoPath', detail: 'name' },
  0x0501: { key: 'message.activity.pushTo_S_Tag', subject: 'repoPath', detail: 'name' },
  0x0502: { key: 'message.activity.pushTo_S_NewTag', subject: 'repoPath', detail: 'name' },
  0x0503: { key: 'message.activity.created_S_NewTag', subject: 'repoPath', detail: 'name' },
  0x0504: { key: 'message.activity.delete_S_Tag', subject: 'repoPath', detail: 'name' },
  0x0701: { key: 'message.activity.open_S_MergeRquest', subject: 'repoPath', detail: 'mergeRequest' },
  0x0702: { key: 'message.activity.close_S_MergeRquest', subject: 'repoPath', detail: 'mergeRequest' },
  0x0703: { key: 'message.activity.merge_S_MergeRquest', subject: 'repoPath', detail: 'mergeRequest' },
  0x0704: { key: 'message.activity.assign_S_Reviewer', subject: 'repoPath', detail: 'reviewer' },
  0x0705: { key: 'message.activity.delete_S_Reviewer', subject: 'repoPath', detail: 'reviewer' },
  0x0706: { key: 'message.activity.review_S_Reviewer', subject: 'repoPath', detail: 'mergeRequest' },
  0x0901: { key: 'message.activity.create_S_Webhook', subject: 'repoPath' },
  0x0902: { key: 'message.activity.update_S_Webhook', subject: 'repoPath' },
  0x0903: { key: 'message.activity.delete_S_Webhook', subject: 'repoPath' },
}

export interface ActivityFeedProps {
  items: ActivityItem[]
  /** 用于把 group/repository 的 id 解析成名字。 */
  groupNameOf?: (id: string | null) => string
  repositoryNameOf?: (id: string | null) => string
  /** 成员名字解析（creator / uid / reviewer）。 */
  memberNameOf?: (id: string) => string
  memberIconOf?: (id: string) => string | null
}

/**
 * 活动流。
 *
 * 旧版是 `config/Activity.js`（682 行、41 个分支）逐类型产出「谁 + 做了什么 + 细节」。
 * 这里把它压缩成一张**码 → 文案键 + 主体来源**的表（逐条对照旧代码的 `message.activity.*`），
 * 因此主链路文案完全一致；未登记的码走通用回退（显示类型码），不会静默丢条目。
 */
export function ActivityFeed({ items, groupNameOf, repositoryNameOf, memberNameOf, memberIconOf }: ActivityFeedProps) {
  const t = useT()

  const subjectOf = (item: ActivityItem, spec: ActivitySpec): string => {
    const content = item.content ?? {}
    switch (spec.subject) {
      case 'group':
        return groupNameOf?.(item.group) ?? String(content.name ?? '')
      case 'repository':
        return repositoryNameOf?.(item.repository) ?? String(content.name ?? '')
      case 'contentName':
        return String(content.name ?? '')
      case 'repoPath': {
        const groupName = groupNameOf?.(item.group) ?? String(content.group ?? '')
        const repositoryName = repositoryNameOf?.(item.repository) ?? String(content.repository ?? '')
        return [groupName, repositoryName].filter(Boolean).join('/')
      }
      default:
        return ''
    }
  }

  const detailOf = (item: ActivityItem, spec: ActivitySpec): string => {
    const content = item.content ?? {}
    if (spec.detail === 'fromTo') {
      const from = String(content.from ?? '')
      const to = String(content.to ?? '')
      return from || to ? `${from} → ${to}` : ''
    }
    if (spec.detail === 'uid') {
      const uid = String(content.uid ?? '')
      return uid ? (memberNameOf?.(uid) ?? uid) : ''
    }
    if (spec.detail === 'name') {
      return String(content.name ?? '')
    }
    if (spec.detail === 'mergeRequest') {
      const id = content.id
      return id === undefined || id === null || id === '' ? '' : `#${String(id)}`
    }
    if (spec.detail === 'reviewer') {
      const reviewer = String(content.reviewer ?? '')
      return reviewer ? (memberNameOf?.(reviewer) ?? reviewer) : ''
    }
    return ''
  }

  return (
    <div className="list-body">
      {items.map((item) => {
        const spec = ACTIVITY_SPECS[item.type]
        const subject = spec ? subjectOf(item, spec) : ''
        const action = spec ? t(spec.key, { s: subject }) : `type 0x${item.type.toString(16)}`
        const detail = spec ? detailOf(item, spec) : ''
        const creatorName = memberNameOf?.(item.creator) ?? item.creator

        return (
          <div className="activity-row" key={item.id}>
            <Avatar size="sm" src={memberIconOf?.(item.creator) ?? undefined} name={creatorName} />
            <div className="activity-row__main">
              <div className="activity-row__text">
                <span className="activity-row__user">{creatorName}</span> {action}
                {detail ? <span className="muted"> · {detail}</span> : null}
              </div>
              <div className="activity-row__time">
                <FormattedTime timestamp={Number(item.time)} />
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
