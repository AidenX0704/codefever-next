import { cn } from '@/helpers/cn'


/** `@pgyer/icons` 图标数据形态（由 `scripts/extract-icons.mjs` 生成）。 */
export interface IconData {
  readonly name: string
  readonly width: number
  readonly height: number
  readonly paths: readonly [string, string]
}

export interface IconProps {
  icon: IconData
  /** 视觉尺寸（px）；默认与旧版页头/行内图标一致的 18px。 */
  size?: number
  className?: string
  title?: string
}

/**
 * 图标渲染器。
 *
 * 旧版依赖 `@fortawesome/react-fontawesome` 渲染，并把 pgyer 的原始数组包装成
 * FontAwesome 的 icon 结构（`helpers/pgyerIcon.js`）。新前端不需要这套间接层：
 * 直接把路径数据渲染成 `<svg viewBox="0 0 w h"><path d/></svg>`，
 * 颜色用 `currentColor`，所以继承文字颜色（hover 变色不需要额外交互逻辑）。
 *
 * 注意：**不要**从 `assets/icons` 建一个 barrel 全量导入 —— 那会把 154 个图标
 * 全部打进产物（旧版就踩过这个坑，见 AGENTS §9）。
 */
export function Icon({ icon, size = 18, className, title }: IconProps) {
  return (
    <svg
      className={cn('cf-icon', className)}
      width={size}
      height={size}
      viewBox={`0 0 ${icon.width} ${icon.height}`}
      fill="currentColor"
      role={title ? 'img' : undefined}
      aria-hidden={title ? undefined : true}
      focusable="false"
    >
      {title ? <title>{title}</title> : null}
      {icon.paths[0] ? <path d={icon.paths[0]} /> : null}
      {icon.paths[1] ? <path d={icon.paths[1]} /> : null}
    </svg>
  )
}
