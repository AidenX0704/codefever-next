/**
 * 展示格式化（纯逻辑，AGENTS §4：格式化归 `helpers/`）。
 *
 * 旧版散落在 `helpers/VaribleHelper.formatNumber` 里（把数字按 1000 拆成
 * k/M/G，仓库体积用它是把 **字节** 当成千进制单位，跟"字节"的语义对不上）。
 * 这里给出两个语义明确的版本，页面只引用，不再各自手拼。
 */

/** 千分位分隔（1234567 → "1,234,567"）。 */
export function formatNumber(value: number): string {
  if (!Number.isFinite(value)) {
    return '0'
  }
  return Math.round(value).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',')
}

/**
 * 字节数 → 人类可读的体积。
 *
 * 后端 `repository/config` 的 `count.file` 是 `(git 目录各部分 KB 数) * 1024`，
 * 即**字节**（见 `backend/internal/handlers/repository.go` 的 config 处理）。
 */
export function formatBytes(bytes: number): string {
  const value = Number.isFinite(bytes) ? Math.max(0, bytes) : 0
  if (value < 1024) {
    return `${Math.round(value)} B`
  }
  const units = ['KB', 'MB', 'GB', 'TB']
  let scaled = value / 1024
  let index = 0
  while (scaled >= 1024 && index < units.length - 1) {
    scaled /= 1024
    index += 1
  }
  return `${scaled.toFixed(1)} ${units[index]}`
}

/**
 * 按"天"分组的日期键（`yyyy-MM-dd`）。
 *
 * 旧版用 `date-fns` 的 `format(ts, 'yyyy-MM-dd')`（CommitList.js:126）；
 * 这里用本地时区的年月日拼装，避免为一个格式再引一次 date-fns。
 */
export function dayKey(timestampSeconds: number): string {
  const date = new Date(timestampSeconds * 1000)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
