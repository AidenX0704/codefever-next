import type { MergeRequestListItem } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { EmptyState } from '@/components/unit/EmptyState'
import { Loading } from '@/components/unit/Loading'
import { SectionCard } from '@/components/unit/SectionCard'
import { MergeRequestRow } from '@/components/view/unit/MergeRequestRow'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


export interface MergeRequestPanelProps {
  title: string
  /** `undefined` = 还没取到（渲染加载态）；`[]` = 确实没有（渲染空态）。 */
  items: MergeRequestListItem[] | undefined
  /** 空态说明文案。 */
  emptyNotice: string
  /** 标题行右侧的"全部 →"。 */
  onViewAll: () => void
  /** 行尾是否显示"待合并 N 天"（用于"我发起的"）。 */
  showAge?: boolean
}

/**
 * 首页待办面板（「待我评审」/「我发起的」共用）。
 *
 * 复用 `MergeRequestRow` 的 **compact 变体**：首页数据来自
 * `GET /api/home/overview`，不含 `commit` / `reviewers`（后端不跑 git），
 * 因此不能走完整行渲染。
 *
 * 空态给"下一步动作"而不是插画（DeepSeek 取向，见方案 §2.2）。
 */
export function MergeRequestPanel({ title, items, emptyNotice, onViewAll, showAge = false }: MergeRequestPanelProps) {
  const t = useT()
  const { repositories } = useAppData()

  /** 行链接：`/{组名}/{仓库名}/mergerequests/{编号}`（目标仓库不存在时整行不可点）。 */
  const hrefOf = (item: MergeRequestListItem): string => {
    const target = repositories.find((repository) => repository.id === item.targetRepository)
    return target ? `/${target.group.name}/${target.name}/mergerequests/${item.number}` : ''
  }

  const ageDays = (item: MergeRequestListItem): number => {
    const seconds = Number(item.update) || 0
    if (!seconds) {
      return 0
    }
    return Math.max(0, Math.floor((Date.now() / 1000 - seconds) / 86400))
  }

  return (
    <SectionCard
      title={
        <>
          {title}
          {items && items.length > 0 ? <span className="home-panel__count">{items.length}</span> : null}
        </>
      }
      actions={
        <Button variant="text" onClick={onViewAll}>
          {t('home.action.viewAll')}
        </Button>
      }
    >
      {items === undefined ? (
        <Loading />
      ) : items.length === 0 ? (
        <EmptyState
          title={t('message._S_listEmpty', { s: t('label.mergeRequest') })}
          notice={emptyNotice}
          action={
            <Button variant="outlined" onClick={onViewAll}>
              {t('home.action.viewAll')}
            </Button>
          }
        />
      ) : (
        <div className="home-panel__list">
          {items.map((item) => {
            const href = hrefOf(item)
            return (
              <div key={item.id} className="home-panel__row">
                <MergeRequestRow item={item} href={href} showRepository compact />
                {showAge ? (
                  <span className="home-panel__age">
                    {t('home.mrAge', { n: ageDays(item) })}
                  </span>
                ) : null}
              </div>
            )
          })}
        </div>
      )}
    </SectionCard>
  )
}
