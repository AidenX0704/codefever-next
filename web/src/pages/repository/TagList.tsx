import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { deleteTag, fetchTagList } from '@/api/repository'
import type { Tag as TagType } from '@/api/types'
import { useConfirm } from '@/components/unit/Confirm'
import { useToast } from '@/components/unit/Toast'
import { RefListView, type RefListItem } from '@/components/view/unit/RefListView'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'

/**
 * 标签列表（`/{组}/{仓库}/tags`）。
 *
 * `repository/tagList` 的每个标签可能带 `detail`（annotated tag 的说明）与 `commit`
 * —— 后端用指针 + omitempty 复刻 PHP 的"键可能不存在"，所以这里都要做缺省处理。
 *
 * 页面只做取数与动作注入；搜索 / 排序 / 分页由 `RefListView` 承担（AGENTS §6）。
 */
export default function TagList() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const navigate = useNavigate()
  const { groupName, repositoryName } = useParams()
  const { summary, config, refresh } = useRepository()

  const [tags, setTags] = useState<TagType[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(() => {
    if (!summary) {
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchTagList(summary.id)
      .then((list) => {
        if (!cancelled) {
          setTags(list)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : '加载失败')
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [summary])

  useEffect(() => load(), [load])

  const base = `/${groupName}/${repositoryName}`

  const items: RefListItem[] = tags.map((tag) => ({
    key: tag.id || tag.name,
    name: tag.name,
    detail: tag.detail,
    commit: tag.commit,
  }))

  const remove = async (name: string) => {
    if (!summary) {
      return
    }
    const ok = await confirm({
      title: t('label.delete'),
      message: t('message.confirmDelete', { s: `${t('label.tag')} '${name}'` }),
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    try {
      await deleteTag(summary.id, name)
      toast.success(t('message.deleted'))
      load()
      refresh()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('base_error_message'))
    }
  }

  return (
    <RefListView
      kind="tag"
      items={items}
      loading={loading}
      error={error}
      members={config?.members}
      base={base}
      onDelete={remove}
      onCreate={() => navigate(`${base}/tags/new`)}
    />
  )
}
