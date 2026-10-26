/**
 * UAC：角色与权限矩阵。
 *
 * 逐字对应旧版 `www/view/src/config/UAC.js` —— 角色整数值由后端下发
 * （`NormalizeRepositoryMember.role` / `groupListItem.role`），
 * 前端只做"这个角色能不能做这件事"的判定。
 */

export const Role = {
  NO_PERMISSION: 0,
  GUEST: 1,
  REPORTER: 2,
  DEVELOPER: 3,
  MAINTAINER: 4,
  OWNER: 5,
  NOBODY: 6,
} as const

export type RoleValue = (typeof Role)[keyof typeof Role]

export const PermissionCode = {
  NO_PERMISSION: 0x00,

  REPO_READ: 0x01,
  REPO_PUSH: 0x02,
  REPO_REMOVE: 0x03,
  REPO_CHANGE_MEMBER: 0x04,
  REPO_CHANGE_INFO: 0x05,

  MR_READ: 0x08,
  MR_PUSH: 0x09,
  MR_MERGE: 0x0a,

  GROUP_CHANGE_MEMBER: 0x10,
  GROUP_CREATE_REPO: 0x11,
} as const

export type PermissionValue = (typeof PermissionCode)[keyof typeof PermissionCode]

export const PermissionConfig: Record<number, ReadonlyArray<PermissionValue>> = {
  [Role.NO_PERMISSION]: [],
  [Role.GUEST]: [PermissionCode.REPO_READ],
  [Role.REPORTER]: [PermissionCode.REPO_READ, PermissionCode.MR_READ],
  [Role.DEVELOPER]: [PermissionCode.REPO_READ, PermissionCode.MR_READ, PermissionCode.MR_PUSH],
  [Role.MAINTAINER]: [
    PermissionCode.REPO_READ,
    PermissionCode.REPO_PUSH,
    PermissionCode.REPO_CHANGE_MEMBER,
    PermissionCode.REPO_CHANGE_INFO,
    PermissionCode.MR_READ,
    PermissionCode.MR_PUSH,
    PermissionCode.MR_MERGE,
    PermissionCode.GROUP_CHANGE_MEMBER,
    PermissionCode.GROUP_CREATE_REPO,
  ],
  [Role.OWNER]: [
    PermissionCode.REPO_READ,
    PermissionCode.REPO_PUSH,
    PermissionCode.REPO_REMOVE,
    PermissionCode.REPO_CHANGE_MEMBER,
    PermissionCode.REPO_CHANGE_INFO,
    PermissionCode.MR_READ,
    PermissionCode.MR_PUSH,
    PermissionCode.MR_MERGE,
    PermissionCode.GROUP_CHANGE_MEMBER,
    PermissionCode.GROUP_CREATE_REPO,
  ],
}

/** 角色 → 是否具备某项权限。 */
export function hasPermission(role: number, permission: PermissionValue): boolean {
  return (PermissionConfig[role] ?? []).includes(permission)
}

/** 是否有读权限（列表过滤用：旧版 `FilterGenerator.withPermission(REPO_READ)`）。 */
export function canReadRepository(role: number): boolean {
  return hasPermission(role, PermissionCode.REPO_READ)
}

export const UAC = { Role, PermissionCode, PermissionConfig, hasPermission, canReadRepository }
export default UAC
