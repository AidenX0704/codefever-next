import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router'

import { fetchRefListContainSHA } from '@/api/repository'
import { Button } from '@/components/unit/Button'
import { Icon } from '@/components/unit/Icon'
import { psBranch, psTag } from '@/config/icons'
import { useT } from '@/i18n'


export interface CommitRefsProps {
  repositoryKey: string
  /** 父提交 SHA 列表（后端 `parent` 是空格分隔的多个 SHA）。 */
  parentHashes: string[]
  /** 当前提交 SHA（用来查"包含该提交的分支 / 标签"）。 */
  currentHash: string
  /** 跳转前缀（`/{组}/{仓库}`）。 */
  base: string
}

/**
 * 提交详情里的"父提交 + 包含该提交的分支 / 标签"。
 *
 * 对应旧版 `components/view/unit/CommitContainRef.js`：用
 * `repository/refListContainSHA` 查出包含当前提交的分支与标签，
 * 父提交则是 `commitDetail.parent` 里的 SHA 列表。
 */
export function CommitRefs({ repositoryKey, parentHashes, currentHash, base }: CommitRefsProps) {
  const t = useT()
  const navigate = useNavigate()
  const [branches, setBranches] = useState<string[]>([])
  const [tags, setTags] = useState<string[]>([])

  useEffect(() => {
    if (!repositoryKey || !currentHash) {
      return
    }
    let cancelled = false
    fetchRefListContainSHA(repositoryKey, currentHash)
      .then((result) => {
        if (!cancelled) {
          setBranches(result.branch)
          setTags(result.tag)
        }
      })
      .catch(() => {
        /* 查不到包含引用不影响父提交展示 */
      })
    return () => {
      cancelled = true
    }
  }, [repositoryKey, currentHash])

  return (
    <div className="commit-refs">
      <div className="commit-refs__block">
        <span className="commit-refs__label">{t('label.parentNode')}</span>
        <div className="commit-refs__links">
          {parentHashes.length === 0 ? (
            <span className="muted">-</span>
          ) : (
            parentHashes.map((hash) => (
              <Link className="cf-link commit-refs__link" key={hash} to={`${base}/commit/${hash.slice(0, 8)}`}>
                {hash.slice(0, 8)}
              </Link>
            ))
          )}
        </div>
      </div>

      <div className="commit-refs__block">
        <span className="commit-refs__label">{t('message.someVersion')}</span>
        <div className="commit-refs__links">
          {branches.map((name) => (
            <Button
              key={`b-${name}`}
              variant="outlined"
              size="sm"
              startIcon={<Icon icon={psBranch} size={14} />}
              onClick={() => navigate(`${base}/files?rev=${encodeURIComponent(name)}`)}
            >
              {name}
            </Button>
          ))}
          {tags.map((name) => (
            <Button
              key={`t-${name}`}
              variant="outlined"
              size="sm"
              startIcon={<Icon icon={psTag} size={14} />}
              onClick={() => navigate(`${base}/files?rev=${encodeURIComponent(name)}`)}
            >
              {name}
            </Button>
          ))}
        </div>
      </div>
    </div>
  )
}
