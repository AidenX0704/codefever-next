import { useEffect, type ReactNode } from 'react'
import { useLocation, useNavigate } from 'react-router'

import { AppLink } from '@/components/unit/AppLink'
import { Button } from '@/components/unit/Button'
import { Card } from '@/components/unit/Card'
import { CodeBlock } from '@/components/unit/CodeBlock'
import { Icon } from '@/components/unit/Icon'
import { DataTable, type Column } from '@/components/unit/Table'
import { Tag, type TagTone } from '@/components/unit/Tag'
import { HELP_LINKS } from '@/config/helpLinks'
import { PUBLIC_API_ENDPOINTS, publicApiDocUrl, type PublicApiAuth, type PublicApiEndpoint } from '@/config/publicApi'
import { plDraft, plFork, plHelp, psNewFeature } from '@/config/icons'
import { useI18n, useT } from '@/i18n'


/** 鉴权要求 → 标签色调（"需要令牌"最需要被看见，用主色强调）。 */
const AUTH_TONE: Record<PublicApiAuth, TagTone> = {
  required: 'primary',
  optional: 'warning',
  public: 'success',
}

interface AboutRow {
  key: string
  label: string
  value: ReactNode
}

/**
 * 帮助中心（`/help`）。
 *
 * 为什么要有这一页：旧版把"技术支持 / 提交反馈 / 贡献 / 关于"四个入口直接散在
 * 页头帮助菜单里，每项都跳站外，站内没有一个"我该去哪儿"的落点；新增的公开 API
 * （`/api/v1`）也没有任何面向使用者的说明。这里把它们收在一页：
 *   1. 四个入口卡片（技术支持 / 提交反馈 / 为 CodeFever Next 贡献代码 / 关于）；
 *   2. 文档中心入口（站内 `/doc/*` 与完整 API 文档）；
 *   3. 公开 API 端点总览 + 鉴权与错误体约定（数据源是 `config/publicApi.ts`，
 *      与后端 `internal/httpapi/publicapi/router.go` 一一对应）；
 *   4. 关于 CodeFever Next（维护者、许可证、上游项目）。
 *
 * 页面只做"字段组装 + 动作绑定"，表格/卡片/代码块/复制全部走 `components/unit/`。
 */
export default function Help() {
  const t = useT()
  const { lang } = useI18n()
  const navigate = useNavigate()
  const { hash } = useLocation()

  // 页内锚点（#api / #about）：react-router 的 Link 不会自动滚动到锚点，这里补一步。
  useEffect(() => {
    if (!hash) {
      return
    }
    document.getElementById(hash.replace(/^#/, ''))?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }, [hash])

  const docCenter = `/doc/${lang === 'en-us' ? 'en' : 'cn'}`
  const apiDoc = publicApiDocUrl(lang)
  const origin = window.location.origin
  const apiBase = `${origin}/api/v1`

  const authSample = [
    `# ${t('help.api.authSampleHint')}`,
    'TOKEN=<YOUR_ACCESS_TOKEN>',
    `curl -H "Authorization: token $TOKEN" ${apiBase}/user`,
    `curl -H "Authorization: Bearer $TOKEN" ${apiBase}/user`,
    `curl "${apiBase}/user?access_token=$TOKEN"`,
  ].join('\n')

  const errorSample = JSON.stringify({ message: t('help.api.errorSampleMessage'), url: `${origin}${apiDoc}` }, null, 2)

  const entries = [
    {
      key: 'support',
      title: t('label.support'),
      desc: t('help.entry.support'),
      icon: plHelp,
      onClick: () => window.open(HELP_LINKS.support, '_blank', 'noopener'),
    },
    {
      key: 'feedback',
      title: t('label.feedback'),
      desc: t('help.entry.feedback'),
      icon: plDraft,
      onClick: () => window.open(HELP_LINKS.feedback, '_blank', 'noopener'),
    },
    {
      key: 'contribute',
      title: t('label.contribute'),
      desc: t('help.entry.contribute'),
      icon: plFork,
      onClick: () => window.open(HELP_LINKS.contribute, '_blank', 'noopener'),
    },
    {
      key: 'about',
      title: t('label.about'),
      desc: t('help.entry.about'),
      icon: psNewFeature,
      onClick: () => document.getElementById('about')?.scrollIntoView({ behavior: 'smooth', block: 'start' }),
    },
  ]

  const columns: ReadonlyArray<Column<PublicApiEndpoint>> = [
    {
      key: 'method',
      title: t('help.api.colMethod'),
      width: '80px',
      render: (row) => <Tag tone="info">{row.method}</Tag>,
    },
    {
      key: 'path',
      title: t('help.api.colPath'),
      render: (row) => <code className="help-code">{row.path}</code>,
    },
    {
      key: 'auth',
      title: t('help.api.colAuth'),
      width: '120px',
      render: (row) => <Tag tone={AUTH_TONE[row.auth]}>{t(`help.api.auth_${row.auth}`)}</Tag>,
    },
    { key: 'desc', title: t('help.api.colDesc'), render: (row) => t(row.descKey) },
  ]

  const aboutRows: ReadonlyArray<AboutRow> = [
    { key: 'project', label: t('help.about.project'), value: t('help.about.projectValue') },
    { key: 'tech', label: t('help.about.tech'), value: t('help.about.techValue') },
    { key: 'license', label: t('help.about.license'), value: 'MIT' },
    {
      key: 'maintainer',
      label: t('help.about.maintainer'),
      value: (
        <AppLink to={HELP_LINKS.maintainer} external>
          GiserXiao
        </AppLink>
      ),
    },
    {
      key: 'repository',
      label: t('help.about.repository'),
      value: (
        <AppLink to={HELP_LINKS.contribute} external>
          github.com/AidenX0704/codefever-next
        </AppLink>
      ),
    },
    {
      key: 'issues',
      label: t('help.about.issues'),
      value: (
        <AppLink to={HELP_LINKS.support} external>
          github.com/AidenX0704/codefever-next/issues
        </AppLink>
      ),
    },
    {
      key: 'security',
      label: t('help.about.security'),
      value: (
        <AppLink to={HELP_LINKS.securityPolicy} external>
          SECURITY.md
        </AppLink>
      ),
    },
    {
      key: 'upstream',
      label: t('help.about.upstream'),
      value: (
        <>
          <AppLink to={HELP_LINKS.upstream} external>
            PGYER/codefever
          </AppLink>
          {' · '}
          <AppLink to={HELP_LINKS.upstreamSite} external>
            codefever.cn
          </AppLink>
        </>
      ),
    },
  ]

  return (
    <div className="page">
      <div>
        <div className="page__title">{t('help.title')}</div>
        <div className="page__subtitle">{t('help.subtitle')}</div>
      </div>

      <section className="section">
        <div className="section__title">{t('help.entrySection')}</div>
        <div className="group-grid">
          {entries.map((entry) => (
            <Card
              key={entry.key}
              hoverable
              className="help-entry"
              role="button"
              tabIndex={0}
              onClick={entry.onClick}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault()
                  entry.onClick()
                }
              }}
            >
              <div className="help-entry__head">
                <Icon icon={entry.icon} size={20} />
                <span className="help-entry__title">{entry.title}</span>
              </div>
              <div className="help-entry__desc">{entry.desc}</div>
              <span className="cf-link">{t('help.open')}</span>
            </Card>
          ))}
        </div>
      </section>

      <section className="section">
        <div className="section__title">{t('help.docs.title')}</div>
        <div className="muted">{t('help.docs.desc')}</div>
        <div className="row">
          <AppLink to={docCenter}>{t('help.docs.open')}</AppLink>
          <span className="muted">·</span>
          <AppLink to={apiDoc}>{t('help.api.readMore')}</AppLink>
          <span className="muted">·</span>
          <AppLink to={HELP_LINKS.contributingGuide} external>
            {t('help.docs.contributing')}
          </AppLink>
        </div>
      </section>

      <section className="section" id="api">
        <div className="section__title">{t('help.api.title')}</div>
        <div className="muted">{t('help.api.desc')}</div>

        <div className="help-subtitle">{t('help.api.summary')}</div>
        <DataTable columns={columns} rows={PUBLIC_API_ENDPOINTS} rowKey={(row) => row.path} />

        <div className="help-subtitle">{t('help.api.auth')}</div>
        <div className="muted">{t('help.api.authDesc')}</div>
        <CodeBlock code={authSample} language="bash" copyMessage={t('help.copied')} />

        <div className="help-subtitle">{t('help.api.errorBody')}</div>
        <div className="muted">{t('help.api.errorDesc')}</div>
        <CodeBlock code={errorSample} language="json" copyMessage={t('help.copied')} />

        <div className="row">
          <Button variant="text" onClick={() => navigate(apiDoc)}>
            {t('help.api.readMore')}
          </Button>
        </div>
      </section>

      <section className="section" id="about">
        <div className="section__title">{t('label.about')}</div>
        <div className="muted">{t('help.about.desc')}</div>
        <div className="help-info">
          {aboutRows.map((row) => (
            <div key={row.key} className="help-info__row">
              <span className="help-info__label">{row.label}</span>
              <span className="help-info__value">{row.value}</span>
            </div>
          ))}
        </div>
      </section>
    </div>
  )
}
