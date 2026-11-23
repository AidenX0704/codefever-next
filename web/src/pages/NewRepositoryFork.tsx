import { useMemo } from 'react'
import { useNavigate, useParams } from 'react-router'

import { forkRepository } from '@/api/repository'
import { Icon } from '@/components/unit/Icon'
import { useToast } from '@/components/unit/Toast'
import { RepositoryForm } from '@/components/view/unit/RepositoryForm'
import { arrowLeftIcon } from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/**
 * Fork 仓库（`/repositories/fork/{源仓库主键}`）。
 *
 * 复用 `RepositoryForm`，只注入源仓库信息与 `forkRepository` 写端点（AGENTS §4/§6）。
 */
export default function NewRepositoryFork() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { repositoryId } = useParams()
  const { repositories, refresh } = useAppData()

  const source = useMemo(() => repositories.find((item) => item.id === repositoryId), [repositories, repositoryId])

  return (
    <div className="page">
      <div className="studio-header">
        <button
          type="button"
          className="group-rail__back"
          onClick={() => navigate(source ? `/${source.group.name}/${source.name}` : '/repositories')}
        >
          <Icon icon={arrowLeftIcon} size={14} />
          <span>{source ? `${source.group.displayName} / ${source.displayName}` : t('label.repository')}</span>
        </button>
        <h1 className="page__title">
          {t('label.forkFrom_S', { s: source ? `${source.group.displayName}/${source.displayName}` : '' })}
        </h1>
        <p className="page__subtitle">{t('repo.forkSubtitle')}</p>
      </div>

      <RepositoryForm
        mode="fork"
        sourceRepository={source}
        onSubmit={async (values) => {
          if (!source) {
            return
          }
          const repository = await forkRepository({
            groupKey: values.groupKey,
            name: values.name,
            displayName: values.displayName,
            description: values.description,
            forkID: source.id,
          })
          toast.success(t('message.addSuccess'))
          await refresh()
          navigate(`/${repository.group.name}/${repository.name}`)
        }}
      />
    </div>
  )
}
