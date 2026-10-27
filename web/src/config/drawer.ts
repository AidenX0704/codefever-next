import {
  psBell,
  psBranch,
  psCommit,
  psEmail,
  psFile,
  psKey,
  psLog,
  psMeter,
  psMemberAlt,
  psMembers,
  psMerge,
  psSetting,
  psRepository,
  psRepositoryGroup,
  psTag,
} from '@/config/icons'
import type { IconData } from '@/components/unit/Icon'
import type { RepositoryConfig } from '@/api/types'

/**
 * 左侧导航（Drawer）配置。
 *
 * 逐字移植旧版 `www/view/src/config/DrawerConfig.js` 的 `makeRepositoryDrawerConfig`：
 * 8 个一级项 + 设置下的 4 个子项，`count` 显示在右侧（提交/分支/标签/成员/未合并 MR）。
 * `active` 判定沿用旧版的**正则语义**（`files` 也命中 `blame`，设置页命中根路径等），
 * 这样 URL 行为与旧版一致。
 */
export interface DrawerEntry {
  path: string
  labelKey: string
  icon: IconData
  count?: number
  children?: DrawerEntry[]
  active: (pathname: string) => boolean
}

export function repositoryDrawerEntries(config: RepositoryConfig | null): DrawerEntry[] {
  if (!config?.repository) {
    return []
  }
  const { group, repository, count, members } = config
  const base = `/${group.name}/${repository.name}`

  const matchGroupRepo = (suffix: RegExp) => (pathname: string) => suffix.test(pathname)

  return [
    {
      path: `${base}/files`,
      labelKey: 'menu.file_pl',
      icon: psFile,
      // 对齐旧版 DrawerConfig 的四条 files 模式：仓库根路径、files、blame。
      // 注意不能写成 `/?(files|blame)?(/.*)?$` —— 那会让"文件"在 commits/branches
      // 等所有仓库子页都命中（`(/.*)?` 吃掉任意后缀），激活态永远不切换。
      active: (pathname) =>
        new RegExp(`^/${group.name}/${repository.name}/?$`, 'i').test(pathname) ||
        new RegExp(`^/${group.name}/${repository.name}/(files|blame)(/.*)?$`, 'i').test(pathname),
    },
    {
      path: `${base}/commits`,
      labelKey: 'menu.commit_pl',
      icon: psCommit,
      count: count?.commit,
      active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/(commits|commit)(/.*)?$`)),
    },
    {
      path: `${base}/mergerequests`,
      labelKey: 'menu.mergeRequest_pl',
      icon: psMerge,
      active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/mergerequests(/.*)?$`)),
    },
    {
      path: `${base}/branches`,
      labelKey: 'menu.branch_pl',
      icon: psBranch,
      count: count?.branch,
      active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/branches(/.*)?$`)),
    },
    {
      path: `${base}/tags`,
      labelKey: 'menu.tag_pl',
      icon: psTag,
      count: count?.tag,
      active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/tags(/.*)?$`)),
    },
    {
      path: `${base}/members`,
      labelKey: 'menu.member_pl',
      icon: psMembers,
      count: members?.length ?? 0,
      active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/members(/.*)?$`)),
    },
    {
      path: `${base}/activities`,
      labelKey: 'menu.activity_pl',
      icon: psLog,
      active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/activities(/.*)?$`)),
    },
    {
      path: `${base}/settings`,
      labelKey: 'menu.setting_pl',
      icon: psSetting,
      active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/settings(/.*)?$`)),
      children: [
        {
          path: `${base}/settings/general`,
          labelKey: 'menu.general',
          icon: psSetting,
          active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/settings(/general)?/?$`)),
        },
        {
          path: `${base}/settings/branch`,
          labelKey: 'menu.branch_pl',
          icon: psSetting,
          active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/settings/branch(/.*)?$`)),
        },
        {
          path: `${base}/settings/webhook`,
          labelKey: 'menu.webhook_pl',
          icon: psSetting,
          active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/settings/webhook(/.*)?$`)),
        },
        {
          path: `${base}/settings/advanced`,
          labelKey: 'menu.advanced',
          icon: psSetting,
          active: matchGroupRepo(new RegExp(`/${group.name}/${repository.name}/settings/advanced(/.*)?$`)),
        },
      ],
    },
  ]
}

/**
 * 个人设置导航（`/settings/*`）。
 *
 * 逐字移植旧 `DrawerConfig.makeDrawerConfig()` 的 4 项与 active 正则：
 * 个人资料 / 邮箱（含提交邮箱）/ SSH Key / 通知。
 */
export function settingsDrawerEntries(): DrawerEntry[] {
  return [
    {
      path: '/settings/profile',
      labelKey: 'menu.profile',
      icon: psMemberAlt,
      active: (pathname) => /^\/settings$/i.test(pathname) || /^\/settings\/profile/i.test(pathname),
    },
    {
      path: '/settings/email',
      labelKey: 'menu.mail',
      icon: psEmail,
      active: (pathname) => /^\/settings\/email/i.test(pathname),
    },
    {
      path: '/settings/sshkey',
      labelKey: 'menu.SSHKey_pl',
      icon: psKey,
      active: (pathname) => /^\/settings\/sshkey/i.test(pathname),
    },
    {
      path: '/settings/notification',
      labelKey: 'menu.notification',
      icon: psBell,
      active: (pathname) => /^\/settings\/notification/i.test(pathname),
    },
    // 访问令牌（公开 API /api/v1 的凭据）。2026-10-27 新增 —— 后端能力早已存在，
    // 这一项是它第一次有入口。
    {
      path: '/settings/tokens',
      labelKey: 'tokens.title',
      icon: psMeter,
      active: (pathname) => /^\/settings\/tokens/i.test(pathname),
    },
  ]
}

/**
 * 仓库组上下文导航（`/groups/{name}/*`）。
 *
 * 逐字移植旧 `DrawerConfig.makeGroupDrawerConfig()`：仓库 / 合并请求 / 成员 / 动态 / 设置
 * （设置下含 常规 与 高级 两个子项）。
 */
export function groupDrawerEntries(group: { name: string } | null): DrawerEntry[] {
  if (!group) {
    return []
  }
  const base = `/groups/${group.name}`
  return [
    {
      path: `${base}/repositories`,
      labelKey: 'menu.repository_pl',
      icon: psRepository,
      // 旧版第二条模式不带结尾锚，`/groups/{组}/repositories/new` 也命中。
      active: (pathname) =>
        new RegExp(`^/groups/${group.name}/?$`, 'i').test(pathname) ||
        new RegExp(`^/groups/${group.name}/repositories(/.*)?$`, 'i').test(pathname),
    },
    {
      path: `${base}/mergerequests`,
      labelKey: 'menu.mergeRequest_pl',
      icon: psMerge,
      active: (pathname) => new RegExp(`/groups/${group.name}/mergerequests`).test(pathname),
    },
    {
      path: `${base}/members`,
      labelKey: 'menu.member_pl',
      icon: psMembers,
      active: (pathname) => new RegExp(`/groups/${group.name}/members/?$`).test(pathname),
    },
    {
      path: `${base}/activities`,
      labelKey: 'menu.activity_pl',
      icon: psLog,
      active: (pathname) => new RegExp(`/groups/${group.name}/activities`).test(pathname),
    },
    {
      path: `${base}/settings`,
      labelKey: 'menu.setting_pl',
      icon: psSetting,
      active: (pathname) => new RegExp(`/groups/${group.name}/settings`).test(pathname),
      children: [
        {
          path: `${base}/settings/general`,
          labelKey: 'menu.general',
          icon: psSetting,
          active: (pathname) => new RegExp(`/groups/${group.name}/settings(/general)?/?$`).test(pathname),
        },
        {
          path: `${base}/settings/advanced`,
          labelKey: 'menu.advanced',
          icon: psSetting,
          active: (pathname) => new RegExp(`/groups/${group.name}/settings/advanced`).test(pathname),
        },
      ],
    },
  ]
}


/**
 * 管理后台导航（`/admin/*`）。
 *
 * 逐字移植旧 `DrawerConfig.makeAdminDrawerConfig()`：仪表盘 / 用户 / 仓库组 / 仓库 / 系统设置。
 */
export function adminDrawerEntries(): DrawerEntry[] {
  return [
    {
      path: '/admin/dashboard',
      labelKey: 'menu.dashboard',
      icon: psLog,
      active: (pathname) => /^\/admin(\/)?$/i.test(pathname) || /^\/admin\/dashboard/i.test(pathname),
    },
    { path: '/admin/users', labelKey: 'menu.user_pl', icon: psMembers, active: (pathname) => /^\/admin\/users/i.test(pathname) },
    {
      path: '/admin/groups',
      labelKey: 'menu.group_pl',
      icon: psRepositoryGroup,
      active: (pathname) => /^\/admin\/groups/i.test(pathname),
    },
    {
      path: '/admin/repositories',
      labelKey: 'menu.repository_pl',
      icon: psRepository,
      active: (pathname) => /^\/admin\/repositories/i.test(pathname),
    },
    {
      path: '/admin/settings',
      labelKey: 'menu.setting_pl',
      icon: psSetting,
      active: (pathname) => /^\/admin\/settings/i.test(pathname),
    },
  ]
}
