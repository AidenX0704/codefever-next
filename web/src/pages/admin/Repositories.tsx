import { useCallback, useEffect, useState } from 'react'

import {
  adminAddRepositoryMember,
  adminRemoveRepositoryMember,
  fetchAdminRepositoryMembers,
  fetchAdminRepositories,
} from '@/api/admin'
import type { AdminMember, Repository } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { DataTable, Pagination } from '@/components/unit/Table'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { SelectField } from '@/components/unit/SelectField'
import { Tag } from '@/components/unit/Tag'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import {
  AdminMembersDialog,
  adminMemberErrorText,
} from '@/components/view/unit/AdminMembersDialog'
import { useDebouncedValue } from '@/helpers/useDebouncedValue'
import { useT } from '@/i18n'


const PAGE_SIZE = 20

/** 排序白名单与后端一致（`domain.AdminRepositorySortColumn`）：name / created。 */
const SORT_OPTIONS = [
  { value: 'name', labelKey: 'label.name' },
  { value: 'created', labelKey: 'label.createTime' },
] as const

/**
 * 管理后台 · 仓库（`/admin/repositories`）。
 *
 * `admin/repositoryList`（POST）→ `{count, list}`，元素是 `NormalizedRepository`
 * （含 `group` / `forkCount` / `mergeRequestCount`）。
 *
 * 与旧版 `components/view/admin/Repositories.js` 的对齐点：
 *   - 名称列带仓库图标（旧版 165-171，无图标时回退首字母头像）；
 *   - 创建时间列（旧版 173）；
 *   - 行内"成员管理"入口（旧版 189-201、350-352）；
 *   - 成员弹窗里必须区分**组继承成员**（`groupMember`）与**仓库直授**：
 *     这是旧版唯一能看出「权限来自组还是来自仓库」的入口（旧版 225-228），
 *     组继承成员在仓库侧不可移除（旧版 232）。
 */
export default function AdminRepositories() {
  const t = useT()
  const toast = useToast()

  const [keywordInput, setKeywordInput] = useState('')
  const keyword = useDebouncedValue(keywordInput, 200)
  const [sort, setSort] = useState('name')
  const [page, setPage] = useState(1)
  const [rows, setRows] = useState<Repository[]>([])
  const [count, setCount] = useState(0)
  const [loading, setLoading] = useState(false)

  const [memberTarget, setMemberTarget] = useState<Repository | null>(null)
  const [members, setMembers] = useState<AdminMember[]>([])
  const [membersLoading, setMembersLoading] = useState(false)

  // 过滤条件变化后回到第一页（渲染期派生，避免多打一次接口）。
  const [lastFilter, setLastFilter] = useState({ sort, keyword })
  if (lastFilter.sort !== sort || lastFilter.keyword !== keyword) {
    setLastFilter({ sort, keyword })
    setPage(1)
  }

  const load = useCallback(() => {
    setLoading(true)
    fetchAdminRepositories({ keyword, sort, page, pagesize: PAGE_SIZE })
      .then((res) => {
        setRows(res.list)
        setCount(res.count)
      })
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => setLoading(false))
  }, [keyword, sort, page, t, toast])

  useEffect(() => {
    load()
  }, [load])

  const loadMembers = useCallback(
    async (repository: Repository) => {
      setMembersLoading(true)
      try {
        setMembers(await fetchAdminRepositoryMembers(repository.id))
      } catch (error) {
        toast.error(error instanceof Error ? error.message : t('base_error_message'))
      } finally {
        setMembersLoading(false)
      }
    },
    [t, toast],
  )

  const openMembers = (repository: Repository) => {
    setMemberTarget(repository)
    void loadMembers(repository)
  }

  const invite = async (email: string) => {
    if (!memberTarget) {
      return
    }
    try {
      await adminAddRepositoryMember(memberTarget.id, email)
      toast.success(t('message.Added'))
      await loadMembers(memberTarget)
    } catch (error) {
      throw new Error(adminMemberErrorText(error, t, 'message.error.AddFail'), { cause: error })
    }
  }

  const remove = async (member: AdminMember) => {
    if (!memberTarget) {
      return
    }
    try {
      await adminRemoveRepositoryMember(memberTarget.id, member.id)
      toast.success(t('message.removed'))
      await loadMembers(memberTarget)
    } catch (error) {
      throw new Error(adminMemberErrorText(error, t, 'message.error.removeFail'), { cause: error })
    }
  }

  return (
    <div className="page">
      <div className="row">
        <TextField
          containerClassName="flex-grow"
          placeholder={t('message.searchRepository')}
          value={keywordInput}
          onChange={(event) => setKeywordInput(event.target.value)}
        />
        <span className="muted">{t('label.sort')}</span>
        <SelectField
          value={sort}
          options={SORT_OPTIONS.map((option) => ({ value: option.value, label: t(option.labelKey) }))}
          onChange={(event) => setSort(event.target.value)}
        />
      </div>

      <DataTable
        columns={[
          {
            key: 'displayName',
            title: t('label.repositoryName'),
            render: (row) => (
              <span className="row">
                <Avatar size="sm" src={row.icon ? `/file/fetch/avatar/${row.icon}` : undefined} name={row.name} />
                <span>{row.displayName}</span>
              </span>
            ),
          },
          { key: 'name', title: t('label.name') },
          { key: 'group', title: t('label.group'), render: (row) => row.group?.displayName ?? '' },
          { key: 'forkCount', title: t('label.forkRepository') },
          {
            key: 'created',
            title: t('label.createdAt'),
            render: (row) => <FormattedTime timestamp={row.created} />,
          },
          {
            key: 'ops',
            title: '',
            render: (row) => (
              <span className="row">
                {row.status === 1 ? (
                  <Button size="sm" variant="text" onClick={() => openMembers(row)}>
                    {t('label.memberManage')}
                  </Button>
                ) : (
                  <Tag tone="danger">{t('message.deleted')}</Tag>
                )}
              </span>
            ),
          },
        ]}
        rows={rows}
        rowKey={(row) => row.id}
        loading={loading}
        emptyTitle={t('message._S_listEmpty', { s: t('label.repository_pl') })}
      />

      <Pagination page={page} pageCount={Math.max(1, Math.ceil(count / PAGE_SIZE))} total={count} onChange={setPage} />

      <AdminMembersDialog
        open={memberTarget !== null}
        title={t('label.memberManage')}
        members={members}
        loading={membersLoading}
        showGroupMember
        onClose={() => setMemberTarget(null)}
        onInvite={invite}
        onRemove={remove}
      />
    </div>
  )
}
