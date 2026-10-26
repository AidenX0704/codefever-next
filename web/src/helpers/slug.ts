/**
 * 标识名（slug / URL 片段）规范化。
 *
 * 逐字对齐旧版 `helpers/NetworkHelper.js` 的 `makeSlug`：
 * `input.replace(/[^\w]/g, '_')` —— 非「字母/数字/下划线」的字符一律换成下划线。
 *
 * 注意 `\w` 在 JS 里只含 ASCII 字母、数字与下划线，所以中文会被逐字换成 `_`，
 * 这与旧版行为一致（不要"优化"成 Unicode 属性类，否则旧链接与旧数据的映射会变）。
 *
 * 落位：`web/src/helpers/`（AGENTS §4：纯逻辑下沉到 helpers，页面不各自实现）。
 */
export function makeSlug(input: string): string {
  return input.replace(/[^\w]/g, '_')
}
