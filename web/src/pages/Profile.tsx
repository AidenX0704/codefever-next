import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import type { HomeOverview, UserProfile } from '@/api/types'
import { fetchHomeOverview } from '@/api/home'
import { fetchUserProfile } from '@/api/user'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { Icon } from '@/components/unit/Icon'
import { PageLoading } from '@/components/unit/Loading'
import { SectionCard } from '@/components/unit/SectionCard'
import { Tag } from '@/components/unit/Tag'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { HomeHero } from '@/components/view/home/HomeHero'
import { MergeRequestPanel } from '@/components/view/home/MergeRequestPanel'
import { MyRepositoriesTable } from '@/components/view/home/MyRepositoriesTable'
import { RecentRepositories } from '@/components/view/home/RecentRepositories'
import { StatStrip } from '@/components/view/home/StatStrip'
import { TeamActivity } from '@/components/view/home/TeamActivity'
import { RepositoryRow } from '@/components/view/unit/RepositoryRow'
import { briefcaseIcon, buildingIcon, clockIcon, locationIcon, repositoryIcon, sparkleIcon } from '@/config/icons'
import { canReadRepository } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'
import NotFound from '@/pages/NotFound'


/**
 * 个人主页（`/:username`）。
 *
 * 2026-11-16 二次定版：/home 改为 gitee.com 式展示首页后，**个人工作台的全部
 * 内容落在这里** ——
 *   - **本人**：问候 Hero（含主推仓库 clone 命令）+ KPI 条 + 待我评审 / 我发起的
 *     + 我的仓库表格 + 最近访问 + 团队动态 —— 与原 /home 个人工作台一致；
 *   - **他人**：公开资料 + 创建的仓库 + TA 的动态（GitHub profile 口径）。
 *
 * 左栏身份卡对两种视图都成立（GitHub 同款布局）。
 *
 * 数据来源（本人 2 个请求，他人 1 个）：
 *   - `user/profile`：公开资料 + 创建的仓库 + 动态（都按请求者可见性过滤）；
 *   - `home/overview`：仅本人 —— 待办面板、KPI 待办数、团队动态。
 */
export default function Profile() {
  const t = useT()
  const navigate = useNavigate()
  const { username = '' } = useParams()
  const { user, groups, repositories, error } = useAppData()

  const isSelf = Boolean(user && user.name === username)

  const [profile, setProfile] = useState<UserProfile | undefined>(undefined)
  const [failed, setFailed] = useState(false)
  const [overview, setOverview] = useState<HomeOverview | undefined>(undefined)

  useEffect(() => {
    let cancelled = false
    setProfile(undefined)
    setFailed(false)
    setOverview(undefined)
    fetchUserProfile(username)
      .then((data) => {
        if (!cancelled) {
          setProfile(data)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true)
        }
      })
    if (isSelf) {
      // 待办面板失败不拖垮整页：面板显示空态，资料 / 仓库 / 动态照常。
      fetchHomeOverview()
        .then((data) => {
          if (!cancelled) {
            setOverview(data)
          }
        })
        .catch(() => {})
    }
    return () => {
      cancelled = true
    }
  }, [username, isSelf])

  // KPI 条（仅本人）：沿用原 /home 的口径 —— 参与的仓库 / 仓库组来自 AppData。
  const readable = useMemo(
    () => repositories.filter((repository) => canReadRepository(repository.role)),
    [repositories],
  )

  // 主推仓库（"我的仓库"里 updated 最新的那个），给问候 Hero 出 clone 命令。
  const primaryRepository = useMemo(
    () => [...readable].sort((a, b) => b.updated - a.updated)[0] ?? null,
    [readable],
  )

  // GitHub 口径：不存在的用户名直接给 404 页（不泄露"这个用户名被占用过"）。
  if (failed) {
    return <NotFound />
  }

  if (!profile) {
    return <PageLoading />
  }

  const { profile: info, repositories: owned, activities } = profile

  return (
    <div className="page profile-page">
      {error ? <div className="auth-message">{error}</div> : null}

      <div className="profile-layout">
        <aside className="profile-side">
          <Avatar size="lg" src={info.icon ? `/file/fetch/avatar/${info.icon}` : undefined} name={info.name} />
          <h1 className="profile-side__name">{info.name}</h1>
          {info.bio ? <p className="profile-side__bio">{info.bio}</p> : null}
          <div className="profile-side__meta">
            {info.team ? (
              <span className="profile-side__meta-row">
                <Icon icon={buildingIcon} size={14} />
                {info.team}
              </span>
            ) : null}
            {info.role ? (
              <span className="profile-side__meta-row">
                <Icon icon={briefcaseIcon} size={14} />
                {info.role}
              </span>
            ) : null}
            {info.location ? (
              <span className="profile-side__meta-row">
                <Icon icon={locationIcon} size={14} />
                {info.location}
              </span>
            ) : null}
            {info.admin ? <Tag tone="primary">{t('label.adminArea')}</Tag> : null}
            {info.created > 0 ? (
              <span className="profile-side__meta-row">
                <Icon icon={clockIcon} size={14} />
                <span>
                  {t('label.joinedAt')} <FormattedTime timestamp={info.created} />
                </span>
              </span>
            ) : null}
          </div>
          {isSelf ? (
            <Button variant="outlined" onClick={() => navigate('/settings/profile')}>
              {t('profile.edit')}
            </Button>
          ) : null}
        </aside>

        <div className="profile-main">
          {isSelf ? (
            <>
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
                    key: 'createdRepositories',
                    label: t('profile.kpi.created'),
                    value: owned.length,
                    onClick: () => navigate('/repositories'),
                  },
                  {
                    key: 'reviewRequests',
                    label: t('home.kpi.reviewRequests'),
                    value: overview?.counts.reviewRequests,
                    onClick: () => navigate('/mergerequests'),
                  },
                ]}
              />
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
              <RecentRepositories onBrowse={() => navigate('/repositories')} />
              <TeamActivity items={overview?.recentActivities ?? []} />
            </>
          ) : (
            <>
              <SectionCard
                variant="plain"
                title={
                  <span className="profile-section__title">
                    <Icon icon={repositoryIcon} size={16} />
                    {t('profile.section.repositories')}
                    <span className="profile-section__count">{owned.length}</span>
                  </span>
                }
              >
                {owned.length === 0 ? (
                  <EmptyState title={t('profile.empty.repositories')} />
                ) : (
                  <div className="profile-repos">
                    {owned.map((repository) => (
                      <RepositoryRow key={repository.id} repository={repository} />
                    ))}
                  </div>
                )}
              </SectionCard>

              {activities.length === 0 ? (
                <SectionCard
                  variant="plain"
                  title={
                    <span className="profile-section__title">
                      <Icon icon={sparkleIcon} size={16} />
                      {t('profile.section.activities')}
                    </span>
                  }
                >
                  <EmptyState title={t('profile.empty.activities')} />
                </SectionCard>
              ) : (
                <TeamActivity
                  items={activities}
                  title={
                    <span className="profile-section__title">
                      <Icon icon={sparkleIcon} size={16} />
                      {t('profile.section.activities')}
                    </span>
                  }
                />
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
