import { useNavigate } from 'react-router'

import type { SiteStats } from '@/api/site'
import { Button } from '@/components/unit/Button'
import { StatStrip } from '@/components/view/home/StatStrip'
import { useT } from '@/i18n'


export interface PlatformHeroProps {
  /** 平台公开统计（`/api/site/stats`，匿名可访问）；取不到时传 `undefined`。 */
  stats: SiteStats | undefined
}

/**
 * 展示首页（/home）顶部（gitee.com 口径的平台变体）。
 *
 * 2026-11-16 二次定版：/home 定位是**跟 gitee.com 一样做展示**的首页 ——
 * 第一屏只有平台叙事（平台名 + 定位 + 平台规模 + 新手动作），个人工作台
 * （问候 / 待办 / 我的仓库）全部在 `/:username`。
 *
 * 数据用匿名的 `site/stats` 而不是登录态的 `home/overview`：展示页的数字
 * 是平台成果（已合并 MR 数），不是"我的待办"。
 */
export function PlatformHero({ stats }: PlatformHeroProps) {
  const t = useT()
  const navigate = useNavigate()

  return (
    <section className="home-hero">
      <div className="home-hero__main">
        <h1 className="home-hero__greeting">{t('home.platform.title')}</h1>
        <p className="home-hero__tagline">{t('home.platform.tagline')}</p>
      </div>

      <div className="home-hero__actions">
        <Button variant="contained" onClick={() => navigate('/repositories/new')}>
          {t('label.newRepository')}
        </Button>
        <Button variant="outlined" onClick={() => navigate('/repositories')}>
          {t('home.action.browse')}
        </Button>
      </div>

      <StatStrip
        items={[
          { key: 'repositories', label: t('label.repository'), value: stats?.repositoryCount, onClick: () => navigate('/repositories') },
          { key: 'groups', label: t('label.group'), value: stats?.groupCount, onClick: () => navigate('/groups') },
          { key: 'members', label: t('home.overview.members'), value: stats?.memberCount, onClick: undefined },
          { key: 'mergedMergeRequests', label: t('home.overview.mergedMergeRequest'), value: stats?.mergedMergeRequestCount, onClick: () => navigate('/mergerequests') },
        ]}
      />
    </section>
  )
}
