import { useNavigate, useParams } from 'react-router'

import { createRepository } from '@/api/repository'
import { Icon } from '@/components/unit/Icon'
import { useToast } from '@/components/unit/Toast'
import { RepositoryForm } from '@/components/view/unit/RepositoryForm'
import { arrowLeftIcon } from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useT } from '@/i18n'


/**
 * 新建仓库（`/repositories/new`、组内 `/groups/{组名}/repositories/new`）。
 *
 * 页面只负责标题、业务动作与创建后跳转；表单与校验全部收敛在 `RepositoryForm`（AGENTS §4/§6）。
 */
export default function NewRepository() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const { groupName } = useParams()
  const { refresh } = useAppData()

  const backTo = groupName ? `/groups/${groupName}/repositories` : '/repositories'

  return (
    <div className="page">
      <div className="studio-header">
        <button type="button" className="group-rail__back" onClick={() => navigate(backTo)}>
          <Icon icon={arrowLeftIcon} size={14} />
          <span>{t('label.repository')}</span>
        </button>
        <h1 className="page__title">{t('label.createRepository')}</h1>
        <p className="page__subtitle">{t('repo.createSubtitle')}</p>
      </div>

      <RepositoryForm
        mode="create"
        initialGroupName={groupName}
        onSubmit={async (values) => {
          const repository = await createRepository({
            groupKey: values.groupKey,
            name: values.name,
            displayName: values.displayName,
            description: values.description,
            initReadme: values.initReadme,
            defaultBranch: values.defaultBranch,
            gitignore: values.gitignore,
          })
          toast.success(t('message.addSuccess'))
          await refresh()
          navigate(`/${repository.group.name}/${repository.name}`)
        }}
      />
    </div>
  )
}
