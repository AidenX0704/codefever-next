import { format, formatRelative } from 'date-fns'
import { enUS, zhCN } from 'date-fns/locale'

import { useI18n } from '@/i18n'

/**
 * 时间显示（相对/绝对自动切换）。
 *
 * 逐字复刻旧版 `components/unit/FormattedTime.js` 的判定与格式：
 *   - 与当前时间差 < 7 天 → 相对时间（`formatRelative`）；
 *   - 否则 → 绝对时间 `yyyy-MM-dd HH:mm:ss`；
 *   - 旧版把相对时间里的 `p`（12 小时制）强制替换为 `H:mm`（24 小时制），
 *     英文环境下这里同样把 `hh:mm AM/PM` 转成 24 小时，保证文案与旧版一致；
 *   - 时间戳是"秒"还是"毫秒"按旧版阈值判断（`< 31536E6` 视为秒）。
 */
export interface FormattedTimeProps {
  /** 后端返回的 Unix 秒级时间戳。 */
  timestamp: number
  absolute?: boolean
  relative?: boolean
}

const WEEK_MS = 6048e5
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

export function FormattedTime({ timestamp, absolute, relative }: FormattedTimeProps) {
  const { lang } = useI18n()
  const locale = lang === 'zh-cn' ? zhCN : enUS

  const ms = timestamp * (timestamp < YEAR_1971_SECONDS ? 1000 : 1)
  const now = Date.now()

  let useAbsolute = absolute ?? false
  let useRelative = relative ?? false
  if (!useAbsolute && !useRelative) {
    if (Math.abs(ms - now) < WEEK_MS) {
      useRelative = true
    } else {
      useAbsolute = true
    }
  }

  let text = ''
  if (useAbsolute && ms > 0) {
    text = format(ms, 'yyyy-MM-dd HH:mm:ss', { locale })
  } else if (useRelative && ms > 0) {
    const relativeText = formatRelative(ms, now, { locale })
    text = lang === 'zh-cn' ? relativeText : to24Hour(relativeText)
  }

  return <span>{text}</span>
}
