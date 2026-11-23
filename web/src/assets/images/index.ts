/**
 * 复用的静态素材（业务代码只从这里取图，不拼接 `/static/images/` URL）。
 */
import branchesEmpty from './branches-empty.png'
import commitsEmpty from './commits-empty.png'
import defaultAvatar from './default_avatar.png'
import headerLogo from './header-codefever-logo.png'
import logo from './logo.png'
import mergerequestsEmpty from './mergerequests-empty.png'
import repositoriesEmpty from './repositories-empty.png'
import tagsEmpty from './tags-empty.png'

/** 空态插图：键名与空态组件 `imageName` prop 对应。 */
export const emptyImages = {
  'repositories-empty.png': repositoriesEmpty,
  'branches-empty.png': branchesEmpty,
  'tags-empty.png': tagsEmpty,
  'commits-empty.png': commitsEmpty,
  'mergerequests-empty.png': mergerequestsEmpty,
} as const

export type EmptyImageName = keyof typeof emptyImages

export {
  branchesEmpty,
  commitsEmpty,
  defaultAvatar,
  headerLogo,
  logo,
  mergerequestsEmpty,
  repositoriesEmpty,
  tagsEmpty,
}
