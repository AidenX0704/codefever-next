/**
 * 设计令牌的类型化引用。
 *
 * 约定（AGENTS §5）：
 *   - 字面值**只**存在于 `theme.css`（原始层 / 尺寸层 / 代码高亮外部色板）；
 *   - 组件与页面通过本模块取引用，禁止在 TSX/CSS 中写字面值；
 *   - 颜色只取语义层（--bg- / --text- / --line / --brand- / --success- …），
 *     不直接引用 --p-* 原始色板，否则深色模式会失效；
 *   - `space(n)` 等价旧代码 `theme.spacing(n)`（n × 8px）。
 *
 * 基线：docs/design-v4/DESIGN.md（v3 → v4 命名映射见 theme.css 头部注记）。
 * 历史导出键名保持稳定（color.* 等），取值已重绑到 v4 语义键；
 * 无引用的导出组（surface / fg / status / font）已按 v4 语义重写。
 */

export const color = {
  /* 动作（品牌） */
  primary: 'var(--brand)',
  primaryLight: 'var(--brand-hover)',
  primaryDark: 'var(--brand-active)',
  primaryLighter: 'var(--brand-subtle)',
  onPrimary: 'var(--text-inverse)',

  /* 信息（v4 起并入品牌族） */
  secondary: 'var(--brand)',
  secondaryLight: 'var(--brand)',
  secondaryDark: 'var(--brand-text)',
  secondaryLighter: 'var(--brand-subtle)',

  /* 状态 */
  success: 'var(--success)',
  successLight: 'var(--success)',
  successDark: 'var(--success-text)',
  successLighter: 'var(--success-subtle)',

  warning: 'var(--warning)',
  warningLight: 'var(--warning)',
  warningDark: 'var(--warning-text)',
  warningLighter: 'var(--warning-subtle)',

  danger: 'var(--danger)',
  dangerLight: 'var(--danger-hover)',
  dangerDark: 'var(--danger-active)',
  dangerLighter: 'var(--danger-subtle)',

  /* 表面 */
  bg: 'var(--bg-canvas)',
  bgMain: 'var(--bg-canvas)',
  bgDark: 'var(--bg-subtle)',
  backdrop: 'var(--scrim)',

  /* 文字 / 描边 */
  border: 'var(--line)',
  text: 'var(--text-primary)',
  textLight: 'var(--text-secondary)',
  textLighter: 'var(--text-tertiary)',

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

/** 表面层级：DESIGN.md —— 画布（页/输入/浮层）、灰区块、选中行/标签底、反色块。 */
export const surface = {
  canvas: 'var(--bg-canvas)',
  subtle: 'var(--bg-subtle)',
  muted: 'var(--bg-muted)',
  inverse: 'var(--bg-inverse)',
} as const

/** 文字层级。text-tertiary 只用于占位符 / 元信息，不得承载正文。 */
export const fg = {
  primary: 'var(--text-primary)',
  secondary: 'var(--text-secondary)',
  tertiary: 'var(--text-tertiary)',
  disabled: 'var(--text-disabled)',
  inverse: 'var(--text-inverse)',
} as const

/** 描边：全站唯一分割线 --line；strong 供输入框 hover / 强分割。 */
export const border = {
  base: 'var(--line)',
  strong: 'var(--line-strong)',
  focus: 'var(--brand)',
} as const

/** 状态四件套：solid 只做圆点/图标，文字一律用 text 压 subtle（对比度注记见 theme.css）。 */
export const status = {
  success: 'var(--success)',
  successText: 'var(--success-text)',
  successSubtle: 'var(--success-subtle)',
  successBorder: 'var(--success-border)',
  warning: 'var(--warning)',
  warningText: 'var(--warning-text)',
  warningSubtle: 'var(--warning-subtle)',
  warningBorder: 'var(--warning-border)',
  danger: 'var(--danger)',
  dangerText: 'var(--danger-text)',
  dangerSubtle: 'var(--danger-subtle)',
  dangerBorder: 'var(--danger-border)',
  brand: 'var(--brand)',
  brandText: 'var(--brand-text)',
  brandSubtle: 'var(--brand-subtle)',
  brandBorder: 'var(--brand-border)',
  purple: 'var(--purple)',
  purpleText: 'var(--purple-text)',
  purpleSubtle: 'var(--purple-subtle)',
  purpleBorder: 'var(--purple-border)',
} as const

export const font = {
  family: 'var(--font-sans)',
  familyMono: 'var(--font-mono)',
  display: 'var(--fs-display)',
  h1: 'var(--fs-h1)',
  h2: 'var(--fs-h2)',
  h3: 'var(--fs-h3)',
  body: 'var(--fs-body)',
  small: 'var(--fs-small)',
  caption: 'var(--fs-caption)',
  code: 'var(--fs-code)',
  weightNormal: 'var(--fw-regular)',
  weightMedium: 'var(--fw-medium)',
  weightSemibold: 'var(--fw-semibold)',
  weightBold: 'var(--fw-bold)',
  lineHeight: 'var(--lh-normal)',
  lineHeightTight: 'var(--lh-tight)',
  lineHeightProse: 'var(--lh-prose)',
} as const

export const radius = {
  /** 控件（按钮/输入/标签）：6px（DESIGN.md --r-sm）。 */
  base: 'var(--r-sm)',
  sm: 'var(--r-sm)',
  /** 卡片 / 弹层：10px（DESIGN.md --r-md）。 */
  md: 'var(--r-md)',
  lg: 'var(--r-md)',
  /** 宣传页大块：16px（DESIGN.md --r-lg）。 */
  xl: 'var(--r-lg)',
  pill: 'var(--r-full)',
} as const

/**
 * DESIGN.md 三档层级：L0 内容无阴影，L1 用 1px 边线，L2（shadow-2）只给浮层。
 * card / cardHover 导出为 none：卡片是 L0（--bg-subtle 底 + hover 换 --bg-muted），
 * 阴影只会出现在 dropdown / dialog / drawer / toast 上。
 */
export const shadow = {
  default: 'var(--shadow-2)',
  card: 'none',
  cardHover: 'none',
  bar: 'var(--shadow-3)',
  dialog: 'var(--shadow-2)',
} as const

export const size = {
  controlHeight: 'var(--ctl-h)',
  controlHeightSm: 'var(--ctl-h-sm)',
  controlHeightLg: 'var(--ctl-h-lg)',
  controlMinWidth: 'var(--ctl-min-w)',
  controlMinWidthSm: 'var(--ctl-min-w-sm)',
  tableRowHeight: 'var(--row-h)',
  tableRowHeightCompact: 'var(--row-h-compact)',
  tableCellPaddingX: 'var(--cell-px)',
  tabHeight: 'var(--tab-h)',
  tabPaddingX: 'var(--tab-px)',
  sidebarWidth: 'var(--layout-side)',
  headerHeight: 'var(--layout-header)',
  contentMaxWidth: 'var(--layout-content-max)',
  codeMaxWidth: 'var(--layout-code-max)',
  proseMaxWidth: 'var(--layout-prose-max)',
  dialogMaxWidth: 'var(--dialog-max-w)',
  emptyImageWidth: 'var(--empty-image-w)',
  emptyTextMaxWidth: 'var(--empty-text-max-w)',
} as const

/** 动效：仅 opacity/transform，120–200ms，ease-out；禁止 transition: all。 */
export const motion = {
  instant: 'var(--dur-instant)',
  fast: 'var(--dur-fast)',
  base: 'var(--dur)',
  slow: 'var(--dur-slow)',
  ease: 'var(--ease-standard)',
  easeOut: 'var(--ease-out)',
} as const

/** 层级：DESIGN.md §3.4 为基准；drawer/tooltip/overlay 为实现扩展。 */
export const z = {
  sticky: 'var(--z-sticky)',
  header: 'var(--z-header)',
  dropdown: 'var(--z-dropdown)',
  overlay: 'var(--z-overlay)',
  dialog: 'var(--z-dialog)',
  drawer: 'var(--z-drawer)',
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

/** 列表密度：舒适 56 / 紧凑 40（DESIGN.md §6.5）。 */
export const density = {
  row: size.tableRowHeight,
  rowCompact: size.tableRowHeightCompact,
} as const
