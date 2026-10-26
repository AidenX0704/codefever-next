import { ApiError, errorMessage, get, post } from './client'
import type {
  AdminListResponse,
  AdminMember,
  AdminSystemStatus,
  AdminUserListItem,
  Repository,
  RepositoryGroup,
} from './types'

/**
 * 管理后台接口（仅管理员可用，后端 `AuthAdminWebUser`）。
 *
 * 注意：列表类端点都是 **POST**，body 携带 `category` / `keyword` / `sort` / `page` / `pagesize`
 * （分页由后端 `domain.AdminPaging` 收敛，前端不应假设上限）。
 */

async function postList<T>(endpoint: string, body: Record<string, string | number>): Promise<AdminListResponse<T>> {
  const { data } = await post<AdminListResponse<T>>(endpoint, body)
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}

/**
 * 用户列表的分类（后端 `domain/admin.go` 白名单）：
 * 0=全部、1=已开启 MFA、2=未开启 MFA、3=已禁用。
 */
export const ADMIN_USER_CATEGORY = { all: 0, mfaEnabled: 1, mfaDisabled: 2, disabled: 3 } as const

export function fetchAdminUsers(params: {
  category?: number
  keyword?: string
  sort?: string
  page?: number
  pagesize?: number
}) {
  return postList<AdminUserListItem>('/api/admin/userList', {
    category: params.category ?? ADMIN_USER_CATEGORY.all,
    keyword: params.keyword ?? '',
    sort: params.sort ?? '',
    page: params.page ?? 1,
    pagesize: params.pagesize ?? 20,
  })
}

export function fetchAdminGroups(params: { keyword?: string; sort?: string; page?: number; pagesize?: number }) {
  return postList<RepositoryGroup>('/api/admin/groupList', {
    keyword: params.keyword ?? '',
    sort: params.sort ?? '',
    page: params.page ?? 1,
    pagesize: params.pagesize ?? 20,
  })
}

export function fetchAdminRepositories(params: { keyword?: string; sort?: string; page?: number; pagesize?: number }) {
  return postList<Repository>('/api/admin/repositoryList', {
    keyword: params.keyword ?? '',
    sort: params.sort ?? '',
    page: params.page ?? 1,
    pagesize: params.pagesize ?? 20,
  })
}

export async function fetchSystemStatus(): Promise<AdminSystemStatus> {
  const { data } = await get<AdminSystemStatus>('/api/admin/systemStatus')
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}

/* ------------------------------ 用户操作 ------------------------------ */

/** 启用/禁用账号（`user` + `status`：1 正常、其它值禁用）。 */
export function updateUserStatus(userKey: string, status: 1 | 0) {
  return post('/api/admin/updateUserStatus', { user: userKey, status })
}

export function closeUserMFA(userKey: string) {
  return post('/api/admin/closeUserMFA', { user: userKey })
}

/** 管理员重置某账号口令（`user` + `password`，无需旧口令）。 */
export function resetAdminUserPassword(userKey: string, password: string) {
  return post('/api/admin/resetPassword', { user: userKey, password })
}

/** 设置/取消管理员（`user` + `admin`：字符串 "1"/"0"）。 */
export function setAdministrator(userKey: string, admin: boolean) {
  return post('/api/admin/setAdministrator', { user: userKey, admin: admin ? '1' : '0' })
}

/** 直接创建账号（`name` + `email`，口令由后端生成后发信）。 */
export function addAdminUser(name: string, email: string) {
  return post('/api/admin/addUser', { name, email })
}

/**
 * 新建账号后端会**明文返回初始口令**（`{"password": "..."}`），
 * 旧版是直接弹窗展示给管理员、并提供一键复制。
 */
export async function addAdminUserWithPassword(name: string, email: string): Promise<string> {
  const { data } = await post<{ password?: string }>('/api/admin/addUser', { name, email })
  return data?.password ?? ''
}

/** 校验当前管理员自己的口令（提权前必须过这一关）。 */
export function checkAdminPassword(password: string) {
  return post('/api/admin/checkPassword', { password })
}

/** 仓库组成员的完整列表（含 `creatorFlag`/`deleteFlag`/`groupMember` 标志位）。 */
export async function fetchAdminGroupMembers(groupKey: string): Promise<AdminMember[]> {
  const { data } = await post<AdminMember[]>('/api/admin/groupMembers', { group: groupKey })
  return data ?? []
}

/** 仓库成员的完整列表（含 `creatorFlag`/`deleteFlag`/`groupMember` 标志位）。 */
export async function fetchAdminRepositoryMembers(repositoryKey: string): Promise<AdminMember[]> {
  const { data } = await post<AdminMember[]>('/api/admin/repositoryMembers', { repository: repositoryKey })
  return data ?? []
}

/* ------------------------------ 组操作 ------------------------------ */

export function setGroupOwner(groupKey: string, userKey: string) {
  return post('/api/admin/setGroupOwner', { group: groupKey, user: userKey })
}

export function adminAddGroupMember(groupKey: string, email: string) {
  return post('/api/admin/groupAddMember', { group: groupKey, email })
}

export function adminRemoveGroupMember(groupKey: string, userKey: string) {
  return post('/api/admin/groupRemoveMember', { group: groupKey, user: userKey })
}

/* ------------------------------ 仓库操作 ------------------------------ */

export function adminAddRepositoryMember(repositoryKey: string, email: string) {
  return post('/api/admin/repositoryAddMember', { repository: repositoryKey, email })
}

export function adminRemoveRepositoryMember(repositoryKey: string, userKey: string) {
  return post('/api/admin/repositoryRemoveMember', { repository: repositoryKey, user: userKey })
}

/* ------------------------------ 站点配置 ------------------------------ */

/** 读取站点配置（整份 config.yaml 的快照，结构化对象）。 */
export async function fetchAdminConfig(): Promise<Record<string, unknown>> {
  const { data } = await get<Record<string, unknown>>('/api/admin/config')
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}

/**
 * 保存站点配置。
 *
 * 后端 body 字段是 `data`，值是**JSON 字符串**；服务端会做：
 * 必须是 JSON 对象 → 与现值浅合并 → 校验结构 → 备份 + 原子写 → 热更新。
 * 因此前端只需保证传的是合法 JSON 对象。
 */
export function saveAdminConfig(payload: Record<string, unknown>) {
  return post('/api/admin/config', { data: JSON.stringify(payload) })
}
