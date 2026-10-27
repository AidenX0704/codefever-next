import { psKey, psBell, psFile, psMeter, psNewFeature, psRepository } from '@/config/icons'

import type { IconData } from '@/components/unit/Icon'

/**
 * 首页「快捷入口」与「平台能力」的配置（唯一数据源）。
 *
 * 为什么放 `config/`（AGENTS §4 常量落位）：这两块是**纯静态清单**，
 * 既是首页的渲染数据，也是"平台有哪些能力"这一产品事实的说明文档。
 * 写进页面组件会导致改一处漏一处，也无法被复用（例如将来的帮助中心/新手指引）。
 */

/** 链接落点：内部路由 / 文档中心 / 站外。 */
export type HomeLink =
  | { kind: 'internal'; path: string }
  /** 文档中心相对路径（相对 `doc/{zh-cn,en-us}/`），渲染时按当前语言补前缀。 */
  | { kind: 'doc'; path: string }
  | { kind: 'external'; url: string }

export interface HomeQuickLink {
  key: string
  /** 文案的 i18n 键（词表见 `i18n/supplement.ts`）。 */
  labelKey: string
  icon: IconData
  link: HomeLink
  /**
   * 该入口是否已经可用。
   *
   * 保留这个字段是因为"配置项先于页面"的情况还会再出现（例如公告后台、代码搜索）：
   * 先标 `false` 灰显并给出说明，**不做一个点了就 404 的入口**。
   * 令牌管理页已于 2026-10-27 落地，因此该项现在为 `true`。
   */
  ready: boolean
}

/** 「快捷入口」常驻链接行（渲染在门户区「平台能力」卡片底部）。 */
export const HOME_QUICK_LINKS: ReadonlyArray<HomeQuickLink> = [
  {
    key: 'sshkey',
    labelKey: 'home.quick.sshKey',
    icon: psKey,
    link: { kind: 'internal', path: '/settings/sshkey' },
    ready: true,
  },
  {
    key: 'tokens',
    labelKey: 'home.quick.tokens',
    icon: psMeter,
    link: { kind: 'internal', path: '/settings/tokens' },
    ready: true,
  },
  {
    key: 'docs',
    labelKey: 'home.quick.docs',
    icon: psFile,
    link: { kind: 'doc', path: 'start/welcome.md' },
    ready: true,
  },
  {
    key: 'publicApi',
    labelKey: 'home.quick.publicApi',
    icon: psRepository,
    link: { kind: 'doc', path: 'api/overview.md' },
    ready: true,
  },
  {
    key: 'webhook',
    labelKey: 'home.quick.webhook',
    icon: psNewFeature,
    link: { kind: 'doc', path: 'repo/webhooks.md' },
    ready: true,
  },
  {
    key: 'help',
    labelKey: 'home.quick.help',
    icon: psBell,
    link: { kind: 'internal', path: '/help' },
    ready: true,
  },
]

export interface PlatformCapability {
  key: string
  labelKey: string
  descKey: string
  icon: IconData
  link: HomeLink
}

/**
 * 「平台能力」清单（8 项，两列 × 4 行）。
 *
 * 只列**确实已经实现**的能力，不做"即将支持"的占位 —— 首页是给员工看门面的，
 * 写了没做的能力比不写更伤信任。
 */
export const PLATFORM_CAPABILITIES: ReadonlyArray<PlatformCapability> = [
  {
    key: 'hosting',
    labelKey: 'home.cap.hosting',
    descKey: 'home.cap.hostingDesc',
    icon: psRepository,
    link: { kind: 'internal', path: '/repositories' },
  },
  {
    key: 'refs',
    labelKey: 'home.cap.refs',
    descKey: 'home.cap.refsDesc',
    icon: psFile,
    link: { kind: 'doc', path: 'repo/create_branch.md' },
  },
  {
    key: 'review',
    labelKey: 'home.cap.review',
    descKey: 'home.cap.reviewDesc',
    icon: psNewFeature,
    link: { kind: 'internal', path: '/mergerequests' },
  },
  {
    key: 'protect',
    labelKey: 'home.cap.protect',
    descKey: 'home.cap.protectDesc',
    icon: psKey,
    link: { kind: 'doc', path: 'repo/protected_branch.md' },
  },
  {
    key: 'members',
    labelKey: 'home.cap.members',
    descKey: 'home.cap.membersDesc',
    icon: psBell,
    link: { kind: 'doc', path: 'repo_group/members.md' },
  },
  {
    key: 'webhook',
    labelKey: 'home.cap.webhook',
    descKey: 'home.cap.webhookDesc',
    icon: psMeter,
    link: { kind: 'doc', path: 'repo/webhooks.md' },
  },
  {
    key: 'api',
    labelKey: 'home.cap.api',
    descKey: 'home.cap.apiDesc',
    icon: psRepository,
    link: { kind: 'doc', path: 'api/overview.md' },
  },
  {
    key: 'notify',
    labelKey: 'home.cap.notify',
    descKey: 'home.cap.notifyDesc',
    icon: psBell,
    link: { kind: 'internal', path: '/settings/notification' },
  },
]
