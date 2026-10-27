import { ApiError, errorMessage, get, post } from './client'
import type { AccessToken, CurrentUser, MFAData, SSHKey } from './types'

/** 取当前登录用户（含未读通知数、提交邮箱）。`GET /api/user/info`。 */
export async function fetchCurrentUser(): Promise<CurrentUser> {
  const { data } = await get<CurrentUser>('/api/user/info')
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}

/* ------------------------------ 个人设置（写操作） ------------------------------ */

/**
 * 更新基本信息（`name` / `email` 必填，`team` / `role` 可选）。
 *
 * 后端 `handlers/user_profile.go:148-227` 的 `updateBasicInfo` 支持
 * `name` / `email` / `team` / `role` 四个字段；`role` 是旧版资料页的可编辑项
 * （旧 `UserSettingGeneral.js:476-493` 随基本信息一起提交）。
 */
export function updateBasicInfo(body: { name: string; email: string; team?: string; role?: string }) {
  return post('/api/user/updateBasicInfo', body)
}

/**
 * 修改登录口令（`current` 旧口令、`new` 新口令）。
 */
export function updatePassword(current: string, next: string) {
  return post('/api/user/updatePassword', { current, new: next })
}

/** 上传头像：multipart，字段名 `avatar`（`_BINARY` 后缀会被客户端剥掉）。 */
export function uploadAvatar(file: File) {
  return post('/api/user/uploadAvatar', { avatar_BINARY: file })
}

/** SSH 公钥列表。 */
export async function listSSHKeys(): Promise<SSHKey[]> {
  const { data } = await get<SSHKey[]>('/api/user/listKey')
  return data ?? []
}

export function addSSHKey(name: string, key: string) {
  return post('/api/user/addSSHKey', { name, key })
}

export function removeSSHKey(id: string) {
  return post('/api/user/removeSSHKey', { id })
}

/** 发送"更改主邮箱"验证码（需当前口令）。 */
export function requestEmailCode(email: string, password: string) {
  return post('/api/user/getEmailCode', { email, password })
}

/** 用验证码更换主邮箱。 */
export function changeEmail(email: string, code: string, password: string) {
  return post('/api/user/changeEmail', { email, code, password })
}

/** 添加提交邮箱（会发验证码到该邮箱）。 */
export function addCommitEmail(email: string) {
  return post('/api/user/addCommitEmail', { email })
}

export function resendCommitEmailCode(email: string) {
  return post('/api/user/resentCommitEmailCode', { email })
}

export function deleteCommitEmail(id: string) {
  return post('/api/user/deleteCommitEmail', { id })
}

export function validateCommitEmailCode(id: string, email: string, code: string) {
  return post('/api/user/validationCommitEmailCode', { id, email, code })
}

/** MFA 数据（GET）。 */
export async function getMFAData(): Promise<MFAData> {
  const { data } = await get<MFAData>('/api/user/getMFAData')
  return data ?? {}
}

export function updateMFAData(secret: string, code1: string, code2: string) {
  return post('/api/user/updateMFAData', { secret, code1, code2 })
}

export function revokeMFAData() {
  return post('/api/user/revokeMFAData', {})
}

/* ------------------------------ 通知设置 ------------------------------ */

/** 通知设置的位掩码位（后端 `repo.NotificationStatus*`）：**位=1 表示关闭**。 */
export const NOTIFICATION_BIT = { mr: 1, email: 2 } as const

/** 切换某一项通知开关（`offset` 是位序号：1=合并请求、2=邮件）。 */
export function toggleNotificationSetting(offset: 1 | 2) {
  return post('/api/user/userNotificationSetting', { offset })
}

export interface NotificationRefuse {
  id: string
  type: number
  target: string
}

/** 当前被拒收通知的仓库组/仓库列表（`type` 1=组、2=仓库）。 */
export async function fetchNotificationRefused(): Promise<NotificationRefuse[]> {
  const { data } = await get<NotificationRefuse[]>('/api/user/notificationRefused')
  return data ?? []
}

/** 恢复/拒收某个仓库组或仓库的通知（级联语义由后端处理）。 */
export function setGroupOrRepoNotification(type: 1 | 2, target: string, open: boolean) {
  return post('/api/user/setGroupOrRepoNotification', { type, target, open })
}

/* ------------------------------ 通知（列表 / 已读 / 删除） ------------------------------ */

/** 通知分类（后端 `NotificationCategory*`）。 */
export const NOTIFICATION_CATEGORY = { all: 0, unread: 1, system: 2, mention: 3 } as const

export interface NotificationItem {
  id: string
  type: number
  group: string | null
  repository: string | null
  /** **JSON 字符串**（后端约定前端自行 JSON.parse）。 */
  data: string
  created: number
  isRead: boolean
}

export async function fetchNotifications(category: number, page = 1): Promise<NotificationItem[]> {
  const { data } = await get<NotificationItem[]>('/api/user/notifications', { category, page })
  return data ?? []
}

/** 标记已读：`all=true` 与 `id` **有且只能有一个**（后端强校验）。 */
export function setNotificationRead(params: { all: true } | { id: string }) {
  if ('all' in params) {
    return post('/api/user/setNotificationRead', { all: '1' })
  }
  return post('/api/user/setNotificationRead', { id: params.id })
}

export function deleteNotification(params: { all: true } | { id: string }) {
  if ('all' in params) {
    return post('/api/user/deleteNotification', { all: '1' })
  }
  return post('/api/user/deleteNotification', { id: params.id })
}

/* ------------------------------ 访问令牌（公开 API /api/v1） ------------------------------ */

/**
 * 列出我名下的访问令牌。
 *
 * 返回项**不含明文也不含摘要** —— 后端只存 SHA-256 且不回传，明文只在创建时出现一次。
 */
export async function listAccessTokens(): Promise<AccessToken[]> {
  const { data } = await get<AccessToken[]>('/api/user/listToken')
  return data ?? []
}

/**
 * 创建访问令牌。
 *
 * 返回体的 `token` 是**明文，只此一次**：关掉弹窗就再也拿不到了，
 * 所以页面上必须显式提示"复制保存，之后无法再查看"。
 */
export async function createAccessToken(name: string): Promise<{ token: string; item: AccessToken }> {
  const { data } = await post<{ token: string; item: AccessToken }>('/api/user/createToken', { name })
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}

/** 吊销访问令牌（只能吊销自己名下的；后端按 `u_key + at_key` 约束）。 */
export function deleteAccessToken(id: string) {
  return post('/api/user/deleteToken', { id })
}
