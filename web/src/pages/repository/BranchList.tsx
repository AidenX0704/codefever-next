import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { ApiError } from '@/api/client'
import { deleteBranch, fetchBranchList } from '@/api/repository'
import type { Branch } from '@/api/types'
import { useConfirm } from '@/components/unit/Confirm'
import { useToast } from '@/components/unit/Toast'
import { RefListView, type RefListItem } from '@/components/view/unit/RefListView'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'

/**
 * 分支列表（`/{组}/{仓库}/branches`）。
 *
 * 数据链路对齐旧 `components/view/BranchList.js` + `components/unit/RefList.js`：
 * `repository/branchList` 每个分支带上最近一次提交（commit）、`protected` 标记与
 * `isMerge`（与默认分支无差异 = 已合并）。
 *
 * 页面只做三件事：取数、映射成 `RefListItem`、注入删除/新建动作；
 * 搜索 / 排序 / 活跃页签 / 分页由 `RefListView` 承担（AGENTS §6）。
 */
export default function BranchList() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const navigate = useNavigate()
  const { groupName, repositoryName } = useParams()
  const { summary, config, refresh } = useRepository()

  const [branches, setBranches] = useState<Branch[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(() => {
    if (!summary) {
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchBranchList(summary.id)
      .then((list) => {
        if (!cancelled) {
          setBranches(list)
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
  const defaultBranch = config?.repository.defaultBranch ?? ''

  const items: RefListItem[] = branches.map((branch) => ({
    key: branch.id || branch.name,
    name: branch.name,
    commit: branch.commit,
    protected: branch.protected,
    isDefault: Boolean(defaultBranch) && branch.name === defaultBranch,
    isMerge: branch.isMerge,
  }))

  const remove = async (name: string) => {
    if (!summary) {
      return
    }
    const ok = await confirm({
      title: t('label.delete'),
      message: t('message.confirmDelete', { s: `${t('label.branch')} '${name}'` }),
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    try {
      await deleteBranch(summary.id, name)
      toast.success(t('message.deleted'))
      load()
      refresh()
    } catch (err) {
      // 0x040F：分支受保护（后端保护分支规则拒绝删除）。
      if (err instanceof ApiError && err.code === 0x040f) {
        toast.error(t('message.error.branchProtected'))
      } else {
        toast.error(err instanceof Error ? err.message : t('base_error_message'))
      }
    }
  }

  return (
    <RefListView
      kind="branch"
      items={items}
      loading={loading}
      error={error}
      members={config?.members}
      base={base}
      showActiveTabs
      canDelete={(item) => !item.isDefault}
      onDelete={remove}
      onCreate={() => navigate(`${base}/branches/new`)}
    />
  )
}
