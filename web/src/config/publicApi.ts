/**
 * 公开 API（`/api/v1`）的端点清单（帮助中心「API 接口文档」区块的唯一数据源）。
 *
 * 为什么单独成文件（AGENTS §4 常量落位）：端点表既被帮助页渲染，也被文档页链接，
 * 放进页面里会导致"改一处漏一处"。
 *
 * 与后端代码的对应关系：端点定义在
 * `backend/internal/httpapi/publicapi/router.go`（路由表），
 * 字段与状态码定义在同目录的 `server.go`。
 * **新增/删除端点时必须同步本文件与 `doc/{zh-cn,en-us}/api/`。**
 */

import { docUrl } from '@/helpers/docUrl'

/** 端点的鉴权要求。 */
export type PublicApiAuth = 'required' | 'optional' | 'public'

export interface PublicApiEndpoint {
  /** HTTP 方法（当前公开 API 只有只读端点）。 */
  method: 'GET'
  /** 请求路径（`{owner}` 这类占位符按原样展示）。 */
  path: string
  /** 鉴权要求：必须令牌 / 令牌可选 / 匿名可读。 */
  auth: PublicApiAuth
  /** 说明文案的 i18n 键（词表见 `i18n/supplement.ts`）。 */
  descKey: string
}

/** 当前已实现的公开 API 端点（顺序与文档呈现一致）。 */
export const PUBLIC_API_ENDPOINTS: ReadonlyArray<PublicApiEndpoint> = [
  { method: 'GET', path: '/api/v1/user', auth: 'required', descKey: 'help.api.rowUser' },
  { method: 'GET', path: '/api/v1/user/repos', auth: 'required', descKey: 'help.api.rowUserRepos' },
  { method: 'GET', path: '/api/v1/users/{username}', auth: 'public', descKey: 'help.api.rowUserByName' },
  { method: 'GET', path: '/api/v1/repos/{owner}/{repo}', auth: 'optional', descKey: 'help.api.rowRepo' },
]

/**
 * 完整 API 文档在文档中心的相对路径（相对 `doc/{zh-cn,en-us}/`）。
 *
 * 文档路由形如 `/doc/cn/api/overview.md`（`cn`/`en` 是 URL 语义，Go 侧映射到目录名）。
 */
export const PUBLIC_API_DOC_PATH = 'api/overview.md'

/** 按界面语言拼出完整 API 文档的站内地址（语言段映射复用 `helpers/docUrl`）。 */
export function publicApiDocUrl(lang: 'zh-cn' | 'en-us'): string {
  return docUrl(lang, PUBLIC_API_DOC_PATH)
}
