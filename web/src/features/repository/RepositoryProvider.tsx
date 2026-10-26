import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

import { fetchRepositoryConfig, findRepository } from '@/api/repository'
import type { RepositoryConfig, RepositoryListItem } from '@/api/types'
import { useAppData } from '@/features/app-data/AppDataProvider'

/**
 * 当前仓库上下文。
 *
 * 解析顺序（与旧版一致）：先用「组名 + 仓库名」在全局仓库列表里找到仓库主键，
 * 再拉 `repository/config`。找不到仓库时 `notFound = true`，页面渲染 404 ——
 * 这是 react-router 8 不支持路径正则之后的替代判定（见计划 §8 偏离 #3）。
 */
export interface RepositoryContextValue {
  /** 全局列表里的那一行（含 role / members 摘要）。 */
  summary: RepositoryListItem | null
  /** config 接口的完整数据（仓库 + 组 + 成员 + 分支/标签 + 计数）。 */
  config: RepositoryConfig | null
  loading: boolean
  error: string | null
  notFound: boolean
  refresh: () => void
}

const RepositoryContext = createContext<RepositoryContextValue | null>(null)

export interface RepositoryProviderProps {
  groupName: string | undefined
  repositoryName: string | undefined
  children: ReactNode
}

export function RepositoryProvider({ groupName, repositoryName, children }: RepositoryProviderProps) {
  const { repositories, loading: appLoading } = useAppData()
  const summary = findRepository(repositories, groupName, repositoryName) ?? null

  const [config, setConfig] = useState<RepositoryConfig | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [tick, setTick] = useState(0)

  const refresh = useCallback(() => setTick((value) => value + 1), [])

  useEffect(() => {
    if (!summary) {
      setConfig(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchRepositoryConfig(summary.id)
      .then((next) => {
        if (!cancelled) {
          setConfig(next)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : '仓库加载失败')
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [summary, tick])

  const value = useMemo<RepositoryContextValue>(
    () => ({
      summary,
      config,
      loading: appLoading || loading,
      error,
      notFound: !appLoading && !summary,
      refresh,
    }),
    [summary, config, appLoading, loading, error, refresh],
  )

  return <RepositoryContext.Provider value={value}>{children}</RepositoryContext.Provider>
}

export function useRepository(): RepositoryContextValue {
  const ctx = useContext(RepositoryContext)
  if (!ctx) {
    throw new Error('useRepository 必须在 <RepositoryProvider> 内使用')
  }
  return ctx
}

/**
 * 可选的仓库上下文：**没有** `RepositoryProvider` 时返回 `null` 而不是抛错。
 *
 * 用于"同一套表单既能在仓库路由内用、也能在仓库外用"的场景 ——
 * 例如全局新建 MR（`/mergerequests/new`）复用 `MergeRequestCreate`，
 * 它在仓库路由里能拿到当前仓库，在全局路由里拿不到（此时由用户选仓库）。
 */
export function useOptionalRepository(): RepositoryContextValue | null {
  return useContext(RepositoryContext)
}
