import { ApiError, errorMessage, get } from './client'

/**
 * 文档中心接口（Go 侧新增的两个只读 JSON 端点，复用 SSR 页面的目录解析）。
 *
 *   GET /api/doc/tree?lang=cn|en            → { menu: [{name, path, submenu?}] }
 *   GET /api/doc/content?lang=cn|en&path=…  → { path, content }（Markdown 原文）
 *
 * `lang` 用旧 URL 的语义（cn / en），Go 侧再映射到目录名 zh-cn / en-us。
 */

export interface DocMenuItem {
  name: string
  path: string
  submenu?: DocMenuItem[]
}

export async function fetchDocTree(lang: 'cn' | 'en'): Promise<DocMenuItem[]> {
  const { data } = await get<{ menu: DocMenuItem[] }>('/api/doc/tree', { lang })
  return data?.menu ?? []
}

export async function fetchDocContent(lang: 'cn' | 'en', path: string): Promise<string> {
  const { data } = await get<{ path: string; content: string }>('/api/doc/content', { lang, path })
  if (!data) {
    throw new ApiError(0x0200, errorMessage(0x0200))
  }
  return data.content
}
