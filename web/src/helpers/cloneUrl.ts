import type { CurrentUser } from '@/api/types'

/**
 * 克隆地址拼装（唯一事实源）。
 *
 * 为什么抽出来：这段逻辑原先只存在于 `view/unit/RepositoryHeader`（仓库页头的"克隆"
 * 下拉），首页要展示"主推仓库的 clone 命令"就成了第二次使用 —— 按 AGENTS §4
 * 「第二次出现即抽取」上收到 helpers。
 *
 * 口径对齐旧版仓库页头：
 *   - HTTPS = `<host>/<组名>/<仓库名>.git`（`host` 由 `user/info` 下发，形如 `http://host`）
 *   - SSH   = `<ssh>/<组名>/<仓库名>.git`；`ssh` 为空时从 `host` 推 `git@<host去掉协议>`
 *
 * **修复（2026-10-27）**：旧实现是裸字符串拼接 `${ssh}${path}`，因此当配置里的
 * `ssh` 不带结尾分隔符时（例如 `ssh://git@localhost`），会拼出
 * `ssh://git@localhostcodefever_team/repo.git` 这种不可用的地址 —— 首页把 clone 命令
 * 直接展示在首屏，这个拼接必须对三种常见写法都成立：
 *   `git@host:`（scp 形式，带冒号） / `git@host`（scp 形式，不带冒号） / `ssh://git@host`
 */

export interface CloneUrls {
  https: string
  ssh: string
}

/** 路径形式（http/https/ssh://）：去重结尾斜杠后用一个 `/` 连接。 */
function joinPath(base: string, path: string): string {
  const trimmed = base.replace(/\/+$/, '')
  return trimmed ? `${trimmed}/${path}` : path
}

/** SSH 地址：按配置实际写法决定分隔符（`/` 还是 `:`）。 */
function joinSSH(sshHost: string, path: string): string {
  if (!sshHost) {
    return path
  }
  if (/[:/]$/.test(sshHost)) {
    // 已经带分隔符：`git@host:` / `ssh://git@host/`
    return `${sshHost}${path}`
  }
  if (sshHost.includes('://')) {
    // URL 形式：`ssh://git@host` → 需要 `/`
    return joinPath(sshHost, path)
  }
  if (sshHost.includes('@')) {
    // scp 形式：`git@host` → 需要 `:`
    return `${sshHost}:${path}`
  }
  return joinPath(sshHost, path)
}

export function cloneUrls(
  user: Pick<CurrentUser, 'host' | 'ssh'> | null | undefined,
  groupName: string,
  repositoryName: string,
): CloneUrls {
  const host = user?.host ?? ''
  const sshHost = user?.ssh || (host ? `git@${host.replace(/^https?:\/\//, '')}` : '')
  const repoPath = `${groupName}/${repositoryName}.git`
  return {
    https: joinPath(host, repoPath),
    ssh: joinSSH(sshHost, repoPath),
  }
}
