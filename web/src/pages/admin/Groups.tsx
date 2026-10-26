import { useCallback, useEffect, useState } from 'react'

import {
  adminAddGroupMember,
  adminRemoveGroupMember,
  fetchAdminGroupMembers,
  fetchAdminGroups,
  setGroupOwner,
} from '@/api/admin'
import type { AdminMember, RepositoryGroup } from '@/api/types'
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

/** 排序白名单与后端一致（`domain.AdminGroupSortColumn`）：name / created。 */
const SORT_OPTIONS = [
  { value: 'name', labelKey: 'label.name' },
  { value: 'created', labelKey: 'label.createTime' },
] as const

/**
 * 管理后台 · 仓库组（`/admin/groups`）。
 *
 * `admin/groupList`（POST）+ `{keyword, sort, page, pagesize}` → `{count, list}`，
 * 列表元素是 `NormalizedGroup`（与业务页同一套字段）。
 *
 * 与旧版 `components/view/admin/Groups.js` 的对齐点：
 *   - 名称列带组图标（旧版 165-171，无图标时回退首字母头像）；
 *   - 创建时间列（旧版 173）；
 *   - 行内"成员管理"入口（旧版 189-201、387-389）；
 *   - 成员弹窗：按邮箱邀请、移除成员、设为所有者（旧版 203-337）。
 */
export default function AdminGroups() {
  const t = useT()
  const toast = useToast()

  const [keywordInput, setKeywordInput] = useState('')
  const keyword = useDebouncedValue(keywordInput, 200)
  const [sort, setSort] = useState('name')
  const [page, setPage] = useState(1)
  const [rows, setRows] = useState<RepositoryGroup[]>([])
  const [count, setCount] = useState(0)
  const [loading, setLoading] = useState(false)

  const [memberTarget, setMemberTarget] = useState<RepositoryGroup | null>(null)
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
    fetchAdminGroups({ keyword, sort, page, pagesize: PAGE_SIZE })
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
    async (group: RepositoryGroup) => {
      setMembersLoading(true)
      try {
        setMembers(await fetchAdminGroupMembers(group.id))
      } catch (error) {
        toast.error(error instanceof Error ? error.message : t('base_error_message'))
      } finally {
        setMembersLoading(false)
      }
    },
    [t, toast],
  )

  const openMembers = (group: RepositoryGroup) => {
    setMemberTarget(group)
    void loadMembers(group)
  }

  const invite = async (email: string) => {
    if (!memberTarget) {
      return
    }
    try {
      await adminAddGroupMember(memberTarget.id, email)
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
      await adminRemoveGroupMember(memberTarget.id, member.id)
      toast.success(t('message.removed'))
      await loadMembers(memberTarget)
    } catch (error) {
      throw new Error(adminMemberErrorText(error, t, 'message.error.removeFail'), { cause: error })
    }
  }

  const transferOwner = async (member: AdminMember) => {
    if (!memberTarget) {
      return
    }
    try {
      await setGroupOwner(memberTarget.id, member.id)
      toast.success(t('message.updated'))
      await loadMembers(memberTarget)
    } catch (error) {
      throw new Error(adminMemberErrorText(error, t, 'message.error.updateFail'), { cause: error })
    }
  }

  return (
    <div className="page">
      <div className="row">
        <TextField
          containerClassName="flex-grow"
          placeholder={t('message.searchGroup')}
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
            title: t('label.groupName'),
            render: (row) => (
              <span className="row">
                <Avatar size="sm" src={row.icon ? `/file/fetch/avatar/${row.icon}` : undefined} name={row.name} />
                <span>{row.displayName}</span>
              </span>
            ),
          },
          { key: 'name', title: t('label.name') },
          { key: 'description', title: t('label.groupDescription') },
          { key: 'owner', title: t('label.owner') },
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
        emptyTitle={t('message._S_listEmpty', { s: t('label.group_pl') })}
      />

      <Pagination page={page} pageCount={Math.max(1, Math.ceil(count / PAGE_SIZE))} total={count} onChange={setPage} />

      <AdminMembersDialog
        open={memberTarget !== null}
        title={t('label.memberManage')}
        members={members}
        loading={membersLoading}
        allowSetOwner
        onClose={() => setMemberTarget(null)}
        onInvite={invite}
        onRemove={remove}
        onSetOwner={transferOwner}
      />
    </div>
  )
}
