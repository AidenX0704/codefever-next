import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import { fetchDocContent, fetchDocTree, type DocMenuItem } from '@/api/doc'
import { headerLogo } from '@/assets/images'
import { Loading } from '@/components/unit/Loading'
import { useT } from '@/i18n'


/** 取菜单里的第一篇文档（进入 `/doc/cn` 时的默认落地页）。 */
function firstLeaf(menu: DocMenuItem[]): DocMenuItem | null {
  for (const item of menu) {
    if (item.submenu?.length) {
      const nested = firstLeaf(item.submenu)
      if (nested) {
        return nested
      }
    }
    if (item.path) {
      return item
    }
  }
  return null
}

/**
 * 文档中心（`/doc/{cn|en}/{path}`）。
 *
 * 数据来自 Go 新增的两个只读端点（`/api/doc/tree`、`/api/doc/content`），
 * 它们复用 SSR 页面的目录解析与路径穿越校验，因此内容与旧文档页完全同源。
 * Markdown 渲染用 `react-markdown` + `remark-gfm`（表格/任务列表），样式沿用
 * `markdown-body.css`（旧版内置的 github-markdown-css）。
 *
 * 与旧版的差异：旧版由服务端把 Markdown 塞进模板、浏览器用 marked 渲染；
 * 这里是"拉 JSON + 客户端渲染"，URL 仍保持 `/doc/{lang}/{path}` 可分享。
 */
export default function Doc() {
  const t = useT()
  const { lang: routeLang, '*': splat } = useParams()
  const lang: 'cn' | 'en' = routeLang === 'en' ? 'en' : 'cn'

  const [menu, setMenu] = useState<DocMenuItem[]>([])
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const selected = (splat ?? '').replace(/^\//, '')

  useEffect(() => {
    let cancelled = false
    fetchDocTree(lang)
      .then((tree) => {
        if (cancelled) {
          return
        }
        setMenu(tree)
      })
      .catch(() => setMenu([]))
    return () => {
      cancelled = true
    }
  }, [lang])

  const fallback = useMemo(() => firstLeaf(menu), [menu])
  const target = selected || fallback?.path || ''

  useEffect(() => {
    if (!target) {
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchDocContent(lang, target)
      .then((text) => {
        if (!cancelled) {
          setContent(text)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : '文档加载失败')
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [lang, target])

  return (
    <div className="doc-page">
      <header className="doc-header">
        <Link to="/repositories">
          <img className="doc-header__logo" src={headerLogo} alt="CodeFever" />
        </Link>
        <span className="doc-header__spacer" />
        <Link className={lang === 'cn' ? 'cf-link doc-header__lang--active' : 'cf-link'} to={`/doc/cn/${target}`}>
          {t('base_cn')}
        </Link>
        <Link className={lang === 'en' ? 'cf-link doc-header__lang--active' : 'cf-link'} to={`/doc/en/${target}`}>
          {t('base_en')}
        </Link>
      </header>

      <div className="doc-body">
        <nav className="doc-menu">
          {menu.map((item) => (
            <div key={item.path || item.name} className="doc-menu__group">
              <Link
                className={item.path === target ? 'doc-menu__item doc-menu__item--active' : 'doc-menu__item'}
                to={`/doc/${lang}/${item.path}`}
              >
                {item.name}
              </Link>
              {(item.submenu ?? []).map((child) => (
                <Link
                  key={child.path}
                  className={child.path === target ? 'doc-menu__item doc-menu__item--active' : 'doc-menu__item doc-menu__item--sub'}
                  to={`/doc/${lang}/${child.path}`}
                >
                  {child.name}
                </Link>
              ))}
            </div>
          ))}
        </nav>

        <article className="doc-content markdown-body">
          {loading ? <Loading /> : null}
          {error ? <div className="auth-message">{error}</div> : null}
          {!loading && !error ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown> : null}
        </article>
      </div>
    </div>
  )
}
