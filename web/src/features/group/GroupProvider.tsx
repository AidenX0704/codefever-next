import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

import { fetchGroupConfig, findGroup } from '@/api/group'
import type { Group, GroupConfig } from '@/api/types'
import { useAppData } from '@/features/app-data/AppDataProvider'

/**
 * 当前仓库组上下文（与 `RepositoryProvider` 同构）。
 *
 * 先用组名在全局组列表里找到组主键（旧版同样做法），再拉 `group/config`
 * （组 + 成员 + 成员数）。找不到即 `notFound`，页面渲染 404。
 */
export interface GroupContextValue {
  summary: Group | null
  config: GroupConfig | null
  loading: boolean
  error: string | null
  notFound: boolean
  refresh: () => void
}

const GroupContext = createContext<GroupContextValue | null>(null)

export function GroupProvider({ groupName, children }: { groupName: string | undefined; children: ReactNode }) {
  const { groups, loading: appLoading } = useAppData()
  const summary = findGroup(groups, groupName) ?? null

  const [config, setConfig] = useState<GroupConfig | null>(null)
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
    fetchGroupConfig(summary.id)
      .then((next) => {
        if (!cancelled) {
          setConfig(next)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : '仓库组加载失败')
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

  const value = useMemo<GroupContextValue>(
    () => ({ summary, config, loading: appLoading || loading, error, notFound: !appLoading && !summary, refresh }),
    [summary, config, appLoading, loading, error, refresh],
  )

  return <GroupContext.Provider value={value}>{children}</GroupContext.Provider>
}

export function useGroup(): GroupContextValue {
  const ctx = useContext(GroupContext)
  if (!ctx) {
    throw new Error('useGroup 必须在 <GroupProvider> 内使用')
  }
  return ctx
}
