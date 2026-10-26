/**
 * 设计令牌的类型化引用。
 *
 * 约定（AGENTS §5）：
 *   - 字面值**只**存在于 `theme.css` 的 `:root`；
 *   - 组件与页面通过本模块取引用，禁止在 TSX/CSS 中写字面值；
 *   - `space(n)` 等价旧代码 `theme.spacing(n)`（MUI v4 单位 8px）。
 */

export const color = {
  primary: 'var(--cf-color-primary)',
  primaryLight: 'var(--cf-color-primary-light)',
  primaryDark: 'var(--cf-color-primary-dark)',
  primaryLighter: 'var(--cf-color-primary-lighter)',
  onPrimary: 'var(--cf-color-on-primary)',

  secondary: 'var(--cf-color-secondary)',
  secondaryLight: 'var(--cf-color-secondary-light)',
  secondaryDark: 'var(--cf-color-secondary-dark)',
  secondaryLighter: 'var(--cf-color-secondary-lighter)',

  success: 'var(--cf-color-success)',
  successLight: 'var(--cf-color-success-light)',
  successDark: 'var(--cf-color-success-dark)',
  successLighter: 'var(--cf-color-success-lighter)',

  warning: 'var(--cf-color-warning)',
  warningLight: 'var(--cf-color-warning-light)',
  warningDark: 'var(--cf-color-warning-dark)',

  danger: 'var(--cf-color-danger)',
  dangerLight: 'var(--cf-color-danger-light)',
  dangerDark: 'var(--cf-color-danger-dark)',
  dangerLighter: 'var(--cf-color-danger-lighter)',

  border: 'var(--cf-color-border)',
  text: 'var(--cf-color-text)',
  textLight: 'var(--cf-color-text-light)',
  textLighter: 'var(--cf-color-text-lighter)',
  bg: 'var(--cf-color-bg)',
  bgMain: 'var(--cf-color-bg-main)',
  bgDark: 'var(--cf-color-bg-dark)',
  backdrop: 'var(--cf-color-backdrop)',

  diffAddition: 'var(--cf-color-diff-addition)',
  diffAdditionDark: 'var(--cf-color-diff-addition-dark)',
  diffAdditionDarker: 'var(--cf-color-diff-addition-darker)',
  diffDeletion: 'var(--cf-color-diff-deletion)',
  diffDeletionDark: 'var(--cf-color-diff-deletion-dark)',
  diffDeletionDarker: 'var(--cf-color-diff-deletion-darker)',
  diffLabel: 'var(--cf-color-diff-label)',
  diffLabelDark: 'var(--cf-color-diff-label-dark)',
} as const

export const font = {
  family: 'var(--cf-font-family)',
  familyMono: 'var(--cf-font-family-mono)',
  size: 'var(--cf-font-size)',
  sizeH1: 'var(--cf-font-size-h1)',
  sizeH4: 'var(--cf-font-size-h4)',
  sizeSmall: 'var(--cf-font-size-small)',
  weightNormal: 'var(--cf-font-weight-normal)',
  weightMedium: 'var(--cf-font-weight-medium)',
  weightBold: 'var(--cf-font-weight-bold)',
  lineHeight: 'var(--cf-line-height)',
  lineHeightTight: 'var(--cf-line-height-tight)',
} as const

export const radius = {
  base: 'var(--cf-radius)',
  sm: 'var(--cf-radius-sm)',
  pill: 'var(--cf-radius-pill)',
} as const

export const shadow = {
  default: 'var(--cf-shadow-default)',
  card: 'var(--cf-shadow-card)',
  cardHover: 'var(--cf-shadow-card-hover)',
  bar: 'var(--cf-shadow-bar)',
  dialog: 'var(--cf-shadow-dialog)',
} as const

export const size = {
  controlHeight: 'var(--cf-control-height)',
  controlHeightSm: 'var(--cf-control-height-sm)',
  controlMinWidth: 'var(--cf-control-min-width)',
  controlMinWidthSm: 'var(--cf-control-min-width-sm)',
  tableRowHeight: 'var(--cf-table-row-height)',
  tableCellPaddingX: 'var(--cf-table-cell-padding-x)',
  tabHeight: 'var(--cf-tab-height)',
  tabPaddingX: 'var(--cf-tab-padding-x)',
  sidebarWidth: 'var(--cf-sidebar-width)',
  headerHeight: 'var(--cf-header-height)',
  emptyImageWidth: 'var(--cf-empty-image-width)',
  emptyTextMaxWidth: 'var(--cf-empty-text-max-width)',
} as const

/**
 * 等价旧代码的 `theme.spacing(n)`：`spacing(2)` → `calc(var(--cf-space) * 2)`（16px）。
 */
export function space(n: number): string {
  return `calc(var(--cf-space) * ${n})`
}

/** 表格密度：旧版只有一档（行高 40px），保留为语义化常量供 DataTable 使用。 */
export const density = {
  row: size.tableRowHeight,
} as const
