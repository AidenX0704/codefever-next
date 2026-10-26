import { useState } from 'react'

import type { RepositoryMember } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { Marker } from '@/components/unit/Marker'
import { SelectField } from '@/components/unit/SelectField'
import { TextField } from '@/components/unit/TextField'
import { Role } from '@/config/uac'
import { useT } from '@/i18n'


export interface MembersManagerProps {
  members: RepositoryMember[]
  /** 当前用户是否有管理权限（决定操作列与新增表单是否渲染）。 */
  canManage: boolean
  onAdd: (email: string) => Promise<void>
  onChangeRole: (userID: string, roleID: number) => Promise<void>
  onRemove: (userID: string, name: string) => Promise<void>
  onTransfer: (userID: string, name: string) => Promise<void>
  /**
   * 分两表展示（**仓库成员** / **仓库组成员**）。
   *
   * 旧版 `MemberList.js` 就是两张表；组成员来自仓库组继承，
   * 在仓库页**不可移除**（只能改角色），与旧版 `removeable` 语义一致。
   */
  splitGroupMembers?: boolean
}

/**
 * 成员列表管理器（仓库成员与仓库组成员共用）。
 *
 * AGENTS §4：同一套"列表 + 角色下拉 + 移除 + 转移所有权"在仓库与组两个页面都要出现，
 * 因此抽成一个组件；页面只负责注入各自的 API 调用与权限判定。
 * 视觉与 `ref-row` 行一致（头像 + 名称邮箱 + 角色徽标 + 操作区）。
 */
export function MembersManager({
  members,
  canManage,
  onAdd,
  onChangeRole,
  onRemove,
  onTransfer,
  splitGroupMembers = false,
}: MembersManagerProps) {
  const t = useT()
  const [email, setEmail] = useState('')
  const [adding, setAdding] = useState(false)

  const submitAdd = async () => {
    if (!email.trim()) {
      return
    }
    setAdding(true)
    try {
      await onAdd(email.trim())
      setEmail('')
    } finally {
      setAdding(false)
    }
  }

  // 已删除成员（deleteFlag）不展示 —— 旧版 `FilterGenerator.notDeleted()`。
  const alive = members.filter((member) => !member.deleteFlag)
  const sections = splitGroupMembers
    ? [
        { title: t('label.repositoryMember'), items: alive.filter((member) => !member.groupMember), removable: true },
        { title: t('label.groupMember'), items: alive.filter((member) => member.groupMember), removable: false },
      ]
    : [{ title: '', items: alive, removable: true }]

  return (
    <div className="page">
      {canManage ? (
        <div className="row">
          <TextField placeholder={t('label.email')} value={email} onChange={(event) => setEmail(event.target.value)} />
          <Button loading={adding} onClick={submitAdd}>
            {t('label.addMember')}
          </Button>
        </div>
      ) : null}

      {sections.map((section) =>
        section.title ? (
          <div className="section" key={section.title}>
            <div className="section__title">{section.title}</div>
            <div className="list-body">
              {section.items.map((member) => (
                <MemberRow
                  key={member.id}
                  member={member}
                  canManage={canManage}
                  removable={section.removable}
                  onChangeRole={onChangeRole}
                  onRemove={onRemove}
                  onTransfer={onTransfer}
                />
              ))}
            </div>
          </div>
        ) : (
          <div className="list-body" key="members">
            {section.items.map((member) => (
              <MemberRow
                key={member.id}
                member={member}
                canManage={canManage}
                removable={section.removable}
                onChangeRole={onChangeRole}
                onRemove={onRemove}
                onTransfer={onTransfer}
              />
            ))}
          </div>
        ),
      )}
    </div>
  )
}

function MemberRow({
  member,
  canManage,
  removable,
  onChangeRole,
  onRemove,
  onTransfer,
}: {
  member: RepositoryMember
  canManage: boolean
  removable: boolean
  onChangeRole: (userID: string, roleID: number) => Promise<void>
  onRemove: (userID: string, name: string) => Promise<void>
  onTransfer: (userID: string, name: string) => Promise<void>
}) {
  const t = useT()

  const roleOptions = [
    { value: String(Role.GUEST), label: t('label.roleID_1') },
    { value: String(Role.REPORTER), label: t('label.roleID_2') },
    { value: String(Role.DEVELOPER), label: t('label.roleID_3') },
    { value: String(Role.MAINTAINER), label: t('label.roleID_4') },
  ]

  return (
    <div className="ref-row">
      <Avatar size="sm" src={member.icon ? `/file/fetch/avatar/${member.icon}` : undefined} name={member.name} />
      <span className="ref-row__name">
        {member.name}
        <span className="muted"> · {member.email}</span>
      </span>
      <span className="ref-row__commit">
        <Marker variant={member.role === Role.OWNER ? 'containedInfo' : 'info'} text={t(`label.roleID_${member.role}`)} />
      </span>
      {canManage ? (
        <span className="row">
          {member.role !== Role.OWNER ? (
            <>
              <SelectField
                options={roleOptions}
                value={String(member.role)}
                onChange={(event) => onChangeRole(member.id, Number(event.target.value))}
                containerClassName="member-role-select"
              />
              <Button size="sm" variant="text" onClick={() => onTransfer(member.id, member.name)}>
                {t('label.changeOwner')}
              </Button>
              {removable ? (
                <Button size="sm" variant="text" tone="danger" onClick={() => onRemove(member.id, member.name)}>
                  {t('label.delete')}
                </Button>
              ) : null}
            </>
          ) : (
            <Marker variant="containedInfo" text={t('label.owner')} />
          )}
        </span>
      ) : null}
    </div>
  )
}
