import { useEffect, useState } from 'react'

import type { SiteStats } from '@/api/site'
import { fetchSiteStats } from '@/api/site'
import { AnnouncementCard } from '@/components/view/home/AnnouncementCard'
import { ChangelogPanel } from '@/components/view/home/ChangelogPanel'
import { PlatformHero } from '@/components/view/home/PlatformHero'
import { PlatformSection } from '@/components/view/home/PlatformSection'


/**
 * 展示首页（`/home`，gitee.com 口径）。
 *
 * 2026-11-16 二次定版：/home 只做**平台展示** ——
 *   1. PlatformHero：平台名 + 定位 + 平台规模（已合并 MR 计成果数）+ 新手动作；
 *   2. PlatformSection：平台概览 / 四步上手 / 团队规范 / 平台能力 + 快捷入口；
 *   3. 平台消息：公告 + 更新日志双栏。
 *
 * 个人内容（问候 / KPI / 待办 / 我的仓库 / 最近访问 / 团队动态）**全部在
 * `/:username` 个人主页** —— 两层分工：/home 回答"这是个什么平台"，
 * /:username 回答"我今天要做什么"。
 *
 * 数据只用匿名的 `site/stats`（1 个请求）：展示页的数字是全站成果，
 * 不依赖登录态聚合（home/overview 是个人工作台的请求，随个人页走）。
 */
export default function Home() {
  const [stats, setStats] = useState<SiteStats | undefined>(undefined)

  useEffect(() => {
    let cancelled = false
    fetchSiteStats()
      .then((data) => {
        if (!cancelled) {
          setStats(data ?? undefined)
        }
      })
      .catch(() => {
        // 统计失败只影响数字条（显示"—"），展示区与平台消息照常渲染。
        if (!cancelled) {
          setStats(undefined)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="page home-page">
      <PlatformHero stats={stats} />

      <PlatformSection stats={stats} />

      <div className="home-messages">
        <AnnouncementCard />
        <ChangelogPanel />
      </div>
    </div>
  )
}
