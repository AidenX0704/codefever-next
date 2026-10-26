import { Link } from 'react-router'

import type { Commit } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { CopyButton } from '@/components/unit/CopyButton'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Icon } from '@/components/unit/Icon'
import { Tag } from '@/components/unit/Tag'
import { plCopy } from '@/config/icons'
import type { ResolvedMember } from '@/helpers/members'
import { useT } from '@/i18n'


export interface CommitRowProps {
  commit: Commit
  /** 详情页链接前缀（`/{组}/{仓库}/commit`）。 */
  linkBase: string
  author: ResolvedMember
  /** 是否显示右侧短哈希（blame 模式不显示，与旧版一致）。 */
  showHash?: boolean
  /**
   * 自定义链接地址（入参 8 位短哈希）。
   * 旧版在子目录提交列表里点提交会跳到「该提交的这个目录」（files@hash+path），
   * 而不是提交详情；需要这个行为时传入，例如
   * `(sha8) => `${base}/files/${subPath}?rev=${sha8}``。
   */
  makeHref?: (sha8: string) => string
}

/**
 * 提交行。
 *
 * 结构对齐旧 `components/unit/CommitItem.js`：
 *   第一行 = 提交标题（点进详情）；第二行 = 头像 + 作者名（临时用户带提示）+ 「提交于 时间」；
 *   右侧 = 8 位短哈希 + 复制按钮（旧 `CommitHashChip` 的组合）。
 */
export function CommitRow({ commit, linkBase, author, showHash = true, makeHref }: CommitRowProps) {
  const t = useT()
  const href = makeHref ? makeHref(commit.sha.slice(0, 8)) : `${linkBase}/${commit.sha.slice(0, 8)}`

  return (
    <div className="commit-row">
      <div className="commit-row__main">
        <Link className="commit-row__message text-overflow" to={href}>
          {commit.commit}
        </Link>
        <div className="commit-row__meta">
          <Avatar size="sm" src={author.icon ?? undefined} name={author.name} />
          <span className="commit-row__author">{author.name}</span>
          {author.temporary ? <Tag tone="warning">{t('message.itsMyEmail')}</Tag> : null}
          <span className="commit-row__time">
            {t('label.editIn')}: <FormattedTime timestamp={Number(commit.time)} />
          </span>
        </div>
      </div>
      {showHash ? (
        <div className="commit-row__hash-group">
          <Link className="commit-row__hash" to={href}>
            {commit.sha.slice(0, 8)}
          </Link>
          <CopyButton text={commit.sha} label={t('label.copyHash')} message={t('label.copied')}>
            <Icon icon={plCopy} size={14} />
          </CopyButton>
        </div>
      ) : null}
    </div>
  )
}
