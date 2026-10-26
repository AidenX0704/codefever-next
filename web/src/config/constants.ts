/**
 * 跨页面常量。
 *
 * 旧版 `config/Constants.js` 的 `getStaticHost()` 用的是
 * `window.location.origin + '/'`（供头像 URL 拼接），这里改成**同源相对路径**：
 * 头像地址不再依赖绝对主机名，部署在子路径或反代后面也不会拼错。
 */
export const AVATAR_URL_PREFIX = '/file/fetch/avatar/'

/** 把后端返回的头像 key 拼成可访问 URL。 */
export function avatarUrl(key: string | null | undefined): string | undefined {
  return key ? `${AVATAR_URL_PREFIX}${key}` : undefined
}
