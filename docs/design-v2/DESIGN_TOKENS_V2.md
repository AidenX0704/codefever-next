# CodeFever 设计令牌 v2（提案）

> ⚠️ **已废弃 2026-10-28：被《Git Platform Design System V3》基线取代。**
> 现行基线见 `docs/design-v3/DESIGN_TOKENS_V3.md` 与 `docs/design-v3/tokens-v3-prototype.html`。
> 本文件仅作历史留档（配色、控件高、行高等数值均已被 v3 覆盖）。

> 状态：**提案，未落地**。本文与 `tokens-v2-prototype.html` 配套；
> 原型里所有色值/尺寸都取自本文的令牌名，改本文即可，不需要再算一遍。
>
> 阅读顺序：§1 结论 → §2 现状诊断 → §3 架构 → §6 别名映射（迁移用）→ §7 AGENTS §5 改写稿。

---

## 1. 结论摘要

| 项 | v1（现状） | v2（提案） |
|---|---|---|
| 结构 | 单层平铺，60+ 个 `--cf-color-*` 混着原始值与用途 | 原始层 → 语义层 → 组件层，三层单向依赖 |
| 深色模式 | 无 | `[data-theme="dark"]` 只重绑语义层 |
| 表面层级 | `bg` / `bg-main` / `bg-dark` 三个，语义含糊 | canvas / surface / sunken / raised / overlay + hover / active / selected |
| 字号 | 4 档，标题一律 700 | 9 档阶梯，标题 600，行高 4 级 |
| 间距 | 只有 8px 与 4px 两个基数 | 12 级阶梯 2→64px |
| 圆角 | 4px 通吃 | 2 / 4 / 6 / 8 / 12 / full，按角色分配 |
| 阴影 | 5 条 MUI elevation 原值 | 4 档，只给浮层；列表改用 1px 边线 |
| 状态色 | 只有 solid | solid / tint / text / border 四件套 |
| 焦点环 | 只有宽度，没有颜色 | 宽度 + 偏移 + 颜色 + halo |
| 语法高亮 | 散落在组件里 | `--tok-*` 8 个角色，浅深双模 |
| 动效 | 无令牌 | 3 档时长 + 2 条曲线 |
| 密度 | 固定 40px 行高 | 标准 / 紧凑两档，一套开关全局生效 |

**一句话**：v1 是"把上一版 MUI 主题抄下来"，v2 是"定义规则，让页面不用再各写一遍"。

---

## 2. 现状诊断（v1 的四个结构性问题）

引用 `web/src/theme/theme.css` 现状：

1. **原始值和用途混在一起。**
   `--cf-color-primary: #3455db` 既是"品牌蓝"这个原始事实，也是"主按钮底色"这个用途。
   一旦要换品牌色，无法判断哪些引用该跟着变。

2. **语义含糊导致重复定义。**
   `--cf-color-bg-main` 同时用于"页面底""表头""抽屉""手风琴标题"——四种不同层级的表面共用一个值。
   结果是想给表头换个更浅的底，就得动全站页面底色。

3. **深色模式无处落脚。**
   所有值写死为浅色。要加深色，只能逐个组件写 `@media` 覆盖，等于重写一遍样式表。

4. **组件内部量泄漏到全局。**
   `--cf-btn-padding`、`--cf-dialog-title-padding`、`--cf-auth-form-width` 这类"某个组件内部的尺寸"
   与全局设计令牌平级。它们不应该出现在组件层之外，也不应该被页面引用。

另外两处历史遗留，v2 明确收口：

- **两套主色**：应用内 `#3455db`，认证页 `#0035ec`（`--cf-color-auth-primary`）。
  v2 统一为一个动作色；认证页只允许在"品牌展示区"用更饱和的渐变，不再另立一套主色。
- **注释即文档**：v1 大量注释在解释"这个值抄自哪个文件的哪一行"。
  v2 的注释解释"这个令牌的用途与约束"，出处移到 §6 的映射表。

---

## 3. 架构：三层，单向依赖

```
原始层 primitive     --p-*        只有这一层允许出现十六进制字面值
      ↓ 只能向下引用
语义层 semantic      --bg- --fg- --bd- --ac- --st- --tok- --diff- --chart- --shadow- --scrim
      ↓ 只能向下引用
组件层 component     --ctl- --row- --cell- --layout- --z- --dur- --r- --sp- --fs-
```

**约束（写进 AGENTS.md §5）**

1. 十六进制 / `rgb()` 字面值**只允许**出现在原始层。
2. 组件 CSS 与 TSX **只允许**引用语义层与尺寸层；引用原始层视为违规（例如 `background: var(--p-blue-600)` 不允许）。
3. 语义层按主题分组：`:root`（浅色）与 `:root[data-theme="dark"]`，两组**必须键名一一对应**，不允许只在一边存在。
4. 新增令牌必须同时给出：用途一句话 + 浅深两值（若为颜色）+ 是否参与密度压缩（若为尺寸）。

---

## 4. 令牌清单

### 4.1 原始层（不随主题变化）

| 组 | 令牌 | 值 |
|---|---|---|
| 中性 | `--p-n-0/25/50/100/150/200/300/400/500/600/700/800/900/950` | `#ffffff` → `#080e18` |
| 品牌蓝 | `--p-blue-50…900` | `#eef4ff` → `#122b63`，主用 `600 = #1f55db` |
| 绿 | `--p-green-50/100/300/500/600/700` | 成功 / 通过 / 已合并 |
| 琥珀 | `--p-amber-50/100/300/500/600/700` | 警告 / 待处理 |
| 红 | `--p-red-50/100/300/500/600/700` | 危险 / 删除 / 失败 |
| 紫 | `--p-violet-50/100/300/500/600/700` | 合并请求 / 草稿态 |
| 青 | `--p-cyan-500` | `#0e9bb8`，图表与次要信息 |

### 4.2 语义层 · 表面与文字

| 令牌 | 浅色 | 深色 | 用途 |
|---|---|---|---|
| `--bg-canvas` | `#f7f9fb` | `#0a101c` | 页面底 |
| `--bg-surface` | `#ffffff` | `#101827` | 卡片、表格、侧栏 |
| `--bg-sunken` | `#eff2f6` | `#0c131f` | 表头、行号槽、代码 gutter |
| `--bg-raised` | `#ffffff` | `#16202f` | 浮起的卡片 |
| `--bg-overlay` | `#ffffff` | `#16202f` | 弹窗、菜单、消息条 |
| `--bg-hover` | `#eff2f6` | `#182233` | 行/项悬停 |
| `--bg-active` | `#e7ebf1` | `#1e2a3d` | 按下 |
| `--bg-selected` | `#eef4ff` | `#16233a` | 选中行 |
| `--bg-inverse` | `#111a28` | `#f7f9fb` | 反色块（tooltip） |
| `--fg-strong` | `#111a28` | `#f1f5f9` | 标题、关键数字 |
| `--fg` | `#343f51` | `#c4d0e0` | 正文 |
| `--fg-muted` | `#626e83` | `#8fa0b8` | 次要说明 |
| `--fg-subtle` | `#9aa5b5` | `#64748b` | 占位符、禁用态（**不用于正文**） |
| `--fg-link` | `#1743b0` | `#8cb1ff` | 链接 |
| `--bd-subtle` | `#e7ebf1` | `#1b2434` | 表格行分隔 |
| `--bd` | `#dde3eb` | `#27334a` | 默认描边 |
| `--bd-strong` | `#c7cfda` | `#3a4a66` | 输入框、需强调的边 |

### 4.3 语义层 · 动作与状态

| 令牌 | 浅色 | 深色 | 用途 |
|---|---|---|---|
| `--ac-primary` | `#1f55db` | `#6e9bff` | 主按钮、选中态 |
| `--ac-primary-hover` | `#1743b0` | `#8cb1ff` | 悬停 |
| `--ac-primary-active` | `#143686` | `#a6c4ff` | 按下 |
| `--ac-primary-fg` | `#ffffff` | `#0a101c` | 主按钮上的文字 |
| `--ac-primary-tint` | `#eef4ff` | `rgba(110,155,255,.13)` | 浅底（标签、选中） |
| `--ac-primary-border` | `#bfd4fd` | `rgba(110,155,255,.32)` | 浅底的边 |
| `--ac-danger` | `#c62a2f` | `#f2555a` | 危险按钮 |
| `--ac-danger-tint` | `#fdeded` | `rgba(242,85,90,.14)` | 危险浅底 |
| `--st-success` / `-text` / `-tint` / `-border` | `#15803d` / `#116b37` / `#e9f8ee` / `#8dd7a6` | `#3fb950` / `#56d364` / `rgba(63,185,80,.14)` / `rgba(63,185,80,.34)` | 成功 |
| `--st-warning` / `-text` / `-tint` / `-border` | `#b4740a` / `#8a5708` / `#fef5e7` / `#f0d8a6` | `#d29922` / `#e3b341` / `rgba(210,153,34,.14)` / `rgba(210,153,34,.34)` | 警告 |
| `--st-danger` / `-text` / `-tint` / `-border` | `#c62a2f` / `#9e2024` / `#fdeded` / `#f2c4c4` | 同 `--ac-danger` 组 | 错误 |
| `--st-info` / `-text` / `-tint` / `-border` | `#1f55db` / `#1743b0` / `#eef4ff` / `#bfd4fd` | 同 `--ac-primary` 组 | 信息 |
| `--st-merged` / `-text` / `-tint` / `-border` | `#6b3fc4` / `#59339f` / `#f3eefe` / `#dcccf8` | `#a371f7` / `#c297ff` / `rgba(163,113,247,.14)` / `rgba(163,113,247,.34)` | 已合并 |
| `--focus-ring` | `#1f55db` | `#8cb1ff` | 键盘焦点 |
| `--focus-halo` | `rgba(31,85,219,.28)` | `rgba(140,177,255,.3)` | 输入框聚焦光晕 |
| `--scrim` | `rgba(17,26,40,.48)` | `rgba(0,0,0,.62)` | 遮罩 |

### 4.4 语义层 · 代码 / diff / 图表

| 组 | 令牌 |
|---|---|
| 代码表面 | `--code-bg` `--code-gutter-bg` `--code-gutter-fg` `--code-border` |
| diff | `--diff-add-bg` `--diff-add-gutter` `--diff-add-word` `--diff-del-bg` `--diff-del-gutter` `--diff-del-word` `--diff-hunk-bg` `--diff-hunk-fg` |
| 语法 | `--tok-kw` `--tok-str` `--tok-num` `--tok-fn` `--tok-com` `--tok-type` `--tok-var` `--tok-punct` |
| 图表 | `--chart-1…8`（蓝/青/琥珀/红/紫/绿/浅蓝/灰，按明度与色盲可辨排序） |

### 4.5 尺寸层（无主题差异）

| 组 | 令牌 |
|---|---|
| 间距 | `--sp-1…12` = 2 / 4 / 6 / 8 / 12 / 16 / 20 / 24 / 32 / 40 / 48 / 64；语义别名 `--sp-gutter`(16) `--sp-section`(32) |
| 圆角 | `--r-xs` 2 / `--r-sm` 4 / `--r-md` 6 / `--r-lg` 8 / `--r-xl` 12 / `--r-2xl` 16 / `--r-full` |
| 字号 | `--fs-11/12/13/14/16/18/20/24/30` |
| 字重 | `--fw-regular` 400 / `--fw-medium` 500 / `--fw-semibold` 600 / `--fw-bold` 700 |
| 行高 | `--lh-tight` 1.25 / `--lh-snug` 1.35 / `--lh-normal` 1.5 / `--lh-prose` 1.7 |
| 字体 | `--font-sans`（系统栈，中文优先）/ `--font-mono`（JetBrains Mono → Cascadia → Consolas） |
| 控件 | `--ctl-h-sm` 26 / `--ctl-h` 32 / `--ctl-h-lg` 36 / `--ctl-px` 12 / `--ctl-px-lg` 16 |
| 表格 | `--row-h` 40（紧凑 34）/ `--cell-px` 12（紧凑 10） |
| 图标 | `--icon-sm` 14 / `--icon` 16 / `--icon-lg` 20 / `--icon-xl` 24 |
| 焦点 | `--focus-w` 2 / `--focus-offset` 2 |
| 布局 | `--layout-side` 216 / `--layout-header` 48 / `--layout-content-max` 1440 / `--layout-prose-max` 720 |
| 动效 | `--dur-fast` 120ms / `--dur` 180ms / `--dur-slow` 260ms / `--ease` `cubic-bezier(.2,0,0,1)` / `--ease-out` `cubic-bezier(.05,.7,.1,1)` |
| 层级 | `--z-sticky` 10 / `--z-header` 100 / `--z-drawer` 200 / `--z-overlay` 1000 / `--z-dialog` 1100 / `--z-toast` 1200 / `--z-tooltip` 1300 |
| 壳层 | `--chrome-bg` `--chrome-fg` `--chrome-fg-muted` `--chrome-bd` `--chrome-hover` `--chrome-active` `--chrome-active-fg` |

### 4.6 密度档位

```css
:root[data-density="compact"] {
  --row-h: 34px;
  --ctl-h: 28px;
  --ctl-h-lg: 32px;
  --cell-px: 10px;
  --sp-section: var(--sp-8);
}
```

只压缩"重复出现的行/控件/区块间距"，不动字号与圆角——字号变小会先坏掉中文可读性。

---

## 5. 对比度实测（WCAG AA = 4.5:1 正文）

| 组合 | 比值 | 结论 |
|---|---|---|
| `--fg-strong` on `--bg-surface`（浅） | **17.47** | AAA |
| `--fg` on `--bg-surface`（浅） | **10.63** | AAA |
| `--fg-muted` on `--bg-surface`（浅） | **5.15** | AA ✓ |
| `--fg-muted` on `--bg-canvas`（浅） | **4.88** | AA ✓（**这是 v1 `#7a859e` 只有 3.4:1 的地方**） |
| `--fg-subtle` on `--bg-surface`（浅） | 2.49 | 仅占位符/禁用态 |
| 白字 on `--ac-primary` | **6.21** | AA ✓ |
| `--st-success-text` on `--st-success-tint` | **6.01** | AA ✓ |
| `--st-warning-text` on `--st-warning-tint` | **5.64** | AA ✓ |
| `--st-danger-text` on `--st-danger-tint` | **6.89** | AA ✓ |
| `--tok-com` on `--code-bg` | **6.00** | AA ✓ |
| 深色：`--fg` on `--bg-surface` | **11.38** | AAA |
| 深色：`--fg-muted` on `--bg-surface` | **6.67** | AA ✓ |
| 深色：`--ac-primary-fg` on `--ac-primary` | **7.06** | AA ✓ |

> v1 的 `--cf-color-text-light: #7a859e` 在 `--cf-color-bg-main: #f1f3f7` 上只有约 **3.4:1**，
> 这是现有界面"次要文字发灰、看不清"的真实原因。v2 把 `--fg-muted` 收到 `#626e83`。

---

## 6. 旧 → 新 别名映射（迁移期用）

第一步只换语义层，把旧变量名保留为别名，页面一行不改：

```css
:root {
  /* 兼容层：迁移期别名，逐页替换后删除 */
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
  --cf-space:                 var(--sp-4);
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
}
```

`space(n)` 的 TS 实现**不变**（仍是 `n × 8px`），`calc(var(--cf-space) * n)` 在别名下继续成立。

**组件内部量**（`--cf-btn-padding`、`--cf-dialog-*`、`--cf-auth-*` 等）不进 v2 的语义层：
它们随组件迁移一起搬到组件自己的 CSS 里（`components.css` 的 `.cf-btn` 区块），
或改用尺寸层令牌 + `calc()` 推导。这一步放在第三步做。

---

## 7. AGENTS.md §5 改写建议

> 用下面这段替换现有 §5 全文。改动点：把"只允许出现在 theme.css"升级为"三层 + 单向依赖"，
> 并把可验收的硬约束写进去。

```markdown
## 5. 设计语言：所有页面遵守同一套视觉规范

**要求**

1. 下列视觉维度必须全站一致：**颜色、字体、字号、间距、圆角、边框、阴影、
   表格密度、按钮样式、弹窗样式、表单样式、动效时长、焦点环**。

2. 本项目的事实标准：**自研设计令牌 + 自研组件库**（2026-10 前端重写后：
   不再使用任何 UI 框架，MUI 已随旧 `www/view` 一起删除）。

3. 令牌分三层，**依赖单向向下**：

   | 层 | 前缀 | 允许出现的位置 | 谁能引用 |
   |---|---|---|---|
   | 原始层 | `--p-*` | 只有 `web/src/theme/theme.css` 的原始层区块 | **任何组件/页面都不得直接引用** |
   | 语义层 | `--bg-` `--fg-` `--bd-` `--ac-` `--st-` `--tok-` `--diff-` `--chart-` `--shadow-` `--scrim` | 同上 | 组件与页面**唯一**可用的颜色来源 |
   | 尺寸层 | `--sp-` `--r-` `--fs-` `--fw-` `--lh-` `--ctl-` `--row-` `--cell-` `--layout-` `--dur-` `--ease` `--z-` | 同上 | 组件与页面唯一可用的尺寸来源 |

4. **禁止**硬编码视觉值：`#3455db`、`padding: 13px`、`border-radius: 5px`、
   自造阴影或表格行高。需要新值时：先判断能否用既有令牌组合（`calc()` 允许），
   不能则加到对应层，再在页面引用。

5. 语义层按主题成对定义：`:root`（浅色）与 `:root[data-theme="dark"]`。
   两组**必须键名一一对应**，不允许只在一边存在；新增颜色令牌时两个主题一起加。

6. 间距一律用 `--sp-*` 阶梯或 `space(n)`（`n × 8px`）；栅格用 `group-grid` /
   `FormGrid`；断点写在组件 CSS 的 `@media` 里并复用既有断点值。

7. 按钮、弹窗、表单、表格一律复用既有组件或既有变体，**不要**为单个页面
   定制一套外观。组件内部量（某个组件自己的 padding / 宽度）留在组件 CSS 里，
   不进全局令牌层。

8. 有明确外部来源的常量（第三方品牌色、代码高亮主题）允许例外，
   但必须集中定义并注释来源。

**验收自检**

- 把两个不相干页面的截图并排放，按钮高度、圆角、间距节奏、表头背景、
  弹窗标题字号应当看不出差异。
- 在 `<html>` 上加 `data-theme="dark"`，**不得出现任何看不清的文字或消失的边线**；
  出现即说明有组件引用了原始层或写了字面值。
- 键盘 Tab 走一遍当前页面，每个可聚焦元素都应有可见焦点环（`--focus-ring`）。
```

---

## 8. 迁移路径（三步，每步可独立回滚）

| 步 | 做什么 | 页面是否改动 | 验收 |
|---|---|---|---|
| **1** | `theme.css` 换成 v2 语义层 + §6 兼容别名 | 不改 | 全站配色变化，`typecheck`/`lint`/`build` 全绿；并排截图无结构差异 |
| **2** | 逐页把 `--cf-color-*` 换成 `--ac-*` / `--fg-*` / `--bd-*` 等；删掉对应的兼容别名 | 改 CSS 引用，不改 DOM | 每删一条别名前 `grep` 确认零引用 |
| **3** | 引入密度档位与深色开关；圆角 4→6、阴影降档；组件内部量下沉 | 组件 CSS | 三档视觉走查 + 键盘可达性走查 |

**不要跳步**：第 1 步和第 3 步混在一起时，"颜色变了"和"尺寸变了"两类问题会互相掩盖，
回滚也无法定位。

---

## 9. 采纳 / 未采纳 ui-ux-pro-max 的哪些建议

该技能给出的是**通用设计情报**（Swiss/Minimalism 风格、developer-tool 配色、中文排版配对、
119 条 UX 规则）。逐条取舍如下——凡未采纳的都写清原因，避免下次重复讨论。

| 技能建议 | 处理 | 理由 |
|---|---|---|
| 风格：Minimalism & Swiss（网格、无装饰、高对比、少阴影） | **采纳** | 与本项目"密集数据 + 自研组件"高度契合；也解释了为什么 v2 砍掉 5 档阴影 |
| developer-tool 配色：`#0F172A` 底 + `#22C55E` 强调 | **部分采纳** | 采纳其 slate 中性阶与 `#1E293B` 系墨色；**拒绝**把绿当强调色——本项目里绿色已被"CI 通过/已合并"占用，主按钮用绿会产生语义冲突 |
| 字体：JetBrains Mono + IBM Plex Sans | **部分采纳** | 等宽栈前置 `JetBrains Mono`（有则用，无则回落 Cascadia/Consolas）；正文**拒绝**引入 IBM Plex Sans 等 Web 字体——本项目要支持离线自托管部署，中文 Web 字体体积不可接受，改走系统栈 |
| 字体：Noto Sans SC | **未采纳** | 同上，中文 Web 字体动辄数 MB，且系统栈在 Windows/macOS 上已有等效字体 |
| 正文基准 16px | **未采纳（记录为偏离）** | 中文 14px 的视觉宽度约等于拉丁 16px；本项目是密集管理界面，16px 会让表格行高与信息密度失衡。**补偿**：长文页（文档/帮助）用 `--fs-16` + `--lh-prose` |
| 触控目标 44×44px | **未采纳（记录为偏离）** | 本项目主场景是桌面端键鼠操作，44px 会让表格密度腰斩。**补偿**：移动断点下控件高提到 36–40px，图标按钮加 `--icon-btn-hit` 扩展点击区 |
| 不用 emoji 当图标、用 SVG | **采纳** | 现有 `web/src/config/icons.ts` 已是按需 SVG，无需改动，仅写进验收清单 |
| hover 过渡 150–300ms | **采纳并细化** | 定为 120/180/260 三档，且"退出快于进入" |
| `prefers-reduced-motion` 必须尊重 | **采纳** | 写进令牌层，全局 `transition: none` |
| 焦点环可见（不移除 outline） | **采纳并补全** | v1 只有宽度没颜色，v2 给出 `--focus-ring` + `--focus-halo` + 偏移 |
| 表格移动端横向滚动而非破版 | **采纳** | 现有 `.table-wrap` 已具备 `overflow`，写进验收 |
| 深色模式对比度需单独验证 | **采纳** | §5 给出深色实测值；验收要求加 `data-theme="dark"` 走查 |
| 阴影"sharp / none" | **部分采纳** | 列表与卡片改为 1px 边线；浮层仍保留 4 档阴影——完全没有阴影的弹窗在中文密集界面上会"糊"在背景里 |
| 图表色板需色盲友好 | **采纳** | `--chart-1…8` 按明度分离排序，且不单靠颜色区分（图例 + 数值） |

---

## 10. 待你确认的三个决策点

1. **壳层配色**：原型右上角可切"白壳 / 墨壳"。墨壳（深色顶栏 + 白内容区）更像开发者工具，
   但与现有界面差异最大。默认建议：**先白壳上线，墨壳作为可选主题**。
2. **主色色相**：`#1f55db` 是 `#3455db` 的收敛版（色相几乎不变、饱和度略降）。
   若想更"新"，可换 `#2563eb`（更偏青）；若想更"稳"，可换 `#1e4bb8`。
3. **圆角**：4px → 6px 是肉眼可见的变化。若担心"变了样"，第一步可以先只换颜色，
   圆角留到第三步与密度一起改。

---

## 11. 相关文件

- 原型：`docs/design-v2/tokens-v2-prototype.html`（单文件，直接双击打开，无需构建）
- 现有令牌：`web/src/theme/theme.css`、`web/src/theme/tokens.ts`
- 组件预览页：`web/src/pages/DesignSystem.tsx`（路由 `/__design`）——v2 落地后应把该页作为验收载体
