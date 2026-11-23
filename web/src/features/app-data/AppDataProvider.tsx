import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'

import { fetchGroupList } from '@/api/group'
import { fetchRepositoryList } from '@/api/repository'
import { fetchCurrentUser } from '@/api/user'
import type { CurrentUser, Group, RepositoryListItem } from '@/api/types'

/**
 * 应用级共享数据（对应旧版 Redux `DataStore` 里的三块内容）。
 *
 * 页头（用户/未读）与列表/上下文 Provider（仓库/组）共享同一份数据，避免重复请求。
 * `refresh()` 返回 Promise，便于新建/重命名后 await 完成再跳转，避免命中旧列表闪 404。
 */
export interface AppData {
  user: CurrentUser | null
  repositories: RepositoryListItem[]
  groups: Group[]
  loading: boolean
  error: string | null
  /** 重新拉取全部三份数据（新增/修改仓库或组之后调用），返回加载完成的 Promise。 */
  refresh: () => Promise<void>
  /** 只刷新用户信息（头像、未读数变化时调用）。 */
  refreshUser: () => void
  /** 同步某个仓库的星标状态到全局缓存，避免切页后回退为旧状态。 */
  updateRepositoryStar: (repositoryId: string, starred: boolean, starCount: number) => void
}

const AppDataContext = createContext<AppData | null>(null)

export function AppDataProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [repositories, setRepositories] = useState<RepositoryListItem[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const reqIdRef = useRef(0)

  const loadAll = useCallback(async (showLoading: boolean) => {
    const reqId = ++reqIdRef.current
    if (showLoading) {
      setLoading(true)
    }
    setError(null)
    try {
      const [nextUser, nextRepositories, nextGroups] = await Promise.all([
        fetchCurrentUser(),
        fetchRepositoryList(),
        fetchGroupList(),
      ])
      if (reqId !== reqIdRef.current) {
        return
      }
      setUser(nextUser)
      setRepositories(nextRepositories)
      setGroups(nextGroups)
    } catch (err: unknown) {
      if (reqId !== reqIdRef.current) {
        return
      }
      setError(err instanceof Error ? err.message : '加载失败')
    } finally {
      if (reqId === reqIdRef.current) {
        setLoading(false)
      }
    }
  }, [])

  const refresh = useCallback(() => loadAll(false), [loadAll])

  const refreshUser = useCallback(() => {
    fetchCurrentUser()
      .then(setUser)
      .catch(() => {
        /* 未登录时由 api client 统一跳转，这里静默 */
      })
  }, [])

  const updateRepositoryStar = useCallback((repositoryId: string, starred: boolean, starCount: number) => {
    setRepositories((prev) =>
      prev.map((item) => (item.id === repositoryId ? { ...item, starred, starCount } : item)),
    )
  }, [])

  useEffect(() => {
    void loadAll(true)
  }, [loadAll])

  const value = useMemo<AppData>(
    () => ({ user, repositories, groups, loading, error, refresh, refreshUser, updateRepositoryStar }),
    [user, repositories, groups, loading, error, refresh, refreshUser, updateRepositoryStar],
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
