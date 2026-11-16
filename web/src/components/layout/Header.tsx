import { Suspense, lazy, useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'

import { HELP_LINKS } from '@/config/helpLinks'
import { publicApiDocUrl } from '@/config/publicApi'
import {
  caretDownIcon,
  checkIcon,
  meterIcon,
  notificationIcon,
  plExit,
  plusIcon,
  questionIcon,
  searchIcon,
  settingsIcon,
  userIcon,
} from '@/config/icons'
import { useAppData } from '@/features/app-data/AppDataProvider'
import { docUrl } from '@/helpers/docUrl'
import { applyTheme, getTheme } from '@/helpers/theme'
import { useI18n, useT } from '@/i18n'
import { Dropdown, type MenuItemSpec } from '@/components/unit/Dropdown'
import { Icon } from '@/components/unit/Icon'
import { SquareIconButton } from '@/components/unit/SquareIconButton'
// 通知抽屉 / 命令面板只在用到时才加载，避免把代码带进首屏。
const NotificationDrawer = lazy(() => import('./NotificationDrawer').then((mod) => ({ default: mod.NotificationDrawer })))
const CommandPalette = lazy(() => import('./CommandPalette').then((mod) => ({ default: mod.CommandPalette })))


/**
 * 顶栏（v5 DESIGN.md §4.3：通栏雾面，只承载上下文与全局动作）。
 *
 * 结构：
 *   左：面包屑（当前平台分区，一级导航已沉入平台侧栏）
 *   中：?K 搜索按钮（打开命令面板）
 *   右：＋新建 / 通知 / 帮助 / 头像（外观、语言、个人信息、退出都在头像菜单）
 *
 * v5 变化：Logo 与主导航（首页/仓库/仓库组/合并请求）沉入平台侧栏
 * （PlatformSidebar）；本栏不再承载一级导航。
 */
export function Header() {
  const t = useT()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { user } = useAppData()
  const { lang, setLang } = useI18n()
  const [notificationOpen, setNotificationOpen] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [, setThemeTick] = useState(0) // 主题切换后重渲染菜单勾选态

  /** 路径是否落在 `base` 这一层（避免 `/g/repo` 误配 `/g/repo2`）。 */
  const underPath = (base: string) => pathname === base || pathname.startsWith(`${base}/`)

  // 全局快捷键：?K / Ctrl+K 呼出命令面板（DESIGN.md §4）。
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setPaletteOpen((open) => !open)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // 面包屑：当前平台分区（v5 §4.3 左位；仓库详情页归「仓库」）。
  const crumbKey = (() => {
    if (underPath('/home')) return 'label.home'
    if (underPath('/groups')) return 'label.group'
    if (underPath('/mergerequests')) return 'label.mergeRequest'
    if (underPath('/search')) return 'label.search'
    if (underPath('/settings')) return 'menu.profile'
    if (underPath('/admin')) return 'label.adminArea'
    if (underPath('/help')) return 'help.center'
    if (/^\/(?!groups\b|mergerequests\b|settings\b|admin\b|search\b|help\b|home\b|user\b|doc\b)[^/]+\/[^/]+/.test(pathname)) {
      return 'label.repository'
    }
    // 单段路径且不是任何静态路由 = 个人主页 /:username。
    if (/^\/(?!home\b|repositories\b|groups\b|mergerequests\b|settings\b|admin\b|search\b|help\b|user\b|doc\b|__design\b)[^/]+$/.test(pathname)) {
      return 'profile.title'
    }
    return 'label.home'
  })()

  const createItems: MenuItemSpec[] = [
    { key: 'new-repository', label: t('label.newRepository'), onClick: () => navigate('/repositories/new') },
    { key: 'new-group', label: t('label.newGroup'), onClick: () => navigate('/groups/new') },
    { key: 'new-merge-request', label: t('label.createMergeRequest'), onClick: () => navigate('/mergerequests/new') },
  ]

  const helpItems: MenuItemSpec[] = [
    { key: 'help-center', label: t('help.center'), onClick: () => navigate('/help') },
    { key: 'docs', label: t('help.docs.title'), onClick: () => navigate(docUrl(lang)) },
    { key: 'api-docs', label: t('help.api.title'), onClick: () => navigate(publicApiDocUrl(lang)), dividerAfter: true },
    { key: 'support', label: t('label.support'), onClick: () => window.open(HELP_LINKS.support, '_blank', 'noopener') },
    { key: 'feedback', label: t('label.feedback'), onClick: () => window.open(HELP_LINKS.feedback, '_blank', 'noopener') },
    { key: 'contribute', label: t('label.contribute'), onClick: () => window.open(HELP_LINKS.contribute, '_blank', 'noopener'), dividerAfter: true },
    { key: 'about', label: t('label.about'), onClick: () => navigate('/help#about') },
  ]

  const languageItems: MenuItemSpec[] = [
    {
      key: 'zh-cn',
      label: t('lang.chinese'),
      trailing: lang === 'zh-cn' ? <Icon icon={checkIcon} size={14} /> : undefined,
      onClick: () => setLang('zh-cn'),
    },
    {
      key: 'en-us',
      label: t('lang.english'),
      trailing: lang === 'en-us' ? <Icon icon={checkIcon} size={14} /> : undefined,
      onClick: () => setLang('en-us'),
    },
  ]

  const appearanceItems: MenuItemSpec[] = [
    {
      key: 'light',
      label: t('appearance.light'),
      trailing: getTheme() === 'light' ? <Icon icon={checkIcon} size={14} /> : undefined,
      onClick: () => {
        applyTheme('light')
        setThemeTick((tick) => tick + 1)
      },
    },
    {
      key: 'dark',
      label: t('appearance.dark'),
      trailing: getTheme() === 'dark' ? <Icon icon={checkIcon} size={14} /> : undefined,
      onClick: () => {
        applyTheme('dark')
        setThemeTick((tick) => tick + 1)
      },
    },
  ]

  // 头像菜单（DESIGN.md §4：管理后台 / 外观 / 语言 / 退出全部收进来）。
  // 「外观 / 语言」两项直接给出两个候选（带当前勾选），避免再做二级菜单。
  const userItems: MenuItemSpec[] = [
    // 个人主页（/:username）与个人信息（/settings/profile）是两个落点：前者是
    // 别人也能看的公开页，后者是编辑表单 —— GitHub 同款分工，都要给入口。
    ...(user
      ? [{ key: 'my-page', label: t('profile.title'), icon: <Icon icon={userIcon} size={16} />, onClick: () => navigate(`/${user.name}`) } as MenuItemSpec]
      : []),
    { key: 'profile', label: t('menu.profile'), icon: <Icon icon={userIcon} size={16} />, onClick: () => navigate('/settings/profile') },
    { key: 'tokens', label: t('tokens.title'), icon: <Icon icon={meterIcon} size={16} />, onClick: () => navigate('/settings/tokens') },
    ...appearanceItems,
    ...languageItems,
    ...(user?.admin
      ? [{ key: 'admin', label: t('label.adminArea'), icon: <Icon icon={settingsIcon} size={16} />, onClick: () => navigate('/admin/dashboard') } as MenuItemSpec]
      : []),
    { key: 'logout', label: t('menu.logout'), icon: <Icon icon={plExit} size={16} />, onClick: () => { window.location.href = '/user/logout' }, dividerAfter: true },
  ]

  return (
    <header className="app-header">
      <div className="app-header__left">
        <span className="app-header__crumb">{t(crumbKey)}</span>
      </div>

      {/* ⌘K 命令面板按钮（DESIGN.md §4 中部）：替代原输入框；命令面板里可搜仓库/组/命令。 */}
      <div className="app-header__palette">
        <button
          type="button"
          className="app-header__palette-btn"
          onClick={() => setPaletteOpen(true)}
          aria-label={t('palette.title')}
        >
          <Icon icon={searchIcon} size={16} />
          <span className="app-header__palette-text">{t('search.placeholder')}</span>
          <kbd className="app-header__palette-kbd">⌘K</kbd>
        </button>
      </div>

      <div className="app-header__right">
        <Dropdown trigger={<SquareIconButton icon={plusIcon} label={t('label.create')} />} items={createItems} align="right" />
        <SquareIconButton
          icon={notificationIcon}
          label={t('label.notification')}
          badge={user?.unReadNotification ?? 0}
          onClick={() => setNotificationOpen(true)}
        />
        <Dropdown trigger={<SquareIconButton icon={questionIcon} label={t('label.help')} />} items={helpItems} align="right" />
        <Dropdown
          align="right"
          minWidth={240}
          items={userItems}
          trigger={
            <span className="app-header__user">
              {user?.icon ? <img className="app-header__avatar" src={`/file/fetch/avatar/${user.icon}`} alt="" /> : <span className="app-header__avatar" />}
              <Icon icon={caretDownIcon} size={12} />
            </span>
          }
        />
      </div>
      {notificationOpen ? (
        <Suspense fallback={null}>
          <NotificationDrawer open onClose={() => setNotificationOpen(false)} />
        </Suspense>
      ) : null}
      {paletteOpen ? (
        <Suspense fallback={null}>
          <CommandPalette open onClose={() => setPaletteOpen(false)} />
        </Suspense>
      ) : null}
    </header>
  )
}
