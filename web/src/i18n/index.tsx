import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

import { getLang, setLang as persistLang } from '@/api/client'

import { messages as enMessages } from './en-us'
import { messages as zhMessages } from './zh-cn'
import { supplement } from './supplement'

/**
 * 轻量 i18n。
 *
 * 词表来源：`backend/internal/httpapi/pages.go` 的 `langZhCN` / `langEnglish`
 * （逐字搬运自 PHP 的 language 包），由 `scripts/extract-lang.mjs` 生成，勿手改。
 *
 * 为什么不引 react-intl/i18next：本项目只需要"键 → 字符串"与语言切换，
 * 不做复数/日期本地化（日期格式由 date-fns 或原生 Intl 在 helper 层处理），
 * 引第三方库只会让首屏多一个依赖。
 */

export type Lang = 'zh-cn' | 'en-us'

export type MessageKey = keyof typeof zhMessages | keyof typeof enMessages

/**
 * 运行时词表 = 自动生成的旧词表 ← 补充词表（补充优先）。
 * 自动生成的表见 scripts/extract-lang.mjs；补充表见 i18n/supplement.ts。
 */
const TABLES: Record<Lang, Record<string, string>> = {
  'zh-cn': { ...zhMessages, ...supplement['zh-cn'] },
  'en-us': { ...enMessages, ...supplement['en-us'] },
}

function normalizeLang(value: string): Lang {
  return value.toLowerCase().startsWith('en') ? 'en-us' : 'zh-cn'
}

export interface I18nApi {
  lang: Lang
  setLang: (lang: Lang) => void
  /**
   * 取文案并做占位符替换。
   *
   * 旧词表里有 `{s}` / `{n}` / `{p}` 这类占位符（如 `label.forkFrom_S` = `fork 自 {s}`），
   * 旧代码用 react-intl 的 `formatMessage({id}, {s: ...})` 填充；这里用最小实现等价替换，
   * 缺失的占位符原样保留（便于发现漏传）。
   */
  t: (key: MessageKey | (string & {}), values?: Record<string, string | number>) => string
}

const I18nContext = createContext<I18nApi | null>(null)

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => normalizeLang(getLang()))

  const setLang = useCallback((next: Lang) => {
    setLangState(next)
    // 与 api client 共用同一个 localStorage 键：请求头 codefever-end-lang 随之切换。
    persistLang(next)
  }, [])

  useEffect(() => {
    document.documentElement.lang = lang
  }, [lang])

  const t = useCallback(
    (key: MessageKey | (string & {}), values?: Record<string, string | number>) => {
      const template = TABLES[lang][key as string] ?? String(key)
      if (!values) {
        return template
      }
      return template.replace(/\{(\w+)\}/g, (match, name: string) =>
        values[name] === undefined ? match : String(values[name]),
      )
    },
    [lang],
  )

  const value = useMemo<I18nApi>(() => ({ lang, setLang, t }), [lang, setLang, t])

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nApi {
  const ctx = useContext(I18nContext)
  if (!ctx) {
    throw new Error('useI18n 必须在 <I18nProvider> 内使用')
  }
  return ctx
}

/** 只取 `t` 的便捷钩子。 */
export function useT(): I18nApi['t'] {
  return useI18n().t
}
