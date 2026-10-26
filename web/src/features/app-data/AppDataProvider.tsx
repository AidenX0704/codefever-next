import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

import { fetchGroupList } from '@/api/group'
import { fetchRepositoryList } from '@/api/repository'
import { fetchCurrentUser } from '@/api/user'
import type { CurrentUser, Group, RepositoryListItem } from '@/api/types'

/**
 * 应用级共享数据（对应旧版 Redux `DataStore` 里的三块内容）。
 *
 * 旧版在 `components/Main.js` 的 `componentDidUpdate` 里按路由参数变化拉取，
 * 再把结果 dispatch 进 store，所有页面从 store 读。这里用 Context + 一次并发拉取
 * 表达同一件事：页头（用户/未读）与列表页（仓库/组）共享同一份数据，避免重复请求。
 *
 * 后续批次接入 TanStack Query 时，本模块是唯一的替换点（对外 API 不变）。
 */
export interface AppData {
  user: CurrentUser | null
  repositories: RepositoryListItem[]
  groups: Group[]
  loading: boolean
  error: string | null
  /** 重新拉取全部三份数据（新增仓库/组之后调用）。 */
  refresh: () => void
  /** 只刷新用户信息（头像、未读数变化时调用）。 */
  refreshUser: () => void
}

const AppDataContext = createContext<AppData | null>(null)

export function AppDataProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [repositories, setRepositories] = useState<RepositoryListItem[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [tick, setTick] = useState(0)

  const refresh = useCallback(() => setTick((value) => value + 1), [])

  const refreshUser = useCallback(() => {
    fetchCurrentUser()
      .then(setUser)
      .catch(() => {
        /* 未登录时由 api client 统一跳转，这里静默 */
      })
  }, [])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)

    Promise.all([fetchCurrentUser(), fetchRepositoryList(), fetchGroupList()])
      .then(([nextUser, nextRepositories, nextGroups]) => {
        if (cancelled) {
          return
        }
        setUser(nextUser)
        setRepositories(nextRepositories)
        setGroups(nextGroups)
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return
        }
        setError(err instanceof Error ? err.message : '加载失败')
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [tick])

  const value = useMemo<AppData>(
    () => ({ user, repositories, groups, loading, error, refresh, refreshUser }),
    [user, repositories, groups, loading, error, refresh, refreshUser],
  )

  return <AppDataContext.Provider value={value}>{children}</AppDataContext.Provider>
}

export function useAppData(): AppData {
  const ctx = useContext(AppDataContext)
  if (!ctx) {
    throw new Error('useAppData 必须在 <AppDataProvider> 内使用')
  }
  return ctx
}
