import type { Lang } from '@/i18n'

/**
 * 文档中心 URL 拼装（唯一事实源）。
 *
 * `doc/` 目录是 `zh-cn` / `en-us`，但 URL 段是 `cn` / `en`（Go 侧做映射）。
 * 这段映射原先散落在 `config/publicApi.ts` 与页头帮助菜单里，首页又用了一次 ——
 * 按 AGENTS §4 上收到 helpers，避免出现第三种写法。
 *
 * 形如：`docUrl('zh-cn', 'repo/webhooks.md')` → `/doc/cn/repo/webhooks.md`
 *       `docUrl('en-us')`                    → `/doc/en`
 */
export function docUrl(lang: Lang, path = ''): string {
  const segment = lang === 'en-us' ? 'en' : 'cn'
  const clean = path.replace(/^\/+/, '')
  return clean ? `/doc/${segment}/${clean}` : `/doc/${segment}`
}
