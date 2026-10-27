import type { HomeLink } from './homeLinks'

/**
 * 首页门户区「四步入門」与「团队规范」的静态配置（唯一数据源）。
 *
 * 口径：
 *   - 「四步入門」只写**新人第一次用这个平台必须做的事**，每步一个"去做"链接；
 *   - 「团队规范」指向文档中心的既有章节，**不复制正文**（避免两处维护同一段话）。
 *
 * 所有 doc 路径都必须是 `doc/{zh-cn,en-us}/` 下真实存在的文件 ——
 * 死链在这个项目里踩过（PROJECT_MAP 2026-10-26 条目），改这里请顺手核对两侧目录。
 */

export interface HomeStep {
  key: string
  /** 步骤标题的 i18n 键。 */
  titleKey: string
  /** 步骤说明的 i18n 键。 */
  descKey: string
  link: HomeLink
  /** 链接按钮的 i18n 键。 */
  actionKey: string
}

export const HOME_STEPS: ReadonlyArray<HomeStep> = [
  {
    key: 'ssh',
    titleKey: 'home.step.ssh',
    descKey: 'home.step.sshDesc',
    link: { kind: 'internal', path: '/settings/sshkey' },
    actionKey: 'home.step.sshAction',
  },
  {
    key: 'clone',
    titleKey: 'home.step.clone',
    descKey: 'home.step.cloneDesc',
    link: { kind: 'doc', path: 'git/clone.md' },
    actionKey: 'home.step.go',
  },
  {
    key: 'mr',
    titleKey: 'home.step.mr',
    descKey: 'home.step.mrDesc',
    link: { kind: 'internal', path: '/mergerequests/new' },
    actionKey: 'home.step.go',
  },
  {
    key: 'hook',
    titleKey: 'home.step.hook',
    descKey: 'home.step.hookDesc',
    link: { kind: 'doc', path: 'repo/webhooks.md' },
    actionKey: 'home.step.go',
  },
]

export interface TeamRule {
  key: string
  titleKey: string
  descKey: string
  link: HomeLink
}

export const TEAM_RULES: ReadonlyArray<TeamRule> = [
  {
    key: 'clone',
    titleKey: 'home.rule.clone',
    descKey: 'home.rule.cloneDesc',
    link: { kind: 'doc', path: 'common/clone_method.md' },
  },
  {
    key: 'workflow',
    titleKey: 'home.rule.workflow',
    descKey: 'home.rule.workflowDesc',
    link: { kind: 'doc', path: 'common/workflow.md' },
  },
  {
    key: 'role',
    titleKey: 'home.rule.role',
    descKey: 'home.rule.roleDesc',
    link: { kind: 'doc', path: 'common/role.md' },
  },
]
