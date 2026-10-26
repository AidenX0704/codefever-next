/**
 * 复用的静态素材（逐张来自旧 `www/static/images`，文件名保持不变，便于与设计稿对照）。
 *
 * 约定：业务代码只从这里取图，不写 `/static/images/xxx.png` 这种 URL ——
 * 旧版正是靠 URL 字符串拼图，路径一改就静默 404（见 AGENTS §9 图标坑的同类问题）。
 */
import activityHero from './activity_pgyer_codefever_.png'
import activityHeroWebp from './activity_pgyer_codefever_.webp'
import branchRulesEmpty from './branch-rules-empty.png'
import branchesEmpty from './branches-empty.png'
import commitsEmpty from './commits-empty.png'
import defaultAvatar from './default_avatar.png'
import headerLogo from './header-codefever-logo.png'
import langEn from './language_english.png'
import langZh from './language_chinese.png'
import logo from './logo.png'
import logoCommunity from './logo-community.png'
import logoCommunityDoc from './logo-community-doc.png'
import logoIco from './logo-ico.png'
import mergerequestsEmpty from './mergerequests-empty.png'
import repositoriesEmpty from './repositories-empty.png'
import tagsEmpty from './tags-empty.png'

/** 空态插图：键名与旧版 `EmptyListNotice` 的 `imageName` prop 完全一致。 */
export const emptyImages = {
  'repositories-empty.png': repositoriesEmpty,
  'branches-empty.png': branchesEmpty,
  'tags-empty.png': tagsEmpty,
  'commits-empty.png': commitsEmpty,
  'mergerequests-empty.png': mergerequestsEmpty,
  'branch-rules-empty.png': branchRulesEmpty,
} as const

export type EmptyImageName = keyof typeof emptyImages

export {
  activityHero,
  activityHeroWebp,
  branchRulesEmpty,
  branchesEmpty,
  commitsEmpty,
  defaultAvatar,
  headerLogo,
  langEn,
  langZh,
  logo,
  logoCommunity,
  logoCommunityDoc,
  logoIco,
  mergerequestsEmpty,
  repositoriesEmpty,
  tagsEmpty,
}
