import { useState } from 'react'

import { applyTheme, getSystemTheme, getStoredTheme, type Theme } from '@/helpers/theme'
import { useI18n, useT } from '@/i18n'
import { Button } from '@/components/unit/Button'
import { SectionCard } from '@/components/unit/SectionCard'


/**
 * 外观与语言（/settings/appearance）。
 *
 * 主题走 helpers/theme 的唯一入口（applyTheme + localStorage 持久化）；
 * 语言走既有 useI18n。DESIGN.md §7.8：两者都是即时生效，无「保存」按钮。
 */
export function Appearance() {
  const t = useT()
  const { lang, setLang } = useI18n()
  // stored=null 表示"跟随系统"，此时按钮组的第三项选中
  const [theme, setTheme] = useState<Theme | 'system'>(getStoredTheme() ?? 'system')

  const choose = (next: Theme | 'system') => {
    setTheme(next)
    applyTheme(next === 'system' ? null : next)
  }

  const systemPrefers = getSystemTheme()

  return (
    <div className="page">
      <div>
        <div className="page__title">{t('appearance.title')}</div>
        <div className="page__subtitle">{t('appearance.themeNote')}</div>
      </div>

      <SectionCard title={t('appearance.theme')}>
        <div className="row">
          {([
            { key: 'light', label: t('appearance.light') },
            { key: 'dark', label: t('appearance.dark') },
            { key: 'system', label: `${t('appearance.system')}（${systemPrefers === 'dark' ? t('appearance.dark') : t('appearance.light')}）` },
          ] as const).map((item) => (
            <Button
              key={item.key}
              size="sm"
              variant={theme === item.key ? 'contained' : 'outlined'}
              aria-pressed={theme === item.key}
              onClick={() => choose(item.key)}
            >
              {item.label}
            </Button>
          ))}
        </div>
      </SectionCard>

      <SectionCard title={t('label.language')}>
        <div className="row">
          <Button
            size="sm"
            variant={lang === 'zh-cn' ? 'contained' : 'outlined'}
            aria-pressed={lang === 'zh-cn'}
            onClick={() => setLang('zh-cn')}
          >
            {t('lang.chinese')}
          </Button>
          <Button
            size="sm"
            variant={lang === 'en-us' ? 'contained' : 'outlined'}
            aria-pressed={lang === 'en-us'}
            onClick={() => setLang('en-us')}
          >
            {t('lang.english')}
          </Button>
        </div>
      </SectionCard>
    </div>
  )
}

export default Appearance
