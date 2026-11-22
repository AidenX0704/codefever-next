import {
  adminIcon,
  groupIcon,
  membersIcon,
  mergeRequestIcon,
  repositoryIcon,
} from '@/config/icons'
import type { IconComponent } from '@/config/icons'

/**
 * 平台级侧栏导航（v5 DESIGN.md §4.2）。
 *
 * v5 口径：侧栏只表达"平台级"结构（代码 / 协作 / 管理），
 * 仓库内部与设置内的二级导航不进这里（仍是各布局的页内导航）。
 * 「管理」分组由调用方按 `user.admin` 门控（服务端为准的前端投影）；
 * 原型的审计日志 / 集成与 Webhook / 系统状态路由后端尚未提供，不渲染空入口。
 *
 * 2026-11-22 减负：「工作台」分组（/home 展示首页）与全局搜索入口下线，
 * 管理分组的公告 / 系统设置入口随 admin 对应页一并下线。
 */
export interface PlatformNavEntry {
  to: string
  labelKey: string
  icon: IconComponent
  active: (pathname: string) => boolean
}

export interface PlatformNavGroup {
  titleKey: string
  entries: PlatformNavEntry[]
}

const under = (base: string) => (pathname: string) =>
  pathname === base || pathname.startsWith(`${base}/`)

/** 仓库详情页是一级路径下的 `/:group/:repo/*`，排除其余平台前缀后整段归「仓库」。 */
const isRepoPage = (pathname: string) =>
  /^\/(?!groups\b|mergerequests\b|settings\b|admin\b|user\b|doc\b|repositories\b)[^/]+\/[^/]+/.test(
    pathname,
  )

export function platformNavGroups(): PlatformNavGroup[] {
  return [
    {
      titleKey: 'nav.code',
      entries: [
        {
          to: '/repositories',
          labelKey: 'label.repository',
          icon: repositoryIcon,
          active: (pathname) => under('/repositories')(pathname) || isRepoPage(pathname),
        },
        { to: '/groups', labelKey: 'label.group', icon: groupIcon, active: under('/groups') },
      ],
    },
    {
      titleKey: 'nav.collab',
      entries: [
        {
          to: '/mergerequests',
          labelKey: 'label.mergeRequest',
          icon: mergeRequestIcon,
          active: under('/mergerequests'),
        },
      ],
    },
    {
      titleKey: 'nav.admin',
      entries: [
        { to: '/admin/dashboard', labelKey: 'nav.adminConsole', icon: adminIcon, active: under('/admin') },
        { to: '/admin/users', labelKey: 'menu.user_pl', icon: membersIcon, active: under('/admin/users') },
        { to: '/admin/repositories', labelKey: 'label.repository_pl', icon: repositoryIcon, active: under('/admin/repositories') },
        { to: '/admin/groups', labelKey: 'label.group_pl', icon: groupIcon, active: under('/admin/groups') },
      ],
    },
  ]
}
