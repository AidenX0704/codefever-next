import { ApiError, errorMessage, get, post } from './client'
import type {
  Branch,
  ActivityItem,
  BlameResult,
  MergeRequestDetail,
  MergeRequestListItem,
  MergeRequestVersionList,
  MergeType,
  ProtectedBranchRule,
  RefListContainSHA,
  Repository,
  RepositoryWebhook,
  RepositoryWebhookLog,
  RepositoryWebhookLogData,
  Commit as CommitSummary,
  CommitDetail,
  FileChangesResponse,
  ObjectView,
  PathStackEntry,
  RepositoryConfig,
  RepositoryListItem,
  Tag,
} from './types'

/**
 * 仓库域接口。
 *
 * 查询参数名逐字对应 Go 侧读取的键（改名会直接 0x0200 缺参）：
 *   `repository/config`      → rKey
 *   `repository/object`      → repository(仓库 key)、parent(树/提交 SHA)、path、revision
 *   `repository/pathStack`   → repository、revision、path
 *   `repository/branchList`  → repository
 *   `repository/tagList`     → repository
 * 约定：所有 `rKey`/`repository` 都指仓库主键（32 位 hex），不是仓库名。
 */

function requireData<T>(data: T | undefined): T {
  if (data === undefined || data === null) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}

/** 取当前用户可见的仓库列表（每项带自己在仓库内的 `role` 与 `members`）。 */
export async function fetchRepositoryList(): Promise<RepositoryListItem[]> {
  const { data } = await get<RepositoryListItem[]>('/api/repository/list')
  return requireData(data)
}

/** 仓库详情 + 成员 + 分支/标签引用 + 计数（进入仓库页时拉一次）。 */
export async function fetchRepositoryConfig(repositoryKey: string): Promise<RepositoryConfig> {
  const { data } = await get<RepositoryConfig>('/api/repository/config', { rKey: repositoryKey })
  return requireData(data)
}

/** 取某个 rev 下的一层目录或一个文件内容。 */
export async function fetchObject(params: {
  repositoryKey: string
  parent: string
  path?: string
  revision?: string
}): Promise<ObjectView> {
  const { data } = await get<ObjectView>('/api/repository/object', {
    repository: params.repositoryKey,
    parent: params.parent,
    path: params.path ?? '',
    revision: params.revision,
  })
  return requireData(data)
}

/** 面包屑路径栈（每一级目录项）。 */
export async function fetchPathStack(params: {
  repositoryKey: string
  revision: string
  path: string
}): Promise<PathStackEntry[]> {
  const { data } = await get<PathStackEntry[]>('/api/repository/pathStack', {
    repository: params.repositoryKey,
    revision: params.revision,
    path: params.path,
  })
  return requireData(data)
}

export async function fetchBranchList(repositoryKey: string): Promise<Branch[]> {
  const { data } = await get<Branch[]>('/api/repository/branchList', { repository: repositoryKey })
  return requireData(data)
}

export async function fetchTagList(repositoryKey: string): Promise<Tag[]> {
  const { data } = await get<Tag[]>('/api/repository/tagList', { repository: repositoryKey })
  return requireData(data)
}

/** 提交列表（`revision` 必填：分支名/标签名/8 位短 SHA；非法值后端返回空数组）。 */
export async function fetchCommitList(params: {
  repositoryKey: string
  revision: string
  keyword?: string
  path?: string
  page?: number
  pagesize?: number
}): Promise<CommitSummary[]> {
  const { data } = await get<CommitSummary[]>('/api/repository/commitList', {
    repository: params.repositoryKey,
    revision: params.revision,
    keyword: params.keyword ?? '',
    path: params.path ?? '',
    page: params.page ?? 1,
    pagesize: params.pagesize ?? 10,
  })
  return requireData(data)
}

/** 提交详情（`commitSHA` 缺省时后端返回 sha/commit/parent/email 全 null、time 0）。 */
export async function fetchCommitDetail(repositoryKey: string, commitSHA: string): Promise<CommitDetail> {
  const { data } = await get<CommitDetail>('/api/repository/commitDetail', {
    repository: repositoryKey,
    commitSHA,
  })
  return requireData(data)
}

/** 两个提交之间的文件级 diff（`original` 缺省为全 0 的 40 位 SHA，即"空树"）。 */
export async function fetchFileChanges(params: {
  repositoryKey: string
  modified: string
  original?: string
}): Promise<FileChangesResponse> {
  const { data } = await get<FileChangesResponse>('/api/repository/fileChanges', {
    repository: params.repositoryKey,
    original: params.original ?? '',
    modified: params.modified,
  })
  return requireData(data)
}

/** 从全局仓库列表里按 `组名 + 仓库名` 找到仓库（旧版同样靠它拿 rKey）。 */
export function findRepository(
  repositories: RepositoryListItem[],
  groupName: string | undefined,
  repositoryName: string | undefined,
): RepositoryListItem | undefined {
  if (!groupName || !repositoryName) {
    return undefined
  }
  return repositories.find(
    (repository) => repository.group.name === groupName && repository.name === repositoryName,
  )
}

/* ------------------------------ 仓库设置（写操作） ------------------------------ */


/** 更新仓库显示名与描述（`displayName` 必填，`description` 必须传，不能缺）。 */
export function updateRepository(repositoryKey: string, displayName: string, description: string) {
  return post('/api/repository/update', { repository: repositoryKey, displayName, description })
}

/** 上传仓库头像（multipart，字段名 `avatar`）。 */
export function uploadRepositoryAvatar(repositoryKey: string, file: File) {
  return post('/api/repository/uploadAvatar', { repository: repositoryKey, avatar_BINARY: file })
}

/** 按邮箱添加仓库成员。 */
export function addRepositoryMember(repositoryKey: string, email: string) {
  return post('/api/repository/addMember', { repository: repositoryKey, email })
}

/** 修改成员角色（`roleID` 取 UAC.Role）。 */
export function changeRepositoryMemberRole(repositoryKey: string, userID: string, roleID: number) {
  return post('/api/repository/changeMemberRole', { repository: repositoryKey, userID, roleID })
}

export function removeRepositoryMember(repositoryKey: string, userID: string) {
  return post('/api/repository/removeMember', { repository: repositoryKey, userID })
}

export function changeRepositoryOwner(repositoryKey: string, userID: string) {
  return post('/api/repository/changeOwner', { repository: repositoryKey, userID })
}

/* ------------------------------ Webhook ------------------------------ */

export async function fetchWebhooks(repositoryKey: string): Promise<RepositoryWebhook[]> {
  const { data } = await post<RepositoryWebhook[]>('/api/repository/webhooks', { repository: repositoryKey })
  return data ?? []
}

export async function fetchWebhook(repositoryKey: string, webhookKey: string): Promise<RepositoryWebhook | null> {
  const { data } = await post<RepositoryWebhook>('/api/repository/getWebhook', {
    repository: repositoryKey,
    rwKey: webhookKey,
  })
  return data ?? null
}

/** 新建（不传 rwKey）或更新 webhook；`active` 必须是 1 或 2。 */
export function saveWebhook(params: {
  repositoryKey: string
  webhookKey?: string
  url: string
  /** 签名密钥。后端 `editWebhook` 缺这个参数会直接 500（PHP 侧 `rw_secret` 是 NOT NULL 列），必须传。 */
  secret: string
  events: string
  active: 1 | 2
}) {
  return post('/api/repository/editWebhook', {
    repository: params.repositoryKey,
    rwKey: params.webhookKey ?? '',
    url: params.url,
    secret: params.secret,
    events: params.events,
    active: params.active,
  })
}

export function deleteWebhook(repositoryKey: string, webhookKey: string) {
  return post('/api/repository/deleteWebhook', { repository: repositoryKey, rwKey: webhookKey })
}

export async function fetchWebhookLogs(webhookKey: string): Promise<RepositoryWebhookLog[]> {
  const { data } = await post<RepositoryWebhookLog[]>('/api/repository/getRepositoryWebhookLogs', {
    webhook: webhookKey,
  })
  return data ?? []
}

export async function fetchWebhookLogData(logID: string): Promise<RepositoryWebhookLogData | null> {
  const { data } = await post<RepositoryWebhookLogData>('/api/repository/getRepositoryWebhookLogData', { id: logID })
  return data ?? null
}

/* ------------------------------ 保护分支 / 默认分支 ------------------------------ */

export async function fetchProtectedBranchRules(repositoryKey: string): Promise<ProtectedBranchRule[]> {
  const { data } = await get<ProtectedBranchRule[]>('/api/repository/protectedBranchRules', {
    repository: repositoryKey,
  })
  return data ?? []
}

export function createProtectedBranchRule(repositoryKey: string, rule: string, rolePush: number, roleMerge: number) {
  return post('/api/repository/createProtectedBranchRule', { repository: repositoryKey, rule, rolePush, roleMerge })
}

export function updateProtectedBranchRule(
  repositoryKey: string,
  id: string,
  rule: string,
  rolePush: number,
  roleMerge: number,
) {
  return post('/api/repository/updateProtectedBranchRule', { repository: repositoryKey, id, rule, rolePush, roleMerge })
}

export function deleteProtectedBranchRule(repositoryKey: string, id: string, rule: string) {
  return post('/api/repository/deleteProtectedBranchRule', { repository: repositoryKey, id, rule })
}

/** 修改默认分支（`old` 为当前值，后端要求 `old !== branch`）。 */
export function changeDefaultBranch(repositoryKey: string, oldBranch: string, branch: string) {
  return post('/api/repository/defaultBranch', { repository: repositoryKey, old: oldBranch, branch })
}

/** 改仓库标识名（URL slug）。 */
export function updateRepositoryName(repositoryKey: string, name: string) {
  return post('/api/repository/updateName', { repository: repositoryKey, name })
}

/** 删除仓库（不可恢复）。 */
export function deleteRepository(repositoryKey: string) {
  return post('/api/repository/deleteRepository', { repository: repositoryKey })
}

/* ------------------------------ 合并请求 ------------------------------ */

export async function fetchMergeRequests(params: {
  repositoryKey: string
  status?: number
  keyword?: string
  sort?: string
  page?: number
  pagesize?: number
}): Promise<MergeRequestListItem[]> {
  const { data } = await get<MergeRequestListItem[]>('/api/repository/mergeRequests', {
    repository: params.repositoryKey,
    status: params.status ?? 0,
    keyword: params.keyword ?? '',
    // 后端把 `sort` 当必填：既不是 `asc` 也不是 `desc` 直接 0x0201。
    sort: params.sort ?? 'desc',
    page: params.page ?? 1,
    pagesize: params.pagesize ?? 20,
  })
  return data ?? []
}

export async function fetchMergeRequestDetail(repositoryKey: string, mid: number): Promise<MergeRequestDetail | null> {
  const { data } = await get<MergeRequestDetail>('/api/repository/mergeRequestDetail', { repository: repositoryKey, mid })
  return data ?? null
}

/** 合并请求的 diff：`fileChanges` 带 `mergeRequest` 参数时按 MR 的源/目标提交比较。 */
export async function fetchMergeRequestChanges(params: {
  repositoryKey: string
  mergeRequestKey: string
  original: string
  modified: string
  targetRepositoryKey?: string
}): Promise<FileChangesResponse> {
  const { data } = await get<FileChangesResponse>('/api/repository/fileChanges', {
    repository: params.repositoryKey,
    mergeRequest: params.mergeRequestKey,
    original: params.original,
    modified: params.modified,
    targetRepository: params.targetRepositoryKey,
  })
  return requireData(data)
}

/** 关闭合并请求（`id` 为 MR 主键）。 */
export function closeMergeRequest(mergeRequestKey: string) {
  return post('/api/repository/mergeRequestClose', { mergeRequest: mergeRequestKey })
}

/** 提交评审（后端只要求 `id`：把当前用户标记为已评审）。 */
export function reviewMergeRequest(mergeRequestKey: string) {
  return post('/api/repository/mergeRequestReview', { id: mergeRequestKey })
}

/** 指派评审人（`id` + `reviewer` 为用户主键）。 */
export function assignReviewer(mergeRequestKey: string, reviewer: string) {
  return post('/api/repository/assignReviewer', { id: mergeRequestKey, reviewer })
}

export function deleteReviewer(mergeRequestKey: string, reviewer: string) {
  return post('/api/repository/deleteReviewer', { id: mergeRequestKey, reviewer })
}

/**
 * 检查合并类型（快进 / 普通合并 / 冲突），供合并按钮提示。
 *
 * 后端返回 `{"canBeFastForward": bool}`：能快进时旧版直接合、不必填合并信息。
 */
export async function checkMergeType(mergeRequestKey: string): Promise<MergeType> {
  const { data } = await post<MergeType>('/api/repository/checkMergeType', { mergeRequest: mergeRequestKey })
  return data ?? { canBeFastForward: false }
}

/**
 * 执行合并（`message` 合并提交信息，`usingSquash` 非 "false" 即压缩）。
 *
 * 后端在**合并冲突**时返回 `{"conflict": true}` 且 `code=0` —— 不是错误码，
 * 所以这里必须自己把冲突翻出来，否则前端会把它当成功。
 */
export async function mergeMergeRequest(
  mergeRequestKey: string,
  message: string,
  usingSquash: boolean,
): Promise<{ conflict: boolean }> {
  const { data } = await post<{ conflict?: boolean }>('/api/repository/mergeBranch', {
    mergeRequest: mergeRequestKey,
    message,
    usingSquash: usingSquash ? 'true' : 'false',
  })
  return { conflict: Boolean(data?.conflict) }
}

/** MR 的版本列表（`mergeRequestVersionList`）：最近修改 / 基线版本。 */
export async function fetchMergeRequestVersionList(mergeRequestKey: string): Promise<MergeRequestVersionList> {
  const { data } = await get<MergeRequestVersionList>('/api/repository/mergeRequestVersionList', {
    mergeRequest: mergeRequestKey,
  })
  return data ?? { mergeVersionList: [], baseVersion: '', baseVersionList: [] }
}

/** MR 涉及的提交列表（`mergeRequestCommits`）。 */
export async function fetchMergeRequestCommits(params: {
  mergeRequestKey: string
  sourceRepositoryKey: string
  sourceBranch: string
  targetRepositoryKey: string
  targetBranch: string
}): Promise<CommitSummary[]> {
  const { data } = await get<CommitSummary[]>('/api/repository/mergeRequestCommits', {
    mergeRequest: params.mergeRequestKey,
    sourceRepository: params.sourceRepositoryKey,
    sourceBranch: params.sourceBranch,
    targetRepository: params.targetRepositoryKey,
    targetBranch: params.targetBranch,
  })
  return data ?? []
}

/** 目标仓库信息（跨仓库 MR 用）：`repository/targetRepository`。 */
export async function fetchTargetRepository(repositoryKey: string): Promise<Repository | null> {
  const { data } = await get<Repository>('/api/repository/targetRepository', { repository: repositoryKey })
  return data ?? null
}

/** 创建合并请求（返回 `{ id }` —— 即业务编号，用于跳详情）。 */
export async function createMergeRequest(params: {
  sourceRepositoryKey: string
  targetRepositoryKey: string
  sourceBranch: string
  targetBranch: string
  title: string
  description: string
  reviewers?: string[]
}): Promise<number> {
  const { data } = await post<{ id: number }>('/api/repository/createMergeRequest', {
    sourceRepository: params.sourceRepositoryKey,
    targetRepository: params.targetRepositoryKey,
    sourceBranch: params.sourceBranch,
    targetBranch: params.targetBranch,
    title: params.title,
    description: params.description,
    reviewers: (params.reviewers ?? []).join(','),
  })
  return data?.id ?? 0
}

/* ------------------------------ 活动流 ------------------------------ */

/** 活动流分类（后端 `parseCategory` 白名单）：全部 / 提交 / 合并请求 / 成员。 */
export const ACTIVITY_CATEGORY = { all: 0, commit: 1, mergeRequest: 2, member: 3 } as const

export async function fetchRepositoryActivities(params: {
  repositoryKey: string
  category?: number
  keyword?: string
  /** 传了就是「只看这个 MR 的动态」（MR 详情的第三个页签）。 */
  mergeRequestKey?: string
  page?: number
  pagesize?: number
}): Promise<ActivityItem[]> {
  const { data } = await get<ActivityItem[]>('/api/repository/activities', {
    repository: params.repositoryKey,
    category: params.category ?? ACTIVITY_CATEGORY.all,
    keyword: params.keyword ?? '',
    mergeRequest: params.mergeRequestKey,
    page: params.page ?? 1,
    pagesize: params.pagesize ?? 20,
  })
  return data ?? []
}

/**
 * 全局合并请求列表（跨当前用户所有仓库）：`repository/relatedMergeRequests`。
 *
 * 后端返回的是**信封里的对象** `{ mergeRequests, relatedRepository }`，
 * 不是数组 —— 旧版取的是 `data.data.mergeRequests`。
 */
export async function fetchRelatedMergeRequests(params: {
  status?: number
  keyword?: string
  sort?: string
  page?: number
  pagesize?: number
}): Promise<{ mergeRequests: MergeRequestListItem[]; relatedRepository: string[] }> {
  const { data } = await get<{ mergeRequests?: MergeRequestListItem[]; relatedRepository?: string[] }>(
    '/api/repository/relatedMergeRequests',
    {
      status: params.status ?? 0,
      keyword: params.keyword ?? '',
      // 后端把 `sort` 当必填：既不是 `asc` 也不是 `desc` 直接 0x0201。
      sort: params.sort ?? 'desc',
      page: params.page ?? 1,
      pagesize: params.pagesize ?? 20,
    },
  )
  return {
    mergeRequests: data?.mergeRequests ?? [],
    relatedRepository: data?.relatedRepository ?? [],
  }
}

/* ------------------------------ 文件浏览辅助 ------------------------------ */

/** 某个提交被哪些分支 / 标签包含（`repository/refListContainSHA`）。 */
export async function fetchRefListContainSHA(repositoryKey: string, hash: string): Promise<RefListContainSHA> {
  const { data } = await get<RefListContainSHA>('/api/repository/refListContainSHA', {
    repository: repositoryKey,
    hash,
  })
  return { branch: data?.branch ?? [], tag: data?.tag ?? [] }
}

/**
 * 某个文件在某次提交下的 blame 归属（`repository/blameInfo`）。
 *
 * 注意后端这里复刻了 PHP 的权限误用（用 MR 读权限判定仓库读），
 * 普通成员可能拿不到 —— 调用方要能容忍失败（见 `FileTree`）。
 */
export async function fetchBlameInfo(params: {
  repositoryKey: string
  revision: string
  path: string
}): Promise<BlameResult> {
  const { data } = await get<BlameResult>('/api/repository/blameInfo', {
    repository: params.repositoryKey,
    revision: params.revision,
    path: params.path,
  })
  return { blame: data?.blame ?? [], lines: data?.lines ?? [] }
}

/* ------------------------------ 创建 / 派生 / 分支标签 ------------------------------ */

/** 新建仓库（`group` + `name`(slug) + `displayName` + `description`）。 */
export async function createRepository(params: {
  groupKey: string
  name: string
  displayName: string
  description: string
}): Promise<Repository> {
  const { data } = await post<Repository>('/api/repository/create', {
    group: params.groupKey,
    name: params.name,
    displayName: params.displayName,
    description: params.description,
  })
  return requireData(data)
}

/** Fork 仓库（比 `create` 多一个 `forkID` = 源仓库主键）。 */
export async function forkRepository(params: {
  groupKey: string
  name: string
  displayName: string
  description: string
  forkID: string
}): Promise<Repository> {
  const { data } = await post<Repository>('/api/repository/fork', {
    group: params.groupKey,
    name: params.name,
    displayName: params.displayName,
    description: params.description,
    forkID: params.forkID,
  })
  return requireData(data)
}

/** 新建分支（`origin` 为起点分支/提交）。 */
export function createBranch(repositoryKey: string, name: string, origin: string) {
  return post('/api/repository/createBranch', { repository: repositoryKey, name, origin })
}

/** 删除分支。 */
export function deleteBranch(repositoryKey: string, branch: string) {
  return post('/api/repository/deleteBranch', { repository: repositoryKey, branch })
}

/** 新建标签（`origin` 为起点分支/提交，`description` 为标签说明）。 */
export function createTag(repositoryKey: string, name: string, origin: string, description: string) {
  return post('/api/repository/createTag', { repository: repositoryKey, name, origin, description })
}

/** 删除标签。 */
export function deleteTag(repositoryKey: string, tag: string) {
  return post('/api/repository/deleteTag', { repository: repositoryKey, tag })
}
