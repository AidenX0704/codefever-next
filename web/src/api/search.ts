import { ApiError, errorMessage, get } from './client'
import type { MergeRequestListItem } from './types'

/**
 * 全局搜索（阶段一：元数据）。
 *
 * 范围：仓库（名称/显示名/描述）、仓库组（名称/显示名/描述）、合并请求**标题**。
 * **不搜代码内容** —— 那需要 `git grep` 或独立索引，属阶段二。
 *
 * 权限：三类结果都只包含我可见的对象（后端 SQL 里 JOIN 成员表），
 * 因此前端**不需要也不应该**再过滤一遍。
 */

export interface SearchRepository {
  id: string
  name: string
  displayName: string
  description: string
  avatar: string | null
  groupName: string
  groupDisplayName: string
  updated: number
}

export interface SearchGroup {
  id: string
  name: string
  displayName: string
  description: string
  avatar: string | null
  repositoryCount: number
  updated: number
}

/**
 * 合并请求结果：字段与 `MergeRequestListItem` 同形，可直接喂给
 * `MergeRequestRow` 的 `compact` 变体（`reviewers` 恒空、`commit` 恒为零值）。
 */
export interface SearchMergeRequest extends MergeRequestListItem {
  /** 发起人主键，用 AppData 里的成员字典解析成名字。 */
  creator: string
}

export interface SearchResults {
  keyword: string
  repositories: SearchRepository[]
  groups: SearchGroup[]
  mergeRequests: SearchMergeRequest[]
  truncated: {
    repositories: boolean
    groups: boolean
    mergeRequests: boolean
  }
}

/** 执行搜索。`keyword` 会由后端再次 trim；空关键词由后端返回 0x0201。 */
export async function fetchSearchResults(keyword: string): Promise<SearchResults> {
  const { data } = await get<SearchResults>('/api/search/all', { keyword })
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}
