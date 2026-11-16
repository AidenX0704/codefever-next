import { get } from './client'

/**
 * 站点公开信息（`/api/site/*`，匿名可访问）。
 *
 * - stats：宣传页信任条与首页平台概览的公开计数；
 * - changelog：更新日志（分页，releasedAt 为 "YYYY-MM-DD" 或 null）；
 * - announcement：启用中的平台公告（≤10 条）。
 *
 * 这些端点未登录也可调用，因此放在独立的模块，不依赖 user 域的类型。
 */

export interface SiteStats {
  repositoryCount: number
  groupCount: number
  memberCount: number
  mergedMergeRequestCount: number
}

export interface ChangelogItem {
  id: number
  version: string
  title: string
  body: string
  /** "YYYY-MM-DD"；null 表示未设置发布日期。 */
  releasedAt: string | null
  /** 秒级时间戳（创建时间）。 */
  createdAt: number
}

export interface ChangelogPage {
  total: number
  page: number
  pagesize: number
  list: ChangelogItem[]
}

export type AnnouncementType = 'security' | 'privacy' | 'support'

export interface AnnouncementItem {
  id: number
  type: AnnouncementType
  body: string
  enabled: boolean
  createdAt: number
}

/** 公开平台统计（匿名）。 */
export async function fetchSiteStats() {
  const { data } = await get<SiteStats>('/api/site/stats')
  return data
}

/** 更新日志分页列表（匿名；pagesize 经后端 ClampPaging 收敛）。 */
export async function fetchChangelogs(params: { page?: number; pagesize?: number } = {}) {
  const query = new URLSearchParams()
  if (params.page) {
    query.set('page', String(params.page))
  }
  if (params.pagesize) {
    query.set('pagesize', String(params.pagesize))
  }
  const qs = query.toString()
  const { data } = await get<ChangelogPage>(`/api/site/changelog${qs ? `?${qs}` : ''}`)
  return data
}

/** 启用中的平台公告（匿名）。 */
export async function fetchAnnouncements() {
  const { data } = await get<AnnouncementItem[]>('/api/site/announcement')
  return data
}
