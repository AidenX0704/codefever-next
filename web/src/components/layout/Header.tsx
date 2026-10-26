import { Suspense, lazy, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'

import { logoCommunity } from '@/assets/images'
import { avatarUrl } from '@/config/constants'
import { HELP_LINKS } from '@/config/helpLinks'
import { publicApiDocUrl } from '@/config/publicApi'
import {
  plBell,
  plCaretDownLine,
  plCheck,
  plDraft,
  plExit,
  plHelp,
  plKey,
  plLocalization,
  plMemberAlt,
  plMerge,
  psAddAlt,
  psRepair,
} from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { useI18n, useT } from '@/i18n'
import { Avatar } from '@/components/unit/Avatar'
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
  const { pathname } = useLocation()
  const { user, repositories, groups } = useAppData()
  const { lang, setLang } = useI18n()
  const [notificationOpen, setNotificationOpen] = useState(false)

  /** 路径是否落在 `base` 这一层（避免 `/g/repo` 误配 `/g/repo2`）。 */
  const underPath = (base: string) => pathname === base || pathname.startsWith(`${base}/`)

  // 当前所在的仓库 / 仓库组：官方原版在菜单里给当前项打勾（旧 `GroupRepositoryMenu`
  // 的 `currentRepositoryKey` / `currentGroupKey`）。这里直接从路由推导，不再引入全局状态。
  const currentRepository = repositories.find((item) => underPath(`/${item.group.name}/${item.name}`))
  const currentGroup = groups.find((item) => underPath(`/groups/${item.name}`))

  // 列表**不做条数截断**（官方原版就是全量列出）：面板自身有 max-height + overflow，
  // 列表长了滚动，而不是只显示前 20 条让人以为"仓库/仓库组少了"。
  const repositoryMenuItems: MenuItemSpec[] = repositories.map((repository) => ({
    key: repository.id,
    label: `${repository.group.displayName}/${repository.displayName}`,
    icon: <Avatar square size="sm" src={avatarUrl(repository.icon)} name={repository.name} />,
    trailing: currentRepository?.id === repository.id ? <Icon icon={plCheck} size={14} /> : undefined,
    onClick: () => navigate(`/${repository.group.name}/${repository.name}`),
  }))

  const groupMenuItems: MenuItemSpec[] = groups.map((group) => ({
    key: group.id,
    label: group.displayName,
    icon: <Avatar square size="sm" src={avatarUrl(group.icon)} name={group.name} />,
    trailing: !currentRepository && currentGroup?.id === group.id ? <Icon icon={plCheck} size={14} /> : undefined,
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
        {/* 仓库 / 仓库组：对齐官方原版 `GroupRepositoryMenu` ——
            **点击触发器 = 跳到完整列表页**（`/repositories` 全部仓库、`/groups` 仓库组卡片列表），
            **悬停 = 展开快速跳转菜单**（全量条目、分类标题、方形头像、当前项打勾）。
            触发器只有文字 + 线框三角箭头（无左侧图标），展开时箭头翻转 180°。 */}
        <Dropdown
          trigger={
            <Button variant="text" endIcon={<Icon icon={plCaretDownLine} size={12} />}>
              {t('label.repository')}
            </Button>
          }
          openOnHover
          onTriggerClick={() => navigate('/repositories')}
          title={t('label.repository')}
          items={repositoryMenuItems}
          emptyLabel={t('message._S_listEmpty', { s: t('label.repository') })}
          minWidth={272}
        />
        <Dropdown
          trigger={
            <Button variant="text" endIcon={<Icon icon={plCaretDownLine} size={12} />}>
              {t('label.group')}
            </Button>
          }
          openOnHover
          onTriggerClick={() => navigate('/groups')}
          title={t('label.group')}
          items={groupMenuItems}
          emptyLabel={t('message._S_listEmpty', { s: t('label.group') })}
          minWidth={272}
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
              {/* 与「仓库 / 仓库组」同一支线框三角（旧版用户菜单同样用 faAngleDown）。 */}
              <Icon icon={plCaretDownLine} size={12} />
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
