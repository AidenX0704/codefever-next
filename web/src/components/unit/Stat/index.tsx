import type { ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface StatProps {
  /** 指标名（如"我的仓库"）。 */
  label: ReactNode
  /** 指标值；`undefined` 时渲染占位符 `—`（加载中/取数失败都走这里，避免布局跳动）。 */
  value: ReactNode
  /** 传入则渲染为可点击（跳到对应列表页）。 */
  onClick?: () => void
  className?: string
}

/**
 * 单个指标（KPI）。
 *
 * 形态刻意做得扁平：**没有卡片外框、没有底色**，只有"数字 + 标签"两行。
 * 依据见 `docs/HOME_PAGE_PLAN_2026-10-27.md` §2.2「风格结论」：对内平台的首页
 * 信息密度优先，卡片只留给门户区；把 4 个指标做成 4 个方块卡属于"用装饰冒充信息量"。
 *
 * 出现场景：首页 KPI 条（4 项）、管理后台用量卡（后续收敛时复用）。
 */
export function Stat({ label, value, onClick, className }: StatProps) {
  const content = (
    <>
      <span className="cf-stat__value">{value === undefined || value === null ? '—' : value}</span>
      <span className="cf-stat__label">{label}</span>
    </>
  )

  if (onClick) {
    return (
      <button type="button" className={cn('cf-stat', 'cf-stat--link', className)} onClick={onClick}>
        {content}
      </button>
    )
  }

  return <div className={cn('cf-stat', className)}>{content}</div>
}
