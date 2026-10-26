/**
 * Webhook 事件清单。
 *
 * 逐字移植旧 `www/view/src/config/WebhookEventConfig.js`：事件名（后端按逗号分隔做**精确匹配**，
 * 见 `domain.WebhookEventMatches`）、对应文案键、以及是否作为分组标题（`divider`）。
 * 文案复用旧词表里的键（`label.pushEvent` 等），不新增说法。
 */
export interface WebhookEventSpec {
  /** 事件名；`null` 表示分组标题行。 */
  event: string | null
  /** 文案键（分组标题用它自己的键）。 */
  labelKey: string
  /** 该行之前画一条分隔线。 */
  divider?: boolean
  /** 默认勾选（旧版仅「推送」默认勾选）。 */
  checked?: boolean
}

export const WEBHOOK_EVENTS: ReadonlyArray<WebhookEventSpec> = [
  { event: 'hook:postReceive', labelKey: 'label.pushEvent', checked: true, divider: true },
  { event: 'repo:fork', labelKey: 'label.forkRepository' },
  { event: 'branch:create', labelKey: 'label.newBranch' },
  { event: 'branch:remove', labelKey: 'label.deleteBranch' },
  { event: 'branch:changeDefaultBranch', labelKey: 'label.defaultBranch' },
  { event: 'tag:create', labelKey: 'label.newTag' },
  { event: 'tag:remove', labelKey: 'label.deleteTag' },
  { event: 'mergeRequest:create', labelKey: 'label.createMergeRequest' },
  { event: 'mergeRequest:close', labelKey: 'label.closeMergeRequest' },
  { event: 'mergeRequest:merge', labelKey: 'label.mergeRequest' },
  { event: 'mergeRequestReviewer:create', labelKey: 'message.selectReviewer' },
  { event: 'mergeRequestReviewer:delete', labelKey: 'message.deleteReviewer' },
  { event: 'mergeRequestReviewer:review', labelKey: 'label.reviewReviewer' },
  { event: 'repo:addMember', labelKey: 'label.inviteMember', divider: true },
  { event: 'repo:changeMemberRole', labelKey: 'label.changeMemberRole' },
  { event: 'repo:removeMember', labelKey: 'label.removeMember' },
  { event: 'repo:updateAvator', labelKey: 'label.repositoryAvatar', divider: true },
  { event: 'repo:updateName', labelKey: 'label.repositoryName' },
  { event: 'repo:updateDescription', labelKey: 'label.repositoryDescription' },
  { event: 'repo:changeOwner', labelKey: 'label.owner' },
  { event: 'repo:changeURL', labelKey: 'label.repositoryURL' },
  { event: 'repo:remove', labelKey: 'label.deleteRepository' },
  { event: 'branch:createProtectedBranchRule', labelKey: 'label.createProtectedBranchRule' },
  { event: 'branch:changeProtectedBranchRule', labelKey: 'label.changeProtectedBranchRule' },
  { event: 'branch:removeProtectedBranchRule', labelKey: 'label.removeProtectedBranchRule' },
]

/** 事件名字符串 ↔ 勾选列表（与后端 `strings.Split(events, ",")` 对称）。 */
export function parseEvents(events: string): string[] {
  return events ? events.split(',').filter(Boolean) : []
}

export function joinEvents(events: string[]): string {
  return events.join(',')
}
