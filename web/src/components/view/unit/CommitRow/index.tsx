import { Link } from 'react-router'

import type { Commit } from '@/api/types'
import { CopyButton } from '@/components/unit/CopyButton'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Tag } from '@/components/unit/Tag'
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
 * 提交行（v5 §6 G6）。
 *
 * 结构：头像 24（成员有头像用图，否则首字母圆）+ 两行主列（标题 13/500 + 
 * "作者 · 提交于 时间" 12 fg-3）+ 右侧 8 位短哈希芯片（well 底）+ 复制。
 */
export function CommitRow({ commit, linkBase, author, showHash = true, makeHref }: CommitRowProps) {
  const t = useT()
  const href = makeHref ? makeHref(commit.sha.slice(0, 8)) : `${linkBase}/${commit.sha.slice(0, 8)}`

  return (
    <div className="commit-row">
      {author.icon ? (
        <img className="commit-row__avatar" src={`/file/fetch/avatar/${author.icon}`} alt="" />
      ) : (
        <span className="commit-row__avatar" aria-hidden="true">
          {author.name.slice(0, 1).toUpperCase()}
        </span>
      )}
      <div className="commit-row__main">
        <Link className="commit-row__message text-overflow" to={href}>
          {commit.commit}
        </Link>
        <div className="commit-row__meta">
          <span className="commit-row__author">{author.name}</span>
          {author.temporary ? <Tag tone="warning">{t('message.itsMyEmail')}</Tag> : null}
          <span className="commit-row__time">
            <FormattedTime timestamp={Number(commit.time)} />
          </span>
        </div>
      </div>
      {showHash ? (
        <div className="commit-row__hash-group">
          <Link className="commit-row__hash" to={href}>
            {commit.sha.slice(0, 7)}
          </Link>
          <CopyButton text={commit.sha} label={t('label.copyHash')} message={t('label.copied')} />
        </div>
      ) : null}
    </div>
  )
}
