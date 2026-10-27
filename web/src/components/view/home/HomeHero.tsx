import { useNavigate } from 'react-router'

import type { CurrentUser, RepositoryListItem } from '@/api/types'
import { Button } from '@/components/unit/Button'
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
 * 首页顶部 HERO。
 *
 * 形态约束（docs/HOME_PAGE_PLAN_2026-10-27.md §2.5）：通栏 `--cf-color-primary-lighter`
 * 色块，**无图、无渐变、无动效** —— 全页唯一的有色块，克制。
 *
 * 内容只做三件事：打招呼、说清"这是哪儿"、给三个最高频动作；
 * 右侧把"主推仓库的 clone 命令"直给（等宽字体），新人第一件事就是 clone。
 */
export function HomeHero({ user, repository }: HomeHeroProps) {
  const t = useT()
  const navigate = useNavigate()

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
        <div className="home-hero__actions">
          <Button variant="contained" onClick={() => navigate('/repositories/new')}>
            {t('label.newRepository')}
          </Button>
          <Button variant="outlined" onClick={() => navigate('/groups/new')}>
            {t('label.newGroup')}
          </Button>
          <Button variant="outlined" onClick={() => navigate('/mergerequests/new')}>
            {t('label.createMergeRequest')}
          </Button>
        </div>
      </div>

      <div className="home-hero__clone">
        {repository && cloneText ? (
          <>
            <div className="home-hero__clone-label">
              {t('home.cloneLabel')} · <span className="home-hero__clone-repo">{repositoryLabel}</span>
            </div>
            <div className="home-hero__clone-row">
              <code className="home-hero__clone-code">git clone {cloneText}</code>
              <CopyButton text={`git clone ${cloneText}`} label={t('label.copy')} />
            </div>
          </>
        ) : (
          <div className="home-hero__clone-label">{t('home.cloneEmpty')}</div>
        )}
      </div>
    </section>
  )
}
