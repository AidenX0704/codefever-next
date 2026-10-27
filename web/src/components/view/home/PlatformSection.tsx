import { useNavigate } from 'react-router'

import { Icon } from '@/components/unit/Icon'
import { SectionCard } from '@/components/unit/SectionCard'
import { Button } from '@/components/unit/Button'
import { HOME_QUICK_LINKS, PLATFORM_CAPABILITIES, type HomeLink } from '@/config/homeLinks'
import { HOME_STEPS, TEAM_RULES } from '@/config/homeOnboarding'
import { docUrl } from '@/helpers/docUrl'
import { useI18n, useT } from '@/i18n'


export interface PlatformSectionProps {
  /** 平台概览数字（全站口径，来自 `home/overview.platform`）；取不到时传 `undefined`。 */
  platform: {
    repositories: number | undefined
    groups: number | undefined
    members: number | undefined
    openMergeRequests: number | undefined
  }
}

/**
 * 门户区（"了解平台"那一层，全员一致）。
 *
 * 定位来自调研结论（docs/HOME_PAGE_PLAN_2026-10-27.md §2.2）：小米这类"平台是整合层"
 * 的定位要求首页不能只做仓库列表，必须有一层"这个平台能给我什么"的说明。
 *
 * 卡片**只在**这一层出现；工作台区（待办 / 我的仓库 / 动态）用无外框的分隔线区块。
 * 卡内仍然是列表/表格，不做插画墙。
 */
export function PlatformSection({ platform }: PlatformSectionProps) {
  const t = useT()
  const { lang } = useI18n()
  const navigate = useNavigate()

  /** 把配置里的链接落点翻译成一次点击（内部路由 / 文档中心 / 站外）。 */
  const go = (link: HomeLink) => {
    if (link.kind === 'internal') {
      navigate(link.path)
      return
    }
    if (link.kind === 'doc') {
      navigate(docUrl(lang, link.path))
      return
    }
    window.open(link.url, '_blank', 'noopener')
  }

  const overviewRows: ReadonlyArray<{ key: string; label: string; value: number | undefined }> = [
    { key: 'repositories', label: t('label.repository'), value: platform.repositories },
    { key: 'groups', label: t('label.group'), value: platform.groups },
    { key: 'members', label: t('home.overview.members'), value: platform.members },
    { key: 'openMergeRequests', label: t('home.overview.openMergeRequest'), value: platform.openMergeRequests },
  ]

  return (
    <div className="home-portal">
      <div className="home-portal__grid">
        <SectionCard variant="card" title={t('home.section.overview')}>
          <dl className="home-overview">
            {overviewRows.map((row) => (
              <div key={row.key} className="home-overview__row">
                <dt>{row.label}</dt>
                <dd>{row.value ?? '—'}</dd>
              </div>
            ))}
          </dl>
        </SectionCard>

        <SectionCard variant="card" title={t('home.section.onboarding')}>
          <ol className="home-steps">
            {HOME_STEPS.map((step, index) => (
              <li key={step.key} className="home-steps__item">
                <span className="home-steps__index">{index + 1}</span>
                <span className="home-steps__text">
                  <span className="home-steps__title">{t(step.titleKey)}</span>
                  <span className="home-steps__desc">{t(step.descKey)}</span>
                </span>
                <Button variant="text" size="sm" onClick={() => go(step.link)}>
                  {t(step.actionKey)}
                </Button>
              </li>
            ))}
          </ol>
        </SectionCard>

        <SectionCard variant="card" title={t('home.section.rules')}>
          <ul className="home-rules">
            {TEAM_RULES.map((rule) => (
              <li key={rule.key} className="home-rules__item">
                <span className="home-rules__title">{t(rule.titleKey)}</span>
                <span className="home-rules__desc">{t(rule.descKey)}</span>
                <Button variant="text" size="sm" onClick={() => go(rule.link)}>
                  {t('home.step.go')}
                </Button>
              </li>
            ))}
          </ul>
        </SectionCard>
      </div>

      <SectionCard variant="card" title={t('home.section.capabilities')}>
        <ul className="home-caps">
          {PLATFORM_CAPABILITIES.map((capability) => (
            <li key={capability.key}>
              <button type="button" className="home-caps__item" onClick={() => go(capability.link)}>
                <span className="home-caps__icon">
                  <Icon icon={capability.icon} size={18} />
                </span>
                <span className="home-caps__text">
                  <span className="home-caps__title">{t(capability.labelKey)}</span>
                  <span className="home-caps__desc">{t(capability.descKey)}</span>
                </span>
              </button>
            </li>
          ))}
        </ul>

        {/* 快捷入口：一行常驻链接，不单独占卡（本质是 6 个链接，为它开一张卡是用装饰掩盖信息量不足）。 */}
        <div className="home-quick">
          <span className="home-quick__label">{t('home.section.quickLinks')}</span>
          {HOME_QUICK_LINKS.map((link) => (
            <button
              key={link.key}
              type="button"
              className="home-quick__link"
              disabled={!link.ready}
              title={link.ready ? undefined : t('home.quick.notReady')}
              onClick={() => go(link.link)}
            >
              {t(link.labelKey)}
            </button>
          ))}
        </div>
      </SectionCard>
    </div>
  )
}
