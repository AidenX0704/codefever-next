import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router'

import { useAppData } from '@/features/app-data/AppDataProvider'
import { readRecentRepositories } from '@/helpers/recentRepos'
import { useT } from '@/i18n'
import { Icon } from '@/components/unit/Icon'
import {
  groupIcon,
  homeIcon,
  mergeRequestIcon,
  meterIcon,
  repositoryIcon,
  searchIcon,
  settingsIcon,
} from '@/config/icons'


interface PaletteItem {
  key: string
  label: string
  hint?: string
  group: string
  icon: React.ReactNode
  action: () => void
}

interface CommandPaletteProps {
  open: boolean
  onClose: () => void
}

/**
 * 命令面板（DESIGN.md §4：⌘K / Ctrl+K 呼出，搜索仓库 / 仓库组 / MR / 页面）。
 *
 * - 数据来自 `useAppData()`（已登录可见的仓库与组）+ localStorage 最近访问；
 * - 前缀 `>` 切命令（页面跳转），其余按关键字过滤；
 * - ↑↓ 选择、Enter 执行、Esc 关闭；`/` 聚焦页头搜索的约定由 Header 处理。
 *
 * 无后端搜索接口对接（/api/search/all 是独立页面），面板内只做本地过滤 ——
 * 真正的全文搜索引导到底部「在搜索页查找 …」。
 */
export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const t = useT()
  const navigate = useNavigate()
  const { user, repositories, groups } = useAppData()
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (open) {
      setQuery('')
      setActive(0)
      // 等挂载完成再聚焦（Dialog 系浮层通用做法）
      requestAnimationFrame(() => inputRef.current?.focus())
    }
  }, [open])

  const commandMode = query.startsWith('>')

  const items = useMemo<PaletteItem[]>(() => {
    const recent = readRecentRepositories()
    const out: PaletteItem[] = []

    if (!commandMode) {
      // 1. 最近访问（有历史才出这一组）
      for (const r of recent) {
        out.push({
          key: `recent:${r.groupName}/${r.repositoryName}`,
          label: `${r.groupName}/${r.repositoryName}`,
          group: t('palette.group.recent'),
          icon: <Icon icon={repositoryIcon} size={16} />,
          action: () => navigate(`/${r.groupName}/${r.repositoryName}`),
        })
      }
      // 2. 仓库
      for (const repo of repositories) {
        out.push({
          key: `repo:${repo.id}`,
          label: `${repo.group.displayName}/${repo.displayName}`,
          hint: `${repo.group.name}/${repo.name}`,
          group: t('label.repository'),
          icon: <Icon icon={repositoryIcon} size={16} />,
          action: () => navigate(`/${repo.group.name}/${repo.name}`),
        })
      }
      // 3. 仓库组
      for (const group of groups) {
        out.push({
          key: `group:${group.id}`,
          label: group.displayName,
          hint: group.name,
          group: t('label.group'),
          icon: <Icon icon={groupIcon} size={16} />,
          action: () => navigate(`/groups/${group.name}`),
        })
      }
    }

    // 4. 命令（页面跳转；命令模式 > 下排在前）
    const commands: PaletteItem[] = [
      {
        key: 'cmd:home',
        label: t('label.home'),
        group: t('palette.group.commands'),
        icon: <Icon icon={homeIcon} size={16} />,
        action: () => navigate('/home'),
      },
      {
        key: 'cmd:repositories',
        label: t('label.repository'),
        group: t('palette.group.commands'),
        icon: <Icon icon={repositoryIcon} size={16} />,
        action: () => navigate('/repositories'),
      },
      {
        key: 'cmd:groups',
        label: t('label.group'),
        group: t('palette.group.commands'),
        icon: <Icon icon={groupIcon} size={16} />,
        action: () => navigate('/groups'),
      },
      {
        key: 'cmd:mrs',
        label: t('label.mergeRequest'),
        group: t('palette.group.commands'),
        icon: <Icon icon={mergeRequestIcon} size={16} />,
        action: () => navigate('/mergerequests'),
      },
      {
        key: 'cmd:search',
        label: t('search.title'),
        group: t('palette.group.commands'),
        icon: <Icon icon={searchIcon} size={16} />,
        action: () => navigate('/search'),
      },
      {
        key: 'cmd:settings',
        label: t('menu.setting_pl'),
        group: t('palette.group.commands'),
        icon: <Icon icon={settingsIcon} size={16} />,
        action: () => navigate('/settings/profile'),
      },
    ]
    if (user?.admin) {
      commands.push({
        key: 'cmd:admin',
        label: t('label.adminArea'),
        group: t('palette.group.commands'),
        icon: <Icon icon={meterIcon} size={16} />,
        action: () => navigate('/admin/dashboard'),
      })
    }
    out.push(...commands)

    const kw = (commandMode ? query.slice(1) : query).trim().toLowerCase()
    if (!kw) {
      // 无关键字：命令模式只看命令；默认模式限 20 条避免一次渲染过长
      return commandMode ? commands : out.slice(0, 20)
    }
    return out.filter(
      (item) =>
        item.label.toLowerCase().includes(kw) ||
        (item.hint ?? '').toLowerCase().includes(kw) ||
        (commandMode && item.group === t('palette.group.commands')),
    )
  }, [commandMode, query, repositories, groups, user?.admin, navigate, t])

  // 关键字变化后选中项可能越界
  useEffect(() => {
    setActive((current) => Math.min(current, Math.max(0, items.length - 1)))
  }, [items.length])

  // 当前选中项滚进可视区（键盘导航时）；open=false 时 listRef 必为空，效果是空转。
  useEffect(() => {
    const list = listRef.current
    if (!list) {
      return
    }
    const el = list.querySelector<HTMLElement>(`[data-index="${active}"]`)
    el?.scrollIntoView({ block: 'nearest' })
  }, [active])

  if (!open) {
    return null
  }

  const run = (item: PaletteItem | undefined) => {
    if (!item) {
      return
    }
    onClose()
    item.action()
  }

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
      return
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      setActive((current) => (items.length ? (current + 1) % items.length : 0))
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      setActive((current) => (items.length ? (current - 1 + items.length) % items.length : 0))
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      run(items[active])
    }
  }

  // 渲染分组标题（相邻同名只出一次）
  let lastGroup = ''

  return (
    <div className="cf-palette-backdrop" onMouseDown={onClose} role="presentation">
      <div
        className="cf-palette"
        role="dialog"
        aria-modal="true"
        aria-label={t('palette.title')}
        onMouseDown={(event) => event.stopPropagation()}
        onKeyDown={onKeyDown}
      >
        <div className="cf-palette__search">
          <Icon icon={searchIcon} size={16} />
          <input
            ref={inputRef}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t('palette.placeholder')}
            aria-label={t('palette.title')}
            spellCheck={false}
            autoComplete="off"
          />
          <kbd className="cf-palette__kbd">Esc</kbd>
        </div>
        <div className="cf-palette__list" ref={listRef} role="listbox">
          {items.length === 0 ? (
            <div className="cf-palette__empty">{t('message.noResult')}</div>
          ) : (
            items.map((item, index) => {
              const showGroup = item.group !== lastGroup
              lastGroup = item.group
              return (
                <div key={item.key}>
                  {showGroup ? <div className="cf-palette__group">{item.group}</div> : null}
                  <button
                    type="button"
                    role="option"
                    aria-selected={index === active}
                    data-index={index}
                    className={`cf-palette__item${index === active ? ' cf-palette__item--active' : ''}`}
                    onMouseEnter={() => setActive(index)}
                    onClick={() => run(item)}
                  >
                    {item.icon}
                    <span className="cf-palette__label">{item.label}</span>
                    {item.hint ? <span className="cf-palette__hint">{item.hint}</span> : null}
                  </button>
                </div>
              )
            })
          )}
        </div>
        <div className="cf-palette__foot">
          <span>↑↓ {t('palette.navigate')}</span>
          <span>↵ {t('palette.open')}</span>
          <span>&gt; {t('palette.commandHint')}</span>
        </div>
      </div>
    </div>
  )
}
