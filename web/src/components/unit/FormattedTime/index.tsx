import { format, formatRelative } from 'date-fns'
import { enUS, zhCN } from 'date-fns/locale'

import { useI18n } from '@/i18n'

/**
 * 时间显示。
 *
 * 规范（AGENTS §5.9 / V3 §5.2）：
 *   - UI 上的**绝对时间只有一种格式**：`yyyy-MM-dd HH:mm:ss`（到秒）；
 *   - **相对时间不得替代主时间字段** —— 它只能作为辅助信息。
 *
 * 因此默认走绝对格式；需要"3 分钟前"这种更易读的辅助文案时传 `relative`，
 * 此时仍会通过 `title` 与 `<time datetime>` 保留精确值，鼠标悬停即可看到。
 *
 * 历史：旧版是「7 天内显示相对、超过显示绝对」的自动切换，导致同一个列表里
 * 两种时间口径混排，且相对时间看不到秒 —— 2026-10 按 v3 规范改为绝对优先。
 *
 * 时间戳是"秒"还是"毫秒"按旧版阈值判断（`< 31536E6` 视为秒）。
 */
export interface FormattedTimeProps {
  /** 后端返回的 Unix 秒级时间戳。 */
  timestamp: number
  /** 强制绝对格式（默认即绝对，保留该参数是为了兼容既有调用点）。 */
  absolute?: boolean
  /** 改为显示相对时间作为**辅助文案**，精确值放进 title。 */
  relative?: boolean
}

const YEAR_1971_SECONDS = 31536e6

/** 把 date-fns 英文相对时间里的 12 小时制转成 24 小时制（对齐旧版 `p → H:mm`）。 */
function to24Hour(text: string): string {
  return text.replace(/(\d{1,2}):(\d{2})\s*(AM|PM)/i, (_match, hourText: string, minuteText: string, meridiem: string) => {
    let hour = Number(hourText) % 12
    if (meridiem.toUpperCase() === 'PM') {
      hour += 12
    }
    return `${String(hour).padStart(2, '0')}:${minuteText}`
  })
}

/** 统一占位符：无效时间显示 `—`，不出现 `Invalid Date`。 */
const PLACEHOLDER = '—'

export function FormattedTime({ timestamp, relative }: FormattedTimeProps) {
  const { lang } = useI18n()
  const locale = lang === 'zh-cn' ? zhCN : enUS

  const ms = timestamp * (timestamp < YEAR_1971_SECONDS ? 1000 : 1)
  const valid = Number.isFinite(ms) && ms > 0

  const absoluteText = valid ? format(ms, 'yyyy-MM-dd HH:mm:ss', { locale }) : PLACEHOLDER
  let text = absoluteText

  if (relative && valid) {
    const relativeText = formatRelative(ms, Date.now(), { locale })
    text = lang === 'zh-cn' ? relativeText : to24Hour(relativeText)
  }

  return (
    <time dateTime={valid ? new Date(ms).toISOString() : undefined} title={relative ? absoluteText : undefined}>
      {text}
    </time>
  )
}
