import type { RepositoryMember } from '@/api/types'

/**
 * 按提交邮箱解析出成员信息。
 *
 * 逐字移植旧 `helpers/VaribleHelper.getUserInfo()`：用成员邮箱列表匹配提交邮箱，
 * 命中返回成员；未命中视为"临时用户"（提交邮箱没有绑定任何账号），
 * 名称为邮箱本身，`temporary` 用于在 UI 上提示"这是我的邮箱"。
 */
export interface ResolvedMember {
  name: string
  icon?: string | null
  email: string
  temporary: boolean
}

export function resolveMember(members: RepositoryMember[] | undefined, email: string | undefined): ResolvedMember {
  const target = (email ?? '').toLowerCase()
  if (target) {
    for (const member of members ?? []) {
      const emails = [member.email, ...(member.emails ?? [])].map((item) => item.toLowerCase())
      if (emails.includes(target)) {
        return { name: member.name, icon: member.icon, email: member.email, temporary: false }
      }
    }
  }
  return { name: email ?? '', email: email ?? '', temporary: true }
}
