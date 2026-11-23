import { useState, type ReactNode } from 'react'

import type { RepositoryMember } from '@/api/types'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { Icon } from '@/components/unit/Icon'
import { Marker } from '@/components/unit/Marker'
import { SelectField } from '@/components/unit/SelectField'
import { TextField } from '@/components/unit/TextField'
import { plSearch } from '@/config/icons'
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
  /** 顶部提示信息（例如仓库组成员权限继承说明）。 */
  hint?: ReactNode
  /**
   * 分两表展示（**仓库成员** / **仓库组成员**）。
   *
   * 组成员来自仓库组继承，在仓库页**不可移除**（只能改角色）。
   */
  splitGroupMembers?: boolean
}

export function MembersManager({
  members,
  canManage,
  onAdd,
  onChangeRole,
  onRemove,
  onTransfer,
  hint,
  splitGroupMembers = false,
}: MembersManagerProps) {
  const t = useT()
  const [email, setEmail] = useState('')
  const [keyword, setKeyword] = useState('')
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

  // 已删除成员（deleteFlag）不展示。
  const alive = members.filter((member) => !member.deleteFlag)
  const kw = keyword.trim().toLowerCase()
  const filtered = kw
    ? alive.filter(
        (member) =>
          member.name.toLowerCase().includes(kw) || member.email.toLowerCase().includes(kw),
      )
    : alive
  const sections = splitGroupMembers
    ? [
        { title: t('label.repositoryMember'), items: filtered.filter((member) => !member.groupMember), removable: true },
        { title: t('label.groupMember'), items: filtered.filter((member) => member.groupMember), removable: false },
      ]
    : [{ title: '', items: filtered, removable: true }]

  return (
    <div className="page">
      {hint ? <div className="form-banner">{hint}</div> : null}

      <div className="members-toolbar">
        {canManage ? (
          <div className="row">
            <TextField
              placeholder={t('label.email')}
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  void submitAdd()
                }
              }}
            />
            <Button loading={adding} onClick={submitAdd}>
              {t('label.addMember')}
            </Button>
          </div>
        ) : null}

        {alive.length > 0 ? (
          <TextField
            containerClassName="cf-filter-bar__search"
            placeholder={t('group.searchMembers')}
            value={keyword}
            startAdornment={<Icon icon={plSearch} size={16} />}
            onChange={(event) => setKeyword(event.target.value)}
          />
        ) : null}
      </div>

      {sections.map((section) =>
        section.title ? (
          <div className="section" key={section.title}>
            <div className="section__title">{section.title}</div>
            <div className="list-sheet">
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
          <div className="list-sheet" key="members">
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
