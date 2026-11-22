/**
 * 公开 API（`/api/v1`）文档路径拼装。
 */

import { docUrl } from '@/helpers/docUrl'

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
