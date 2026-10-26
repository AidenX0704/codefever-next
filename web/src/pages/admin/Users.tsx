import { useCallback, useEffect, useState } from 'react'

import { ApiError } from '@/api/client'
import {
  ADMIN_USER_CATEGORY,
  addAdminUserWithPassword,
  checkAdminPassword,
  closeUserMFA,
  fetchAdminUsers,
  resetAdminUserPassword,
  setAdministrator,
  updateUserStatus,
} from '@/api/admin'
import type { AdminUserListItem } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { useConfirm } from '@/components/unit/Confirm'
import { CopyButton } from '@/components/unit/CopyButton'
import { DataTable, Pagination } from '@/components/unit/Table'
import { Dialog } from '@/components/unit/Dialog'
import { Form, FormItem } from '@/components/unit/Form'
import { SelectField } from '@/components/unit/SelectField'
import { Tabs } from '@/components/unit/Tabs'
import { Tag } from '@/components/unit/Tag'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { useDebouncedValue } from '@/helpers/useDebouncedValue'
import { useT } from '@/i18n'


const PAGE_SIZE = 20

/** 页签 → 后端分类（`domain.AdminUserCategory*`）。 */
const CATEGORY_KEYS = {
  all: ADMIN_USER_CATEGORY.all,
  mfaEnabled: ADMIN_USER_CATEGORY.mfaEnabled,
  mfaDisabled: ADMIN_USER_CATEGORY.mfaDisabled,
  disabled: ADMIN_USER_CATEGORY.disabled,
} as const

type CategoryKey = keyof typeof CATEGORY_KEYS

/** 排序白名单与后端一致（`domain.AdminUserSortColumn`）：name / email / created。 */
const SORT_OPTIONS = [
  { value: 'name', labelKey: 'label.name' },
  { value: 'email', labelKey: 'label.email' },
  { value: 'created', labelKey: 'label.createTime' },
] as const

/**
 * 邮箱格式校验。
 *
 * 逐字取自后端 `domain.AdminEmailValid`（`^[a-z0-9]+([._\-]*[a-z0-9])*@...`），
 * 与旧版 `admin/Users.js:123` 的正则一致（**区分大小写**，大写邮箱会被判非法）。
 */
const EMAIL_PATTERN = /^[a-z0-9]+([._-]*[a-z0-9])*@([a-z0-9]+[-a-z0-9]*[a-z0-9]+.){1,63}[a-z0-9]+$/

/**
 * 管理后台 · 用户（`/admin/users`）。
 *
 * `admin/userList` 是 **POST** + `{category, keyword, sort, page, pagesize}`，返回 `{count, list}`。
 * 行内动作：启用/禁用、设为/取消管理员、关闭 MFA、重置口令（管理员重置不需要旧口令）。
 *
 * 与旧版 `components/view/admin/Users.js` 的对齐点：
 *   - 四个分类页签（旧版 392-401）→ `category`；
 *   - name/email/created 三种排序（旧版 415-425）→ `sort`；
 *   - 添加用户后弹窗展示初始口令并可一键复制（旧版 584-612）；
 *   - 设管理员前必须校验**当前管理员口令**（旧版 331、514-541）；
 *   - 关闭 MFA / 设管理员 / 重置口令均需二次确认；
 *   - 搜索输入 200ms 防抖（旧版 164-175），避免每次击键都打一次接口。
 */
export default function AdminUsers() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()

  const [keywordInput, setKeywordInput] = useState('')
  const keyword = useDebouncedValue(keywordInput, 200)
  const [category, setCategory] = useState<CategoryKey>('all')
  const [sort, setSort] = useState('name')
  const [page, setPage] = useState(1)
  const [rows, setRows] = useState<AdminUserListItem[]>([])
  const [count, setCount] = useState(0)
  const [loading, setLoading] = useState(false)

  const [resetTarget, setResetTarget] = useState<AdminUserListItem | null>(null)
  const [newPassword, setNewPassword] = useState('')
  const [newPassword2, setNewPassword2] = useState('')

  const [adminTarget, setAdminTarget] = useState<AdminUserListItem | null>(null)
  const [adminPassword, setAdminPassword] = useState('')

  const [addOpen, setAddOpen] = useState(false)
  const [addName, setAddName] = useState('')
  const [addEmail, setAddEmail] = useState('')
  const [adding, setAdding] = useState(false)
  const [created, setCreated] = useState<{ name: string; email: string; password: string } | null>(null)

  // 过滤条件（分类 / 关键字 / 排序）变化后回到第一页。
  // 用"渲染期派生"而不是 effect，避免多打一次接口（旧版 shouldComponentUpdate 同口径）。
  const [lastFilter, setLastFilter] = useState({ category, sort, keyword })
  if (lastFilter.category !== category || lastFilter.sort !== sort || lastFilter.keyword !== keyword) {
    setLastFilter({ category, sort, keyword })
    setPage(1)
  }

  const load = useCallback(() => {
    setLoading(true)
    fetchAdminUsers({ category: CATEGORY_KEYS[category], keyword, sort, page, pagesize: PAGE_SIZE })
      .then((res) => {
        setRows(res.list)
        setCount(res.count)
      })
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => setLoading(false))
  }, [category, keyword, sort, page, t, toast])

  useEffect(() => {
    load()
  }, [load])

  const run = async (action: () => Promise<unknown>, successText: string) => {
    try {
      await action()
      toast.success(successText)
      load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const toggleStatus = async (row: AdminUserListItem) => {
    const ok = await confirm({
      title: row.status ? t('message.userStatusDisabled') : t('message.userStatusEnabled'),
      message: t(row.status ? 'message.userStatusDisabled_N' : 'message.userStatusEnabled_N', { n: row.name }),
      tone: row.status ? 'danger' : 'primary',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    await run(() => updateUserStatus(row.id, row.status ? 0 : 1), row.status ? t('message.disabled') : t('message.enabled'))
  }

  const closeMFA = async (row: AdminUserListItem) => {
    const ok = await confirm({
      title: t('message.closeMFA'),
      message: t('message.closeMFA_N', { n: row.name }),
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    await run(() => closeUserMFA(row.id), t('message.closed'))
  }

  /** 设管理员是提权操作：先二次确认，再校验当前管理员口令（旧版 514-541）。 */
  const openSetAdministrator = async (row: AdminUserListItem) => {
    const ok = await confirm({
      title: row.admin ? t('message.cancelAdministrator') : t('message.setAdministrator'),
      message: row.name,
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    setAdminTarget(row)
    setAdminPassword('')
  }

  const submitSetAdministrator = async () => {
    if (!adminTarget) {
      return
    }
    if (!adminPassword) {
      toast.error(t('message.error._S_empty', { s: t('label.password') }))
      return
    }
    try {
      // 先校验本人口令，校验通过才允许改管理员标志（旧版 AdminData.checkPassword）。
      await checkAdminPassword(adminPassword)
    } catch (error) {
      // 0x0405 = 口令比对失败（后端 `admin/checkPassword`）；其它错误按通用错误提示。
      const code = error instanceof ApiError ? error.code : 0
      if (code === 0x0405) {
        toast.error(t('message.error._S_invalid', { s: t('label.password') }))
      } else {
        toast.error(error instanceof Error ? error.message : t('base_error_message'))
      }
      return
    }
    try {
      await setAdministrator(adminTarget.id, !adminTarget.admin)
      toast.success(t('message.updated'))
      setAdminTarget(null)
      load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const openResetPassword = async (row: AdminUserListItem) => {
    const ok = await confirm({
      title: t('message.resetPassword'),
      message: t('message.reset_N_Password', { n: row.name }),
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    setResetTarget(row)
    setNewPassword('')
    setNewPassword2('')
  }

  const submitResetPassword = async () => {
    if (!resetTarget) {
      return
    }
    if (newPassword.length < 6 || newPassword.length > 50) {
      toast.error(t('message.error.within_N1_to_N2_characters', { n1: 6, n2: 50 }))
      return
    }
    if (newPassword !== newPassword2) {
      toast.error(t('message.error.inputNotSame'))
      return
    }
    await run(() => resetAdminUserPassword(resetTarget.id, newPassword), t('message.reseted'))
    setResetTarget(null)
  }

  const submitAddUser = async () => {
    const name = addName.trim()
    const email = addEmail.trim()
    if (name.length < 1 || name.length > 15) {
      toast.error(t('message.error.within_N1_to_N2_characters', { n1: 1, n2: 15 }))
      return
    }
    if (!EMAIL_PATTERN.test(email)) {
      toast.error(t('message.error._S_invalid', { s: t('label.email') }))
      return
    }
    setAdding(true)
    try {
      // 后端返回明文初始口令，旧版直接弹窗展示给管理员（旧版 372、584-612）。
      const password = await addAdminUserWithPassword(name, email)
      if (!password) {
        toast.error(t('message.error.AddFail'))
        return
      }
      setAddOpen(false)
      setAddName('')
      setAddEmail('')
      setCreated({ name, email, password })
      load()
    } catch (error) {
      const code = error instanceof ApiError ? error.code : 0
      if (code === 0x0406) {
        toast.error(t('message.error._S_duplicate', { s: t('label.email') }))
        return
      }
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setAdding(false)
    }
  }

  return (
    <div className="page">
      <div className="list-header">
        <Tabs<CategoryKey>
          value={category}
          onChange={setCategory}
          items={[
            { key: 'all', label: t('label.all') },
            { key: 'mfaEnabled', label: t('label.enabledMFA') },
            { key: 'mfaDisabled', label: t('label.disabledMFA') },
            { key: 'disabled', label: t('label.blocked') },
          ]}
        />
        <Button
          variant="contained"
          onClick={() => {
            setAddOpen(true)
            setAddName('')
            setAddEmail('')
          }}
        >
          {t('label.addUser')}
        </Button>
      </div>

      <div className="row">
        <TextField
          containerClassName="flex-grow"
          placeholder={t('message.searchByName')}
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
            key: 'name',
            title: t('label.name'),
            render: (row) => (
              <span className="row">
                <Avatar size="sm" src={row.avatar ? `/file/fetch/avatar/${row.avatar}` : undefined} name={row.name} />
                <span>
                  {row.name}
                  <span className="muted"> · {row.email}</span>
                </span>
              </span>
            ),
          },
          { key: 'projects', title: t('label.repository_pl') },
          { key: 'created', title: t('label.createdAt') },
          {
            key: 'flags',
            title: '',
            render: (row) => (
              <span className="row">
                {row.mfaEnabled ? <Tag tone="success">2FA</Tag> : null}
                {row.admin ? <Tag tone="primary">{t('label.administrator')}</Tag> : null}
                {!row.status ? <Tag tone="danger">{t('message.disabled')}</Tag> : null}
              </span>
            ),
          },
          {
            key: 'ops',
            title: '',
            render: (row) => (
              <span className="row">
                <Button size="sm" variant="text" onClick={() => toggleStatus(row)}>
                  {row.status ? t('label.disable') : t('label.enable')}
                </Button>
                {row.status ? (
                  <>
                    <Button size="sm" variant="text" onClick={() => openSetAdministrator(row)}>
                      {row.admin ? t('label.removeAdministrator') : t('label.setAdministrator')}
                    </Button>
                    {row.mfaEnabled ? (
                      <Button size="sm" variant="text" tone="danger" onClick={() => closeMFA(row)}>
                        {t('label.removeMFADevice')}
                      </Button>
                    ) : null}
                    <Button size="sm" variant="text" tone="danger" onClick={() => openResetPassword(row)}>
                      {t('message.resetPassword')}
                    </Button>
                  </>
                ) : null}
              </span>
            ),
          },
        ]}
        rows={rows}
        rowKey={(row) => row.id}
        loading={loading}
        emptyTitle={t('message._S_listEmpty', { s: t('label.user_pl') })}
      />

      <Pagination page={page} pageCount={Math.max(1, Math.ceil(count / PAGE_SIZE))} total={count} onChange={setPage} />

      {/* 重置口令：旧版要求两次输入一致（旧版 Users.js:302-325）。 */}
      <Dialog
        open={resetTarget !== null}
        title={t('message.reset_N_Password', { n: resetTarget?.name ?? '' })}
        onClose={() => setResetTarget(null)}
        actions={
          <>
            <Button variant="text" onClick={() => setResetTarget(null)}>
              {t('label.cancel')}
            </Button>
            <Button variant="contained" onClick={submitResetPassword}>
              {t('label.ok')}
            </Button>
          </>
        }
      >
        <Form>
          <FormItem label={t('label.newPassword')} required help={resetTarget?.email}>
            <TextField type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} />
          </FormItem>
          <FormItem label={t('label.newPasswordConfirm')} required>
            <TextField type="password" value={newPassword2} onChange={(event) => setNewPassword2(event.target.value)} />
          </FormItem>
        </Form>
      </Dialog>

      {/* 设管理员：必须先输入当前管理员口令（旧版 Users.js:514-541）。 */}
      <Dialog
        open={adminTarget !== null}
        title={t(adminTarget?.admin ? 'message.cancelAdministrator' : 'message.setAdministrator')}
        onClose={() => setAdminTarget(null)}
        actions={
          <>
            <Button variant="text" onClick={() => setAdminTarget(null)}>
              {t('label.cancel')}
            </Button>
            <Button variant="contained" onClick={submitSetAdministrator}>
              {t('label.ok')}
            </Button>
          </>
        }
      >
        <Form>
          <FormItem label={t('label.password')} required help={adminTarget?.email}>
            <TextField
              type="password"
              value={adminPassword}
              onChange={(event) => setAdminPassword(event.target.value)}
            />
          </FormItem>
        </Form>
      </Dialog>

      {/* 添加用户（旧版 Users.js:544-583）。 */}
      <Dialog
        open={addOpen}
        title={t('label.addUser')}
        onClose={() => setAddOpen(false)}
        actions={
          <>
            <Button variant="text" onClick={() => setAddOpen(false)}>
              {t('label.cancel')}
            </Button>
            <Button variant="contained" loading={adding} onClick={submitAddUser}>
              {t('label.ok')}
            </Button>
          </>
        }
      >
        <Form>
          <FormItem label={t('label.name')} required>
            <TextField value={addName} onChange={(event) => setAddName(event.target.value)} />
          </FormItem>
          <FormItem label={t('label.email')} required>
            <TextField value={addEmail} onChange={(event) => setAddEmail(event.target.value)} />
          </FormItem>
        </Form>
      </Dialog>

      {/* 新建成功：展示后端生成的初始口令并提供一键复制（旧版 Users.js:584-612）。 */}
      <Dialog
        open={created !== null}
        title={t('message.successAddUser')}
        onClose={() => setCreated(null)}
        actions={
          <Button variant="contained" onClick={() => setCreated(null)}>
            {t('label.ok')}
          </Button>
        }
      >
        <div className="stack">
          <div>
            {t('label.name')}: {created?.name}
          </div>
          <div>
            {t('label.email')}: {created?.email}
          </div>
          <div className="row">
            <span>
              {t('label.newPassword')}: {created?.password}
            </span>
            <CopyButton text={created?.password ?? ''} message={t('message.copied')} label={t('label.copy')} />
          </div>
        </div>
      </Dialog>
    </div>
  )
}
