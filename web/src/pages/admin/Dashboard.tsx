import { useEffect, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'

import { fetchSystemStatus } from '@/api/admin'
import type { AdminSystemStatus } from '@/api/types'
import { Card } from '@/components/unit/Card'
import { Icon, type IconProps } from '@/components/unit/Icon'
import { Loading } from '@/components/unit/Loading'
import { Tag, type TagTone } from '@/components/unit/Tag'
import { useToast } from '@/components/unit/Toast'
import { psFolder, psMembers, psMeter, psNewFeature, psRepository, psRepositoryGroup } from '@/config/icons'
import { useI18n, useT } from '@/i18n'


/**
 * 管理后台仪表盘（`/admin/dashboard`）。
 *
 * `admin/systemStatus` 返回三段：`usage`（用户/仓库/组数量）、`vm`（cpu/memory 百分比与磁盘用量）、
 * `service`（nginx / codefever / sendmail 是否在跑）。
 *
 * 与旧版 `components/view/admin/Dashboard.js` + `components/unit/NumbericDashboard.js` 的对齐点：
 *   - 三张统计卡可点击跳转到对应管理页（旧版 Dashboard.js:68-75 → NumbericDashboard.js:80）；
 *   - 百分比阈值配色：≥85 红、≥70 黄、其余绿（旧版 NumbericDashboard.js:47-58），CPU / 内存都适用；
 *   - 数值与单位分开渲染，数字带千分位（旧版 `helpers/VaribleHelper.js` 的 formatNumber / formatUnit）；
 *   - 「磁盘使用量」独立成组（旧版 Dashboard.js:85-87），不内联在「系统资源」下。
 */

/** 使用率阈值配色（旧版 NumbericDashboard.js:47-58）。 */
function usageTone(percent: number): TagTone {
  if (percent >= 85) {
    return 'danger'
  }
  if (percent >= 70) {
    return 'warning'
  }
  return 'success'
}

/**
 * 数值格式化：千分位 + 单位分离。
 *
 * 旧版 `helpers/VaribleHelper.js` 的 `formatNumber` / `formatUnit` 是"数字与单位分开渲染"
 * （单位单独一格，便于排版）；这里保留这个结构：数字按旧版口径取整到两位小数并加千分位，
 * 单位由调用方单独传回，渲染时另起一格。
 */
function formatMetric(value: number, locale: string, unit: string): { value: string; unit: string } {
  const text = Number.isInteger(value) ? value.toLocaleString(locale) : String(Math.round(value * 100) / 100)
  return { value: text, unit }
}

interface MetricCardProps {
  title: ReactNode
  value: number
  /** 单位（如 `%`），单独渲染，不与数字混排。 */
  unit?: string
  icon: IconProps['icon']
  /** 使用率阈值配色；不传则按普通数值展示。 */
  tone?: TagTone
  /** 点击跳转（统计卡 → 对应管理页）。 */
  onClick?: () => void
}

/** 指标卡（旧版 NumbericDashboard.js：标题 + 数值 + 右上角图标，可点击跳转）。 */
function MetricCard({ title, value, unit = '', icon, tone, onClick }: MetricCardProps) {
  const { lang } = useI18n()
  const metric = formatMetric(value, lang === 'zh-cn' ? 'zh-CN' : 'en-US', unit)

  return (
    <Card
      hoverable={Boolean(onClick)}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      onClick={onClick}
      onKeyDown={
        onClick
          ? (event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                onClick()
              }
            }
          : undefined
      }
    >
      <div className="row flex-between">
        <span className="muted">{title}</span>
        <Icon icon={icon} size={24} />
      </div>
      <div className="repo-head__title">
        {tone ? <Tag tone={tone}>{metric.value}</Tag> : <span>{metric.value}</span>}
        {metric.unit ? <span className="muted">{metric.unit}</span> : null}
      </div>
    </Card>
  )
}

export default function AdminDashboard() {
  const t = useT()
  const toast = useToast()
  const navigate = useNavigate()
  const [status, setStatus] = useState<AdminSystemStatus | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    fetchSystemStatus()
      .then((data) => {
        if (!cancelled) {
          setStatus(data)
        }
      })
      .catch((error: unknown) => toast.error(error instanceof Error ? error.message : t('base_error_message')))
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [t, toast])

  if (loading) {
    return <Loading />
  }

  const cpu = status?.vm.cpu ?? 0
  const memory = status?.vm.memory ?? 0
  const disks = status?.vm.disk ?? []

  return (
    <div className="page">
      <section className="section">
        <div className="section__title">{t('label.statistic')}</div>
        <div className="group-grid">
          <MetricCard
            title={t('label.user_pl')}
            value={status?.usage.user ?? 0}
            icon={psMembers}
            onClick={() => navigate('/admin/users')}
          />
          <MetricCard
            title={t('label.group_pl')}
            value={status?.usage.group ?? 0}
            icon={psRepositoryGroup}
            onClick={() => navigate('/admin/groups')}
          />
          <MetricCard
            title={t('label.repository_pl')}
            value={status?.usage.repository ?? 0}
            icon={psRepository}
            onClick={() => navigate('/admin/repositories')}
          />
        </div>
      </section>

      <section className="section">
        <div className="section__title">{t('label.systemResources')}</div>
        <div className="group-grid">
          <MetricCard title="CPU" value={cpu} unit="%" icon={psMeter} tone={usageTone(cpu)} />
          <MetricCard title="RAM" value={memory} unit="%" icon={psMeter} tone={usageTone(memory)} />
        </div>
      </section>

      {disks.length > 0 ? (
        <section className="section">
          <div className="section__title">{t('label.diskUsgae')}</div>
          <div className="group-grid">
            {disks.map((disk) => (
              <MetricCard
                key={disk.name}
                title={disk.name}
                value={disk.usage}
                unit="%"
                icon={psFolder}
                tone={usageTone(disk.usage)}
              />
            ))}
          </div>
        </section>
      ) : null}

      <section className="section">
        <div className="section__title">{t('label.serviceStatus')}</div>
        <div className="group-grid">
          {(['codefever', 'nginx', 'sendmail'] as const).map((name) => (
            <Card key={name}>
              <div className="row flex-between">
                <span className="muted">{name}</span>
                <Icon icon={psNewFeature} size={24} />
              </div>
              <div className="repo-head__title">
                <Tag tone={status?.service[name] ? 'success' : 'danger'}>
                  {status?.service[name] ? t('label.running') : t('label.stopped')}
                </Tag>
              </div>
            </Card>
          ))}
        </div>
      </section>
    </div>
  )
}
