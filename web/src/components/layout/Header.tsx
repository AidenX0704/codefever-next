import { Suspense, lazy, useState } from 'react'
import { useNavigate } from 'react-router'

import { logoCommunity } from '@/assets/images'
import { HELP_LINKS } from '@/config/helpLinks'
import { publicApiDocUrl } from '@/config/publicApi'
import { plBell, plExit, plDraft, plHelp, plKey, plLocalization, plMemberAlt, plMerge, plRepository, psAddAlt, psCaretDown, psRepair, psRepositoryGroup } from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useI18n, useT } from '@/i18n'
import { Dropdown, type MenuItemSpec } from '@/components/unit/Dropdown'
import { Icon } from '@/components/unit/Icon'
import { Button } from '@/components/unit/Button'
import { SquareIconButton } from '@/components/unit/SquareIconButton'
// 通知抽屉只在点铃铛时用，懒加载以免把它的代码（含通知接口）带进首屏。
const NotificationDrawer = lazy(() => import('./NotificationDrawer').then((mod) => ({ default: mod.NotificationDrawer })))


/**
 * 页头。
 *
 * 结构逐项对齐旧版 `components/Header.js`：
 *   左：品牌 logo（高 32px、左 24px、右 48px）→ 仓库菜单 → 仓库组菜单 → 合并请求按钮
 *   右：新建菜单（＋）→ 语言 → 帮助 → 管理后台（仅管理员）→ 用户菜单
 *
 * 与旧版的差异（记录在 `docs/FRONTEND_REWRITE_PLAN.md` §10 批 2）：
 *   - **未读通知铃铛暂不渲染**：通知抽屉是批 5 的内容，提前放一个无目标的按钮
 *     会变成死交互；`unReadNotification` 数据已经在 `useAppData()` 里备好。
 *   - 滚动阴影（旧版 `withScrollTrigger` 的 elevation 0→2）暂缺省：
 *     该效果依赖 `.app-content` 滚动监听，批 3 随仓库页布局一起补。
 */
export function Header() {
  const t = useT()
  const navigate = useNavigate()
  const { user, repositories, groups } = useAppData()
  const { lang, setLang } = useI18n()
  const [notificationOpen, setNotificationOpen] = useState(false)

  const repositoryMenuItems: MenuItemSpec[] = repositories.slice(0, 20).map((repository) => ({
    key: repository.id,
    label: `${repository.group.displayName}/${repository.displayName}`,
    icon: <Icon icon={plRepository} size={16} />,
    onClick: () => navigate(`/${repository.group.name}/${repository.name}`),
  }))

  const groupMenuItems: MenuItemSpec[] = groups.slice(0, 20).map((group) => ({
    key: group.id,
    label: group.displayName,
    icon: <Icon icon={psRepositoryGroup} size={16} />,
    onClick: () => navigate(`/groups/${group.name}`),
  }))

  const createItems: MenuItemSpec[] = [
    { key: 'new-repository', label: t('label.newRepository'), onClick: () => navigate('/repositories/new') },
    { key: 'new-group', label: t('label.newGroup'), onClick: () => navigate('/groups/new') },
    { key: 'new-merge-request', label: t('label.createMergeRequest'), onClick: () => navigate('/mergerequests/new') },
  ]

  // 帮助菜单：站内落点优先（帮助中心回答"去哪儿"，文档中心回答"怎么用"），
  // 站外链接只保留"求助 / 反馈 / 贡献"三件必须出站的事。
  // 与旧版差异：旧版四项全部跳站外，站内没有帮助页；地址口径从上游 PGYER/codefever
  // 改到本仓库（见 `config/helpLinks.ts` 的说明）。
  const helpItems: MenuItemSpec[] = [
    { key: 'help-center', label: t('help.center'), onClick: () => navigate('/help') },
    { key: 'docs', label: t('help.docs.title'), onClick: () => navigate(`/doc/${lang === 'en-us' ? 'en' : 'cn'}`) },
    { key: 'api-docs', label: t('help.api.title'), onClick: () => navigate(publicApiDocUrl(lang)), dividerAfter: true },
    { key: 'support', label: t('label.support'), onClick: () => window.open(HELP_LINKS.support, '_blank', 'noopener') },
    { key: 'feedback', label: t('label.feedback'), onClick: () => window.open(HELP_LINKS.feedback, '_blank', 'noopener') },
    { key: 'contribute', label: t('label.contribute'), onClick: () => window.open(HELP_LINKS.contribute, '_blank', 'noopener'), dividerAfter: true },
    { key: 'about', label: t('label.about'), onClick: () => navigate('/help#about') },
  ]

  const languageItems: MenuItemSpec[] = [
    { key: 'zh-cn', label: t('lang.chinese'), onClick: () => setLang('zh-cn') },
    { key: 'en-us', label: t('lang.english'), onClick: () => setLang('en-us') },
  ]

  const userItems: MenuItemSpec[] = [
    { key: 'profile', label: t('menu.profile'), icon: <Icon icon={plMemberAlt} size={16} />, onClick: () => navigate('/settings/profile') },
    { key: 'mail', label: t('menu.mail'), icon: <Icon icon={plDraft} size={16} />, onClick: () => navigate('/settings/email') },
    { key: 'sshkey', label: t('menu.SSHKey_pl'), icon: <Icon icon={plKey} size={16} />, onClick: () => navigate('/settings/sshkey') },
    { key: 'logout', label: t('menu.logout'), icon: <Icon icon={plExit} size={16} />, onClick: () => { window.location.href = '/user/logout' }, dividerAfter: false },
  ]

  return (
    <header className="app-header">
      <div className="app-header__left">
        <img className="app-header__logo" src={logoCommunity} alt="CodeFever" onClick={() => navigate('/repositories')} />
        <Dropdown
          trigger={
            <Button variant="text" startIcon={<Icon icon={plRepository} size={16} />}>
              {t('label.repository')}
            </Button>
          }
          items={repositoryMenuItems}
          minWidth={240}
        />
        <Dropdown
          trigger={
            <Button variant="text" startIcon={<Icon icon={psRepositoryGroup} size={16} />}>
              {t('label.group')}
            </Button>
          }
          items={groupMenuItems}
          minWidth={240}
        />
        <SquareIconButton icon={plMerge} label={t('label.mergeRequest')} onClick={() => navigate('/mergerequests')} />
      </div>

      <div className="app-header__right">
        <Dropdown trigger={<SquareIconButton icon={psAddAlt} label={t('label.create')} />} items={createItems} align="right" />
        <SquareIconButton
          icon={plBell}
          label={t('label.notification')}
          badge={user?.unReadNotification ?? 0}
          onClick={() => setNotificationOpen(true)}
        />
        <Dropdown trigger={<SquareIconButton icon={plLocalization} label={t('label.language')} />} items={languageItems} align="right" />
        <Dropdown trigger={<SquareIconButton icon={plHelp} label={t('label.help')} />} items={helpItems} align="right" />
        {user?.admin ? (
          <SquareIconButton icon={psRepair} label={t('label.adminArea')} onClick={() => navigate('/admin')} />
        ) : null}
        <Dropdown
          align="right"
          minWidth={240}
          items={userItems}
          trigger={
            <span className="app-header__user">
              {user?.icon ? <img className="app-header__avatar" src={`/file/fetch/avatar/${user.icon}`} alt="" /> : <span className="app-header__avatar" />}
              <Icon icon={psCaretDown} size={12} />
            </span>
          }
        />
      </div>
      {notificationOpen ? (
        <Suspense fallback={null}>
          <NotificationDrawer open onClose={() => setNotificationOpen(false)} />
        </Suspense>
      ) : null}
    </header>
  )
}
