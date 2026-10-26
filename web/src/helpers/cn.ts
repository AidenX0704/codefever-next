/** 拼接 className 的小工具（替代 classnames 依赖，保持零 UI 依赖）。 */
export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ')
}
