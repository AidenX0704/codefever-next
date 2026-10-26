/**
 * 后端返回的数据形状（类型化）。
 *
 * 字段名逐字对应 Go 侧的归一化结构，**不要改名**：
 *   - `backend/internal/domain/user.go`        NormalizedUser / NormalizedGroup / NormalizedRepository
 *   - `backend/internal/handlers/group.go`     groupListItem（Group + role + members）
 *   - `backend/internal/handlers/repository.go` RepositoryListItem（Repository + role + members）
 */

export interface RepositoryMember {
  id: string
  icon: string | null
  name: string
  email: string
  phoneCode: string | null
  phoneNumber: string | null
  team: string | null
  /** 成员在仓库/组内的角色整数（UAC.Role）。 */
  role: number
  notification: number
  mfaEnabled: boolean
  admin: boolean
  emails: string[]
  unReadNotification: number
  status: boolean
  host: string
  ssh: string
  joined: number
  /** 由仓库组继承而来的成员（仓库成员表里的"组成员"行）。 */
  groupMember?: boolean
  /** 仓库/组的创建者。 */
  creatorFlag?: boolean
  /** 已被移除的成员（成员表里只保留徽标，不再可操作）。 */
  deleteFlag?: boolean
}

/**
 * 管理后台的成员条目（`admin/groupMembers`、`admin/repositoryMembers`）。
 *
 * 比普通成员多三个标志位：
 *   - `groupMember`：由组继承而来的成员（仓库成员表里才有）；
 *   - `creatorFlag`：该成员是组/仓库的创建者；
 *   - `deleteFlag`：已被移除。
 */
export interface AdminMember extends RepositoryMember {
  groupMember: boolean
  creatorFlag: boolean
  deleteFlag: boolean
}

export interface RepositoryGroup {
  id: string
  icon: string | null
  displayName: string
  name: string
  description: string
  owner: string
  type: number
  created: number
  updated: number
  status: number
}

export interface MergeRequestCount {
  open: number
  merged: number
  closed: number
  /** 后端 `admin/repositoryList` 还返回 `all`（含已关闭），前端此前漏声明。 */
  all?: number
}

export interface Repository {
  id: string
  icon: string | null
  displayName: string
  name: string
  description: string
  owner: string
  forkFrom: string | null
  created: number
  updated: number
  group: RepositoryGroup
  forkCount: number
  mergeRequestCount: MergeRequestCount
  defaultBranch: string | null
  status: number
}

export interface RepositoryListItem extends Repository {
  role: number
  members: RepositoryMember[]
}

export interface Group extends RepositoryGroup {
  role: number
  members: RepositoryMember[]
}

export interface CommitEmail {
  id: string
  email: string
  isCheck: boolean
}

export interface CurrentUser {
  id: string
  icon: string | null
  name: string
  email: string
  phoneCode: string | null
  phoneNumber: string | null
  team: string | null
  role: string | null
  notification: number
  mfaEnabled: boolean
  admin: boolean
  emails: Array<CommitEmail | string>
  unReadNotification: number
  status: boolean
  host: string
  ssh: string
}

/* ------------------------------ 仓库浏览 ------------------------------ */

/** 提交摘要（`domain.Commit`）。 */
export interface Commit {
  sha: string
  commit: string
  detail: string
  email: string
  time: number
}

/** tree 对象中的一项（`domain.ObjectEntry`）。 */
export interface ObjectEntry {
  mode: string
  type: string
  object: string
  name: string
  commit?: Commit
  url: string
}

/** blob 内容（`parseObjectToFile` 的输出；字段按类型出现）。 */
export interface FileObject {
  mime?: string
  size?: number
  raw?: string
  base64Encoded?: string
  width?: number
  height?: number
  unsupport?: boolean
  oversize?: boolean
}

/** `repository/object` 的响应（`domain.ObjectView`）。 */
export interface ObjectView {
  type: 'blob' | 'tree' | string
  object: FileObject | ObjectEntry[]
  commit?: Commit
  path?: string
}

/** `domain.PathStackEntry`。 */
export interface PathStackEntry {
  mode: string
  type: string
  object: string
  name: string
}

/** `domain.Branch`。 */
export interface Branch {
  id: string
  name: string
  commit: Commit
  isMerge: boolean
  protected: boolean
}

/** `domain.Tag`。 */
export interface Tag {
  id: string
  name: string
  detail?: string
  commit?: Commit
}

export interface BranchRef {
  id: string
  name: string
}

/** `repository/config` 的响应（`domain.RepositoryConfig`）。 */
export interface RepositoryConfig {
  repository: Repository
  group: RepositoryGroup
  members: RepositoryMember[]
  branches: BranchRef[]
  tags: BranchRef[]
  /** `file` 是仓库体积（KB），`mergeRequest` 是该仓库的合并请求数。 */
  count: { commit: number; branch: number; tag: number; file: number; mergeRequest: number }
}

/* ------------------------------ 提交与 diff ------------------------------ */

/**
 * blame 的一个归属区间（`repository/blameInfo` 的元素）。
 *
 * 后端 `domain.BlameBlock`：start/length 是 **1 基**的行号区间。
 */
export interface BlameBlock {
  start: number
  length: number
  commit: Commit
}

/** `repository/blameInfo` 的响应（`domain.BlameResult`）。 */
export interface BlameResult {
  blame: BlameBlock[]
  lines: string[]
}

/** `repository/refListContainSHA` 的响应：包含某个提交的分支 / 标签名。 */
export interface RefListContainSHA {
  branch: string[]
  tag: string[]
}

/** 提交详情（`domain.CommitDetail`）。 */
export interface CommitDetail {
  sha: string | null
  commit: string | null
  parent: string | null
  email: string | null
  time: number
}

export interface DiffSide {
  name: string | null
  mode?: string | null
  sha?: string | null
}

/** diff 正文：`line` 是 unified diff 的逐行数组，`raw` 是全文（超限时为空）。 */
export interface DiffBody {
  raw?: string | null
  line: string[]
  additions: number
  /** ⚠️ 后端沿用 PHP 的 `deletions--` 写法，可能是负数（见 domain.DiffBody 注释）。 */
  deletions: number
}

export interface DiffFile {
  original: DiffSide
  modified: DiffSide
  diff: DiffBody
  /** `text` | `oversize`（单文件行数或字节超限）。 */
  type: string
}

export interface FileChanges {
  count: number
  detail: DiffFile[]
}

/** 命中"文件数超过 100"时的另一种响应形状。 */
export interface FileChangesLarge {
  large: true
  count: number
  detail: string
}

export type FileChangesResponse = FileChanges | FileChangesLarge

/* ------------------------------ 个人设置 ------------------------------ */

/** SSH 公钥（`domain.NormalizedSSHKey`）。 */
export interface SSHKey {
  id: string
  creator: string
  name: string
  hash: string
  type: string
  created: number
  deleteFlag: boolean
}

/** MFA 设置数据（`user/getMFAData`）：secret 用于生成二维码，已开启时 status 为 true。 */
export interface MFAData {
  /** 二维码图片（SVG data URI）。后端 `getMFAData` 始终返回，前端要渲染它。 */
  image?: string
  secret?: string
  status?: boolean
  [key: string]: unknown
}

/* ------------------------------ 仓库设置：Webhook / 保护分支 ------------------------------ */

export interface RepositoryWebhook {
  id: string
  user: string | null
  url: string
  secret: string
  /** 逗号分隔的事件名（后端做精确匹配）。 */
  events: string
  /** 1=启用、2=停用。 */
  active: number
  updated: number
}

export interface RepositoryWebhookLog {
  id: string
  webhook: string
  start: number
  end: number
  status: number
  success: boolean
  created: string
}

/** 单条投递日志的详情（`request` / `response` 是原始报文）。 */
export interface RepositoryWebhookLogData {
  id: string
  /** 后端是 `any`（已 JSON 解码的对象或字符串），前端要自己决定怎么渲染。 */
  request?: unknown
  response?: unknown
  [key: string]: unknown
}

export interface ProtectedBranchRule {
  id: string
  rule: string
  /** 允许 push 的最低角色（UAC.Role）。 */
  rolePush: number
  /** 允许合并的最低角色（UAC.Role）。 */
  roleMerge: number
}

/** `group/config` 的响应（`handlers.groupConfig`）。 */
export interface GroupConfig {
  group: RepositoryGroup
  members: RepositoryMember[]
  count: { member: number }
}

/* ------------------------------ 合并请求 ------------------------------ */

/** 合并请求状态（后端 `domain.MergeRequestStatus*`）。 */
export const MERGE_REQUEST_STATUS = { open: 1, merged: 2, closed: 3 } as const

export interface MergeRequestListItem {
  id: string
  sourceRepository: string
  sourceBranch: string
  targetRepository: string
  targetBranch: string
  number: number
  title: string
  status: number
  create: number
  update: number
  reviewers: MergeRequestReviewer[]
  commit: Commit
}

/**
 * 合并请求的评审人（后端 `reviewersOf` 的输出）。
 *
 * 注意：后端返回的是**评审记录**，`user` 才是用户主键（u_key）；
 * 删除评审人时后端按 `u_key` 匹配（`repo.DeleteReviewer`），传 `id` 会静默失败。
 */
export interface MergeRequestReviewer {
  /** 评审记录主键（mrr_key）。 */
  id: string
  /** 评审人用户主键（u_key）。 */
  user: string
  /** 是否已评审。 */
  isReview: boolean
}

export interface MergeRequestDetail {
  id: string
  user: string
  sourceRepository: string
  sourceBranch: string
  sourceObject: string
  targetRepository: string
  targetBranch: string
  targetObject: string
  submitter: CurrentUser
  sourceRepoInfo: Repository & { members: RepositoryMember[] }
  targetRepoInfo?: Repository & { members: RepositoryMember[] }
  title: string
  description: string | null
  number: number
  handler: string | null
  status: number
  create: number
  update: number
  close: number
  merge: number
  reviewers: MergeRequestReviewer[]
  email?: string
}

/** `repository/mergeRequestVersionList` 的版本列表。 */
export interface MergeRequestVersionList {
  mergeVersionList: Array<{ sha: string; time?: number; user?: string }>
  baseVersion: string
  baseVersionList: Array<{ sha: string; time?: number; user?: string }>
}

/** `repository/checkMergeType` 的响应。 */
export interface MergeType {
  canBeFastForward: boolean
}

/* ------------------------------ 管理后台 ------------------------------ */

/** 管理后台列表的通用信封：`{ count, list }`。 */
export interface AdminListResponse<T> {
  count: number
  list: T[]
}

export interface AdminUserListItem {
  id: string
  email: string
  name: string
  avatar: string | null
  mfaEnabled: boolean
  created: string
  status: boolean
  admin: boolean
  projects: number
}

export interface AdminUsage {
  user: number
  repository: number
  group: number
}

export interface AdminDiskUsage {
  name: string
  usage: number
}

export interface AdminSystemStatus {
  usage: AdminUsage
  vm: { cpu: number; memory: number; disk: AdminDiskUsage[] }
  service: { nginx: boolean; codefever: boolean; sendmail: boolean }
}

/* ------------------------------ 活动流 ------------------------------ */

/** 动态条目（`domain.RepositoryActivityView` / 组的同构结构）。 */
export interface ActivityItem {
  id: string
  /** 活动类型码（`0x02xx` 组、`0x03xx` 仓库、`0x04xx` 分支、`0x05xx` 标签、`0x07xx` MR、`0x09xx` Webhook）。 */
  type: number
  creator: string
  group: string | null
  repository: string | null
  content: Record<string, unknown>
  time: number
}

/**
 * 判断某个 object 响应是不是"目录列表"。
 *
 * ⚠️ 真实后端的 `repository/object` 在 `parent` 是**提交**（最常见：分支名/tag/
 * pathStack 的上一级 object）时返回 `type: "commit"`，只有 `parent` 本身是 tree SHA
 * 时才返回 `type: "tree"`；两种情况 `object` 都是条目数组。
 * 因此**不要**用 `type === 'tree'` 判断目录，否则文件列表会是空的（曾因此出现"仓库
 * 文件列表不显示"）。
 */
export function isDirectoryView(view: ObjectView | null | undefined): view is ObjectView & { object: ObjectEntry[] } {
  return Boolean(view) && view!.type !== 'blob' && Array.isArray(view!.object)
}
