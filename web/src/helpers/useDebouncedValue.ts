import { useEffect, useState } from 'react'

/**
 * 防抖值：连续击键只在一段静默期后提交新值。
 *
 * 旧版在三个管理后台列表页各写了一遍 200ms 防抖
 * （`admin/Users.js:164-175`、`admin/Groups.js:127-137`、`admin/Repositories.js`），
 * 这里抽成共用 hook（AGENTS §4："第二处重复即抽取"）。
 *
 * 注意：防抖只延迟**值**，不延迟副作用。调用方应当把请求挂在防抖值上，
 * 并在"过滤条件变了就回到第 1 页"这类联动上用**未防抖**的原始值，避免多打一次接口。
 */
export function useDebouncedValue<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay)
    return () => window.clearTimeout(timer)
  }, [value, delay])

  return debounced
}
