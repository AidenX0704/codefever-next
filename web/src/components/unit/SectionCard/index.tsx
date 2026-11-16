import type { ReactNode } from 'react'

import { cn } from '@/helpers/cn'


export interface SectionCardProps {
  /** 区块标题。 */
  title: ReactNode
  /**
   * 形态：
   *  - `plain`（默认）：无外框，底部 1px 分隔线 —— 用于工作台区（待办 / 仓库 / 动态），
   *    列表本身才是主体，容器不该比内容更抢眼；
   *  - `card`：白底 + 1px 边框 + 圆角 —— 只用于门户区（了解平台的那一层）。
   */
  variant?: 'plain' | 'card'
  /** 标题行右侧的操作区（"全部 →"、计数徽标等）。 */
  actions?: ReactNode
  /** 标题行下方的工具条（页签、搜索框等；首页「我的仓库」用它放页签）。 */
  toolbar?: ReactNode
  children: ReactNode
  className?: string
}

/**
 * 带标题的区块容器。
 *
 * 为什么要有它：首页有 8 个区块，每块都是"标题 + 右侧操作 + 内容"。
 * 若各写一遍，就会出现 8 份 `<section><div class="section__head">…` 的结构性重复
 * （AGENTS §4：同一片段出现第二次就要抽）。
 *
 * DESIGN.md §3.2：区块标题不再使用左侧蓝色竖条，层级由字重 + 间距建立
 *（v3 的 `--barred` 修饰符与 `--section-bar-*` 令牌已随 v4 移除）。
 */
export function SectionCard({ title, variant = 'plain', actions, toolbar, children, className }: SectionCardProps) {
  return (
    <section
      className={cn(
        variant === 'card' ? 'cf-card cf-card--flat cf-section-card' : 'section',
        className,
      )}
    >
      <div className="section__head">
        <h2 className="section__title">{title}</h2>
        {actions ? <div className="section__actions">{actions}</div> : null}
      </div>
      {toolbar ? <div className="section__toolbar">{toolbar}</div> : null}
      {children}
    </section>
  )
}
