import { ApiError, errorMessage, get, post } from './client'
import type { ActivityItem, Group, GroupConfig, RepositoryGroup } from './types'

/**
 * 取当前用户可见的仓库组列表（含自己在组内的角色与成员）。
 *
 * `GET /api/group/list` —— 后端把"我拥有的"和"我参与的"合并去重后返回，
 * 每项带 `role`（UAC 角色整数）与 `members`（成员列表，含角色）。
 */
export async function fetchGroupList(): Promise<Group[]> {
  const { data } = await get<Group[]>('/api/group/list')
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}

/* ------------------------------ 组设置（写操作） ------------------------------ */

export async function fetchGroupConfig(groupKey: string): Promise<GroupConfig> {
  const { data } = await get<GroupConfig>('/api/group/config', { gKey: groupKey })
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}

/** 更新组显示名与描述（`description` 必须传）。 */
export function updateGroup(groupKey: string, displayName: string, description: string) {
  return post('/api/group/update', { group: groupKey, displayName, description })
}

/** 改组标识名（URL slug），后端会校验唯一性。 */
export function updateGroupName(groupKey: string, name: string) {
  return post('/api/group/updateName', { group: groupKey, name })
}

export function uploadGroupAvatar(groupKey: string, file: File) {
  return post('/api/group/uploadAvatar', { group: groupKey, avatar_BINARY: file })
}

export function addGroupMember(groupKey: string, email: string) {
  return post('/api/group/addMember', { group: groupKey, email })
}

export function changeGroupMemberRole(groupKey: string, userID: string, roleID: number) {
  return post('/api/group/changeMemberRole', { group: groupKey, userID, roleID })
}

export function removeGroupMember(groupKey: string, userID: string) {
  return post('/api/group/removeMember', { group: groupKey, userID })
}

export function changeGroupOwner(groupKey: string, userID: string) {
  return post('/api/group/changeOwner', { group: groupKey, userID })
}

export function deleteGroup(groupKey: string) {
  return post('/api/group/deleteGroup', { group: groupKey })
}

/** 从全局组列表里按名称找到组（与 `findRepository` 同构）。 */
export function findGroup(groups: Group[], groupName: string | undefined): Group | undefined {
  if (!groupName) {
    return undefined
  }
  return groups.find((group) => group.name === groupName)
}

/** 仓库组类型（后端 `domain.GroupType*`）。 */
export const GROUP_TYPE = { normal: 1, user: 2 } as const

/**
 * 新建仓库组（`displayName` + `name`(slug) + `type` 三者缺一即 0x0201）。
 *
 * `type` 必须是 1（普通组）或 2（用户个人默认分组），后端按白名单校验。
 */
export async function createGroup(params: {
  name: string
  displayName: string
  description: string
  type?: number
}): Promise<RepositoryGroup> {
  const { data } = await post<RepositoryGroup>('/api/group/create', {
    displayName: params.displayName,
    name: params.name,
    type: params.type ?? GROUP_TYPE.normal,
    description: params.description,
  })
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}

/* ------------------------------ 活动流 ------------------------------ */

export async function fetchGroupActivities(params: {
  groupKey: string
  category?: number
  keyword?: string
}): Promise<ActivityItem[]> {
  const { data } = await get<ActivityItem[]>('/api/group/activities', {
    group: params.groupKey,
    category: params.category ?? 0,
    keyword: params.keyword ?? '',
  })
  return data ?? []
}

/** 组内合并请求列表：`group/mergeRequests`（`group` + `status` + `sort`）。 */
export async function fetchGroupMergeRequests(params: {
  groupKey: string
  status?: number
  keyword?: string
  sort?: string
  page?: number
  pagesize?: number
}): Promise<import('./types').MergeRequestListItem[]> {
  const { data } = await get<import('./types').MergeRequestListItem[]>('/api/group/mergeRequests', {
    group: params.groupKey,
    status: params.status ?? 0,
    keyword: params.keyword ?? '',
    // 后端把 `sort` 当必填：既不是 `asc` 也不是 `desc` 直接 0x0201。
    sort: params.sort ?? 'desc',
    // `keyword` / `page` / `pagesize` 缺失时 PHP 侧会抛 TypeError，
    // Go 侧等价行为是 500，所以这三个必须显式带上。
    page: params.page ?? 1,
    pagesize: params.pagesize ?? 20,
  })
  return data ?? []
}
