import { CopyButton } from '@/components/unit/CopyButton'
import { useT } from '@/i18n'


export interface RepositoryEmptyProps {
  /** 仓库名（生成 README 首行）。 */
  repositoryName: string
  /** 组名（拼 remote 地址）。 */
  groupName: string
  /** 站点 host（如 `https://codefever.example.com`）。 */
  host: string
}

/**
 * 空仓库引导（对应旧版 `components/unit/RepositoryEmpty.js`）。
 *
 * 两段命令：**新建一个仓库并推上去** / **把已有仓库推上来**，各带一个复制按钮。
 */
export function RepositoryEmpty({ repositoryName, groupName, host }: RepositoryEmptyProps) {
  const t = useT()

  const remoteCode = `git remote add origin ${host}/${groupName}/${repositoryName}.git`
  const createCode = [
    `echo "# ${repositoryName}" >> README.md`,
    'git init',
    'git add README.md',
    'git commit -m "first commit"',
    'git branch -M main',
    remoteCode,
    'git push -u origin main',
  ]
  const pushCode = [remoteCode, 'git branch -M main', 'git push -u origin main']

  return (
    <div className="stack">
      <div className="repo-empty">
        <div className="repo-empty__title">{t('message.createNewRepository')}</div>
        <div className="repo-empty__code">
          <span className="repo-empty__copy">
            <CopyButton text={createCode.join('\n')} label={t('label.copy')} />
          </span>
          {createCode.map((line) => (
            <div className="repo-empty__line" key={line}>
              <code>{line}</code>
            </div>
          ))}
        </div>
      </div>

      <div className="repo-empty repo-empty--flush">
        <div className="repo-empty__title">{t('message.pushRepository')}</div>
        <div className="repo-empty__code">
          <span className="repo-empty__copy">
            <CopyButton text={pushCode.join('\n')} label={t('label.copy')} />
          </span>
          {pushCode.map((line) => (
            <div className="repo-empty__line" key={line}>
              <code>{line}</code>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
