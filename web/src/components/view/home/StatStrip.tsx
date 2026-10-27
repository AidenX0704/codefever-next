import { Stat } from '@/components/unit/Stat'


export interface StatItem {
  key: string
  label: string
  /** 取不到时传 `undefined` → 渲染 `—`（不留空白，避免布局跳动）。 */
  value: number | string | undefined
  onClick?: () => void
}

export interface StatStripProps {
  items: ReadonlyArray<StatItem>
}

/**
 * KPI 条：一行若干指标，之间用 1px 竖线分隔。
 *
 * 刻意**没有卡片外框、没有底色** —— 见 `docs/HOME_PAGE_PLAN_2026-10-27.md` §2.2：
 * 对内平台首页密度优先，把 4 个指标做成 4 个方块卡是"用装饰冒充信息量"。
 */
export function StatStrip({ items }: StatStripProps) {
  return (
    <div className="home-stats">
      {items.map((item) => (
        <Stat
          key={item.key}
          label={item.label}
          value={item.value}
          onClick={item.onClick}
          className="home-stats__item"
        />
      ))}
    </div>
  )
}
