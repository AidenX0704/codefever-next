/**
 * 标识名（slug / URL 片段）规范化与前端校验规则。
 *
 * 与后端 `domain.RepoSlugValid` / `domain.GroupSlugValid` / `domain.GroupNameIsPreserved`
 * 保持完全一致：允许字母、数字、下划线与中划线，且以字母或数字开头。
 *
 * 落位：`web/src/helpers/`（AGENTS §4：纯逻辑下沉到 helpers，页面不各自实现）。
 */

export const REPO_SLUG_RE = /^[0-9a-zA-Z][0-9a-zA-Z_-]{0,29}$/
export const GROUP_SLUG_RE = /^[0-9a-zA-Z][0-9a-zA-Z_-]{4,29}$/
export const BRANCH_NAME_RE = /^\w[\w/.-]{0,29}$/

const RESERVED_GROUP_PATTERNS: ReadonlyArray<RegExp> = [
  /^(feature|doc|community|lang|service|about|pricing|user|users|file|avatar|boss|captcha)$/i,
  /^(static|static-\d+)$/i,
  /^(api|apiv\d+|apis)$/i,
  /^(userInfo|userInfos)$/i,
  /^(pay|payment|transaction|order|coupon|invoice)$/i,
  /^(group|groups|repository|repositories|setting|settings|mergerequest|mergerequests|admin)$/i,
]

/** 判断仓库组标识名是否命中系统保留路径。 */
export function isReservedGroupSlug(input: string): boolean {
  const trimmed = input.trim()
  return RESERVED_GROUP_PATTERNS.some((re) => re.test(trimmed))
}

/**
 * 从显示名推导推荐的 URL 标识名（用于自动填充）：
 * 空白与常见分隔符转中划线、剔除非法字符、合并连续中划线、去掉首尾符号。
 */
export function suggestSlug(input: string): string {
  return input
    .trim()
    .toLowerCase()
    .replace(/[\s./]+/g, '-')
    .replace(/[^0-9a-zA-Z_-]/g, '')
    .replace(/-{2,}/g, '-')
    .replace(/^[^0-9a-zA-Z]+/, '')
    .slice(0, 30)
}

/**
 * 清洗用户正在输入的 slug（保留字母、数字、下划线与中划线，不吞掉正在输入的中划线）。
 */
export function makeSlug(input: string): string {
  return input.replace(/[^0-9a-zA-Z_-]/g, '').slice(0, 30)
}
