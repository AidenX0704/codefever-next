/**
 * 全局 UI 开关。
 *
 * `tooltipsEnabled`：悬浮提示（tooltip）总开关。
 * 2026-10 按产品要求**全局关闭**——页头图标按钮、图表说明等处不再弹气泡；
 * 组件本体保留（`Tooltip` 仍接收参数），把这里改回 `true` 即可整体恢复。
 * 注意：关闭时组件仍输出 `aria-label`，无障碍信息不丢失。
 */
export const UI_CONFIG = {
  tooltipsEnabled: false,
} as const

export type UIConfig = typeof UI_CONFIG
