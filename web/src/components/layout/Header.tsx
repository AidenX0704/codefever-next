import { Suspense, lazy, useState } from 'react'
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
// 通知抽屉只在用到时才加载，避免把代码带进首屏。
const NotificationDrawer = lazy(() => import('./NotificationDrawer').then((mod) => ({ default: mod.NotificationDrawer })))


/**
 * 顶栏（v5 DESIGN.md §4.3：通栏雾面，只承载上下文与全局动作）。
 *
 * 结构：
 *   左：面包屑（当前平台分区，一级导航已沉入平台侧栏）
 *   右：＋新建 / 通知 / 帮助 / 头像（外观、语言、个人信息、退出都在头像菜单）
 *
 * 2026-11-22 减负：⌘K 命令面板下线（与侧栏导航、各列表页搜索三重重复），
 * 面包屑不再有个人主页分支（/:username 已下线）。
 */
export function Header() {
  const t = useT()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { user } = useAppData()
  const { lang, setLang } = useI18n()
  const [notificationOpen, setNotificationOpen] = useState(false)
  const [, setThemeTick] = useState(0) // 主题切换后重渲染菜单勾选态

  /** 路径是否落在 `base` 这一层（避免 `/g/repo` 误配 `/g/repo2`）。 */
  const underPath = (base: string) => pathname === base || pathname.startsWith(`${base}/`)

  // 面包屑：当前平台分区（v5 §4.3 左位；仓库页与未知路径一律归「仓库」）。
  const crumbKey = (() => {
    if (underPath('/groups')) return 'label.group'
    if (underPath('/mergerequests')) return 'label.mergeRequest'
    if (underPath('/settings')) return 'menu.profile'
    if (underPath('/admin')) return 'label.adminArea'
    return 'label.repository'
  })()

  const createItems: MenuItemSpec[] = [
    { key: 'new-repository', label: t('label.newRepository'), onClick: () => navigate('/repositories/new') },
    { key: 'new-group', label: t('label.newGroup'), onClick: () => navigate('/groups/new') },
    { key: 'new-merge-request', label: t('label.createMergeRequest'), onClick: () => navigate('/mergerequests/new') },
  ]

  const helpItems: MenuItemSpec[] = [
    { key: 'docs', label: t('help.docs.title'), onClick: () => navigate(docUrl(lang)) },
    { key: 'api-docs', label: t('help.api.title'), onClick: () => navigate(publicApiDocUrl(lang)), dividerAfter: true },
    { key: 'support', label: t('label.support'), onClick: () => window.open(HELP_LINKS.support, '_blank', 'noopener') },
    { key: 'feedback', label: t('label.feedback'), onClick: () => window.open(HELP_LINKS.feedback, '_blank', 'noopener') },
    { key: 'contribute', label: t('label.contribute'), onClick: () => window.open(HELP_LINKS.contribute, '_blank', 'noopener') },
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
    </header>
  )
}
