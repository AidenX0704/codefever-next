import type { CurrentUser, RepositoryListItem } from '@/api/types'
import { CopyButton } from '@/components/unit/CopyButton'
import { cloneUrls } from '@/helpers/cloneUrl'
import { useT } from '@/i18n'


export interface HomeHeroProps {
  user: CurrentUser | null
  /** 主推仓库（"我的仓库"里 `updated` 最新的那个），用于给出可复制的 clone 命令。 */
  repository: RepositoryListItem | null
}

/** 按本地时间给问候语。 */
function greetingKey(hour: number): string {
  if (hour < 6) {
    return 'home.greetingNight'
  }
  if (hour < 12) {
    return 'home.greetingMorning'
  }
  if (hour < 18) {
    return 'home.greetingAfternoon'
  }
  return 'home.greetingEvening'
}

/**
 * 首页顶部（v5 DESIGN.md §6 G1 页头）。
 *
 * v5 口径：页头是**纯文字**（问候 24/600 + 副标题 fg-3），无色块、无按钮 ——
 * 新建动作在顶栏「＋新建」里。偏离注记（AGENTS §7）：右侧保留"主推仓库
 * clone 命令"等宽小块（HOME_PAGE_PLAN §2.5 的高频功能），材质用 --bg-well。
 */
export function HomeHero({ user, repository }: HomeHeroProps) {
  const t = useT()

  const name = user?.name ?? ''
  const urls = repository ? cloneUrls(user, repository.group.name, repository.name) : null
  const cloneText = urls?.ssh || urls?.https || ''
  const repositoryLabel = repository ? `${repository.group.displayName}/${repository.displayName}` : ''

  return (
    <section className="home-hero">
      <div className="home-hero__main">
        <h1 className="home-hero__greeting">
          {t(greetingKey(new Date().getHours()), { s: name })}
        </h1>
        <p className="home-hero__tagline">{t('home.tagline')}</p>
      </div>

      {repository && cloneText ? (
        <div className="home-hero__clone">
          <div className="home-hero__clone-label">
            {t('home.cloneLabel')} · <span className="home-hero__clone-repo">{repositoryLabel}</span>
          </div>
          <div className="home-hero__clone-row">
            <code className="home-hero__clone-code">git clone {cloneText}</code>
            <CopyButton text={`git clone ${cloneText}`} label={t('label.copy')} />
          </div>
        </div>
      ) : null}
    </section>
  )
}
