# CodeFever 设计令牌 v3（落地基线）

> 基线文档：《Git Platform Design System V3》
> 状态：**已确认（2026-10-28），待落地**。按 §8 的 P1→P5 推进，本文即 P1 的唯一事实来源。
> 可视化：`docs/design-v3/tokens-v3-prototype.html`（双击打开，右上角可切主题 / 壳层 / 密度）
> 上一版提案（已废弃 2026-10-28：被 V3 基线取代）：`docs/design-v2/DESIGN_TOKENS_V2.md`

---

## 1. 结论

V3 文档可以直接作为本项目的设计基线，**不需要重写**。本次做的是三件它没做的事：

1. **对齐本仓库现状** —— V3 写的是通用规范，这里给出与现有 `web/src/theme/theme.css`
   （60+ 个 `--cf-*`、MUI v4 抄录值）的逐条别名映射，让 P1 可以"只换值不换名"。
2. **逐条对比度实测** —— V3 说"必须验证"，这里把 40+ 组组合真跑了一遍，并修掉 4 处不达标。
3. **补出可执行的部分** —— 令牌 CSS 全文、`formatDateTime()` 实现、CI 检查清单、五阶段验收。

**V3 原值需要修订 4 处**，其余全部照抄。修订明细见 §3。

### 已确认的决策（2026-10-28）

| # | 决策 | 结论 | 落地影响 |
|---|---|---|---|
| 1 | 主色 | ✅ 切到 `#2563eb` | 认证页 `#0035ec` 一并收口，全站单一动作色 |
| 2 | 控件 / 行高 | ✅ 34px / 42px | 列表与表单整体松 2px；截图基线在 P3 一次更新 |
| 3 | 紧凑密度 | ⛔ 本期不做 | 不定义 `--row-h-compact` / `--cell-px-compact`，不加 `data-density` 开关 |
| 4 | 壳层 | ✅ 只用白壳 | `--chrome-*` 保留（深色自动跟随），不做墨壳变体，不加 `data-chrome` 开关 |

> 这三条决定了令牌的最终形状：**少两套开关，少 4 个尺寸令牌**。
> 将来若要紧凑档，只需恢复 `--row-h-compact` / `--cell-px-compact` / `--ctl-h` / `--cell-px` 四个值并加回 `data-density` 块，不影响其他任何令牌。

---

## 2. 相对现有 v1 的变化

| 维度 | v1（现状 `theme.css`） | v3（本文） |
|---|---|---|
| 结构 | 单层平铺，原始值与用途混在一起 | 原始层 `--p-*` → 语义层 → 尺寸层，单向依赖 |
| 深色模式 | 无 | `[data-theme="dark"]` 重绑语义层，96 个键一一对应 |
| 主色 | `#3455db`（应用）/ `#0035ec`（认证页）两套 | 统一 `#2563eb`，认证页不再另立主色 |
| 表面层级 | `bg` / `bg-main` / `bg-dark` 三个 | canvas / surface / sunken / raised / overlay + hover / active / selected / selected-hover |
| 字号 | 4 档（12/14/16/20），标题一律 700 | 9 档；标题 600（深色 500），正文 400 |
| 间距 | 只有 8px 与 4px 两个基数 | 12 级阶梯 2→64px |
| 圆角 | 4px 通吃 | 2 / 4 / 6 / 8 / 12 / full，按角色分配 |
| 阴影 | 5 条 MUI elevation 原值 | 4 档（5% / 9% / 12% / 16%），只给浮层 |
| 状态色 | 只有 solid | solid / text / tint / border 四件套 |
| 控件高 | 32px | 34px（小 28，大 40） |
| 表格行高 | 40px | 42px |
| 焦点环 | 只有宽度，没有颜色 | `--focus-ring` + `--focus-halo` + 偏移，深浅各一套 |
| 语法高亮 | 散落在组件里 | `--tok-*` 8 个角色，浅深双模 |
| 动效 | 无令牌，各处各写 0.2s / 0.3s | 3 档 + 硬上限 220ms，`prefers-reduced-motion` 统一归零 |
| 时间显示 | 各页面自行格式化 | 全站唯一 `yyyy-MM-dd HH:mm:ss`，集中在 `formatDateTime()` |
| 数字 | 无约束 | `tabular-nums lining-nums`，数值列右对齐 |
| 密度 | 固定 40px 行高 | 单密度：控件 34 / 行高 42；紧凑档本期不做 |

---

## 3. 对比度审计：V3 原值需修 4 处

实测方法：WCAG 2.x 相对亮度公式；浅色基准 `--bg-surface: #ffffff`，深色基准 `--bg-surface: #111827`；
tint 类背景先按 alpha 合成到表面色再计算。

### 3.1 需要修的

| # | 项 | V3 原值 | 实测 | 修订 | 理由 |
|---|---|---|---|---|---|
| 1 | 深色 `--fg-subtle` | `#718096` | **4.42:1** | `#7b8a9f`（5.05:1） | 差 0.08 到 AA。占位符虽不承载正文，但输入框提示看不清直接影响填表 |
| 2 | 危险按钮 text 变体 | `--ac-danger` 压 `--ac-danger-tint` | **4.41:1** | 文字改用 `--st-danger-text`（5.91:1） | 把"动作色"与"状态文字色"分开后天然达标 |
| 3 | 实心绿承载白字 | `--st-success #16a34a` | **3.30:1** | 约束：实心状态色只做圆点 / 图标 / 进度条 | 状态实心块本就不该放文字；文字一律走 `--st-*-text` 压 tint |
| 4 | 边线 `--bd` / `--bd-strong` | `#e2e8f0` / `#cbd5e1` | **1.23 / 1.48** | 记录为**已知偏离** | WCAG 1.4.11 要求控件边界 3:1，但达标边线约需 `#767676`，密集表格会显脏；GitHub / Linear 同样不达标 |

**第 4 项的补偿措施（三条，缺一不可）**

1. 输入框 / 选择器 hover 时边框加深到 `--fg-subtle`，focus 时用 `--bd-focus` + 3px 光晕；
2. 复选框、开关的"选中"用实心填充而非只换边框；
3. 表格不靠边线区分行，靠 `--bg-hover` 与留白。

### 3.2 实测通过的（节选）

| 组合 | 比值 |
|---|---|
| 浅色 `--fg-strong` / `--fg` / `--fg-muted` on surface | 17.85 / 10.35 / 4.76 |
| 浅色 `--fg-muted` on canvas | 4.55 |
| 白字压 `--ac-primary` / hover / active | 5.17 / 6.70 / 8.72 |
| 白字压 `--ac-danger` | 4.83 |
| 状态文字压 tint（success/warning/danger/info/merged） | 6.81 / 6.84 / 5.91 / 8.01 / 6.48 |
| 语法高亮 8 色 on `--code-bg`（浅） | 4.55 – 9.90 |
| 深色 `--fg-strong` / `--fg` / `--fg-muted` | 14.68 / 10.71 / 6.67 |
| 深色 `--ac-primary-fg` 压 `--ac-primary` | 7.16 |
| 深色状态文字压 tint | 7.89 – 9.92 |
| 深色语法高亮 8 色 | 6.72 – 12.71 |

> `--fg-subtle`（浅色 `#94a3b8` = 2.56:1）**不是缺陷**：V3 明确规定它只用于占位符与禁用态，
> 不得承载正文。这条要写进验收清单，防止有人拿它当"浅一点的正文色"。

---

## 4. 令牌全文（P1 直接替换 `web/src/theme/theme.css`）

> 这是本文档的核心交付物。原型里的视觉与这份值完全一致；**改值只改这里**。
>
> **已落地时的细化（2026-10-28）**：浅色语义层实际写成了 `var(--p-*)` 引用
> （如 `--bg-canvas: var(--p-n-50)`），而不是下面示例里的字面值 —— 否则原始色阶
> 全是死令牌，改品牌色仍要动 88 处。深色语义层保留具体值（深色色阶不在浅色
> palette 上）。落地后的权威文件是 `web/src/theme/theme.css`，与下方示例仅此一处差异。
> 分文件落地时按 V3 §2 拆成 `tokens/primitives.css`、`semantic.css`、`semantic.dark.css`、
> `dimensions.css`、`aliases.css`，内容不变。

```css
/* =====================================================================
 * CodeFever 设计令牌 v3
 * 基线：Git Platform Design System V3
 * 约定：色值只允许出现在本文件的原始层；组件与页面只能引用语义层与尺寸层。
 * ===================================================================== */

/* ---------------- 1. 原始层 Primitives（不随主题变化） ---------------- */
:root {
  /* Neutral（slate） */
  --p-n-0:   #ffffff;
  --p-n-25:  #fcfcfd;
  --p-n-50:  #f8fafc;
  --p-n-100: #f1f5f9;
  --p-n-150: #e9eef5;
  --p-n-200: #e2e8f0;
  --p-n-300: #cbd5e1;
  --p-n-400: #94a3b8;
  --p-n-500: #64748b;
  --p-n-600: #475569;
  --p-n-700: #334155;
  --p-n-800: #1e293b;
  --p-n-900: #0f172a;
  --p-n-950: #080e18;

  /* Brand blue */
  --p-blue-50:  #eff6ff;
  --p-blue-100: #dbeafe;
  --p-blue-200: #bfdbfe;
  --p-blue-300: #93c5fd;
  --p-blue-400: #60a5fa;
  --p-blue-500: #3b82f6;
  --p-blue-600: #2563eb;
  --p-blue-700: #1d4ed8;
  --p-blue-800: #1e40af;
  --p-blue-900: #1e3a8a;

  /* Green: success / CI passed / merged */
  --p-green-50:  #f0fdf4;
  --p-green-100: #dcfce7;
  --p-green-300: #86efac;
  --p-green-500: #22c55e;
  --p-green-600: #16a34a;
  --p-green-700: #15803d;
  --p-green-800: #166534;

  /* Amber: warning / pending */
  --p-amber-50:  #fffbeb;
  --p-amber-100: #fef3c7;
  --p-amber-300: #fcd34d;
  --p-amber-500: #f59e0b;
  --p-amber-600: #d97706;
  --p-amber-700: #b45309;
  --p-amber-800: #92400e;

  /* Red: destructive / failed */
  --p-red-50:  #fef2f2;
  --p-red-100: #fee2e2;
  --p-red-200: #fecaca;
  --p-red-300: #fca5a5;
  --p-red-500: #ef4444;
  --p-red-600: #dc2626;
  --p-red-700: #b91c1c;

  /* Violet: merge request / review metadata */
  --p-violet-50:  #f5f3ff;
  --p-violet-100: #ede9fe;
  --p-violet-200: #ddd6fe;
  --p-violet-300: #c4b5fd;
  --p-violet-500: #8b5cf6;
  --p-violet-600: #7c3aed;
  --p-violet-700: #6d28d9;

  /* Teal / Cyan: code type + data viz only */
  --p-teal-700: #0f766e;
  --p-cyan-500: #06b6d4;
  --p-cyan-600: #0891b2;

  /* ---------------- 尺寸层（无主题差异） ---------------- */
  /* Spacing: 1 unit = 4px */
  --sp-1: 2px;   --sp-2: 4px;   --sp-3: 6px;   --sp-4: 8px;
  --sp-5: 12px;  --sp-6: 16px;  --sp-7: 20px;  --sp-8: 24px;
  --sp-9: 32px;  --sp-10: 40px; --sp-11: 48px; --sp-12: 64px;
  --sp-gutter: 24px;
  --sp-section: 32px;

  /* Radius */
  --r-xs: 2px; --r-sm: 4px; --r-md: 6px; --r-lg: 8px;
  --r-xl: 12px; --r-2xl: 16px; --r-full: 9999px;

  /* Typography */
  --fs-11: 11px; --fs-12: 12px; --fs-13: 13px; --fs-14: 14px;
  --fs-16: 16px; --fs-18: 18px; --fs-20: 20px; --fs-24: 24px; --fs-30: 30px;
  --fw-regular: 400; --fw-medium: 500; --fw-semibold: 600; --fw-bold: 700;
  --lh-tight: 1.25; --lh-snug: 1.35; --lh-normal: 1.5; --lh-prose: 1.7;

  /* System fonts: no network font dependency */
  --font-sans: -apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display",
    "PingFang SC", "苹方", "Noto Sans SC", "Microsoft YaHei", "微软雅黑",
    "Segoe UI", Roboto, sans-serif;
  --font-mono: ui-monospace, "SFMono-Regular", "SF Mono", "Cascadia Code",
    "JetBrains Mono", Consolas, "Liberation Mono", Menlo, monospace;

  /* Controls */
  --ctl-h-sm: 28px; --ctl-h: 34px; --ctl-h-lg: 40px;
  --ctl-px: 12px; --ctl-px-lg: 16px;
  --ctl-icon-size: 16px; --ctl-border-width: 1px;

  /* Table */
  --row-h: 42px;
  --cell-px: 12px;

  /* Icon */
  --icon-xs: 12px; --icon-sm: 14px; --icon: 16px;
  --icon-lg: 20px; --icon-xl: 24px; --icon-btn-hit: 32px;

  /* Focus */
  --focus-w: 2px; --focus-offset: 2px; --focus-ring: var(--bd-focus);

  /* Layout */
  --layout-side: 232px; --layout-side-collapsed: 64px;
  --layout-header: 56px; --layout-toolbar: 48px;
  --layout-content-max: 1600px; --layout-prose-max: 760px;
  --layout-page-px: 32px; --layout-page-py: 28px;

  /* Motion: 硬上限见 §7，任何新增动效不得超过 220ms */
  --dur-instant: 0ms; --dur-fast: 100ms; --dur: 160ms; --dur-slow: 220ms;
  --ease-standard: cubic-bezier(0.2, 0, 0, 1);
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);
  --ease-linear: linear;

  /* Z-index */
  --z-base: 0; --z-sticky: 10; --z-header: 100; --z-dropdown: 500;
  --z-drawer: 800; --z-overlay: 1000; --z-dialog: 1100; --z-toast: 1200; --z-tooltip: 1300;
}

/* ---------------- 2. 语义层 · 浅色 ---------------- */
:root,
:root[data-theme="light"] {
  color-scheme: light;

  --bg-canvas: #f8fafc;
  --bg-surface: #ffffff;
  --bg-sunken: #f1f5f9;
  --bg-raised: #ffffff;
  --bg-overlay: #ffffff;
  --bg-hover: #f1f5f9;
  --bg-active: #e9eef5;
  --bg-selected: #eff6ff;
  --bg-selected-hover: #dbeafe;
  --bg-inverse: #0f172a;

  --fg-strong: #0f172a;
  --fg: #334155;
  --fg-muted: #64748b;
  --fg-subtle: #94a3b8;   /* 仅占位符 / 禁用态，不得承载正文 */
  --fg-disabled: #94a3b8;
  --fg-inverse: #ffffff;
  --fg-link: #1d4ed8;

  --bd-subtle: #e9eef5;
  --bd: #e2e8f0;
  --bd-strong: #cbd5e1;
  --bd-focus: #2563eb;

  --ac-primary: #2563eb;
  --ac-primary-hover: #1d4ed8;
  --ac-primary-active: #1e40af;
  --ac-primary-fg: #ffffff;
  --ac-primary-tint: #eff6ff;
  --ac-primary-border: #bfdbfe;

  --ac-danger: #dc2626;
  --ac-danger-hover: #b91c1c;
  --ac-danger-active: #991b1b;
  --ac-danger-fg: #ffffff;
  --ac-danger-tint: #fef2f2;
  --ac-danger-border: #fecaca;

  --st-success: #16a34a;
  --st-success-text: #166534;
  --st-success-tint: #f0fdf4;
  --st-success-border: #bbf7d0;

  --st-warning: #b45309;
  --st-warning-text: #92400e;
  --st-warning-tint: #fffbeb;
  --st-warning-border: #fde68a;

  --st-danger: #dc2626;
  --st-danger-text: #b91c1c;
  --st-danger-tint: #fef2f2;
  --st-danger-border: #fecaca;

  --st-info: #1d4ed8;
  --st-info-text: #1e40af;
  --st-info-tint: #eff6ff;
  --st-info-border: #bfdbfe;

  --st-merged: #7c3aed;
  --st-merged-text: #6d28d9;
  --st-merged-tint: #f5f3ff;
  --st-merged-border: #ddd6fe;

  --code-bg: #f8fafc;
  --code-gutter-bg: #f1f5f9;
  --code-gutter-fg: #64748b;
  --code-border: #e2e8f0;
  --diff-add-bg: #f0fdf4;
  --diff-add-gutter: #dcfce7;
  --diff-add-word: #bbf7d0;
  --diff-del-bg: #fef2f2;
  --diff-del-gutter: #fee2e2;
  --diff-del-word: #fecaca;
  --diff-hunk-bg: #eff6ff;
  --diff-hunk-fg: #1e40af;

  --tok-kw: #7c3aed;
  --tok-str: #15803d;
  --tok-num: #b45309;
  --tok-fn: #1d4ed8;
  --tok-com: #64748b;
  --tok-type: #0f766e;
  --tok-var: #334155;
  --tok-punct: #475569;

  --chart-1: #2563eb;
  --chart-2: #0891b2;
  --chart-3: #d97706;
  --chart-4: #dc2626;
  --chart-5: #7c3aed;
  --chart-6: #16a34a;
  --chart-7: #60a5fa;
  --chart-8: #64748b;

  --scrim: rgb(15 23 42 / 42%);
  --shadow-1: 0 1px 2px rgb(15 23 42 / 5%);
  --shadow-2: 0 4px 12px rgb(15 23 42 / 9%);
  --shadow-3: 0 12px 28px rgb(15 23 42 / 12%);
  --shadow-4: 0 20px 48px rgb(15 23 42 / 16%);

  --chrome-bg: #ffffff;
  --chrome-fg: #0f172a;
  --chrome-fg-muted: #64748b;
  --chrome-bd: #e2e8f0;
  --chrome-hover: #f1f5f9;
  --chrome-active: #eff6ff;
  --chrome-active-fg: #1d4ed8;

  --focus-halo: rgb(37 99 235 / 18%);
  --theme-heading-weight: 600;
  --theme-body-weight: 400;
}

/* ---------------- 3. 语义层 · 深色（键名与浅色一一对应） ---------------- */
:root[data-theme="dark"] {
  color-scheme: dark;

  --bg-canvas: #0b1120;
  --bg-surface: #111827;
  --bg-sunken: #0f172a;
  --bg-raised: #172033;
  --bg-overlay: #182235;
  --bg-hover: #1b2638;
  --bg-active: #243249;
  --bg-selected: #172b49;
  --bg-selected-hover: #1d365a;
  --bg-inverse: #f8fafc;

  --fg-strong: #e5eaf2;
  --fg: #c0cad8;
  --fg-muted: #91a0b5;
  --fg-subtle: #7b8a9f;   /* v3 修订：V3 原值 #718096 只有 4.42:1 */
  --fg-disabled: #7b8a9f;
  --fg-inverse: #0b1120;
  --fg-link: #8ab4ff;

  --bd-subtle: #202b3d;
  --bd: #2a374b;
  --bd-strong: #3b4b63;
  --bd-focus: #8ab4ff;

  --ac-primary: #6d9eff;
  --ac-primary-hover: #8ab4ff;
  --ac-primary-active: #a6c6ff;
  --ac-primary-fg: #0b1120;
  --ac-primary-tint: rgb(109 158 255 / 13%);
  --ac-primary-border: rgb(109 158 255 / 34%);

  --ac-danger: #f87171;
  --ac-danger-hover: #fca5a5;
  --ac-danger-active: #fecaca;
  --ac-danger-fg: #2b0b0b;
  --ac-danger-tint: rgb(248 113 113 / 13%);
  --ac-danger-border: rgb(248 113 113 / 34%);

  --st-success: #4ade80;
  --st-success-text: #86efac;
  --st-success-tint: rgb(74 222 128 / 12%);
  --st-success-border: rgb(74 222 128 / 30%);

  --st-warning: #fbbf24;
  --st-warning-text: #fcd34d;
  --st-warning-tint: rgb(251 191 36 / 12%);
  --st-warning-border: rgb(251 191 36 / 30%);

  --st-danger: #f87171;
  --st-danger-text: #fca5a5;
  --st-danger-tint: rgb(248 113 113 / 12%);
  --st-danger-border: rgb(248 113 113 / 30%);

  --st-info: #8ab4ff;
  --st-info-text: #a6c6ff;
  --st-info-tint: rgb(138 180 255 / 12%);
  --st-info-border: rgb(138 180 255 / 30%);

  --st-merged: #b69cff;
  --st-merged-text: #c8b5ff;
  --st-merged-tint: rgb(182 156 255 / 12%);
  --st-merged-border: rgb(182 156 255 / 30%);

  --code-bg: #0f172a;
  --code-gutter-bg: #111c2e;
  --code-gutter-fg: #91a0b5;
  --code-border: #2a374b;
  --diff-add-bg: rgb(34 197 94 / 10%);
  --diff-add-gutter: rgb(34 197 94 / 17%);
  --diff-add-word: rgb(34 197 94 / 24%);
  --diff-del-bg: rgb(248 113 113 / 10%);
  --diff-del-gutter: rgb(248 113 113 / 17%);
  --diff-del-word: rgb(248 113 113 / 24%);
  --diff-hunk-bg: rgb(109 158 255 / 12%);
  --diff-hunk-fg: #a6c6ff;

  --tok-kw: #c4a7ff;
  --tok-str: #86efac;
  --tok-num: #fcd34d;
  --tok-fn: #8ab4ff;
  --tok-com: #91a0b5;
  --tok-type: #5eead4;
  --tok-var: #c0cad8;
  --tok-punct: #a9b6c8;

  --chart-1: #8ab4ff;
  --chart-2: #5eead4;
  --chart-3: #fbbf24;
  --chart-4: #f87171;
  --chart-5: #b69cff;
  --chart-6: #4ade80;
  --chart-7: #60a5fa;
  --chart-8: #91a0b5;

  --scrim: rgb(0 0 0 / 62%);
  --shadow-1: 0 1px 2px rgb(0 0 0 / 18%);
  --shadow-2: 0 4px 12px rgb(0 0 0 / 24%);
  --shadow-3: 0 12px 28px rgb(0 0 0 / 30%);
  --shadow-4: 0 20px 48px rgb(0 0 0 / 38%);

  --chrome-bg: #111827;
  --chrome-fg: #e5eaf2;
  --chrome-fg-muted: #91a0b5;
  --chrome-bd: #2a374b;
  --chrome-hover: #1b2638;
  --chrome-active: #172b49;
  --chrome-active-fg: #a6c6ff;

  --focus-halo: rgb(138 180 255 / 24%);
  --theme-heading-weight: 500;
  --theme-body-weight: 400;
}

/* ---------------- 4. 兼容别名（P5 删除） ---------------- */
:root {
  --cf-color-primary:         var(--ac-primary);
  --cf-color-primary-light:   var(--ac-primary-hover);
  --cf-color-primary-dark:    var(--ac-primary-active);
  --cf-color-primary-lighter: var(--ac-primary-tint);
  --cf-color-on-primary:      var(--ac-primary-fg);
  --cf-color-secondary:       var(--st-info);
  --cf-color-success:         var(--st-success);
  --cf-color-success-lighter: var(--st-success-tint);
  --cf-color-warning:         var(--st-warning);
  --cf-color-danger:          var(--ac-danger);
  --cf-color-danger-lighter:  var(--st-danger-tint);
  --cf-color-border:          var(--bd);
  --cf-color-text:            var(--fg);
  --cf-color-text-light:      var(--fg-muted);
  --cf-color-text-lighter:    var(--fg-subtle);
  --cf-color-bg:              var(--bg-surface);
  --cf-color-bg-main:         var(--bg-canvas);
  --cf-color-bg-dark:         var(--bg-hover);
  --cf-color-backdrop:        var(--scrim);
  --cf-radius:                var(--r-md);
  --cf-space:                 var(--sp-4);      /* 保持 space(n) = n × 8px 语义不变 */
  --cf-control-height:        var(--ctl-h);
  --cf-table-row-height:      var(--row-h);
  --cf-table-cell-padding-x:  var(--cell-px);
  --cf-sidebar-width:         var(--layout-side);
  --cf-header-height:         var(--layout-header);
  --cf-shadow-card:           var(--shadow-1);
  --cf-shadow-card-hover:     var(--shadow-2);
  --cf-shadow-bar:            var(--shadow-3);
  --cf-shadow-dialog:         var(--shadow-4);
  --cf-font-size:             var(--fs-14);
  --cf-font-size-small:       var(--fs-12);
  --cf-font-family:           var(--font-sans);
  --cf-font-family-mono:      var(--font-mono);
  --cf-focus-ring:            var(--focus-w);
}
```

**别名层的三条纪律**

1. 别名**只增不减**到 P5；P5 之前任何新代码禁止使用 `--cf-*`。
2. 删别名前必须 `grep` 确认零引用，一次删一批并单独 commit。
3. `--cf-space` 保持 `8px`（不是 `--sp-4` 的语义替换），因为 TS 侧 `space(n)` 是
   `calc(var(--cf-space) * n)`，改基数会让全站间距腰斩。

---

## 5. 时间与数字（V3 §5，新增硬约束）

**唯一展示格式：`yyyy-MM-dd HH:mm:ss`**，精确到秒。

```ts
// web/src/helpers/datetime.ts
const DATE_TIME_FORMATTER = new Intl.DateTimeFormat('sv-SE', {
  year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', second: '2-digit',
  hourCycle: 'h23',
});

/**
 * 全站统一时间显示：yyyy-MM-dd HH:mm:ss。
 * 输入可为 ISO 字符串、Unix 毫秒时间戳或 Date；无效值统一返回 '—'。
 * 时区策略必须与应用设置一致（如需固定时区，在此处统一配置 timeZone）。
 */
export function formatDateTime(value: string | number | Date | null | undefined): string {
  if (value == null || value === '') return '—';
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return '—';

  const parts = DATE_TIME_FORMATTER.formatToParts(date);
  const get = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((part) => part.type === type)?.value ?? '';

  return `${get('year')}-${get('month')}-${get('day')} ${get('hour')}:${get('minute')}:${get('second')}`;
}
```

配套基础样式（放 `styles/base/typography.css`）：

```css
:where(.numeric, time, .timestamp, .stat-value, .duration, .line-number) {
  font-variant-numeric: tabular-nums lining-nums;
}

:where(code, kbd, samp, .technical-id, .commit-sha, .branch-name, .file-path) {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums lining-nums;
}
```

> **现状提醒**：仓库里已有 `web/src/components/unit/FormattedTime`。P3 要做的不是新建，
> 而是把它收敛成唯一入口并改用 `formatDateTime()`，同时检查是否有页面仍在自行 `new Date().toLocaleString()`。

---

## 6. 组件状态矩阵（验收用）

每个组件必须覆盖的状态，缺一个都算未完成：

| 组件 | 必须覆盖 |
|---|---|
| 按钮 | default / hover / active / focus-visible / disabled / loading |
| 输入框 | default / hover / focus / error / disabled / readonly |
| 选择器 | 同上 + 展开态 + 键盘上下选择 |
| 复选框 / 开关 | 未选 / 已选 / 半选（仅复选）/ 禁用 / focus |
| 标签 | 7 种状态语义 × tint 样式；可交互标签需 hover / focus / pressed |
| 表格 | 默认 / hover / selected / focus / 排序中 / 加载 / 空 / 错误 / 无权限 |
| 弹窗 / 抽屉 | 进入 / 退出 / Esc / 遮罩点击 / 焦点返回 / 未保存拦截 |
| Toast | success / error / info × 自动消失 / 手动关闭 |
| 空状态 | 空数据 / 无权限 / 请求失败 / 加载中（Skeleton） |

原型 §5「组件状态矩阵」已把按钮、输入框、选择控件、状态徽标四组并排画出来，
P2 的验收就照着它逐格比对。

---

## 7. 动效硬上限（摘要）

| 场景 | 上限 |
|---|---|
| 按下 / 焦点 / 颜色反馈 | 0–100ms |
| Hover 进入 / 退出 | ≤100ms / ≤80ms |
| 菜单、Tooltip | ≤100ms |
| 弹窗、抽屉进入 / 退出 | ≤180ms / ≤140ms |
| 局部展开折叠、Toast | ≤160ms |
| **任意新增动效** | **绝对上限 220ms** |

禁止：`transition: all`、全局 `* { transition }`、按钮缩放弹跳、长距离滑入、连续脉冲、
以及任何"等动画播完才响应下一次操作"的实现。`prefers-reduced-motion: reduce` 下全部归零。

---

## 8. 落地计划

| 阶段 | 做什么 | 主要文件 | 验收 |
|---|---|---|---|
| **P1** | 令牌与样式入口：换 `theme.css`，加别名层，**不动 DOM** | `theme/theme.css`、`theme/tokens.ts` | 配色变化；`typecheck / lint / build` 全绿；并排截图无结构差异 |
| **P2** | 公共组件基线：按钮/输入/选择/表格/标签/弹窗/Toast 统一，升级 `/__design` 预览页 | `components/unit/*`、`styles/components.css` | 两个无关页面并排，控件高度、边框、字重、圆角一致 |
| **P3** | 布局与 Git 核心页：壳层、页面标题区、筛选栏、列表、详情、空态；时间格式统一 | `styles/layout.css`、`styles/pages*.css`、`helpers/datetime.ts` | 布局节奏一致；全站时间到秒 |
| **P4** | 交互与动效：开关、焦点、键盘、加载与取消；删阻塞型动画 | `unit/{Dialog,Toast,Dropdown}` | 动画期间可继续操作；关闭后无焦点残留 |
| **P5** | 清理兼容层 + 上 CI 静态检查 | `scripts/verify.ps1`、`.github/workflows/ci.yml` | 硬编码扫描 0 命中；浅深键一致；无 `--p-*` 外泄 |

每阶段一个 commit，可独立回滚。**P1 与 P3 不要合并**——颜色变了和尺寸变了混在一起，
出问题时无法定位是哪一类。

---

## 9. 建议进 CI 的静态检查

按拦截收益排序，先做前三条：

1. **业务 CSS 出现十六进制 / `rgb()`** —— 正则扫 `web/src/**/*.css`，白名单只有原始层文件。
2. **页面引用 `--p-*`** —— 扫 `var(--p-`，命中即失败。
3. **浅深语义键集合不一致** —— 解析两个主题块，diff 键名（原型 §07 已实现同样的逻辑，可直接搬）。
4. `transition: all` / 全局 `* { transition }` —— Stylelint 自定义规则。
5. 硬编码 `border-radius` / `box-shadow` / 控件高度 —— 同上。
6. 时间格式 —— `formatDateTime()` 单测：ISO / 时间戳 / 无效 / 补零 / 秒保留 / 时区一致。
7. 动效时长超 220ms —— 扫 `--dur-*` 与裸 `transition-duration`。

---

## 10. 决策记录

见 §1「已确认的决策」。本文所有令牌值均已按上述结论定稿，P1 直接照 §4 的 CSS 落地，无需再改值。

---

## 11. 相关文件

- 原型：`docs/design-v3/tokens-v3-prototype.html` —— 9 个章节、5 个页面 mockup（仓库列表 / 代码浏览 / 合并请求 / 工作台 / 流水线日志），可切浅色深色
- 待替换：`web/src/theme/theme.css`、`web/src/theme/tokens.ts`
- 组件预览页：`web/src/pages/DesignSystem.tsx`（路由 `/__design`），P2 起作为验收载体
- 时间入口（现状）：`web/src/components/unit/FormattedTime`
