# DESIGN.md — 企业 Git 代码托管平台设计系统 · 格·代码（Cell Code）v1.0

> 适用范围：企业内部 Git 代码托管平台（基于 CodeFever 重写，产品名占位 `CODE FEVER Community`）的全部前端页面、公共组件与 AI Agent 生成的界面。
> 同源基线：**格·工作台（Cell Workbench）v5.6**——企业 AI 模型管理平台的设计系统。本文件是它在"代码托管"领域的**延伸版**：基线（Token、材质、外壳、通用组件、动效）**原值沿用，不得改动**；Git 领域的新增（代码、Diff、提交图、仓库与仓库组、合并请求、AI 问答首页、图标）在本文件定义。
> 一句话风格：**雾白底上的洁净纸页，固定主题蓝（#1677ff）负责行动；悬浮玻璃侧栏 + 通栏雾面顶栏；一次提交是一格，一个仓库是一格，一切读数皆由"方格"组成。**
> 本文件的数值是**确定值**，直接使用，不要"参考"或"接近"。

---

## 0. 同源关系与统一性守则

### 0.1 为什么要同源

公司内部的 AI 模型平台、Git 平台以及后续业务系统，用户会在它们之间频繁切换。**切换系统时，用户不应察觉"换了一套界面"**：侧栏在同一个位置、同一种玻璃质感；主按钮同一个蓝；表格、表单、弹窗、消息提示的手感一致。差异只应来自业务内容本身（代码、Diff、提交图），而不是来自视觉底座。

### 0.2 继承矩阵（改动任何"逐字继承"项，必须先改 Cell Workbench 主文件）

| 层级 | 内容 | 本文件的处理 |
| --- | --- | --- |
| **逐字继承** | 主题色 `#1677ff`、雾白 / 纸页 / 发丝线 / 前景 4 档、状态色、分类色、方格色阶、暗色中性黑、毛玻璃（顶栏 / 侧栏专用配方）、圆角档位、阴影、焦点、间距、字号、动效、z-index、断点 | §3 原值全量收录（自成一体，Agent 无需再翻基线文件） |
| **逐字继承** | 外壳（悬浮玻璃侧栏 + 通栏雾面顶栏 + 折叠按钮在侧栏底部）、`.sheet / .sheet--hero / .well / .frost / .veil / .halo`、Button / 表单 / Select / Dropdown / Pagination / StatusBadge / Tag / Tabs / Segmented / Steps / 浮层 / Alert / 全局 Message（顶部居中胶囊）/ CellLoader / Empty / Error / DataTable（表格自滚、表头容器内吸顶） | §4–§7 只写 Git 场景下的**用法与差异**，不重述通用 CSS 细节；通用细节以基线 §4、§6 为准 |
| **领域新增** | 代码高亮、Diff、提交图、贡献热力、语言条、仓库头像、仓库行 / 仓库组树、克隆面板、分支选择器、文件树 / 文件列表、合并检查清单、评审人列表、AI 问答面板、更新日志时间轴、图标体系 | §3.8、§6–§9 |
| **领域特有页面** | 未登录产品首页（G0）、登录后首页（G1）、仓库 / 仓库组 / 仓库详情 / 文件 / 提交 / 分支 / 合并请求 / Diff / 设置 / 管理 | §6 |

### 0.3 守则

1. **同一类事，只有一种做法。** 基线已有的组件，Git 页面直接用，不另做"Git 风格的按钮 / 表格 / 弹窗"。
2. **领域组件只用基线语汇搭建：** 纸页、方格、发丝线、主题蓝、状态徽标。圆点、环形仪表、单侧色条、叠页、渐变彩球一律不引入。
3. **版本对齐：** 本文件头部声明所依赖的基线版本（现为 v5.6）。基线升级后，本文件须在同一迭代内对齐并更新版本号；不得出现两个系统的 Token 值不一致。
4. **跨系统入口一致：** 顶栏头像菜单、主题切换、全局搜索 / ⌘K、全局 Message 位置与行为，在所有系统中保持完全一致。

---

## 1. 给 Agent 的硬规则（先读这一节）

### 1.1 继承自基线的 15 条（精要）

1. **只用 Token，不写字面量。** 颜色、圆角、阴影、间距、字号、时长一律引用 §3。页面里出现 `#hex`、自写 `box-shadow`、自写 `backdrop-filter` 视为缺陷。
2. **实现顺序：** `tokens.css` → `materials.css` → `cells.css` → `git-tokens.css`（§3.8）→ 基础组件 → 领域组件 → 页面。
3. **五个签名元素缺一不可：** ① 方格（Cell）母题；② 纸页（Sheet）+ 主角（Hero）；③ 主题色行动（固定 `#1677ff`，不换肤）；④ 左侧悬浮玻璃侧栏；⑤ 有目的的毛玻璃 + 健康度光晕。
4. **一个页面一个视觉主角**（一张表 / 一块 AI 面板 / 一个代码视图 / 一个 Diff），其余缩小减重。
5. **长时间阅读的内容放纸页：** 代码、Diff、README、表格、表单、日志永不使用半透明材质。
6. **颜色只在"需要注意"时出现：** 绿 / 琥珀 / 红只表示状态，且必须配文字或图标。
7. **数字一律 `tabular-nums`**，大读数收紧字距；单位小一档、`--fg-3`、基线对齐。
8. **每个数据区五种状态：** 加载 / 空 / 错误 / 无权限 / 正常。
9. **可读性与安全优先于好看。**
10. **需求与本文件冲突：** 先用现有变体解决；确需改基线，先改文件再改代码。
11. **一面一边一影：** 控件用真实 `1px border`、零 `box-shadow`；容器用 `box-shadow` 发丝环；**全站禁用单侧色条**（`border-left` / 左侧竖条表示状态或选中）。
12. **同级卡片之间必有 `--gap`**，间距只由容器 `gap` 提供，纸页自身 margin 恒为 0。
13. **角部必须干净：** 圆角容器 `overflow: clip`；光晕、网纹只画进裁切层；圆角 ≤ 16；内层圆角 = 外层 − 内边距。
14. **粘性层必须不透明**（表头、吸顶文件头、吸附列）。
15. **吸顶偏移只读 `--chrome-h`**；例外：表格表头只在表格容器内吸顶（`top: 0`）；Diff 文件头在其上叠加 `--diff-toolbar-h`（§6 G10）。

### 1.2 Git 领域新增 10 条

**G1. 精简优先，少线少影。** 仓库、提交、文件、评论这类**列表**一律用"行 + 横向发丝线"呈现，**不把每一条记录包成卡片**；卡片只用于"画廊视图"和少量概览。一屏可见的边界层级（边 / 影 / 底色变化）≤ 3 层。
**G2. 代码是长文本，永远在实色面上。** 代码块、文件视图、Diff 的底只能是 `--bg-sheet` 或 `--bg-sheet-2`；增删行用 §3.8 的实色 `--diff-*`，不用 `opacity`。
**G3. Diff 不画左侧色条。** 增 / 删行 = **整行底色 + 行号栏的 `+` / `−` 符号**；不得用 `border-left` 或竖条表达增删、选中、评论。
**G4. 节点即方格。** 提交图、分支图标、时间轴的节点是圆角小方格（边长 × 25% 圆角），不用圆点。例外仅有：用户头像（圆）、Message 倒计时圆环。
**G5. 用户圆，仓库方。** 用户头像 `--rad-full`；仓库 / 仓库组头像是圆角方块（`边长 × 25%`）。一眼区分"人"与"物"。
**G6. 图标成体系。** 全站图标只用 §8 的同一套线性图标族，禁止混用多个图标库、emoji、彩色渐变图标、位图小图标。
**G7. AI 只读你有权看的代码。** AI 问答的检索与引用范围 = 当前用户的仓库权限；回答必须带可点击的**来源**；AI 生成内容必须标注，且不得自动写入仓库（§9）。
**G8. 权限以服务端为准，不透露不可见资源。** 无权访问的仓库 / 仓库组，页面表现与"不存在"一致（Forbidden 文案不暴露资源是否存在）。
**G9. 危险操作要说清对象与后果。** 删除仓库、转移、改可见性、强制推送、删除受保护分支、撤销令牌：走基线 ConfirmActionDialog，**不可恢复**操作需输入**仓库完整路径**才能确认。
**G10. 术语统一。** 仓库 / 仓库组 / 分支 / 标签 / 提交 / 合并请求（MR）/ 议题 / 评审 / 克隆 / 推送 / 拉取 / 合并 / 变基 / 压缩合并 / 受保护分支 / 访问令牌 / SSH 密钥 / Webhook。禁止混用 PR、Pull Request、Repo、Merge Request 等写法（§12）。

> **复现自检：** 一眼看去像"另一个产品"，多半是侧栏 / 顶栏 / 主按钮偏离了基线；像"GitHub 山寨"，多半是节点用了圆点、Diff 画了左侧色条、列表每条都包成卡片；仓库列表"又挤又吵"，多半是每行同时放了边框、阴影、彩色标签——回到 G1。

---

## 2. 设计定位

### 2.1 产品

公司内部的代码托管与协作平台：**仓库与仓库组、代码浏览、提交与分支、合并请求与代码评审、权限与成员、访问令牌与 SSH 密钥、Webhook 集成、审计与系统运维，以及贯穿全站的 AI 问答助手**。面向研发、测试、运维、管理员。不是社交平台，不做"关注 / 点赞"式的社区运营界面（Star、Fork 作为可选的轻量能力，见 §7.5）。

### 2.2 概念：为什么 Git 也是"格"

Git 的世界同样由**离散单元**构成：一次提交、一个分支、一个仓库、一天的贡献。把它们统一画成圆角小方格，用户读到的是"1 格 = 1 次提交 / 1 个仓库"的可读单位，而不是装饰：

| 母题落点 | 表现 |
| --- | --- |
| 提交图节点 | 10px 方格（圆角 2.5），合并提交为 12px 空心方格 |
| 贡献热力、仓库活跃度 | `CellGrid`：12px 方格 / 间隙 2，走 `--ramp-*` 单色 5 档 |
| 语言分布 | 堆叠方格条，一格 = 1% |
| 仓库头像 | 圆角方块 + 首字母（分类色底），仓库组头像叠 2×2 小方格 |
| 分支 / 合并图标 | 端点是方格节点，不是圆点 |
| 复选、光标、加载、空状态 | 沿用基线（方格复选、方格光标、3×3 加载器、3×3 空状态图形） |
| 更新日志时间轴 | 8px 方格节点串在 1px 发丝线上 |

### 2.3 风格三词

| 词 | 含义 | 手段 |
| --- | --- | --- |
| **清** | 干净、有呼吸 | 雾白底、纸页白、发丝边；列表靠留白和横线，不靠卡片 |
| **简** | 规则少、元素少 | 主题蓝 + 中性灰阶；一页一个主角；少边框少阴影 |
| **精** | 细节讲究 | 同族线性图标 + 方格强调点、等宽哈希与路径、收紧的读数、Halo 主角卡 |

### 2.4 与"通用代码托管 UI"的差异（评审用）

通用代码托管 UI：圆点节点与彩色语言圆点、每个仓库一张带阴影的卡、Diff 左侧红绿竖条、满屏彩色徽标、多套图标混用、AI 入口是角落里的浮动气泡。
本系统：方格节点与方格语言条、行式列表 + 发丝线、整行底色 Diff、徽标只在"需要注意"时才有颜色、同族图标、AI 问答是首页的主角面板并带来源引用。

---

## 3. Design Tokens（`src/styles/tokens.css` + `git-tokens.css`）

> 3.1–3.7 与 Cell Workbench v5.6 **逐值一致**；3.8 为 Git 领域新增。暗色必须显式重定义，只有 §3.1 末尾白名单中的 token 允许主题无关。

### 3.1 颜色

```css
:root,
[data-theme="light"] {
  color-scheme: light;

  /* 表面 */
  --bg-page: #f4f6f9;     /* 雾白：应用底 */
  --bg-sheet: #ffffff;    /* 纸页：卡片、表格、代码、表单、弹窗 */
  --bg-sheet-2: #f8f9fb;  /* 次级纸页：表头、代码块头、README 代码块、卡内 inset */
  --bg-well: #eceff4;     /* 凹槽：分段控件底座、禁用输入、骨架、空方格 */
  --bg-hover: #f0f3f8;    /* 悬停（实色） */

  /* 前景：4 档，层级只靠字色，禁止 opacity */
  --fg-1: #0e1a2b;        /* 标题、主数据 */
  --fg-2: #3f4b5f;        /* 正文 */
  --fg-3: #647087;        /* 标签、表头、辅助 */
  --fg-4: #9aa4b5;        /* 占位、禁用（不用于必须阅读的信息） */

  /* 发丝线 */
  --hair: #e3e7ee;
  --hair-soft: #eef1f5;
  --hair-strong: #cbd2de;

  /* 墨：仅 Tooltip、深底强对比小块 */
  --ink: #12203a;
  --ink-hover: #1d3157;
  --ink-active: #0a1426;
  --on-ink: #ffffff;

  /* 主题色：固定 #1677ff，Light / Dark 同值，不换肤
     填充与图标 --signal；文字 --signal-text；浅底 --signal-soft */
  --signal: #1677ff;
  --signal-hover: #4096ff;
  --signal-active: #0958d9;
  --signal-soft: rgba(22, 119, 255, 0.08);
  --signal-soft-2: rgba(22, 119, 255, 0.14);
  --signal-ring: rgba(22, 119, 255, 0.28);
  --signal-text: #0958d9;
  --link: var(--signal-text);
  --on-signal: #ffffff;

  /* 状态：fg 文字 / 图标，bg 徽标底，dot 方格色 */
  --ok-fg: #0b7f58;   --ok-bg: rgba(15, 157, 110, 0.11);  --ok-dot: #16a370;
  --warn-fg: #9a5b00; --warn-bg: rgba(242, 154, 13, 0.15); --warn-dot: #f29a0d;
  --bad-fg: #c42a20;  --bad-bg: rgba(214, 54, 43, 0.1);   --bad-dot: #d6362b;
  --info-fg: #0958d9; --info-bg: rgba(22, 119, 255, 0.09); --info-dot: #1677ff;
  --idle-fg: #647087; --idle-bg: rgba(100, 112, 135, 0.12); --idle-dot: #9aa4b5;
  --bad-solid: #d6362b;
  --bad-solid-hover: #b42318;

  /* 图表系列（语义固定，不得每页重配） */
  --s1: #1677ff; --s2: #19b8a0; --s3: #a06cf2; --s4: #f2a20c; --s5: #f0568b; --s6: #9aa4b5;
  --s-total: #0e1a2b;
  --chart-grid: #e9edf3;
  --chart-axis: #9aa4b5;

  /* 方格色阶（热力、日历、活跃度）：单色 5 档 */
  --ramp-0: #eceff4; --ramp-1: #e6f4ff; --ramp-2: #bae0ff;
  --ramp-3: #69b1ff; --ramp-4: #4096ff; --ramp-5: #1677ff;

  /* 彩底读数墨色：在填充色上写字时按填充明度选墨 */
  --ink-on-light-fill: #0a1426;
  --ink-on-deep-fill: #ffffff;
  --cell-ink-3: var(--ink-on-light-fill);
  --cell-ink-4: var(--ink-on-light-fill);
  --cell-ink-5: var(--ink-on-light-fill);
  --cell-ink-warn: var(--ink-on-light-fill);
  --cell-ink-bad: var(--ink-on-deep-fill);

  /* 分类色（只区分类别，不表示好坏）：底 + 深字 */
  --cat1-bg: #e6f4ff; --cat1-fg: #0958d9;
  --cat2-bg: #ddf5f0; --cat2-fg: #14806e;
  --cat3-bg: #efe6fd; --cat3-fg: #6b3fc4;
  --cat4-bg: #fde6ee; --cat4-fg: #b02d63;
  --cat5-bg: #fff0d6; --cat5-fg: #98600a;

  /* 毛玻璃 */
  --frost-bg: rgba(255, 255, 255, 0.72);
  --frost-bg-strong: rgba(255, 255, 255, 0.88);
  --frost-edge: inset 0 1px 0 rgba(255, 255, 255, 0.9), 0 0 0 1px rgba(14, 26, 43, 0.07);
  --frost-blur: 18px;
  --frost-sat: 150%;
  --frost-solid: #ffffff;
  --veil: rgba(14, 26, 43, 0.34);

  /* 顶栏专用雾面 */
  --topbar-frost-bg: rgba(228, 232, 239, 0.55);
  --topbar-blur: 32px;
  --topbar-sat: 140%;
  --topbar-fade: 28px;

  /* 侧栏专用玻璃 */
  --rail-frost-bg: rgba(255, 255, 255, 0.62);
  --rail-frost-solid: #ffffff;
  --rail-blur: 28px;
  --rail-sat: 150%;
  --rail-glass-edge: inset 0 1px 0 rgba(255, 255, 255, 0.95),
    inset 0 0 0 1px rgba(255, 255, 255, 0.55), 0 0 0 1px rgba(14, 26, 43, 0.06),
    0 16px 40px -16px rgba(14, 26, 43, 0.18);

  /* 光晕：随真实健康度切换（主题无关） */
  --halo-ok: var(--signal);
  --halo-warn: #f2a20c;
  --halo-bad: #e5483b;
  --halo-opacity: 0.14;

  /* 深底控制台块（令牌展示、日志尾部；两种主题都是深底） */
  --console-bg: linear-gradient(180deg, #10192b, #0b1220);
  --console-ink: #edf1f8;
  --console-soft: #8f9bb4;
  --console-ok: #5ee0a6;
  --console-bad: #ff8a80;
  --console-edge: inset 0 1px 0 rgba(255, 255, 255, 0.08), 0 0 0 1px rgba(255, 255, 255, 0.06);

  /* 选中行不透明色（粘性层不得用半透明 --signal-soft） */
  --sel-solid: color-mix(in srgb, var(--signal) 9%, var(--bg-sheet));

  /* 全局 Message 胶囊：底 + 字均为不透明实色 */
  --msg-ok-bg: #e3f5ec;      --msg-ok-fg: #07684a;      --msg-ok-edge: rgba(15, 157, 110, 0.28);
  --msg-info-bg: #e6f4ff;    --msg-info-fg: #0958d9;    --msg-info-edge: rgba(22, 119, 255, 0.28);
  --msg-warn-bg: #fff1d6;    --msg-warn-fg: #7a4600;    --msg-warn-edge: rgba(242, 154, 13, 0.38);
  --msg-bad-bg: #fde8e6;     --msg-bad-fg: #a3201a;     --msg-bad-edge: rgba(214, 54, 43, 0.3);
  --msg-neutral-bg: #eef1f5; --msg-neutral-fg: #0e1a2b; --msg-neutral-edge: rgba(14, 26, 43, 0.1);
}

[data-theme="dark"] {
  color-scheme: dark;

  /* 暗色 = 中性黑（无蓝相）；主题蓝、状态色、分类色、系列色保持不变 */
  --bg-page: #0a0a0b;
  --bg-sheet: #151517;
  --bg-sheet-2: #1a1a1d;
  --bg-well: #0f0f11;
  --bg-hover: #212124;
  --fg-1: #f1f2f4;
  --fg-2: #c2c4ca;
  --fg-3: #8d9099;
  --fg-4: #5d6067;
  --hair: #29292e;
  --hair-soft: #1f1f23;
  --hair-strong: #3f3f46;

  --ink: #f1f2f4;
  --ink-hover: #ffffff;
  --ink-active: #d7d8dc;
  --on-ink: #111114;

  --signal: #1677ff;
  --signal-hover: #4096ff;
  --signal-active: #0958d9;
  --signal-soft: rgba(22, 119, 255, 0.18);
  --signal-soft-2: rgba(22, 119, 255, 0.28);
  --signal-ring: rgba(64, 150, 255, 0.45);
  --signal-text: #69b1ff;
  --link: #69b1ff;

  --ok-fg: #47cd89;   --ok-bg: rgba(23, 178, 106, 0.15);  --ok-dot: #47cd89;
  --warn-fg: #fdb022; --warn-bg: rgba(247, 144, 9, 0.15);  --warn-dot: #fdb022;
  --bad-fg: #f97066;  --bad-bg: rgba(240, 68, 56, 0.15);  --bad-dot: #f97066;
  --info-fg: #69b1ff; --info-bg: rgba(22, 119, 255, 0.18); --info-dot: #4096ff;
  --idle-fg: #8d9099; --idle-bg: rgba(141, 144, 153, 0.15); --idle-dot: #6f7278;
  --bad-solid: #e5483b;
  --bad-solid-hover: #f05a4d;

  --s1: #1677ff; --s2: #2fd4bc; --s3: #b58cff; --s4: #fbbf4d; --s5: #f580a8; --s6: #6f7278;
  --s-total: #f1f2f4;
  --chart-grid: #232327;
  --chart-axis: #6f7278;

  --ramp-0: #1c1c20; --ramp-1: #112a4d; --ramp-2: #15417e;
  --ramp-3: #1554ad; --ramp-4: #1668dc; --ramp-5: #4096ff;

  --cell-ink-3: var(--ink-on-deep-fill);
  --cell-ink-4: var(--ink-on-deep-fill);
  --cell-ink-5: var(--ink-on-light-fill);
  --cell-ink-warn: var(--ink-on-light-fill);
  --cell-ink-bad: var(--ink-on-light-fill);

  --cat1-bg: #112a4d; --cat1-fg: #8ec5ff;
  --cat2-bg: #123a35; --cat2-fg: #7fe0cf;
  --cat3-bg: #2a2150; --cat3-fg: #c5a8ff;
  --cat4-bg: #3a1d2d; --cat4-fg: #f5a3c2;
  --cat5-bg: #3a2d12; --cat5-fg: #f0c070;

  --frost-bg: rgba(21, 21, 23, 0.7);
  --frost-bg-strong: rgba(21, 21, 23, 0.88);
  --frost-edge: inset 0 1px 0 rgba(255, 255, 255, 0.07), 0 0 0 1px rgba(255, 255, 255, 0.07);
  --frost-blur: 22px;
  --frost-sat: 130%;
  --frost-solid: #1a1a1d;
  --veil: rgba(0, 0, 0, 0.62);

  --topbar-frost-bg: rgba(12, 12, 14, 0.5);
  --topbar-blur: 32px;
  --topbar-sat: 130%;
  --rail-frost-bg: rgba(24, 24, 27, 0.58);
  --rail-frost-solid: #151517;
  --rail-blur: 28px;
  --rail-sat: 130%;
  --rail-glass-edge: inset 0 1px 0 rgba(255, 255, 255, 0.12),
    inset 0 0 0 1px rgba(255, 255, 255, 0.09), 0 0 0 1px rgba(0, 0, 0, 0.35),
    0 16px 40px -16px rgba(0, 0, 0, 0.55);
  --halo-opacity: 0.22;
  --sel-solid: color-mix(in srgb, var(--signal) 12%, var(--bg-sheet));

  --console-bg: linear-gradient(180deg, #141417, #0b0b0d);
  --console-ink: #f1f2f4;
  --console-soft: #999ba2;

  --msg-ok-bg: #103a2c;      --msg-ok-fg: #7fe3b8;      --msg-ok-edge: rgba(71, 205, 137, 0.3);
  --msg-info-bg: #0f2747;    --msg-info-fg: #8ec5ff;    --msg-info-edge: rgba(64, 150, 255, 0.32);
  --msg-warn-bg: #3a2a0e;    --msg-warn-fg: #ffc95c;    --msg-warn-edge: rgba(253, 176, 34, 0.32);
  --msg-bad-bg: #3f1b19;     --msg-bad-fg: #ff9a92;     --msg-bad-edge: rgba(249, 112, 102, 0.32);
  --msg-neutral-bg: #202024; --msg-neutral-fg: #f1f2f4; --msg-neutral-edge: rgba(255, 255, 255, 0.08);
}
```

**颜色规则**

- **主题色 = 行动 + 选中 + 数据**（主按钮、选中行、激活导航、链接、焦点、主系列），固定 `#1677ff`；页面与 Agent 不得自定义主色。一个区域最多一个实心主按钮。
- 白字压在 `#1677ff` 上约 4.1:1：仅限按钮文字（14/500 及以上）；**小字号文字（计数角标、Tag）一律不放在主题色底上**，改用 `--signal-soft-2` 底 + `--signal-text` 字。
- 分类色 `--cat*` 只用于头像底、标签分组；不表示好坏。方格色阶 `--ramp-*` 只用于"量的深浅"。
- **彩底读数**取 `--cell-ink-*`，不得写死 `#fff`。
- 文字层级只用 `--fg-3/4`，**禁止 `opacity`**。
- **主题无关 token 白名单**：`--halo-ok / --halo-warn / --halo-bad`、`--console-*`、`--ink-on-light-fill / --ink-on-deep-fill`、`--topbar-fade`、`--lang-*`（§3.8）。其余 token 暗色必须显式重定。

### 3.2 圆角、阴影、焦点

```css
:root {
  --rad-cell-s: 2px;   /* 6–8px 方格 */
  --rad-cell-m: 4px;   /* 12–16px 方格、复选框 */
  --rad-cell-l: 6px;   /* 24–28px 方格 */
  --rad-chip: 6px;     /* 标签、芯片、键位、哈希芯片 */
  --rad-ctl: 8px;      /* 按钮、输入、导航项、仓库头像（32px）*/
  --rad-pop: 10px;     /* 菜单、Popover、Tooltip、图标瓷砖（40px）*/
  --rad-sheet: 12px;   /* 纸页、卡片、Banner、底部浮条、AI 输入框 */
  --rad-hero: 16px;    /* 主角卡、弹窗、抽屉、登录卡、命令面板 */
  --rad-full: 999px;   /* 徽标、用户头像、开关 */

  --sh-0: none;
  --sh-sheet: 0 0 0 1px var(--hair), 0 1px 2px rgba(14, 26, 43, 0.04);
  --sh-raise: 0 0 0 1px var(--hair), 0 8px 22px -10px rgba(14, 26, 43, 0.18);
  --sh-pop: 0 4px 10px rgba(14, 26, 43, 0.06), 0 20px 44px -16px rgba(14, 26, 43, 0.26);
  --sh-modal: 0 6px 16px rgba(14, 26, 43, 0.08), 0 40px 90px -24px rgba(14, 26, 43, 0.38);
  --sh-rail: 0 0 0 1px var(--hair), 0 16px 40px -16px rgba(14, 26, 43, 0.16);
  --sh-stuck: 0 8px 12px -10px rgba(14, 26, 43, 0.18);

  --focus-outline: 2px solid var(--signal);      /* 键盘焦点，offset 2px（单元格 / 行 / 导航项内 -2px） */
  --focus-halo: 2px solid var(--signal-ring);    /* 输入聚焦外晕：border-color 变 --signal 再叠此 outline */
  --focus-halo-bad: 2px solid rgba(214, 54, 43, 0.26);
}
[data-theme="dark"] {
  --sh-sheet: 0 0 0 1px var(--hair), 0 1px 2px rgba(0, 0, 0, 0.3);
  --sh-raise: 0 0 0 1px var(--hair-strong), 0 10px 26px -10px rgba(0, 0, 0, 0.6);
  --sh-pop: 0 4px 10px rgba(0, 0, 0, 0.4), 0 22px 48px -16px rgba(0, 0, 0, 0.7);
  --sh-modal: 0 6px 16px rgba(0, 0, 0, 0.45), 0 44px 96px -24px rgba(0, 0, 0, 0.8);
  --sh-rail: 0 0 0 1px var(--hair), 0 16px 40px -16px rgba(0, 0, 0, 0.55);
  --sh-stuck: 0 8px 12px -10px rgba(0, 0, 0, 0.6);
  --focus-halo-bad: 2px solid rgba(249, 112, 102, 0.55);
}
```

**圆角规则：** 档位只有 2 / 4 / 6 / 8 / 10 / 12 / 16 / 999，最大 16。**唯一几何豁免：方格组件按 `边长 × 25%` 取圆角**（仓库头像 / 提交图节点 / 语言格 / 热力格同理）。内层圆角 = 外层圆角 − 内边距（< 4 取 4）。
**控件规则：** 按钮、输入、Select、分页项、验证码格**只用真实 `border: 1px solid`，零 box-shadow**，含悬停与聚焦。

### 3.3 间距与尺寸

```css
:root {
  --sp-1: 4px;  --sp-2: 8px;  --sp-3: 12px; --sp-4: 16px; --sp-5: 20px;
  --sp-6: 24px; --sp-8: 32px; --sp-10: 40px; --sp-12: 48px;

  --topbar-h: 56px;
  --rail-w: 224px;
  --rail-w-collapsed: 72px;
  --rail-edge: 12px;
  --rail-gap: 12px;
  --ctl-h-sm: 30px; --ctl-h: 36px; --ctl-h-lg: 42px;
  --row-h: 48px;
  --th-h: 40px;
  --page-max: 1680px;
  --page-px: 28px;           /* ≤1199: 24；≤767: 16 */
  --gap: 16px;
  --gap-section: 24px;
  --sheet-pad: 20px;
  --sheet-pad-hero: 24px;
  --form-w-sm: 160px; --form-w-md: 280px; --form-w-lg: 480px; --form-max: 720px;

  --cell-xs: 6px; --cell-s: 8px; --cell-m: 12px; --cell-l: 16px; --cell-xl: 24px;
  --cell-gap: 2px;

  --rail-current: calc(var(--rail-w) + var(--rail-edge) + var(--rail-gap));
  --content-left: var(--rail-current);
  --chrome-h: var(--topbar-h);
}
:root[data-rail="collapsed"] { --rail-current: calc(var(--rail-w-collapsed) + var(--rail-edge) + var(--rail-gap)); }
@media (max-width: 1199px) { :root, :root[data-rail] { --rail-current: calc(var(--rail-w-collapsed) + var(--rail-edge) + var(--rail-gap)); } }
@media (max-width: 819px)  { :root, :root[data-rail] { --rail-current: 0px; } }
:root[data-shell="bare"]   { --rail-current: 0px; }   /* 登录、未登录首页、整页错误：无侧栏 */
[data-density="compact"] { --row-h: 40px; --ctl-h: 32px; --gap: 12px; }
[data-density="comfy"]   { --row-h: 56px; }
```

| 场景 | 值 |
| --- | --- |
| 页面区块之间 / 栅格 gap | 24px / 16px |
| 纸页内边距 | 20px（主角 24px；卡片头 16px 20px） |
| 表单：字段间距 / 标签到控件 / 控件到提示 | 20px / 6px / 6px |
| 侧栏导航项：高 / 圆角 / 图标 / 图标文字间距 | 38 / `--rad-ctl` / 18px / 10px |
| **仓库行（Git 新增）** | 高 64（紧凑密度 56），头像 32，行内左右 20 |
| **提交行 / 文件行 / 分支行（Git 新增）** | 高 48（紧凑 40） |
| **代码行（Git 新增）** | 行高 20，等宽 13 |

### 3.4 字体

```css
:root {
  --font-sans: "Geist", -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI",
    "PingFang SC", "HarmonyOS Sans SC", "Noto Sans SC", "Microsoft YaHei", sans-serif;
  --font-mono: "Geist Mono", "JetBrains Mono", "SF Mono", ui-monospace, Menlo, Consolas, monospace;

  --fs-xs: 12px; --fs-sm: 13px; --fs-md: 14px; --fs-lg: 16px; --fs-xl: 20px; --fs-2xl: 24px;
  --fs-read-md: 22px; --fs-read-lg: 32px; --fs-read-xl: 52px;
  --fw-regular: 400; --fw-medium: 500; --fw-semibold: 600;
  --tr-title: -0.02em; --tr-read: -0.03em;
}
html { font: 14px/1.5 var(--font-sans); color: var(--fg-2); -webkit-font-smoothing: antialiased; font-feature-settings: "tnum" 1, "cv11" 1; }
```

| 角色 | 字号 / 字重 / 行高 | 颜色 | 字距 |
| --- | --- | --- | --- |
| 页面标题 | 24 / 600 / 32 | `--fg-1` | `--tr-title` |
| 区块标题 | 16 / 600 / 24 | `--fg-1` | 0 |
| 卡片标题 | 14 / 600 / 20 | `--fg-1` | 0 |
| 营销页主标题（G0 Hero，**仅此一处**） | 52 / 600 / 1.08 | `--fg-1` | `--tr-read` |
| 主角读数 / 标准读数 / 紧凑读数 | 52 / 32 / 22，600 | `--fg-1` | `--tr-read` |
| 正文 | 14 / 400 / 22 | `--fg-2` | 0 |
| 表格 / 列表正文 | 13 / 400 / 20 | `--fg-1`（主列）/ `--fg-2` | 0 |
| 表头 | 12 / 500 / 16 | `--fg-3`，不全大写 | 0 |
| 辅助 / 时间 / 轴 | 12 / 400 | `--fg-3` | 0 |
| **仓库名（列表主列）** | 14 / 600 | `--fg-1`；组路径前缀 `--fg-3` 400 | 0 |
| **提交信息标题** | 13 / 500 | `--fg-1` | 0 |
| **哈希 / 分支名 / 路径 / 令牌 / 代码** | 12–13 / 400 `--font-mono` | `--fg-2` | 0；关闭连字 |
| **README / Markdown 正文** | 14 / 400 / 24；h1 24/600、h2 20/600、h3 16/600 | `--fg-2` / `--fg-1` | 0 |

- 只用 400 / 500 / 600 三档字重；禁止全大写英文眉标与 `01/02` 编号。
- 等宽字体只给：哈希、分支 / 标签名、文件路径、令牌与 Key、克隆地址、日志、代码。**仓库名、提交信息标题不用等宽。**
- 代码字体**关闭连字**（`font-variant-ligatures: none`），避免 `=>`、`!=` 被合成。

### 3.5 动效

```css
:root {
  --ease: cubic-bezier(0.22, 0.9, 0.3, 1);
  --t-fast: 120ms;   /* 悬停、按压、颜色 */
  --t-base: 200ms;   /* 展开、切换、下拉 */
  --t-slow: 280ms;   /* 弹窗、抽屉、页面进场 */
  --t-leave: 100ms;  /* 页面离场 */
  --t-draw: 520ms;   /* 图表首次绘制 */
  --t-sweep: 320ms;  /* 方格扫描点亮总时长 */
}
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { animation-duration: 1ms !important; animation-delay: 0ms !important; transition-duration: 1ms !important; }
}
```

### 3.6 z-index 与断点

- z-index：`--z-sticky:10` · `--z-topbar:20` · `--z-rail:30` · `--z-dropdown:1000` · `--z-drawer:1100` · `--z-modal:1200` · `--z-message:1300` · `--z-toast:1400`。
- 断点（统一 `max-width`，全局只此一套）：`1199`（侧栏强制收起，`--page-px` 24）· `991`（多栏堆叠）· `819`（侧栏抽屉化）· `767`（`--page-px` 16）· `599` · `419`。不声明 1440 档，超宽屏只由 `--page-max` 居中。

### 3.7 材质一览

| 类 | 质感 | 真模糊 | 用于 |
| --- | --- | --- | --- |
| `.sheet` | 白色纸页 + 发丝环 | ❌ | 卡片、表格、代码、Diff、表单（**默认**） |
| `.sheet--hero` | 纸页 + 网纹 + Halo | ❌ | 每页唯一主角（G1 的 AI 面板、G4 的仓库头部） |
| `.well` | 凹槽 | ❌ | 分段控件底座、输入底座 |
| `.inset` | 卡内浅底（`--bg-sheet-2`，圆角 6） | ❌ | 评论线程、来源引用卡、AI 评审摘要 |
| `.frost` | 毛玻璃 | ✅ | 顶栏、侧栏、命令面板、底部浮条 |
| `.veil` | 遮罩 + 8px 静态模糊 | ✅ | 弹窗与抽屉背后 |
| `.halo` | 角落模糊光斑（必须在裁切层内） | — | 主角卡、空状态、未登录首页 Hero |

> 毛玻璃只出现在"浮在内容之上"的面；Select / Dropdown / Popover / 分支选择器面板为实色纸页。同屏真模糊层 ≤ 4。`--fg-4` 不得出现在毛玻璃上。Halo 颜色由真实服务健康度（Git / SSH / Hook 队列）驱动，不得手动指定装饰色。

### 3.8 Git 领域新增 Token（`src/styles/git-tokens.css`）

> 新增 token 必须同时给 Light / Dark 两套（语言色除外）。下列配色已在**代码块底（`--bg-sheet-2`）与 Diff 增删行底**上实测：亮色代码文字最低 **4.8:1**，暗色最低 **5.4:1**。

```css
:root,
[data-theme="light"] {
  /* Diff：整行实色底（不透明）；行内逐词变更叠半透明加深 */
  --diff-add-bg: #e9f6f2;
  --diff-del-bg: #fcefee;
  --diff-add-word: rgba(15, 157, 110, 0.26);
  --diff-del-word: rgba(214, 54, 43, 0.24);
  --diff-add-sign: var(--ok-fg);
  --diff-del-sign: var(--bad-fg);
  --diff-hunk-bg: var(--bg-sheet-2);
  --diff-hunk-fg: var(--signal-text);
  --diff-ctx-bg: var(--bg-sheet);

  /* 代码语法：只给高亮引擎的 class 映射使用 */
  --syn-keyword: #7a3fd0;
  --syn-string: #0a7550;
  --syn-number: #9a5b00;
  --syn-comment: #5a667c;
  --syn-function: #0958d9;
  --syn-type: #0f7363;
  --syn-variable: #b02d63;
  --syn-operator: #3f4b5f;
  --syn-tag: #0958d9;
  --syn-property: #6b3fc4;
  --code-bg: var(--bg-sheet-2);
  --code-line-hl: var(--sel-solid);        /* 选中 / 锚点行：整行实色底 */
  --code-gutter-fg: var(--fg-3);
  --diff-toolbar-h: 52px;                  /* Diff 粘性工具栏高度（主题无关尺寸）；文件头吸顶 = --chrome-h + 它 */
}
[data-theme="dark"] {
  --diff-add-bg: #152b23;
  --diff-del-bg: #341c1c;
  --diff-add-word: rgba(23, 178, 106, 0.34);
  --diff-del-word: rgba(240, 68, 56, 0.36);

  --syn-keyword: #c5a8ff;
  --syn-string: #47cd89;
  --syn-number: #fdb022;
  --syn-comment: #999ba2;
  --syn-function: #69b1ff;
  --syn-type: #7fe0cf;
  --syn-variable: #f5a3c2;
  --syn-operator: #c2c4ca;
  --syn-tag: #69b1ff;
  --syn-property: #b58cff;
}

/* 语言色：主题无关的"标记色"，永远与语言名文字并列出现，不单独承载信息 */
:root {
  --lang-ts: #2f74c0;   --lang-js: #c99700;   --lang-py: #7e57c8;   --lang-go: #0f9f8a;
  --lang-java: #d9622b; --lang-rust: #b0553a; --lang-c: #7a869a;    --lang-shell: #3f9b45;
  --lang-other: var(--s6);
}
/* 提交图泳道：循环取 --s1…--s5，第 6 条起回到 --s1；主干（默认分支）恒为 --s1 */
```

**使用规则**

- 增删行底只用 `--diff-add-bg / --diff-del-bg`；逐词变更叠 `--diff-*-word`。**行号栏的 `+ / −` 符号是增删的第二通道**（不只靠颜色）。
- 选中行 / 锚点行（`#L12-L20`）整行 `--code-line-hl`，**无左侧竖条**。
- 语法色只通过 `--syn-*` 引用，高亮引擎自带的主题 CSS 一律不加载。
- 语言色只用于语言条的方格与语言名前的 8px 方格；新增语言必须补一条 `--lang-*` 并登记白名单，未登记的归 `--lang-other`。

---

## 4. 外壳与导航

### 4.1 结构（与基线完全一致）

- **侧栏 = 悬浮玻璃面板**：`top/left/bottom: var(--rail-edge)`，宽 224（收起 72），16px 圆角，`--rail-frost-bg` + `backdrop-filter` + `--rail-glass-edge`，叠在顶栏之上（`--z-rail:30 > --z-topbar:20`）。降级 / 减少透明度时退回实色 `--rail-frost-solid` + `--sh-rail`。
- **顶栏 = 通栏雾面毛玻璃条**，高 56，`left:0; right:0`，无下缘线，下缘 `--topbar-fade`(28px) 渐隐；`padding-left = --content-left` 为侧栏让位；内层与页面内容共用 `--page-max` 与 `--page-px`。
- **折叠按钮在侧栏底部**（`.rail-foot` 最上一项），其下是服务状态文字 + 版本号（真实运行事实；异常时文字染 `--warn-fg / --bad-fg`，无色点）。
- 收起 = 只留图标 + Tooltip；`<1200` 强制收起；`<820` 变抽屉。
- 内容区 `.main { margin-left: var(--content-left) }`。登录、**未登录首页（G0）**、整页错误使用 `data-shell="bare"`，不渲染侧栏。
- 导航激活 = 主题色浅底滑块 `.nav-pill`（只补间 y）+ 主题色图标与文字，无边、无影、无竖条、无前置点。

### 4.2 侧栏导航清单

分组标题 12/500 `--fg-3`；一级 = 分组名，二级 = 页面项（图标 18 + 名称）。**仓库内部的二级导航不进侧栏**，用页内 Tabs（§6 G4），保证侧栏在全公司所有系统中始终只表达"平台级"结构。

| 分组 | 页面项 | 图标 | 路由 | 可见性 |
| --- | --- | --- | --- | --- |
| 工作台 | 首页 | `home` | `/` | 全员 |
| 工作台 | 我的工作 | `inbox` | `/work` | 全员；右侧计数 = 待我评审数 |
| 代码 | 仓库 | `repo` | `/repos` | 全员 |
| 代码 | 仓库组 | `group` | `/groups` | 全员 |
| 代码 | 代码搜索 | `search-code` | `/search` | 全员 |
| 协作 | 合并请求 | `merge-request` | `/merge-requests` | 全员 |
| 协作 | 议题 ○ | `issue` | `/issues` | 产品含议题模块时 |
| 协作 | 流水线 ○ | `pipeline` | `/pipelines` | 产品含流水线模块时 |
| 管理 | 用户与团队 | `users` | `/admin/users` | 管理员 |
| 管理 | 审计日志 | `audit` | `/admin/audit` | 管理员 / 审计员 |
| 管理 | 集成与 Webhook | `webhook` | `/admin/integrations` | 管理员 |
| 管理 | 系统状态 | `activity` | `/admin/status` | 管理员 / 运维 |
| 管理 | 系统设置 | `settings` | `/admin/settings` | 管理员 |

> ○ = 可选模块，以实际产品能力为准；没有的模块**整项不渲染**，不留"敬请期待"。"管理"分组由服务端按权限下发，无权限用户看不到该分组标题。
> 个人级入口（个人资料、访问令牌、SSH 密钥、偏好、退出）放在顶栏头像菜单，不进侧栏。

**导航计数角标：** 高 18、全圆角改 `--rad-chip`(6)，12/600 tabular，`--signal-soft-2` 底 + `--signal-text` 字；0 不显示；> 99 记 `99+`。

### 4.3 顶栏：上下文 + 全局功能入口

顶栏只承载**上下文与全局动作**，不放一级导航、不放折叠按钮（`<820` 抽屉模式下最左出现菜单图标钮是唯一例外）。

| 位置 | 内容 |
| --- | --- |
| 左 | 面包屑：最多 3 级，当前页 13/500 `--fg-1`，上级 `--fg-3`，分隔 `›`；仓库路径超长时中间折叠为 `…`（点击展开 Popover）；`<768` 只显示"← 上级 / 当前页" |
| 中 | **留空**，为全局 Message 胶囊让出居中位置 |
| 右（按序） | ① 搜索框（扁平，高 36，`--bg-page` 填充 + 1px `--hair`，宽 `clamp(220px, 22vw, 320px)`，占位"搜索代码、仓库，或询问 AI…"，右侧 `.kbd` ⌘K）② **新建 ▾**（`.btn` 次按钮 + 加号；菜单：新建仓库 / 新建仓库组 / 新建合并请求 / 导入仓库）③ 待我评审（`.icon-btn` + 计数角标）④ 通知（`.icon-btn`）⑤ 主题（`.icon-btn`）⑥ 头像 + 下拉 |

> **关于"顶部放功能入口"：** 功能入口（新建、评审待办、通知、搜索 / AI）放顶栏；**平台级页面导航仍在侧栏**。这样 Git 平台与公司其他系统的"导航在哪、动作在哪"完全一致，用户无需重新学习。

### 4.4 命令面板（⌘K / Ctrl+K）

`.frost`，宽 560，圆角 `--rad-hero`，距顶 15vh；上方输入框 + 下方结果分区，键盘 ↑↓ / Enter / Esc。分区顺序：**询问 AI**（输入看起来像自然语言问题时置顶，Enter = 带着问题去首页 AI 面板或打开侧滑 AI 抽屉）→ 跳转（仓库 / 仓库组 / 页面）→ 命令（新建仓库、切换主题…）→ 最近访问。前缀：`>` 命令、`#` 合并请求、`@` 用户、`/` 当前仓库内文件。

### 4.5 快捷键

输入框 / 编辑器聚焦时不触发单键快捷键。

| 键 | 行为 | 范围 |
| --- | --- | --- |
| ⌘K / Ctrl+K | 命令面板 | 全局 |
| `/` | 聚焦搜索框 | 全局 |
| `t` | 转到文件（仓库内模糊查找） | 仓库 |
| `b` | 切换分支 | 仓库 |
| `g` `c` / `g` `m` | 去代码 / 去合并请求 | 仓库 |
| `l` | 转到行 | 文件视图 |
| `j` / `k`、`n` / `p` | 下 / 上一个文件、下 / 上一条评论 | Diff |
| `v` | 标记当前文件为已查看 | Diff |
| `?` | 快捷键帮助 Dialog | 全局 |

### 4.6 PageHeader

标题 24/600 + 副标题 13 `--fg-3`（单行）+ 右侧操作区；一个页头**最多一个主按钮**。仓库页使用专用 RepoHeader（§7.4）。

---

## 5. 布局体系

### 5.1 栅格与纸页（与基线一致）

```css
.page, .section { display: grid; gap: var(--gap-section); }   /* 页头、区块之间 24 */
.stack { display: grid; gap: var(--gap); }                    /* 单列同级卡片 16 */
.grid, .bento { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); gap: var(--gap); align-items: stretch; }
.col-1 … .col-12 { grid-column: span N; }
@media (max-width: 991px) { [class*="col-"] { grid-column: span 12; } }
.sheet-head { min-height: 56px; padding: 12px var(--sheet-pad); box-shadow: inset 0 -1px 0 var(--hair-soft); }
.sheet-body { padding: var(--sheet-pad); }
.sheet-body--flush { padding: 0; }
.sheet-foot { min-height: 56px; padding: 12px var(--sheet-pad); box-shadow: inset 0 1px 0 var(--hair-soft); }
```

- 纸页自身 margin 恒为 0；间距只来自容器 `gap`。两张纸页贴在一起 = 容器漏了 `gap`，修容器，不给纸页加 margin。
- 内容区最大宽 `--page-max`（1680）居中；**文本阅读宽度** `--prose-max: 880px`（README、描述、评论正文超出不再拉宽）。
- **代码与 Diff 视图**可使用通栏（占满 `--page-max`），不套 `--prose-max`。

### 5.2 Git 领域版式库

| 版式 | 名称 | 结构 | 主角 |
| --- | --- | --- | --- |
| **G0** | 未登录产品首页 | 通栏顶栏 + Hero + 界面示例 + 特性 bento + 页脚（`bare`） | Hero 标题与界面示例 |
| **G1** | 登录后首页 | `.bento` 3 / 6 / 3：常用仓库 · AI 问答 · 更新日志 | AI 问答面板（`.sheet--hero`） |
| **G2** | 仓库列表 | 页头 + Tabs + 工具栏 + 行式列表 sheet（flush） | 列表 |
| **G3** | 仓库组（树 / 详情） | 树形表 · 详情头 + Tabs + 概览双栏 | 树 / 概览 |
| **G4** | 仓库首页 | RepoHeader + Tabs + 9/3（文件列表 + README ｜ 关于） | 文件列表 |
| **G5** | 文件视图 | 工具栏 + 代码 sheet（通栏） | 代码 |
| **G6** | 提交 / 提交图 / 提交详情 | 工具栏 + 行式列表或图谱 sheet | 列表 / 图谱 |
| **G7** | 分支与标签 | Tabs + DataTable | 表 |
| **G8** | 合并请求列表 | Tabs + 工具栏 + 行式列表 | 列表 |
| **G9** | 合并请求详情 | 头部 + Tabs + 8/4（时间线 + MergeBox ｜ 侧栏） | 时间线 |
| **G10** | Diff 视图 | 粘性工具栏 + 文件树（280）+ 文件块 | Diff |
| **G11** | 设置 | 左锚点导航（200）+ 分区 sheet | 表单 |
| **G12** | 令牌与密钥 | 页头 + DataTable + 创建 Dialog | 表 |
| **G13** | 管理后台 | 复用基线 P2 / P7 / P9 / P10 | 因页而异 |
| **G14** | 搜索 | 左筛选（240）+ 结果列表 | 结果 |
| **G15** | 用户主页 | 左资料（3）+ 右贡献热力 / 活动（9） | 热力图 |

---

## 6. 页面版式详述

### G0 未登录产品首页（`/`，未登录；`data-shell="bare"`）

类似 GitHub 的产品宣传页，但用格·语汇表达。**只讲真实能力，不放虚构数据。**

**结构（自上而下，最多 5 段）**

| 段 | 规格 |
| --- | --- |
| **顶栏** | `.frost` 通栏，高 56，内层居中最大宽 1200。左：品牌图形标 28 + 产品名 14/600；中左：`产品` `文档` `更新日志`（13/500 `--fg-2`，悬停 `--bg-hover` 圆角 8）；右：主题 `.icon-btn` + **登录**（次按钮）。顶栏不放主按钮，主 CTA 留给 Hero |
| **Hero** | 画布 `--bg-page` + **方格网纹四周淡出** + 一枚 Halo（颜色由真实服务健康度驱动）。居中：主标题 52/600（`--tr-read`，≤767 缩为 40，≤419 缩为 32），副标题 16/26 `--fg-2`（最大宽 560），CTA 组：**使用企业账号登录**（主按钮 lg 42）+ **查看文档**（次按钮 lg）。CTA 下方 12 `--fg-3` 一行说明（可选）。示例文案（可替换）：主标题"让每一次提交，都有迹可循"，副标题"统一托管仓库、评审合并请求，并用 AI 读懂你的代码。" |
| **界面示例** | `.sheet--hero`（宽 `min(1120px, 100%)`，高 480，下缘渐隐遮罩），内部用**真实组件的静态示例态**拼出：仓库行列表 + AI 回答片段 + 合并检查清单；右上角 Tag「界面示例」（idle）。随主题明暗切换。首次进入视口做一次方格扫描点亮（`--t-sweep`），不循环 |
| **特性 bento** | `.bento` 12 列：第一行 `col-7` 仓库与仓库组、`col-5` 合并请求与评审；第二行三个 `col-4`：AI 助手、权限与安全、集成与自动化。卡片 = `.sheet` + 40 图标瓷砖（§8.3）+ 标题 16/600 + 说明 13/20 `--fg-3`（≤ 2 行）+ 底部 ≤ 120 高的**方格示意图**。卡片不可点击，不加悬停抬升 |
| **同源矩阵**（可选） | 一行小瓷砖：公司内部其他平台入口（当前系统高亮 `--signal-soft`），数据来自配置；只有一个系统时整段不渲染 |
| **收尾 CTA** | 居中 `.sheet`：标题 24/600"使用企业账号登录" + 主按钮 |
| **页脚** | 顶部发丝线；左：© 公司名 + 版本（等宽）；中：文档 · 更新日志 · 联系管理员；右：**服务状态文字**（真实接口：正常 / 降级 / 异常，染 `--ok-fg / --warn-fg / --bad-fg`，无色点） |

**纪律**

- **禁止**虚构用户数、仓库数、提交数；任何数字必须来自真实接口。
- 禁止库存照片、3D 插画、自动播放视频、渐变彩球；视觉只用方格、网纹、Halo、真实组件示例。
- 一页一个 Halo（在 Hero）；特性卡不再放 Halo。
- 已有有效登录态时直接跳转 G1，不展示 G0。登录跳转携带 `redirect`。
- `<992` bento 整行堆叠；`<820` 顶栏中左导航收进菜单图标钮。

### G1 登录后首页（`/`）

```
页头：下午好，{姓名}                         副标题：今天有 3 个合并请求等你评审（无待办则不显示该句）
┌ 常用仓库（3）─────┐ ┌ AI 问答（6，.sheet--hero）─────────┐ ┌ 更新日志（3）────────┐
│ ▣ repo-a          │ │ 想了解什么？                          │ │ 11 月                  │
│   group · 2 小时前│ │ ┌ 输入框 ───────────────────────┐   │ │ ◼ v1.4.0  11-05       │
│ ▣ repo-b          │ │ │ …                              │   │ │   AI 问答支持引用文件  │
│   …               │ │ └ [范围 ▾] [# 引用] [模型 ▾]  [发送] ┘ │ │ ◼ v1.3.2  10-28       │
│ 查看全部仓库 →    │ │ 推荐问题 chips                        │ │   …                    │
└───────────────────┘ │ 回答区：流式文本 ▌ + 来源引用卡       │ │ 查看全部更新日志 →     │
                      └──────────────────────────────────────┘ └──────────────────────┘
```

- **栅格：** `.bento`，≥ 992 为 `col-3 / col-6 / col-3`，`align-items: start`；`<992` 堆叠顺序 **AI 问答 → 常用仓库 → 更新日志**。
- **页头：** 标题"上午好 / 下午好 / 晚上好，{姓名}"（按本地时间），副标题只在有真实待办时显示；页头不放主按钮（新建在顶栏）。
- **左：常用仓库**（`.sheet`，flush）。头 56：标题 14/600 + `.icon-btn` 新建仓库。体：最多 8 个 `RepoMiniRow`（高 52：仓库头像 28 + 仓库名 13/500 `--fg-1` + 第二行 `组路径 · 相对时间` 12 `--fg-3`），悬停出现"置顶"`.icon-btn`。尾：文字链"查看全部仓库 →"。**排序规则：** 置顶在前，其余按"近 14 天本人活跃度（提交 + 评审 + 评论）"降序，同分按最近推送。空：小号 Empty"还没有常用仓库"+ [浏览仓库]。
- **左下（可选 ○）：待我评审**：另一张 `.sheet`，最多 5 条 `MrMiniRow`（标题单行 + `#id · 作者 · 时间`），尾部链到 `/work`。
- **中：AI 问答面板**（§9），本页唯一 `.sheet--hero`，Halo 随服务健康度变化。
- **右：更新日志**（§7.12 `ChangelogTimeline`），`.sheet`，头：标题 + "全部标为已读"文字钮。
- 三块各自独立加载，任一失败只在该块内显示 ErrorState（带重试），不拖垮整页。

### G2 仓库列表（`/repos`）

- **页头：** 标题"仓库"，副标题"共 N 个"；右：[导入仓库]（次）+ [新建仓库]（主）。
- **Tabs（下划线，高 44）：** 全部 · 我的 · 我参与 · 已收藏 ○ · 已归档，右侧带计数。
- **工具栏（SearchToolbar）：** 搜索（280）→ 可见性 Select → 语言 Select → 仓库组 Combobox → 排序 Select（最近推送 / 名称 / 创建时间 / 活跃度）；右：视图 Segmented **[列表｜分组｜画廊]**（三枚图标）+ 列设置。已应用筛选以芯片展示在工具栏下，可单个移除；筛选、排序、视图写入 URL。
- **列表视图（默认）：** `.sheet` flush，一行一个仓库，**行与行之间只有 `--hair-soft` 横线，无边框、无阴影、无斑马纹**。

| 列 | 宽 | 内容 |
| --- | --- | --- |
| 主列 | 弹性 ≥ 280 | 仓库头像 32 + 第一行：`组路径 /`（`--fg-3` 400）+ **仓库名**（`--fg-1` 600）+ 状态 Tag（可见性图标 Tag、已归档、模板）；第二行：描述 13 `--fg-3`，单行省略 |
| 语言 | 144 | 8px 语言方格 + 语言名 13 `--fg-2`；无语言留空 |
| 活跃度 | 112 | **14 格活跃度**：近 14 天每天一格（8px，间隙 2，走 `--ramp-0…5`），悬停 Tooltip"日期 · N 次提交" |
| 合并请求 | 96 | 图标 + 开放数（右对齐，`tabular-nums`；0 显示 `—`） |
| 最近推送 | 120 | 相对时间 13 `--fg-2`，悬停绝对时间 |
| 操作 | 56（吸附右侧） | `.icon-btn` 更多 → Dropdown：复制克隆地址 / 收藏 ○ / 仓库设置 / 归档 / 删除（危险置底并分隔） |

- 行高 64（紧凑 56）；整行可点击进入仓库，`tabindex=0`，Enter 触发，焦点环内收（`outline-offset: -2px`）。
- 响应式：`<1200` 隐藏"合并请求"列；`<992` 隐藏"语言"；`<767` 行变两行：头像 + 名称 / 描述，下一行 `语言 · 活跃度 · 时间`。
- **分组视图：** 同 G3 的树形表，叶子为仓库行。
- **画廊视图：** `.grid` 每卡 `col-4`（≥ 1200）/ `col-6`（≥ 768）/ `col-12`；卡片 = `.sheet--clickable`，内边距 16：头像 40 + 名称 + 可见性图标；描述 13/20 最多 2 行；底部一行：语言方格 + 名称、14 格活跃度、相对时间。每页 24 张。
- 分页在 `.sheet-foot`（基线 §6.2.3），翻页保留条件，**翻页后表体 `scrollTop` 归零**。
- 空：Empty"还没有仓库"+ [新建仓库] [导入仓库]；筛选无结果："没有匹配的仓库"+ [清除筛选]。加载：5 行骨架（行高同 `--row-h`）。

### G3 仓库组（`/groups`、`/groups/:path`）

**组树（`/groups`）** —— 树形表，`.sheet` flush：

- 组行（高 56）：展开箭头（16，展开时旋转 90°，`--t-base`）+ **仓库组头像 28**（圆角方块，内含 2×2 小方格）+ 组名 14/600 + 可见性图标 Tag + 描述（`--fg-3` 单行）。右侧列：`12 个仓库`、`3 个子组`、成员头像叠放（最多 4 个 + `+N`，重叠 −6，描边用 `outline: 2px solid var(--bg-sheet)`）、最近活动相对时间。
- **缩进与导视线：** 每级缩进 24；每个祖先层级画一条 1px `--hair-soft` 竖直导视线（纯连接线，不着色，不表示状态或选中）。
- 子节点**懒加载**：展开时行内显示 CellLoader；一次最多渲染 50 个子节点，其后"显示更多（N）"。排序：**子组在前、仓库在后**，各自按名称。
- 叶子（仓库）行：仓库头像 24 + 名称 + 描述 + 语言 + 时间，字号 13，与 G2 行同语汇。
- 工具栏：搜索、可见性、"全部展开 / 全部折叠"、新建 ▾（仓库组 / 仓库）。**搜索时平铺为结果列表**，每条显示完整路径，命中文字用 `--signal-soft` 底高亮（不加下划线）。
- 空：Empty"还没有仓库组"+ [新建仓库组]。

**组详情（`/groups/:path`）** —— 沿用基线 P12 详情版式：

- 头部 `.sheet`：组头像 48 + 组名 24/600 + 路径（等宽 12）+ 可见性 Tag + 描述 + 操作 [新建仓库] [新建子组] [设置]；右侧三个紧凑读数：仓库 / 子组 / 成员。
- Tabs：**概览 · 仓库 · 子组 · 成员 · 设置**。
- 概览：左 `col-8`：**子组瓷砖**（`.grid` 每块 `col-4`：头像 + 名称 + "N 仓库"，最多 6 块 +"查看全部"）+ 最近更新的 8 个仓库行；右 `col-4`：关于 / 成员叠放 / 近 12 周活跃度方格。
- 成员 Tab：DataTable，角色列用 Select（角色集合以产品权限模型为准，变更需 ConfirmActionDialog 并展示影响范围）。

### G4 仓库首页（`/:repo`）

```
RepoHeader（.sheet）：▣ 组 / 仓库名  [私有]            [收藏 ○] [Fork ○] [克隆 ▾]
                      描述一句话……   #topic #topic
                      代码 | 提交 | 分支 | 标签 | 合并请求 (3) | 议题 ○ | 流水线 ○ | 统计 | 设置
┌ 工具栏：[⎇ main ▾]  [转到文件 t]            1,248 次提交   [添加文件 ▾] ┐
┌ 文件列表 .sheet（col-9）──────────────────────┐ ┌ 关于 .sheet（col-3，吸顶）┐
│ ◯ 张三  修复登录跳转  a1b2c3d · 2 小时前  ⟲    │ │ 描述 / 链接                │
│ ▤ src/        feat: 重构路由           3 天前  │ │ 语言 ▮▮▮▮▮▮▯▯             │
│ ▤ docs/       docs: 补充说明           1 周前  │ │ 贡献者 ◯◯◯ +5              │
│ ▢ README.md   docs: 更新               1 周前  │ │ 最新标签 v1.4.0            │
└──────────────────────────────────────────────┘ │ 近 12 周活跃度 ▢▢▣▣        │
┌ README.md .sheet ────────────────────────────┐ └──────────────────────────┘
```

- **不使用 `.sheet--hero`**：仓库页追求安静，主角是文件列表（层级最重的块），不是光晕卡。
- **RepoHeader**：见 §7.4。
- **工具栏**（位于纸页外，页面雾白底上）：`BranchSelect`（§7.6）→ 进入子目录时的路径面包屑（等宽）→ 弹性空白 → `转到文件`（次按钮 + `.kbd` t）→ 提交数链接 → `添加文件 ▾`（新建文件 / 上传文件，仅有写权限时出现）。
- **文件列表 `.sheet`：** `.sheet-head`（高 48）为**最新提交条**：作者头像 20 + 姓名 13/500 + 提交标题（单行省略，可点）+ 哈希芯片 + 相对时间 + CI 徽标（图标 + Tooltip）+ 历史 `.icon-btn`；体：行高 40，列 = 图标 16 + 名称 13/500 `--fg-1` | 最近提交说明 13 `--fg-3`（`<992` 隐藏）| 相对时间 12 `--fg-3`（宽 112，右对齐）。**目录在前文件在后**；仅含单个子目录的链路折叠为 `a/b/c`。
- **README 纸页：** 头 48：文件图标 + `README.md` 13/500 + 右侧 `.icon-btn`（目录 outline Popover、编辑）；体内边距 `--sheet-pad`，正文最大宽 `--prose-max`（§7.9 Markdown 规格）。
- **关于侧栏**（`col-3`，`position: sticky; top: calc(var(--chrome-h) + var(--sp-4))`）：**单张 `.sheet`**，内部用发丝线分段而不是套卡：描述与链接 → 语言分布（`LanguageBar`）→ 贡献者头像叠放 → 最新标签 / 版本 → 近 12 周活跃度（7×12 方格，8px）。`<992` 移到主内容下方。
- **空仓库（无提交）：** 主体替换为 Empty 变体：标题"这个仓库还是空的"，下接三块 `.inset`（"克隆并推送""导入已有仓库""在网页创建文件"），每块内含等宽命令 + 复制按钮；克隆地址**不含任何凭据**。
- 无权访问：整页 Forbidden，文案与"仓库不存在"一致，不泄露仓库是否存在（G8）。

### G5 文件视图（`/:repo/blob/:ref/:path`）

- 工具栏同 G4（分支选择 + 路径面包屑，路径等宽，末段 `--fg-1`）。
- **代码 `.sheet`：** `.sheet-head`（高 48，**吸顶** `top: var(--chrome-h)`，实色 `--bg-sheet-2`）：文件名等宽 13/500 + 元信息"142 行 · 4.6 KB" + 右侧 Segmented **[代码｜Blame]**（Markdown / 图片 / CSV 另有 **[预览]**）+ `.icon-btn`：复制内容、下载、Raw、永久链接、编辑（有权限时）、更多。头下方一行 40 高的最新提交条（同 G4）。
- **代码体：** 底 `--code-bg`；行号栏宽 56，右对齐，12 等宽 `--code-gutter-fg`；代码 13/20 等宽。**悬停行**行号变 `--fg-1`；**点击行号选中该行，Shift+点击选区间**，选中行整行 `--code-line-hl`，URL 写入 `#L10-L20`，**无左侧竖条**。
- **溢出：** 默认不折行，代码容器内部横向滚动（`overscroll-behavior-x: contain`，不撑破纸页）；更多菜单提供"自动换行"。制表符宽 4（可配置）。
- **Blame：** 每个提交块左侧一栏（宽 280，`--bg-sheet-2`）：头像 16 + 提交标题（省略）+ 相对时间；提交块之间用 `--hair-soft` 整行横线分隔，**不画竖条**。
- **边界：** 文件 > 1 MB 不渲染，显示 Empty"文件过大"+ [下载] [查看 Raw]；二进制显示预览（图片）或下载；符号链接显示目标路径。

### G6 提交（`/:repo/commits/:ref`、`/:repo/commit/:hash`）

- **工具栏：** BranchSelect + 搜索 + 作者 Select + 时间范围 + Segmented **[列表｜图谱]**。
- **列表视图：** `.sheet` flush，按日期分组；**日期分组行**高 32，实色 `--bg-sheet-2`，12/500 `--fg-3`（"2026-11-06"）。**提交行**高 56：作者头像 24 + 提交标题 13/500 `--fg-1`（单行省略；有正文时尾随 `…` 展开芯片）+ 第二行"作者 · 相对时间"12 `--fg-3`；右侧：CI 徽标（小号）+ **哈希芯片**（等宽 12，`--bg-well`，圆角 6，悬停显示复制）+ "浏览代码"`.icon-btn`。联合作者以头像叠放展示。
- **图谱视图（`CommitGraph`，§7.8）：** 行高固定 40，左侧图形列 + 右侧同列表信息（单行）。
- **提交详情：** 头部 `.sheet`：提交标题 20/600 + 正文（`pre-wrap`，`--fg-2`）+ 元信息行（作者头像 + 姓名、提交时间、父提交哈希芯片、完整哈希等宽 + 复制、所在分支芯片（懒加载）、CI 徽标）；其下**变更概览条**：`N 个文件`、`+a −d`、五格比例条（§7.10）；再下是 Diff（G10）。

### G7 分支与标签（`/:repo/branches`、`/:repo/tags`）

- Tabs **[分支｜标签]**；分支页顶部 Segmented **[概览｜活跃｜陈旧｜全部]**（陈旧 = 60 天无更新）。
- 分支 DataTable：名称（等宽 13/500 + **默认** Tag(info) + **受保护** Tag(idle，锁图标) + 陈旧 Tag(idle)）| 最后更新（相对时间 + 作者头像 20）| **领先 / 落后**（`↓3 ↑12`，`tabular-nums`，0 显示 `—`，列头"落后 / 领先默认分支"）| CI（小号徽标）| 合并请求（`#123` + 状态徽标，无则 `—`）| 操作（[新建合并请求] 文字钮 + 更多：设为默认 / 保护 / 删除，危险置底）。
- 默认分支行置顶且不可删除。删除已合并分支：标准确认；删除未合并分支：说明"N 个提交不会出现在任何其他分支"，需输入分支名确认。
- 标签 DataTable：名称（等宽）| 提交哈希芯片 | 说明 | 时间 | 操作（下载 ▾ zip / tar.gz、删除）。

### G8 合并请求列表（`/merge-requests`、`/:repo/merge-requests`）

- **状态 Tabs：** 开放 (n) · 已合并 · 已关闭 · 全部。全局页多一组 Segmented：**待我评审 · 我创建的 · 指派给我 · 全部**。
- **行**（高 72，行式列表，无卡片）：左 MR 图标（`--fg-3`，不染色）；中：第一行标题 14/500 `--fg-1` + 标签 Tag（分类色）；第二行 12 `--fg-3`：`#123`（等宽）· 作者头像 16 + 姓名 · "创建于 2 天前" · **源分支 → 目标分支**（两枚等宽芯片）；右：评审人头像叠放（每个头像右下角带 12px 状态小图标：已批准 ✓ / 请求修改 ✕）· CI 徽标 · 评论数（图标 + 数）· **状态徽标** · 更新时间。
- **状态徽标词表**见 §11.2；草稿 MR 标题前加 `草稿` Tag(idle)。

### G9 合并请求详情（`/:repo/merge-requests/:id`）

```
头部 .sheet：标题 20/600 #123      [状态徽标]   [feature/x → main]   张三 · 2 小时前   [编辑] [关闭]
Tabs：讨论 (8) | 提交 (3) | 变更 (5 个文件) | 流水线 ○
┌ 时间线 .sheet（col-8）─────────────────────┐ ┌ 侧栏 .sheet（col-4，吸顶）┐
│ 描述（Markdown）                            │ │ 评审人  ＋                │
│ ◉ AI 评审摘要 .inset（可选 §9.5）           │ │  ◯ 李四   已批准          │
│ ──────────────────────────────────         │ │  ◯ 王五   待评审          │
│ ◯ 李四 评论于 2 小时前 …                    │ │ 指派人 / 标签 / 里程碑     │
│ ◼ 推送了 3 个提交                            │ │ 关联议题 / 参与者          │
└────────────────────────────────────────────┘ │ 订阅  ○─                  │
┌ MergeBox .sheet ──────────────────────────┐ └──────────────────────────┘
│ ◼ 还有 2 项未满足 · 评审 1/2 · 流水线运行中 │
│ [合并 ▾]（禁用，Tooltip 说明原因）           │
└───────────────────────────────────────────┘
评论输入（Markdown 编辑器）
```

- **头部：** 标题、`#id`（等宽 `--fg-3`）、状态徽标、**源 → 目标分支芯片**、作者与时间；右侧按状态给操作（开放：[编辑] [关闭]；草稿：[标记为就绪] 主按钮；已关闭：[重新打开]）。**"合并"按钮只在 MergeBox 里**，头部不重复。
- **时间线是一张纸页**（`.sheet`，内边距 20），**条目之间用 `--hair-soft` 横线分隔，不把每条评论包成卡片**：
  - 评论：头像 28（圆）+ 姓名 13/500 + "评论于 2 小时前" 12 `--fg-3` + 右侧更多；正文 Markdown。
  - 系统事件（推送提交、评审结果、标签变更、合并）：单行 13 `--fg-3`，行首 24px 瓷砖（`--bg-well`，圆角 6）内放 16px 图标。
  - **时间线导线：** 瓷砖之间可连一条 1px `--hair` 竖线，只做连接，**不着色、不表示状态**。
- **MergeBox**（`.sheet`，时间线之后、评论输入之前）：
  - 头：状态汇总图标 + 一句话——"可以合并"（ok）/ "还有 N 项未满足"（warn）/ "存在冲突"（bad）/ "已合并到 main"（ok）。
  - 体：**检查清单** `MergeChecklist`（§7.11），每项高 40。
  - 尾：左——选项复选（合并后删除源分支、压缩提交）；右——**分体按钮 [合并 ▾]**：主体"合并"，下拉选择合并方式（合并提交 / 压缩合并 / 变基合并）；阻塞时按钮禁用，Tooltip 写明阻塞原因；管理员的"强制合并"藏在下拉底部，走危险确认。
- **侧栏**（`col-4`，`position: sticky; top: calc(var(--chrome-h) + var(--sp-4))`）：**单张 `.sheet`**，分段用发丝线。评审人行：头像 24 + 姓名 + 状态徽标（已批准 ok / 请求修改 bad / 待评审 idle / 评审中 info）；其下指派人、标签、里程碑、关联议题、参与者头像叠放、订阅开关。
- **提交 Tab：** 同 G6 列表，作用域限定本 MR。**变更 Tab：** G10。**流水线 Tab ○：** 流水线阶段表 + 日志。
- 合并成功：全局 Message（success）"已合并到 main"，页面状态由服务端返回刷新，不做乐观更新。

### G10 Diff 视图（MR 变更 Tab、提交详情、分支对比共用）

- **粘性工具栏**（实色 `--bg-sheet`，`top: var(--chrome-h)`，滚动离开顶部后补 `--sh-stuck`）：`N 个文件 +a −d` · Segmented **[统一｜并排]** · 忽略空白开关 · 全部展开 / 折叠 · **评审进度**（`已查看 x/N`，10 格 `CellMeter`）· [提交评审 ▾]（主按钮，仅 MR 评审场景）。
- **左：文件树**（宽 280，`position: sticky`，`top: calc(var(--chrome-h) + var(--sp-4))`，`max-height: calc(100vh - var(--chrome-h) - var(--sp-8))`，自身纵向滚动）：顶部筛选输入；行高 32：文件夹箭头 / 文件图标 + 名称 + **变更类型字母瓷砖**（`A` 新增 / `M` 修改 / `D` 删除 / `R` 重命名，等宽 11/600，圆角 6，底色取 ok / info / bad / idle，悬停 Tooltip 写全称）+ 右侧"已查看"方格复选。`<992` 变为抽屉。
- **右：文件块**（每个文件一张 `.sheet`，块间 `--gap`）：
  - **文件头**（高 44，**吸顶** `top: calc(var(--chrome-h) + var(--diff-toolbar-h))`，实色 `--bg-sheet-2`）：折叠箭头 + 路径（等宽 13，目录部分 `--fg-3`、文件名 `--fg-1` 500）+ 复制路径 `.icon-btn` + `+12 −3`（等宽，`--ok-fg` / `--bad-fg`）+ 五格比例条 + "已查看"方格复选 + 更多。重命名显示 `旧路径 → 新路径`。
  - **Diff 表：** 列 = 旧行号（48）｜新行号（48）｜符号（20）｜代码（弹性）；行高 20，等宽 13。**增行** `--diff-add-bg` + 符号 `+`（`--diff-add-sign`）；**删行** `--diff-del-bg` + 符号 `−`；**上下文行** `--diff-ctx-bg`。逐词变更叠 `--diff-add-word / --diff-del-word`。**无任何左侧竖条。**
  - **块头行**（`@@ … @@`）：高 32，`--diff-hunk-bg`，等宽 12，`--diff-hunk-fg`；行号栏放"向上 / 向下展开"`.icon-btn`；隐藏的未变更上下文显示为一行"展开 24 行"（悬停变 `--bg-hover`）。
  - **并排模式：** 左右各自带行号与代码，中间 1px `--hair`；`<992` 强制回到统一模式。
  - **行内评论：** 悬停行号出现 20×20"+"按钮（`--signal` 底白图标，圆角 6）；点击在该行下方插入整宽的**评论线程行**（`.inset`，外边距 `8px 12px`，圆角 8），含头像 24、正文、回复框、[解决] 文字钮；已解决线程折叠为一行并带 Tag「已解决」(ok)。区间评论（Shift 选多行）选区整行 `--code-line-hl`。
- **大文件与噪声：** 变更 > 1000 行的文件默认折叠，显示"此文件变更较大 · [显示变更]"；锁文件与自动生成文件默认折叠并带 Tag「自动生成」；文件 > 300 个时按 100 个分批加载；Diff 行**虚拟化渲染**。
- 二进制 / 图片：左右并排预览（显示尺寸与大小）；无法预览显示"二进制文件已修改"。
- **提交评审 Dialog**（宽 560）：评审结论三选一单选列表（**批准** / **请求修改** / **仅评论**，每项带一行说明）+ 评论框 + [提交评审] 主按钮。

### G11 设置（仓库 / 仓库组 / 个人）

- 沿用基线 P9：**左锚点导航**（宽 200，`sticky`，`top: calc(var(--chrome-h) + var(--sp-4))`，当前项 = `--signal-soft` 底 + 主题色字，无竖条）+ 右侧分区 `.sheet`（头：分区标题 16/600；体：表单；尾：右对齐 [保存]）。
- 仓库设置分区：基本信息 · 成员与权限 · 分支保护 · 合并设置 · Webhook · 部署密钥 · 集成 · **危险操作**。
- **分支保护规则**：规则表（分支模式等宽、允许推送、允许合并、强推、所需评审数）+ 新建 / 编辑 Dialog（表单：分支模式、允许推送 Select、允许合并 Select、禁止强制推送开关、所需评审人数步进、要求流水线通过开关）。
- **危险操作**置底，一张 `.sheet`，分区标题用 `--bad-fg` 字色（**不加红色边框与红色竖条**）；每项一行：标题 14/600 + 说明 13 `--fg-3` + 右侧 danger（描边）按钮；点击后进入 ConfirmActionDialog，**不可恢复项需输入仓库完整路径**。
- 有未保存修改：底部 Frost 浮条（基线 §6.7）。

### G12 访问令牌与 SSH 密钥（`/settings/tokens`、`/settings/keys`）

- **令牌表：** 名称 | 前缀（`cf_a1b2••••`，等宽，**列表只显示前缀**）| 权限范围（Tag 组）| 状态（有效 ok / 即将过期 warn / 已过期 bad / 已撤销 idle）| 最近使用 | 操作（撤销，danger，确认需输入令牌名称）。
- **创建 Dialog**（宽 560）：名称、过期（7 天 / 30 天 / 90 天 / 自定义 / 永不过期——选"永不过期"出现 warning Alert 提示风险，且可被管理员策略禁用）、权限范围（方格复选分组：读仓库、写仓库、API…）。
- **创建成功第二步：** 同一 Dialog 切换为"令牌仅显示这一次"：深底 `.console` 块展示完整令牌（等宽）+ [复制] 按钮 + warning Alert；关闭前二次提醒"关闭后无法再次查看"。
- **SSH 密钥表：** 标题 | 指纹（`SHA256:…`，等宽，省略 + 复制）| 类型 Tag(idle，如 `ed25519`) | 添加时间 | 最近使用 | 删除（danger，确认）。添加 Dialog：标题 + 多行等宽文本框（粘贴公钥）。

### G13 管理后台

- **用户与团队：** 复用基线 P7 授权表。用户状态词：正常 ok · 已禁用 idle · 待激活 info · 已锁定 warn。
- **审计日志：** 复用基线 P2：KPI 行（操作总数、失败数、敏感操作数）+ 趋势带 + 明细表。明细列：时间、操作人、动作、对象、来源 IP、结果；行点击开 Drawer（640）展示详情 JSON（`.console` 深底块）。
- **集成与 Webhook：** 列表（URL 脱敏显示 host、事件 Tag 组、状态、最近投递）；点击进 Drawer：**投递记录**表 + 请求 / 响应 Tabs（等宽）+ [重新投递]。
- **系统状态：** 复用基线 P10 运维甲板：服务方格（Git 服务 / SSH / Hook 队列 / 存储 / 搜索索引 / AI 服务，各带状态徽标、可用性 %、90 格可用性条）+ 存储 `CellMeter` + 近期事件 + 日志尾部 `.console`。Halo 与登录页、G0 的健康度同源。
- **系统设置：** G11 版式。

### G14 搜索（`/search?q=`）

- 页头：搜索输入（大号 42）+ Tabs **代码 · 仓库 · 提交 · 合并请求 · 用户**（带计数）。
- 左筛选（宽 240，`.sheet`）：仓库组 / 语言 / 路径 / 仓库，每项"方格复选 + 名称 + 计数"，默认折叠到 5 项 +"更多"。
- **AI 行：** 查询看起来像问题时，结果顶部一条 `.inset`"向 AI 提问：{查询}"，点击带着问题进入 AI 面板。
- **代码结果：** 单条 = 头（仓库路径链接 + 文件路径等宽链接 + 命中数）+ 代码片段（`--code-bg`，圆角 8，带行号，**命中文字用 `--signal-soft-2` 底**，不加下划线）；条目间用 `--hair-soft` 横线，每条最多 3 个片段 +"显示更多（N 处）"。
- 语法提示放搜索框右侧 Popover（支持的限定符：`repo:` `path:` `lang:` `author:`，以实际检索能力为准）。

### G15 用户主页（`/:user`）

- 左 `col-3`：头像 72（圆）+ 姓名 24/600 + 账号（等宽）+ 部门 / 邮箱 / 所在组；自己的主页有 [编辑资料]。
- 右 `col-9`：**贡献热力** `.sheet`（`CellGrid` 53 周 × 7 天，格 12 / 间隙 2；`<1200` 格 10；上方月份标签、左侧"一 三 五"12 `--fg-3`；右上年份 Segmented；底部图例"少 ▢▢▢▢▢ 多"）→ 置顶仓库（≤ 6，画廊卡）→ 活动时间线（`.sheet`，按月分组，节点方格）。

---

## 7. 组件规格

> 基线已有的通用组件（Button、Input、Textarea、Checkbox、Switch、Select / Combobox、Dropdown、Pagination、Tabs、Segmented、Steps、Tag、StatusBadge、DataTable、Dialog、Drawer、Popover、Tooltip、Alert、Banner、全局 Message、CellLoader、Skeleton、Empty / Error / Forbidden、KPI）**直接使用，规格见 Cell Workbench v5.6 §6**。本节只定义 Git 场景的**用法约定**与**领域新增组件**。

### 7.1 通用组件在 Git 场景的用法

| 组件 | 约定 |
| --- | --- |
| 主按钮 | 一个区域最多一个：仓库页 = 克隆；MR 详情 = 合并（在 MergeBox）；设置分区 = 保存；Diff = 提交评审 |
| 危险按钮 | 列表与设置里用 danger（描边）；**只有确认 Dialog 内**才出现 danger-solid |
| 分体按钮（合并 ▾、提交评审 ▾） | 主体与下拉箭头共用一条 1px 边，中间 1px `--hair-strong` 分隔；两半同高、同圆角外缘；禁用时整体禁用 |
| Select / Combobox | 分支、仓库组、仓库、用户选择一律 Combobox（带搜索）；面板实色纸页 portal 到 `body`，视口定位与翻转（基线 §6.2.1） |
| Tabs | 仓库页、MR 详情、仓库组详情的一级内容切换用下划线 Tabs；右侧可带计数；**不用胶囊 Tabs** |
| Segmented | 视图 / 粒度切换（列表｜分组｜画廊、统一｜并排、代码｜Blame） |
| Tag | 分类与属性（可见性、受保护、默认、已归档、标签）；**不表达状态好坏** |
| StatusBadge | 状态（开放 / 已合并 / 成功 / 失败…），词表见 §11.2，页面不得自造 |
| DataTable | 分支、标签、令牌、密钥、用户、审计；**列表型内容（仓库、提交、MR、文件）用行式列表**，沿用 DataTable 的滚动与吸顶规则但不显示表头时可省略 `<thead>` |
| Message | 仅用于瞬时反馈（已复制、已合并、已保存）；失败详情用 Alert / ErrorState，不靠 Message |
| ConfirmActionDialog | 必含对象、影响范围、后果、确认动作；不可恢复需输入对象名称（仓库为完整路径） |

### 7.2 头像（Avatar）

| 类型 | 形状 | 尺寸档 | 默认底 |
| --- | --- | --- | --- |
| **用户** `UserAvatar` | 圆（`--rad-full`） | 16 / 20 / 24 / 28 / 32 / 72 | 按用户 ID 哈希取 `--cat1…5-bg` + `--cat*-fg` 首字，字重 600，字号 = 边长 × 0.45 |
| **仓库** `RepoAvatar` | 圆角方块，圆角 = 边长 × 25%（24→6、28→7、32→8、40→10、48→12） | 24 / 28 / 32 / 40 / 48 | 同上，按仓库 ID 哈希；已上传图片 `object-fit: cover`，外缘 `outline: 1px solid var(--hair-soft); outline-offset: -1px` |
| **仓库组** `GroupAvatar` | 同仓库方块 | 24 / 28 / 40 / 48 | 同上，**中央叠一枚 2×2 小方格图案**（占边长 50%；一格 `--cat*-fg`，其余三格 `color-mix(in srgb, var(--cat*-fg) 40%, var(--cat*-bg))`）——一眼区分"组"与"仓库" |

- 头像叠放 `AvatarStack`：重叠 −6，外缘 `outline: 2px solid var(--bg-sheet)`；最多 4 个 + `+N` 芯片；悬停 Tooltip 列出全部姓名。
- 评审人头像右下角可带 12px 状态小图标（✓ 已批准 / ✕ 请求修改），图标自带 `--bg-sheet` 描边底，**必须有 Tooltip 文字**（不只靠图形）。

### 7.3 芯片：哈希、分支、可见性

- **HashChip：** 高 22，等宽 12，7 位短哈希，`--bg-well` 底，圆角 `--rad-chip`，内边距 `0 6px`；悬停 Tooltip 显示完整哈希，点击复制，图标就地切换为对勾 1.2s（不弹全局 Message，避免刷屏）。
- **BranchChip：** 高 22，分支图标 12 + 等宽名称，`--bg-well`，圆角 6，最大宽 200 省略（Tooltip 全名）；源 → 目标之间的箭头为 12px 线性图标 `--fg-3`。
- **VisibilityTag：** `Tag`（idle）+ 12px 图标 + 文字：私有（`lock`）/ 内部（`building`）/ 公开（`globe`）。**三种可见性不用三种颜色**，差别靠图标与文字。可见性取值以产品实际权限模型为准。
- 归档 Tag(idle)、模板 Tag(info)、默认 Tag(info)、受保护 Tag(idle + 锁图标)。

### 7.4 RepoHeader（仓库页头）

`.sheet`，**不使用 hero**。内边距 `20px 20px 0`，由上到下：

1. **标题行**：`RepoAvatar 40` + `组路径 /`（`--fg-3` 400，可点击回到组）+ **仓库名**（20/600 `--fg-1`）+ VisibilityTag + 归档 Tag；右侧动作区：`收藏 ○`、`Fork ○`（次按钮，带计数）、**`克隆 ▾`（主按钮）**。
2. **说明行**：描述 13/20 `--fg-2`（≤ 2 行）+ topics（Tag，分类色轮换，最多 5 个 +"更多"）。
3. **Tabs 行**：下划线 Tabs，贴纸页底，下方 `--hair` 分隔：代码 · 提交 · 分支 · 标签 · 合并请求 (n) · 议题 ○ (n) · 流水线 ○ · 统计 · 设置（无权限不渲染）。`<992` Tabs 横向滚动，隐藏滚动条。
- 归档仓库：标题下方出现 info Banner"该仓库已归档，只读"。

### 7.5 轻量协作能力（○ 可选）

收藏（Star）、Fork、关注（Watch）为可选能力：没有就整项不渲染。计数用 `tabular-nums`、K / M 缩写（§12.2）；**不做社区化的"热门榜""趋势榜"界面**。

### 7.6 BranchSelect（分支 / 标签选择器）

- **触发器：** 次按钮，高 36：分支图标 16 + 当前名称（等宽 13，最大宽 200 省略）+ 下拉箭头；在标签上时图标换 `tag`。
- **面板**（宽 320，实色纸页 + `--sh-pop`，圆角 `--rad-pop`，内边距 4）：顶部搜索输入（自动聚焦）→ Segmented **[分支｜标签]**（小号）→ 列表（项高 34，圆角 6，等宽名称；默认分支带 `默认` Tag；当前项右侧对勾）→ 底部"查看全部分支"文字链。
- 排序：默认分支 → 最近使用 → 最近更新；搜索时按匹配度。长列表虚拟化，≤ 50 项一次渲染。
- 键盘：↑↓ 移动、Enter 选择、Esc 关闭，焦点回触发器；在仓库内按 `b` 打开。
- 没有匹配项：面板内 13 `--fg-3` "没有匹配的分支"；输入的名称在**有写权限**时提供"创建分支 {name}"项。

### 7.7 CloneDropdown（克隆面板）

- 触发 = RepoHeader 的主按钮"克隆 ▾"。面板宽 400，实色纸页 Popover：
  - 顶部下划线 Tabs **[HTTPS｜SSH]**（仅启用的协议）。
  - 只读等宽输入框（`--bg-sheet-2`）+ 右侧"复制"次按钮；下方 12 `--fg-3` 提示一行（HTTPS 提示"使用账号 + 访问令牌认证"并链到 G12；SSH 提示"需要先添加 SSH 密钥"并链到 G12）。
  - 分隔线后：`下载 ZIP` / `下载 tar.gz` 两项菜单行（项高 34）；可选 ○ `在 IDE 中打开`。
- **克隆地址不含用户名或令牌**，永不拼接凭据。

### 7.8 CommitGraph（提交图）

- 渲染：SVG，图形列宽 = 泳道数 × 16 + 16（最多显示 8 条泳道，超出的合并为一条灰色 `--s6`"…"泳道）；行高固定 **40**，节点纵向居中于行（y = 20）。
- **节点 = 方格**：普通提交 10px 实心方格（圆角 2.5）；**合并提交 12px 空心方格**（2px 描边取泳道色，内填 `--bg-sheet`）；当前 HEAD 外加 `--signal` 描边环。
- **泳道线** 2px；分叉 / 合并处用半径 6 的圆弧连接；泳道色循环取 `--s1…--s5`，**默认分支恒为 `--s1`**，同一分支在整个图中保持同色。
- **引用标签**（跟在提交标题后）：分支 = 等宽芯片（`--cat1-bg/-fg`，分支图标）；标签 = 芯片（`--cat5-bg/-fg`，标签图标）；`HEAD` = `--signal-soft-2` 底 + `--signal-text` 字。
- 交互：悬停行 = `--bg-hover`；选中行 = `--sel-solid`；点击行进入提交详情。分页每次 100 个，到底自动续载并显示 CellLoader。**不使用 `opacity` 压暗其他泳道。**

### 7.9 Markdown 与代码块

- **正文**：最大宽 `--prose-max`(880)，14/24；标题 h1 24/600、h2 20/600、h3 16/600（`--fg-1`），h1/h2 下方 `--hair-soft` 分隔线；悬停标题左侧出现锚点图标。
- **链接** `--link`，悬停下划线；外链带 12px 外链图标并 `rel="noopener noreferrer"`；仓库内相对链接按当前 ref 解析。
- **行内代码**：等宽 0.92em，`--bg-well`，圆角 4，内边距 `1px 5px`。
- **代码块**：`--code-bg`，圆角 8，内边距 `12px 16px`，等宽 13/20，横向内部滚动；右上角悬停显示"复制"`.icon-btn` 与语言标签（12 `--fg-3`）。语法色只用 `--syn-*`。
- **表格**：表头 `--bg-sheet-2` 12/500 `--fg-3`；只有横向 `--hair-soft` 线，无竖线、无斑马纹；宽表在自身容器内横向滚动。
- **引用块**：**不使用左侧竖条**（G3 / 反模式）。渲染为 `.inset`（`--bg-sheet-2`，圆角 8，内边距 `12px 16px`），行首 16px 引号图标 `--fg-3`，文字 `--fg-2`。
- **提示块**（`> [!NOTE]` / `[!WARNING]` 等）渲染为基线 Alert（整圈同色系边 + 图标），不画竖条。
- **任务列表**：只读方格复选框；**图片**：`max-width: 100%`，圆角 8，外缘 `outline: 1px solid var(--hair-soft)`；**水平线** `--hair`。
- **安全**：Markdown 一律经消毒（禁止 `script`、`iframe`、内联事件、`javascript:` 链接）；用户内容永远不以 `innerHTML` 直写。Mermaid ○ 在沙箱中渲染，颜色取 `--s1…--s6`。

### 7.10 度量组件：活跃度、语言、变更

- **ActivityCells（14 天活跃度）**：14 个 8px 方格横排，间隙 2；按当日提交数映射色阶（全局固定阈值，保证跨仓库可比）：`0 → --ramp-0`、`1 → --ramp-1`、`2–3 → --ramp-2`、`4–6 → --ramp-3`、`7–10 → --ramp-4`、`≥11 → --ramp-5`。Tooltip：`2026-11-06 · 5 次提交`。
- **ContribGrid（贡献热力）**：`CellGrid` 7 行 × 53 列，格 12（`<1200` 取 10）/ 间隙 2；阈值 `0 / 1–2 / 3–5 / 6–9 / 10–14 / ≥15 → --ramp-0…5`；上方月份标签、左侧"一 三 五"12 `--fg-3`；右下图例"少 ▢▢▢▢▢▢ 多"。支持键盘方向键在格间移动，焦点格显示 Tooltip。
- **LanguageBar（语言分布）**：**20 格堆叠方格条**，每格 = 5%，高 8、圆角 2、间隙 2，用最大余数法把百分比分配到 20 格；最多展示 5 种语言 + "其他"（`--lang-other`）。图例两列：8px 方格 + 语言名 13 `--fg-2` + 百分比 12 `--fg-3` `tabular-nums`。**语言色永远与名称并列**。
- **ChangeStat（变更五格条）**：`+a −d` 文本（等宽，`--ok-fg` / `--bad-fg`）+ 5 个 8px 方格，按增删比例分配：增 `--ok-dot`、删 `--bad-dot`、无变更 `--ramp-0`；a + d = 0 时全部 `--ramp-0`。
- **AheadBehind（领先 / 落后）**：`↓3 ↑12`，`tabular-nums`，`--fg-2`；0 显示 `—`；Tooltip 写"落后默认分支 3 个提交，领先 12 个提交"。

### 7.11 合并请求组件

- **MergeChecklist 检查项**（高 40）：状态图标 16 + 标题 13/500 `--fg-1` + 说明 13 `--fg-3` + 右侧动作文字钮。状态：通过（ok，对勾）、进行中（info，14px CellLoader）、未满足（warn，感叹号）、失败 / 冲突（bad，叉号）、跳过（idle，横线）。**图标 + 文字双通道**，不只靠颜色。固定检查项（按产品能力裁剪）：评审（"需 2 人批准，已 1/2"）· 流水线 ○（"成功 / 失败 / 运行中 · 查看"）· 冲突（"无冲突 / N 个文件冲突 · [解决冲突]"）· 分支保护（"已满足受保护分支策略"）· 草稿状态。
- **ReviewerRow**：头像 24 + 姓名 13/500 + 状态徽标 + 悬停出现"重新请求评审"文字钮；"＋"打开用户 Combobox。
- **CIStatusBadge**：小号 StatusBadge（高 20）：成功 ok · 失败 bad · 运行中 info（带 CellLoader）· 排队 idle · 已取消 idle；仅图标形态用于行内，必须有 Tooltip 写文字。

### 7.12 ChangelogTimeline（更新日志时间轴，G1 右栏）

- **数据：** 平台发布说明（版本、日期、标题、摘要、类型、详情链接），来源于真实 `CHANGELOG`，不得手写示例数据。
- **结构：** 月份分组标题 12/500 `--fg-3`；条目左侧一枚 **8px 方格节点**（圆角 2；**最新 / 未读 = `--signal`，其余 = `--hair-strong`**），节点之间 1px `--hair` 导线（只做连接，不着色、不表示状态）；内容左缩进 20。
- **条目：** 第一行：版本芯片（等宽 12，`--bg-well`，圆角 6）+ 日期（12 `--fg-3` 等宽）+ 未读时的 Tag「新」(info)；标题 14/600 `--fg-1`（≤ 2 行）；摘要 13/20 `--fg-2`（≤ 3 行）；类型 Tag（新增 `cat1` / 改进 `cat2` / 修复 `cat5`，仅分类不表好坏）；"详情 →"文字链在悬停 / 聚焦时出现。
- 首页显示最近 5 条，尾部"查看全部更新日志"进入 `/changelog`（居中单栏，宽 720，同一时间轴语汇）。"全部标为已读"按用户记在服务端。
- 空："暂无更新日志"；加载：3 条骨架；失败：行内 ErrorState + 重试。

### 7.13 Markdown 编辑器（评论 / 描述）

- 下划线小号 Tabs **[编辑｜预览]**；工具栏 `.icon-btn`（标题、粗体、斜体、引用、代码、链接、列表、任务列表、@ 提及、引用对象 `#`）；输入区 14 无衬线，最小高 120，随内容增长到 400 后内部滚动。
- `@` 弹出用户 Combobox，`#` 弹出 MR / 议题 / 提交 Combobox；拖拽或粘贴图片自动上传（上传中 CellLoader，失败行内提示），插入 Markdown 图片语法。
- **⌘/Ctrl+Enter 提交**；输入法组合状态下 Enter 不提交；草稿按对象本地自动保存，输入区右下 12 `--fg-3` 提示"草稿已保存"。
- 按钮：[取消] [评论]（主）；空内容时主按钮禁用。

### 7.14 FileTree 与文件图标

- 文件树行高 32（文件列表行高 40）；目录在前，各自按名称（自然排序，不区分大小写）。
- 展开箭头 14，旋转 90°（`--t-fast`）；缩进 16 / 级；缩进导视线 1px `--hair-soft`（纯连接线）。
- 文件图标见 §8.4。选中项 = `--signal-soft` 底 + 主题色字，**无竖条**；键盘：↑↓ 移动，→ 展开，← 折叠，Enter 打开。

---

## 8. 图标体系

> 目标：**精致、统一、有记忆点**。全站只有一套图标族，所有图标出自同一栅格、同一笔画规格，并带有"格"的签名——**端点与节点是方格，而不是圆点**。

### 8.1 总则

1. **一套图标族：** 全站图标只来自本节定义的线性图标族。允许以成熟线性图标库（如 Lucide / Tabler）的几何为基础**统一改造**，但最终进入产品的必须是同一栅格、同一笔画规格的一套；**禁止同页混用多个图标库、emoji、彩色渐变图标、位图小图标。**
2. **只用组件引用：** `<Icon name="repo" size={16} />`，由 SVG sprite / 组件树摇出；禁止在页面里粘贴原始 `<svg>`。
3. **颜色：** 行内图标一律单色 `currentColor`（默认 `--fg-3`，悬停 `--fg-2`，激活 `--signal` / `--signal-text`）；语义图标取 `--ok-fg / --warn-fg / --bad-fg / --info-fg`，**必须配文字或 Tooltip**。
4. **可访问性：** 与文字并排的图标 `aria-hidden="true"`；单独出现的图标（`.icon-btn`）必须有 `aria-label` + Tooltip。

### 8.2 栅格与笔画

| 项 | 规格 |
| --- | --- |
| 画布 | `viewBox="0 0 24 24"`，安全区四周留 2 |
| 笔画 | **1.5**，`stroke-linecap: round`，`stroke-linejoin: round`，不填充（除"格点"） |
| 圆角 | 外框方块圆角 4（24 栅格下）；折角、拐角半径 2 |
| **节点方格** | 4×4，圆角 1（= 边长 × 25%），位置对齐 0.5 网格以保证 1× 屏清晰；用于分支、提交、合并、流水线等一切"节点" |
| **格点（accent）** | 每枚图标**至多一个** 3×3 实心方格（圆角 0.75），用作识别点；行内单色图标中用 `currentColor` |
| 尺寸档 | 12（Tag / 芯片内）· 14（紧凑按钮、芯片）· **16（默认行内、按钮）**· 18（侧栏导航）· 20（工具栏大图标钮）· 24（空状态、特性）· 40（瓷砖） |
| 对齐 | 与文字并排时与 `x-height` 视觉居中；图标与文字间距 6（按钮内）/ 10（导航项） |

### 8.3 图标瓷砖（Icon Tile，大场景使用）

用于 G0 特性卡、Empty、设置分区标题等"大图标"场景：

| 变体 | 尺寸 / 圆角 | 底 | 图标 | 格点 |
| --- | --- | --- | --- | --- |
| 强调瓷砖 | 40 / `--rad-pop`(10) | `--signal-soft` | 20，`--signal` | 5×5 实心方格（圆角 1.25），右上内缩 7，`--signal` |
| 小号强调 | 32 / `--rad-ctl`(8) | `--signal-soft` | 16，`--signal` | 4×4，右上内缩 6 |
| 中性瓷砖 | 24 / `--rad-chip`(6) | `--bg-well` | 16，`--fg-2` | 无（用于时间线系统事件） |

> 同一页面强调瓷砖 ≤ 6 个；瓷砖无阴影、无边框、无悬停动效。

### 8.4 文件与文件夹图标（16）

- **文件夹：** 圆角文件夹轮廓，描边 `--fg-2`，填充 `--signal-soft`（暗色同 token，已显式定义）；打开态用 `folder-open`；子模块 = 文件夹 + 链接角标；符号链接 = 文件 + 箭头角标。
- **文件：** 折角文档轮廓，描边 `--fg-3`；右下一枚 4×4 **类别格点**，色取分类色：

| 类别 | 格点色 | 常见扩展名 / 文件名 |
| --- | --- | --- |
| 代码 | `--cat1-fg` | ts tsx js jsx py go java rs c cpp h cs rb php swift kt sh sql vue css html |
| 配置 / 数据 | `--cat5-fg` | json yaml yml toml ini env xml lock csv `.gitignore` `Dockerfile` `Makefile` |
| 文档 | `--cat2-fg` | md txt rst pdf `README` `LICENSE` |
| 图片 / 媒体 | `--cat3-fg` | png jpg jpeg gif svg webp mp4 mp3 |
| 压缩 / 二进制 | `--cat4-fg` | zip tar gz jar exe bin so |
| 其他 | 无格点 | — |

### 8.5 图标清单（命名与图形要点）

| 分组 | 名称 | 图形要点 |
| --- | --- | --- |
| 导航 | `home` | 屋形轮廓，门洞处一格点 |
| 导航 | `inbox` | 收件盒，盒口一格点 |
| 导航 | `repo` | 圆角方块内 **2×2 方格**，左上一格实心——"一个格子 = 一个仓库" |
| 导航 | `group` | 两个错位叠放的 `repo` 轮廓 |
| 导航 | `search-code` | 放大镜，镜内一对 `‹ ›` |
| 导航 | `merge-request` | 两个节点方格 + 一条弧线与箭头 |
| 导航 | `issue` | 圆角方块 + 感叹号（竖线 + 格点） |
| 导航 | `pipeline` | 三个节点方格串联，带向右箭头 |
| 导航 | `users` | 两个头形轮廓 |
| 导航 | `audit` | 文档轮廓 + 对勾 |
| 导航 | `webhook` | 钩形线 + 端点方格 |
| 导航 | `activity` | 折线脉搏，末端一格点 |
| 导航 | `settings` | 三根滑杆（不用齿轮），滑钮为方格 |
| Git 对象 | `branch` | 竖线上下两个节点方格 + 右侧分叉节点方格 |
| Git 对象 | `commit` | 横线贯穿中央一个节点方格 |
| Git 对象 | `merge` | 两条线汇入一个节点方格 |
| Git 对象 | `fork` | 一个节点方格向上分成两路 |
| Git 对象 | `tag` | 标签轮廓，孔为方格 |
| Git 对象 | `diff` | 圆角方块内 `+` 与 `−` |
| Git 对象 | `compare` | 两个并排圆角方块 + 双向箭头 |
| Git 对象 | `clone` | 两层错位方块 |
| Git 对象 | `push` / `pull` | 向上 / 向下箭头 + 底线 |
| Git 对象 | `history` | 回环箭头，中心一格点 |
| Git 对象 | `blame` | 头形 + 三行线 |
| 文件 | `file` / `folder` / `folder-open` / `file-code` / `file-text` / `file-image` / `file-config` / `file-archive` | 见 §8.4 |
| 操作 | `plus` `search` `filter` `sort` `copy` `check` `x` `edit` `trash` `download` `upload` `link` `external-link` `refresh` `send` `stop` | 标准线性语汇，端点圆角 |
| 操作 | `chevron-up/down/left/right` `expand-up` `expand-down` `panel-left` | 折叠 / 展开类 |
| 操作 | `more` | **三个小方格**横排（不用圆点），`more-vertical` 竖排 |
| 状态 | `lock` `globe` `building` `eye` `eye-off` `star` `pin` `bell` `key` `shield` `user` `calendar` | 线性；`lock` 锁孔为方格 |
| 反馈 | `info` `success` `warning` `error` | 圆角方块底形（不用圆）+ 内部符号 |
| 主题 | `sun` `moon` | 线性 |
| **AI** | `sparkle` | 四角星 + 右上一格点（AI 的统一标记，仅用于 AI 相关入口） |

### 8.6 品牌标

沿用基线 §13.1 的五条纪律：**投放点**（侧栏品牌行、登录卡、G0 顶栏与页脚、标签页）、**尺寸**（`/brand/logo-mark.png` 28×28，透明底，另备 `favicon-32 / -192 / apple-touch-icon`）、**底色**（不铺 `--signal`，不加底、不加投影）、**替代文本**（产品名）、**禁止项**（不换第二个变体、不仿制 GitHub / GitLab / 钉钉 / 微信官方 Logo）。图形标应由方格构成，与整套"格"语汇一致。

---

## 9. AI 问答体验

### 9.1 定位

AI 助手是 Git 平台的**一等公民**，但它是"带来源的助手"，不是"聊天玩具"：在首页是主角（G1），在其他页面以**上下文入口**出现（§9.5）。它帮用户**理解代码、总结变更、定位答案**，不替用户做无确认的写操作。

### 9.2 AskPanel 结构（G1 中栏，`.sheet--hero`）

- **尺寸与状态机：** 纸页高度 `clamp(560px, calc(100vh - var(--chrome-h) - 160px), 760px)`，内部纵向 flex。
  - **空态：** 内容垂直居中：标题 20/600"想了解什么？" → 输入框 → 推荐问题。
  - **对话态：** 上方 transcript 区（唯一内部滚动容器，`overscroll-behavior: contain`）+ 底部固定输入框（上缘 `--hair-soft` 分隔）。
- **顶行：** 标题（对话态缩为 14/600 对话标题）+ 右侧 `.icon-btn`：对话历史（Popover 列表）、新对话。
- **输入框 `AskInput`：** 真实 `1px --hair` 边、圆角 `--rad-sheet`(12)、实色 `--bg-sheet`；聚焦 = `border-color: var(--signal)` + `--focus-halo`；最小高 96，随内容增长到 240。占位符 14 `--fg-4`："问问仓库里的代码，或描述你想完成的事…"
  - **上下文芯片行**：已引用的仓库 / 文件 / MR，等宽路径芯片（圆角 6，带 × 移除）。
  - **底部操作条**（高 44）：左——**范围 Select**（小号：全部我可访问的仓库 / 常用仓库 / 指定仓库…）+ `# 引用`按钮（Combobox，添加上下文芯片）；右——**模型 Select**（仅多模型时出现）+ **[发送]**（主按钮小号）；生成中变为 **[停止]**（次按钮）。
  - **Enter 发送，Shift+Enter 换行；输入法组合状态下 Enter 不发送。**
  - 疑似密钥 / 令牌（正则命中）时，输入框下方出现 warning Alert"检测到疑似密钥，发送前请确认"（○）。
- **推荐问题**：空态下 2 × 2 次按钮式芯片，**来自模板 + 用户常用仓库名**（如"总结 {repo} 最近一周的提交""{repo} 怎么本地运行"），不由模型自由生成；点击即填入并发送。
- **输入框下方辅助文字**：12 `--fg-3` 一行"AI 生成内容可能有误，请核对来源"（每次对话只显示一次）。

### 9.3 回答与来源

- **版式：** 对话为**平铺文本流，不用气泡**。每轮：角色行（用户 = 圆形头像 20 + "你"；AI = 20 瓷砖（`--signal-soft` 底，`sparkle` 图标）+ "AI"）→ 内容。AI 内容按 Markdown 渲染（§7.9），代码块带复制。
- **流式：** 文本逐段追加，末尾是基线 **CellCaret**（8×16 方格光标，`--signal`，呼吸）；减少动态时静止。滚动：默认跟随到底；用户上滚后停止跟随并出现"回到底部"芯片。
- **来源（必须）：** 回答中的事实性引用以上标芯片 `[1]`（16×16，圆角 4，`--signal-soft-2` 底 + `--signal-text` 11/600）标注；悬停显示 Popover 里的来源卡；回答结束后，文末"**来源（n）**"区展示 `SourceCard`：`.inset`（圆角 8）= 文件图标 + `仓库 / 路径:L12–40`（等宽 12）+ 1–2 行代码摘录（`--code-bg`，等宽 12）；点击在 Drawer（宽 800）里预览文件并滚到引用行（`#L12-L40`），Drawer 内提供"在文件视图中打开"。
- **回答动作行**（12，`.icon-btn`）：复制、有用、无用、重新生成；MR 场景 ○ 提供"引用到评论"。
- **没有来源时：** 回答头部加一行 info Alert"这条回答没有引用仓库内容"，避免用户误以为出自代码库。

### 9.4 权限、安全与边界（硬性）

1. **检索范围 = 当前用户的仓库权限**，由服务端检索层过滤；前端不信任、不重复判断。无权限仓库的内容永不出现在回答、来源、推荐问题中。
2. **范围 Select 的值显式可见**，用户始终知道"AI 正在看哪些仓库"。
3. **AI 不执行写操作。** 它给出的改动建议只能"复制"或"创建合并请求草稿"，后者必须经确认 Dialog 展示变更内容与目标分支；**绝不自动推送、合并、删除**。
4. **AI 生成内容必须可识别**：AI 角色行、"AI 生成"Tag、辅助文字三处之一不可缺（默认三者齐备）。
5. 输入与输出按公司数据规范留存与审计；面板提供"数据与隐私说明"文字链。
6. **失败可恢复：** 生成失败 = 对话流内 error Alert + [重试]；限流 = warning Alert 写明恢复时间；AI 服务不可用 = 面板内 ErrorState，**其余首页模块与全站功能不受影响**。

### 9.5 上下文入口（○ 可选，复用同一套 AskPanel 与来源规则）

| 入口 | 触发 | 落点 |
| --- | --- | --- |
| **AI 评审摘要**（MR 讨论 Tab） | 打开 MR 或点"生成摘要" | 时间线顶部 `.inset`：头（`sparkle` 瓷砖 + "AI 评审摘要" + Tag「AI 生成」(info) + 生成时间）→ 概述 2–3 句 → **关注点**列表（≤ 5 条，每条 = 严重度徽标（低 info / 中 warn / 高 bad，带文字）+ 描述 + `文件:行` 等宽链接）→ 尾部 [重新生成] [有用] [无用]。**不替代人工评审，不计入"评审通过数"。** |
| 解释这段代码 | 文件视图选中行 → 浮出"询问 AI"小按钮 | 右侧 Drawer（宽 480）= AskPanel，已带该文件与行范围为上下文芯片 |
| 解释这个 Diff | Diff 文件头"更多"菜单 | 同上 |
| 生成 MR 描述 / 提交说明 | 编辑器工具栏 `sparkle` | 结果以**建议稿**形式出现在编辑器下方 `.inset`，用户点"采用"才写入输入区 |
| 全局提问 | ⌘K 中的"询问 AI" | G1 面板或 Drawer |

---

## 10. 数据可视化

### 10.1 总则（继承基线 §7.1）

- 图表颜色固定取 `--s1…--s6`，语义固定、不得每页重配；网格 `--chart-grid`，坐标轴 `--chart-axis`。
- 缺失数据**断开并提示**，不画成 0；"量的深浅"一律走 `--ramp-*`；状态一律 StatusBadge。
- **不用环形刻度仪表、不用饼图 / 环形图**；占比用方格条（LanguageBar）、排行用横向条。
- 图表首次绘制 `--t-draw`(520ms)，数据更新 200ms 交叉淡入，不做逐位滚动。

### 10.2 Git 图表图谱

| 编号 | 图表 | 用途 | 规格要点 |
| --- | --- | --- | --- |
| KG1 | **ContribGrid** 贡献热力 | 用户主页、仓库统计 | §7.10；7 × 53 方格，固定阈值 |
| KG2 | **ActivityCells** 14 格活跃度 | 仓库行、画廊卡 | §7.10；8px 方格 |
| KG3 | **LanguageBar** 语言分布 | 仓库关于侧栏、统计 | §7.10；20 格堆叠方格条 |
| KG4 | 提交趋势（面积 / 柱） | 仓库统计 | `--s1`；粒度 Segmented [日｜周｜月]；Tooltip 显示日期与数量 |
| KG5 | 代码增删（堆叠柱） | 仓库统计 | 新增 `--ok-dot`、删除 `--bad-dot`（与 Diff 语汇一致，图例写明文字） |
| KG6 | 贡献者排行（横向方格条） | 仓库统计、仓库组概览 | 一格 = N 次提交，图例写明；前 10 + "其他" |
| KG7 | 提交时段热力 7 × 24 | 仓库统计 | 方格 12 / 间隙 2，走 `--ramp-*`，悬停显示"周三 14:00 · 12 次提交" |
| KG8 | MR 评审时长（折线 + 分位旗） | 仓库统计、管理 | P50 / P90 以旗标标注；缺失周断线 |
| KG9 | CommitGraph 提交图 | 提交页 | §7.8 |
| KG10 | 存储用量 `CellMeter` | 系统状态、仓库设置 | 20 格，≥ 80% 转 `--warn-dot`，≥ 95% 转 `--bad-dot` 并追加文字 |

### 10.3 仓库统计页（P2 分析带版式）

页头 + 时间范围 → KPI 行（提交数、贡献者数、合并请求数、平均评审时长，共用一张纸页、竖向发丝线分隔）→ 趋势带 `.sheet`（Segmented [提交｜代码增删｜贡献者]，高 220）→ 贡献者排行 + 提交时段热力（各 `col-6`）。读数一律 `tabular-nums`；DeltaPill 仅在方向有好坏含义时用绿 / 红。

---

## 11. 业务页面 → 版式映射与状态词表

### 11.1 页面映射

| 页面 | 路由（示意，以实现为准） | 版式 | 主角 | 特别要求 |
| --- | --- | --- | --- | --- |
| 未登录首页 | `/`（未登录） | G0 | Hero + 界面示例 | `bare`；不放虚构数据；有登录态直接跳 G1 |
| 登录 | `/login` | 基线 §5.3 登录 | 登录卡 | 居中单卡，不摆假读数 |
| 登录后首页 | `/` | G1 | AI 问答 | 三块独立加载；一个 Halo |
| 我的工作 | `/work` | G8 变体 | 列表 | Segmented 待我评审 / 我创建的 / 指派给我 |
| 仓库列表 | `/repos` | G2 | 列表 | 三视图；筛选入 URL |
| 仓库组 | `/groups` `/groups/:path` | G3 | 树 / 概览 | 树懒加载；搜索平铺 |
| 仓库首页 | `/:repo` | G4 | 文件列表 | 空仓库、无权限、归档 Banner |
| 文件 | `/:repo/blob/:ref/:path` | G5 | 代码 | 行锚点；大文件降级 |
| 提交 / 图谱 / 详情 | `/:repo/commits` `/:repo/commit/:hash` | G6 | 列表 / 图谱 | 提交图虚拟化分页 |
| 分支 / 标签 | `/:repo/branches` `/:repo/tags` | G7 | 表 | 受保护与默认分支规则 |
| 合并请求列表 | `/merge-requests` `/:repo/merge-requests` | G8 | 列表 | 状态 Tabs + 计数 |
| 合并请求详情 | `/:repo/merge-requests/:id` | G9 | 时间线 | 合并按钮只在 MergeBox |
| Diff | MR「变更」· 提交详情 · 对比 | G10 | Diff | 虚拟化；大文件默认折叠 |
| 仓库统计 | `/:repo/stats` | §10.3 | 趋势带 | — |
| 仓库 / 组 / 个人设置 | `/:repo/settings` … | G11 | 表单 | 危险操作置底 |
| 令牌与密钥 | `/settings/tokens` `/settings/keys` | G12 | 表 | 一次性展示；脱敏 |
| 管理后台 | `/admin/*` | G13 | 因页而异 | 仅管理员 / 审计员 / 运维可见 |
| 搜索 | `/search` | G14 | 结果 | AI 行置顶（可选） |
| 用户主页 | `/:user` | G15 | 热力图 | — |
| 更新日志 | `/changelog` | 单栏 720 时间轴 | 时间轴 | 真实发布说明 |
| 403 / 404 / 500 | — | 基线整页错误 | — | 不透露资源是否存在；不展示堆栈 |

### 11.2 统一状态词表（由 `StatusBadge` 的 `type` 映射，页面不得自造文案）

| 对象 | 状态 → 色调 |
| --- | --- |
| 合并请求 | 草稿 idle · 开放 info · 已合并 ok · 已关闭 idle |
| MR 附加状态 | 有冲突 bad · 被阻塞 warn · 可合并 ok |
| 评审结论 | 待评审 idle · 评审中 info · 已批准 ok · 请求修改 bad |
| 流水线 / CI ○ | 成功 ok · 失败 bad · 运行中 info · 排队中 idle · 已取消 idle · 已跳过 idle · 待人工确认 warn |
| 仓库 | 已归档 idle · 导入中 info · 导入失败 bad · 维护中 info |
| 访问令牌 / SSH 密钥 | 有效 ok · 即将过期 warn · 已过期 bad · 已撤销 idle |
| Webhook 投递 | 成功 ok · 失败 bad · 重试中 warn · 未触发 idle |
| 用户 | 正常 ok · 已禁用 idle · 待激活 info · 已锁定 warn |
| 服务 | 健康 ok · 降级 warn · 异常 bad · 维护中 info · 未知 idle |
| AI 评审关注点严重度 | 低 info · 中 warn · 高 bad |

> 一个对象的**不同维度分开成多个徽标**，不得合并（例如 MR 的"开放"与"有冲突"、"评审结论"是三件事）。

---

## 12. 业务格式化与文案

### 12.1 术语表（页面、提示、按钮、AI 回答统一使用）

| 标准写法 | 禁止写法 |
| --- | --- |
| 仓库 | Repo、Repository、项目（指代仓库时） |
| 仓库组 | 群组、Group、命名空间（面向用户时） |
| 合并请求（MR） | PR、Pull Request、Merge Request |
| 分支 / 标签 / 提交 | Branch / Tag / Commit |
| 评审 / 评审人 | Review / Reviewer、审查 |
| 克隆 / 推送 / 拉取 | Clone / Push / Pull |
| 合并 / 压缩合并 / 变基合并 | Merge / Squash / Rebase |
| 受保护分支 | 保护分支、Protected Branch |
| 访问令牌 | Token（单独出现时）、密钥（指令牌时） |
| SSH 密钥 | SSH Key |
| Webhook | 钩子、回调（面向用户时统一 Webhook） |
| 议题 ○ | Issue |

### 12.2 格式化

| 项 | 规则 |
| --- | --- |
| 相对时间 | < 1 分钟"刚刚"；< 1 小时"N 分钟前"；< 24 小时"N 小时前"；< 7 天"N 天前"；≥ 7 天显示 `MM-dd`（跨年 `yyyy-MM-dd`）。**悬停 Tooltip 一律给绝对时间** `yyyy-MM-dd HH:mm:ss` |
| 日期分组标题 | `yyyy-MM-dd`，12/500 `--fg-3` |
| 提交哈希 | 列表 7 位短哈希（等宽）；详情页同时给完整 40 位 + 复制 |
| 数量 | < 1000 原样；≥ 1000 缩写 `1.2k` / `3.4M`（Tooltip 给完整数字）；`tabular-nums` |
| 文件大小 | 1024 进制，`B / KB / MB / GB`，保留 1 位小数 |
| 行数 | `142 行`；变更 `+12 −3`（使用真正的减号 `−`，等宽） |
| 路径 / 分支 / 标签 | 等宽；过长中间省略（保留首段与末段）并 Tooltip 全文 |
| 百分比 | 整数或 1 位小数，不混用 |

### 12.3 文案

- 一句话，动词 + 宾语，句末不加感叹号；按钮写动词（"新建仓库""合并""撤销令牌"）。
- 空状态：标题 16/600 说明"现在是什么情况"，副文 13 `--fg-3` ≤ 2 行说"可以做什么"，**只给一个下一步按钮**。
- 错误：人话摘要 + [重试]，技术详情（错误码、Trace ID、时间）折叠展示；**不展示堆栈**。
- 无权限：写"你没有权限访问此内容。如需访问，请联系仓库所有者或管理员。"——**不得出现"仓库存在但你不能看"这类泄露存在性的表述**。
- 危险确认：写清对象、影响范围、后果；例如"将删除仓库 `team/payments`，其 128 个分支与 3,402 次提交将无法恢复。"
- 营销文案（G0）：具体、克制，说能力不说口号堆砌；不使用最高级形容词，不编造客户与数据。

---

## 13. 动效、状态、安全、无障碍与响应式

### 13.1 动效

继承基线 §10（路由级离场→进场编排 `AnimatedOutlet`、`.fade-swap` 内容切换、滑块式 Segmented 与导航激活、Message 读秒与 FLIP）。Git 领域补充：

| 场景 | 规格 |
| --- | --- |
| 树 / 分组展开 | 高度用 `grid-template-rows: 0fr → 1fr`，`--t-base`；箭头旋转 90°；子节点懒加载时行内 CellLoader |
| MergeChecklist 状态变化 | 图标与文字 200ms 交叉淡入，不跳变 |
| AI 流式输出 | **逐块追加、不逐字动画**；末尾 CellCaret 呼吸；"回到底部"芯片淡入 `--t-fast` |
| Diff 展开上下文 / 折叠文件 | 高度 `--t-base` 过渡；行本身不做动画 |
| G0 界面示例 | 首次进入视口做一次方格扫描点亮（`--t-sweep`），之后静止；**不做视差、不做循环动画** |
| 悬停 | 行悬停只改底色（`--bg-hover`），**不位移**；可点击画廊卡悬停抬升 1px（`--sh-raise`） |
| 减少动态 | 全部回到瞬时 / 淡入，CellCaret 与 CellLoader 静止 |

### 13.2 状态覆盖（每个数据区五种状态：加载 / 空 / 错误 / 无权限 / 正常）

| 数据区 | 加载 | 空 | 错误 | 无权限 |
| --- | --- | --- | --- | --- |
| 仓库 / MR / 提交列表 | 5 行骨架（行高同 `--row-h`） | Empty + 下一步 | ErrorState + 重试 | Forbidden（不泄露存在性） |
| 文件列表 / README | 骨架行 / 骨架段落 | 空仓库引导（G4） | ErrorState | Forbidden |
| 代码 / Diff | 行骨架（`--bg-well`） | "该文件为空" | 行内 ErrorState | Forbidden |
| 提交图 | 骨架 + CellLoader | "还没有提交" | ErrorState | Forbidden |
| AI 面板 | 流式 CellCaret | 推荐问题 | 对话流内 error Alert + 重试 | 范围内无可访问仓库时提示申请权限 |
| 更新日志 | 3 条骨架 | "暂无更新日志" | 行内 ErrorState | — |

**特殊场景：** ① 找不到分支 / 标签：整页提示"找不到 `feature/x`"+ [回到默认分支]；② 仓库重命名 / 迁移：顶部 info Banner"已从旧路径重定向"；③ 仓库导入中：Banner + Steps（排队 → 拉取 → 校验 → 完成）；④ 归档 / 维护中：只读 Banner，写操作按钮禁用并带 Tooltip；⑤ 大仓库：目录 > 1000 项分页，提交图分页 100。

### 13.3 安全与权限体验

1. **权限以服务端为准**，前端只负责隐藏与禁用；无权限按钮**禁用并带 Tooltip 说明原因**，而不是无声消失（管理类入口除外，整段不渲染）。
2. **不透露资源存在性**（G8）：无权访问与不存在在页面表现上一致。
3. **令牌 / 密钥：** 列表只显示前缀；完整令牌只在创建后的第二步展示一次；撤销需输入名称；SSH 指纹可复制但私钥永不出现在界面。
4. **克隆地址不含凭据；** 日志、Webhook 配置里的敏感头与 URL 查询参数脱敏显示。
5. **危险操作**（删除仓库 / 转移 / 改可见性 / 强制推送 / 删除受保护分支 / 撤销令牌 / 强制合并）：ConfirmActionDialog 含对象、影响范围、后果；不可恢复需输入仓库完整路径或对象名称。
6. **强制合并、绕过保护、管理员代操作**写入审计日志，界面上提示"此操作将被记录"。
7. **用户生成内容**（README、评论、提交信息）一律消毒后渲染，外链加 `noopener noreferrer`。
8. AI 的权限与边界见 §9.4。

### 13.4 无障碍

- 键盘可达：文件树、Diff、Combobox、Tabs、分体按钮；焦点环 `--focus-outline`，行 / 单元格内收（`outline-offset: -2px`）；鼠标点击不出现焦点环。
- **增删不只靠颜色：** Diff 行号栏的 `+ / −` 为真实文本，并带 `aria-label="新增行" / "删除行"`；变更类型字母瓷砖带 Tooltip；评审人状态小图标带文字替代。
- 图标钮必有 `aria-label`；Tooltip 延迟 300ms，键盘聚焦同样触发。
- 毛玻璃上正文用 `--fg-1/2`，对比度 ≥ 4.5:1；`prefers-reduced-transparency` 退回实色；`forced-colors` 下保留边界与符号。
- Message / Alert 的 ARIA 同基线：success / info `status`，warning / error `alert`。
- 色彩：状态色一律配文字或图标；语言色仅作标记，必配名称。

### 13.5 响应式

| 断点 | 行为 |
| --- | --- |
| ≤ 1199 | 侧栏强制收起（72 图标栏）；`--page-px` 24；G2 隐藏"合并请求"列 |
| ≤ 991 | 12 栅格列整行堆叠；G1 顺序"AI → 常用仓库 → 更新日志"；G4 关于侧栏移到下方；G10 文件树变抽屉、并排强制回统一模式；G2 隐藏"语言"列 |
| ≤ 819 | 侧栏抽屉化，顶栏最左出现菜单图标钮；G0 顶栏导航收进菜单 |
| ≤ 767 | `--page-px` 16；G2 行变两行；Dialog 限宽；面包屑只显示"← 上级 / 当前页"；G0 主标题 40 |
| ≤ 599 / ≤ 419 | 工具栏筛选收进"筛选"Popover；G0 主标题 32；Tabs 横向滚动 |

- 代码与 Diff 在小屏**横向滚动**，不折行、不缩小字号；行号栏保持吸附左侧。
- 触摸设备：可点击目标 ≥ 36×36（`.icon-btn` 保持 36）；悬停才出现的操作在触摸端常驻。

---

## 14. 前端工程约束

### 14.1 样式分层与引入顺序

```
tokens.css          // §3.1–§3.6，与 Cell Workbench 同一份，逐值一致
materials.css       // §3.7 材质
cells.css           // 方格组件族（Cell / CellMeter / CellGrid / CellBars / CellCaret / CellLoader）
git-tokens.css      // §3.8 Diff / 语法 / 语言 / 泳道
components/*.css    // 基础组件
git/*.css           // 领域组件：avatar、repo-row、group-tree、commit-graph、diff、code、markdown、ask-panel、changelog
pages/*.css         // 页面
```

- **共享包：** `tokens.css / materials.css / cells.css` 与基础组件应以公司内部共享包（如 `@company/cell-ui`）形式在各系统间复用，**禁止各系统复制粘贴后各自微调**；`git-tokens.css` 与 `git/*` 只属于 Git 平台。
- 若使用第三方组件库（如 Ant Design），通过其主题 / ConfigProvider 把 Token **只做左值映射**映射到本文件 token，映射层只减不增，不得在映射层引入新色值。
- 页面里出现 `#hex`、圆角字面量、自写 `backdrop-filter`、自写 `box-shadow` = 缺陷。

### 14.2 代码、Diff 与 Markdown 渲染

- **高亮：** 在 Web Worker 中执行，输出的 class 只映射到 `--syn-*`，不加载高亮库自带主题 CSS；一次高亮超过 5000 行时按可视窗口分块。
- **Diff：** 由服务端计算，前端只渲染；行级**虚拟化**；单文件 > 1000 变更行默认折叠；文件 > 300 分批加载。
- **提交图：** 服务端返回泳道布局，前端 SVG 渲染，分页 100，虚拟化。
- **Markdown：** 先消毒再渲染；禁止 `dangerouslySetInnerHTML` 直写未消毒内容；图片懒加载。
- **路由：** 筛选、排序、视图、分页写入 URL；代码行锚点 `#L10-L20`；永久链接用**提交哈希**而非分支名；ref 名含 `/` 时需有明确的路由分隔符（如 `/-/`）以免歧义。
- **字体：** Geist / Geist Mono 自托管、`font-display: swap`；未部署时回退系统栈，**不得因此改字号**。

### 14.3 页面进场实现纪律

沿用基线：路由级离场 → 进场编排；模块各自独立加载并各自带骨架；Tab / Segmented / 视图切换使用 `.fade-swap`（200ms）。G1 三块互不阻塞，任一块失败只在块内降级。

### 14.4 组件新增门槛

新增领域组件前必须回答：① 现有基线组件 / 方格组件能否解决；② 它属于 Git 专有还是应上升为公司共享（若其他系统也会用，应提交到共享包）；③ 它引入的新 token 是否同时给了 Light / Dark；④ 它是否满足 G1–G10。三条以上"否"则不得新增。

---

## 15. 验收清单（UI Quality Gate）

**统一性（与公司其他系统对齐）**

- [ ] `tokens.css` 与基线逐值一致；主题色 `#1677ff` 未被改动；Light / Dark 均显式定义。
- [ ] 侧栏悬浮玻璃、顶栏通栏雾面、折叠按钮在侧栏底部；与 AI 模型平台并排打开时位置、材质、动效一致。
- [ ] 主按钮、表格、表单、Select、Dialog、全局 Message 与基线观感一致；无"Git 专属"的变体按钮或表格。

**材质、边线与间距**

- [ ] 一屏边界层级 ≤ 3；列表无逐条卡片、无逐行边框与阴影。
- [ ] 控件只有 1px 真实 border、零 box-shadow；容器一边一影；**全站无单侧色条**。
- [ ] 同级纸页之间有 `--gap`；纸页自身 margin 为 0；卡内无嵌套 `.sheet`。
- [ ] 圆角档位合规（≤ 16）；方格组件圆角 = 边长 × 25%；粘性层实色。
- [ ] 一页最多一个 `.sheet--hero` 与一个 Halo；仓库页不使用 hero。

**导航与外壳**

- [ ] 侧栏只含平台级导航；仓库内二级导航是页内 Tabs；"管理"分组仅有权限者可见。
- [ ] 顶栏功能入口顺序：搜索 / AI → 新建 → 待我评审 → 通知 → 主题 → 头像；中部留空给 Message。

**列表与表格**

- [ ] 仓库 / 提交 / MR / 文件为行式列表；分支 / 标签 / 令牌 / 用户 / 审计为 DataTable。
- [ ] 表格自滚、表头容器内吸顶、翻页后 `scrollTop` 归零；筛选 / 排序 / 视图在 URL。
- [ ] 仓库行含：方块头像、组路径 + 仓库名、描述、语言方格、14 格活跃度、MR 数、最近推送。

**代码与 Diff**

- [ ] 代码 / Diff / README 底为实色；增删行为整行实色底 + `+ / −` 符号；**无左侧竖条**。
- [ ] 语法色只引用 `--syn-*`；代码字体关闭连字；选中行整行高亮。
- [ ] Diff 文件头吸顶且实色；大文件默认折叠；行级虚拟化；并排在 `<992` 回统一模式。
- [ ] 提交图节点为方格；合并提交为空心方格；默认分支恒 `--s1`。

**图标与头像**

- [ ] 全站仅一套图标族；无 emoji、无多库混用；`more` 为三方格。
- [ ] 用户头像圆、仓库 / 组头像方；仓库组头像带 2×2 图案。
- [ ] 文件图标带类别格点；文件夹填 `--signal-soft`。

**首页与 AI**

- [ ] G0 无虚构数据；一个 Halo；已登录直达 G1。
- [ ] G1 三栏 3 / 6 / 3，`<992` 堆叠顺序正确；各块独立加载与降级。
- [ ] AI 回答带来源（上标 + 来源卡）；检索范围由服务端按权限限制；不自动写入仓库；AI 内容可识别。
- [ ] 中文输入法组合状态下 Enter 不发送。
- [ ] 更新日志数据来自真实发布说明；未读节点 `--signal`。

**安全与权限**

- [ ] 克隆地址不含凭据；令牌列表只显示前缀；完整令牌仅展示一次。
- [ ] 无权限与不存在在页面表现一致；危险操作确认齐全（对象 / 范围 / 后果；不可恢复需输入完整路径或名称）。
- [ ] Markdown 已消毒；外链 `noopener noreferrer`。

**可访问性与响应式**

- [ ] 文件树、Diff、Combobox、Tabs 全键盘可达；焦点环可见；图标钮有 `aria-label`。
- [ ] 增删、状态、评审结论不只靠颜色；对比度达标（Light / Dark 均验）。
- [ ] 6 个断点逐一走查；小屏代码横向滚动；触摸目标 ≥ 36。

---

## 16. 反模式（出现即退回）

| 反模式 | 正确做法 |
| --- | --- |
| **Diff / 代码选中 / 引用块 / 评论 / 状态用左侧竖条**（`border-left`、3px 色条） | 整行实色底 + 行号栏 `+ / −`；选中行 `--code-line-hl`；引用块用 `.inset` |
| 提交图、分支图标、时间轴节点用**圆点** | 方格节点（边长 × 25% 圆角）；合并提交空心方格 |
| 每个仓库 / 提交 / MR / 评论都是带边框带阴影的**卡片** | 行式列表 + `--hair-soft` 横线；评论流为一张纸页 |
| 语言用彩色圆点，或语言色单独承载信息 | 方格语言条 + 语言名并列 |
| 用户与仓库头像都做成圆 | 用户圆、仓库 / 组方 |
| 混用多个图标库 / emoji / 渐变彩色图标 | 同一套线性图标族（§8） |
| 仓库名、提交标题用等宽字体 | 等宽只给哈希、路径、分支 / 标签名、令牌、代码 |
| 代码字体开启连字 | `font-variant-ligatures: none` |
| 克隆地址拼接用户名 / 令牌 | 地址不含凭据；认证说明放提示 |
| 令牌明文常驻列表或可反复查看 | 列表只显前缀；创建后仅展示一次 |
| AI 回答无来源、越权引用、自动写入仓库 | 来源必带；服务端按权限检索；写操作须确认 Dialog |
| AI 用聊天气泡 + 角落浮动按钮 | 首页主角面板 + 平铺文本流 + 上下文入口 |
| 营销页虚构用户数 / 仓库数；3D 插画、渐变彩球、自动播放视频 | 只用真实接口数据；方格 + 网纹 + Halo + 真实组件示例 |
| 顶栏塞满一级导航；把仓库内二级导航塞进侧栏 | 平台级导航在侧栏；仓库内用页内 Tabs |
| MR 头部与 MergeBox 重复放"合并"按钮；合并禁用却不说原因 | 合并只在 MergeBox；禁用必带 Tooltip 原因 |
| Diff 粘性文件头用半透明 / `inherit` 底 | 实色 `--bg-sheet-2`，吸顶偏移读 `--chrome-h` |
| 大 Diff 一次渲染全部行 | 虚拟化 + 默认折叠 + 分批加载 |
| 只显示相对时间或只显示绝对时间且无 Tooltip | 相对时间 + 悬停绝对时间 |
| 删除分支 / 强制推送无影响说明 | ConfirmActionDialog：对象、范围、后果 |
| 状态只靠颜色 | 色底徽标 + 文字；图标 + 文字 |
| Markdown 未消毒就渲染；`innerHTML` 直写用户内容 | 先消毒再渲染 |
| 用 `opacity` 做禁用或次要文字 | `--fg-3/4` 与 `--bg-well` |
| Message 代替页面错误；Message 刷屏（复制哈希也弹） | ErrorState + 重试；就地对勾反馈 |
| 页面里写 `#hex` / 圆角字面量 / 自写 blur / 自写阴影 | 引用 Token 与材质类 |
| 同级纸页紧贴；卡内再套卡；纸页自带 margin | 容器 `gap`；卡内用发丝线 / `.inset` |
| 一页多个 Halo / 多个 hero | 一页一个主角卡 |
| 毛玻璃上用 `--fg-4`；表格 / 代码放进半透明材质 | 毛玻璃上用 `--fg-1/2`；长文本放纸页 |
| 术语混用（PR / Repo / Pull Request） | 按 §12.1 术语表统一 |
| 无权限页面暴露"资源存在但你无权看" | 与"不存在"一致 |
| 各系统复制粘贴基线样式后各自改 | 共享包复用；改基线先改主文件 |

---

## 17. 变更与扩展

1. 说明变更原因、场景、影响范围，并判断它属于**基线（Cell Workbench）**还是**Git 领域**：影响公共 Token / 外壳 / 通用组件的，先改基线主文件并同步所有系统。
2. 先检查现有 Token / 材质 / 方格组件 / 版式 / 图表能否解决。
3. 更新本文件 → 更新 `tokens.css` / `git-tokens.css` / `materials.css` / `cells.css` 与组件 → 检查受影响页面及 Light / Dark、六个断点。
4. 新模块先回答：它属于侧栏哪个分组（还是仓库内 Tabs）？选哪个版式（§5.2）？谁是视觉主角？用哪些图表（§10.2）？涉及哪些敏感数据与危险操作？能否用方格语汇表达？AI 是否需要介入（§9）？

> **一个公司，一套设计语言。** 纸页承载内容，方格度量一切，主题蓝（#1677ff）负责行动与信号；侧栏承载结构，顶栏承载上下文；代码是长文本，永远放在实色纸页上。当视觉与数据、安全、可用性冲突，保数据、安全、可用性。
