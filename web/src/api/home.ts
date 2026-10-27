import { ApiError, errorMessage, get } from './client'
import type { HomeOverview } from './types'

/**
 * 首页聚合（`GET /api/home/overview`）。
 *
 * 一次请求取回待我评审 / 我发起的 / 团队动态 / 平台概览四类数据 —— 它们跨
 * user / repository / group 三个域，逐个复用现有端点需要 4~5 个请求，
 * 而且"待我评审"与"跨仓库动态"现有端点根本表达不了（见后端
 * `backend/internal/domain/home.go` 的注释）。
 *
 * 注意：返回的合并请求**不含** `commit` 与 `reviewers`（后端刻意不跑 git），
 * 渲染时必须用 `MergeRequestRow` 的 `compact` 变体。
 */
export async function fetchHomeOverview(): Promise<HomeOverview> {
  const { data } = await get<HomeOverview>('/api/home/overview')
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data
}
