import { useState } from 'react'
import { useNavigate } from 'react-router'

import type { CurrentUser, RepositoryConfig, RepositoryListItem } from '@/api/types'
import { AppLink } from '@/components/unit/AppLink'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { CopyButton } from '@/components/unit/CopyButton'
import { Icon } from '@/components/unit/Icon'
import { Tabs } from '@/components/unit/Tabs'
import { Tag } from '@/components/unit/Tag'
import { TextField } from '@/components/unit/TextField'
import { plFork, plMerge, psBranch, psCommit, psFile, psMembers, psTag } from '@/config/icons'
import { cloneUrls } from '@/helpers/cloneUrl'
import { formatBytes } from '@/helpers/format'
import { useT } from '@/i18n'


export interface RepositoryHeaderProps {
  /** `repository/config` 的完整数据。 */
  config: RepositoryConfig
  /** 当前登录用户（取 host / ssh 拼克隆地址）。 */
  user: CurrentUser | null
  /** fork 来源仓库（全局仓库列表里按 `forkFrom` 找到的那一条）。 */
  forkSource?: RepositoryListItem | null
}

type CloneTab = 'https' | 'ssh'

/**
 * 仓库页头部（对应旧版 `components/unit/RepositoryDashboard.js`）。
 *
 * 三块内容：
 *   1. 身份区：头像 + 组名/显示名 + 描述 + **fork 来源链接**；
 *   2. 操作区：**Fork 按钮 + fork 数**、**克隆地址（HTTPS / SSH + 复制）**；
 *   3. 计数条：提交 / 分支 / 标签 / **仓库体积** / **未合并 MR** / 成员数。
 *
 * 数据全部来自 `repository/config`（fork 数与未合并 MR 数在后端已随 config 下发，
 * 见 `backend/internal/handlers/repository.go` 的 config 处理）。
 */
export function RepositoryHeader({ config, user, forkSource }: RepositoryHeaderProps) {
  const t = useT()
  const navigate = useNavigate()
  const [cloneOpen, setCloneOpen] = useState(false)
  const [cloneTab, setCloneTab] = useState<CloneTab>('https')

  // 克隆地址的拼装口径已上收到 `helpers/cloneUrl`（首页也要用同一份），这里只选协议。
  const urls = cloneUrls(user, config.group.name, config.repository.name)
  const cloneUrl = cloneTab === 'https' ? urls.https : urls.ssh
  return (
    <div className="repo-head">
      <div className="repo-head__top">
        <Avatar
          square
          size="lg"
          src={config.repository.icon ? `/file/fetch/avatar/${config.repository.icon}` : undefined}
          name={config.repository.name}
        />
        <div className="repo-head__identity">
          <div className="repo-head__title">
            <span className="repo-head__group">{config.group.displayName}/</span>
            {config.repository.displayName}
            {forkSource ? (
              <span className="repo-head__fork">
                <AppLink to={`/${forkSource.group.name}/${forkSource.name}`}>
                  {t('label.forkFrom_S', { s: `${forkSource.group.displayName}/${forkSource.displayName}` })}
                </AppLink>
              </span>
            ) : null}
          </div>
          {config.repository.description ? <div className="repo-head__desc">{config.repository.description}</div> : null}
        </div>

        <div className="repo-head__actions">
          <Button
            variant="outlined"
            startIcon={<Icon icon={plFork} size={16} />}
            onClick={() => navigate(`/repositories/fork/${config.repository.id}`)}
          >
            {t('label.fork')}
          </Button>
          <Tag tone="primary" title={t('label.fork')}>
            {config.repository.forkCount}
          </Tag>
          <Button variant="contained" onClick={() => setCloneOpen((value) => !value)}>
            {t('label.clone')}
          </Button>
        </div>
      </div>

      {cloneOpen ? (
        <div className="repo-clone">
          <Tabs
            items={[
              { key: 'https', label: t('label.https') },
              { key: 'ssh', label: t('label.ssh') },
            ]}
            value={cloneTab}
            onChange={setCloneTab}
          />
          <p className="repo-clone__hint">
            {cloneTab === 'https' ? t('message.differenceBetweenHTTPSAndSSH') : t('message.generateAndSetSSHKey')}
          </p>
          <div className="repo-clone__row">
            <TextField containerClassName="repo-clone__field" readOnly value={cloneUrl} />
            <CopyButton text={cloneUrl} label={t('label.copy')} />
          </div>
        </div>
      ) : null}

      <div className="repo-head__stats">
        <Stat icon={psCommit} label={t('label.commit')} value={config.count.commit} />
        <Stat icon={psBranch} label={t('label.branch')} value={config.count.branch} />
        <Stat icon={psTag} label={t('label.tag')} value={config.count.tag} />
        <Stat icon={psFile} label={t('label.repoSize')} value={formatBytes(config.count.file)} />
        <Stat icon={plMerge} label={t('label.mergeRequestCount')} value={config.repository.mergeRequestCount.open} />
        <Stat icon={psMembers} label={t('label.memberCount')} value={config.members.length} />
      </div>
    </div>
  )
}

function Stat({ icon, label, value }: { icon: Parameters<typeof Icon>[0]['icon']; label: string; value: number | string }) {
  return (
    <span className="repo-stat">
      <Icon icon={icon} size={16} />
      <span className="repo-stat__value">{value}</span>
      <span className="repo-stat__label">{label}</span>
    </span>
  )
}
