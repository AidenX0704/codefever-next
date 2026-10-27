/**
 * 「最近访问的仓库」——纯前端记录（localStorage）。
 *
 * 为什么不用服务端表（docs/HOME_PAGE_PLAN_2026-10-27.md 的取舍 D1）：
 *   - 服务端记录需要在每次进入仓库时写一次库（写放大），还要新表 + 保留期清理；
 *   - "最近访问"是**个人态**数据、跨设备价值低，丢了也没有数据损失。
 *
 * 代价与回退路径：换设备/换浏览器看不到历史。若产品后来要求跨设备，
 * 只需建 `cc_repository_visits` 并把本文件的读写换成 API 调用 ——
 * 调用方（RepositoryLayout 记录、首页展示）无需改动。
 *
 * 容错：localStorage 可能被禁用、被写入脏数据、或存着旧版本的形状；
 * 所有读写都 try/catch，失败一律降级成"没有历史"，绝不抛错到渲染层。
 */

export interface RecentRepository {
  groupName: string
  repositoryName: string
  /** 访问时间（毫秒时间戳），用于排序与"多久之前"的展示。 */
  visitedAt: number
}

const STORAGE_KEY = 'cf:recent-repos:v1'
/** 最多保留几条（首页侧栏只放得下 6 行）。 */
const MAX_ENTRIES = 6

/** 读最近访问（最新在前）。读不到/数据损坏时返回空数组。 */
export function readRecentRepositories(): RecentRepository[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return []
    }
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }
    const out: RecentRepository[] = []
    for (const item of parsed) {
      if (
        item &&
        typeof item === 'object' &&
        typeof (item as RecentRepository).groupName === 'string' &&
        typeof (item as RecentRepository).repositoryName === 'string' &&
        typeof (item as RecentRepository).visitedAt === 'number'
      ) {
        out.push({
          groupName: (item as RecentRepository).groupName,
          repositoryName: (item as RecentRepository).repositoryName,
          visitedAt: (item as RecentRepository).visitedAt,
        })
      }
    }
    return out.sort((a, b) => b.visitedAt - a.visitedAt).slice(0, MAX_ENTRIES)
  } catch {
    return []
  }
}

/** 记录一次访问（同仓库去重后置顶，最多保留 MAX_ENTRIES 条）。 */
export function recordRecentRepository(groupName: string, repositoryName: string): void {
  if (!groupName || !repositoryName) {
    return
  }
  try {
    const next: RecentRepository[] = [
      { groupName, repositoryName, visitedAt: Date.now() },
      ...readRecentRepositories().filter(
        (item) => !(item.groupName === groupName && item.repositoryName === repositoryName),
      ),
    ].slice(0, MAX_ENTRIES)
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
  } catch {
    // 隐私模式 / 配额满：静默失败，"最近访问"不是关键路径。
  }
}
