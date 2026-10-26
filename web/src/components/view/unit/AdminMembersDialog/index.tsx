import { useEffect, useState, type ReactNode } from 'react'

import { ApiError } from '@/api/client'
import type { AdminMember } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { useConfirm } from '@/components/unit/Confirm'
import { DataTable, type Column } from '@/components/unit/Table'
import { Dialog } from '@/components/unit/Dialog'
import { FormattedTime } from '@/components/unit/FormattedTime'
import { Marker } from '@/components/unit/Marker'
import { Tag } from '@/components/unit/Tag'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { Role } from '@/config/uac'
import { useT } from '@/i18n'


type Translate = ReturnType<typeof useT>

/** 成员操作失败时的兜底文案（按调用场景区分）。 */
export type AdminMemberErrorKey =
  | 'message.error.AddFail'
  | 'message.error.removeFail'
  | 'message.error.updateFail'

/**
 * 成员操作失败的本地化文案。
 *
 * 错误码口径见 `backend/internal/contract/errorcode.go`：
 *   - `0x0407` = 邮箱查无此人（`CodeUserNotFound`）；
 *   - `0x0408` = 不能把创建者加为成员（`CodeCannotAddCreator`）。
 * 其余（多为 `0x0405` 操作失败）交给调用方指定的兜底文案。
 */
export function adminMemberErrorText(error: unknown, t: Translate, fallbackKey: AdminMemberErrorKey): string {
  const code = error instanceof ApiError ? error.code : 0
  if (code === 0x0407) {
    return t('message.error._S_notFound', { s: t('label.email') })
  }
  if (code === 0x0408) {
    return t('message.error.canNotAddCreatorAsMember')
  }
  return t(fallbackKey)
}

export interface AdminMembersDialogProps {
  open: boolean
  /** 弹窗标题（默认"成员管理"）。 */
  title?: ReactNode
  /** 成员列表（`admin/groupMembers` / `admin/repositoryMembers`）。 */
  members: AdminMember[]
  loading?: boolean
  /**
   * 是否展示"组继承成员"标志（`groupMember`）。
   *
   * 只有**仓库**成员表能区分「组继承」与「仓库直授」（旧版
   * `components/view/admin/Repositories.js:225-228`），仓库组成员表没有这一列；
   * 该标志同时决定组继承成员是否可移除（组继承的权限归组管，仓库侧不可删）。
   */
  showGroupMember?: boolean
  /** 是否提供"设为所有者"（仅仓库组有 `admin/setGroupOwner` 端点）。 */
  allowSetOwner?: boolean
  onClose: () => void
  /** 按邮箱邀请成员（`admin/{group,repository}AddMember`）。 */
  onInvite: (email: string) => Promise<void>
  /** 移除成员（`admin/{group,repository}RemoveMember`）。 */
  onRemove: (member: AdminMember) => Promise<void>
  /** 设为所有者（`admin/setGroupOwner`）。 */
  onSetOwner?: (member: AdminMember) => Promise<void>
}

/** 角色整数 → 角色文案键（与旧版 `config/UAC.js` 的角色枚举一致）。 */
const ROLE_LABEL_KEYS: Partial<Record<number, string>> = {
  [Role.GUEST]: 'label.roleID_1',
  [Role.REPORTER]: 'label.roleID_2',
  [Role.DEVELOPER]: 'label.roleID_3',
  [Role.MAINTAINER]: 'label.roleID_4',
  [Role.OWNER]: 'label.roleID_5',
}

/**
 * 管理后台 · 成员管理弹窗（仓库组与仓库共用）。
 *
 * 旧版两处结构几乎相同（`admin/Groups.js:203-247` 与 `admin/Repositories.js:203-244`），
 * 差别只有两点：仓库多了「组继承成员」标志，仓库组多了「设为所有者」操作。
 * 因此这里抽成一个组件，两个页面各自注入端点与文案（AGENTS §4）。
 *
 * 行内规则逐字对齐旧版：
 *   - 已移除（`deleteFlag`）→ 只显示"已删除"，不给操作；
 *   - 创建者（`creatorFlag`）→ 不给移除/设所有者；
 *   - 角色为所有者（`role >= UAC.Role.OWNER`）→ 不给移除/设所有者；
 *   - 仓库侧组继承成员（`groupMember`）→ 不给移除。
 */
export function AdminMembersDialog({
  open,
  title,
  members,
  loading = false,
  showGroupMember = false,
  allowSetOwner = false,
  onClose,
  onInvite,
  onRemove,
  onSetOwner,
}: AdminMembersDialogProps) {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()

  const [email, setEmail] = useState('')
  const [inviting, setInviting] = useState(false)

  useEffect(() => {
    if (!open) {
      setEmail('')
    }
  }, [open])

  const reportError = (error: unknown) => {
    toast.error(error instanceof Error ? error.message : t('base_error_message'))
  }

  const invite = async () => {
    const value = email.trim()
    if (!value) {
      toast.error(t('message.error._S_empty', { s: t('label.email') }))
      return
    }
    setInviting(true)
    try {
      await onInvite(value)
      setEmail('')
    } catch (error) {
      reportError(error)
    } finally {
      setInviting(false)
    }
  }

  const remove = async (member: AdminMember) => {
    const ok = await confirm({
      title: t('message.memberRemoveConfirm'),
      message: t('message.confirmDelete', { s: `'${member.name}'` }),
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    try {
      await onRemove(member)
    } catch (error) {
      reportError(error)
    }
  }

  const setOwner = async (member: AdminMember) => {
    const ok = await confirm({
      title: t('message.setGroupOwner_S', { s: `'${member.name}'` }),
      message: member.email,
      tone: 'danger',
      confirmText: t('base_confirm'),
    })
    if (!ok) {
      return
    }
    try {
      await onSetOwner?.(member)
    } catch (error) {
      reportError(error)
    }
  }

  const columns: Column<AdminMember>[] = [
    {
      key: 'member',
      title: t('label.name'),
      render: (row) => (
        <span className="row">
          <Avatar size="sm" src={row.icon ? `/file/fetch/avatar/${row.icon}` : undefined} name={row.name} />
          <span>
            {row.name}
            <span className="muted"> · {row.email}</span>
          </span>
        </span>
      ),
    },
    {
      key: 'role',
      title: t('label.role'),
      render: (row) => (
        <Marker
          variant={row.role === Role.OWNER ? 'containedInfo' : 'info'}
          text={t(ROLE_LABEL_KEYS[row.role] ?? 'label.roleID_6')}
        />
      ),
    },
    {
      key: 'flags',
      title: '',
      render: (row) => (
        <span className="row">
          {row.creatorFlag ? <Tag tone="success">{t('label.creator')}</Tag> : null}
          {showGroupMember && row.groupMember ? (
            <Tag tone="success">
              {t('label.group')}
              {t('label.member')}
            </Tag>
          ) : null}
          {row.deleteFlag ? <Tag tone="danger">{t('message.deleted')}</Tag> : null}
        </span>
      ),
    },
    {
      key: 'joined',
      title: t('label.joinedAt'),
      render: (row) => <FormattedTime timestamp={row.joined} />,
    },
    {
      key: 'ops',
      title: '',
      render: (row) => {
        const removed = row.deleteFlag
        const canOperate = !removed && !row.creatorFlag && row.role < Role.OWNER
        const canRemove = canOperate && !(showGroupMember && row.groupMember)
        if (!canRemove && !(allowSetOwner && canOperate)) {
          return <span className="muted">-</span>
        }
        return (
          <span className="row">
            {allowSetOwner && canOperate ? (
              <Button size="sm" variant="text" onClick={() => setOwner(row)}>
                {t('message.roleToOwner')}
              </Button>
            ) : null}
            {canRemove ? (
              <Button size="sm" variant="text" tone="danger" onClick={() => remove(row)}>
                {t('label.delete')}
              </Button>
            ) : null}
          </span>
        )
      },
    },
  ]

  return (
    <Dialog
      open={open}
      title={title ?? t('label.memberManage')}
      onClose={onClose}
      actions={
        <Button variant="text" onClick={onClose}>
          {t('label.cancel')}
        </Button>
      }
    >
      <div className="stack">
        <div className="row">
          <TextField
            containerClassName="flex-grow"
            placeholder={t('message.plsInputInvitedEmail')}
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
          <Button variant="contained" loading={inviting} onClick={invite}>
            {t('label.invite')}
          </Button>
        </div>

        <DataTable
          columns={columns}
          rows={members}
          rowKey={(row) => row.id}
          loading={loading}
          emptyTitle={t('message._S_listEmpty', { s: t('label.member') })}
        />
      </div>
    </Dialog>
  )
}
