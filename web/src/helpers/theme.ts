/**
 * 主题（浅色 / 深色）的唯一入口。
 *
 * 约定（AGENTS §5）：
 *   - 主题只通过 `<html data-theme="light|dark">` 表达，令牌在 theme.css 里成对定义；
 *   - **首屏渲染前**就要应用，否则会先闪一下浅色再切深色（V3 §9）；
 *   - 组件里禁止写 `@media (prefers-color-scheme: dark)`，一律读这里的属性。
 *
 * 优先级：用户显式选择（localStorage）> 系统偏好。
 */

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'cf-theme'

function isTheme(value: unknown): value is Theme {
  return value === 'light' || value === 'dark'
}

/** 读取用户已保存的主题；没有则返回 null（表示"跟随系统"）。 */
export function getStoredTheme(): Theme | null {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    return isTheme(raw) ? raw : null
  } catch {
    // 隐私模式 / 禁用存储时不影响渲染
    return null
  }
}

/** 系统偏好。 */
export function getSystemTheme(): Theme {
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** 当前生效的主题。 */
export function getTheme(): Theme {
  const attr = document.documentElement.getAttribute('data-theme')
  return isTheme(attr) ? attr : 'light'
}

/** 应用并持久化主题。传 null 表示清除用户选择、回到跟随系统。 */
export function applyTheme(theme: Theme | null): Theme {
  const next = theme ?? getSystemTheme()
  document.documentElement.setAttribute('data-theme', next)
  try {
    if (theme) window.localStorage.setItem(STORAGE_KEY, theme)
    else window.localStorage.removeItem(STORAGE_KEY)
  } catch {
    // 存不了就算了，本次会话仍然生效
  }
  return next
}

/** 首屏入口：在 createRoot 之前调用。 */
export function initTheme(): void {
  document.documentElement.setAttribute('data-theme', getStoredTheme() ?? getSystemTheme())
}
