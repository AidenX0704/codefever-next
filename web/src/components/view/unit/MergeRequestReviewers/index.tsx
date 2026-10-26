import type { MergeRequestReviewer, RepositoryMember } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { Icon } from '@/components/unit/Icon'
import { avatarUrl } from '@/config/constants'
import { psClock, psConfirm } from '@/config/icons'
import { useT } from '@/i18n'


export interface MergeRequestReviewersProps {
  /** 评审记录（后端返回 `{ id, user, isReview }`，`user` 才是用户主键）。 */
  reviewers: MergeRequestReviewer[]
  /** 目标仓库成员：按 `member.id` 匹配 `reviewer.user` 取头像与姓名。 */
  members: RepositoryMember[]
}

/**
 * 评审人头像组（已评审 / 未评审角标）。
 *
 * 逐字对齐旧版 `components/unit/MergeRequestReviewers.js`：
 *   - 头像取**目标仓库**成员（`reviewers[i].user` → `members[].id`）；
 *   - 右下角角标：已评审 = 对勾（success 色），未评审 = 时钟；
 *   - 头像相互叠压 8px。
 *
 * 入参刻意做成「评审记录 + 成员」两个纯数据，详情页的评审人面板也能复用。
 */
export function MergeRequestReviewers({ reviewers, members }: MergeRequestReviewersProps) {
  const t = useT()

  if (reviewers.length === 0) {
    return null
  }

  return (
    <div className="cf-mr-reviewers">
      {reviewers.map((reviewer) => {
        const member = members.find((item) => item.id === reviewer.user)
        const label = reviewer.isReview ? t('message.approved') : t('message.notApprove')
        return (
          <span className="cf-mr-reviewer" key={reviewer.id} title={`${member?.name ?? reviewer.user} · ${label}`}>
            <Avatar src={avatarUrl(member?.icon)} name={member?.name} />
            <span
              className={reviewer.isReview ? 'cf-mr-reviewer__badge cf-mr-reviewer__badge--done' : 'cf-mr-reviewer__badge'}
              aria-label={label}
            >
              <Icon icon={reviewer.isReview ? psConfirm : psClock} size={10} />
            </span>
          </span>
        )
      })}
    </div>
  )
}
