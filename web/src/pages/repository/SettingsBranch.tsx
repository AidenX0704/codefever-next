import { useCallback, useEffect, useState } from 'react'

import {
  changeDefaultBranch,
  createProtectedBranchRule,
  deleteProtectedBranchRule,
  fetchProtectedBranchRules,
  updateProtectedBranchRule,
} from '@/api/repository'
import type { ProtectedBranchRule } from '@/api/types'
import { Button } from '@/components/unit/Button'
import { useConfirm } from '@/components/unit/Confirm'
import { DataTable } from '@/components/unit/Table'
import { Dialog } from '@/components/unit/Dialog'
import { Form, FormGrid, FormItem } from '@/components/unit/Form'
import { SelectField } from '@/components/unit/SelectField'
import { TextField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { Role } from '@/config/uac'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useRepository } from '@/features/repository/RepositoryProvider'
import { useT } from '@/i18n'


interface RuleState {
  id: string
  rule: string
  rolePush: number
  roleMerge: number
}

/**
 * 「允许角色」是**集合**语义：最低角色及其以上的所有角色都能操作。
 *
 * 逐字对齐旧版 `RepositorySettingBranch.getAllowRoler`：
 *   roleIds = [不允许任何人, 所有者, 维护者, 开发者]
 *   slice(roleId === NOBODY ? 0 : 1, NOBODY - roleId + 1)
 */
function allowRoleLabel(roleId: number, t: (key: string) => string): string {
  const names: string[] = []
  if (roleId === Role.NOBODY) {
    return t('message.noBodyAllow')
  }
  if (roleId <= Role.OWNER) {
    names.push(t('label.owner'))
  }
  if (roleId <= Role.MAINTAINER) {
    names.push(t('label.maintianer'))
  }
  if (roleId <= Role.DEVELOPER) {
    names.push(t('label.developer'))
  }
  return names.join(' + ')
}

/**
 * 仓库设置 · 分支（`/{组}/{仓库}/settings/branch`）。
 *
 * 两块内容：
 *   1. 默认分支：`repository/defaultBranch`（body `repository` + `old` + `branch`，后端要求 old !== branch）；
 *   2. 保护分支规则：`protectedBranchRules`（GET）/ `createProtectedBranchRule` /
 *      `updateProtectedBranchRule` / `deleteProtectedBranchRule`。
 *      角色取值受后端白名单限制（push：NOBODY/OWNER/MAINTAINER/DEVELOPER；
 *      merge：NOBODY/OWNER/MAINTAINER —— 后端 `ProtectedBranchMergeRoleAllowed`
 *      只认这三个），因此下拉只提供后端接受的选项，避免"前端能选、后端报 0x0201"。
 */
export default function SettingsBranch() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { summary, config, refresh } = useRepository()
  const { refresh: refreshAppData } = useAppData()

  const [rules, setRules] = useState<ProtectedBranchRule[]>([])
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<RuleState | null>(null)
  const [saving, setSaving] = useState(false)
  const [defaultBranch, setDefaultBranch] = useState('')

  const reload = useCallback(() => {
    if (!summary) {
      return
    }
    setLoading(true)
    fetchProtectedBranchRules(summary.id)
      .then(setRules)
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => setLoading(false))
  }, [summary, t, toast])

  useEffect(() => {
    reload()
  }, [reload])

  useEffect(() => {
    setDefaultBranch(config?.repository.defaultBranch ?? '')
  }, [config])

  // push 的后端白名单：NOBODY / OWNER / MAINTAINER / DEVELOPER。
  const pushRoleOptions = [
    { value: String(Role.NOBODY), label: t('message.noBodyAllow') },
    { value: String(Role.OWNER), label: allowRoleLabel(Role.OWNER, t) },
    { value: String(Role.MAINTAINER), label: allowRoleLabel(Role.MAINTAINER, t) },
    { value: String(Role.DEVELOPER), label: allowRoleLabel(Role.DEVELOPER, t) },
  ]

  // merge 的后端白名单只有 NOBODY / OWNER / MAINTAINER
  // （`domain.ProtectedBranchMergeRoleAllowed`），多给一个选项就会 0x0201。
  const mergeRoleOptions = [
    { value: String(Role.NOBODY), label: t('message.noBodyAllow') },
    { value: String(Role.OWNER), label: allowRoleLabel(Role.OWNER, t) },
    { value: String(Role.MAINTAINER), label: allowRoleLabel(Role.MAINTAINER, t) },
  ]

  const saveDefaultBranch = async (branch: string) => {
    if (!summary) {
      return
    }
    const old = config?.repository.defaultBranch ?? ''
    if (branch === old) {
      return
    }
    try {
      await changeDefaultBranch(summary.id, old, branch)
      toast.success(t('message.updateSuccess'))
      refresh()
      refreshAppData()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  const saveRule = async () => {
    if (!summary || !editing) {
      return
    }
    if (!editing.rule.trim()) {
      toast.error(t('label.createProtectedBranchRule'))
      return
    }
    setSaving(true)
    try {
      if (editing.id) {
        await updateProtectedBranchRule(summary.id, editing.id, editing.rule.trim(), editing.rolePush, editing.roleMerge)
      } else {
        await createProtectedBranchRule(summary.id, editing.rule.trim(), editing.rolePush, editing.roleMerge)
      }
      toast.success(t('message.updateSuccess'))
      setEditing(null)
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    } finally {
      setSaving(false)
    }
  }

  const removeRule = async (rule: ProtectedBranchRule) => {
    if (!summary) {
      return
    }
    const ok = await confirm({ title: t('label.delete'), message: rule.rule, tone: 'danger', confirmText: t('base_confirm') })
    if (!ok) {
      return
    }
    try {
      await deleteProtectedBranchRule(summary.id, rule.id, rule.rule)
      toast.success(t('message.deleteSuccess'))
      reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('base_error_message'))
    }
  }

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.defaultBranch')}</div>
        <SelectField
          options={(config?.branches ?? []).map((branch) => ({ value: branch.name, label: branch.name }))}
          value={defaultBranch}
          onChange={(event) => {
            setDefaultBranch(event.target.value)
            saveDefaultBranch(event.target.value)
          }}
        />
      </section>

      <section className="section">
        <div className="section__title">{t('label.protectedBranch')}</div>
        <div className="row">
          <Button variant="contained" onClick={() => setEditing({ id: '', rule: '', rolePush: Role.MAINTAINER, roleMerge: Role.MAINTAINER })}>
            {t('label.createProtectedBranchRule')}
          </Button>
        </div>
        <DataTable
          columns={[
            { key: 'rule', title: t('label.branch_pl') },
            {
              key: 'rolePush',
              title: t('label.allowPush'),
              render: (row) => allowRoleLabel(row.rolePush, t),
            },
            {
              key: 'roleMerge',
              title: t('label.allowMerge'),
              render: (row) => allowRoleLabel(row.roleMerge, t),
            },
            {
              key: 'ops',
              title: '',
              render: (row) => (
                <span className="row">
                  <Button size="sm" variant="text" onClick={() => setEditing({ ...row })}>
                    {t('label.edit')}
                  </Button>
                  <Button size="sm" variant="text" tone="danger" onClick={() => removeRule(row)}>
                    {t('label.delete')}
                  </Button>
                </span>
              ),
            },
          ]}
          rows={rules}
          rowKey={(row) => row.id}
          loading={loading}
          emptyTitle={t('message._S_listEmpty', { s: t('label.protectedBranch') })}
        />
      </section>

      <Dialog
        open={editing !== null}
        title={editing?.id ? t('label.changeProtectedBranchRule') : t('label.createProtectedBranchRule')}
        onClose={() => setEditing(null)}
        actions={
          <>
            <Button variant="text" onClick={() => setEditing(null)}>
              {t('base_confirm')}
            </Button>
            <Button variant="contained" loading={saving} onClick={saveRule}>
              {t('label.save')}
            </Button>
          </>
        }
      >
        <Form>
          <FormItem label={t('label.branch_pl')} required help="main、release/*">
            <TextField
              value={editing?.rule ?? ''}
              onChange={(event) => setEditing((current) => (current ? { ...current, rule: event.target.value } : current))}
            />
          </FormItem>
          <FormGrid>
            <FormItem label={t('label.allowPush')} required>
              <SelectField
                options={pushRoleOptions}
                value={String(editing?.rolePush ?? Role.MAINTAINER)}
                onChange={(event) =>
                  setEditing((current) => (current ? { ...current, rolePush: Number(event.target.value) } : current))
                }
              />
            </FormItem>
            <FormItem label={t('label.allowMerge')} required>
              <SelectField
                options={mergeRoleOptions}
                value={String(editing?.roleMerge ?? Role.MAINTAINER)}
                onChange={(event) =>
                  setEditing((current) => (current ? { ...current, roleMerge: Number(event.target.value) } : current))
                }
              />
            </FormItem>
          </FormGrid>
        </Form>
      </Dialog>
    </div>
  )
}
