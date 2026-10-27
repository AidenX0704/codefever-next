import { SectionCard } from '@/components/unit/SectionCard'
import { useT } from '@/i18n'


/**
 * 「平台公告」。
 *
 * 数据源现状：**内置三条使用须知**（安全规范 / 保密提醒 / 支持渠道），不读配置。
 *
 * 为什么不接站点配置：`site.announcement` 这个字段目前不存在于 `config.yaml` 与
 * 管理后台表单里，凭空读一个不存在的键只会在页面上留一块空。方案里把它排在
 * **P2 批**（后台可编辑 + 热加载），落地时把这里换成 `useAppData` 里的配置即可。
 * 宁可先展示确定的、对员工有用的三条须知，也不做"敬请期待"的占位。
 */
export function AnnouncementCard() {
  const t = useT()

  return (
    <SectionCard title={t('home.section.announcement')}>
      <ul className="home-announcement">
        <li>{t('home.announcement.security')}</li>
        <li>{t('home.announcement.confidential')}</li>
        <li>{t('home.announcement.support')}</li>
      </ul>
    </SectionCard>
  )
}
