import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'

import type { HomeOverview } from '@/api/types'
import { fetchHomeOverview } from '@/api/home'
import { AnnouncementCard } from '@/components/view/home/AnnouncementCard'
import { HomeHero } from '@/components/view/home/HomeHero'
import { MergeRequestPanel } from '@/components/view/home/MergeRequestPanel'
import { MyRepositoriesTable } from '@/components/view/home/MyRepositoriesTable'
import { PlatformSection } from '@/components/view/home/PlatformSection'
import { RecentRepositories } from '@/components/view/home/RecentRepositories'
import { StatStrip } from '@/components/view/home/StatStrip'
import { TeamActivity } from '@/components/view/home/TeamActivity'
import { canReadRepository } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/**
 * 员工首页（`/home`）。
 *
 * 定位与取舍见 `docs/HOME_PAGE_PLAN_2026-10-27.md`：
 *   - 上半屏是**个人工作台**（我该做什么 / 我最近在做什么），下半屏是**平台门户**
 *     （这个平台能给我什么、怎么上手、规范在哪）；
 *   - 形态**列表优先**：KPI 是一条无外框的数字行，仓库是表格，卡片只出现在门户区；
 *   - 首屏请求共 4 个：`user/info`、`repository/list`、`group/list`（都由
 *     `useAppData()` 拉，页头本来就要）＋ 本页的 `home/overview` 聚合请求。
 *
 * 与 `/repositories` 的分工（有意重叠、必须守住）：首页只给"最近 8 个"与入口，
 * 全量列表、筛选、排序在 `/repositories`。不要把完整列表搬回首页。
 */
export default function Home() {
  const t = useT()
  const navigate = useNavigate()
  const { user, repositories, groups, error } = useAppData()

  const [overview, setOverview] = useState<HomeOverview | undefined>(undefined)
  const [overviewFailed, setOverviewFailed] = useState(false)

  useEffect(() => {
    let cancelled = false
    fetchHomeOverview()
      .then((data) => {
        if (!cancelled) {
          setOverview(data)
        }
      })
      .catch(() => {
        // 聚合端点失败不能让整页空白：KPI 与两个待办面板显示"—"/空态，
        // 我的仓库与门户区照常可用（它们的数据来自 AppData）。
        if (!cancelled) {
          setOverviewFailed(true)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  const readable = repositories.filter((repository) => canReadRepository(repository.role))
  const byUpdatedDesc = [...readable].sort((a, b) => b.updated - a.updated)
  const primaryRepository = byUpdatedDesc[0] ?? null

  return (
    <div className="page home-page">
      {error ? <div className="auth-message">{error}</div> : null}

      <HomeHero user={user} repository={primaryRepository} />

      <StatStrip
        items={[
          {
            key: 'repositories',
            label: t('home.kpi.repositories'),
            value: readable.length,
            onClick: () => navigate('/repositories'),
          },
          {
            key: 'groups',
            label: t('home.kpi.groups'),
            value: groups.length,
            onClick: () => navigate('/groups'),
          },
          {
            key: 'reviewRequests',
            label: t('home.kpi.reviewRequests'),
            value: overviewFailed ? undefined : overview?.counts.reviewRequests,
            onClick: () => navigate('/mergerequests'),
          },
          {
            key: 'myOpenMergeRequests',
            label: t('home.kpi.myOpenMergeRequests'),
            value: overviewFailed ? undefined : overview?.counts.myOpenMergeRequests,
            onClick: () => navigate('/mergerequests'),
          },
        ]}
      />

      <div className="home-work">
        <div className="home-work__main">
          <MergeRequestPanel
            title={t('home.section.reviewRequests')}
            items={overview?.reviewRequests}
            emptyNotice={t('home.empty.reviewRequests')}
            onViewAll={() => navigate('/mergerequests')}
          />
          <MergeRequestPanel
            title={t('home.section.myMergeRequests')}
            items={overview?.myMergeRequests}
            emptyNotice={t('home.empty.myMergeRequests')}
            onViewAll={() => navigate('/mergerequests')}
            showAge
          />
          <MyRepositoriesTable onViewAll={() => navigate('/repositories')} />
        </div>

        <div className="home-work__side">
          <RecentRepositories onBrowse={() => navigate('/repositories')} />
          <AnnouncementCard />
        </div>
      </div>

      <TeamActivity items={overview?.recentActivities ?? []} />

      <PlatformSection
        platform={{
          repositories: overview?.platform.repositories,
          groups: overview?.platform.groups,
          members: overview?.platform.members,
          openMergeRequests: overview?.platform.openMergeRequests,
        }}
      />
    </div>
  )
}
