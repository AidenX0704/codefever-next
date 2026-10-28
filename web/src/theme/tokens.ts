/**
 * 设计令牌的类型化引用。
 *
 * 约定（AGENTS §5）：
 *   - 字面值**只**存在于 `theme.css` 的原始层（--p-*）与尺寸层；
 *   - 组件与页面通过本模块取引用，禁止在 TSX/CSS 中写字面值；
 *   - 颜色只取语义层（--bg- / --fg- / --bd- / --ac- / --st- / …），
 *     不直接引用 --p-* 原始色板，否则深色模式会失效；
 *   - `space(n)` 等价旧代码 `theme.spacing(n)`（n × 8px）。
 *
 * 基线：Git Platform Design System V3。
 */

export const color = {
  /* 动作 */
  primary: 'var(--ac-primary)',
  primaryLight: 'var(--ac-primary-hover)',
  primaryDark: 'var(--ac-primary-active)',
  primaryLighter: 'var(--ac-primary-tint)',
  onPrimary: 'var(--ac-primary-fg)',

  /* 信息（旧 secondary 语义） */
  secondary: 'var(--st-info)',
  secondaryLight: 'var(--st-info)',
  secondaryDark: 'var(--st-info-text)',
  secondaryLighter: 'var(--st-info-tint)',

  /* 状态 */
  success: 'var(--st-success)',
  successLight: 'var(--st-success)',
  successDark: 'var(--st-success-text)',
  successLighter: 'var(--st-success-tint)',

  warning: 'var(--st-warning)',
  warningLight: 'var(--st-warning)',
  warningDark: 'var(--st-warning-text)',

  danger: 'var(--ac-danger)',
  dangerLight: 'var(--ac-danger-hover)',
  dangerDark: 'var(--ac-danger-active)',
  dangerLighter: 'var(--st-danger-tint)',

  /* 表面 */
  bg: 'var(--bg-surface)',
  bgMain: 'var(--bg-canvas)',
  bgDark: 'var(--bg-hover)',
  backdrop: 'var(--scrim)',

  /* 文字 / 描边 */
  border: 'var(--bd)',
  text: 'var(--fg)',
  textLight: 'var(--fg-muted)',
  textLighter: 'var(--fg-subtle)',

  /* diff */
  diffAddition: 'var(--diff-add-bg)',
  diffAdditionDark: 'var(--diff-add-gutter)',
  diffAdditionDarker: 'var(--diff-add-word)',
  diffDeletion: 'var(--diff-del-bg)',
  diffDeletionDark: 'var(--diff-del-gutter)',
  diffDeletionDarker: 'var(--diff-del-word)',
  diffLabel: 'var(--diff-hunk-bg)',
  diffLabelDark: 'var(--diff-hunk-bg)',
} as const

/** 表面层级：卡片 / 表头 / 浮层 / 悬停 / 选中，按用途取，不按颜色取。 */
export const surface = {
  canvas: 'var(--bg-canvas)',
  base: 'var(--bg-surface)',
  sunken: 'var(--bg-sunken)',
  raised: 'var(--bg-raised)',
  overlay: 'var(--bg-overlay)',
  hover: 'var(--bg-hover)',
  active: 'var(--bg-active)',
  selected: 'var(--bg-selected)',
  selectedHover: 'var(--bg-selected-hover)',
  inverse: 'var(--bg-inverse)',
} as const

/** 文字层级。fg-subtle 只用于占位符 / 禁用态，不得承载正文。 */
export const fg = {
  strong: 'var(--fg-strong)',
  base: 'var(--fg)',
  muted: 'var(--fg-muted)',
  subtle: 'var(--fg-subtle)',
  disabled: 'var(--fg-disabled)',
  inverse: 'var(--fg-inverse)',
  link: 'var(--fg-link)',
} as const

/** 描边层级。 */
export const border = {
  subtle: 'var(--bd-subtle)',
  base: 'var(--bd)',
  strong: 'var(--bd-strong)',
  focus: 'var(--bd-focus)',
} as const

/** 状态四件套：solid 只做圆点/图标，文字一律用 text 压 tint。 */
export const status = {
  success: 'var(--st-success)',
  successText: 'var(--st-success-text)',
  successTint: 'var(--st-success-tint)',
  successBorder: 'var(--st-success-border)',
  warning: 'var(--st-warning)',
  warningText: 'var(--st-warning-text)',
  warningTint: 'var(--st-warning-tint)',
  warningBorder: 'var(--st-warning-border)',
  danger: 'var(--st-danger)',
  dangerText: 'var(--st-danger-text)',
  dangerTint: 'var(--st-danger-tint)',
  dangerBorder: 'var(--st-danger-border)',
  info: 'var(--st-info)',
  infoText: 'var(--st-info-text)',
  infoTint: 'var(--st-info-tint)',
  infoBorder: 'var(--st-info-border)',
  merged: 'var(--st-merged)',
  mergedText: 'var(--st-merged-text)',
  mergedTint: 'var(--st-merged-tint)',
  mergedBorder: 'var(--st-merged-border)',
} as const

export const font = {
  family: 'var(--font-sans)',
  familyMono: 'var(--font-mono)',
  size: 'var(--fs-14)',
  sizeH1: 'var(--fs-20)',
  sizeH4: 'var(--fs-16)',
  sizeSmall: 'var(--fs-12)',
  weightNormal: 'var(--fw-regular)',
  weightMedium: 'var(--fw-medium)',
  weightSemibold: 'var(--fw-semibold)',
  weightBold: 'var(--fw-bold)',
  lineHeight: 'var(--lh-normal)',
  lineHeightTight: 'var(--lh-tight)',
  lineHeightProse: 'var(--lh-prose)',
} as const

export const radius = {
  base: 'var(--r-md)',
  sm: 'var(--r-xs)',
  lg: 'var(--r-lg)',
  xl: 'var(--r-xl)',
  pill: 'var(--r-full)',
} as const

/** 阴影只给真正浮起的层；列表与卡片用 1px 边线分层。 */
export const shadow = {
  default: 'var(--shadow-2)',
  card: 'var(--shadow-1)',
  cardHover: 'var(--shadow-2)',
  bar: 'var(--shadow-3)',
  dialog: 'var(--shadow-4)',
} as const

export const size = {
  controlHeight: 'var(--ctl-h)',
  controlHeightSm: 'var(--ctl-h-sm)',
  controlHeightLg: 'var(--ctl-h-lg)',
  controlMinWidth: 'var(--ctl-min-w)',
  controlMinWidthSm: 'var(--ctl-min-w-sm)',
  tableRowHeight: 'var(--row-h)',
  tableCellPaddingX: 'var(--cell-px)',
  tabHeight: 'var(--tab-h)',
  tabPaddingX: 'var(--tab-px)',
  sidebarWidth: 'var(--layout-side)',
  headerHeight: 'var(--layout-header)',
  contentMaxWidth: 'var(--layout-content-max)',
  proseMaxWidth: 'var(--layout-prose-max)',
  dialogMaxWidth: 'var(--dialog-max-w)',
  emptyImageWidth: 'var(--empty-image-w)',
  emptyTextMaxWidth: 'var(--empty-text-max-w)',
} as const

/** 动效：上限 220ms，禁止 transition: all。 */
export const motion = {
  instant: 'var(--dur-instant)',
  fast: 'var(--dur-fast)',
  base: 'var(--dur)',
  slow: 'var(--dur-slow)',
  ease: 'var(--ease-standard)',
  easeOut: 'var(--ease-out)',
} as const

/** 层级：禁止组件自写 z-index。 */
export const z = {
  sticky: 'var(--z-sticky)',
  header: 'var(--z-header)',
  dropdown: 'var(--z-dropdown)',
  drawer: 'var(--z-drawer)',
  overlay: 'var(--z-overlay)',
  dialog: 'var(--z-dialog)',
  toast: 'var(--z-toast)',
  tooltip: 'var(--z-tooltip)',
} as const

/**
 * 等价旧代码的 `theme.spacing(n)`：`spacing(2)` → `calc(var(--sp-4) * 2)`（16px）。
 * 8px 基数保持不变，现有调用零改动。
 */
export function space(n: number): string {
  return `calc(var(--sp-4) * ${n})`
}

/** 表格密度：本期单档（行高 42px）。 */
export const density = {
  row: size.tableRowHeight,
} as const
